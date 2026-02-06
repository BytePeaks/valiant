package shared

import (
	"database/sql"
	"path/filepath"
	"valiant/tests/common"
)

const migrationsRelPath = "../../../backend/migrations"

// SetupTestDB creates a new, isolated schema for a test, runs migrations,
// and returns a connection pool locked to that schema.
func SetupTestDB() (*sql.DB, string, error) {
	return common.SetupTestDB(filepath.Join(migrationsRelPath))
}

// CleanupTestDB closes the schema-specific connection and drops the schema.
func CleanupTestDB(db *sql.DB, schemaName string) {
	common.CleanupTestDB(db, schemaName)
}
