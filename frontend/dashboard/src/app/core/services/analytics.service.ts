import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';

export interface Site {
  id: string;
  user_id: string;
  domain: string;
  created_at: string;
}

export interface Hit {
  id: string;
  site_id: string;
  session_id: string;
  page_id: string;
  timestamp: string;
  path: string;
  referrer?: string;
  user_agent?: string;
  viewport_width?: number;
  viewport_height?: number;
  language?: string;
  is_unique?: boolean;
}

export interface ChartDataPoint {
  time: string;
  pageviews: number;
  visitors: number;
}

export interface SiteStats {
  total_pageviews: number;
  unique_sessions: number;
  bounce_rate: number;
  avg_session_duration: number; // New field (Seconds)
  pages_per_session: number;    // New field
  chart_data: ChartDataPoint[];
}

@Injectable({
  providedIn: 'root'
})
export class AnalyticsService {
  private http = inject(HttpClient);

  /**
   * Fetches the list of websites tracked by the user.
   */
  getSites(): Observable<Site[]> {
    return this.http.get<Site[]>('/api/sites');
  }

  /**
   * Creates a new website for the user.
   */
  createSite(domain: string): Observable<Site> {
    return this.http.post<Site>('/api/sites', { domain });
  }

  /**
   * Fetches aggregated statistics and chart data.
   * Supports optional date range filtering.
   */
  getSiteStats(siteId: string, from?: string, to?: string): Observable<SiteStats> {
    let params = new HttpParams();
    if (from) params = params.set('from', from);
    if (to) params = params.set('to', to);

    console.debug(`Fetching stats for site ID: ${siteId}`, { from, to });
    return this.http.get<SiteStats>(`/api/sites/${siteId}/stats`, { params });
  }

  /**
   * Fetches hits for a specific site.
   */
  getHits(siteId: string): Observable<Hit[]> {
    return this.http.get<Hit[]>(`/api/hits`, {
      params: { site_id: siteId }
    });
  }
}