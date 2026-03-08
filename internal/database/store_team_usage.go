package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func (s *Store) CountTeamMembers(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM tenant_members
		WHERE tenant_id = ?
	`, tenantID).Scan(&count); err != nil {
		return 0, fmt.Errorf("could not count team members: %w", err)
	}
	return count, nil
}

func (s *Store) CountPendingTeamInvites(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM team_invites
		WHERE tenant_id = ? AND status = ?
	`, tenantID, TeamInviteStatusPending).Scan(&count); err != nil {
		return 0, fmt.Errorf("could not count pending team invites: %w", err)
	}
	return count, nil
}

func (s *Store) CountMonthlyIngestedEventsForSites(ctx context.Context, siteIDs []uuid.UUID, start, end time.Time) (int64, error) {
	if len(siteIDs) == 0 {
		return 0, nil
	}

	hitsCount, err := s.countMonthlyRowsForSites(ctx, "hits", siteIDs, start, end)
	if err != nil {
		return 0, err
	}

	customEventCount, err := s.countMonthlyRowsForSites(ctx, "events", siteIDs, start, end)
	if err != nil {
		return 0, err
	}

	return hitsCount + customEventCount, nil
}

func (s *Store) countMonthlyRowsForSites(ctx context.Context, table string, siteIDs []uuid.UUID, start, end time.Time) (int64, error) {
	args := make([]any, 0, 2+len(siteIDs))
	args = append(args, start.UTC(), end.UTC())
	inClause := "(?"
	for idx, siteID := range siteIDs {
		if idx > 0 {
			inClause += ", ?"
		}
		args = append(args, siteID)
	}
	inClause += ")"

	var query string
	switch table {
	case "hits":
		query = `
			SELECT COUNT(*)
			FROM hits
			WHERE timestamp >= ? AND timestamp < ?
				AND site_id IN ` + inClause
	case "events":
		query = `
			SELECT COUNT(*)
			FROM events
			WHERE timestamp >= ? AND timestamp < ?
				AND site_id IN ` + inClause
	default:
		return 0, fmt.Errorf("unsupported monthly usage table %q", table)
	}

	var count sql.NullInt64
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("could not count monthly rows in %s: %w", table, err)
	}
	return count.Int64, nil
}

func monthBoundsUTC(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}

func (s *Store) BuildTeamUsageSummary(ctx context.Context, tenantID uuid.UUID, analyticsStore *Store) (*api.TeamUsageSummary, error) {
	sites, err := s.ListSitesForTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	memberCount, err := s.CountTeamMembers(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	pendingInviteCount, err := s.CountPendingTeamInvites(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	siteIDs := make([]uuid.UUID, 0, len(sites))
	for _, site := range sites {
		siteIDs = append(siteIDs, site.ID)
	}
	monthStart, monthEnd := monthBoundsUTC(time.Now())
	monthlyEvents, err := analyticsStore.CountMonthlyIngestedEventsForSites(ctx, siteIDs, monthStart, monthEnd)
	if err != nil {
		return nil, err
	}

	return &api.TeamUsageSummary{
		CurrentSites:          len(sites),
		CurrentMembers:        memberCount,
		CurrentPendingInvites: pendingInviteCount,
		CurrentMonthlyEvents:  monthlyEvents,
	}, nil
}
