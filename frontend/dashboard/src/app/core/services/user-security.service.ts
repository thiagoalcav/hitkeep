import { Injectable, inject } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable } from "rxjs";

export interface UserPasskey {
    id: string;
    name: string;
    created_at: string;
    updated_at: string;
}

export interface UserSecurityStatus {
    totp_enabled: boolean;
    totp_pending: boolean;
    passkeys: UserPasskey[];
    recovery_codes_generated: boolean;
    recovery_codes_remaining: number;
}

export interface UserTotpSetup {
    secret: string;
    otpauth_url: string;
    expires_at: string;
}

export interface PasskeyRegistrationStartResponse {
    publicKey: {
        challenge: string;
        rp: {
            name: string;
            id: string;
        };
        user: {
            id: string;
            name: string;
            displayName: string;
        };
        pubKeyCredParams: {
            type: PublicKeyCredentialType;
            alg: number;
        }[];
        timeout: number;
        attestation: AttestationConveyancePreference;
        authenticatorSelection: {
            residentKey: ResidentKeyRequirement;
            userVerification: UserVerificationRequirement;
        };
    };
}

export interface PasskeyRegistrationFinishRequest {
    name?: string;
    credential_id: string;
    client_data_json: string;
    public_key: string;
    transports?: string[];
}

export interface UserRecoveryCodesResponse {
    codes: string[];
    remaining: number;
}

@Injectable({ providedIn: "root" })
export class UserSecurityService {
    private http = inject(HttpClient);

    loadStatus(): Observable<UserSecurityStatus> {
        return this.http.get<UserSecurityStatus>("/api/user/security");
    }

    startTotpSetup(): Observable<UserTotpSetup> {
        return this.http.post<UserTotpSetup>("/api/user/security/totp/setup/start", {});
    }

    verifyTotpSetup(code: string): Observable<UserSecurityStatus> {
        return this.http.post<UserSecurityStatus>("/api/user/security/totp/setup/verify", { code });
    }

    disableTotp(code: string): Observable<UserSecurityStatus> {
        return this.http.post<UserSecurityStatus>("/api/user/security/totp/disable", { code });
    }

    startPasskeyRegistration(name?: string): Observable<PasskeyRegistrationStartResponse> {
        return this.http.post<PasskeyRegistrationStartResponse>("/api/user/security/passkeys/register/start", { name: name?.trim() || "" });
    }

    finishPasskeyRegistration(payload: PasskeyRegistrationFinishRequest): Observable<UserSecurityStatus> {
        return this.http.post<UserSecurityStatus>("/api/user/security/passkeys/register/finish", payload);
    }

    deletePasskey(passkeyID: string): Observable<void> {
        return this.http.delete<void>(`/api/user/security/passkeys/${passkeyID}`);
    }

    regenerateRecoveryCodes(): Observable<UserRecoveryCodesResponse> {
        return this.http.post<UserRecoveryCodesResponse>("/api/user/security/recovery-codes/regenerate", {});
    }
}
