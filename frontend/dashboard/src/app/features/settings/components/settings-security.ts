import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { firstValueFrom, finalize } from 'rxjs';
import { TranslocoPipe } from '@jsverse/transloco';

// PrimeNG
import { ButtonModule } from 'primeng/button';
import { PasswordModule } from 'primeng/password';
import { MessageModule } from 'primeng/message';
import { InputOtpModule } from 'primeng/inputotp';
import { SettingsCard } from '@features/settings/components/settings-card';
import { RelativeDateTime } from '@components/relative-date-time/relative-date-time';

// Core
import { AuthService } from '@services/auth.service';
import { PasskeyRegistrationFinishRequest, PasskeyRegistrationStartResponse, UserSecurityService, UserSecurityStatus, UserTotpSetup } from '@services/user-security.service';

@Component({
    selector: 'app-settings-security',
    imports: [CommonModule, ReactiveFormsModule, ButtonModule, PasswordModule, MessageModule, InputOtpModule, SettingsCard, RelativeDateTime, TranslocoPipe],
    templateUrl: './settings-security.html',
    styleUrl: './settings-security.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class SettingsSecurity {
    private authService = inject(AuthService);
    private securityService = inject(UserSecurityService);

    protected readonly isPasswordLoading = signal(false);
    protected readonly passwordError = signal<string | null>(null);
    protected readonly passwordSuccess = signal(false);

    protected readonly isSecurityLoading = signal(false);
    protected readonly securityError = signal<string | null>(null);
    protected readonly securityStatus = signal<UserSecurityStatus | null>(null);
    protected readonly totpSetup = signal<UserTotpSetup | null>(null);
    protected readonly totpError = signal<string | null>(null);
    protected readonly totpSuccess = signal<string | null>(null);
    protected readonly passkeyError = signal<string | null>(null);
    protected readonly passkeySuccess = signal<string | null>(null);
    protected readonly isTotpActionLoading = signal(false);
    protected readonly isPasskeyActionLoading = signal(false);

    private readonly formModel = signal({
        currentPassword: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
        newPassword: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.minLength(8)] }),
        totpCode: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(/^[0-9]{6}$/)] }),
        disableTotpCode: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(/^[0-9]{6}$/)] }),
        passkeyName: new FormControl('', { nonNullable: true, validators: [Validators.maxLength(64)] })
    });
    protected readonly form = compatForm(this.formModel);
    protected readonly totpEnabled = computed(() => this.securityStatus()?.totp_enabled ?? false);
    protected readonly passkeys = computed(() => this.securityStatus()?.passkeys ?? []);

    constructor() {
        this.loadSecurityStatus();
    }

    protected onSubmit(event?: Event): void {
        event?.preventDefault();
        if (this.form().invalid()) {
            this.form.currentPassword().markAsTouched();
            this.form.newPassword().markAsTouched();
            return;
        }

        this.isPasswordLoading.set(true);
        this.passwordError.set(null);
        this.passwordSuccess.set(false);

        const currentPassword = this.form.currentPassword().value();
        const newPassword = this.form.newPassword().value();

        this.authService
            .changePassword(currentPassword, newPassword)
            .pipe(finalize(() => this.isPasswordLoading.set(false)))
            .subscribe({
                next: () => {
                    this.passwordSuccess.set(true);
                    this.form.currentPassword().control().reset('');
                    this.form.newPassword().control().reset('');
                },
                error: (err) => {
                    // 403 is returned by backend for invalid current password
                    // TODO: adhere to RESTFUL
                    const msg = err.status === 403 ? 'settings.security.errors.invalidCurrentPassword' : 'settings.security.errors.updateFailed';
                    this.passwordError.set(msg);
                }
            });
    }

    protected startTotpSetup(): void {
        this.isTotpActionLoading.set(true);
        this.totpError.set(null);
        this.totpSuccess.set(null);

        this.securityService
            .startTotpSetup()
            .pipe(finalize(() => this.isTotpActionLoading.set(false)))
            .subscribe({
                next: (setup) => {
                    this.totpSetup.set(setup);
                    this.form.totpCode().control().reset('');
                },
                error: () => {
                    this.totpError.set('settings.security.twoFactor.errors.startFailed');
                }
            });
    }

    protected verifyTotpSetup(): void {
        const control = this.form.totpCode().control();
        if (control.invalid || !this.totpSetup()) {
            control.markAsTouched();
            return;
        }

        this.isTotpActionLoading.set(true);
        this.totpError.set(null);
        this.totpSuccess.set(null);

        this.securityService
            .verifyTotpSetup(control.value)
            .pipe(finalize(() => this.isTotpActionLoading.set(false)))
            .subscribe({
                next: (status) => {
                    this.securityStatus.set(status);
                    this.totpSetup.set(null);
                    control.reset('');
                    this.form.disableTotpCode().control().reset('');
                    this.totpSuccess.set('settings.security.twoFactor.enabled');
                },
                error: () => {
                    this.totpError.set('settings.security.twoFactor.errors.verifyFailed');
                }
            });
    }

    protected disableTotp(): void {
        const control = this.form.disableTotpCode().control();
        if (control.invalid || !this.totpEnabled()) {
            control.markAsTouched();
            return;
        }

        this.isTotpActionLoading.set(true);
        this.totpError.set(null);
        this.totpSuccess.set(null);

        this.securityService
            .disableTotp(control.value)
            .pipe(finalize(() => this.isTotpActionLoading.set(false)))
            .subscribe({
                next: (status) => {
                    this.securityStatus.set(status);
                    this.totpSetup.set(null);
                    this.form.totpCode().control().reset('');
                    control.reset('');
                    this.totpSuccess.set('settings.security.twoFactor.disabled');
                },
                error: () => {
                    this.totpError.set('settings.security.twoFactor.errors.disableFailed');
                }
            });
    }

    protected async registerPasskey(): Promise<void> {
        if (typeof window === 'undefined' || typeof navigator === 'undefined' || !window.PublicKeyCredential || !navigator.credentials) {
            this.passkeyError.set('settings.security.passkeys.errors.notSupported');
            return;
        }

        if (this.form.passkeyName().invalid()) {
            this.form.passkeyName().markAsTouched();
            return;
        }

        const requestedName = this.form.passkeyName().value().trim();
        this.isPasskeyActionLoading.set(true);
        this.passkeyError.set(null);
        this.passkeySuccess.set(null);

        try {
            const begin = await firstValueFrom(this.securityService.startPasskeyRegistration(requestedName || undefined));
            const credential = await navigator.credentials.create({
                publicKey: this.toPublicKeyOptions(begin)
            });

            if (!(credential instanceof PublicKeyCredential) || !(credential.response instanceof AuthenticatorAttestationResponse)) {
                this.passkeyError.set('settings.security.passkeys.errors.registrationFailed');
                return;
            }

            const payload = this.toPasskeyFinishPayload(credential, requestedName || undefined);
            if (!payload) {
                this.passkeyError.set('settings.security.passkeys.errors.notSupported');
                return;
            }
            const status = await firstValueFrom(this.securityService.finishPasskeyRegistration(payload));
            this.securityStatus.set(status);
            this.form.passkeyName().control().reset('');
            this.passkeySuccess.set('settings.security.passkeys.registered');
        } catch {
            this.passkeyError.set('settings.security.passkeys.errors.registrationFailed');
        } finally {
            this.isPasskeyActionLoading.set(false);
        }
    }

    protected deletePasskey(passkeyID: string): void {
        this.isPasskeyActionLoading.set(true);
        this.passkeyError.set(null);
        this.passkeySuccess.set(null);

        this.securityService
            .deletePasskey(passkeyID)
            .pipe(finalize(() => this.isPasskeyActionLoading.set(false)))
            .subscribe({
                next: () => {
                    this.securityStatus.update((current) => {
                        if (!current) return current;
                        return {
                            ...current,
                            passkeys: current.passkeys.filter((entry) => entry.id !== passkeyID)
                        };
                    });
                    this.passkeySuccess.set('settings.security.passkeys.deleted');
                },
                error: () => {
                    this.passkeyError.set('settings.security.passkeys.errors.deleteFailed');
                }
            });
    }

    private loadSecurityStatus(): void {
        this.isSecurityLoading.set(true);
        this.securityError.set(null);

        this.securityService
            .loadStatus()
            .pipe(finalize(() => this.isSecurityLoading.set(false)))
            .subscribe({
                next: (status) => {
                    this.securityStatus.set(status);
                },
                error: () => {
                    this.securityError.set('settings.security.errors.loadFailed');
                }
            });
    }

    private toPublicKeyOptions(response: PasskeyRegistrationStartResponse): PublicKeyCredentialCreationOptions {
        const { publicKey } = response;
        return {
            challenge: this.base64UrlToArrayBuffer(publicKey.challenge),
            rp: {
                name: publicKey.rp.name,
                id: publicKey.rp.id
            },
            user: {
                id: this.base64UrlToArrayBuffer(publicKey.user.id),
                name: publicKey.user.name,
                displayName: publicKey.user.displayName
            },
            pubKeyCredParams: publicKey.pubKeyCredParams,
            timeout: publicKey.timeout,
            attestation: publicKey.attestation,
            authenticatorSelection: {
                residentKey: publicKey.authenticatorSelection.residentKey,
                userVerification: publicKey.authenticatorSelection.userVerification
            }
        };
    }

    private toPasskeyFinishPayload(credential: PublicKeyCredential, name?: string): PasskeyRegistrationFinishRequest | null {
        const response = credential.response as AuthenticatorAttestationResponse;
        const publicKey = response.getPublicKey ? response.getPublicKey() : null;
        if (!publicKey) {
            return null;
        }
        const transports = response.getTransports ? response.getTransports() : [];

        return {
            name: name?.trim() || undefined,
            credential_id: credential.id,
            client_data_json: this.arrayBufferToBase64Url(response.clientDataJSON),
            public_key: this.arrayBufferToBase64Url(publicKey),
            transports: transports.length > 0 ? transports : undefined
        };
    }

    private base64UrlToArrayBuffer(value: string): ArrayBuffer {
        const normalized = value.replace(/-/g, '+').replace(/_/g, '/');
        const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4);
        const binary = atob(padded);
        const out = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i += 1) {
            out[i] = binary.charCodeAt(i);
        }
        return out.buffer.slice(0);
    }

    private arrayBufferToBase64Url(value: ArrayBuffer): string {
        const bytes = new Uint8Array(value);
        let binary = '';
        for (const byte of bytes) {
            binary += String.fromCharCode(byte);
        }
        return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
    }
}
