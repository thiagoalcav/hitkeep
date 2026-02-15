import { Component, inject, signal } from '@angular/core';

import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { RouterLink } from '@angular/router';
import { finalize } from 'rxjs';
import { TranslocoPipe } from '@jsverse/transloco';

// PrimeNG
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';

// Core
import { Brand } from '@components/brand/brand';
import { AuthService } from '@services/auth.service';

@Component({
    selector: 'app-forgot-password',
    standalone: true,
    imports: [ReactiveFormsModule, RouterLink, Brand, ButtonModule, InputTextModule, MessageModule, TranslocoPipe],
    template: `
        <div class="min-h-screen flex items-center justify-center bg-[var(--p-surface-ground)] p-4 transition-colors duration-200">
            <div class="w-full max-w-md">
                <div class="bg-[var(--p-surface-card)] border border-surface-200 dark:border-surface-700 rounded-xl shadow-sm p-6 md:p-8">
                    <div class="mb-8 flex flex-col items-center justify-center gap-2">
                        <app-brand size="large"></app-brand>
                        <h1 class="text-xl font-semibold mt-4">{{ 'password.forgot.title' | transloco }}</h1>
                        <p class="text-sm text-[var(--p-text-muted-color)] text-center">{{ 'password.forgot.description' | transloco }}</p>
                    </div>

                    @if (successMessage()) {
                        <div class="flex flex-col gap-4 text-center animate-fadein">
                            <div class="bg-green-500/10 border border-green-500/20 text-green-600 rounded-lg p-4">
                                <i class="pi pi-check-circle text-2xl mb-2"></i>
                                <p class="text-sm font-medium">{{ 'password.forgot.successTitle' | transloco }}</p>
                                <p class="text-xs mt-1">{{ 'password.forgot.successDescription' | transloco: { email: form.email().value() } }}</p>
                            </div>
                            <a routerLink="/login" class="text-sm text-primary hover:underline">{{ 'password.forgot.returnToSignIn' | transloco }}</a>
                        </div>
                    } @else {
                        <form (submit)="onSubmit($event)" class="flex flex-col gap-4" novalidate>
                            @if (errorMessage()) {
                                <p-message severity="error" styleClass="w-full">{{ errorMessage()! | transloco }}</p-message>
                            }

                            <div class="flex flex-col gap-2">
                                <label for="email" class="text-sm font-medium text-[var(--p-text-color)]">{{ 'common.emailAddress' | transloco }}</label>
                                <input pInputText id="email" [formControl]="form.email().control()" class="w-full" [placeholder]="'common.emailPlaceholder' | transloco" />
                            </div>

                            <p-button [label]="'password.forgot.sendResetLink' | transloco" type="submit" [loading]="isLoading()" class="w-full" [disabled]="isLoading() || form().invalid()"> </p-button>

                            <div class="text-center mt-2">
                                <a routerLink="/login" class="text-sm text-[var(--p-text-muted-color)] hover:text-[var(--p-primary-color)] transition-colors"> {{ 'password.forgot.backToSignIn' | transloco }} </a>
                            </div>
                        </form>
                    }
                </div>
            </div>
        </div>
    `
})
export class ForgotPassword {
    private authService = inject(AuthService);

    protected isLoading = signal(false);
    protected errorMessage = signal<string | null>(null);
    protected successMessage = signal(false);

    private readonly formModel = signal({
        email: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.email] })
    });
    protected readonly form = compatForm(this.formModel);

    onSubmit(event?: Event) {
        event?.preventDefault();
        if (this.form().invalid()) {
            this.form.email().markAsTouched();
            return;
        }

        this.isLoading.set(true);
        this.errorMessage.set(null);

        this.authService
            .requestPasswordReset(this.form.email().value())
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: () => this.successMessage.set(true),
                error: () => {
                    // Even on error, we usually don't want to block the UI for security enumeration reasons,
                    // but if the API fails hard (500), show a generic message.
                    this.errorMessage.set('password.forgot.errors.unexpected');
                }
            });
    }
}
