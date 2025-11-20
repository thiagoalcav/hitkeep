import { Injectable, inject } from '@angular/core';
import { HttpClient } from '@angular/common/http';
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
   */
  getSiteStats(siteId: string): Observable<SiteStats> {
    console.debug(`Fetching stats for site ID: ${siteId}`);
    return this.http.get<SiteStats>(`/api/sites/${siteId}/stats`);
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