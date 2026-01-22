package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
		TopCountries: []api.MetricStat{},
		Goals:        []api.GoalStats{},
	}

	filterSQL, filterArgs := buildHitFilters(params.Filters, "h")
	funnelPathSQL, funnelPathArgs, err := s.buildFunnelPathFilter(ctx, params, "h")
	if err != nil {
		return nil, err
	}
	sessionSQL, sessionArgs, err := s.buildSessionFilter(ctx, params, "h")
	if err != nil {
		return nil, err
	}
	filterSQL += funnelPathSQL
	filterSQL += sessionSQL
	filterArgs = append(filterArgs, funnelPathArgs...)
	filterArgs = append(filterArgs, sessionArgs...)
	useRollups := len(params.Filters) == 0
	if sessionSQL != "" || funnelPathSQL != "" {
		useRollups = false
	}

	liveThreshold := time.Now().Add(-5 * time.Minute)
	liveQuery := "SELECT COUNT(DISTINCT h.session_id) FROM hits h WHERE h.site_id = ? AND h.timestamp >= ?" + filterSQL
	err = s.db.QueryRowContext(ctx, liveQuery, append([]any{params.SiteID, liveThreshold}, filterArgs...)...).Scan(&stats.LiveVisitors)
	if err != nil {
		return nil, fmt.Errorf("failed to calc live visitors: %w", err)
	}

	duration := params.End.Sub(params.Start)
	interval := "1 DAY"
	truncUnit := "day"
	rollupKind := rollupHourly

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
		if duration >= 180*24*time.Hour {
			interval = "1 MONTH"
			truncUnit = "month"
			rollupKind = rollupMonthly
			gridStart = time.Date(gridStart.Year(), gridStart.Month(), 1, 0, 0, 0, 0, gridStart.Location())
			gridEnd = time.Date(gridEnd.Year(), gridEnd.Month(), 1, 0, 0, 0, 0, gridEnd.Location())
			if !gridEnd.After(params.End) {
				gridEnd = gridEnd.AddDate(0, 1, 0)
			}
		} else {
			rollupKind = rollupDaily
		}
	}

	if useRollups {
		switch rollupKind {
		case rollupHourly:
			if err := s.ensureHourlyRollups(ctx, params.SiteID, gridStart, gridEnd); err != nil {
				return nil, fmt.Errorf("failed to update hourly rollups: %w", err)
			}
			if err := s.ensureHourlySessionRollups(ctx, params.SiteID, gridStart, gridEnd); err != nil {
				return nil, fmt.Errorf("failed to update hourly session rollups: %w", err)
			}
		case rollupDaily:
			if err := s.ensureDailyRollups(ctx, params.SiteID, gridStart, gridEnd); err != nil {
				return nil, fmt.Errorf("failed to update daily rollups: %w", err)
			}
			if err := s.ensureDailySessionRollups(ctx, params.SiteID, gridStart, gridEnd); err != nil {
				return nil, fmt.Errorf("failed to update daily session rollups: %w", err)
			}
		case rollupMonthly:
			if err := s.ensureMonthlyRollups(ctx, params.SiteID, gridStart, gridEnd); err != nil {
				return nil, fmt.Errorf("failed to update monthly rollups: %w", err)
			}
			if err := s.ensureMonthlySessionRollups(ctx, params.SiteID, gridStart, gridEnd); err != nil {
				return nil, fmt.Errorf("failed to update monthly session rollups: %w", err)
			}
		}
	}

	err = s.queryKpis(ctx, params, filterSQL, filterArgs, useRollups, rollupKind, &stats.TotalPageviews, &stats.UniqueSessions, &stats.BounceRate, &stats.AvgSessionDuration, &stats.PagesPerSession)
	if err != nil {
		return nil, fmt.Errorf("failed to calc KPIs: %w", err)
	}

	if useRollups {
		stats.ChartData, err = s.queryHybridChartData(ctx, params, truncUnit, rollupKind)
		if err != nil {
			return nil, fmt.Errorf("failed to query hybrid chart data: %w", err)
		}
	} else {
		rows, err := s.queryChartData(ctx, params, gridStart, gridEnd, interval, truncUnit, filterSQL, filterArgs, useRollups, rollupKind)
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
	}

	// Top lists via GROUPING SETS to keep a single scan.
	//nolint:gosec // filterSQL is derived from a fixed allowlist
	topQuery := fmt.Sprintf(`
		WITH base AS (
			SELECT
				h.path AS path,
				CASE
					WHEN h.referrer IS NULL OR h.referrer = '' THEN '(Direct)'
					WHEN h.referrer LIKE 'http%%' THEN regexp_extract(h.referrer, 'https?://([^/]+)', 1)
					ELSE h.referrer
				END AS referrer,
				CASE
					WHEN h.viewport_width < 576 THEN 'Mobile'
					WHEN h.viewport_width < 992 THEN 'Tablet'
					ELSE 'Desktop'
				END AS device,
				COALESCE(NULLIF(h.country_code, ''), '(Unknown)') AS country
			FROM hits h
			WHERE h.site_id = ? AND h.timestamp >= ? AND h.timestamp <= ?%s
		),
		agg AS (
			SELECT
				CASE
					WHEN GROUPING(path) = 0 THEN 'path'
					WHEN GROUPING(referrer) = 0 THEN 'referrer'
					WHEN GROUPING(device) = 0 THEN 'device'
					WHEN GROUPING(country) = 0 THEN 'country'
				END AS dim,
				COALESCE(path, referrer, device, country) AS name,
				COUNT(*) AS val
			FROM base
			GROUP BY GROUPING SETS ((path), (referrer), (device), (country))
		),
		ranked AS (
			SELECT
				dim,
				name,
				val,
				ROW_NUMBER() OVER (PARTITION BY dim ORDER BY val DESC) AS rn
			FROM agg
		)
		SELECT dim, name, val
		FROM ranked
		WHERE rn <= 10
		ORDER BY dim, val DESC;
	`, filterSQL)

	topRows, err := s.db.QueryContext(ctx, topQuery, append([]any{params.SiteID, params.Start, params.End}, filterArgs...)...)
	if err != nil {
		return nil, err
	}
	defer topRows.Close()

	for topRows.Next() {
		var dim string
		var m api.MetricStat
		if err := topRows.Scan(&dim, &m.Name, &m.Value); err != nil {
			return nil, err
		}
		switch dim {
		case "path":
			stats.TopPages = append(stats.TopPages, m)
		case "referrer":
			stats.TopReferrers = append(stats.TopReferrers, m)
		case "device":
			stats.TopDevices = append(stats.TopDevices, m)
		case "country":
			stats.TopCountries = append(stats.TopCountries, m)
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

func (s *Store) buildSessionFilter(ctx context.Context, params api.AnalyticsParams, alias string) (string, []any, error) {
	if len(params.GoalIDs) == 0 && len(params.FunnelIDs) == 0 {
		return "", nil, nil
	}

	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}

	var clauses []string
	var args []any

	if len(params.GoalIDs) > 0 {
		goals, err := s.GetGoals(ctx, params.SiteID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to load goals: %w", err)
		}
		allowed := make(map[uuid.UUID]struct{}, len(params.GoalIDs))
		for _, id := range params.GoalIDs {
			allowed[id] = struct{}{}
		}

		var pathValues []string
		var eventValues []string
		for _, goal := range goals {
			if _, ok := allowed[goal.ID]; !ok {
				continue
			}
			switch goal.Type {
			case "path":
				pathValues = append(pathValues, goal.Value)
			case "event":
				eventValues = append(eventValues, goal.Value)
			}
		}

		if len(pathValues) == 0 && len(eventValues) == 0 {
			return " AND 1=0", nil, nil
		}

		subquery, subArgs := buildSessionUnionSubquery(params.SiteID, params.Start, params.End, pathValues, eventValues)
		clauses = append(clauses, fmt.Sprintf("%ssession_id IN (%s)", prefix, subquery))
		args = append(args, subArgs...)
	}

	if len(params.FunnelIDs) > 0 {
		funnels, err := s.GetFunnels(ctx, params.SiteID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to load funnels: %w", err)
		}
		allowed := make(map[uuid.UUID]struct{}, len(params.FunnelIDs))
		for _, id := range params.FunnelIDs {
			allowed[id] = struct{}{}
		}

		var entryPathValues []string
		var entryEventValues []string
		for _, funnel := range funnels {
			if _, ok := allowed[funnel.ID]; !ok {
				continue
			}
			if len(funnel.Steps) == 0 {
				continue
			}
			first := funnel.Steps[0]
			switch first.Type {
			case "path":
				entryPathValues = append(entryPathValues, first.Value)
			case "event":
				entryEventValues = append(entryEventValues, first.Value)
			}
		}

		if len(entryPathValues) == 0 && len(entryEventValues) == 0 {
			return " AND 1=0", nil, nil
		}

		subquery, subArgs := buildSessionUnionSubquery(params.SiteID, params.Start, params.End, entryPathValues, entryEventValues)
		clauses = append(clauses, fmt.Sprintf("%ssession_id IN (%s)", prefix, subquery))
		args = append(args, subArgs...)
	}

	if len(clauses) == 0 {
		return "", nil, nil
	}

	return " AND " + strings.Join(clauses, " AND "), args, nil
}

func (s *Store) buildFunnelPathFilter(ctx context.Context, params api.AnalyticsParams, alias string) (string, []any, error) {
	if len(params.FunnelIDs) == 0 {
		return "", nil, nil
	}

	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}

	funnels, err := s.GetFunnels(ctx, params.SiteID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load funnels: %w", err)
	}
	allowed := make(map[uuid.UUID]struct{}, len(params.FunnelIDs))
	for _, id := range params.FunnelIDs {
		allowed[id] = struct{}{}
	}

	pathSet := make(map[string]struct{})
	for _, funnel := range funnels {
		if _, ok := allowed[funnel.ID]; !ok {
			continue
		}
		for _, step := range funnel.Steps {
			if step.Type == "path" && step.Value != "" {
				pathSet[step.Value] = struct{}{}
			}
		}
	}

	if len(pathSet) == 0 {
		return " AND 1=0", nil, nil
	}

	values := make([]string, 0, len(pathSet))
	for value := range pathSet {
		values = append(values, value)
	}

	placeholders := buildPlaceholders(len(values))
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}

	return fmt.Sprintf(" AND %spath IN (%s)", prefix, placeholders), args, nil
}

