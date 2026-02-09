package shared

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"valiant/tests/common"
)

const migrationsRelPath = "../../../backend/migrations"

// SetupTestDB creates a new, isolated schema for a test, runs migrations,
// and returns a connection pool locked to that schema.
func SetupTestDB() (*sql.DB, string, error) {
	// Get the directory of the current file (testdb.go in deep-linking/shared)
	_, filename, _, _ := runtime.Caller(0)
	currentDir := filepath.Dir(filename)

	// Construct the absolute path to the migrations directory
	absMigrationsPath := filepath.Join(currentDir, migrationsRelPath)

	// Pass the absolute path to common.SetupTestDB
	return common.SetupTestDB(absMigrationsPath)
}

// CleanupTestDB closes the schema-specific connection and drops the schema.
func CleanupTestDB(db *sql.DB, schemaName string) {
	common.CleanupTestDB(db, schemaName)
}
