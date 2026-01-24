import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { finalize } from 'rxjs';
import { SiteStats } from '../../../core/models/analytics.types';

@Injectable({ providedIn: 'root' })
export class StatsService {
  private http = inject(HttpClient);

  readonly stats = signal<SiteStats | null>(null);
  readonly isLoading = signal<boolean>(false);

  loadStats(
    siteId: string,
    from: string,
    to: string,
    filters: { type: string; value: string }[] = [],
    goalIds: string[] = [],
    funnelIds: string[] = []
  ) {
    this.isLoading.set(true);

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

    this.http.get<SiteStats>(`/api/sites/${siteId}/stats`, { params })
      .pipe(finalize(() => this.isLoading.set(false)))
      .subscribe({
        next: (data) => this.stats.set(data),
        error: (e) => console.error(e) // Handle errors globally in interceptor usually
      });
  }
}
