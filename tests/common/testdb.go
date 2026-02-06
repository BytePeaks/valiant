package common

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

// SetupTestDB creates a new, isolated schema for a test, runs migrations,
// and returns a connection pool locked to that schema.
// migrationsRelPath should be the relative path from the calling test's working directory
// to the backend/migrations/ directory (e.g. "../../../backend/migrations").
func SetupTestDB(migrationsRelPath string) (*sql.DB, string, error) {
	// 1. Connect to the main test database
	host := GetEnv("DB_HOST", "localhost")
	port := GetEnv("DB_PORT", "5432")
	user := GetEnv("DB_USER", "testuser")
	password := GetEnv("DB_PASSWORD", "testpass")
	dbname := GetEnv("DB_NAME", "valiant_test")

	mainDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	mainDB, err := sql.Open("postgres", mainDSN)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open main db connection: %w", err)
	}
	defer mainDB.Close()
	if err := mainDB.Ping(); err != nil {
		return nil, "", fmt.Errorf("failed to ping main db: %w", err)
	}

	// 2. Create a unique schema
	schemaName := "test_schema_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + strconv.Itoa(rand.Intn(1000))
	if _, err := mainDB.Exec("CREATE SCHEMA " + schemaName); err != nil {
		return nil, "", fmt.Errorf("failed to create schema %s: %w", schemaName, err)
	}

	// 3. Create a new DSN and connection pool specifically for that schema
	schemaDSN := mainDSN + " search_path=" + schemaName
	schemaDB, err := sql.Open("postgres", schemaDSN)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open schema-specific db connection: %w", err)
	}
	if err := schemaDB.Ping(); err != nil {
		return nil, "", fmt.Errorf("failed to ping schema-specific db: %w", err)
	}

	// 4. Run migrations within the new schema
	if err := runMigrations(schemaDB, migrationsRelPath); err != nil {
		// Best effort to clean up if migrations fail
		CleanupTestDB(schemaDB, schemaName)
		return nil, "", fmt.Errorf("failed to run migrations in schema %s: %w", schemaName, err)
	}

	return schemaDB, schemaName, nil
}

// CleanupTestDB closes the schema-specific connection and drops the schema.
func CleanupTestDB(db *sql.DB, schemaName string) {
	if db != nil {
		db.Close()
	}

	// Connect to the main database to drop the test schema
	host := GetEnv("DB_HOST", "localhost")
	port := GetEnv("DB_PORT", "5432")
	user := GetEnv("DB_USER", "testuser")
	password := GetEnv("DB_PASSWORD", "testpass")
	dbname := GetEnv("DB_NAME", "valiant_test")

	mainDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	mainDB, err := sql.Open("postgres", mainDSN)
	if err != nil {
		fmt.Printf("Error opening main DB for cleanup: %v\n", err)
		return
	}
	defer mainDB.Close()

	if _, err := mainDB.Exec("DROP SCHEMA " + schemaName + " CASCADE"); err != nil {
		fmt.Printf("Error dropping schema %s: %v\n", schemaName, err)
	}
}

// runMigrations executes all SQL migration files from the given directory path.
func runMigrations(db *sql.DB, migrationsPath string) error {
	files, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".sql" {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsPath, file.Name()))
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file.Name(), err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file.Name(), err)
		}
	}

	return nil
}

// GetEnv returns the value of an environment variable, or a fallback default.
func GetEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
