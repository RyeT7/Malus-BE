package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strings"
)

var batchSeparator = regexp.MustCompile(`(?im)^\s*GO\s*$`)

const createMigrationsTable = `
IF OBJECT_ID(N'dbo.schema_migrations', N'U') IS NULL
CREATE TABLE dbo.schema_migrations (
    version    NVARCHAR(255)  NOT NULL CONSTRAINT pk_schema_migrations PRIMARY KEY,
    applied_at DATETIMEOFFSET NOT NULL CONSTRAINT df_schema_migrations_applied_at DEFAULT SYSDATETIMEOFFSET()
)`

func Migrate(ctx context.Context, db *sql.DB, migrations fs.FS, log *slog.Logger) error {
	if _, err := db.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(migrations, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	applied := 0
	for _, name := range names {
		body, err := fs.ReadFile(migrations, name)
		if err != nil {
			return err
		}
		ran, err := apply(ctx, db, name, string(body))
		if err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if ran {
			applied++
			log.InfoContext(ctx, "migration applied", "version", name)
		}
	}
	log.InfoContext(ctx, "migrations complete", "applied", applied, "total", len(names))
	return nil
}

func apply(ctx context.Context, db *sql.DB, version, body string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `EXEC sp_getapplock @Resource = N'schema_migrations', @LockMode = N'Exclusive', @LockOwner = N'Transaction', @LockTimeout = 30000`); err != nil {
		return false, fmt.Errorf("acquire migration lock: %w", err)
	}

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM dbo.schema_migrations WHERE version = @version`, sql.Named("version", version)).Scan(&exists); err != nil {
		return false, err
	}
	if exists > 0 {
		return false, nil
	}

	for _, batch := range splitBatches(body) {
		if _, err := tx.ExecContext(ctx, batch); err != nil {
			return false, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO dbo.schema_migrations (version) VALUES (@version)`, sql.Named("version", version)); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func splitBatches(body string) []string {
	var batches []string
	for _, part := range batchSeparator.Split(body, -1) {
		if part = strings.TrimSpace(part); part != "" {
			batches = append(batches, part)
		}
	}
	return batches
}
