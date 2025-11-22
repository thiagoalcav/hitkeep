import { Injectable, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { tap } from 'rxjs';
import { Site } from '../../../core/models/analytics.types';

@Injectable({ providedIn: 'root' })
export class SiteService {
  private http = inject(HttpClient);

  // Global State for Sites
  readonly sites = signal<Site[]>([]);
  readonly activeSite = signal<Site | null>(null);
  readonly isLoading = signal<boolean>(false);

  loadSites() {
    this.isLoading.set(true);
    this.http.get<Site[]>('/api/sites').subscribe({
      next: (data) => {
        this.sites.set(data);
        // Auto-select first site if none active
        if (data.length > 0 && !this.activeSite()) {
          this.activeSite.set(data[0]);
        }
        this.isLoading.set(false);
      },
      error: () => this.isLoading.set(false)
    });
  }

  selectSite(site: Site) {
    this.activeSite.set(site);
  }

  createSite(domain: string) {
    return this.http.post<Site>('/api/sites', { domain }).pipe(
      tap((newSite) => {
        this.sites.update(list => [newSite, ...list]);
        this.activeSite.set(newSite);
      })
    );
  }
}
