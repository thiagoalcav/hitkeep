import { Injectable, computed, inject, signal } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable, finalize, tap } from "rxjs";

import { PublicKeyCredentialAssertionJson, PublicKeyCredentialRequestOptionsJson } from "@core/utils/webauthn";

export type AuthStatus = "unknown" | "authenticated" | "unauthenticated";

export interface PasskeyLoginStartResponse {
    challenge_token: string;
    publicKey: PublicKeyCredentialRequestOptionsJson;
}

export interface PasskeyLoginFinishRequest {
    challenge_token: string;
    credential: PublicKeyCredentialAssertionJson;
    remember_me?: boolean;
}

export interface LoginResponse {
    status: "ok" | "mfa_required";
    challenge_token?: string;
    factors?: ("totp" | "passkey" | "recovery_code" | "email_link")[];
    passkey?: PasskeyLoginStartResponse["publicKey"];
}

@Injectable({ providedIn: "root" })
export class AuthService {
    private http = inject(HttpClient);
    readonly status = signal<AuthStatus>("unknown");
    readonly isAuthenticated = computed(() => this.status() === "authenticated");

    login(credentials: { email: string; password: string; remember_me?: boolean }): Observable<LoginResponse> {
        return this.http.post<LoginResponse>("/api/login", credentials).pipe(
            tap((resp) => {
                if (resp.status === "ok") {
                    this.status.set("authenticated");
                }
            })
        );
    }

    logout(): Observable<void> {
        return this.http.post<void>("/api/logout", {}).pipe(finalize(() => this.status.set("unauthenticated")));
    }

    requestPasswordReset(email: string): Observable<void> {
        return this.http.post<void>("/api/auth/forgot-password", { email });
    }

    resetPassword(token: string, password: string): Observable<void> {
        return this.http.post<void>("/api/auth/reset-password", { token, password });
    }

    changePassword(current: string, newPass: string): Observable<void> {
        return this.http.post<void>("/api/user/password", {
            current_password: current,
            new_password: newPass
        });
    }

    startPasskeyLogin(): Observable<PasskeyLoginStartResponse> {
        return this.http.post<PasskeyLoginStartResponse>("/api/auth/passkey/login/start", {});
    }

    finishPasskeyLogin(payload: PasskeyLoginFinishRequest): Observable<void> {
        return this.http.post<void>("/api/auth/passkey/login/finish", payload).pipe(tap(() => this.status.set("authenticated")));
    }

    verifyMfaTotp(challengeToken: string, code: string): Observable<void> {
        return this.http
            .post<void>("/api/auth/mfa/totp/verify", {
                challenge_token: challengeToken,
                code
            })
            .pipe(tap(() => this.status.set("authenticated")));
    }

    verifyMfaRecoveryCode(challengeToken: string, code: string): Observable<void> {
        return this.http
            .post<void>("/api/auth/mfa/recovery-code/verify", {
                challenge_token: challengeToken,
                code
            })
            .pipe(tap(() => this.status.set("authenticated")));
    }

    requestMfaEmailLink(challengeToken: string, returnUrl: string): Observable<void> {
        return this.http.post<void>("/api/auth/mfa/email-link/request", {
            challenge_token: challengeToken,
            return_url: returnUrl
        });
    }

    markAuthenticated() {
        this.status.set("authenticated");
    }

    markUnauthenticated() {
        this.status.set("unauthenticated");
    }
}
