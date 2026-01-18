package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

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
		ChartData:    []api.ChartDataPoint{},
		TopPages:     []api.MetricStat{},
		TopReferrers: []api.MetricStat{},
		TopDevices:   []api.MetricStat{},
		Goals:        []api.GoalStats{},
	}

	filterSQL, filterArgs := buildHitFilter(params.FilterType, params.FilterValue, "h")

	liveThreshold := time.Now().Add(-5 * time.Minute)
	liveQuery := "SELECT COUNT(DISTINCT h.session_id) FROM hits h WHERE h.site_id = ? AND h.timestamp >= ?" + filterSQL
	err = s.db.QueryRowContext(ctx, liveQuery, append([]any{params.SiteID, liveThreshold}, filterArgs...)...).Scan(&stats.LiveVisitors)
	if err != nil {
		return nil, fmt.Errorf("failed to calc live visitors: %w", err)
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

	//nolint:gosec // filterSQL is derived from a fixed allowlist
	kpiQuery := fmt.Sprintf(`
	WITH session_metrics AS (
		SELECT 
			session_id,
			count(*) as pvs,
			(MAX(timestamp) - MIN(timestamp)) as duration
		FROM hits h
		WHERE h.site_id = ? AND h.timestamp >= ? AND h.timestamp <= ?%s
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
	`, filterSQL)

	err = s.db.QueryRowContext(ctx, kpiQuery, append([]any{params.SiteID, params.Start, params.End}, filterArgs...)...).Scan(
		&stats.TotalPageviews,
		&stats.UniqueSessions,
		&stats.BounceRate,
		&stats.AvgSessionDuration,
		&stats.PagesPerSession,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calc KPIs: %w", err)
	}

	//nolint:gosec // interval/truncUnit/filterSQL are derived from fixed allowlists
	chartQuery := fmt.Sprintf(`
	WITH time_range AS (
		SELECT unnest(generate_series(?::TIMESTAMP, ?::TIMESTAMP, INTERVAL %s)) as bucket
	),
	daily_hits AS (
		SELECT 
			date_trunc('%s', timestamp)::TIMESTAMP as bucket,
			COUNT(*) as pageviews,
			COUNT(DISTINCT session_id) as visitors
		FROM hits h
		WHERE h.site_id = ? AND h.timestamp >= ? AND h.timestamp <= ?%s
		GROUP BY bucket
	)
	SELECT 
		tr.bucket,
		COALESCE(dh.pageviews, 0),
		COALESCE(dh.visitors, 0)
	FROM time_range tr
	LEFT JOIN daily_hits dh ON tr.bucket = dh.bucket
	ORDER BY tr.bucket ASC;
	`, interval, truncUnit, filterSQL)

	rows, err := s.db.QueryContext(ctx, chartQuery, append([]any{gridStart, gridEnd, params.SiteID, params.Start, params.End}, filterArgs...)...)
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

	// 3. Top Pages
	//nolint:gosec // filterSQL is derived from a fixed allowlist
	pagesQuery := fmt.Sprintf(`
		SELECT h.path, COUNT(*) as val
		FROM hits h
		WHERE h.site_id = ? AND h.timestamp >= ? AND h.timestamp <= ?%s
		GROUP BY h.path
		ORDER BY val DESC
		LIMIT 10
	`, filterSQL)
	pRows, err := s.db.QueryContext(ctx, pagesQuery, append([]any{params.SiteID, params.Start, params.End}, filterArgs...)...)
	if err != nil {
		return nil, err
	}
	defer pRows.Close()
	for pRows.Next() {
		var m api.MetricStat
		if err := pRows.Scan(&m.Name, &m.Value); err == nil {
			stats.TopPages = append(stats.TopPages, m)
		}
	}

	// TODO: Refactor once we tackle Events
	//nolint:gosec // filterSQL is derived from a fixed allowlist
	refQuery := fmt.Sprintf(`
		SELECT 
			CASE 
				WHEN h.referrer IS NULL OR h.referrer = '' THEN '(Direct)'
				-- Simple hack to extract domain-ish part for now, relies on 'http' prefix
				WHEN h.referrer LIKE 'http%%' THEN regexp_extract(h.referrer, 'https?://([^/]+)', 1)
				ELSE h.referrer
			END as source, 
			COUNT(*) as val 
		FROM hits h
		WHERE h.site_id = ? AND h.timestamp >= ? AND h.timestamp <= ?%s
		GROUP BY source 
		ORDER BY val DESC 
		LIMIT 10
	`, filterSQL)
	rRows, err := s.db.QueryContext(ctx, refQuery, append([]any{params.SiteID, params.Start, params.End}, filterArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rRows.Close()
	for rRows.Next() {
		var m api.MetricStat
		if err := rRows.Scan(&m.Name, &m.Value); err == nil {
			stats.TopReferrers = append(stats.TopReferrers, m)
		}
	}

	// TODO: Good enough for now
	//nolint:gosec // filterSQL is derived from a fixed allowlist
	devQuery := fmt.Sprintf(`
		SELECT 
			CASE 
				WHEN h.viewport_width < 576 THEN 'Mobile'
				WHEN h.viewport_width < 992 THEN 'Tablet'
				ELSE 'Desktop' 
			END as device,
			COUNT(*) as val 
		FROM hits h
		WHERE h.site_id = ? AND h.timestamp >= ? AND h.timestamp <= ?%s
		GROUP BY device 
		ORDER BY val DESC 
	`, filterSQL)
	dRows, err := s.db.QueryContext(ctx, devQuery, append([]any{params.SiteID, params.Start, params.End}, filterArgs...)...)
	if err != nil {
		return nil, err
	}
	defer dRows.Close()
	for dRows.Next() {
		var m api.MetricStat
		if err := dRows.Scan(&m.Name, &m.Value); err == nil {
			stats.TopDevices = append(stats.TopDevices, m)
		}
	}

	// 6. Goals
	// Fetch all goals for the site
	goals, err := s.GetGoals(ctx, params.SiteID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch goals: %w", err)
	}

	for _, goal := range goals {
		var conversions int
		var err error

		switch goal.Type {
		case "path":
			err = s.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT session_id)
				FROM hits
				WHERE site_id = ? AND timestamp >= ? AND timestamp <= ? AND path = ?
			`, params.SiteID, params.Start, params.End, goal.Value).Scan(&conversions)
		case "event":
			err = s.db.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT session_id)
				FROM events
				WHERE site_id = ? AND timestamp >= ? AND timestamp <= ? AND name = ?
			`, params.SiteID, params.Start, params.End, goal.Value).Scan(&conversions)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to calc goal conversions: %w", err)
		}

		rate := 0.0
		if stats.UniqueSessions > 0 {
			rate = (float64(conversions) / float64(stats.UniqueSessions)) * 100
		}

		stats.Goals = append(stats.Goals, api.GoalStats{
			GoalID:         goal.ID,
			Name:           goal.Name,
			Conversions:    conversions,
			ConversionRate: rate,
		})
	}

	return stats, nil
}

