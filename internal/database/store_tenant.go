package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

const (
	defaultTenantName = "Default Tenant"
	defaultLocaleCode = "en"

	TenantRoleOwner  = "owner"
	TenantRoleAdmin  = "admin"
	TenantRoleMember = "member"
)

var ErrTenantMembershipRequired = errors.New("tenant membership required")
var ErrTeamLastOwner = errors.New("team must retain at least one owner")
var ErrUserOnlyTeam = errors.New("user cannot leave their only team")
var ErrTeamInviteAlreadyPending = errors.New("team invite already pending")
var ErrTeamInviteNotFound = errors.New("team invite not found")
var ErrTeamTransferRequiresOwner = errors.New("team ownership transfer requires owner")
var ErrTeamTransferSelf = errors.New("cannot transfer ownership to self")
var ErrTeamTransferTargetNotMember = errors.New("ownership transfer target must be a team member")
var ErrTeamTransferTargetAlreadyOwner = errors.New("ownership transfer target is already owner")

const (
	TeamInviteStatusPending  = "pending"
	TeamInviteStatusAccepted = "accepted"
	TeamInviteStatusRevoked  = "revoked"
)

type tenantRowQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func (s *Store) GetPrimaryTenantID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	return getPrimaryTenantID(ctx, s.db, userID)
}

func (s *Store) GetActiveTenantID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	return getActiveTenantID(ctx, s.db, userID)
}

func (s *Store) GetDefaultTenantID(ctx context.Context) (uuid.UUID, error) {
	return getDefaultTenantID(ctx, s.db)
}

func (s *Store) GetSiteTenantID(ctx context.Context, siteID uuid.UUID) (uuid.UUID, error) {
	var tenantIDRaw sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT CAST(st.tenant_id AS VARCHAR)
		FROM sites s
		LEFT JOIN site_tenants st ON st.site_id = s.id
		WHERE s.id = ?
	`, siteID).Scan(&tenantIDRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("site not found")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not query site tenant: %w", err)
	}

	if !tenantIDRaw.Valid || strings.TrimSpace(tenantIDRaw.String) == "" {
		return getDefaultTenantID(ctx, s.db)
	}

	tenantID, err := uuid.Parse(tenantIDRaw.String)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid tenant id %q: %w", tenantIDRaw.String, err)
	}
	return tenantID, nil
}

func (s *Store) IsTenantMember(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
		tenantID, userID,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("could not query tenant membership: %w", err)
	}
	return count > 0, nil
}

func (s *Store) GetTenantRole(ctx context.Context, tenantID, userID uuid.UUID) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		"SELECT role FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
		tenantID, userID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no access to tenant")
	}
	if err != nil {
		return "", fmt.Errorf("could not query tenant role: %w", err)
	}
	return strings.TrimSpace(role), nil
}

func (s *Store) SetActiveTenantID(ctx context.Context, userID, tenantID uuid.UUID) error {
	isMember, err := s.IsTenantMember(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return fmt.Errorf("could not set active tenant: %w", ErrTenantMembershipRequired)
	}

	var currentLocale sql.NullString
	if err := s.QueryRowOrNil(ctx, "SELECT default_locale FROM user_preferences WHERE user_id = ?", []any{&currentLocale}, userID); err != nil {
		return fmt.Errorf("could not query user preferences: %w", err)
	}

	locale := strings.TrimSpace(currentLocale.String)
	if locale == "" {
		locale = defaultLocaleCode
	}
	now := time.Now().UTC()
	err = s.Exec(ctx, `
		INSERT INTO user_preferences (user_id, default_locale, updated_at, active_tenant_id)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			active_tenant_id = excluded.active_tenant_id,
			updated_at = excluded.updated_at
	`, userID, locale, now, tenantID)
	if err != nil {
		return fmt.Errorf("could not set active tenant: %w", err)
	}
	return nil
}

func (s *Store) ListUserTeams(ctx context.Context, userID uuid.UUID) ([]api.Team, uuid.UUID, error) {
	activeTenantID, err := s.GetActiveTenantID(ctx, userID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("could not resolve active team: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.name, COALESCE(t.logo_url, ''), tm.role, t.created_at
		FROM tenants t
		JOIN tenant_members tm ON tm.tenant_id = t.id
		WHERE tm.user_id = ?
		ORDER BY
			CASE tm.role
				WHEN 'owner' THEN 0
				WHEN 'admin' THEN 1
				ELSE 2
			END ASC,
			t.created_at ASC
	`, userID)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("could not list teams: %w", err)
	}
	defer rows.Close()

	teams := make([]api.Team, 0)
	for rows.Next() {
		var team api.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.LogoURL, &team.Role, &team.CreatedAt); err != nil {
			return nil, uuid.Nil, fmt.Errorf("could not scan team: %w", err)
		}
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		return nil, uuid.Nil, fmt.Errorf("could not read team rows: %w", err)
	}

	return teams, activeTenantID, nil
}

