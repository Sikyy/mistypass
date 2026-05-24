package state

import (
	"database/sql"
	"fmt"
	"testing"
)

func TestRunMigrationsCreatesTrackingTable(t *testing.T) {
	store := newTestPostgresStore(t)

	// RunMigrations should have been called via EnsureSchema already,
	// so schema_migrations must exist.
	var count int
	err := store.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count)
	if err != nil {
		t.Fatalf("schema_migrations table should exist: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 applied migration, got %d", count)
	}
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	store := newTestPostgresStore(t)

	// Running again should be a no-op, not an error.
	if err := RunMigrations(store.db, AllMigrations()); err != nil {
		t.Fatalf("second RunMigrations call should succeed: %v", err)
	}

	applied, err := appliedVersions(store.db)
	if err != nil {
		t.Fatalf("query applied versions failed: %v", err)
	}
	for _, migration := range AllMigrations() {
		if !applied[migration.Version] {
			t.Fatalf("expected migration version %d to be applied; applied=%v", migration.Version, applied)
		}
	}
}

func TestRunMigrationsSkipsApplied(t *testing.T) {
	store := newTestPostgresStore(t)

	calls := 0
	probe := Migration{
		Version:     9999,
		Description: "test probe",
		Up: func(tx *sql.Tx) error {
			calls++
			return nil
		},
	}

	all := append(AllMigrations(), probe)

	if err := RunMigrations(store.db, all); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected probe to run once, ran %d times", calls)
	}

	// Second run — probe should be skipped.
	if err := RunMigrations(store.db, all); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected probe to still be 1 after second run, got %d", calls)
	}

	// Cleanup the test migration row.
	_, _ = store.db.Exec(`DELETE FROM schema_migrations WHERE version = 9999`)
}

func TestRunMigrationsRollsBackOnError(t *testing.T) {
	store := newTestPostgresStore(t)

	bad := Migration{
		Version:     9998,
		Description: "intentional failure",
		Up: func(tx *sql.Tx) error {
			return fmt.Errorf("boom")
		},
	}

	err := RunMigrations(store.db, append(AllMigrations(), bad))
	if err == nil {
		t.Fatalf("expected error from failing migration")
	}

	// Version 9998 should NOT be recorded.
	var count int
	if err := store.db.QueryRow(
		`SELECT count(*) FROM schema_migrations WHERE version = 9998`,
	).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed migration should not be recorded, got count %d", count)
	}
}

func TestRunMigrationsRejectsNilDB(t *testing.T) {
	if err := RunMigrations(nil, AllMigrations()); err == nil {
		t.Fatalf("expected error for nil db")
	}
}
