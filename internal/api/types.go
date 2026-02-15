package api

import (
	"time"

	"github.com/google/uuid"
)

type Hit struct {
	ID             uuid.UUID `json:"id"`
	SiteID         uuid.UUID `json:"site_id"`
	SessionID      uuid.UUID `json:"session_id"`
	PageID         uuid.UUID `json:"page_id"`
	Timestamp      time.Time `json:"timestamp"`
	Path           string    `json:"path"`
	Referrer       *string   `json:"referrer"`
	UserAgent      *string   `json:"user_agent"`
	ViewportWidth  *int      `json:"viewport_width"`
	ViewportHeight *int      `json:"viewport_height"`
	ScreenWidth    *int      `json:"screen_width"`
	ScreenHeight   *int      `json:"screen_height"`
	Language       *string   `json:"language"`
	CountryCode    *string   `json:"country_code"`
	UTMSource      *string   `json:"utm_source"`
	UTMMedium      *string   `json:"utm_medium"`
	UTMCampaign    *string   `json:"utm_campaign"`
	UTMTerm        *string   `json:"utm_term"`
	UTMContent     *string   `json:"utm_content"`
	IsUnique       *bool     `json:"is_unique"`
}

type Site struct {
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id"`
	Domain            string    `json:"domain"`
	DataRetentionDays int       `json:"data_retention_days"`
	CreatedAt         time.Time `json:"created_at"`
}

type ShareLink struct {
	ID        uuid.UUID `json:"id"`
	SiteID    uuid.UUID `json:"site_id"`
	TokenHint string    `json:"token_hint"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	ID         uuid.UUID      `json:"id"`
	SiteID     uuid.UUID      `json:"site_id"`
	SessionID  uuid.UUID      `json:"session_id"`
	Name       string         `json:"name"`
	Properties map[string]any `json:"properties"`
	Timestamp  time.Time      `json:"timestamp"`
}

type Goal struct {
	ID        uuid.UUID `json:"id"`
	SiteID    uuid.UUID `json:"site_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "event" or "path"
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

type FunnelStep struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Funnel struct {
	ID        uuid.UUID    `json:"id"`
	SiteID    uuid.UUID    `json:"site_id"`
	Name      string       `json:"name"`
	Steps     []FunnelStep `json:"steps"`
	CreatedAt time.Time    `json:"created_at"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type UserProfile struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
}

type UserPreferences struct {
	DefaultLocale string `json:"default_locale"`
}

type AnalyticsParams struct {
	SiteID    uuid.UUID
	UserID    uuid.UUID
	Start     time.Time
	End       time.Time
	Filters   []Filter
	GoalIDs   []uuid.UUID
	FunnelIDs []uuid.UUID
}

type ChartDataPoint struct {
	Time      time.Time `json:"time"`
	Pageviews int       `json:"pageviews"`
	Visitors  int       `json:"visitors"`
}

type GoalSeriesPoint struct {
	Time        time.Time `json:"time"`
	Conversions int       `json:"conversions"`
}

type FunnelSeriesPoint struct {
	Time        time.Time `json:"time"`
	Entries     int       `json:"entries"`
	Completions int       `json:"completions"`
}

type MetricStat struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

type SiteStats struct {
	LiveVisitors int `json:"live_visitors"`

	TotalPageviews     int              `json:"total_pageviews"`
	UniqueSessions     int              `json:"unique_sessions"`
	BounceRate         float64          `json:"bounce_rate"`
	AvgSessionDuration float64          `json:"avg_session_duration"`
	PagesPerSession    float64          `json:"pages_per_session"`
	ChartData          []ChartDataPoint `json:"chart_data"`
	TopPages           []MetricStat     `json:"top_pages"`
	TopReferrers       []MetricStat     `json:"top_referrers"`
	TopDevices         []MetricStat     `json:"top_devices"`
	TopCountries       []MetricStat     `json:"top_countries"`
	TopUTMCampaigns    []MetricStat     `json:"top_utm_campaigns"`
	TopUTMContents     []MetricStat     `json:"top_utm_contents"`
	TopUTMMediums      []MetricStat     `json:"top_utm_mediums"`
	TopUTMSources      []MetricStat     `json:"top_utm_sources"`
	TopUTMTerms        []MetricStat     `json:"top_utm_terms"`
	UTMCampaignHits    int              `json:"utm_campaign_hits"`
	UTMContentHits     int              `json:"utm_content_hits"`
	UTMMediumHits      int              `json:"utm_medium_hits"`
	UTMSourceHits      int              `json:"utm_source_hits"`
	UTMTermHits        int              `json:"utm_term_hits"`
	Goals              []GoalStats      `json:"goals"`
}

type GoalStats struct {
	GoalID         uuid.UUID `json:"goal_id"`
	Name           string    `json:"name"`
	Conversions    int       `json:"conversions"`
	ConversionRate float64   `json:"conversion_rate"`
}

type FunnelStepStats struct {
	StepIndex      int     `json:"step_index"`
	Name           string  `json:"name"`
	Visitors       int     `json:"visitors"`
	Dropoff        int     `json:"dropoff"`
	ConversionRate float64 `json:"conversion_rate"`
}

type FunnelStats struct {
	FunnelID              uuid.UUID         `json:"funnel_id"`
	Name                  string            `json:"name"`
	Steps                 []FunnelStepStats `json:"steps"`
	TotalEntries          int               `json:"total_entries"`
	TotalCompletions      int               `json:"total_completions"`
	OverallConversionRate float64           `json:"overall_conversion_rate"`
}

type HitQueryParams struct {
	SiteID    uuid.UUID
	UserID    uuid.UUID
	Start     time.Time
	End       time.Time
	Query     string
	SortField string
	SortOrder string
	Limit     int
	Offset    int
	Filters   []Filter
}

type Filter struct {
	Type  string
	Value string
}

type PaginatedHits struct {
	Data  []Hit `json:"data"`
	Total int   `json:"total"`
}

type SiteMember struct {
	ID      uuid.UUID `json:"id"`
	UserID  uuid.UUID `json:"user_id"`
	Email   string    `json:"email"`
	Role    string    `json:"role"`
	AddedAt time.Time `json:"added_at"`
}
