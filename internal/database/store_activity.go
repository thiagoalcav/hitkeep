package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

const (
	activityDormantAfter = 7 * 24 * time.Hour
	activityCountWindow  = 8 * 24 * time.Hour
)

var automaticActivityEvents = map[string]struct{}{
	"outbound_click": {},
	"file_download":  {},
	"form_submit":    {},
}

func normalizeTrackerMetadata(source, version string) (string, string) {
	source = strings.TrimSpace(source)
	version = strings.TrimSpace(version)
	if source == "" {
		source = "hk.js"
	}
	if len(source) > 64 {
		source = source[:64]
	}
	if len(version) > 64 {
		version = version[:64]
	}
	return source, version
}

func (s *Store) RecordHitActivity(ctx context.Context, hits []*api.Hit) error {
	for _, hit := range hits {
		if hit == nil || hit.SiteID == uuid.Nil {
			continue
		}
		source, version := normalizeTrackerMetadata(hit.TrackerSource, hit.TrackerVersion)
		hostname := ""
		if hit.Hostname != nil {
			hostname = strings.TrimSpace(*hit.Hostname)
		}
		if err := s.upsertSiteHitActivity(ctx, hit.SiteID, hit.Timestamp.UTC(), hostname, source, version); err != nil {
			return err
		}
	}
	if err := s.pruneSiteActivityCounts(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) RecordEventActivity(ctx context.Context, events []*api.Event) error {
	for _, event := range events {
		if event == nil || event.SiteID == uuid.Nil {
			continue
		}
		source, version := normalizeTrackerMetadata(event.TrackerSource, event.TrackerVersion)
		if err := s.upsertSiteEventActivity(ctx, event.SiteID, event.Timestamp.UTC(), event.Name, source, version); err != nil {
			return err
		}
	}
	if err := s.pruneSiteActivityCounts(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) upsertSiteHitActivity(ctx context.Context, siteID uuid.UUID, ts time.Time, hostname, source, version string) error {
	tenantID, err := s.GetSiteTenantID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("resolve site tenant for hit activity: %w", err)
	}
	now := time.Now().UTC()
	if err := s.Exec(ctx, `
		INSERT INTO site_activity_summary (
			site_id, tenant_id, first_hit_at, last_hit_at, last_hostname,
			tracker_source, tracker_version, updated_at
		)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)
		ON CONFLICT (site_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			first_hit_at = CASE
				WHEN site_activity_summary.first_hit_at IS NULL OR excluded.first_hit_at < site_activity_summary.first_hit_at
					THEN excluded.first_hit_at
				ELSE site_activity_summary.first_hit_at
			END,
			last_hit_at = CASE
				WHEN site_activity_summary.last_hit_at IS NULL OR excluded.last_hit_at >= site_activity_summary.last_hit_at
					THEN excluded.last_hit_at
				ELSE site_activity_summary.last_hit_at
			END,
			last_hostname = CASE
				WHEN site_activity_summary.last_hit_at IS NULL OR excluded.last_hit_at >= site_activity_summary.last_hit_at
					THEN COALESCE(excluded.last_hostname, site_activity_summary.last_hostname)
				ELSE site_activity_summary.last_hostname
			END,
			tracker_source = COALESCE(excluded.tracker_source, site_activity_summary.tracker_source),
			tracker_version = COALESCE(excluded.tracker_version, site_activity_summary.tracker_version),
			updated_at = excluded.updated_at
	`, siteID, tenantID, ts, ts, hostname, source, version, now); err != nil {
		return fmt.Errorf("upsert site hit activity: %w", err)
	}

	return s.incrementSiteActivityCount(ctx, siteID, tenantID, ts, 1, 0)
}

func (s *Store) upsertSiteEventActivity(ctx context.Context, siteID uuid.UUID, ts time.Time, eventName, source, version string) error {
	tenantID, err := s.GetSiteTenantID(ctx, siteID)
	if err != nil {
		return fmt.Errorf("resolve site tenant for event activity: %w", err)
	}
	eventName = strings.TrimSpace(eventName)
	_, isAutomatic := automaticActivityEvents[eventName]
	now := time.Now().UTC()

	var automaticAt any
	var automaticName any
	if isAutomatic {
		automaticAt = ts
		automaticName = eventName
	}

	if err := s.Exec(ctx, `
		INSERT INTO site_activity_summary (
			site_id, tenant_id, last_event_at, last_event_name,
			last_automatic_event_at, last_automatic_event_name,
			tracker_source, tracker_version, updated_at
		)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?)
		ON CONFLICT (site_id) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			last_event_at = CASE
				WHEN site_activity_summary.last_event_at IS NULL OR excluded.last_event_at >= site_activity_summary.last_event_at
					THEN excluded.last_event_at
				ELSE site_activity_summary.last_event_at
			END,
			last_event_name = CASE
				WHEN site_activity_summary.last_event_at IS NULL OR excluded.last_event_at >= site_activity_summary.last_event_at
					THEN COALESCE(excluded.last_event_name, site_activity_summary.last_event_name)
				ELSE site_activity_summary.last_event_name
			END,
			last_automatic_event_at = CASE
				WHEN excluded.last_automatic_event_at IS NOT NULL
					AND (site_activity_summary.last_automatic_event_at IS NULL OR excluded.last_automatic_event_at >= site_activity_summary.last_automatic_event_at)
					THEN excluded.last_automatic_event_at
				ELSE site_activity_summary.last_automatic_event_at
			END,
			last_automatic_event_name = CASE
				WHEN excluded.last_automatic_event_at IS NOT NULL
					AND (site_activity_summary.last_automatic_event_at IS NULL OR excluded.last_automatic_event_at >= site_activity_summary.last_automatic_event_at)
					THEN COALESCE(excluded.last_automatic_event_name, site_activity_summary.last_automatic_event_name)
				ELSE site_activity_summary.last_automatic_event_name
			END,
			tracker_source = COALESCE(excluded.tracker_source, site_activity_summary.tracker_source),
			tracker_version = COALESCE(excluded.tracker_version, site_activity_summary.tracker_version),
			updated_at = excluded.updated_at
	`, siteID, tenantID, ts, eventName, automaticAt, automaticName, source, version, now); err != nil {
		return fmt.Errorf("upsert site event activity: %w", err)
	}

	return s.incrementSiteActivityCount(ctx, siteID, tenantID, ts, 0, 1)
}

func (s *Store) incrementSiteActivityCount(ctx context.Context, siteID, tenantID uuid.UUID, ts time.Time, hits, events int) error {
	bucket := ts.UTC().Truncate(time.Hour)
	if err := s.Exec(ctx, `
		INSERT INTO site_activity_hourly_counts (site_id, tenant_id, bucket, hits, events, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (site_id, bucket) DO UPDATE SET
			tenant_id = excluded.tenant_id,
			hits = site_activity_hourly_counts.hits + excluded.hits,
			events = site_activity_hourly_counts.events + excluded.events,
			updated_at = excluded.updated_at
	`, siteID, tenantID, bucket, hits, events, time.Now().UTC()); err != nil {
		return fmt.Errorf("increment site activity counts: %w", err)
	}
	return nil
}

func (s *Store) pruneSiteActivityCounts(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-activityCountWindow).Truncate(time.Hour)
	if err := s.Exec(ctx, "DELETE FROM site_activity_hourly_counts WHERE bucket < ?", cutoff); err != nil {
		return fmt.Errorf("prune site activity counts: %w", err)
	}
	return nil
}

