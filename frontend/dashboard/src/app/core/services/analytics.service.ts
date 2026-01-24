import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { GoalSeriesPoint, FunnelSeriesPoint } from '../models/analytics.types';

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
  top_pages: MetricStat[];
  top_referrers: MetricStat[];
  top_devices: MetricStat[];
  top_countries: MetricStat[];
  goals: GoalStats[];
}

export interface MetricStat {
  name: string;
  value: number;
}

export interface Goal {
  id: string;
  site_id: string;
  name: string;
  type: 'event' | 'path';
  value: string;
  created_at: string;
}

export interface GoalStats {
  goal_id: string;
  name: string;
  conversions: number;
  conversion_rate: number;
}

export interface FunnelStep {
  type: 'event' | 'path';
  value: string;
}

export interface Funnel {
  id: string;
  site_id: string;
  name: string;
  steps: FunnelStep[];
  created_at: string;
}

export interface FunnelStepStats {
  step_index: number;
  name: string;
  visitors: number;
  dropoff: number;
  conversion_rate: number;
}

export interface FunnelStats {
  funnel_id: string;
  name: string;
  steps: FunnelStepStats[];
  total_entries: number;
  total_completions: number;
  overall_conversion_rate: number;
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

  updateSiteRetention(siteId: string, days: number): Observable<void> {
    return this.http.put<void>(`/api/sites/${siteId}/retention`, { days });
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

  // Goals
  getGoals(siteId: string): Observable<Goal[]> {
    return this.http.get<Goal[]>(`/api/sites/${siteId}/goals`);
  }

  getGoalTimeseries(siteId: string, from?: string, to?: string, goalIds: string[] = []): Observable<GoalSeriesPoint[]> {
    let params = new HttpParams();
    if (from) params = params.set('from', from);
    if (to) params = params.set('to', to);
    for (const id of goalIds) {
      params = params.append('goal_id', id);
    }
    return this.http.get<GoalSeriesPoint[]>(`/api/sites/${siteId}/goals/timeseries`, { params });
  }

  createGoal(siteId: string, goal: Partial<Goal>): Observable<void> {
    return this.http.post<void>(`/api/sites/${siteId}/goals`, goal);
  }

  deleteGoal(siteId: string, goalId: string): Observable<void> {
    return this.http.delete<void>(`/api/sites/${siteId}/goals/${goalId}`);
  }

  // Funnels
  getFunnels(siteId: string): Observable<Funnel[]> {
    return this.http.get<Funnel[]>(`/api/sites/${siteId}/funnels`);
  }

  getFunnelTimeseries(siteId: string, from?: string, to?: string, funnelIds: string[] = []): Observable<FunnelSeriesPoint[]> {
    let params = new HttpParams();
    if (from) params = params.set('from', from);
    if (to) params = params.set('to', to);
    for (const id of funnelIds) {
      params = params.append('funnel_id', id);
    }
    return this.http.get<FunnelSeriesPoint[]>(`/api/sites/${siteId}/funnels/timeseries`, { params });
  }

  createFunnel(siteId: string, funnel: Partial<Funnel>): Observable<void> {
    return this.http.post<void>(`/api/sites/${siteId}/funnels`, funnel);
  }

  deleteFunnel(siteId: string, funnelId: string): Observable<void> {
    return this.http.delete<void>(`/api/sites/${siteId}/funnels/${funnelId}`);
  }

  getFunnelStats(siteId: string, funnelId: string, from?: string, to?: string): Observable<FunnelStats> {
    let params = new HttpParams();
    if (from) params = params.set('from', from);
    if (to) params = params.set('to', to);
    return this.http.get<FunnelStats>(`/api/sites/${siteId}/funnels/${funnelId}/stats`, { params });
  }
}
