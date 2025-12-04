package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

// Goals

func (s *Store) CreateGoal(ctx context.Context, goal *api.Goal) error {
	if goal.CreatedAt.IsZero() {
		goal.CreatedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO goals (site_id, name, type, value, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		goal.SiteID, goal.Name, goal.Type, goal.Value, goal.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create goal: %w", err)
	}
	return nil
}

func (s *Store) GetGoals(ctx context.Context, siteID uuid.UUID) ([]api.Goal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, site_id, name, type, value, created_at
		FROM goals
		WHERE site_id = ?
		ORDER BY created_at DESC`,
		siteID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query goals: %w", err)
	}
	defer rows.Close()

	var goals []api.Goal
	for rows.Next() {
		var g api.Goal
		if err := rows.Scan(&g.ID, &g.SiteID, &g.Name, &g.Type, &g.Value, &g.CreatedAt); err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, nil
}

func (s *Store) DeleteGoal(ctx context.Context, id uuid.UUID, siteID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM goals WHERE id = ? AND site_id = ?", id, siteID)
	if err != nil {
		return fmt.Errorf("failed to delete goal: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("goal not found")
	}
	return nil
}

// Funnels

func (s *Store) CreateFunnel(ctx context.Context, funnel *api.Funnel) error {
	if funnel.CreatedAt.IsZero() {
		funnel.CreatedAt = time.Now()
	}

	stepsJSON, err := json.Marshal(funnel.Steps)
	if err != nil {
		return fmt.Errorf("failed to marshal funnel steps: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO funnels (site_id, name, steps, created_at)
		VALUES (?, ?, ?, ?)`,
		funnel.SiteID, funnel.Name, stepsJSON, funnel.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create funnel: %w", err)
	}
	return nil
}

func (s *Store) GetFunnels(ctx context.Context, siteID uuid.UUID) ([]api.Funnel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, site_id, name, steps, created_at
		FROM funnels
		WHERE site_id = ?
		ORDER BY created_at DESC`,
		siteID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query funnels: %w", err)
	}
	defer rows.Close()

	var funnels []api.Funnel
	for rows.Next() {
		var f api.Funnel
		var stepsJSON string
		if err := rows.Scan(&f.ID, &f.SiteID, &f.Name, &stepsJSON, &f.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(stepsJSON), &f.Steps); err != nil {
			return nil, fmt.Errorf("failed to unmarshal funnel steps: %w", err)
		}
		funnels = append(funnels, f)
	}
	return funnels, nil
}

func (s *Store) DeleteFunnel(ctx context.Context, id uuid.UUID, siteID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM funnels WHERE id = ? AND site_id = ?", id, siteID)
	if err != nil {
		return fmt.Errorf("failed to delete funnel: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("funnel not found")
	}
	return nil
}
