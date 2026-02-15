package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/auth"
)

var ErrAPIClientNotFound = errors.New("api client not found")

type APIClientAuth struct {
	ClientID     uuid.UUID
	UserID       uuid.UUID
	InstanceRole auth.InstanceRole
	SiteRoles    map[uuid.UUID]auth.SiteRole
}

func (s *Store) CreateAPIClient(
	ctx context.Context,
	userID uuid.UUID,
	name string,
	description string,
	instanceRole auth.InstanceRole,
	siteRoles map[uuid.UUID]auth.SiteRole,
	expiresAt *time.Time,
) (*api.APIClient, string, error) {
	id := uuid.New()
	now := time.Now().UTC()
	token, tokenHash, err := generateAPIClientToken()
	if err != nil {
		return nil, "", err
	}

	description = strings.TrimSpace(description)
	exp := toUTCPtr(expiresAt)

	err = s.Transact(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO api_clients (
				id, user_id, name, description, secret_hash, instance_role, expires_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, userID, name, description, tokenHash, instanceRole, exp, now, now)
		if err != nil {
			return fmt.Errorf("failed to create api client: %w", err)
		}

		if err := replaceAPIClientSiteRolesTx(ctx, tx, id, siteRoles, now); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, "", err
	}

	client := api.APIClient{
		ID:           id,
		UserID:       userID,
		Name:         name,
		Description:  description,
		InstanceRole: string(instanceRole),
		ExpiresAt:    exp,
		CreatedAt:    now,
		UpdatedAt:    now,
		SiteRoles:    flattenSiteRoles(siteRoles),
	}
	return &client, token, nil
}

