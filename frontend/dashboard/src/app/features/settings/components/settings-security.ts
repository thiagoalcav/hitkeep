import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { finalize } from 'rxjs';

// PrimeNG
import { ButtonModule } from 'primeng/button';
import { PasswordModule } from 'primeng/password';
import { MessageModule } from 'primeng/message';

// Core
import { AuthService } from '@services/auth.service';

@Component({
    selector: 'app-settings-security',
    standalone: true,
    imports: [CommonModule, ReactiveFormsModule, ButtonModule, PasswordModule, MessageModule],
    styles: [
        `
            :host ::ng-deep .p-password input {
                width: 100%;
            }
        `
    ],
    template: `
        <div class="bg-[var(--p-surface-card)] border border-surface-200 dark:border-surface-700 rounded-xl shadow-sm p-6">
            <h2 class="text-lg font-semibold mb-4 flex items-center gap-2"><i class="pi pi-shield text-primary"></i> Security</h2>

            <form [formGroup]="form" (ngSubmit)="onSubmit()" class="flex flex-col gap-4">
                <h3 class="text-sm font-medium text-muted-color uppercase tracking-wider mb-2">Change Password</h3>

                @if (error()) {
                    <p-message severity="error" styleClass="w-full">{{ error()! }}</p-message>
                }
                @if (success()) {
                    <p-message severity="success" styleClass="w-full">Password updated successfully.</p-message>
                }

                <div class="flex flex-col gap-2">
                    <label for="currentPassword" class="text-sm font-medium">Current Password</label>
                    <p-password
                        id="currentPassword"
                        formControlName="currentPassword"
                        [toggleMask]="true"
                        [feedback]="false"
                        class="w-full"
                        placeholder="••••••••"
                        [class.ng-invalid]="form.get('currentPassword')?.touched && form.get('currentPassword')?.invalid"
                        [class.ng-dirty]="form.get('currentPassword')?.dirty"
                    ></p-password>
                </div>

                <div class="flex flex-col gap-2">
                    <label for="newPassword" class="text-sm font-medium">New Password</label>
                    <p-password id="newPassword" formControlName="newPassword" [toggleMask]="true" [feedback]="true" class="w-full" placeholder="••••••••"></p-password>
                    <small class="text-xs text-muted-color">Minimum 8 characters</small>
                </div>

                <div class="flex justify-end mt-4">
                    <p-button label="Update Password" type="submit" [loading]="isLoading()" [disabled]="form.invalid"></p-button>
                </div>
            </form>
        </div>
    `
})
export class SettingsSecurity {
    private fb = inject(FormBuilder);
    private authService = inject(AuthService);

    protected isLoading = signal(false);
    protected error = signal<string | null>(null);
    protected success = signal(false);

    protected form = this.fb.group({
        currentPassword: ['', [Validators.required]],
        newPassword: ['', [Validators.required, Validators.minLength(8)]]
    });

    onSubmit() {
        if (this.form.invalid) return;

        this.isLoading.set(true);
        this.error.set(null);
        this.success.set(false);

        const { currentPassword, newPassword } = this.form.value;

        this.authService
            .changePassword(currentPassword!, newPassword!)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: () => {
                    this.success.set(true);
                    this.form.reset();
                },
                error: (err) => {
                    // 403 is returned by backend for invalid current password
                    // TODO: adhere to RESTFUL
                    const msg = err.status === 403 ? 'Incorrect current password.' : 'Failed to update password. Please try again.';
                    this.error.set(msg);
                }
            });
    }
}
