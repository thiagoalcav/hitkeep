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

export interface PaginatedHits {
  data: Hit[];
  total: number;
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
  avg_session_duration: number; // Seconds
  pages_per_session: number;
  chart_data: ChartDataPoint[];
}

export interface SystemStatus {
  needs_setup: boolean;
  version: string;
}

@Injectable({
  providedIn: 'root'
})
export class AnalyticsService {
  private http = inject(HttpClient);

  getSystemStatus(): Observable<SystemStatus> {
    return this.http.get<SystemStatus>('/api/status');
  }

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
  getSiteStats(siteId: string, from?: string, to?: string): Observable<SiteStats> {
    let params = new HttpParams();
    if (from) params = params.set('from', from);
    if (to) params = params.set('to', to);

    return this.http.get<SiteStats>(`/api/sites/${siteId}/stats`, { params });
  }

  /**
   * Fetches raw hits (RESTful nested resource).
   * GET /api/sites/{id}/hits
   */
  getHits(
    siteId: string, 
    from: string, 
    to: string,
    page = 1, 
    pageSize = 10,
    sortField?: string,
    sortOrder?: string,
    query?: string
  ): Observable<PaginatedHits> {
    
    let params = new HttpParams()
      .set('from', from)
      .set('to', to)
      .set('limit', pageSize)
      .set('offset', (page - 1) * pageSize);

    if (sortField) params = params.set('sort', sortField);
    if (sortOrder) params = params.set('order', sortOrder);
    if (query) params = params.set('q', query);

    return this.http.get<PaginatedHits>(`/api/sites/${siteId}/hits`, { params });
  }
}