import { ChangeDetectionStrategy, Component, DestroyRef, computed, inject, signal } from "@angular/core";
import { Router, RouterLink } from "@angular/router";
import { FormControl, ReactiveFormsModule, Validators } from "@angular/forms";
import { compatForm } from "@angular/forms/signals/compat";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { finalize } from "rxjs/operators";
import { TranslocoPipe } from "@jsverse/transloco";

import { PasswordModule } from "primeng/password";
import { ButtonModule } from "primeng/button";
import { InputTextModule } from "primeng/inputtext";
import { TagModule } from "primeng/tag";

import { Brand } from "@components/brand/brand";
import { injectActiveLang } from "@core/i18n/active-lang";
import { CloudStatus } from "@models/analytics.types";
import { AnalyticsService } from "@services/analytics.service";
import { CloudService, CloudSignupRequest } from "@services/cloud.service";

type PlanCode = "free" | "pro" | "business";

interface PlanOption {
    value: PlanCode;
}

@Component({
    selector: "app-signup",
    imports: [Brand, ReactiveFormsModule, PasswordModule, ButtonModule, InputTextModule, TagModule, RouterLink, TranslocoPipe],
    templateUrl: "./signup.html",
    styleUrl: "./signup.css",
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class Signup {
    private readonly destroyRef = inject(DestroyRef);
    private readonly router = inject(Router);
    private readonly analytics = inject(AnalyticsService);
    private readonly cloud = inject(CloudService);

    protected readonly isLoading = signal(false);
    protected readonly errorMessage = signal<string | null>(null);
    protected readonly cloudStatus = signal<CloudStatus | null>(null);
    private readonly activeLanguage = injectActiveLang();
    protected readonly planOptions: readonly PlanOption[] = [{ value: "free" }, { value: "pro" }, { value: "business" }];
    protected readonly selectedPlan = signal<PlanCode>("free");
    protected readonly currentYear = new Date().getFullYear();
    protected readonly jurisdictionLabel = computed(() => this.cloudStatus()?.jurisdiction?.trim() || "EU");

    private readonly signupModel = signal({
        givenName: new FormControl("", { nonNullable: true }),
        lastName: new FormControl("", { nonNullable: true }),
        teamName: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.maxLength(120)] }),
        email: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.email] }),
        password: new FormControl("", { nonNullable: true, validators: [Validators.required, Validators.minLength(8)] })
    });
    protected readonly signupForm = compatForm(this.signupModel);

    constructor() {
        this.analytics
            .getSystemStatus()
            .pipe(takeUntilDestroyed(this.destroyRef))
            .subscribe({
                next: (status) => this.cloudStatus.set(status.cloud ?? null),
                error: (err) => {
                    console.error("Failed to load cloud status for signup", err);
                }
            });
    }

    protected selectPlan(planCode: PlanCode): void {
        this.selectedPlan.set(planCode);
    }

    protected onSubmit(event?: Event): void {
        event?.preventDefault();
        if (this.signupForm().invalid()) {
            this.signupForm.teamName().markAsTouched();
            this.signupForm.email().markAsTouched();
            this.signupForm.password().markAsTouched();
            return;
        }

        const payload: CloudSignupRequest = {
            email: this.signupForm.email().value().trim().toLowerCase(),
            password: this.signupForm.password().value(),
            team_name: this.signupForm.teamName().value().trim(),
            plan_code: this.selectedPlan(),
            jurisdiction: this.cloudStatus()?.jurisdiction,
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
}