func (s *Store) queryChartData(
	ctx context.Context,
	params api.AnalyticsParams,
	gridStart time.Time,
	gridEnd time.Time,
	interval string,
	truncUnit string,
	filterSQL string,
	filterArgs []any,
	useRollups bool,
	rollupKind rollupKind,
) (*sql.Rows, error) {
	if useRollups {
		return s.queryRollupChart(ctx, params.SiteID, gridStart, gridEnd, interval, truncUnit, rollupKind)
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

	return s.db.QueryContext(ctx, chartQuery, append([]any{gridStart, gridEnd, params.SiteID, params.Start, params.End}, filterArgs...)...)
}

func (s *Store) queryKpis(
	ctx context.Context,
	params api.AnalyticsParams,
	filterSQL string,
	filterArgs []any,
	useRollups bool,
	kind rollupKind,
	totalPageviews *int,
	uniqueSessions *int,
	bounceRate *float64,
	avgDuration *float64,
	pagesPerSession *float64,
) error {
	if useRollups {
		table := "session_rollups_hourly"
		switch kind {
		case rollupDaily:
			table = "session_rollups_daily"
		case rollupMonthly:
			table = "session_rollups_monthly"
		case rollupHourly:
			table = "session_rollups_hourly"
		}

		query := fmt.Sprintf(`
			SELECT
				COALESCE(SUM(pageviews), 0) as total_pageviews,
				COALESCE(SUM(sessions), 0) as unique_sessions,
				CASE
					WHEN COALESCE(SUM(sessions), 0) = 0 THEN 0
					ELSE CAST(SUM(bounced_sessions) AS FLOAT) / SUM(sessions) * 100
				END as bounce_rate,
				CASE
					WHEN COALESCE(SUM(sessions), 0) = 0 THEN 0
					ELSE COALESCE(SUM(duration_sum_seconds), 0) / SUM(sessions)
				END as avg_duration_seconds,
				CASE
					WHEN COALESCE(SUM(sessions), 0) = 0 THEN 0
					ELSE CAST(SUM(pageviews) AS FLOAT) / SUM(sessions)
				END as pages_per_session
			FROM %s
			WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		`, table)

		return s.db.QueryRowContext(ctx, query, params.SiteID, params.Start, params.End).Scan(
			totalPageviews,
			uniqueSessions,
			bounceRate,
			avgDuration,
			pagesPerSession,
		)
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

	return s.db.QueryRowContext(ctx, kpiQuery, append([]any{params.SiteID, params.Start, params.End}, filterArgs...)...).Scan(
		totalPageviews,
		uniqueSessions,
		bounceRate,
		avgDuration,
		pagesPerSession,
	)
}

type rollupKind string

const (
	rollupHourly  rollupKind = "hourly"
	rollupDaily   rollupKind = "daily"
	rollupMonthly rollupKind = "monthly"
)

func rollupKindFromTruncUnit(truncUnit string) rollupKind {
	switch truncUnit {
	case "hour":
		return rollupHourly
	case "month":
		return rollupMonthly
	default:
		return rollupDaily
	}
}

func (s *Store) queryRollupChart(
	ctx context.Context,
	siteID uuid.UUID,
	gridStart time.Time,
	gridEnd time.Time,
	interval string,
	truncUnit string,
	kind rollupKind,
) (*sql.Rows, error) {
	var table string
	switch kind {
	case rollupHourly:
		table = "hit_rollups_hourly"
	case rollupDaily:
		table = "hit_rollups_daily"
	case rollupMonthly:
		table = "hit_rollups_monthly"
	default:
		table = "hit_rollups_hourly"
	}

	//nolint:gosec // interval/truncUnit are derived from fixed allowlists
	chartQuery := fmt.Sprintf(`
	WITH time_range AS (
		SELECT unnest(generate_series(?::TIMESTAMP, ?::TIMESTAMP, INTERVAL %s)) as bucket
	),
	rollup_hits AS (
		SELECT 
			date_trunc('%s', bucket)::TIMESTAMP as bucket,
			SUM(pageviews) as pageviews,
			SUM(visitors) as visitors
		FROM %s
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
	)
	SELECT 
		tr.bucket,
		COALESCE(rh.pageviews, 0),
		COALESCE(rh.visitors, 0)
	FROM time_range tr
	LEFT JOIN rollup_hits rh ON tr.bucket = rh.bucket
	ORDER BY tr.bucket ASC;
	`, interval, truncUnit, table)

	return s.db.QueryContext(ctx, chartQuery, gridStart, gridEnd, siteID, gridStart, gridEnd)
}

func (s *Store) ensureHourlyRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.ensureHourlyRollup(ctx, "hit_rollups_hourly", siteID, start, end, s.insertHourlyRollups)
}

func (s *Store) insertHourlyRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.Add(time.Hour)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO hit_rollups_hourly (site_id, bucket, pageviews, visitors)
		SELECT
			site_id,
			date_trunc('hour', timestamp)::TIMESTAMPTZ as bucket,
			COUNT(*) as pageviews,
			COUNT(DISTINCT session_id) as visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ?
		GROUP BY site_id, bucket
	`, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert rollups: %w", err)
	}
	return nil
}

func (s *Store) ensureDailyRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.ensureDailyRollup(ctx, "hit_rollups_daily", siteID, start, end, s.insertDailyRollups)
}

func (s *Store) insertDailyRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.AddDate(0, 0, 1)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO hit_rollups_daily (site_id, bucket, pageviews, visitors)
		SELECT
			site_id,
			date_trunc('day', timestamp)::DATE as bucket,
			COUNT(*) as pageviews,
			COUNT(DISTINCT session_id) as visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ?
		GROUP BY site_id, bucket
	`, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert daily rollups: %w", err)
	}
	return nil
}