func (s *Store) ListTeamMembers(ctx context.Context, tenantID uuid.UUID) ([]api.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tm.id, tm.user_id, u.email, tm.role, tm.added_at
		FROM tenant_members tm
		JOIN users u ON u.id = tm.user_id
		WHERE tm.tenant_id = ?
		ORDER BY
			CASE tm.role
				WHEN 'owner' THEN 0
				WHEN 'admin' THEN 1
				ELSE 2
			END ASC,
			tm.added_at ASC
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("could not list team members: %w", err)
	}
	defer rows.Close()

	members := make([]api.TeamMember, 0)
	for rows.Next() {
		var member api.TeamMember
		if err := rows.Scan(&member.ID, &member.UserID, &member.Email, &member.Role, &member.AddedAt); err != nil {
			return nil, fmt.Errorf("could not scan team member: %w", err)
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read team member rows: %w", err)
	}

	return members, nil
}

func (s *Store) AddTeamMember(ctx context.Context, tenantID, userID uuid.UUID, role string, addedBy uuid.UUID) error {
	role = strings.TrimSpace(strings.ToLower(role))
	if !IsValidTenantRole(role) {
		return fmt.Errorf("invalid tenant role")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenant_members (tenant_id, user_id, role, added_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (tenant_id, user_id) DO UPDATE SET
			role = excluded.role,
			added_by = excluded.added_by,
			added_at = NOW()
	`, tenantID, userID, role, nullableUUID(addedBy))
	if err != nil {
		return fmt.Errorf("could not add tenant member: %w", err)
	}

	return nil
}

func (s *Store) CreateTeamInvite(
	ctx context.Context,
	tenantID uuid.UUID,
	email string,
	role string,
	invitedUserID *uuid.UUID,
	createdBy uuid.UUID,
) (*api.TeamInvite, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	role = strings.TrimSpace(strings.ToLower(role))
	if email == "" {
		return nil, fmt.Errorf("invite email is required")
	}
	if !IsValidTenantRole(role) {
		return nil, fmt.Errorf("invalid tenant role")
	}

	invite := &api.TeamInvite{
		ID:        uuid.New(),
		TeamID:    tenantID,
		Email:     email,
		Role:      role,
		Status:    TeamInviteStatusPending,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
	}
	if invitedUserID != nil && *invitedUserID != uuid.Nil {
		userID := *invitedUserID
		invite.InvitedUserID = &userID
	}
	if createdBy != uuid.Nil {
		actorID := createdBy
		invite.CreatedBy = &actorID
	}

	err := s.Transact(ctx, func(tx *sql.Tx) error {
		var existingID uuid.UUID
		err := tx.QueryRowContext(ctx, `
			SELECT id
			FROM team_invites
			WHERE tenant_id = ? AND lower(email) = lower(?) AND status = ?
			ORDER BY created_at DESC
			LIMIT 1
		`, tenantID, email, TeamInviteStatusPending).Scan(&existingID)
		if err == nil {
			return ErrTeamInviteAlreadyPending
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("could not query pending invite: %w", err)
		}

		var invitedUserValue any
		if invite.InvitedUserID != nil {
			invitedUserValue = *invite.InvitedUserID
		}
		var createdByValue any
		if invite.CreatedBy != nil {
			createdByValue = *invite.CreatedBy
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO team_invites (
				id,
				tenant_id,
				email,
				role,
				invited_user_id,
				status,
				created_by,
				created_at,
				expires_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, invite.ID, invite.TeamID, invite.Email, invite.Role, invitedUserValue, invite.Status, createdByValue, invite.CreatedAt, invite.ExpiresAt); err != nil {
			return fmt.Errorf("could not insert team invite: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return invite, nil
}

func (s *Store) ListTeamInvites(ctx context.Context, tenantID uuid.UUID) ([]api.TeamInvite, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			tenant_id,
			email,
			role,
			CAST(invited_user_id AS VARCHAR),
			status,
			CAST(created_by AS VARCHAR),
			created_at,
			expires_at,
			accepted_at,
			revoked_at
		FROM team_invites
		WHERE tenant_id = ? AND status = ?
		ORDER BY created_at DESC
	`, tenantID, TeamInviteStatusPending)
	if err != nil {
		return nil, fmt.Errorf("could not list team invites: %w", err)
	}
	defer rows.Close()

	invites := make([]api.TeamInvite, 0)
	for rows.Next() {
		invite, err := scanTeamInvite(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, invite)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read team invites: %w", err)
	}

	return invites, nil
}

func (s *Store) GetTeamInvite(ctx context.Context, tenantID, inviteID uuid.UUID) (*api.TeamInvite, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			id,
			tenant_id,
			email,
			role,
			CAST(invited_user_id AS VARCHAR),
			status,
			CAST(created_by AS VARCHAR),
			created_at,
			expires_at,
			accepted_at,
			revoked_at
		FROM team_invites
		WHERE tenant_id = ? AND id = ?
		LIMIT 1
	`, tenantID, inviteID)

	invite, err := scanTeamInviteRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTeamInviteNotFound
	}
	if err != nil {
		return nil, err
	}
	return invite, nil
}

func (s *Store) ResendTeamInvite(ctx context.Context, tenantID, inviteID uuid.UUID) (*api.TeamInvite, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(7 * 24 * time.Hour)

	err := s.Transact(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE team_invites
			SET created_at = ?, expires_at = ?
			WHERE tenant_id = ? AND id = ? AND status = ?
		`, now, expiresAt, tenantID, inviteID, TeamInviteStatusPending)
		if err != nil {
			return fmt.Errorf("could not update team invite: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("could not update team invite: %w", err)
		}
		if rowsAffected == 0 {
			return ErrTeamInviteNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetTeamInvite(ctx, tenantID, inviteID)
}

func (s *Store) RevokeTeamInvite(ctx context.Context, tenantID, inviteID uuid.UUID) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE team_invites
		SET status = ?, revoked_at = ?
		WHERE tenant_id = ? AND id = ? AND status = ?
	`, TeamInviteStatusRevoked, now, tenantID, inviteID, TeamInviteStatusPending)
	if err != nil {
		return fmt.Errorf("could not revoke team invite: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not revoke team invite: %w", err)
	}
	if rowsAffected == 0 {
		return ErrTeamInviteNotFound
	}
	return nil
}

func (s *Store) AcceptTeamInvitesByEmail(ctx context.Context, email string, userID uuid.UUID) ([]api.TeamInvite, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, fmt.Errorf("invite email is required")
	}

	accepted := make([]api.TeamInvite, 0)
	err := s.Transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT
				id,
				tenant_id,
				email,
				role,
				CAST(invited_user_id AS VARCHAR),
				status,
				CAST(created_by AS VARCHAR),
				created_at,
				expires_at,
				accepted_at,
				revoked_at
			FROM team_invites
			WHERE lower(email) = lower(?) AND status = ?
			ORDER BY created_at ASC
		`, email, TeamInviteStatusPending)
		if err != nil {
			return fmt.Errorf("could not query pending invites: %w", err)
		}
		defer rows.Close()

		now := time.Now().UTC()
		for rows.Next() {
			invite, err := scanTeamInvite(rows)
			if err != nil {
				return err
			}
			if invite.ExpiresAt.Before(now) {
				if _, err := tx.ExecContext(ctx, `
					UPDATE team_invites
					SET status = ?, revoked_at = ?
					WHERE id = ?
				`, TeamInviteStatusRevoked, now, invite.ID); err != nil {
					return fmt.Errorf("could not expire team invite: %w", err)
				}
				continue
			}

			createdBy := uuid.Nil
			if invite.CreatedBy != nil {
				createdBy = *invite.CreatedBy
			}
			if err := ensureTenantMemberTx(ctx, tx, invite.TeamID, userID, invite.Role, createdBy); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE team_invites
				SET invited_user_id = ?, status = ?, accepted_at = ?
				WHERE id = ?
			`, userID, TeamInviteStatusAccepted, now, invite.ID); err != nil {
				return fmt.Errorf("could not accept team invite: %w", err)
			}
			userIDCopy := userID
			invite.InvitedUserID = &userIDCopy
			invite.Status = TeamInviteStatusAccepted
			invite.AcceptedAt = &now
			accepted = append(accepted, invite)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("could not read pending invites: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return accepted, nil
}

func (s *Store) RemoveTeamMember(ctx context.Context, tenantID, userID uuid.UUID) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?", tenantID, userID)
	if err != nil {
		return fmt.Errorf("could not remove tenant member: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not remove tenant member: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("tenant member not found")
	}
	return nil
}

func (s *Store) LeaveTeam(ctx context.Context, tenantID, userID uuid.UUID) (uuid.UUID, error) {
	nextActiveTenantID := uuid.Nil

	err := s.Transact(ctx, func(tx *sql.Tx) error {
		var role string
		if err := tx.QueryRowContext(ctx,
			"SELECT role FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
			tenantID, userID,
		).Scan(&role); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTenantMembershipRequired
			}
			return fmt.Errorf("could not resolve tenant membership: %w", err)
		}

		var userTeamCount int
		if err := tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM tenant_members WHERE user_id = ?",
			userID,
		).Scan(&userTeamCount); err != nil {
			return fmt.Errorf("could not count user teams: %w", err)
		}
		if userTeamCount <= 1 {
			return ErrUserOnlyTeam
		}

		if strings.EqualFold(strings.TrimSpace(role), TenantRoleOwner) {
			ownerCount, err := countTeamOwnersTx(ctx, tx, tenantID)
			if err != nil {
				return err
			}
			if ownerCount <= 1 {
				return ErrTeamLastOwner
			}
		}

		if _, err := tx.ExecContext(ctx,
			"DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
			tenantID, userID,
		); err != nil {
			return fmt.Errorf("could not remove tenant member: %w", err)
		}

		currentActiveTenantID, err := getActiveTenantID(ctx, tx, userID)
		if err != nil {
			currentActiveTenantID = uuid.Nil
		}
		nextActiveTenantID = currentActiveTenantID

		if currentActiveTenantID == uuid.Nil || currentActiveTenantID == tenantID {
			replacementTenantID, err := getPrimaryTenantID(ctx, tx, userID)
			if err != nil {
				return fmt.Errorf("could not resolve replacement team: %w", err)
			}
			nextActiveTenantID = replacementTenantID
		}

		locale, err := getUserLocaleTx(ctx, tx, userID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_preferences (user_id, default_locale, updated_at, active_tenant_id)
			VALUES (?, ?, ?, ?)
			ON CONFLICT (user_id) DO UPDATE SET
				active_tenant_id = excluded.active_tenant_id,
				updated_at = excluded.updated_at
		`, userID, locale, now, nextActiveTenantID); err != nil {
			return fmt.Errorf("could not update active team after leave: %w", err)
		}

		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	return nextActiveTenantID, nil
}