func (s *Store) GetSiteTrackingStatus(ctx context.Context, siteID uuid.UUID, now time.Time) (*api.SiteTrackingStatus, error) {
	site, err := s.GetSiteByID(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, nil
	}
	tenantID, err := s.GetSiteTenantID(ctx, siteID)
	if err != nil {
		return nil, err
	}

	var (
		firstHit           sql.NullTime
		lastHit            sql.NullTime
		lastEvent          sql.NullTime
		lastHostname       sql.NullString
		lastEventName      sql.NullString
		lastAutomaticEvent sql.NullTime
		lastAutomaticName  sql.NullString
		trackerSource      sql.NullString
		trackerVersion     sql.NullString
		updatedAt          sql.NullTime
	)
	err = s.QueryRowOrNil(ctx, `
		SELECT first_hit_at, last_hit_at, last_event_at, last_hostname, last_event_name,
			last_automatic_event_at, last_automatic_event_name, tracker_source, tracker_version, updated_at
		FROM site_activity_summary
		WHERE site_id = ?
	`, []any{&firstHit, &lastHit, &lastEvent, &lastHostname, &lastEventName, &lastAutomaticEvent, &lastAutomaticName, &trackerSource, &trackerVersion, &updatedAt}, siteID)
	if err != nil {
		return nil, fmt.Errorf("query site tracking status: %w", err)
	}

	status := api.TrackingStatusWaiting
	if lastHit.Valid {
		status = api.TrackingStatusLive
		if lastHostname.Valid && !sameActivationDomain(lastHostname.String, site.Domain) {
			status = api.TrackingStatusDomainMismatch
		} else if !now.IsZero() && now.UTC().Sub(lastHit.Time.UTC()) > activityDormantAfter {
			status = api.TrackingStatusDormant
		}
	}

	return &api.SiteTrackingStatus{
		SiteID:                 siteID,
		TenantID:               tenantID,
		Status:                 status,
		FirstHitAt:             nullTimePtr(firstHit),
		LastHitAt:              nullTimePtr(lastHit),
		LastEventAt:            nullTimePtr(lastEvent),
		LastHostname:           nullStringValue(lastHostname),
		LastEventName:          nullStringValue(lastEventName),
		LastAutomaticEventAt:   nullTimePtr(lastAutomaticEvent),
		LastAutomaticEventName: nullStringValue(lastAutomaticName),
		TrackerSource:          nullStringValue(trackerSource),
		TrackerVersion:         nullStringValue(trackerVersion),
		ConfiguredDomain:       site.Domain,
		UpdatedAt:              nullTimePtr(updatedAt),
	}, nil
}