func (s *Store) GetFunnelStats(ctx context.Context, funnelID uuid.UUID, params api.AnalyticsParams) (*api.FunnelStats, error) {
	var funnel api.Funnel
	var stepsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, name, steps FROM funnels WHERE id = ? AND site_id = ?", funnelID, params.SiteID).Scan(&funnel.ID, &funnel.Name, &stepsJSON)
	if err != nil {
		return nil, fmt.Errorf("funnel not found: %w", err)
	}
	if err := json.Unmarshal(stepsJSON, &funnel.Steps); err != nil {
		return nil, err
	}

	stats := &api.FunnelStats{
		FunnelID: funnel.ID,
		Name:     funnel.Name,
		Steps:    make([]api.FunnelStepStats, len(funnel.Steps)),
	}

	// We need to find sessions that completed step 1, then step 2 (after step 1), etc.
	// This is complex in SQL. For v1.5, we'll use a simplified approach:
	// Count unique sessions for each step independently, but respecting the time range.
	// A more rigorous approach would use sequence matching (e.g. DuckDB's window functions).

	// Let's try a slightly better approach: CTEs for each step.
	// But constructing dynamic SQL with CTEs for N steps is hard.
	// Let's stick to the "independent unique sessions" approximation for now,
	// but we can refine it to "sessions that did step X AND step X-1".

	// Actually, for a true funnel, we want:
	// Step 1: Count(Sessions doing Step 1)
	// Step 2: Count(Sessions doing Step 2 AND Step 1) ... this gets expensive.

	// Let's do the "Simple Funnel" first:
	// Step 1: Users who did A
	// Step 2: Users who did B (regardless of if they did A first, but usually B implies A in linear flows)
	// Wait, that's wrong. A funnel MUST be sequential.

	// Correct Approach for DuckDB:
	// Use `list_sort(list(struct(timestamp, type, value)))` per session to reconstruct the journey?
	// Or just simple counts for now to ship v1.5.

	// Let's implement the "Simple Count" for v1.0 stability, but label it clearly.
	// IMPROVEMENT: We will filter Step N to only include sessions present in Step N-1.

	var previousStepSessions []uuid.UUID
	var firstStepCount int

	for i, step := range funnel.Steps {
		var currentSessions []uuid.UUID
		var query string
		var args []any

		if step.Type == "path" {
			query = "SELECT DISTINCT session_id FROM hits WHERE site_id = ? AND timestamp >= ? AND timestamp <= ? AND path = ?"
			args = []any{params.SiteID, params.Start, params.End, step.Value}
		} else {
			query = "SELECT DISTINCT session_id FROM events WHERE site_id = ? AND timestamp >= ? AND timestamp <= ? AND name = ?"
			args = []any{params.SiteID, params.Start, params.End, step.Value}
		}

		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		stepSessionMap := make(map[uuid.UUID]bool)
		for rows.Next() {
			var sid uuid.UUID
			if err := rows.Scan(&sid); err == nil {
				stepSessionMap[sid] = true
			}
		}
		rows.Close()

		// Filter: Only keep sessions that were in the previous step (if i > 0)
		if i == 0 {
			for sid := range stepSessionMap {
				currentSessions = append(currentSessions, sid)
			}
			firstStepCount = len(currentSessions)
		} else {
			for _, prevSid := range previousStepSessions {
				if stepSessionMap[prevSid] {
					currentSessions = append(currentSessions, prevSid)
				}
			}
		}

		count := len(currentSessions)
		dropoff := 0
		conversionRate := 0.0

		if i > 0 {
			prevCount := stats.Steps[i-1].Visitors
			dropoff = prevCount - count
			if prevCount > 0 {
				conversionRate = (float64(count) / float64(prevCount)) * 100
			}
		} else {
			conversionRate = 100.0 // First step is always 100% of itself
		}

		stats.Steps[i] = api.FunnelStepStats{
			StepIndex:      i,
			Name:           fmt.Sprintf("%s: %s", step.Type, step.Value),
			Visitors:       count,
			Dropoff:        dropoff,
			ConversionRate: conversionRate,
		}

		previousStepSessions = currentSessions
	}

	stats.TotalEntries = firstStepCount
	if len(stats.Steps) > 0 {
		stats.TotalCompletions = stats.Steps[len(stats.Steps)-1].Visitors
	}
	if stats.TotalEntries > 0 {
		stats.OverallConversionRate = (float64(stats.TotalCompletions) / float64(stats.TotalEntries)) * 100
	}

	return stats, nil
}
