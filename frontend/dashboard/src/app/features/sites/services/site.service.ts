import { Injectable, inject, signal } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { tap } from 'rxjs';
import { Site } from '../../../core/models/analytics.types';

const LAST_SITE_KEY = 'hk_last_site_id';

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
        
        if (data.length > 0 && !this.activeSite()) {
          const lastId = localStorage.getItem(LAST_SITE_KEY);
          
          const matchedSite = lastId ? data.find(s => s.id === lastId) : null;
          
          this.activeSite.set(matchedSite || data[0]);
        }
        
        this.isLoading.set(false);
      },
      error: () => this.isLoading.set(false)
    });
  }

  selectSite(site: Site) {
    this.activeSite.set(site);
    localStorage.setItem(LAST_SITE_KEY, site.id);
  }

  createSite(domain: string) {
    return this.http.post<Site>('/api/sites', { domain }).pipe(
      tap((newSite) => {
        this.sites.update(list => [newSite, ...list]);
        this.selectSite(newSite);
      })
    );
  }
}