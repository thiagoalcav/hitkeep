package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"hitkeep/internal/api"
)

func (s *Store) CreateEvent(ctx context.Context, event *api.Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	props, err := json.Marshal(event.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal event properties: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
        INSERT INTO events (
            site_id, session_id, name, properties, timestamp
        ) VALUES (?, ?, ?, ?, ?)`,
		event.SiteID, event.SessionID, event.Name, props, event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("could not insert event: %w", err)
	}
	return nil
}
