package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
)

//go:embed sql/*.sql
var migrationFS embed.FS

type migration struct {
	Version int
	Name    string
	SQL     string
}

func Up(ctx context.Context, db *sql.DB) error {
	currentVersion, err := getCurrentVersion(ctx, db)
	if err != nil {
		return err
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if migration.Version <= currentVersion {
			continue
		}

		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
	}

	return nil
}

func getCurrentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read sqlite user_version: %w", err)
	}

	return version, nil
}

func loadMigrations() ([]migration, error) {
	paths, err := fs.Glob(migrationFS, "sql/*.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migration files: %w", err)
	}

	migrations := make([]migration, 0, len(paths))
	for _, migrationPath := range paths {
		fileName := path.Base(migrationPath)
		version, err := parseVersion(fileName)
		if err != nil {
			return nil, fmt.Errorf("parse migration version for %q: %w", fileName, err)
		}

		body, err := fs.ReadFile(migrationFS, migrationPath)
		if err != nil {
			return nil, fmt.Errorf("read migration file %q: %w", migrationPath, err)
		}

		migrations = append(migrations, migration{
			Version: version,
			Name:    fileName,
			SQL:     string(body),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func parseVersion(fileName string) (int, error) {
	parts := strings.SplitN(fileName, "_", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid migration name, expected NNN_description.sql")
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	return version, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", migration.Name, err)
	}

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("execute migration %q: %w", migration.Name, err)
	}

	pragmaStatement := fmt.Sprintf("PRAGMA user_version = %d", migration.Version)
	if _, err := tx.ExecContext(ctx, pragmaStatement); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("set user_version for migration %q: %w", migration.Name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", migration.Name, err)
	}

	return nil
}
