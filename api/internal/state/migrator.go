package state

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"time"
)

// Migration represents a single versioned schema change.
type Migration struct {
	Version     int
	Description string
	Up          func(tx *sql.Tx) error
}

// RunMigrations applies pending migrations in order. Each migration runs in
// its own transaction; the schema_migrations tracking table is created
// automatically if it does not exist.
func RunMigrations(db *sql.DB, migrations []Migration) error {
	if db == nil {
		return fmt.Errorf("migrator: db is nil")
	}

	// Ensure tracking table exists.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INT          PRIMARY KEY,
			applied_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("migrator: create tracking table: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return fmt.Errorf("migrator: read applied versions: %w", err)
	}

	// Sort migrations by version so they run in order.
	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version < sorted[j].Version })

	for _, m := range sorted {
		if applied[m.Version] {
			continue
		}

		start := time.Now()

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migrator: begin tx for v%d: %w", m.Version, err)
		}

		if err := m.Up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrator: v%d (%s) failed: %w", m.Version, m.Description, err)
		}

		if _, err := tx.Exec(
			`INSERT INTO schema_migrations (version) VALUES ($1)`,
			m.Version,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrator: record v%d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrator: commit v%d: %w", m.Version, err)
		}

		slog.Info("migration applied",
			"version", m.Version,
			"description", m.Description,
			"elapsed", time.Since(start).Round(time.Millisecond).String(),
		)
	}

	return nil
}

// appliedVersions returns a set of migration versions already recorded in the
// schema_migrations table.
func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// AllMigrations returns the ordered list of every known migration. New
// migrations should be appended here.
func AllMigrations() []Migration {
	return []Migration{
		migrations001InitialSchema(),
	}
}
