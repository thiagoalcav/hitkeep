package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

type rollupKind string

const (
	rollupHourly  rollupKind = "hourly"
	rollupDaily   rollupKind = "daily"
	rollupMonthly rollupKind = "monthly"
)

const (
	hitRollupsHourlyQuery = `
		SELECT bucket, SUM(pageviews) as pageviews, SUM(visitors) as visitors
		FROM hit_rollups_hourly
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	hitRollupsDailyQuery = `
		SELECT bucket, SUM(pageviews) as pageviews, SUM(visitors) as visitors
		FROM hit_rollups_daily
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	hitRollupsMonthlyQuery = `
		SELECT bucket, SUM(pageviews) as pageviews, SUM(visitors) as visitors
		FROM hit_rollups_monthly
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	goalRollupsHourlyQuery = `
		SELECT bucket, SUM(conversions) as conversions
		FROM goal_rollups_hourly
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	goalRollupsDailyQuery = `
		SELECT bucket, SUM(conversions) as conversions
		FROM goal_rollups_daily
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	goalRollupsMonthlyQuery = `
		SELECT bucket, SUM(conversions) as conversions
		FROM goal_rollups_monthly
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelRollupsHourlyQuery = `
		SELECT bucket, SUM(entries) as entries, SUM(completions) as completions
		FROM funnel_rollups_hourly
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelRollupsDailyQuery = `
		SELECT bucket, SUM(entries) as entries, SUM(completions) as completions
		FROM funnel_rollups_daily
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelRollupsMonthlyQuery = `
		SELECT bucket, SUM(entries) as entries, SUM(completions) as completions
		FROM funnel_rollups_monthly
		WHERE site_id = ? AND bucket >= ? AND bucket <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	hitSeriesHourlyQuery = `
		SELECT date_trunc('hour', timestamp) AS bucket, COUNT(*) AS pageviews, COUNT(DISTINCT session_id) AS visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	hitSeriesDailyQuery = `
		SELECT date_trunc('day', timestamp) AS bucket, COUNT(*) AS pageviews, COUNT(DISTINCT session_id) AS visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	hitSeriesMonthlyQuery = `
		SELECT date_trunc('month', timestamp) AS bucket, COUNT(*) AS pageviews, COUNT(DISTINCT session_id) AS visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelStepHitsPathHourlyQuery = `
		SELECT date_trunc('hour', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ? AND path = ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelStepHitsPathDailyQuery = `
		SELECT date_trunc('day', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ? AND path = ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelStepHitsPathMonthlyQuery = `
		SELECT date_trunc('month', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ? AND path = ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelStepEventsNameHourlyQuery = `
		SELECT date_trunc('hour', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM events
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ? AND name = ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelStepEventsNameDailyQuery = `
		SELECT date_trunc('day', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM events
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ? AND name = ?
		GROUP BY bucket
		ORDER BY bucket
	`
	funnelStepEventsNameMonthlyQuery = `
		SELECT date_trunc('month', timestamp) AS bucket, COUNT(DISTINCT session_id) AS conversions
		FROM events
		WHERE site_id = ? AND timestamp >= ? AND timestamp < ? AND name = ?
		GROUP BY bucket
		ORDER BY bucket
	`
)

type analyticsStatements struct {
	hitRollupsHourly            *sql.Stmt
	hitRollupsDaily             *sql.Stmt
	hitRollupsMonthly           *sql.Stmt
	goalRollupsHourly           *sql.Stmt
	goalRollupsDaily            *sql.Stmt
	goalRollupsMonthly          *sql.Stmt
	funnelRollupsHourly         *sql.Stmt
	funnelRollupsDaily          *sql.Stmt
	funnelRollupsMonthly        *sql.Stmt
	hitSeriesHourly             *sql.Stmt
	hitSeriesDaily              *sql.Stmt
	hitSeriesMonthly            *sql.Stmt
	funnelStepHitsPathHourly    *sql.Stmt
	funnelStepHitsPathDaily     *sql.Stmt
	funnelStepHitsPathMonthly   *sql.Stmt
	funnelStepEventsNameHourly  *sql.Stmt
	funnelStepEventsNameDaily   *sql.Stmt
	funnelStepEventsNameMonthly *sql.Stmt
}

func (a *analyticsStatements) close() error {
	var firstErr error
	for _, stmt := range []*sql.Stmt{
		a.hitRollupsHourly,
		a.hitRollupsDaily,
		a.hitRollupsMonthly,
		a.goalRollupsHourly,
		a.goalRollupsDaily,
		a.goalRollupsMonthly,
		a.funnelRollupsHourly,
		a.funnelRollupsDaily,
		a.funnelRollupsMonthly,
		a.hitSeriesHourly,
		a.hitSeriesDaily,
		a.hitSeriesMonthly,
		a.funnelStepHitsPathHourly,
		a.funnelStepHitsPathDaily,
		a.funnelStepHitsPathMonthly,
		a.funnelStepEventsNameHourly,
		a.funnelStepEventsNameDaily,
		a.funnelStepEventsNameMonthly,
	} {
		if stmt == nil {
			continue
		}
		if err := stmt.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (a *analyticsStatements) hitRollupStmt(kind rollupKind) *sql.Stmt {
	switch kind {
	case rollupHourly:
		return a.hitRollupsHourly
	case rollupDaily:
		return a.hitRollupsDaily
	case rollupMonthly:
		return a.hitRollupsMonthly
	}
	return a.hitRollupsHourly
}

func (a *analyticsStatements) goalRollupStmt(kind rollupKind) *sql.Stmt {
	switch kind {
	case rollupHourly:
		return a.goalRollupsHourly
	case rollupDaily:
		return a.goalRollupsDaily
	case rollupMonthly:
		return a.goalRollupsMonthly
	}
	return a.goalRollupsHourly
}

func (a *analyticsStatements) funnelRollupStmt(kind rollupKind) *sql.Stmt {
	switch kind {
	case rollupHourly:
		return a.funnelRollupsHourly
	case rollupDaily:
		return a.funnelRollupsDaily
	case rollupMonthly:
		return a.funnelRollupsMonthly
	}
	return a.funnelRollupsHourly
}

func (a *analyticsStatements) hitSeriesStmt(truncUnit string) *sql.Stmt {
	switch truncUnit {
	case "hour":
		return a.hitSeriesHourly
	case "month":
		return a.hitSeriesMonthly
	default:
		return a.hitSeriesDaily
	}
}

func (a *analyticsStatements) funnelStepStmt(stepType string, truncUnit string) *sql.Stmt {
	useHits := stepType == "path"
	switch truncUnit {
	case "hour":
		if useHits {
			return a.funnelStepHitsPathHourly
		}
		return a.funnelStepEventsNameHourly
	case "month":
		if useHits {
			return a.funnelStepHitsPathMonthly
		}
		return a.funnelStepEventsNameMonthly
	default:
		if useHits {
			return a.funnelStepHitsPathDaily
		}
		return a.funnelStepEventsNameDaily
	}
}

func prepareAnalyticsStatements(ctx context.Context, db *sql.DB) (*analyticsStatements, error) {
	stmts := &analyticsStatements{}
	var err error

	stmts.hitRollupsHourly, err = db.PrepareContext(ctx, hitRollupsHourlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.hitRollupsDaily, err = db.PrepareContext(ctx, hitRollupsDailyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.hitRollupsMonthly, err = db.PrepareContext(ctx, hitRollupsMonthlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}

	stmts.goalRollupsHourly, err = db.PrepareContext(ctx, goalRollupsHourlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.goalRollupsDaily, err = db.PrepareContext(ctx, goalRollupsDailyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.goalRollupsMonthly, err = db.PrepareContext(ctx, goalRollupsMonthlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}

	stmts.funnelRollupsHourly, err = db.PrepareContext(ctx, funnelRollupsHourlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.funnelRollupsDaily, err = db.PrepareContext(ctx, funnelRollupsDailyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.funnelRollupsMonthly, err = db.PrepareContext(ctx, funnelRollupsMonthlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}

	stmts.hitSeriesHourly, err = db.PrepareContext(ctx, hitSeriesHourlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.hitSeriesDaily, err = db.PrepareContext(ctx, hitSeriesDailyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.hitSeriesMonthly, err = db.PrepareContext(ctx, hitSeriesMonthlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}

	stmts.funnelStepHitsPathHourly, err = db.PrepareContext(ctx, funnelStepHitsPathHourlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.funnelStepHitsPathDaily, err = db.PrepareContext(ctx, funnelStepHitsPathDailyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.funnelStepHitsPathMonthly, err = db.PrepareContext(ctx, funnelStepHitsPathMonthlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}

	stmts.funnelStepEventsNameHourly, err = db.PrepareContext(ctx, funnelStepEventsNameHourlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.funnelStepEventsNameDaily, err = db.PrepareContext(ctx, funnelStepEventsNameDailyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}
	stmts.funnelStepEventsNameMonthly, err = db.PrepareContext(ctx, funnelStepEventsNameMonthlyQuery)
	if err != nil {
		_ = stmts.close()
		return nil, err
	}

	return stmts, nil
}

func (s *Store) ensureAnalyticsStatements(ctx context.Context) (*analyticsStatements, error) {
	s.analyticsMu.Lock()
	defer s.analyticsMu.Unlock()
	if s.analyticsStatements != nil {
		return s.analyticsStatements, nil
	}

	stmts, err := prepareAnalyticsStatements(ctx, s.db)
	if err != nil {
		return nil, err
	}
	s.analyticsStatements = stmts
	return stmts, nil
}

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
		ON CONFLICT (site_id, bucket) DO UPDATE SET
			pageviews = EXCLUDED.pageviews,
			visitors = EXCLUDED.visitors
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
		ON CONFLICT (site_id, bucket) DO UPDATE SET
			pageviews = EXCLUDED.pageviews,
			visitors = EXCLUDED.visitors
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
		ON CONFLICT (site_id, bucket) DO UPDATE SET
			sessions = EXCLUDED.sessions,
			bounced_sessions = EXCLUDED.bounced_sessions,
			duration_sum_seconds = EXCLUDED.duration_sum_seconds,
			pageviews = EXCLUDED.pageviews
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
		ON CONFLICT (site_id, bucket) DO UPDATE SET
			sessions = EXCLUDED.sessions,
			bounced_sessions = EXCLUDED.bounced_sessions,
			duration_sum_seconds = EXCLUDED.duration_sum_seconds,
			pageviews = EXCLUDED.pageviews
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
		ON CONFLICT (site_id, bucket) DO UPDATE SET
			sessions = EXCLUDED.sessions,
			bounced_sessions = EXCLUDED.bounced_sessions,
			duration_sum_seconds = EXCLUDED.duration_sum_seconds,
			pageviews = EXCLUDED.pageviews
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

	return insertFn(ctx, siteID, startBucket, endBucket)
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

	return insertFn(ctx, siteID, startBucket, endBucket)
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

	return insertFn(ctx, siteID, startBucket, endBucket)
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
		ON CONFLICT (site_id, bucket) DO UPDATE SET
			pageviews = EXCLUDED.pageviews,
			visitors = EXCLUDED.visitors
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
		ON CONFLICT (site_id, goal_id, bucket) DO UPDATE SET
			conversions = EXCLUDED.conversions
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
		ON CONFLICT (site_id, goal_id, bucket) DO UPDATE SET
			conversions = EXCLUDED.conversions
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
		ON CONFLICT (site_id, goal_id, bucket) DO UPDATE SET
			conversions = EXCLUDED.conversions
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
		ON CONFLICT (site_id, funnel_id, bucket) DO UPDATE SET
			entries = EXCLUDED.entries,
			completions = EXCLUDED.completions
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

	stmts, err := s.ensureAnalyticsStatements(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := stmts.funnelStepStmt(step.Type, truncUnit).QueryContext(ctx, siteID, start, end, step.Value)
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
	var (
		rows  *sql.Rows
		err   error
		stmts *analyticsStatements
	)

	if len(goalIDs) == 0 {
		stmts, err = s.ensureAnalyticsStatements(ctx)
		if err != nil {
			return nil, err
		}
		rows, err = stmts.goalRollupStmt(kind).QueryContext(ctx, siteID, start, end)
	} else {
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
		filterSQL := fmt.Sprintf(" AND goal_id IN (%s)", buildPlaceholders(len(goalIDs)))
		for _, id := range goalIDs {
			args = append(args, id)
		}

		//nolint:gosec // table is selected from fixed allowlist
		query := fmt.Sprintf(`
			SELECT bucket, SUM(conversions) as conversions
			FROM %s
			WHERE site_id = ? AND bucket >= ? AND bucket <= ?%s
			GROUP BY bucket
			ORDER BY bucket
		`, table, filterSQL)

		rows, err = s.db.QueryContext(ctx, query, args...)
	}
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
	var (
		rows  *sql.Rows
		err   error
		stmts *analyticsStatements
	)

	if len(funnelIDs) == 0 {
		stmts, err = s.ensureAnalyticsStatements(ctx)
		if err != nil {
			return nil, nil, err
		}
		rows, err = stmts.funnelRollupStmt(kind).QueryContext(ctx, siteID, start, end)
	} else {
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
		filterSQL := fmt.Sprintf(" AND funnel_id IN (%s)", buildPlaceholders(len(funnelIDs)))
		for _, id := range funnelIDs {
			args = append(args, id)
		}

		//nolint:gosec // table is selected from fixed allowlist
		query := fmt.Sprintf(`
			SELECT bucket, SUM(entries) as entries, SUM(completions) as completions
			FROM %s
			WHERE site_id = ? AND bucket >= ? AND bucket <= ?%s
			GROUP BY bucket
			ORDER BY bucket
		`, table, filterSQL)

		rows, err = s.db.QueryContext(ctx, query, args...)
	}
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
		if err := s.refreshDirtyRollupsInRange(ctx, params.SiteID, dirtyRollupHit, rollupKind, window.FullStart, window.FullEnd); err != nil {
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

func (s *Store) queryHitRollupCounts(ctx context.Context, kind rollupKind, siteID uuid.UUID, start time.Time, end time.Time) (map[time.Time]api.ChartDataPoint, error) {
	stmts, err := s.ensureAnalyticsStatements(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := stmts.hitRollupStmt(kind).QueryContext(ctx, siteID, start, end)
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
	stmts, err := s.ensureAnalyticsStatements(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := stmts.hitSeriesStmt(truncUnit).QueryContext(ctx, params.SiteID, params.Start, params.End)
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
