import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { finalize } from 'rxjs';
import { SiteStats } from '../../../core/models/analytics.types';

@Injectable({ providedIn: 'root' })
export class StatsService {
  private http = inject(HttpClient);

  readonly stats = signal<SiteStats | null>(null);
  readonly isLoading = signal<boolean>(false);

  loadStats(siteId: string, from: string, to: string) {
    this.isLoading.set(true);

    const params = new HttpParams().set('from', from).set('to', to);

    this.http.get<SiteStats>(`/api/sites/${siteId}/stats`, { params })
      .pipe(finalize(() => this.isLoading.set(false)))
      .subscribe({
        next: (data) => this.stats.set(data),
        error: (e) => console.error(e) // Handle errors globally in interceptor usually
      });
  }
}