func (s *Store) ListAPIClients(ctx context.Context, userID uuid.UUID) ([]api.APIClient, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, description, instance_role, expires_at, last_used_at, revoked_at, created_at, updated_at
		FROM api_clients
		WHERE user_id = ?
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list api clients: %w", err)
	}
	defer rows.Close()

	clients := make([]api.APIClient, 0)
	indexByID := make(map[uuid.UUID]int)
	for rows.Next() {
		var client api.APIClient
		var description sql.NullString
		var expiresAt sql.NullTime
		var lastUsedAt sql.NullTime
		var revokedAt sql.NullTime

		if err := rows.Scan(
			&client.ID,
			&client.UserID,
			&client.Name,
			&description,
			&client.InstanceRole,
			&expiresAt,
			&lastUsedAt,
			&revokedAt,
			&client.CreatedAt,
			&client.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan api client: %w", err)
		}

		client.Description = strings.TrimSpace(description.String)
		client.ExpiresAt = nullTimePtr(expiresAt)
		client.LastUsedAt = nullTimePtr(lastUsedAt)
		client.RevokedAt = nullTimePtr(revokedAt)
		client.SiteRoles = make([]api.APIClientSiteRole, 0)
		indexByID[client.ID] = len(clients)
		clients = append(clients, client)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating api clients: %w", err)
	}

	if len(clients) == 0 {
		return clients, nil
	}

	roleRows, err := s.db.QueryContext(ctx, `
		SELECT r.api_client_id, r.site_id, r.role
		FROM api_client_site_roles r
		JOIN api_clients c ON c.id = r.api_client_id
		WHERE c.user_id = ?
		ORDER BY r.api_client_id, r.site_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list api client site roles: %w", err)
	}
	defer roleRows.Close()

	for roleRows.Next() {
		var clientID uuid.UUID
		var siteID uuid.UUID
		var role string
		if err := roleRows.Scan(&clientID, &siteID, &role); err != nil {
			return nil, fmt.Errorf("failed to scan api client site role: %w", err)
		}
		idx, ok := indexByID[clientID]
		if !ok {
			continue
		}
		clients[idx].SiteRoles = append(clients[idx].SiteRoles, api.APIClientSiteRole{
			SiteID: siteID,
			Role:   role,
		})
	}
	if err := roleRows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating api client site roles: %w", err)
	}

	return clients, nil
}

func (s *Store) GetAPIClient(ctx context.Context, userID uuid.UUID, clientID uuid.UUID) (*api.APIClient, error) {
	var client api.APIClient
	var description sql.NullString
	var expiresAt sql.NullTime
	var lastUsedAt sql.NullTime
	var revokedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, description, instance_role, expires_at, last_used_at, revoked_at, created_at, updated_at
		FROM api_clients
		WHERE id = ? AND user_id = ?
	`, clientID, userID).Scan(
		&client.ID,
		&client.UserID,
		&client.Name,
		&description,
		&client.InstanceRole,
		&expiresAt,
		&lastUsedAt,
		&revokedAt,
		&client.CreatedAt,
		&client.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get api client: %w", err)
	}

	client.Description = strings.TrimSpace(description.String)
	client.ExpiresAt = nullTimePtr(expiresAt)
	client.LastUsedAt = nullTimePtr(lastUsedAt)
	client.RevokedAt = nullTimePtr(revokedAt)
	client.SiteRoles = make([]api.APIClientSiteRole, 0)

	rows, err := s.db.QueryContext(ctx, `
		SELECT site_id, role
		FROM api_client_site_roles
		WHERE api_client_id = ?
		ORDER BY site_id
	`, client.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get api client site roles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var siteID uuid.UUID
		var role string
		if err := rows.Scan(&siteID, &role); err != nil {
			return nil, fmt.Errorf("failed to scan api client site role: %w", err)
		}
		client.SiteRoles = append(client.SiteRoles, api.APIClientSiteRole{
			SiteID: siteID,
			Role:   role,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating api client site roles: %w", err)
	}

	return &client, nil
}

func (s *Store) UpdateAPIClient(
	ctx context.Context,
	userID uuid.UUID,
	clientID uuid.UUID,
	name string,
	description string,
	instanceRole auth.InstanceRole,
	siteRoles map[uuid.UUID]auth.SiteRole,
	expiresAt *time.Time,
	revoked bool,
) (*api.APIClient, error) {
	now := time.Now().UTC()
	exp := toUTCPtr(expiresAt)

	var revokedAt any
	if revoked {
		revokedAt = now
	}

	err := s.Transact(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE api_clients
			SET name = ?, description = ?, instance_role = ?, expires_at = ?, revoked_at = ?, updated_at = ?
			WHERE id = ? AND user_id = ?
		`, name, description, instanceRole, exp, revokedAt, now, clientID, userID)
		if err != nil {
			return fmt.Errorf("failed to update api client: %w", err)
		}

		rows, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to determine rows affected: %w", err)
		}
		if rows == 0 {
			return ErrAPIClientNotFound
		}

		if err := replaceAPIClientSiteRolesTx(ctx, tx, clientID, siteRoles, now); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, ErrAPIClientNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return s.GetAPIClient(ctx, userID, clientID)
}

func (s *Store) DeleteAPIClient(ctx context.Context, userID uuid.UUID, clientID uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM api_client_site_roles WHERE api_client_id = CAST(? AS UUID)", clientID.String()); err != nil {
		return fmt.Errorf("failed to delete api client site roles: %w", err)
	}

	res, err := s.db.ExecContext(ctx, "DELETE FROM api_clients WHERE id = CAST(? AS UUID) AND user_id = CAST(? AS UUID)", clientID.String(), userID.String())
	if err != nil {
		return fmt.Errorf("failed to delete api client: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine rows affected: %w", err)
	}
	if rows == 0 {
		return ErrAPIClientNotFound
	}
	return nil
}

func (s *Store) GetAPIClientAuth(ctx context.Context, token string) (*APIClientAuth, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil
	}

	tokenHash := hashAPIClientToken(token)
	var authz APIClientAuth
	var instanceRole string
	var expiresAt sql.NullTime
	var revokedAt sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, instance_role, expires_at, revoked_at
		FROM api_clients
		WHERE secret_hash = ?
	`, tokenHash).Scan(
		&authz.ClientID,
		&authz.UserID,
		&instanceRole,
		&expiresAt,
		&revokedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get api client auth: %w", err)
	}

	if revokedAt.Valid {
		return nil, nil
	}
	if expiresAt.Valid && time.Now().UTC().After(expiresAt.Time.UTC()) {
		return nil, nil
	}

	authz.InstanceRole = auth.InstanceRole(instanceRole)
	authz.SiteRoles = make(map[uuid.UUID]auth.SiteRole)

	rows, err := s.db.QueryContext(ctx, `
		SELECT site_id, role
		FROM api_client_site_roles
		WHERE api_client_id = ?
	`, authz.ClientID)
	if err != nil {
		return nil, fmt.Errorf("failed to query api client site roles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var siteID uuid.UUID
		var role string
		if err := rows.Scan(&siteID, &role); err != nil {
			return nil, fmt.Errorf("failed to scan api client site role: %w", err)
		}
		authz.SiteRoles[siteID] = auth.SiteRole(role)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating api client site roles: %w", err)
	}

	_, _ = s.db.ExecContext(ctx, "UPDATE api_clients SET last_used_at = ?, updated_at = ? WHERE id = ?", time.Now().UTC(), time.Now().UTC(), authz.ClientID)

	return &authz, nil
}

func replaceAPIClientSiteRolesTx(ctx context.Context, tx *sql.Tx, clientID uuid.UUID, siteRoles map[uuid.UUID]auth.SiteRole, now time.Time) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM api_client_site_roles WHERE api_client_id = ?", clientID); err != nil {
		return fmt.Errorf("failed to replace api client site roles: %w", err)
	}

	if len(siteRoles) == 0 {
		return nil
	}

	siteIDs := make([]uuid.UUID, 0, len(siteRoles))
	for siteID := range siteRoles {
		siteIDs = append(siteIDs, siteID)
	}
	sort.Slice(siteIDs, func(i, j int) bool {
		return siteIDs[i].String() < siteIDs[j].String()
	})

	for _, siteID := range siteIDs {
		role := siteRoles[siteID]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO api_client_site_roles (id, api_client_id, site_id, role, created_at)
			VALUES (?, ?, ?, ?, ?)
		`, uuid.New(), clientID, siteID, role, now); err != nil {
			return fmt.Errorf("failed to insert api client site role: %w", err)
		}
	}

	return nil
}

func generateAPIClientToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("failed to generate api client token: %w", err)
	}

	token := hex.EncodeToString(buf)
	return token, hashAPIClientToken(token), nil
}

func hashAPIClientToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func flattenSiteRoles(siteRoles map[uuid.UUID]auth.SiteRole) []api.APIClientSiteRole {
	if len(siteRoles) == 0 {
		return []api.APIClientSiteRole{}
	}

	siteIDs := make([]uuid.UUID, 0, len(siteRoles))
	for siteID := range siteRoles {
		siteIDs = append(siteIDs, siteID)
	}
	sort.Slice(siteIDs, func(i, j int) bool {
		return siteIDs[i].String() < siteIDs[j].String()
	})

	roles := make([]api.APIClientSiteRole, 0, len(siteIDs))
	for _, siteID := range siteIDs {
		roles = append(roles, api.APIClientSiteRole{
			SiteID: siteID,
			Role:   string(siteRoles[siteID]),
		})
	}
	return roles
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	ts := value.Time.UTC()
	return &ts
}

func toUTCPtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	ts := value.UTC()
	return &ts
}