type ActivationQuery struct {
	Status       string
	Team         string
	Domain       string
	LastSeenFrom *time.Time
	LastSeenTo   *time.Time
	Limit        int
	Offset       int
	Now          time.Time
}

func (s *Store) ListSystemActivation(ctx context.Context, q ActivationQuery) (*api.SystemActivationResponse, error) {
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Now.IsZero() {
		q.Now = time.Now().UTC()
	}

	dormantCutoff := q.Now.Add(-activityDormantAfter)
	statusFilter := strings.TrimSpace(q.Status)
	teamFilter := likeFilter(q.Team)
	domainFilter := likeFilter(q.Domain)
	var lastSeenFrom any
	if q.LastSeenFrom != nil {
		lastSeenFrom = q.LastSeenFrom.UTC()
	}
	var lastSeenTo any
	if q.LastSeenTo != nil {
		lastSeenTo = q.LastSeenTo.UTC()
	}

	var total int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sites s
		JOIN site_tenants st ON st.site_id = s.id
		JOIN tenants t ON t.id = st.tenant_id
		LEFT JOIN tenant_archives ta ON ta.tenant_id = t.id
		LEFT JOIN site_activity_summary sas ON sas.site_id = s.id
		WHERE ta.tenant_id IS NULL
			AND (
				? = ''
				OR CASE
					WHEN sas.first_hit_at IS NULL THEN 'waiting'
					WHEN sas.last_hostname IS NOT NULL
						AND CASE WHEN starts_with(lower(sas.last_hostname), 'www.') THEN substr(lower(sas.last_hostname), 5) ELSE lower(sas.last_hostname) END
							<> CASE WHEN starts_with(lower(s.domain), 'www.') THEN substr(lower(s.domain), 5) ELSE lower(s.domain) END
						THEN 'domain_mismatch'
					WHEN sas.last_hit_at < ? THEN 'dormant'
					ELSE 'live'
				END = ?
			)
			AND (? = '' OR lower(t.name) LIKE ?)
			AND (? = '' OR lower(s.domain) LIKE ?)
			AND (? IS NULL OR sas.last_hit_at >= ?)
			AND (? IS NULL OR sas.last_hit_at <= ?)
	`, statusFilter, dormantCutoff, statusFilter, teamFilter, teamFilter, domainFilter, domainFilter, lastSeenFrom, lastSeenFrom, lastSeenTo, lastSeenTo).Scan(&total); err != nil {
		return nil, fmt.Errorf("count activation rows: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		WITH counts AS (
			SELECT
				site_id,
				SUM(CASE WHEN bucket >= ? THEN hits ELSE 0 END) AS hits_24h,
				SUM(CASE WHEN bucket >= ? THEN hits ELSE 0 END) AS hits_7d,
				SUM(CASE WHEN bucket >= ? THEN events ELSE 0 END) AS events_7d
			FROM site_activity_hourly_counts
			GROUP BY site_id
		),
		team_site_counts AS (
			SELECT st.tenant_id, COUNT(*) AS sites_count
			FROM site_tenants st
			JOIN sites s ON s.id = st.site_id
			GROUP BY st.tenant_id
		),
		team_active_counts AS (
			SELECT st.tenant_id, COUNT(*) AS active_sites
			FROM site_tenants st
			JOIN sites s ON s.id = st.site_id
			JOIN site_activity_summary sas ON sas.site_id = s.id
			WHERE sas.last_hit_at >= ?
				AND (
					sas.last_hostname IS NULL
					OR CASE WHEN starts_with(lower(sas.last_hostname), 'www.') THEN substr(lower(sas.last_hostname), 5) ELSE lower(sas.last_hostname) END
						= CASE WHEN starts_with(lower(s.domain), 'www.') THEN substr(lower(s.domain), 5) ELSE lower(s.domain) END
				)
			GROUP BY st.tenant_id
		),
		team_owners AS (
			SELECT tm.tenant_id, MIN(u.email) AS owner_email
			FROM tenant_members tm
			JOIN users u ON u.id = tm.user_id
			WHERE tm.role = 'owner'
			GROUP BY tm.tenant_id
		)
		SELECT
			t.id, t.name, COALESCE(team_owners.owner_email, ''),
			COALESCE(cba.plan_code, ''), COALESCE(cba.plan_name, ''),
			s.id, s.domain,
			COALESCE(team_site_counts.sites_count, 0),
			COALESCE(team_active_counts.active_sites, 0),
			CASE
				WHEN sas.first_hit_at IS NULL THEN 'waiting'
				WHEN sas.last_hostname IS NOT NULL
					AND CASE WHEN starts_with(lower(sas.last_hostname), 'www.') THEN substr(lower(sas.last_hostname), 5) ELSE lower(sas.last_hostname) END
						<> CASE WHEN starts_with(lower(s.domain), 'www.') THEN substr(lower(s.domain), 5) ELSE lower(s.domain) END
					THEN 'domain_mismatch'
				WHEN sas.last_hit_at < ? THEN 'dormant'
				ELSE 'live'
			END AS status,
			sas.first_hit_at, sas.last_hit_at, sas.last_event_at, sas.last_event_name,
			COALESCE(counts.hits_24h, 0), COALESCE(counts.hits_7d, 0), COALESCE(counts.events_7d, 0),
			COALESCE(sas.tracker_source, ''), COALESCE(sas.tracker_version, '')
		FROM sites s
		JOIN site_tenants st ON st.site_id = s.id
		JOIN tenants t ON t.id = st.tenant_id
		LEFT JOIN tenant_archives ta ON ta.tenant_id = t.id
		LEFT JOIN site_activity_summary sas ON sas.site_id = s.id
		LEFT JOIN counts ON counts.site_id = s.id
		LEFT JOIN team_site_counts ON team_site_counts.tenant_id = t.id
		LEFT JOIN team_active_counts ON team_active_counts.tenant_id = t.id
		LEFT JOIN team_owners ON team_owners.tenant_id = t.id
		LEFT JOIN cloud_billing_accounts cba ON cba.tenant_id = t.id
		WHERE ta.tenant_id IS NULL
			AND (
				? = ''
				OR CASE
					WHEN sas.first_hit_at IS NULL THEN 'waiting'
					WHEN sas.last_hostname IS NOT NULL
						AND CASE WHEN starts_with(lower(sas.last_hostname), 'www.') THEN substr(lower(sas.last_hostname), 5) ELSE lower(sas.last_hostname) END
							<> CASE WHEN starts_with(lower(s.domain), 'www.') THEN substr(lower(s.domain), 5) ELSE lower(s.domain) END
						THEN 'domain_mismatch'
					WHEN sas.last_hit_at < ? THEN 'dormant'
					ELSE 'live'
				END = ?
			)
			AND (? = '' OR lower(t.name) LIKE ?)
			AND (? = '' OR lower(s.domain) LIKE ?)
			AND (? IS NULL OR sas.last_hit_at >= ?)
			AND (? IS NULL OR sas.last_hit_at <= ?)
		ORDER BY COALESCE(sas.last_hit_at, s.created_at) DESC, s.domain ASC
		LIMIT ? OFFSET ?
	`,
		q.Now.Add(-24*time.Hour),
		q.Now.Add(-7*24*time.Hour),
		q.Now.Add(-7*24*time.Hour),
		dormantCutoff,
		dormantCutoff,
		statusFilter,
		dormantCutoff,
		statusFilter,
		teamFilter,
		teamFilter,
		domainFilter,
		domainFilter,
		lastSeenFrom,
		lastSeenFrom,
		lastSeenTo,
		lastSeenTo,
		q.Limit,
		q.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query activation rows: %w", err)
	}
	defer rows.Close()

	resp := &api.SystemActivationResponse{
		Rows:   make([]api.SystemActivationRow, 0),
		Total:  total,
		Limit:  q.Limit,
		Offset: q.Offset,
	}
	for rows.Next() {
		var (
			row           api.SystemActivationRow
			status        string
			firstHit      sql.NullTime
			lastHit       sql.NullTime
			lastEvent     sql.NullTime
			lastEventName sql.NullString
		)
		if err := rows.Scan(
			&row.TeamID, &row.TeamName, &row.OwnerEmail, &row.PlanCode, &row.PlanName,
			&row.SiteID, &row.SiteDomain, &row.SitesCount, &row.ActiveSites, &status,
			&firstHit, &lastHit, &lastEvent, &lastEventName,
			&row.HitsLast24h, &row.HitsLast7d, &row.EventsLast7d,
			&row.TrackerSource, &row.TrackerVersion,
		); err != nil {
			return nil, fmt.Errorf("scan activation row: %w", err)
		}
		row.Status = api.TrackingStatus(status)
		row.FirstHitAt = nullTimePtr(firstHit)
		row.LastHitAt = nullTimePtr(lastHit)
		row.LastEventAt = nullTimePtr(lastEvent)
		row.LastEventName = nullStringValue(lastEventName)
		resp.Rows = append(resp.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read activation rows: %w", err)
	}
	resp.HasMore = q.Offset+len(resp.Rows) < total
	return resp, nil
}

func sameActivationDomain(a, b string) bool {
	normalize := func(value string) string {
		value = strings.ToLower(strings.TrimSpace(value))
		return strings.TrimPrefix(value, "www.")
	}
	return normalize(a) == normalize(b)
}

func likeFilter(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return "%" + value + "%"
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