func (s *Store) TransferTeamOwnership(ctx context.Context, tenantID, actorID, targetUserID uuid.UUID) error {
	if actorID == targetUserID {
		return ErrTeamTransferSelf
	}

	return s.Transact(ctx, func(tx *sql.Tx) error {
		var actorRole string
		if err := tx.QueryRowContext(ctx,
			"SELECT role FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
			tenantID, actorID,
		).Scan(&actorRole); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTenantMembershipRequired
			}
			return fmt.Errorf("could not resolve actor role: %w", err)
		}
		if actorRole != TenantRoleOwner {
			return ErrTeamTransferRequiresOwner
		}

		var targetRole string
		if err := tx.QueryRowContext(ctx,
			"SELECT role FROM tenant_members WHERE tenant_id = ? AND user_id = ?",
			tenantID, targetUserID,
		).Scan(&targetRole); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTeamTransferTargetNotMember
			}
			return fmt.Errorf("could not resolve target role: %w", err)
		}
		if targetRole == TenantRoleOwner {
			return ErrTeamTransferTargetAlreadyOwner
		}

		if _, err := tx.ExecContext(ctx,
			"UPDATE tenant_members SET role = ? WHERE tenant_id = ? AND user_id = ?",
			TenantRoleOwner, tenantID, targetUserID,
		); err != nil {
			return fmt.Errorf("could not promote target owner: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"UPDATE tenant_members SET role = ? WHERE tenant_id = ? AND user_id = ?",
			TenantRoleAdmin, tenantID, actorID,
		); err != nil {
			return fmt.Errorf("could not demote current owner: %w", err)
		}

		return nil
	})
}

