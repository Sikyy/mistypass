package migrations

import "database/sql"

// InitialSchemaUp is a no-op marker for the initial schema. The existing
// EnsureSchema() / ensureProjectionTables() path already creates all tables
// using CREATE TABLE IF NOT EXISTS, so this migration simply records that the
// baseline has been established. Future migrations (002, 003, ...) will
// contain real ALTER TABLE / CREATE TABLE statements.
func InitialSchemaUp(tx *sql.Tx) error {
	// All baseline DDL is handled by EnsureSchema(). This migration exists
	// so the schema_migrations table starts at version 1 and subsequent
	// migrations can rely on version ordering.
	_, err := tx.Exec(`SELECT 1`) // intentional no-op
	return err
}
