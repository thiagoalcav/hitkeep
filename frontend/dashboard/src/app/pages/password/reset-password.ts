import { Component, inject, signal, OnInit } from '@angular/core';

import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { ActivatedRoute, Router } from '@angular/router';
import { finalize } from 'rxjs';
import { TranslocoPipe } from '@jsverse/transloco';

// PrimeNG
import { ButtonModule } from 'primeng/button';
import { PasswordModule } from 'primeng/password';
import { MessageModule } from 'primeng/message';

// Core
import { Brand } from '@components/brand/brand';
import { AuthService } from '@services/auth.service';

@Component({
    selector: 'app-reset-password',
    standalone: true,
    imports: [ReactiveFormsModule, Brand, ButtonModule, PasswordModule, MessageModule, TranslocoPipe],
    styles: [
        `
            :host ::ng-deep .p-password input {
                width: 100%;
            }
        `
    ],
    template: `
        <div class="min-h-screen flex items-center justify-center bg-[var(--p-surface-ground)] p-4 transition-colors duration-200">
            <div class="w-full max-w-md">
                <div class="bg-[var(--p-surface-card)] border border-surface-200 dark:border-surface-700 rounded-xl shadow-sm p-6 md:p-8">
                    <div class="mb-8 flex flex-col items-center justify-center gap-2">
                        <app-brand size="large"></app-brand>
                        <h1 class="text-xl font-semibold mt-4">{{ 'password.reset.title' | transloco }}</h1>
                    </div>

                    @if (successMessage()) {
                        <div class="flex flex-col gap-4 text-center animate-fadein">
                            <div class="bg-green-500/10 border border-green-500/20 text-green-600 rounded-lg p-4">
                                <i class="pi pi-check-circle text-2xl mb-2"></i>
                                <p class="text-sm font-medium">{{ 'password.reset.success' | transloco }}</p>
                            </div>
                            <p-button [label]="'password.reset.signInNow' | transloco" (onClick)="goToLogin()" class="w-full"></p-button>
                        </div>
                    } @else {
                        <form (submit)="onSubmit($event)" class="flex flex-col gap-4" novalidate>
                            @if (errorMessage()) {
                                <p-message severity="error" [text]="errorMessage()! | transloco" styleClass="w-full"></p-message>
                            }

                            @if (!token) {
                                <div class="text-center text-red-500 text-sm p-4 border border-red-200 rounded bg-red-50">{{ 'password.reset.errors.tokenMissing' | transloco }}</div>
                            } @else {
                                <div class="flex flex-col gap-2">
                                    <label for="password" class="text-sm font-medium text-[var(--p-text-color)]">{{ 'password.reset.newPasswordLabel' | transloco }}</label>
                                    <p-password id="password" [formControl]="form.password().control()" [toggleMask]="true" [feedback]="true" class="w-full" [placeholder]="'common.passwordPlaceholder' | transloco"></p-password>
                                    <small class="text-xs text-muted-color">{{ 'password.reset.minimumLength' | transloco }}</small>
                                </div>

                                <p-button [label]="'password.reset.submit' | transloco" type="submit" [loading]="isLoading()" class="w-full" [disabled]="isLoading() || form().invalid()"> </p-button>
                            }
                        </form>
                    }
                </div>
            </div>
        </div>
    `
})
export class ResetPassword implements OnInit {
    private route = inject(ActivatedRoute);
    private router = inject(Router);
    private authService = inject(AuthService);

    protected token: string | null = null;
    protected isLoading = signal(false);
    protected errorMessage = signal<string | null>(null);
    protected successMessage = signal(false);

    private readonly formModel = signal({
        password: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.minLength(8)] })
    });
    protected readonly form = compatForm(this.formModel);

    ngOnInit() {
        this.token = this.route.snapshot.queryParamMap.get('token');
        if (!this.token) {
            this.errorMessage.set('password.reset.errors.invalidLink');
        }
    }

    onSubmit(event?: Event) {
        event?.preventDefault();
        if (!this.token) return;
        if (this.form().invalid()) {
            this.form.password().markAsTouched();
            return;
        }

        this.isLoading.set(true);
        this.errorMessage.set(null);

        this.authService
            .resetPassword(this.token, this.form.password().value())
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: () => this.successMessage.set(true),
                error: (err) => {
                    if (err.status === 400) {
                        this.errorMessage.set('password.reset.errors.expiredOrInvalid');
                    } else {
                        this.errorMessage.set('password.reset.errors.resetFailed');
                    }
                }
            });
    }

    goToLogin() {
        this.router.navigate(['/login']);
    }
}
