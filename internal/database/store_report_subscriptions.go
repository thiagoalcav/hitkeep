package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

// PendingSiteReport holds the data needed to send a per-site report email.
type PendingSiteReport struct {
	UserID    uuid.UUID
	UserEmail string
	SiteID    uuid.UUID
	Domain    string
}

// DigestSite is a single site entry within a PendingDigest.
type DigestSite struct {
	SiteID uuid.UUID
	Domain string
}

// PendingDigest holds the data needed to send a consolidated digest email.
type PendingDigest struct {
	UserID    uuid.UUID
	UserEmail string
	Sites     []DigestSite
}

// GetReportSubscriptions returns the full subscription state for a user across all their accessible sites.
func (s *Store) GetReportSubscriptions(ctx context.Context, userID uuid.UUID) (*api.ReportSubscriptions, error) {
	// Fetch all sites the user owns or is a member of.
	siteRows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT s.id, s.domain
		FROM sites s
		LEFT JOIN site_members sm ON sm.site_id = s.id AND sm.user_id = ?
		WHERE s.user_id = ? OR sm.user_id IS NOT NULL
		ORDER BY s.domain ASC
	`, userID, userID)
	if err != nil {
		return nil, err
	}
	defer siteRows.Close()

	type siteRow struct {
		id     uuid.UUID
		domain string
	}
	var sites []siteRow
	for siteRows.Next() {
		var sr siteRow
		if err := siteRows.Scan(&sr.id, &sr.domain); err != nil {
			return nil, err
		}
		sites = append(sites, sr)
	}
	if err := siteRows.Err(); err != nil {
		return nil, err
	}
	siteRows.Close()

	// Fetch all site_report_subscriptions for this user in one query.
	subRows, err := s.db.QueryContext(ctx, `
		SELECT site_id, frequency, enabled
		FROM site_report_subscriptions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer subRows.Close()

	type subKey struct {
		siteID uuid.UUID
		freq   string
	}
	subMap := make(map[subKey]bool)
	for subRows.Next() {
		var siteID uuid.UUID
		var freq string
		var enabled bool
		if err := subRows.Scan(&siteID, &freq, &enabled); err != nil {
			return nil, err
		}
		subMap[subKey{siteID: siteID, freq: freq}] = enabled
	}
	if err := subRows.Err(); err != nil {
		return nil, err
	}
	subRows.Close()

	siteSubscriptions := make([]api.SiteReportSubscription, 0, len(sites))
	for _, site := range sites {
		siteSubscriptions = append(siteSubscriptions, api.SiteReportSubscription{
			SiteID:  site.id,
			Domain:  site.domain,
			Daily:   subMap[subKey{siteID: site.id, freq: "daily"}],
			Weekly:  subMap[subKey{siteID: site.id, freq: "weekly"}],
			Monthly: subMap[subKey{siteID: site.id, freq: "monthly"}],
		})
	}

	// Fetch digest subscriptions.
	digestRows, err := s.db.QueryContext(ctx, `
		SELECT frequency, enabled
		FROM digest_subscriptions
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, err
	}
	defer digestRows.Close()

	digestMap := make(map[string]bool)
	for digestRows.Next() {
		var freq string
		var enabled bool
		if err := digestRows.Scan(&freq, &enabled); err != nil {
			return nil, err
		}
		digestMap[freq] = enabled
	}
	if err := digestRows.Err(); err != nil {
		return nil, err
	}

	return &api.ReportSubscriptions{
		Sites: siteSubscriptions,
		Digest: api.DigestSubscription{
			Daily:   digestMap["daily"],
			Weekly:  digestMap["weekly"],
			Monthly: digestMap["monthly"],
		},
	}, nil
}

// UpsertSiteReportSubscription inserts or updates a per-site subscription for the given frequency.
func (s *Store) UpsertSiteReportSubscription(ctx context.Context, userID, siteID uuid.UUID, freq api.ReportFrequency, enabled bool) error {
	now := time.Now().UTC()
	return s.Exec(ctx, `
		INSERT INTO site_report_subscriptions (id, user_id, site_id, frequency, enabled, created_at, updated_at)
		VALUES (uuidv7(), ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, site_id, frequency) DO UPDATE SET
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, userID, siteID, string(freq), enabled, now, now)
}

