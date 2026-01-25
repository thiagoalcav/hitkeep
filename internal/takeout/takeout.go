package takeout

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func (s *TakeoutService) ExportUserData(ctx context.Context, userID uuid.UUID, format string) (string, error) {
	// Ensure export directory exists
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return "", fmt.Errorf("failed to create export directory: %w", err)
	}

	ext, duckFormat, allowFallback := resolveExportFormat(format)
	filename := filepath.Join(s.path, fmt.Sprintf("user_takeout_%s_%d.%s", userID, time.Now().Unix(), ext))
	fallbackFilename := filepath.Join(s.path, fmt.Sprintf("user_takeout_%s_%d.csv", userID, time.Now().Unix()))
	whereClause := fmt.Sprintf("site_id IN (SELECT site_id FROM site_members WHERE user_id = '%s')", userID)

	return s.exportTakeout(ctx, "user", filename, duckFormat, allowFallback, fallbackFilename, whereClause)
}

// ExportSiteData exports active data to the specified format.
// Format is validated by the handler (xlsx, csv, parquet).
func (s *TakeoutService) ExportSiteData(ctx context.Context, siteID uuid.UUID, format string) (string, error) {
	if err := os.MkdirAll(s.path, 0755); err != nil {
		return "", fmt.Errorf("failed to create export directory: %w", err)
	}

	ext, duckFormat, allowFallback := resolveExportFormat(format)
	filename := filepath.Join(s.path, fmt.Sprintf("site_takeout_%s_%d.%s", siteID, time.Now().Unix(), ext))
	fallbackFilename := filepath.Join(s.path, fmt.Sprintf("site_takeout_%s_%d.csv", siteID, time.Now().Unix()))
	whereClause := fmt.Sprintf("site_id = '%s'", siteID)

	return s.exportTakeout(ctx, "site", filename, duckFormat, allowFallback, fallbackFilename, whereClause)
}

func resolveExportFormat(format string) (string, string, bool) {
	switch strings.ToLower(format) {
	case "parquet":
		return "parquet", "PARQUET, COMPRESSION 'SNAPPY'", false
	case "csv":
		return "csv", "CSV, HEADER", false
	case "xlsx":
		fallthrough
	default:
		return "xlsx", "XLSX", true
	}
}

func (s *TakeoutService) exportTakeout(ctx context.Context, label, filename, duckFormat string, allowFallback bool, fallbackFilename string, whereClause string) (string, error) {
	query := buildTakeoutQuery(whereClause, filename, duckFormat)
	if _, err := s.store.DB().ExecContext(ctx, query); err != nil {
		if !allowFallback {
			return "", fmt.Errorf("failed to export %s data: %w", label, err)
		}

		fallbackQuery := buildTakeoutQuery(whereClause, fallbackFilename, "CSV, HEADER")
		if _, err := s.store.DB().ExecContext(ctx, fallbackQuery); err != nil {
			return "", fmt.Errorf("failed to export %s data: %w", label, err)
		}
		return fallbackFilename, nil
	}

	return filename, nil
}

func buildTakeoutQuery(whereClause, filename, format string) string {
	return fmt.Sprintf(`
	COPY (
		SELECT 'hit' as record_type, * FROM hits WHERE %s
		UNION BY NAME
		SELECT 'event' as record_type, * FROM events WHERE %s
		UNION BY NAME
		SELECT 'goal' as record_type, * FROM goals WHERE %s
		UNION BY NAME
		SELECT 'funnel' as record_type, * FROM funnels WHERE %s
	) TO '%s' (FORMAT %s);
`, whereClause, whereClause, whereClause, whereClause, filename, format)
}
