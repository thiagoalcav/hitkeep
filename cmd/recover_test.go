package hitkeepcmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"hitkeep/internal/database"
)

func TestMoveExistingDatabaseAsideRenamesDatabaseAndWal(t *testing.T) {
	t.Parallel()

	targetPath := filepath.Join(t.TempDir(), "hitkeep.db")
	if err := os.WriteFile(targetPath, []byte("db"), 0644); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if err := os.WriteFile(targetPath+".wal", []byte("wal"), 0644); err != nil {
		t.Fatalf("write wal: %v", err)
	}

	backupPath, err := moveExistingDatabaseAside(targetPath)
	if err != nil {
		t.Fatalf("moveExistingDatabaseAside: %v", err)
	}
	if backupPath == "" {
		t.Fatal("expected backup path")
	}

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("expected target db to be moved aside, stat err=%v", err)
	}
	if _, err := os.Stat(targetPath + ".wal"); !os.IsNotExist(err) {
		t.Fatalf("expected target wal to be moved aside, stat err=%v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup db to exist: %v", err)
	}
	if _, err := os.Stat(backupPath + ".wal"); err != nil {
		t.Fatalf("expected backup wal to exist: %v", err)
	}
}

func TestRestoreDatabaseDoesNotLeaveWal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	sourceDBPath := filepath.Join(tmpDir, "source.db")
	sourceSnapshotPath := filepath.Join(tmpDir, "snapshot")

	sourceStore := database.NewStore(sourceDBPath)
	if err := sourceStore.Connect(); err != nil {
		t.Fatalf("connect source: %v", err)
	}

	sourceDB := sourceStore.DB()
	if _, err := sourceDB.ExecContext(ctx, "CREATE TABLE recover_test(id INTEGER, name VARCHAR);"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sourceDB.ExecContext(ctx, "INSERT INTO recover_test VALUES (1, 'acme');"); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	if _, err := sourceDB.ExecContext(ctx, "CHECKPOINT;"); err != nil {
		t.Fatalf("checkpoint source: %v", err)
	}
	if err := os.MkdirAll(sourceSnapshotPath, 0755); err != nil {
		t.Fatalf("mkdir snapshot: %v", err)
	}
	if _, err := sourceDB.ExecContext(ctx, fmt.Sprintf("EXPORT DATABASE '%s' (FORMAT PARQUET);", sourceSnapshotPath)); err != nil {
		t.Fatalf("export database: %v", err)
	}
	if err := sourceStore.Close(); err != nil {
		t.Fatalf("close source store: %v", err)
	}

	targetPath := filepath.Join(tmpDir, "restored.db")
	if err := restoreDatabase(ctx, targetPath, sourceSnapshotPath, false, nil); err != nil {
		t.Fatalf("restoreDatabase: %v", err)
	}

	if _, err := os.Stat(targetPath + ".wal"); !os.IsNotExist(err) {
		t.Fatalf("expected no wal file after restore, stat err=%v", err)
	}

	targetStore := database.NewStore(targetPath)
	if err := targetStore.Connect(); err != nil {
		t.Fatalf("connect restored: %v", err)
	}
	defer targetStore.Close()

	var (
		id   int
		name string
	)
	if err := targetStore.DB().QueryRowContext(ctx, "SELECT id, name FROM recover_test;").Scan(&id, &name); err != nil {
		t.Fatalf("query restored row: %v", err)
	}
	if id != 1 || name != "acme" {
		t.Fatalf("unexpected restored row: id=%d name=%q", id, name)
	}
}

func TestRestoreDatabasePreservesExistingBrokenWalWithoutOpeningTarget(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmpDir := t.TempDir()
	sourceDBPath := filepath.Join(tmpDir, "source.db")
	sourceSnapshotPath := filepath.Join(tmpDir, "snapshot")

	db, err := sql.Open("duckdb", sourceDBPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, "CREATE TABLE recover_test(id INTEGER);"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO recover_test VALUES (7);"); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	if err := os.MkdirAll(sourceSnapshotPath, 0755); err != nil {
		t.Fatalf("mkdir snapshot: %v", err)
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("EXPORT DATABASE '%s' (FORMAT PARQUET);", sourceSnapshotPath)); err != nil {
		t.Fatalf("export database: %v", err)
	}

	targetPath := filepath.Join(tmpDir, "hitkeep.db")
	if err := os.WriteFile(targetPath, []byte("not-a-db"), 0644); err != nil {
		t.Fatalf("write target db: %v", err)
	}
	if err := os.WriteFile(targetPath+".wal", []byte("broken-wal"), 0644); err != nil {
		t.Fatalf("write target wal: %v", err)
	}

	if err := restoreDatabase(ctx, targetPath, sourceSnapshotPath, false, nil); err != nil {
		t.Fatalf("restoreDatabase with existing wal: %v", err)
	}

	if _, err := os.Stat(targetPath + ".wal"); !os.IsNotExist(err) {
		t.Fatalf("expected no wal file after restore, stat err=%v", err)
	}
	if matches, err := filepath.Glob(targetPath + ".pre-restore.*.wal"); err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one preserved wal backup, matches=%v err=%v", matches, err)
	}
}
