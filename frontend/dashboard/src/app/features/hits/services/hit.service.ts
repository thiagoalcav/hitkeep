import { Injectable, inject, signal } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { finalize } from 'rxjs';
import { Hit, PaginatedHits } from '../../../core/models/analytics.types';

@Injectable({ providedIn: 'root' })
export class HitService {
  private http = inject(HttpClient);

  readonly hits = signal<Hit[]>([]);
  readonly total = signal<number>(0);
  readonly isLoading = signal<boolean>(false);

  loadHits(
    siteId: string,
    from: string,
    to: string,
    page = 1,
    pageSize = 10,
    sortField?: string,
    sortOrder?: string,
    query?: string,
    filters: { type: string; value: string }[] = []
  ) {
    this.isLoading.set(true);

    let params = new HttpParams()
      .set('from', from)
      .set('to', to)
      .set('limit', pageSize)
      .set('offset', (page - 1) * pageSize);

    if (sortField) params = params.set('sort', sortField);
    if (sortOrder) params = params.set('order', sortOrder);
    if (query) params = params.set('q', query);
    for (const filter of filters) {
      params = params.append('filter', `${filter.type}:${filter.value}`);
    }

    this.http.get<PaginatedHits>(`/api/sites/${siteId}/hits`, { params })
      .pipe(finalize(() => this.isLoading.set(false)))
      .subscribe({
        next: (res) => {
          this.hits.set(res.data);
          this.total.set(res.total);
        }
      });
  }
}