func (s *Store) CountTeamOwners(ctx context.Context, tenantID uuid.UUID) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tenant_members WHERE tenant_id = ? AND role = ?",
		tenantID, TenantRoleOwner,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("could not count tenant owners: %w", err)
	}
	return count, nil
}

func (s *Store) AppendTeamAuditEntry(ctx context.Context, tenantID, actorID uuid.UUID, action, details string, targetUserID *uuid.UUID) error {
	action = strings.TrimSpace(action)
	if action == "" {
		return fmt.Errorf("team audit action is required")
	}

	details = strings.TrimSpace(details)
	var targetArg any
	if targetUserID != nil && *targetUserID != uuid.Nil {
		targetArg = *targetUserID
	}

	if err := s.Exec(ctx, `
		INSERT INTO team_audit_log (tenant_id, actor_id, target_user_id, action, details)
		VALUES (?, ?, ?, ?, ?)
	`, tenantID, nullableUUID(actorID), targetArg, action, details); err != nil {
		return fmt.Errorf("could not append team audit entry: %w", err)
	}
	return nil
}

func (s *Store) ListTeamAuditEntries(ctx context.Context, tenantID uuid.UUID, limit int) ([]api.TeamAuditEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			ta.id,
			ta.tenant_id,
			ta.action,
			ta.details,
			ta.created_at,
			CAST(ta.actor_id AS VARCHAR),
			COALESCE(actor.email, ''),
			CAST(ta.target_user_id AS VARCHAR),
			COALESCE(target.email, '')
		FROM team_audit_log ta
		LEFT JOIN users actor ON actor.id = ta.actor_id
		LEFT JOIN users target ON target.id = ta.target_user_id
		WHERE ta.tenant_id = ?
		ORDER BY ta.created_at DESC
		LIMIT ?
	`, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("could not list team audit entries: %w", err)
	}
	defer rows.Close()

	entries := make([]api.TeamAuditEntry, 0, limit)
	for rows.Next() {
		var entry api.TeamAuditEntry
		var actorIDRaw sql.NullString
		var targetIDRaw sql.NullString
		if err := rows.Scan(
			&entry.ID,
			&entry.TeamID,
			&entry.Action,
			&entry.Details,
			&entry.CreatedAt,
			&actorIDRaw,
			&entry.ActorEmail,
			&targetIDRaw,
			&entry.TargetEmail,
		); err != nil {
			return nil, fmt.Errorf("could not scan team audit entry: %w", err)
		}

		if actorIDRaw.Valid && strings.TrimSpace(actorIDRaw.String) != "" {
			if actorID, err := uuid.Parse(actorIDRaw.String); err == nil {
				entry.ActorUserID = &actorID
			}
		}
		if targetIDRaw.Valid && strings.TrimSpace(targetIDRaw.String) != "" {
			if targetID, err := uuid.Parse(targetIDRaw.String); err == nil {
				entry.TargetUserID = &targetID
			}
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read team audit entries: %w", err)
	}

	return entries, nil
}

func (s *Store) GetTenant(ctx context.Context, tenantID uuid.UUID) (*api.Team, error) {
	var team api.Team
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(logo_url, ''), created_at FROM tenants WHERE id = ? LIMIT 1",
		tenantID,
	).Scan(&team.ID, &team.Name, &team.LogoURL, &team.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not query tenant: %w", err)
	}
	return &team, nil
}

func (s *Store) UpdateTenant(ctx context.Context, tenantID uuid.UUID, name, logoURL string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("team name cannot be empty")
	}
	logoURL = strings.TrimSpace(logoURL)
	_, err := s.db.ExecContext(ctx,
		"UPDATE tenants SET name = ?, logo_url = ? WHERE id = ?",
		name, logoURL, tenantID,
	)
	if err != nil {
		return fmt.Errorf("could not update tenant: %w", err)
	}
	return nil
}

func (s *Store) CreateTenant(ctx context.Context, creatorID uuid.UUID, name, logoURL string) (*api.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("team name cannot be empty")
	}
	logoURL = strings.TrimSpace(logoURL)

	var team api.Team
	err := s.Transact(ctx, func(tx *sql.Tx) error {
		tenantID := uuid.New()
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO tenants (id, name, logo_url, created_at) VALUES (?, ?, ?, ?)",
			tenantID, name, logoURL, now,
		); err != nil {
			return fmt.Errorf("could not insert tenant: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO tenant_members (tenant_id, user_id, role, added_by) VALUES (?, ?, ?, ?)",
			tenantID, creatorID, TenantRoleOwner, creatorID,
		); err != nil {
			return fmt.Errorf("could not insert tenant owner: %w", err)
		}

		team = api.Team{
			ID:        tenantID,
			Name:      name,
			LogoURL:   logoURL,
			Role:      TenantRoleOwner,
			CreatedAt: now,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not create tenant: %w", err)
	}
	return &team, nil
}

func (s *Store) ListNonDefaultTenantIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT CAST(id AS VARCHAR) FROM tenants WHERE is_default = FALSE ORDER BY created_at",
	)
	if err != nil {
		return nil, fmt.Errorf("could not list non-default tenants: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("could not scan tenant id: %w", err)
		}
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid tenant id %q: %w", raw, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not read tenant rows: %w", err)
	}
	return ids, nil
}

func ensureDefaultTenantTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO tenants (name, is_default)
		SELECT ?, TRUE
		WHERE NOT EXISTS (
			SELECT 1
			FROM tenants
			WHERE is_default = TRUE
		)
	`, defaultTenantName)
	if err != nil {
		return fmt.Errorf("could not ensure default tenant: %w", err)
	}
	return nil
}

