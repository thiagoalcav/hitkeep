package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"hitkeep/internal/api"
)

// GetSiteStats returns aggregated KPIs and time-series data using the AnalyticsParams struct.
func (s *Store) GetSiteStats(ctx context.Context, params api.AnalyticsParams) (*api.SiteStats, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM sites WHERE id = ? AND user_id = ?", params.SiteID, params.UserID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("site not found or access denied")
	} else if err != nil {
		return nil, err
	}

	stats := &api.SiteStats{
		ChartData: []api.ChartDataPoint{},
	}

	duration := params.End.Sub(params.Start)
	interval := "1 DAY"
	truncUnit := "day"

	var gridStart, gridEnd time.Time

	if duration < 48*time.Hour {
		interval = "1 HOUR"
		truncUnit = "hour"
		gridStart = params.Start.Truncate(time.Hour)
		gridEnd = params.End.Truncate(time.Hour)
		// Add one buffer hour to ensure we cover th last partial hour
		if !gridEnd.After(params.End) {
			gridEnd = gridEnd.Add(time.Hour)
		}
	} else {
		y, m, d := params.Start.Date()
		gridStart = time.Date(y, m, d, 0, 0, 0, 0, params.Start.Location())

		y, m, d = params.End.Date()
		gridEnd = time.Date(y, m, d, 0, 0, 0, 0, params.End.Location())
		if !gridEnd.After(params.End) {
			gridEnd = gridEnd.AddDate(0, 0, 1)
		}
	}

	kpiQuery := `
	WITH session_metrics AS (
		SELECT 
			session_id,
			count(*) as pvs,
			(MAX(timestamp) - MIN(timestamp)) as duration
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY session_id
	)
	SELECT 
		COALESCE(SUM(pvs), 0) as total_pageviews,
		COUNT(session_id) as unique_sessions,
		CASE 
			WHEN COUNT(session_id) = 0 THEN 0 
			ELSE CAST(COUNT(CASE WHEN pvs = 1 THEN 1 END) AS FLOAT) / COUNT(session_id) * 100 
		END as bounce_rate,
		COALESCE(AVG(EXTRACT('epoch' FROM duration)), 0) as avg_duration_seconds,
		COALESCE(AVG(pvs), 0) as pages_per_session
	FROM session_metrics;
	`

	err = s.db.QueryRowContext(ctx, kpiQuery, params.SiteID, params.Start, params.End).Scan(
		&stats.TotalPageviews,
		&stats.UniqueSessions,
		&stats.BounceRate,
		&stats.AvgSessionDuration,
		&stats.PagesPerSession,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calc KPIs: %w", err)
	}

	//nolint:gosec // not user input
	chartQuery := fmt.Sprintf(`
	WITH time_range AS (
		SELECT unnest(generate_series(?::TIMESTAMP, ?::TIMESTAMP, INTERVAL %s)) as bucket
	),
	daily_hits AS (
		SELECT 
			date_trunc('%s', timestamp)::TIMESTAMP as bucket,
			COUNT(*) as pageviews,
			COUNT(DISTINCT session_id) as visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY bucket
	)
	SELECT 
		tr.bucket,
		COALESCE(dh.pageviews, 0),
		COALESCE(dh.visitors, 0)
	FROM time_range tr
	LEFT JOIN daily_hits dh ON tr.bucket = dh.bucket
	ORDER BY tr.bucket ASC;
	`, interval, truncUnit)

	rows, err := s.db.QueryContext(ctx, chartQuery, gridStart, gridEnd, params.SiteID, params.Start, params.End)
	if err != nil {
		return nil, fmt.Errorf("failed to query chart data: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p api.ChartDataPoint
		if err := rows.Scan(&p.Time, &p.Pageviews, &p.Visitors); err != nil {
			return nil, err
		}
		stats.ChartData = append(stats.ChartData, p)
	}

	return stats, nil
}