// UpsertDigestSubscription inserts or updates a digest subscription for the given frequency.
func (s *Store) UpsertDigestSubscription(ctx context.Context, userID uuid.UUID, freq api.ReportFrequency, enabled bool) error {
	now := time.Now().UTC()
	return s.Exec(ctx, `
		INSERT INTO digest_subscriptions (id, user_id, frequency, enabled, created_at, updated_at)
		VALUES (uuidv7(), ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, frequency) DO UPDATE SET
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, userID, string(freq), enabled, now, now)
}

// GetPendingSiteReports returns all active per-site report subscriptions for the given frequency,
// verifying that the user still has access to each site.
func (s *Store) GetPendingSiteReports(ctx context.Context, freq api.ReportFrequency) ([]PendingSiteReport, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT srs.user_id, u.email, srs.site_id, si.domain
		FROM site_report_subscriptions srs
		JOIN users u ON u.id = srs.user_id
		JOIN sites si ON si.id = srs.site_id
		WHERE srs.frequency = ?
		  AND srs.enabled = true
		  AND (
		      si.user_id = srs.user_id
		      OR EXISTS (
		          SELECT 1 FROM site_members sm
		          WHERE sm.site_id = srs.site_id AND sm.user_id = srs.user_id
		      )
		  )
	`, string(freq))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PendingSiteReport
	for rows.Next() {
		var r PendingSiteReport
		if err := rows.Scan(&r.UserID, &r.UserEmail, &r.SiteID, &r.Domain); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetDailyPageviewsForPeriod returns daily pageview counts from hit_rollups_daily
// for the given site over [start, end), ordered oldest-first.
func (s *Store) GetDailyPageviewsForPeriod(ctx context.Context, siteID uuid.UUID, start, end time.Time) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pageviews
		FROM hit_rollups_daily
		WHERE site_id = ?
		  AND bucket >= ? AND bucket < ?
		ORDER BY bucket ASC
	`, siteID, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("query daily pageviews: %w", err)
	}
	defer rows.Close()

	var result []int
	for rows.Next() {
		var pv int
		if err := rows.Scan(&pv); err != nil {
			return nil, fmt.Errorf("scan daily pageviews: %w", err)
		}
		result = append(result, pv)
	}
	return result, rows.Err()
}

// GetPendingDigests returns all users with an active digest subscription for the given frequency,
// along with all sites they have access to.
func (s *Store) GetPendingDigests(ctx context.Context, freq api.ReportFrequency) ([]PendingDigest, error) {
	// Get subscribed users.
	userRows, err := s.db.QueryContext(ctx, `
		SELECT ds.user_id, u.email
		FROM digest_subscriptions ds
		JOIN users u ON u.id = ds.user_id
		WHERE ds.frequency = ? AND ds.enabled = true
	`, string(freq))
	if err != nil {
		return nil, err
	}
	defer userRows.Close()

	type subscribedUser struct {
		userID uuid.UUID
		email  string
	}
	var users []subscribedUser
	for userRows.Next() {
		var su subscribedUser
		if err := userRows.Scan(&su.userID, &su.email); err != nil {
			return nil, err
		}
		users = append(users, su)
	}
	if err := userRows.Err(); err != nil {
		return nil, err
	}
	userRows.Close()

	result := make([]PendingDigest, 0, len(users))
	for _, u := range users {
		siteRows, err := s.db.QueryContext(ctx, `
			SELECT DISTINCT s.id, s.domain
			FROM sites s
			LEFT JOIN site_members sm ON sm.site_id = s.id AND sm.user_id = ?
			WHERE s.user_id = ? OR sm.user_id IS NOT NULL
			ORDER BY s.domain ASC
		`, u.userID, u.userID)
		if err != nil {
			return nil, err
		}

		var sites []DigestSite
		for siteRows.Next() {
			var ds DigestSite
			if err := siteRows.Scan(&ds.SiteID, &ds.Domain); err != nil {
				siteRows.Close()
				return nil, err
			}
			sites = append(sites, ds)
		}
		siteRows.Close()
		if err := siteRows.Err(); err != nil {
			return nil, err
		}

		if len(sites) > 0 {
			result = append(result, PendingDigest{
				UserID:    u.userID,
				UserEmail: u.email,
				Sites:     sites,
			})
		}
	}
	return result, nil
}
