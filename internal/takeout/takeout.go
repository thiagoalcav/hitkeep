package takeout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/database"
)

type TakeoutService struct {
	store *database.Store
	path  string
}

func NewTakeoutService(store *database.Store, path string) *TakeoutService {
	return &TakeoutService{
		store: store,
		path:  path,
	}
}
func (s *TakeoutService) ExportUserData(ctx context.Context, userID uuid.UUID) (string, error) {
	// Ensure export directory exists
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return "", fmt.Errorf("failed to create export directory: %w", err)
	}

	filename := filepath.Join(s.path, fmt.Sprintf("user_takeout_%s_%d.xlsx", userID, time.Now().Unix()))

	query := fmt.Sprintf(`
	COPY (
		SELECT 'hit' as record_type, * FROM hits WHERE site_id IN (SELECT site_id FROM site_members WHERE user_id = '%s')
		UNION BY NAME
		SELECT 'event' as record_type, * FROM events WHERE site_id IN (SELECT site_id FROM site_members WHERE user_id = '%s')
	) TO '%s' (FORMAT XLSX);
`, userID, userID, filename)

	if _, err := s.store.DB().ExecContext(ctx, query); err != nil {
		// Fallback to CSV if XLSX fails
		csvFilename := filepath.Join(s.path, fmt.Sprintf("user_takeout_%s_%d.csv", userID, time.Now().Unix()))
		query = fmt.Sprintf(`
		COPY (
			SELECT 'hit' as record_type, * FROM hits WHERE site_id IN (SELECT site_id FROM site_members WHERE user_id = '%s')
			UNION BY NAME
			SELECT 'event' as record_type, * FROM events WHERE site_id IN (SELECT site_id FROM site_members WHERE user_id = '%s')
		) TO '%s' (FORMAT CSV, HEADER);
	`, userID, userID, csvFilename)

		if _, err := s.store.DB().ExecContext(ctx, query); err != nil {
			return "", fmt.Errorf("failed to export user data: %w", err)
		}
		return csvFilename, nil
	}

	return filename, nil
}

// ExportSiteData exports active data to the specified format.
// Format is validated by the handler (xlsx, csv, parquet).
func (s *TakeoutService) ExportSiteData(ctx context.Context, siteID uuid.UUID, format string) (string, error) {
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return "", fmt.Errorf("failed to create export directory: %w", err)
	}

	var ext, duckFormat string
	allowFallback := false

	switch format {
	case "parquet":
		ext = "parquet"
		duckFormat = "PARQUET, COMPRESSION 'SNAPPY'"
	case "csv":
		ext = "csv"
		duckFormat = "CSV, HEADER"
	case "xlsx":
		fallthrough
	default:
		ext = "xlsx"
		duckFormat = "XLSX"
		allowFallback = true
	}

	filename := filepath.Join(s.path, fmt.Sprintf("site_takeout_%s_%d.%s", siteID, time.Now().Unix(), ext))

	query := fmt.Sprintf(`
	COPY (
		SELECT 'hit' as record_type, * FROM hits WHERE site_id = '%s'
		UNION BY NAME
		SELECT 'event' as record_type, * FROM events WHERE site_id = '%s'
	) TO '%s' (FORMAT %s);
`, siteID, siteID, filename, duckFormat)

	if _, err := s.store.DB().ExecContext(ctx, query); err != nil {
		if !allowFallback {
			return "", fmt.Errorf("failed to export site data: %w", err)
		}

		csvFilename := filepath.Join(s.path, fmt.Sprintf("site_takeout_%s_%d.csv", siteID, time.Now().Unix()))
		fallbackQuery := fmt.Sprintf(`
	COPY (
		SELECT 'hit' as record_type, * FROM hits WHERE site_id = '%s'
		UNION BY NAME
		SELECT 'event' as record_type, * FROM events WHERE site_id = '%s'
	) TO '%s' (FORMAT CSV, HEADER);
`, siteID, siteID, csvFilename)

		if _, err := s.store.DB().ExecContext(ctx, fallbackQuery); err != nil {
			return "", fmt.Errorf("failed to export site data: %w", err)
		}
		return csvFilename, nil
	}

	return filename, nil
}
