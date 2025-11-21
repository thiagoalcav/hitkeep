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
	// 1. Validation & Ownership Check
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

	// 2. Determine Interval for Chart (Dynamic Bucketing)
	// If range < 48 hours, use hour buckets. Otherwise, use day buckets.
	duration := params.End.Sub(params.Start)
	interval := "1 DAY"
	if duration < 48*time.Hour {
		interval = "1 HOUR"
	}

	// 3. Complex CTE for KPIs (Totals, Bounce Rate, Duration, Pages/Session)
	// We use DuckDB's power to calculate session-level metrics on the fly.
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

	// 4. Time Series Data (Gap Filled)
	// We inject the determined 'interval' directly into the generate_series and date_trunc functions.
	// Note: DuckDB parameter substitution (?) works for values, not keywords like INTERVAL '1 DAY'.
	// So we use Sprintf for the interval string, which is safe because we control the variable 'interval' internally above.
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
	`, interval, (func() string {
		if interval == "1 HOUR" {
			return "hour"
		} else {
			return "day"
		}
	})())

	// Normalize start/end for the generate_series bounds
	// For charts, we usually want to align to the bucket start
	startBucket := params.Start
	endBucket := params.End

	rows, err := s.db.QueryContext(ctx, chartQuery, startBucket, endBucket, params.SiteID, params.Start, params.End)
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
