import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

export type InstanceRole = 'owner' | 'admin' | 'user';
export type SiteRole = 'owner' | 'admin' | 'editor' | 'viewer';

export interface APIClientSiteRole {
    site_id: string;
    role: SiteRole;
}

export interface APIClient {
    id: string;
    user_id: string;
    name: string;
    description: string;
    instance_role: InstanceRole;
    expires_at?: string | null;
    last_used_at?: string | null;
    revoked_at?: string | null;
    created_at: string;
    updated_at: string;
    site_roles: APIClientSiteRole[];
}

export interface CreateAPIClientRequest {
    name: string;
    description: string;
    instance_role: InstanceRole;
    expires_at?: string | null;
    site_roles: APIClientSiteRole[];
}

export interface UpdateAPIClientRequest {
    name: string;
    description: string;
    instance_role: InstanceRole;
    expires_at?: string | null;
    revoked?: boolean;
    site_roles: APIClientSiteRole[];
}

export interface CreateAPIClientResponse {
    client: APIClient;
    token: string;
}

export interface SiteSummary {
    id: string;
    domain: string;
}

@Injectable({ providedIn: 'root' })
export class APIClientsService {
    private http = inject(HttpClient);

    listClients(): Observable<APIClient[]> {
        return this.http.get<APIClient[]>('/api/user/api-clients');
    }

    createClient(payload: CreateAPIClientRequest): Observable<CreateAPIClientResponse> {
        return this.http.post<CreateAPIClientResponse>('/api/user/api-clients', payload);
    }

    updateClient(clientID: string, payload: UpdateAPIClientRequest): Observable<APIClient> {
        return this.http.put<APIClient>(`/api/user/api-clients/${clientID}`, payload);
    }

    deleteClient(clientID: string): Observable<void> {
        return this.http.delete<void>(`/api/user/api-clients/${clientID}`);
    }

    listSites(): Observable<SiteSummary[]> {
        return this.http.get<SiteSummary[]>('/api/sites');
    }
}
