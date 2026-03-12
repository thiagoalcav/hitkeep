import { Injectable, inject } from "@angular/core";
import { HttpClient } from "@angular/common/http";
import { Observable } from "rxjs";

export interface CloudSignupRequest {
    email: string;
    password: string;
    team_name: string;
    plan_code: "free" | "pro" | "business";
    jurisdiction?: string;
    locale?: string;
    given_name?: string;
    last_name?: string;
}

export interface CloudSignupResponse {
    status: string;
    plan_code: string;
    redirect_url?: string;
    checkout_url?: string;
}

export interface BillingPortalSessionResponse {
    url: string;
}

export interface BillingPortalSessionRequest {
    locale?: string;
}

export interface BillingCheckoutSessionRequest {
    plan_code: "pro" | "business";
    locale?: string;
}

@Injectable({ providedIn: "root" })
export class CloudService {
    private readonly http = inject(HttpClient);

    signup(payload: CloudSignupRequest): Observable<CloudSignupResponse> {
        return this.http.post<CloudSignupResponse>("/api/cloud/signup", payload);
    }

    createBillingPortalSession(payload: BillingPortalSessionRequest = {}): Observable<BillingPortalSessionResponse> {
        return this.http.post<BillingPortalSessionResponse>("/api/cloud/billing/portal", payload);
    }

    createBillingCheckoutSession(payload: BillingCheckoutSessionRequest): Observable<BillingPortalSessionResponse> {
        return this.http.post<BillingPortalSessionResponse>("/api/cloud/billing/checkout", payload);
    }
}
