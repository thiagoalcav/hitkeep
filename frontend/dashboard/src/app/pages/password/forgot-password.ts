import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { finalize } from 'rxjs';

// PrimeNG
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { MessageModule } from 'primeng/message';

// Core
import { Brand } from '../../core/components/brand/brand';
import { AuthService } from '../../core/services/auth.service';

@Component({
  selector: 'app-forgot-password',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    RouterLink,
    Brand,
    ButtonModule,
    InputTextModule,
    MessageModule
  ],
  template: `
    <div class="min-h-screen flex items-center justify-center bg-[var(--p-surface-ground)] p-4 transition-colors duration-200">
      <div class="w-full max-w-md">

        <div class="bg-[var(--p-surface-card)] border border-surface-200 dark:border-surface-700 rounded-xl shadow-sm p-6 md:p-8">
          <div class="mb-8 flex flex-col items-center justify-center gap-2">
            <app-brand size="large"></app-brand>
            <h1 class="text-xl font-semibold mt-4">Reset Password</h1>
            <p class="text-sm text-[var(--p-text-muted-color)] text-center">
              Enter your email address and we'll send you a link to reset your password.
            </p>
          </div>

          @if (successMessage()) {
            <div class="flex flex-col gap-4 text-center animate-fadein">
              <div class="bg-green-500/10 border border-green-500/20 text-green-600 rounded-lg p-4">
                <i class="pi pi-check-circle text-2xl mb-2"></i>
                <p class="text-sm font-medium">Check your inbox!</p>
                <p class="text-xs mt-1">If an account exists for <strong>{{ form.get('email')?.value }}</strong>, we have sent a reset link.</p>
              </div>
              <a routerLink="/login" class="text-sm text-primary hover:underline">Return to Sign In</a>
            </div>
          } @else {
            <form [formGroup]="form" (ngSubmit)="onSubmit()" class="flex flex-col gap-4">

              @if (errorMessage()) {
                <p-message severity="error" styleClass="w-full">{{ errorMessage()! }}</p-message>
              }

              <div class="flex flex-col gap-2">
                <label for="email" class="text-sm font-medium text-[var(--p-text-color)]">Email Address</label>
                <input
                  pInputText
                  id="email"
                  formControlName="email"
                  class="w-full"
                  placeholder="you@example.com"
                />
              </div>

              <p-button
                label="Send Reset Link"
                type="submit"
                [loading]="isLoading()"
                class="w-full"
                [disabled]="form.invalid">
              </p-button>

              <div class="text-center mt-2">
                <a routerLink="/login" class="text-sm text-[var(--p-text-muted-color)] hover:text-[var(--p-primary-color)] transition-colors">
                  Back to Sign In
                </a>
              </div>
            </form>
          }
        </div>
      </div>
    </div>
  `
})
export class ForgotPassword {
  private fb = inject(FormBuilder);
  private authService = inject(AuthService);

  protected isLoading = signal(false);
  protected errorMessage = signal<string | null>(null);
  protected successMessage = signal(false);

  protected form = this.fb.group({
    email: ['', [Validators.required, Validators.email]]
  });

  onSubmit() {
    if (this.form.invalid) return;

    this.isLoading.set(true);
    this.errorMessage.set(null);

    this.authService.requestPasswordReset(this.form.value.email!)
      .pipe(finalize(() => this.isLoading.set(false)))
      .subscribe({
        next: () => this.successMessage.set(true),
        error: () => {
          // Even on error, we usually don't want to block the UI for security enumeration reasons,
          // but if the API fails hard (500), show a generic message.
          this.errorMessage.set('Something went wrong. Please try again later.');
        }
      });
  }
}
