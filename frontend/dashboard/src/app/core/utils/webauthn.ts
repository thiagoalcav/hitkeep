export interface CredentialDescriptorJson {
    type: PublicKeyCredentialType;
    id: string;
    transports?: AuthenticatorTransport[];
}

export interface PublicKeyCredentialRequestOptionsJson {
    challenge: string;
    rpId?: string;
    timeout?: number;
    userVerification?: UserVerificationRequirement;
    allowCredentials?: CredentialDescriptorJson[];
}

export interface PublicKeyCredentialCreationOptionsJson {
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
    timeout?: number;
    attestation?: AttestationConveyancePreference;
    authenticatorSelection?: {
        authenticatorAttachment?: AuthenticatorAttachment;
        residentKey?: ResidentKeyRequirement;
        userVerification?: UserVerificationRequirement;
    };
    excludeCredentials?: CredentialDescriptorJson[];
}

export interface PublicKeyCredentialAssertionJson {
    id: string;
    type: PublicKeyCredentialType;
    rawId: string;
    response: {
        clientDataJSON: string;
        authenticatorData: string;
        signature: string;
        userHandle?: string;
    };
    clientExtensionResults?: AuthenticationExtensionsClientOutputs;
    authenticatorAttachment?: string;
}

export interface PublicKeyCredentialCreationJson {
    id: string;
    type: PublicKeyCredentialType;
    rawId: string;
    response: {
        clientDataJSON: string;
        attestationObject: string;
        transports?: AuthenticatorTransport[];
    };
    clientExtensionResults?: AuthenticationExtensionsClientOutputs;
    authenticatorAttachment?: string;
}

export function toPublicKeyRequestOptions(options: PublicKeyCredentialRequestOptionsJson): PublicKeyCredentialRequestOptions {
    return {
        challenge: base64UrlToArrayBuffer(options.challenge),
        rpId: options.rpId,
        timeout: options.timeout,
        userVerification: options.userVerification,
        allowCredentials: options.allowCredentials?.map((credential) => ({
            ...credential,
            id: base64UrlToArrayBuffer(credential.id)
        }))
    };
}

export function toPublicKeyCreationOptions(options: PublicKeyCredentialCreationOptionsJson): PublicKeyCredentialCreationOptions {
    return {
        challenge: base64UrlToArrayBuffer(options.challenge),
        rp: {
            name: options.rp.name,
            id: options.rp.id
        },
        user: {
            id: base64UrlToArrayBuffer(options.user.id),
            name: options.user.name,
            displayName: options.user.displayName
        },
        pubKeyCredParams: options.pubKeyCredParams,
        timeout: options.timeout,
        attestation: options.attestation,
        authenticatorSelection: options.authenticatorSelection,
        excludeCredentials: options.excludeCredentials?.map((credential) => ({
            ...credential,
            id: base64UrlToArrayBuffer(credential.id)
        }))
    };
}

export function toAssertionResponseJson(credential: PublicKeyCredential): PublicKeyCredentialAssertionJson | null {
    if (!(credential.response instanceof AuthenticatorAssertionResponse)) {
        return null;
    }

    return {
        id: credential.id,
        type: credential.type as PublicKeyCredentialType,
        rawId: arrayBufferToBase64Url(credential.rawId),
        response: {
            clientDataJSON: arrayBufferToBase64Url(credential.response.clientDataJSON),
            authenticatorData: arrayBufferToBase64Url(credential.response.authenticatorData),
            signature: arrayBufferToBase64Url(credential.response.signature),
            userHandle: credential.response.userHandle ? arrayBufferToBase64Url(credential.response.userHandle) : undefined
        },
        clientExtensionResults: credential.getClientExtensionResults(),
        authenticatorAttachment: credential.authenticatorAttachment ?? undefined
    };
}

export function toCreationResponseJson(credential: PublicKeyCredential): PublicKeyCredentialCreationJson | null {
    if (!(credential.response instanceof AuthenticatorAttestationResponse)) {
        return null;
    }

    const transports = credential.response.getTransports?.();

    return {
        id: credential.id,
        type: credential.type as PublicKeyCredentialType,
        rawId: arrayBufferToBase64Url(credential.rawId),
        response: {
            clientDataJSON: arrayBufferToBase64Url(credential.response.clientDataJSON),
            attestationObject: arrayBufferToBase64Url(credential.response.attestationObject),
            transports: transports?.map((transport) => transport as AuthenticatorTransport)
        },
        clientExtensionResults: credential.getClientExtensionResults(),
        authenticatorAttachment: credential.authenticatorAttachment ?? undefined
    };
}

export function base64UrlToArrayBuffer(value: string): ArrayBuffer {
    const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
    const padded = normalized + "=".repeat((4 - (normalized.length % 4)) % 4);
    const binary = atob(padded);
    const out = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) {
        out[index] = binary.charCodeAt(index);
    }
    return out.buffer.slice(0);
}

export function arrayBufferToBase64Url(value: ArrayBuffer): string {
    const bytes = new Uint8Array(value);
    let binary = "";
    for (const byte of bytes) {
        binary += String.fromCharCode(byte);
    }
    return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}
