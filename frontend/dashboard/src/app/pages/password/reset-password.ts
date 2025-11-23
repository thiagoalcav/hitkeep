import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router, RouterLink } from '@angular/router';
import { finalize } from 'rxjs';

// PrimeNG
import { ButtonModule } from 'primeng/button';
import { PasswordModule } from 'primeng/password';
import { MessageModule } from 'primeng/message';

// Core
import { Brand } from '../../core/components/brand/brand';
import { AuthService } from '../../core/services/auth.service';

@Component({
  selector: 'app-reset-password',
  standalone: true,
  imports: [
    CommonModule,
    ReactiveFormsModule,
    Brand,
    ButtonModule,
    PasswordModule,
    MessageModule
  ],
  styles: [`
    :host ::ng-deep .p-password input { width: 100%; }
  `],
  template: `
    <div class="min-h-screen flex items-center justify-center bg-[var(--p-surface-ground)] p-4 transition-colors duration-200">
      <div class="w-full max-w-md">

        <div class="bg-[var(--p-surface-card)] border border-surface-200 dark:border-surface-700 rounded-xl shadow-sm p-6 md:p-8">
          <div class="mb-8 flex flex-col items-center justify-center gap-2">
            <app-brand size="large"></app-brand>
            <h1 class="text-xl font-semibold mt-4">Set New Password</h1>
          </div>

          @if (successMessage()) {
             <div class="flex flex-col gap-4 text-center animate-fadein">
              <div class="bg-green-500/10 border border-green-500/20 text-green-600 rounded-lg p-4">
                <i class="pi pi-check-circle text-2xl mb-2"></i>
                <p class="text-sm font-medium">Password updated successfully!</p>
              </div>
              <p-button label="Sign In Now" (onClick)="goToLogin()" class="w-full"></p-button>
            </div>
          } @else {
            <form [formGroup]="form" (ngSubmit)="onSubmit()" class="flex flex-col gap-4">

              @if (errorMessage()) {
                <p-message severity="error" [text]="errorMessage()!" styleClass="w-full"></p-message>
              }

              @if (!token) {
                <div class="text-center text-red-500 text-sm p-4 border border-red-200 rounded bg-red-50">
                   Invalid link. The token is missing.
                </div>
              } @else {
                <div class="flex flex-col gap-2">
                  <label for="password" class="text-sm font-medium text-[var(--p-text-color)]">New Password</label>
                  <p-password
                    id="password"
                    formControlName="password"
                    [toggleMask]="true"
                    [feedback]="true"
                    class="w-full"
                    placeholder="••••••••"
                  ></p-password>
                  <small class="text-xs text-muted-color">Must be at least 8 characters.</small>
                </div>

                <p-button
                  label="Reset Password"
                  type="submit"
                  [loading]="isLoading()"
                  class="w-full"
                  [disabled]="form.invalid">
                </p-button>
              }
            </form>
          }
        </div>
      </div>
    </div>
  `
})
export class ResetPassword implements OnInit {
  private fb = inject(FormBuilder);
  private route = inject(ActivatedRoute);
  private router = inject(Router);
  private authService = inject(AuthService);

  protected token: string | null = null;
  protected isLoading = signal(false);
  protected errorMessage = signal<string | null>(null);
  protected successMessage = signal(false);

  protected form = this.fb.group({
    password: ['', [Validators.required, Validators.minLength(8)]]
  });

  ngOnInit() {
    this.token = this.route.snapshot.queryParamMap.get('token');
    if (!this.token) {
      this.errorMessage.set('Invalid password reset link.');
    }
  }

  onSubmit() {
    if (this.form.invalid || !this.token) return;

    this.isLoading.set(true);
    this.errorMessage.set(null);

    this.authService.resetPassword(this.token, this.form.value.password!)
      .pipe(finalize(() => this.isLoading.set(false)))
      .subscribe({
        next: () => this.successMessage.set(true),
        error: (err) => {
          if (err.status === 400) {
            this.errorMessage.set('This link has expired or is invalid.');
          } else {
            this.errorMessage.set('Failed to reset password. Please try again.');
          }
        }
      });
  }

  goToLogin() {
    this.router.navigate(['/login']);
  }
}