func (s *Store) ensureMonthlyRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.ensureMonthlyRollup(ctx, "hit_rollups_monthly", siteID, start, end, s.insertMonthlyRollups)
}

func (s *Store) ensureGoalRollups(ctx context.Context, kind rollupKind, siteID uuid.UUID, start time.Time, end time.Time) error {
	switch kind {
	case rollupHourly:
		return s.ensureHourlyRollup(ctx, "goal_rollups_hourly", siteID, start, end, s.insertHourlyGoalRollups)
	case rollupDaily:
		return s.ensureDailyRollup(ctx, "goal_rollups_daily", siteID, start, end, s.insertDailyGoalRollups)
	case rollupMonthly:
		return s.ensureMonthlyRollup(ctx, "goal_rollups_monthly", siteID, start, end, s.insertMonthlyGoalRollups)
	default:
		return s.ensureDailyRollup(ctx, "goal_rollups_daily", siteID, start, end, s.insertDailyGoalRollups)
	}
}

func (s *Store) ensureFunnelRollups(ctx context.Context, kind rollupKind, siteID uuid.UUID, start time.Time, end time.Time) error {
	switch kind {
	case rollupHourly:
		return s.ensureHourlyRollup(ctx, "funnel_rollups_hourly", siteID, start, end, s.insertHourlyFunnelRollups)
	case rollupDaily:
		return s.ensureDailyRollup(ctx, "funnel_rollups_daily", siteID, start, end, s.insertDailyFunnelRollups)
	case rollupMonthly:
		return s.ensureMonthlyRollup(ctx, "funnel_rollups_monthly", siteID, start, end, s.insertMonthlyFunnelRollups)
	default:
		return s.ensureDailyRollup(ctx, "funnel_rollups_daily", siteID, start, end, s.insertDailyFunnelRollups)
	}
}

func (s *Store) ensureHourlySessionRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.ensureHourlyRollup(ctx, "session_rollups_hourly", siteID, start, end, s.insertHourlySessionRollups)
}

func (s *Store) insertHourlySessionRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.Add(time.Hour)

	_, err := s.db.ExecContext(ctx, `
		WITH session_metrics AS (
			SELECT
				site_id,
				session_id,
				date_trunc('hour', timestamp)::TIMESTAMPTZ as bucket,
				COUNT(*) as pvs,
				EXTRACT('epoch' FROM (MAX(timestamp) - MIN(timestamp))) as duration_seconds
			FROM hits
			WHERE site_id = ? AND timestamp >= ? AND timestamp < ?
			GROUP BY site_id, session_id, bucket
		)
		INSERT INTO session_rollups_hourly (site_id, bucket, sessions, bounced_sessions, duration_sum_seconds, pageviews)
		SELECT
			site_id,
			bucket,
			COUNT(*) as sessions,
			SUM(CASE WHEN pvs = 1 THEN 1 ELSE 0 END) as bounced_sessions,
			COALESCE(SUM(duration_seconds), 0) as duration_sum_seconds,
			COALESCE(SUM(pvs), 0) as pageviews
		FROM session_metrics
		GROUP BY site_id, bucket
	`, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert hourly session rollups: %w", err)
	}
	return nil
}

func (s *Store) ensureDailySessionRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.ensureDailyRollup(ctx, "session_rollups_daily", siteID, start, end, s.insertDailySessionRollups)
}

func (s *Store) insertDailySessionRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.AddDate(0, 0, 1)

	_, err := s.db.ExecContext(ctx, `
		WITH session_metrics AS (
			SELECT
				site_id,
				session_id,
				date_trunc('day', timestamp)::DATE as bucket,
				COUNT(*) as pvs,
				EXTRACT('epoch' FROM (MAX(timestamp) - MIN(timestamp))) as duration_seconds
			FROM hits
			WHERE site_id = ? AND timestamp >= ? AND timestamp < ?
			GROUP BY site_id, session_id, bucket
		)
		INSERT INTO session_rollups_daily (site_id, bucket, sessions, bounced_sessions, duration_sum_seconds, pageviews)
		SELECT
			site_id,
			bucket,
			COUNT(*) as sessions,
			SUM(CASE WHEN pvs = 1 THEN 1 ELSE 0 END) as bounced_sessions,
			COALESCE(SUM(duration_seconds), 0) as duration_sum_seconds,
			COALESCE(SUM(pvs), 0) as pageviews
		FROM session_metrics
		GROUP BY site_id, bucket
	`, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert daily session rollups: %w", err)
	}
	return nil
}

func (s *Store) ensureMonthlySessionRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.ensureMonthlyRollup(ctx, "session_rollups_monthly", siteID, start, end, s.insertMonthlySessionRollups)
}