func ensureTenantMemberTx(ctx context.Context, tx *sql.Tx, tenantID, userID uuid.UUID, role string, addedBy uuid.UUID) error {
	role = strings.TrimSpace(role)
	if role == "" {
		role = TenantRoleMember
	}

	var addedByValue any
	if addedBy != uuid.Nil {
		addedByValue = addedBy
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO tenant_members (tenant_id, user_id, role, added_by)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (tenant_id, user_id) DO NOTHING
	`, tenantID, userID, role, addedByValue)
	if err != nil {
		return fmt.Errorf("could not ensure tenant membership: %w", err)
	}
	return nil
}

func IsValidTenantRole(role string) bool {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case TenantRoleOwner, TenantRoleAdmin, TenantRoleMember:
		return true
	default:
		return false
	}
}

func CanAssignTenantRole(actorRole, requestedRole string) bool {
	if !IsValidTenantRole(actorRole) || !IsValidTenantRole(requestedRole) {
		return false
	}
	return tenantRoleRank(requestedRole) >= tenantRoleRank(actorRole)
}

func tenantRoleRank(role string) int {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case TenantRoleOwner:
		return 0
	case TenantRoleAdmin:
		return 1
	case TenantRoleMember:
		return 2
	default:
		return 99
	}
}

func getPrimaryTenantID(ctx context.Context, q tenantRowQueryer, userID uuid.UUID) (uuid.UUID, error) {
	var tenantIDRaw sql.NullString
	err := q.QueryRowContext(ctx, `
		SELECT CAST(tenant_id AS VARCHAR)
		FROM tenant_members
		WHERE user_id = ?
		ORDER BY
			CASE role
				WHEN 'owner' THEN 0
				WHEN 'admin' THEN 1
				ELSE 2
			END ASC,
			added_at ASC
		LIMIT 1
	`, userID).Scan(&tenantIDRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return getDefaultTenantID(ctx, q)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not query primary tenant: %w", err)
	}

	if !tenantIDRaw.Valid || strings.TrimSpace(tenantIDRaw.String) == "" {
		return getDefaultTenantID(ctx, q)
	}

	tenantID, err := uuid.Parse(tenantIDRaw.String)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid tenant id %q: %w", tenantIDRaw.String, err)
	}
	return tenantID, nil
}

func getActiveTenantID(ctx context.Context, q tenantRowQueryer, userID uuid.UUID) (uuid.UUID, error) {
	var tenantIDRaw sql.NullString
	err := q.QueryRowContext(ctx, `
		SELECT CAST(up.active_tenant_id AS VARCHAR)
		FROM user_preferences up
		JOIN tenant_members tm
			ON tm.tenant_id = up.active_tenant_id
			AND tm.user_id = ?
		WHERE up.user_id = ?
		LIMIT 1
	`, userID, userID).Scan(&tenantIDRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return getPrimaryTenantID(ctx, q, userID)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not query active tenant: %w", err)
	}
	if !tenantIDRaw.Valid || strings.TrimSpace(tenantIDRaw.String) == "" {
		return getPrimaryTenantID(ctx, q, userID)
	}

	tenantID, err := uuid.Parse(tenantIDRaw.String)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid active tenant id %q: %w", tenantIDRaw.String, err)
	}
	return tenantID, nil
}

func getDefaultTenantID(ctx context.Context, q tenantRowQueryer) (uuid.UUID, error) {
	var tenantIDRaw sql.NullString
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS VARCHAR) FROM tenants WHERE is_default = TRUE LIMIT 1").Scan(&tenantIDRaw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		// The is_default column may not exist yet (pre-migration). Fall back
		// to the oldest tenant which is the implicit default in that state.
		if isBinderError(err) {
			return getFirstTenantID(ctx, q)
		}
		return uuid.Nil, fmt.Errorf("could not query default tenant: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) || !tenantIDRaw.Valid || strings.TrimSpace(tenantIDRaw.String) == "" {
		return uuid.Nil, fmt.Errorf("default tenant not found")
	}

	tenantID, err := uuid.Parse(tenantIDRaw.String)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid default tenant id %q: %w", tenantIDRaw.String, err)
	}
	return tenantID, nil
}

// getFirstTenantID returns the oldest tenant by created_at. Used as a fallback
// when the is_default column doesn't exist yet (pre-migration schema).
func getFirstTenantID(ctx context.Context, q tenantRowQueryer) (uuid.UUID, error) {
	var tenantIDRaw sql.NullString
	err := q.QueryRowContext(ctx, "SELECT CAST(id AS VARCHAR) FROM tenants ORDER BY created_at ASC LIMIT 1").Scan(&tenantIDRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("default tenant not found")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not query first tenant: %w", err)
	}
	if !tenantIDRaw.Valid || strings.TrimSpace(tenantIDRaw.String) == "" {
		return uuid.Nil, fmt.Errorf("first tenant id is empty")
	}

	tenantID, err := uuid.Parse(tenantIDRaw.String)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid first tenant id %q: %w", tenantIDRaw.String, err)
	}
	return tenantID, nil
}

// isBinderError returns true when DuckDB reports a Binder Error, typically
// because a referenced column or table doesn't exist in the current schema.
func isBinderError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "binder error")
}

func countTeamOwnersTx(ctx context.Context, tx *sql.Tx, tenantID uuid.UUID) (int, error) {
	var count int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM tenant_members WHERE tenant_id = ? AND role = ?",
		tenantID, TenantRoleOwner,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("could not count tenant owners: %w", err)
	}
	return count, nil
}

func scanTeamInvite(scanner interface {
	Scan(dest ...any) error
}) (api.TeamInvite, error) {
	var invite api.TeamInvite
	var invitedUserRaw sql.NullString
	var createdByRaw sql.NullString
	var acceptedAt sql.NullTime
	var revokedAt sql.NullTime
	if err := scanner.Scan(
		&invite.ID,
		&invite.TeamID,
		&invite.Email,
		&invite.Role,
		&invitedUserRaw,
		&invite.Status,
		&createdByRaw,
		&invite.CreatedAt,
		&invite.ExpiresAt,
		&acceptedAt,
		&revokedAt,
	); err != nil {
		return api.TeamInvite{}, fmt.Errorf("could not scan team invite: %w", err)
	}

	if invitedUserRaw.Valid && strings.TrimSpace(invitedUserRaw.String) != "" {
		invitedUserID, err := uuid.Parse(invitedUserRaw.String)
		if err != nil {
			return api.TeamInvite{}, fmt.Errorf("invalid invited user id %q: %w", invitedUserRaw.String, err)
		}
		invite.InvitedUserID = &invitedUserID
	}
	if createdByRaw.Valid && strings.TrimSpace(createdByRaw.String) != "" {
		createdBy, err := uuid.Parse(createdByRaw.String)
		if err != nil {
			return api.TeamInvite{}, fmt.Errorf("invalid created by id %q: %w", createdByRaw.String, err)
		}
		invite.CreatedBy = &createdBy
	}
	if acceptedAt.Valid {
		accepted := acceptedAt.Time
		invite.AcceptedAt = &accepted
	}
	if revokedAt.Valid {
		revoked := revokedAt.Time
		invite.RevokedAt = &revoked
	}

	return invite, nil
}

func scanTeamInviteRow(row *sql.Row) (*api.TeamInvite, error) {
	invite, err := scanTeamInvite(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}
	return &invite, nil
}

func getUserLocaleTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (string, error) {
	var locale sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT default_locale FROM user_preferences WHERE user_id = ?", userID).Scan(&locale)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("could not query user locale: %w", err)
	}

	out := strings.TrimSpace(locale.String)
	if out == "" {
		out = defaultLocaleCode
	}
	return out, nil
}
