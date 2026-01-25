import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { finalize, Observable } from 'rxjs';
import { SiteStats } from '@models/analytics.types';

@Injectable({ providedIn: 'root' })
export class StatsService {
    private http = inject(HttpClient);

    readonly stats = signal<SiteStats | null>(null);
    readonly isLoading = signal<boolean>(false);

    loadStats(siteId: string, from: string, to: string, filters: { type: string; value: string }[] = [], goalIds: string[] = [], funnelIds: string[] = []) {
        this.isLoading.set(true);
        this.fetchStats(siteId, from, to, filters, goalIds, funnelIds)
            .pipe(finalize(() => this.isLoading.set(false)))
            .subscribe({
                next: (data) => this.stats.set(data),
                error: (e) => console.error(e) // Handle errors globally in interceptor usually
            });
    }

    fetchStats(siteId: string, from: string, to: string, filters: { type: string; value: string }[] = [], goalIds: string[] = [], funnelIds: string[] = []): Observable<SiteStats> {
        let params = new HttpParams().set('from', from).set('to', to);
        for (const filter of filters) {
            params = params.append('filter', `${filter.type}:${filter.value}`);
        }
        for (const id of goalIds) {
            params = params.append('goal_id', id);
        }
        for (const id of funnelIds) {
            params = params.append('funnel_id', id);
        }

        return this.http.get<SiteStats>(`/api/sites/${siteId}/stats`, { params });
    }
}
