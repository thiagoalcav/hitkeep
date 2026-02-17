import { ChangeDetectionStrategy, Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router, RouterLink } from '@angular/router';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
import { compatForm } from '@angular/forms/signals/compat';
import { firstValueFrom } from 'rxjs';
import { finalize } from 'rxjs/operators';
import { TranslocoPipe } from '@jsverse/transloco';

import { PasswordModule } from 'primeng/password';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { CheckboxModule } from 'primeng/checkbox';
import { InputOtpModule } from 'primeng/inputotp';

import { Brand } from '@components/brand/brand';
import { AuthService, LoginResponse, PasskeyLoginFinishRequest, PasskeyLoginStartResponse } from '@services/auth.service';
import { UserPreferencesService } from '@services/user-preferences.service';

@Component({
    selector: 'app-login',
    standalone: true,
    imports: [Brand, CommonModule, ReactiveFormsModule, PasswordModule, ButtonModule, InputTextModule, CheckboxModule, InputOtpModule, RouterLink, TranslocoPipe],
    templateUrl: './login.html',
    styleUrl: './login.css',
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class Login {
    private static readonly PASSKEY_DEVICE_HISTORY_KEY = 'hitkeep.passkey.used_on_device';
    private router = inject(Router);
    private auth = inject(AuthService);
    private preferences = inject(UserPreferencesService);
    private conditionalPasskeyAbortController: AbortController | null = null;
    private hasAttemptedConditionalPasskey = false;
    protected isLoading = signal(false);
    protected isPasskeyLoading = signal(false);
    protected errorMessage = signal<string | null>(null);
    protected currentYear = new Date().getFullYear();
    protected readonly isPasskeySupported = signal(typeof window !== 'undefined' && typeof navigator !== 'undefined' && Boolean(window.PublicKeyCredential) && Boolean(navigator.credentials));
    protected readonly mfaChallengeToken = signal<string | null>(null);
    protected readonly mfaFactors = signal<('totp' | 'passkey')[]>([]);
    protected readonly mfaPasskeyOptions = signal<PasskeyLoginStartResponse['publicKey'] | null>(null);
    protected readonly isMfaRequired = computed(() => this.mfaChallengeToken() !== null);
    protected readonly mfaHasTotp = computed(() => this.mfaFactors().includes('totp'));
    protected readonly mfaHasPasskey = computed(() => this.mfaFactors().includes('passkey'));

    private readonly loginModel = signal({
        email: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.email] }),
        password: new FormControl('', { nonNullable: true, validators: [Validators.required] }),
        rememberMe: new FormControl(false, { nonNullable: true }),
        mfaCode: new FormControl('', { nonNullable: true, validators: [Validators.required, Validators.pattern(/^[0-9]{6}$/)] })
    });
    protected readonly loginForm = compatForm(this.loginModel);

    constructor() {
        if (this.shouldAttemptConditionalPasskey()) {
            void this.startConditionalPasskeyLogin();
        }
    }

    onSubmit(event?: Event): void {
        event?.preventDefault();
        if (this.isMfaRequired()) {
            if (this.mfaHasTotp()) {
                this.verifyTotpMfa();
                return;
            }
            if (this.mfaHasPasskey()) {
                void this.onPasskeyLogin();
                return;
            }
            this.errorMessage.set('login.errors.unexpected');
            return;
        }
        if (this.loginForm.email().invalid() || this.loginForm.password().invalid()) {
            this.loginForm.email().markAsTouched();
            this.loginForm.password().markAsTouched();
            return;
        }
        this.startPasswordLogin();
    }

    private startPasswordLogin(): void {
        this.abortConditionalPasskeyPrompt();
        this.isLoading.set(true);
        this.errorMessage.set(null);

        const email = this.loginForm.email().value();
        const password = this.loginForm.password().value();
        const rememberMe = this.loginForm.rememberMe().value();

        this.auth
            .login({ email, password, remember_me: rememberMe })
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (resp) => {
                    if (resp.status === 'mfa_required') {
                        this.enterMfaState(resp);
                        return;
                    }
                    this.clearMfaState();
                    this.redirectAfterLogin();
                },
                error: (err) => {
                    console.error('Login failed:', err);
                    if (err.status === 401) {
                        this.errorMessage.set('login.errors.invalidCredentials');
                    } else {
                        this.errorMessage.set('login.errors.unexpected');
                    }
                }
            });
    }

    protected async onPasskeyLogin(): Promise<void> {
        if (!this.isPasskeySupported()) {
            this.errorMessage.set('login.errors.passkeyNotSupported');
            return;
        }

        this.abortConditionalPasskeyPrompt();
        this.isPasskeyLoading.set(true);
        this.errorMessage.set(null);

        try {
            const isMfaPasskey = this.isMfaRequired() && this.mfaFactors().includes('passkey');
            let challengeToken = this.mfaChallengeToken() ?? '';
            let options: PasskeyLoginStartResponse['publicKey'] | null = this.mfaPasskeyOptions();

            if (!isMfaPasskey) {
                const start = await firstValueFrom(this.auth.startPasskeyLogin());
                challengeToken = start.challenge_token;
                options = start.publicKey;
            }
            if (!challengeToken || !options) {
                this.errorMessage.set('login.errors.passkeyFailed');
                return;
            }

            const credential = await navigator.credentials.get({
                publicKey: this.toPasskeyRequestOptions(options)
            });

            if (!(credential instanceof PublicKeyCredential) || !(credential.response instanceof AuthenticatorAssertionResponse)) {
                this.errorMessage.set('login.errors.passkeyFailed');
                return;
            }

            const payload = this.toPasskeyFinishPayload(credential, challengeToken, this.loginForm.rememberMe().value());
            await firstValueFrom(this.auth.finishPasskeyLogin(payload));
            this.markPasskeyUsedOnDevice();
            this.clearMfaState();
            this.redirectAfterLogin();
        } catch (err) {
            if ((err as { status?: number })?.status === 401 || (err as { status?: number })?.status === 403) {
                this.errorMessage.set('login.errors.passkeyFailed');
            } else {
                this.errorMessage.set('login.errors.passkeyFailed');
            }
        } finally {
            this.isPasskeyLoading.set(false);
        }
    }

    protected clearMfaState(): void {
        this.mfaChallengeToken.set(null);
        this.mfaFactors.set([]);
        this.mfaPasskeyOptions.set(null);
        this.loginForm.mfaCode().control().reset('');
    }

    private redirectAfterLogin() {
        this.preferences.load().subscribe({
            next: () => {
                this.router.navigate(['/dashboard']);
            },
            error: () => {
                this.router.navigate(['/dashboard']);
            }
        });
    }

    private enterMfaState(resp: LoginResponse): void {
        if (!resp.challenge_token) {
            this.errorMessage.set('login.errors.unexpected');
            return;
        }
        const factors: ('totp' | 'passkey')[] = resp.factors && resp.factors.length > 0 ? resp.factors : resp.passkey ? ['passkey'] : ['totp'];
        this.mfaChallengeToken.set(resp.challenge_token);
        this.mfaFactors.set(factors);
        this.mfaPasskeyOptions.set(resp.passkey ?? null);
        this.loginForm.mfaCode().control().reset('');
        this.errorMessage.set(null);
    }

    private verifyTotpMfa(): void {
        this.abortConditionalPasskeyPrompt();
        const challengeToken = this.mfaChallengeToken();
        if (!challengeToken) {
            this.errorMessage.set('login.errors.unexpected');
            return;
        }
        if (this.loginForm.mfaCode().invalid()) {
            this.loginForm.mfaCode().markAsTouched();
            return;
        }

        this.isLoading.set(true);
        this.errorMessage.set(null);
        this.auth
            .verifyMfaTotp(challengeToken, this.loginForm.mfaCode().value())
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: () => {
                    this.clearMfaState();
                    this.redirectAfterLogin();
                },
                error: (err) => {
                    if (err.status === 401 || err.status === 403) {
                        this.errorMessage.set('login.errors.invalidTotp');
                    } else {
                        this.errorMessage.set('login.errors.unexpected');
                    }
                }
            });
    }

    private async startConditionalPasskeyLogin(): Promise<void> {
        this.hasAttemptedConditionalPasskey = true;
        this.conditionalPasskeyAbortController = new AbortController();
        const abortSignal = this.conditionalPasskeyAbortController.signal;

        try {
            const start = await firstValueFrom(this.auth.startPasskeyLogin());
            if (abortSignal.aborted) {
                return;
            }

            const request: CredentialRequestOptions = {
                publicKey: this.toPasskeyRequestOptions(start.publicKey),
                mediation: 'conditional' as CredentialMediationRequirement,
                signal: abortSignal
            };
            const credential = await navigator.credentials.get(request);
            if (abortSignal.aborted || !credential) {
                return;
            }

            if (!(credential instanceof PublicKeyCredential) || !(credential.response instanceof AuthenticatorAssertionResponse)) {
                return;
            }

            const payload = this.toPasskeyFinishPayload(credential, start.challenge_token, this.loginForm.rememberMe().value());
            await firstValueFrom(this.auth.finishPasskeyLogin(payload));
            this.markPasskeyUsedOnDevice();
            this.clearMfaState();
            this.redirectAfterLogin();
        } catch (err) {
            if (this.isAbortError(err) || this.isNotAllowedError(err)) {
                return;
            }
            // Keep conditional mediation silent on unexpected failures.
            console.warn('Conditional passkey mediation failed', err);
        } finally {
            this.conditionalPasskeyAbortController = null;
        }
    }

    private shouldAttemptConditionalPasskey(): boolean {
        if (this.hasAttemptedConditionalPasskey) {
            return false;
        }
        if (!this.isPasskeySupported()) {
            return false;
        }
        return this.hasUsedPasskeyOnDevice();
    }

    private hasUsedPasskeyOnDevice(): boolean {
        if (typeof window === 'undefined') {
            return false;
        }
        try {
            return window.localStorage.getItem(Login.PASSKEY_DEVICE_HISTORY_KEY) === '1';
        } catch {
            return false;
        }
    }

    private markPasskeyUsedOnDevice(): void {
        if (typeof window === 'undefined') {
            return;
        }
        try {
            window.localStorage.setItem(Login.PASSKEY_DEVICE_HISTORY_KEY, '1');
        } catch {
            // best-effort only
        }
    }

    private abortConditionalPasskeyPrompt(): void {
        if (this.conditionalPasskeyAbortController) {
            this.conditionalPasskeyAbortController.abort();
            this.conditionalPasskeyAbortController = null;
        }
    }

    private isAbortError(err: unknown): boolean {
        return err instanceof DOMException && err.name === 'AbortError';
    }

    private isNotAllowedError(err: unknown): boolean {
        return err instanceof DOMException && err.name === 'NotAllowedError';
    }

    private toPasskeyRequestOptions(options: PasskeyLoginStartResponse['publicKey']): PublicKeyCredentialRequestOptions {
        return {
            challenge: this.base64UrlToArrayBuffer(options.challenge),
            rpId: options.rpId,
            timeout: options.timeout,
            userVerification: options.userVerification
        };
    }

    private toPasskeyFinishPayload(credential: PublicKeyCredential, challengeToken: string, rememberMe: boolean): PasskeyLoginFinishRequest {
        const response = credential.response as AuthenticatorAssertionResponse;
        return {
            challenge_token: challengeToken,
            credential_id: credential.id,
            client_data_json: this.arrayBufferToBase64Url(response.clientDataJSON),
            authenticator_data: this.arrayBufferToBase64Url(response.authenticatorData),
            signature: this.arrayBufferToBase64Url(response.signature),
            remember_me: rememberMe
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
