export interface Site {
  id: string;
  user_id: string;
  domain: string;
  created_at: string;
  data_retention_days?: number;
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

export interface GoalSeriesPoint {
  time: string;
  conversions: number;
}

export interface FunnelSeriesPoint {
  time: string;
  entries: number;
  completions: number;
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
  top_countries: MetricStat[];
  goals: GoalStats[];
  funnels: Funnel[];
}

export interface GoalStats {
  goal_id: string;
  name: string;
  conversions: number;
  conversion_rate: number;
}

export interface Goal {
  id: string;
  site_id: string;
  name: string;
  type: 'event' | 'path';
  value: string;
  created_at: string;
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
