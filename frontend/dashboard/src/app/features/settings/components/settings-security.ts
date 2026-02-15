import { Component, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { finalize } from 'rxjs';
import { TranslocoPipe } from '@jsverse/transloco';

// PrimeNG
import { ButtonModule } from 'primeng/button';
import { PasswordModule } from 'primeng/password';
import { MessageModule } from 'primeng/message';
import { SettingsCard } from '@features/settings/components/settings-card';

// Core
import { AuthService } from '@services/auth.service';

@Component({
    selector: 'app-settings-security',
    standalone: true,
    imports: [CommonModule, ReactiveFormsModule, ButtonModule, PasswordModule, MessageModule, SettingsCard, TranslocoPipe],
    styles: [
        `
            :host ::ng-deep .p-password input {
                width: 100%;
            }

            :host ::ng-deep .settings-action-btn {
                min-width: 11rem;
                justify-content: center;
            }
        `
    ],
    template: `
        <app-settings-card [title]="'settings.security.title' | transloco" [subtitle]="'settings.security.changePasswordTitle' | transloco" icon="pi pi-shield">
            <form id="settings-security-form" settings-card-body (submit)="onSubmit($event)" class="flex flex-col gap-4" novalidate>
                @if (error()) {
                    <p-message severity="error" styleClass="w-full">{{ error()! | transloco }}</p-message>
                }
                @if (success()) {
                    <p-message severity="success" styleClass="w-full">{{ 'settings.security.passwordUpdated' | transloco }}</p-message>
                }

                <div class="flex flex-col gap-2">
                    <label for="currentPassword" class="text-sm font-medium">{{ 'settings.security.currentPasswordLabel' | transloco }}</label>
                    <p-password
                        id="currentPassword"
                        [formControl]="form.currentPassword().control()"
                        [toggleMask]="true"
                        [feedback]="false"
                        class="w-full"
                        [placeholder]="'common.passwordPlaceholder' | transloco"
                        [class.ng-invalid]="form.currentPassword().touched() && form.currentPassword().invalid()"
                        [class.ng-dirty]="form.currentPassword().dirty()"
                    ></p-password>
                </div>

                <div class="flex flex-col gap-2">
                    <label for="newPassword" class="text-sm font-medium">{{ 'settings.security.newPasswordLabel' | transloco }}</label>
                    <p-password id="newPassword" [formControl]="form.newPassword().control()" [toggleMask]="true" [feedback]="true" class="w-full" [placeholder]="'common.passwordPlaceholder' | transloco"></p-password>
                    <small class="text-xs text-muted-color">{{ 'settings.security.minimumLengthHint' | transloco }}</small>
                </div>
            </form>

            <div settings-card-footer class="flex flex-col items-end gap-1">
                <button pButton type="submit" form="settings-security-form" icon="pi pi-check" [loading]="isLoading()" [disabled]="isLoading() || form().invalid()" class="settings-action-btn">
                    {{ 'settings.security.updatePassword' | transloco }}
                </button>
            </div>
        </app-settings-card>
    `
})
export class SettingsSecurity {
    private authService = inject(AuthService);

    protected isLoading = signal(false);
    protected error = signal<string | null>(null);
    protected success = signal(false);

    private readonly formModel = signal({
        currentPassword: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
        newPassword: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.minLength(8)] })
    });
    protected readonly form = compatForm(this.formModel);

    onSubmit(event?: Event) {
        event?.preventDefault();
        if (this.form().invalid()) {
            this.form.currentPassword().markAsTouched();
            this.form.newPassword().markAsTouched();
            return;
        }

        this.isLoading.set(true);
        this.error.set(null);
        this.success.set(false);

        const currentPassword = this.form.currentPassword().value();
        const newPassword = this.form.newPassword().value();

        this.authService
            .changePassword(currentPassword, newPassword)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: () => {
                    this.success.set(true);
                    this.form.currentPassword().control().reset('');
                    this.form.newPassword().control().reset('');
                },
                error: (err) => {
                    // 403 is returned by backend for invalid current password
                    // TODO: adhere to RESTFUL
                    const msg = err.status === 403 ? 'settings.security.errors.invalidCurrentPassword' : 'settings.security.errors.updateFailed';
                    this.error.set(msg);
                }
            });
    }
}
