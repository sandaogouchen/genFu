package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ApplyMigrations(ctx context.Context, db *DB) error {
	if db == nil || db.DB == nil {
		return errors.New("db_not_initialized")
	}
	if err := ensureMigrationsTable(ctx, db.DB); err != nil {
		return err
	}
	migrationsDir := filepath.Join("internal", "db", "migrations")
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasSuffix(name, ".sql") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		applied, err := isMigrationApplied(ctx, db.DB, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigrationFile(ctx, db.DB, filepath.Join(migrationsDir, name), version); err != nil {
			return err
		}
	}
	return nil
}

func ensureMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		create table if not exists schema_migrations (
			version text primary key,
			applied_at text not null default CURRENT_TIMESTAMP
		)
	`)
	return err
}

func isMigrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `select exists(select 1 from schema_migrations where version = ?)`, version).Scan(&exists)
	return exists, err
}

func applyMigrationFile(ctx context.Context, db *sql.DB, path string, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	segments := strings.Split(string(data), ";")
	for _, stmt := range segments {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `insert into schema_migrations(version) values (?)`, version); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
