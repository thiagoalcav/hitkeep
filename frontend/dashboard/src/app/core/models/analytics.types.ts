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

export interface MetricStat {
  name: string;
  value: number;
}

export interface SiteStats {
  live_visitors: number;
  total_pageviews: number;
  unique_sessions: number;
  bounce_rate: number;
  avg_session_duration: number;
  pages_per_session: number;
  chart_data: ChartDataPoint[];
  top_pages: MetricStat[];
  top_referrers: MetricStat[];
  top_devices: MetricStat[];
}

export interface SystemStatus {
  needs_setup: boolean;
  version: string;
}