func (s *Store) insertMonthlySessionRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.AddDate(0, 1, 0)

	_, err := s.db.ExecContext(ctx, `
		WITH session_metrics AS (
			SELECT
				site_id,
				session_id,
				date_trunc('month', timestamp)::DATE as bucket,
				COUNT(*) as pvs,
				EXTRACT('epoch' FROM (MAX(timestamp) - MIN(timestamp))) as duration_seconds
			FROM hits
			WHERE site_id = ? AND timestamp >= ? AND timestamp < ?
			GROUP BY site_id, session_id, bucket
		)
		INSERT INTO session_rollups_monthly (site_id, bucket, sessions, bounced_sessions, duration_sum_seconds, pageviews)
		SELECT
			site_id,
			bucket,
			COUNT(*) as sessions,
			SUM(CASE WHEN pvs = 1 THEN 1 ELSE 0 END) as bounced_sessions,
			COALESCE(SUM(duration_seconds), 0) as duration_sum_seconds,
			COALESCE(SUM(pvs), 0) as pageviews
		FROM session_metrics
		GROUP BY site_id, bucket
	`, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert monthly session rollups: %w", err)
	}
	return nil
}

func (s *Store) ensureMonthlyRollup(
	ctx context.Context,
	table string,
	siteID uuid.UUID,
	start time.Time,
	end time.Time,
	insertFn func(context.Context, uuid.UUID, time.Time, time.Time) error,
) error {
	startBucket := monthOnly(start)
	endBucket := monthOnly(end)
	if endBucket.Before(startBucket) {
		return nil
	}

	var minBucket sql.NullTime
	var maxBucket sql.NullTime
	//nolint:gosec // table is selected from fixed allowlist
	query := fmt.Sprintf("SELECT MIN(bucket), MAX(bucket) FROM %s WHERE site_id = ?", table)
	if err := s.db.QueryRowContext(ctx, query, siteID).Scan(&minBucket, &maxBucket); err != nil {
		return err
	}

	if !minBucket.Valid || !maxBucket.Valid {
		return insertFn(ctx, siteID, startBucket, endBucket)
	}

	if startBucket.Before(minBucket.Time) {
		if err := insertFn(ctx, siteID, startBucket, minBucket.Time.AddDate(0, -1, 0)); err != nil {
			return err
		}
	}

	if endBucket.After(maxBucket.Time) {
		if err := insertFn(ctx, siteID, maxBucket.Time.AddDate(0, 1, 0), endBucket); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) ensureDailyRollup(
	ctx context.Context,
	table string,
	siteID uuid.UUID,
	start time.Time,
	end time.Time,
	insertFn func(context.Context, uuid.UUID, time.Time, time.Time) error,
) error {
	startBucket := dateOnly(start)
	endBucket := dateOnly(end)
	if endBucket.Before(startBucket) {
		return nil
	}

	var minBucket sql.NullTime
	var maxBucket sql.NullTime
	//nolint:gosec // table is selected from fixed allowlist
	query := fmt.Sprintf("SELECT MIN(bucket), MAX(bucket) FROM %s WHERE site_id = ?", table)
	if err := s.db.QueryRowContext(ctx, query, siteID).Scan(&minBucket, &maxBucket); err != nil {
		return err
	}

	if !minBucket.Valid || !maxBucket.Valid {
		return insertFn(ctx, siteID, startBucket, endBucket)
	}

	if startBucket.Before(minBucket.Time) {
		if err := insertFn(ctx, siteID, startBucket, minBucket.Time.AddDate(0, 0, -1)); err != nil {
			return err
		}
	}

	if endBucket.After(maxBucket.Time) {
		if err := insertFn(ctx, siteID, maxBucket.Time.AddDate(0, 0, 1), endBucket); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) ensureHourlyRollup(
	ctx context.Context,
	table string,
	siteID uuid.UUID,
	start time.Time,
	end time.Time,
	insertFn func(context.Context, uuid.UUID, time.Time, time.Time) error,
) error {
	startBucket := start.Truncate(time.Hour)
	endBucket := end.Truncate(time.Hour)
	if endBucket.Before(startBucket) {
		return nil
	}

	var minBucket sql.NullTime
	var maxBucket sql.NullTime
	//nolint:gosec // table is selected from fixed allowlist
	query := fmt.Sprintf("SELECT MIN(bucket), MAX(bucket) FROM %s WHERE site_id = ?", table)
	if err := s.db.QueryRowContext(ctx, query, siteID).Scan(&minBucket, &maxBucket); err != nil {
		return err
	}

	if !minBucket.Valid || !maxBucket.Valid {
		return insertFn(ctx, siteID, startBucket, endBucket)
	}

	if startBucket.Before(minBucket.Time) {
		if err := insertFn(ctx, siteID, startBucket, minBucket.Time.Add(-time.Hour)); err != nil {
			return err
		}
	}

	if endBucket.After(maxBucket.Time) {
		if err := insertFn(ctx, siteID, maxBucket.Time.Add(time.Hour), endBucket); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) BackfillRollups(ctx context.Context, siteID uuid.UUID) error {
	var minTS sql.NullTime
	var maxTS sql.NullTime
	if err := s.db.QueryRowContext(ctx,
		"SELECT MIN(timestamp), MAX(timestamp) FROM hits WHERE site_id = ?",
		siteID,
	).Scan(&minTS, &maxTS); err != nil {
		return err
	}
	if !minTS.Valid || !maxTS.Valid {
		return nil
	}

	start := minTS.Time
	end := maxTS.Time

	if err := s.ensureHourlyRollups(ctx, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureDailyRollups(ctx, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureMonthlyRollups(ctx, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureHourlySessionRollups(ctx, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureDailySessionRollups(ctx, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureMonthlySessionRollups(ctx, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureGoalRollups(ctx, rollupHourly, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureGoalRollups(ctx, rollupDaily, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureGoalRollups(ctx, rollupMonthly, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureFunnelRollups(ctx, rollupHourly, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureFunnelRollups(ctx, rollupDaily, siteID, start, end); err != nil {
		return err
	}
	if err := s.ensureFunnelRollups(ctx, rollupMonthly, siteID, start, end); err != nil {
		return err
	}

	return nil
}

func (s *Store) insertMonthlyRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.AddDate(0, 1, 0)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO hit_rollups_monthly (site_id, bucket, pageviews, visitors)
		SELECT
			site_id,
			date_trunc('month', timestamp)::DATE as bucket,
			COUNT(*) as pageviews,
			COUNT(DISTINCT session_id) as visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ?
		GROUP BY site_id, bucket
	`, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert monthly rollups: %w", err)
	}
	return nil
}

func (s *Store) insertHourlyGoalRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.Add(time.Hour)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO goal_rollups_hourly (site_id, goal_id, bucket, conversions)
		SELECT
			g.site_id,
			g.id,
			date_trunc('hour', h.timestamp)::TIMESTAMPTZ as bucket,
			COUNT(DISTINCT h.session_id) as conversions
		FROM goals g
		JOIN hits h ON h.site_id = g.site_id AND g.type = 'path' AND h.path = g.value
		WHERE g.site_id = ? AND h.timestamp >= ? AND h.timestamp < ?
		GROUP BY g.site_id, g.id, bucket
		UNION ALL
		SELECT
			g.site_id,
			g.id,
			date_trunc('hour', e.timestamp)::TIMESTAMPTZ as bucket,
			COUNT(DISTINCT e.session_id) as conversions
		FROM goals g
		JOIN events e ON e.site_id = g.site_id AND g.type = 'event' AND e.name = g.value
		WHERE g.site_id = ? AND e.timestamp >= ? AND e.timestamp < ?
		GROUP BY g.site_id, g.id, bucket
	`, siteID, start, endExclusive, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert hourly goal rollups: %w", err)
	}
	return nil
}

func (s *Store) insertDailyGoalRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.AddDate(0, 0, 1)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO goal_rollups_daily (site_id, goal_id, bucket, conversions)
		SELECT
			g.site_id,
			g.id,
			date_trunc('day', h.timestamp)::DATE as bucket,
			COUNT(DISTINCT h.session_id) as conversions
		FROM goals g
		JOIN hits h ON h.site_id = g.site_id AND g.type = 'path' AND h.path = g.value
		WHERE g.site_id = ? AND h.timestamp >= ? AND h.timestamp < ?
		GROUP BY g.site_id, g.id, bucket
		UNION ALL
		SELECT
			g.site_id,
			g.id,
			date_trunc('day', e.timestamp)::DATE as bucket,
			COUNT(DISTINCT e.session_id) as conversions
		FROM goals g
		JOIN events e ON e.site_id = g.site_id AND g.type = 'event' AND e.name = g.value
		WHERE g.site_id = ? AND e.timestamp >= ? AND e.timestamp < ?
		GROUP BY g.site_id, g.id, bucket
	`, siteID, start, endExclusive, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert daily goal rollups: %w", err)
	}
	return nil
}

func (s *Store) insertMonthlyGoalRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	if end.Before(start) {
		return nil
	}
	endExclusive := end.AddDate(0, 1, 0)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO goal_rollups_monthly (site_id, goal_id, bucket, conversions)
		SELECT
			g.site_id,
			g.id,
			date_trunc('month', h.timestamp)::DATE as bucket,
			COUNT(DISTINCT h.session_id) as conversions
		FROM goals g
		JOIN hits h ON h.site_id = g.site_id AND g.type = 'path' AND h.path = g.value
		WHERE g.site_id = ? AND h.timestamp >= ? AND h.timestamp < ?
		GROUP BY g.site_id, g.id, bucket
		UNION ALL
		SELECT
			g.site_id,
			g.id,
			date_trunc('month', e.timestamp)::DATE as bucket,
			COUNT(DISTINCT e.session_id) as conversions
		FROM goals g
		JOIN events e ON e.site_id = g.site_id AND g.type = 'event' AND e.name = g.value
		WHERE g.site_id = ? AND e.timestamp >= ? AND e.timestamp < ?
		GROUP BY g.site_id, g.id, bucket
	`, siteID, start, endExclusive, siteID, start, endExclusive)
	if err != nil {
		return fmt.Errorf("failed to insert monthly goal rollups: %w", err)
	}
	return nil
}

func (s *Store) insertHourlyFunnelRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.insertFunnelRollups(ctx, "funnel_rollups_hourly", "hour", siteID, start, end.Add(time.Hour))
}

func (s *Store) insertDailyFunnelRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.insertFunnelRollups(ctx, "funnel_rollups_daily", "day", siteID, start, end.AddDate(0, 0, 1))
}

func (s *Store) insertMonthlyFunnelRollups(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time) error {
	return s.insertFunnelRollups(ctx, "funnel_rollups_monthly", "month", siteID, start, end.AddDate(0, 1, 0))
}

func (s *Store) insertFunnelRollups(ctx context.Context, table string, truncUnit string, siteID uuid.UUID, start time.Time, endExclusive time.Time) error {
	if endExclusive.Before(start) {
		return nil
	}

	funnels, err := s.GetFunnels(ctx, siteID)
	if err != nil {
		return fmt.Errorf("failed to fetch funnels: %w", err)
	}
	if len(funnels) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	//nolint:gosec // table is selected from fixed allowlist
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (site_id, funnel_id, bucket, entries, completions)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (site_id, funnel_id, bucket) DO NOTHING
	`, table))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, funnel := range funnels {
		if len(funnel.Steps) == 0 {
			continue
		}
		first := funnel.Steps[0]
		last := funnel.Steps[len(funnel.Steps)-1]

		entryCounts, err := s.queryFunnelStepCounts(ctx, siteID, start, endExclusive, truncUnit, first)
		if err != nil {
			return err
		}
		completionCounts, err := s.queryFunnelStepCounts(ctx, siteID, start, endExclusive, truncUnit, last)
		if err != nil {
			return err
		}

		buckets := make(map[time.Time]bool)
		for bucket := range entryCounts {
			buckets[bucket] = true
		}
		for bucket := range completionCounts {
			buckets[bucket] = true
		}

		for bucket := range buckets {
			entries := entryCounts[bucket]
			completions := completionCounts[bucket]
			if entries == 0 && completions == 0 {
				continue
			}
			if _, err := stmt.ExecContext(ctx, siteID, funnel.ID, bucket, entries, completions); err != nil {
				return fmt.Errorf("failed to insert funnel rollup: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) queryFunnelStepCounts(ctx context.Context, siteID uuid.UUID, start time.Time, end time.Time, truncUnit string, step api.FunnelStep) (map[time.Time]int, error) {
	if end.Before(start) {
		return map[time.Time]int{}, nil
	}

	var (
		table      string
		valueField string
		value      string
	)
	if step.Type == "path" {
		table = "hits"
		valueField = "path"
		value = step.Value
	} else {
		table = "events"
		valueField = "name"
		value = step.Value
	}

	//nolint:gosec // table/valueField/truncUnit are from fixed allowlists
	query := fmt.Sprintf(`
		SELECT date_trunc('%s', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM %s
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ? AND %s = ?
		GROUP BY bucket
		ORDER BY bucket
	`, truncUnit, table, valueField)

	rows, err := s.db.QueryContext(ctx, query, siteID, start, end, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[time.Time]int)
	for rows.Next() {
		var bucket time.Time
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, err
		}
		result[truncToUnit(bucket, truncUnit)] = count
	}

	return result, nil
}

func (s *Store) queryGoalRollupCounts(ctx context.Context, kind rollupKind, siteID uuid.UUID, start time.Time, end time.Time, goalIDs []uuid.UUID) (map[time.Time]int, error) {
	var table string
	switch kind {
	case rollupHourly:
		table = "goal_rollups_hourly"
	case rollupDaily:
		table = "goal_rollups_daily"
	case rollupMonthly:
		table = "goal_rollups_monthly"
	default:
		table = "goal_rollups_hourly"
	}

	args := []any{siteID, start, end}
	filterSQL := ""
	if len(goalIDs) > 0 {
		filterSQL = fmt.Sprintf(" AND goal_id IN (%s)", buildPlaceholders(len(goalIDs)))
		for _, id := range goalIDs {
			args = append(args, id)
		}
	}

	//nolint:gosec // table is selected from fixed allowlist
	query := fmt.Sprintf(`
		SELECT bucket, SUM(conversions) as conversions
		FROM %s
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?%s
		GROUP BY bucket
		ORDER BY bucket
	`, table, filterSQL)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[time.Time]int)
	for rows.Next() {
		var bucket time.Time
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, err
		}
		result[truncToUnit(bucket, kindToTruncUnit(kind))] = count
	}
	return result, nil
}

func (s *Store) queryFunnelRollupCounts(ctx context.Context, kind rollupKind, siteID uuid.UUID, start time.Time, end time.Time, funnelIDs []uuid.UUID) (map[time.Time]int, map[time.Time]int, error) {
	var table string
	switch kind {
	case rollupHourly:
		table = "funnel_rollups_hourly"
	case rollupDaily:
		table = "funnel_rollups_daily"
	case rollupMonthly:
		table = "funnel_rollups_monthly"
	default:
		table = "funnel_rollups_hourly"
	}

	args := []any{siteID, start, end}
	filterSQL := ""
	if len(funnelIDs) > 0 {
		filterSQL = fmt.Sprintf(" AND funnel_id IN (%s)", buildPlaceholders(len(funnelIDs)))
		for _, id := range funnelIDs {
			args = append(args, id)
		}
	}

	//nolint:gosec // table is selected from fixed allowlist
	query := fmt.Sprintf(`
		SELECT bucket, SUM(entries) as entries, SUM(completions) as completions
		FROM %s
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?%s
		GROUP BY bucket
		ORDER BY bucket
	`, table, filterSQL)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	entryCounts := make(map[time.Time]int)
	completionCounts := make(map[time.Time]int)
	for rows.Next() {
		var bucket time.Time
		var entries int
		var completions int
		if err := rows.Scan(&bucket, &entries, &completions); err != nil {
			return nil, nil, err
		}
		normalized := truncToUnit(bucket, kindToTruncUnit(kind))
		entryCounts[normalized] = entries
		completionCounts[normalized] = completions
	}
	return entryCounts, completionCounts, nil
}

func kindToTruncUnit(kind rollupKind) string {
	switch kind {
	case rollupHourly:
		return "hour"
	case rollupDaily:
		return "day"
	case rollupMonthly:
		return "month"
	default:
		return "day"
	}
}
func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func monthOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
}

func truncUnitForRange(start time.Time, end time.Time) string {
	duration := end.Sub(start)
	if duration < 48*time.Hour {
		return "hour"
	}
	if duration >= 180*24*time.Hour {
		return "month"
	}
	return "day"
}

func addUnit(value time.Time, truncUnit string, delta int) time.Time {
	switch truncUnit {
	case "hour":
		return value.Add(time.Duration(delta) * time.Hour)
	case "day":
		return value.AddDate(0, 0, delta)
	case "month":
		return value.AddDate(0, delta, 0)
	default:
		return value
	}
}

func truncToUnit(value time.Time, truncUnit string) time.Time {
	switch truncUnit {
	case "hour":
		return value.Truncate(time.Hour)
	case "day":
		return dateOnly(value)
	case "month":
		return monthOnly(value)
	default:
		return value
	}
}

func isAlignedToUnit(value time.Time, truncUnit string) bool {
	return value.Equal(truncToUnit(value, truncUnit))
}

type rollupWindow struct {
	FullStart time.Time
	FullEnd   time.Time
	Leading   *time.Time
	Trailing  *time.Time
	UseRollup bool
}

func buildRollupWindow(start time.Time, end time.Time, truncUnit string) rollupWindow {
	startBucket := truncToUnit(start, truncUnit)
	endBucket := truncToUnit(end, truncUnit)

	fullStart := startBucket
	if !isAlignedToUnit(start, truncUnit) {
		fullStart = addUnit(startBucket, truncUnit, 1)
	}

	fullEnd := addUnit(endBucket, truncUnit, -1)

	if fullStart.After(end) || fullEnd.Before(fullStart) {
		leadEnd := end
		return rollupWindow{
			Leading:   &leadEnd,
			UseRollup: false,
		}
	}

	window := rollupWindow{
		FullStart: fullStart,
		FullEnd:   fullEnd,
		UseRollup: true,
	}

	if start.Before(fullStart) {
		leadEnd := fullStart
		window.Leading = &leadEnd
	}

	trailingStart := addUnit(fullEnd, truncUnit, 1)
	if end.After(trailingStart) {
		window.Trailing = &trailingStart
	}

	return window
}

func buildSeriesBuckets(start time.Time, end time.Time, truncUnit string) []time.Time {
	if end.Before(start) {
		return nil
	}

	cursor := truncToUnit(start, truncUnit)
	last := truncToUnit(end, truncUnit)
	var buckets []time.Time
	for !cursor.After(last) {
		buckets = append(buckets, cursor)
		cursor = addUnit(cursor, truncUnit, 1)
	}
	return buckets
}

func (s *Store) queryHybridChartData(ctx context.Context, params api.AnalyticsParams, truncUnit string, rollupKind rollupKind) ([]api.ChartDataPoint, error) {
	window := buildRollupWindow(params.Start, params.End, truncUnit)
	counts := make(map[time.Time]api.ChartDataPoint)

	if window.UseRollup {
		if err := s.ensureHitRollups(ctx, rollupKind, params.SiteID, window.FullStart, window.FullEnd); err != nil {
			return nil, err
		}
		rollupCounts, err := s.queryHitRollupCounts(ctx, rollupKind, params.SiteID, window.FullStart, window.FullEnd)
		if err != nil {
			return nil, err
		}
		for bucket, point := range rollupCounts {
			counts[bucket] = mergeChartPoint(counts[bucket], point)
		}
	}

	if window.Leading != nil {
		edgeEnd := *window.Leading
		if edgeEnd.After(params.Start) {
			edgeEnd = edgeEnd.Add(-time.Nanosecond)
		}
		edgeParams := params
		edgeParams.Start = params.Start
		edgeParams.End = edgeEnd
		rawCounts, err := s.queryHitSeriesCounts(ctx, edgeParams, truncUnit)
		if err != nil {
			return nil, err
		}
		for bucket, point := range rawCounts {
			counts[bucket] = mergeChartPoint(counts[bucket], point)
		}
	}

	if window.Trailing != nil {
		edgeStart := *window.Trailing
		edgeParams := params
		edgeParams.Start = edgeStart
		edgeParams.End = params.End
		rawCounts, err := s.queryHitSeriesCounts(ctx, edgeParams, truncUnit)
		if err != nil {
			return nil, err
		}
		for bucket, point := range rawCounts {
			counts[bucket] = mergeChartPoint(counts[bucket], point)
		}
	}

	buckets := buildSeriesBuckets(params.Start, params.End, truncUnit)
	series := make([]api.ChartDataPoint, 0, len(buckets))
	for _, bucket := range buckets {
		point := counts[bucket]
		point.Time = bucket
		series = append(series, point)
	}

	return series, nil
}

func mergeChartPoint(base api.ChartDataPoint, next api.ChartDataPoint) api.ChartDataPoint {
	return api.ChartDataPoint{
		Time:      base.Time,
		Pageviews: base.Pageviews + next.Pageviews,
		Visitors:  base.Visitors + next.Visitors,
	}
}

func (s *Store) ensureHitRollups(ctx context.Context, kind rollupKind, siteID uuid.UUID, start time.Time, end time.Time) error {
	switch kind {
	case rollupHourly:
		return s.ensureHourlyRollups(ctx, siteID, start, end)
	case rollupDaily:
		return s.ensureDailyRollups(ctx, siteID, start, end)
	case rollupMonthly:
		return s.ensureMonthlyRollups(ctx, siteID, start, end)
	default:
		return s.ensureDailyRollups(ctx, siteID, start, end)
	}
}

func (s *Store) queryHitRollupCounts(ctx context.Context, kind rollupKind, siteID uuid.UUID, start time.Time, end time.Time) (map[time.Time]api.ChartDataPoint, error) {
	table := "hit_rollups_hourly"
	switch kind {
	case rollupHourly:
		table = "hit_rollups_hourly"
	case rollupDaily:
		table = "hit_rollups_daily"
	case rollupMonthly:
		table = "hit_rollups_monthly"
	}

	//nolint:gosec // table is selected from fixed allowlist
	query := fmt.Sprintf(`
		SELECT bucket, SUM(pageviews) as pageviews, SUM(visitors) as visitors
		FROM %s
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`, table)

	rows, err := s.db.QueryContext(ctx, query, siteID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[time.Time]api.ChartDataPoint)
	for rows.Next() {
		var bucket time.Time
		var pageviews int
		var visitors int
		if err := rows.Scan(&bucket, &pageviews, &visitors); err != nil {
			return nil, err
		}
		normalized := truncToUnit(bucket, kindToTruncUnit(kind))
		result[normalized] = api.ChartDataPoint{
			Time:      normalized,
			Pageviews: pageviews,
			Visitors:  visitors,
		}
	}
	return result, nil
}

func (s *Store) queryHitSeriesCounts(ctx context.Context, params api.AnalyticsParams, truncUnit string) (map[time.Time]api.ChartDataPoint, error) {
	//nolint:gosec // truncUnit is from fixed allowlist
	query := fmt.Sprintf(`
		SELECT date_trunc('%s', timestamp) AS bucket, COUNT(*) AS pageviews, COUNT(DISTINCT session_id) AS visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY bucket
		ORDER BY bucket
	`, truncUnit)

	rows, err := s.db.QueryContext(ctx, query, params.SiteID, params.Start, params.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[time.Time]api.ChartDataPoint)
	for rows.Next() {
		var bucket time.Time
		var pageviews int
		var visitors int
		if err := rows.Scan(&bucket, &pageviews, &visitors); err != nil {
			return nil, err
		}
		normalized := truncToUnit(bucket, truncUnit)
		result[normalized] = api.ChartDataPoint{
			Time:      normalized,
			Pageviews: pageviews,
			Visitors:  visitors,
		}
	}
	return result, nil
}

func buildPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}

func buildSessionUnionSubquery(siteID uuid.UUID, start time.Time, end time.Time, pathValues []string, eventValues []string) (string, []any) {
	var parts []string
	var args []any

	if len(pathValues) > 0 {
		parts = append(parts, buildSessionValueSubquery("hits", "path", len(pathValues)))
		args = append(args, siteID, start, end)
		for _, value := range pathValues {
			args = append(args, value)
		}
	}

	if len(eventValues) > 0 {
		parts = append(parts, buildSessionValueSubquery("events", "name", len(eventValues)))
		args = append(args, siteID, start, end)
		for _, value := range eventValues {
			args = append(args, value)
		}
	}

	return strings.Join(parts, " UNION "), args
}

func buildSessionValueSubquery(table string, field string, valueCount int) string {
	placeholders := buildPlaceholders(valueCount)
	//nolint:gosec // table/field are fixed allowlists in call sites
	return fmt.Sprintf(
		"SELECT DISTINCT session_id FROM %s WHERE site_id = ? AND timestamp >= ? AND timestamp <= ? AND %s IN (%s)",
		table,
		field,
		placeholders,
	)
}

func (s *Store) querySeriesCounts(ctx context.Context, table string, valueField string, values []string, params api.AnalyticsParams, truncUnit string) (map[time.Time]int, error) {
	result := make(map[time.Time]int)
	if len(values) == 0 {
		return result, nil
	}

	placeholders := buildPlaceholders(len(values))
	args := make([]any, 0, 3+len(values))
	args = append(args, params.SiteID, params.Start, params.End)
	for _, v := range values {
		args = append(args, v)
	}

	//nolint:gosec // table/valueField/truncUnit are from fixed allowlists
	query := fmt.Sprintf(`
		SELECT date_trunc('%s', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM %s
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ? AND %s IN (%s)
		GROUP BY bucket
		ORDER BY bucket
	`, truncUnit, table, valueField, placeholders)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var bucket time.Time
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, err
		}
		result[truncToUnit(bucket, truncUnit)] = count
	}
	return result, nil
}

func (s *Store) GetGoalTimeseries(ctx context.Context, params api.AnalyticsParams, goalIDs []uuid.UUID) ([]api.GoalSeriesPoint, error) {
	goals, err := s.GetGoals(ctx, params.SiteID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch goals: %w", err)
	}
	if len(goals) == 0 {
		return []api.GoalSeriesPoint{}, nil
	}

	if len(goalIDs) > 0 {
		allowed := make(map[uuid.UUID]bool, len(goalIDs))
		for _, id := range goalIDs {
			allowed[id] = true
		}
		filtered := goals[:0]
		for _, goal := range goals {
			if allowed[goal.ID] {
				filtered = append(filtered, goal)
			}
		}
		goals = filtered
		if len(goals) == 0 {
			return []api.GoalSeriesPoint{}, nil
		}
	}

	var pathValues []string
	var eventValues []string
	for _, goal := range goals {
		if goal.Type == "path" {
			pathValues = append(pathValues, goal.Value)
		}
		if goal.Type == "event" {
			eventValues = append(eventValues, goal.Value)
		}
	}

	truncUnit := truncUnitForRange(params.Start, params.End)
	rollupKind := rollupKindFromTruncUnit(truncUnit)
	window := buildRollupWindow(params.Start, params.End, truncUnit)

	counts := make(map[time.Time]int)
	if window.UseRollup {
		if err := s.ensureGoalRollups(ctx, rollupKind, params.SiteID, window.FullStart, window.FullEnd); err != nil {
			return nil, err
		}
		rollupCounts, err := s.queryGoalRollupCounts(ctx, rollupKind, params.SiteID, window.FullStart, window.FullEnd, goalIDs)
		if err != nil {
			return nil, err
		}
		for bucket, count := range rollupCounts {
			counts[bucket] += count
		}
	}

	if window.Leading != nil {
		edgeEnd := *window.Leading
		if edgeEnd.After(params.Start) {
			edgeEnd = edgeEnd.Add(-time.Nanosecond)
		}
		edgeParams := params
		edgeParams.Start = params.Start
		edgeParams.End = edgeEnd
		if len(pathValues) > 0 {
			pathCounts, err := s.querySeriesCounts(ctx, "hits", "path", pathValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range pathCounts {
				counts[bucket] += count
			}
		}
		if len(eventValues) > 0 {
			eventCounts, err := s.querySeriesCounts(ctx, "events", "name", eventValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range eventCounts {
				counts[bucket] += count
			}
		}
	}

	if window.Trailing != nil {
		edgeStart := *window.Trailing
		edgeParams := params
		edgeParams.Start = edgeStart
		edgeParams.End = params.End
		if len(pathValues) > 0 {
			pathCounts, err := s.querySeriesCounts(ctx, "hits", "path", pathValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range pathCounts {
				counts[bucket] += count
			}
		}
		if len(eventValues) > 0 {
			eventCounts, err := s.querySeriesCounts(ctx, "events", "name", eventValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range eventCounts {
				counts[bucket] += count
			}
		}
	}

	buckets := buildSeriesBuckets(params.Start, params.End, truncUnit)
	series := make([]api.GoalSeriesPoint, 0, len(buckets))
	for _, bucket := range buckets {
		series = append(series, api.GoalSeriesPoint{
			Time:        bucket,
			Conversions: counts[bucket],
		})
	}
	return series, nil
}

func (s *Store) GetFunnelTimeseries(ctx context.Context, params api.AnalyticsParams, funnelIDs []uuid.UUID) ([]api.FunnelSeriesPoint, error) {
	funnels, err := s.GetFunnels(ctx, params.SiteID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch funnels: %w", err)
	}
	if len(funnels) == 0 {
		return []api.FunnelSeriesPoint{}, nil
	}

	if len(funnelIDs) > 0 {
		allowed := make(map[uuid.UUID]bool, len(funnelIDs))
		for _, id := range funnelIDs {
			allowed[id] = true
		}
		filtered := funnels[:0]
		for _, funnel := range funnels {
			if allowed[funnel.ID] {
				filtered = append(filtered, funnel)
			}
		}
		funnels = filtered
		if len(funnels) == 0 {
			return []api.FunnelSeriesPoint{}, nil
		}
	}

	entryPaths := make(map[string]bool)
	entryEvents := make(map[string]bool)
	completionPaths := make(map[string]bool)
	completionEvents := make(map[string]bool)

	for _, funnel := range funnels {
		if len(funnel.Steps) == 0 {
			continue
		}
		first := funnel.Steps[0]
		last := funnel.Steps[len(funnel.Steps)-1]

		if first.Type == "path" {
			entryPaths[first.Value] = true
		} else {
			entryEvents[first.Value] = true
		}

		if last.Type == "path" {
			completionPaths[last.Value] = true
		} else {
			completionEvents[last.Value] = true
		}
	}

	if len(entryPaths) == 0 && len(entryEvents) == 0 && len(completionPaths) == 0 && len(completionEvents) == 0 {
		return []api.FunnelSeriesPoint{}, nil
	}

	truncUnit := truncUnitForRange(params.Start, params.End)
	rollupKind := rollupKindFromTruncUnit(truncUnit)
	window := buildRollupWindow(params.Start, params.End, truncUnit)
	entryCounts := make(map[time.Time]int)
	completionCounts := make(map[time.Time]int)

	entryPathValues := make([]string, 0, len(entryPaths))
	for v := range entryPaths {
		entryPathValues = append(entryPathValues, v)
	}
	entryEventValues := make([]string, 0, len(entryEvents))
	for v := range entryEvents {
		entryEventValues = append(entryEventValues, v)
	}
	completionPathValues := make([]string, 0, len(completionPaths))
	for v := range completionPaths {
		completionPathValues = append(completionPathValues, v)
	}
	completionEventValues := make([]string, 0, len(completionEvents))
	for v := range completionEvents {
		completionEventValues = append(completionEventValues, v)
	}

	if window.UseRollup {
		if err := s.ensureFunnelRollups(ctx, rollupKind, params.SiteID, window.FullStart, window.FullEnd); err != nil {
			return nil, err
		}
		rollupEntries, rollupCompletions, err := s.queryFunnelRollupCounts(ctx, rollupKind, params.SiteID, window.FullStart, window.FullEnd, funnelIDs)
		if err != nil {
			return nil, err
		}
		for bucket, count := range rollupEntries {
			entryCounts[bucket] += count
		}
		for bucket, count := range rollupCompletions {
			completionCounts[bucket] += count
		}
	}

	if window.Leading != nil {
		edgeEnd := *window.Leading
		if edgeEnd.After(params.Start) {
			edgeEnd = edgeEnd.Add(-time.Nanosecond)
		}
		edgeParams := params
		edgeParams.Start = params.Start
		edgeParams.End = edgeEnd
		if len(entryPathValues) > 0 {
			pathCounts, err := s.querySeriesCounts(ctx, "hits", "path", entryPathValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range pathCounts {
				entryCounts[bucket] += count
			}
		}
		if len(entryEventValues) > 0 {
			eventCounts, err := s.querySeriesCounts(ctx, "events", "name", entryEventValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range eventCounts {
				entryCounts[bucket] += count
			}
		}
		if len(completionPathValues) > 0 {
			pathCounts, err := s.querySeriesCounts(ctx, "hits", "path", completionPathValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range pathCounts {
				completionCounts[bucket] += count
			}
		}
		if len(completionEventValues) > 0 {
			eventCounts, err := s.querySeriesCounts(ctx, "events", "name", completionEventValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range eventCounts {
				completionCounts[bucket] += count
			}
		}
	}

	if window.Trailing != nil {
		edgeStart := *window.Trailing
		edgeParams := params
		edgeParams.Start = edgeStart
		edgeParams.End = params.End
		if len(entryPathValues) > 0 {
			pathCounts, err := s.querySeriesCounts(ctx, "hits", "path", entryPathValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range pathCounts {
				entryCounts[bucket] += count
			}
		}
		if len(entryEventValues) > 0 {
			eventCounts, err := s.querySeriesCounts(ctx, "events", "name", entryEventValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range eventCounts {
				entryCounts[bucket] += count
			}
		}
		if len(completionPathValues) > 0 {
			pathCounts, err := s.querySeriesCounts(ctx, "hits", "path", completionPathValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range pathCounts {
				completionCounts[bucket] += count
			}
		}
		if len(completionEventValues) > 0 {
			eventCounts, err := s.querySeriesCounts(ctx, "events", "name", completionEventValues, edgeParams, truncUnit)
			if err != nil {
				return nil, err
			}
			for bucket, count := range eventCounts {
				completionCounts[bucket] += count
			}
		}
	}

	buckets := buildSeriesBuckets(params.Start, params.End, truncUnit)
	series := make([]api.FunnelSeriesPoint, 0, len(buckets))
	for _, bucket := range buckets {
		series = append(series, api.FunnelSeriesPoint{
			Time:        bucket,
			Entries:     entryCounts[bucket],
			Completions: completionCounts[bucket],
		})
	}
	return series, nil
}

func (s *Store) GetFunnelStats(ctx context.Context, funnelID uuid.UUID, params api.AnalyticsParams) (*api.FunnelStats, error) {
	var funnel api.Funnel
	var stepsJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT id, name, CAST(steps AS VARCHAR) FROM funnels WHERE id = ? AND site_id = ?", funnelID, params.SiteID).Scan(&funnel.ID, &funnel.Name, &stepsJSON)
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
