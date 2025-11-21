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
	IsUnique       *bool     `json:"is_unique"`
}

type Site struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type AnalyticsParams struct {
	SiteID uuid.UUID
	UserID uuid.UUID
	Start  time.Time
	End    time.Time
}

type ChartDataPoint struct {
	Time      time.Time `json:"time"`
	Pageviews int       `json:"pageviews"`
	Visitors  int       `json:"visitors"`
}

type SiteStats struct {
	TotalPageviews     int              `json:"total_pageviews"`
	UniqueSessions     int              `json:"unique_sessions"`
	BounceRate         float64          `json:"bounce_rate"`
	AvgSessionDuration float64          `json:"avg_session_duration"` // Seconds
	PagesPerSession    float64          `json:"pages_per_session"`
	ChartData          []ChartDataPoint `json:"chart_data"`
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
}

type PaginatedHits struct {
	Data  []Hit `json:"data"`
	Total int   `json:"total"`
}
