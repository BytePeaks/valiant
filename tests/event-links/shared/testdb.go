package shared

import (
	"database/sql"
	"path/filepath"
	"valiant/tests/common"
)

const migrationsRelPath = "../../../backend/migrations"

func SetupTestDB() (*sql.DB, string, error) {
	return common.SetupTestDB(filepath.Join(migrationsRelPath))
}

func CleanupTestDB(db *sql.DB, schemaName string) {
	common.CleanupTestDB(db, schemaName)
}
