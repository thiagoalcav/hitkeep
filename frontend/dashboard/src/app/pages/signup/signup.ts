import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, signal } from "@angular/core";
import { ActivatedRoute, Router, RouterLink } from "@angular/router";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { finalize } from "rxjs/operators";
import { TranslocoPipe } from "@jsverse/transloco";

import { PasswordModule } from "primeng/password";
import { ButtonModule } from "primeng/button";
import { InputTextModule } from "primeng/inputtext";

import { Brand } from "@components/brand/brand";
import { injectActiveLang } from "@core/i18n/active-lang";
import { CloudStatus } from "@models/analytics.types";
import { AnalyticsService } from "@services/analytics.service";
import { CloudService, CloudSignupRequest } from "@services/cloud.service";

type Jurisdiction = "EU" | "US";

@Component({
    selector: "app-signup",
    imports: [Brand, ReactiveFormsModule, PasswordModule, ButtonModule, InputTextModule, RouterLink, TranslocoPipe],
    templateUrl: "./signup.html",
    styleUrl: "./signup.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class Signup {
    private readonly destroyRef = inject(DestroyRef);
    private readonly router = inject(Router);
    private readonly route = inject(ActivatedRoute);
    private readonly analytics = inject(AnalyticsService);
    private readonly cloud = inject(CloudService);

    protected readonly isLoading = signal(false);
    protected readonly errorMessage = signal<string | null>(null);
    protected readonly cloudStatus = signal<CloudStatus | null>(null);
    private readonly activeLanguage = injectActiveLang();
    protected readonly currentYear = new Date().getFullYear();
    protected readonly currentJurisdiction = computed<Jurisdiction>(() => this.normalizeJurisdiction(this.cloudStatus()?.jurisdiction) ?? this.inferJurisdictionFromHost());
    protected readonly alternateJurisdiction = computed<Jurisdiction>(() => (this.currentJurisdiction() === "EU" ? "US" : "EU"));

    private readonly signupModel = signal({
        email: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.email] }),
        password: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.minLength(8)] }),
        teamName: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.maxLength(120)] })
    });
    protected readonly signupForm = compatForm(this.signupModel);

    constructor() {
        this.hydrateFromQuery();
        this.analytics
            .getSystemStatus()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: (status) => {
                    this.cloudStatus.set(status.cloud ?? null);
                },
                error: (err) => {
                    console.error("Failed to load cloud status for signup", err);
                }
            });
    }

    protected onSubmit(event?: Event): void {
        event?.preventDefault();
        if (this.signupForm().invalid()) {
            this.signupForm.email().markAsTouched();
            this.signupForm.password().markAsTouched();
            this.signupForm.teamName().markAsTouched();
            return;
        }

        const payload: CloudSignupRequest = {
            email: this.signupForm.email().value().trim().toLowerCase(),
            password: this.signupForm.password().value(),
            team_name: this.signupForm.teamName().value().trim(),
            plan_code: "free",
            jurisdiction: this.currentJurisdiction(),
            locale: this.activeLanguage()
        };

        this.isLoading.set(true);
        this.errorMessage.set(null);
        this.cloud
            .signup(payload)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (response) => {
                    const target = response.redirect_url?.trim() || "/dashboard";
                    void this.router.navigateByUrl(target);
                },
                error: (err) => {
                    console.error("Cloud signup failed", err);
                    if (err.status === 409) {
                        this.errorMessage.set("signup.errors.emailExists");
                        return;
                    }
                    this.errorMessage.set("signup.errors.unexpected");
                }
            });
    }

    protected jurisdictionHref(jurisdiction: Jurisdiction): string {
        const baseURL = jurisdiction === "US" ? "https://cloud.hitkeep.com" : "https://cloud.hitkeep.eu";
        const url = new URL("/signup", baseURL);

        const email = this.signupForm.email().value().trim().toLowerCase();
        const teamName = this.signupForm.teamName().value().trim();
        if (email !== "") {
            url.searchParams.set("email", email);
        }
        if (teamName !== "") {
            url.searchParams.set("team_name", teamName);
        }

        return url.toString();
    }

    private hydrateFromQuery(): void {
        const params = this.route.snapshot.queryParamMap;

        const teamName = params.get("team_name")?.trim();
        if (teamName) {
            this.signupForm.teamName().control().setValue(teamName);
        }

        const email = params.get("email")?.trim().toLowerCase();
        if (email) {
            this.signupForm.email().control().setValue(email);
        }
    }

    private inferJurisdictionFromHost(hostname?: string): Jurisdiction {
        const value = (hostname ?? (typeof window !== "undefined" ? window.location.hostname : "")).trim().toLowerCase();
        if (value === "cloud.hitkeep.com" || value.endsWith(".hitkeep.com")) {
            return "US";
        }
        return "EU";
    }

    private normalizeJurisdiction(value: string | null | undefined): Jurisdiction | null {
        const normalized = value?.trim().toUpperCase();
        if (normalized === "EU" || normalized === "US") {
            return normalized;
        }
        return null;
    }
}
