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

type PlanCode = "free" | "pro" | "business";
type Jurisdiction = "EU" | "US";

interface PlanOption {
    value: PlanCode;
}

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
    private hasExplicitJurisdictionSelection = false;

    protected readonly isLoading = signal(false);
    protected readonly errorMessage = signal<string | null>(null);
    protected readonly cloudStatus = signal<CloudStatus | null>(null);
    private readonly activeLanguage = injectActiveLang();
    protected readonly planOptions: readonly PlanOption[] = [{ value: "free" }, { value: "pro" }, { value: "business" }];
    protected readonly jurisdictionOptions = ["EU", "US"] as const;
    protected readonly selectedPlan = signal<PlanCode>("free");
    protected readonly selectedJurisdiction = signal<Jurisdiction>("EU");
    protected readonly currentYear = new Date().getFullYear();
    protected readonly currentJurisdiction = computed<Jurisdiction>(() => this.normalizeJurisdiction(this.cloudStatus()?.jurisdiction) ?? this.inferJurisdictionFromHost());
    protected readonly alternateJurisdiction = computed<Jurisdiction>(() => (this.currentJurisdiction() === "EU" ? "US" : "EU"));

    private readonly signupModel = signal({
        givenName: new FormControl("", { nonNullable: true }),
        lastName: new FormControl("", { nonNullable: true }),
        teamName: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.maxLength(120)] }),
        email: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.email] }),
        password: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.minLength(8)] })
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
                    if (!this.hasExplicitJurisdictionSelection) {
                        this.selectedJurisdiction.set(this.currentJurisdiction());
                    }
                },
                error: (err) => {
                    console.error("Failed to load cloud status for signup", err);
                }
            });
    }

    protected selectPlan(planCode: PlanCode): void {
        this.selectedPlan.set(planCode);
    }

    protected selectJurisdiction(jurisdiction: Jurisdiction): void {
        this.hasExplicitJurisdictionSelection = true;
        this.selectedJurisdiction.set(jurisdiction);
    }

    protected onSubmit(event?: Event): void {
        event?.preventDefault();
        if (this.signupForm().invalid()) {
            this.signupForm.teamName().markAsTouched();
            this.signupForm.email().markAsTouched();
            this.signupForm.password().markAsTouched();
            return;
        }

        if (this.selectedJurisdiction() !== this.currentJurisdiction()) {
            this.redirectToExternal(this.buildSignupUrl(this.selectedJurisdiction(), true));
            return;
        }

        const payload: CloudSignupRequest = {
            email: this.signupForm.email().value().trim().toLowerCase(),
            password: this.signupForm.password().value(),
            team_name: this.signupForm.teamName().value().trim(),
            plan_code: this.selectedPlan(),
            jurisdiction: this.currentJurisdiction(),
            locale: this.activeLanguage()
        };

        const givenName = this.signupForm.givenName().value().trim();
        const lastName = this.signupForm.lastName().value().trim();
        if (givenName !== "") {
            payload.given_name = givenName;
        }
        if (lastName !== "") {
            payload.last_name = lastName;
        }

        this.isLoading.set(true);
        this.errorMessage.set(null);
        this.cloud
            .signup(payload)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (response) => {
                    if (response.checkout_url) {
                        if (typeof window !== "undefined") {
                            window.location.assign(response.checkout_url);
                            return;
                        }
                    }

                    const target = response.redirect_url?.trim() || "/dashboard";
                    void this.router.navigateByUrl(target);
                },
                error: (err) => {
                    console.error("Cloud signup failed", err);
                    if (err.status === 409) {
                        this.errorMessage.set("signup.errors.emailExists");
                        return;
                    }
                    if (err.status === 400 || err.status === 404) {
                        this.errorMessage.set("signup.errors.planUnavailable");
                        return;
                    }
                    this.errorMessage.set("signup.errors.unexpected");
                }
            });
    }

    protected jurisdictionHref(jurisdiction: Jurisdiction): string {
        return this.buildSignupUrl(jurisdiction, true);
    }

    protected isCurrentJurisdiction(jurisdiction: Jurisdiction): boolean {
        return this.currentJurisdiction() === jurisdiction;
    }

    private hydrateFromQuery(): void {
        const params = this.route.snapshot.queryParamMap;

        const plan = this.normalizePlan(params.get("plan"));
        if (plan) {
            this.selectedPlan.set(plan);
        }

        const jurisdiction = this.normalizeJurisdiction(params.get("jurisdiction"));
        if (jurisdiction) {
            this.hasExplicitJurisdictionSelection = true;
            this.selectedJurisdiction.set(jurisdiction);
        }

        const givenName = params.get("given_name")?.trim();
        if (givenName) {
            this.signupForm.givenName().control().setValue(givenName);
        }

        const lastName = params.get("last_name")?.trim();
        if (lastName) {
            this.signupForm.lastName().control().setValue(lastName);
        }

        const teamName = params.get("team_name")?.trim();
        if (teamName) {
            this.signupForm.teamName().control().setValue(teamName);
        }

        const email = params.get("email")?.trim().toLowerCase();
        if (email) {
            this.signupForm.email().control().setValue(email);
        }
    }

    private buildSignupUrl(jurisdiction: Jurisdiction, preserveFormState: boolean): string {
        const baseURL = this.signupBaseUrl(jurisdiction);
        const url = new URL("/signup", baseURL);
        url.searchParams.set("jurisdiction", jurisdiction);
        url.searchParams.set("plan", this.selectedPlan());

        if (preserveFormState) {
            const givenName = this.signupForm.givenName().value().trim();
            const lastName = this.signupForm.lastName().value().trim();
            const teamName = this.signupForm.teamName().value().trim();
            const email = this.signupForm.email().value().trim().toLowerCase();

            if (givenName !== "") {
                url.searchParams.set("given_name", givenName);
            }
            if (lastName !== "") {
                url.searchParams.set("last_name", lastName);
            }
            if (teamName !== "") {
                url.searchParams.set("team_name", teamName);
            }
            if (email !== "") {
                url.searchParams.set("email", email);
            }
        }

        return url.toString();
    }

    private signupBaseUrl(jurisdiction: Jurisdiction): string {
        if (typeof window !== "undefined") {
            const currentHostJurisdiction = this.inferJurisdictionFromHost(window.location.hostname);
            if (currentHostJurisdiction === jurisdiction) {
                return window.location.origin;
            }
        }

        return jurisdiction === "US" ? "https://cloud.hitkeep.com" : "https://cloud.hitkeep.eu";
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

    private normalizePlan(value: string | null | undefined): PlanCode | null {
        const normalized = value?.trim().toLowerCase();
        if (normalized === "free" || normalized === "pro" || normalized === "business") {
            return normalized;
        }
        return null;
    }

    private redirectToExternal(url: string): void {
        if (typeof window !== "undefined") {
            window.location.assign(url);
        }
    }
}
