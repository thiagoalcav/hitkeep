package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type duckdbExtensionExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// LoadInstalledCoreExtension loads an already-installed core DuckDB extension.
func LoadInstalledCoreExtension(ctx context.Context, exec duckdbExtensionExecutor, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("duckdb extension name is required")
	}

	loadQuery := fmt.Sprintf("LOAD %s;", trimmed)
	if _, err := exec.ExecContext(ctx, loadQuery); err != nil {
		return fmt.Errorf("load %s extension: %w", trimmed, err)
	}

	return nil
}

// EnsureCoreExtension loads a core DuckDB extension, installing it first if necessary.
func EnsureCoreExtension(ctx context.Context, exec duckdbExtensionExecutor, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("duckdb extension name is required")
	}

	if err := LoadInstalledCoreExtension(ctx, exec, trimmed); err == nil {
		return nil
	} else {
		loadErr := err
		installQuery := fmt.Sprintf("INSTALL %s;", trimmed)
		if _, err := exec.ExecContext(ctx, installQuery); err != nil {
			return fmt.Errorf("load %s extension: %w; install %s extension: %v", trimmed, loadErr, trimmed, err)
		}
		if err := LoadInstalledCoreExtension(ctx, exec, trimmed); err != nil {
			return fmt.Errorf("load %s extension after install: %w", trimmed, err)
		}
	}

	return nil
}
