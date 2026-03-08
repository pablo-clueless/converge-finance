package database

import (
	"database/sql"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func RunMigrations(databaseURL string, direction string) error {
	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer m.Close()

	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("failed to run up migrations: %w", err)
		}
	case "down":
		if err := m.Steps(-1); err != nil {
			return fmt.Errorf("failed to run down migration: %w", err)
		}
	case "reset":
		if err := m.Drop(); err != nil {
			return fmt.Errorf("failed to drop database: %w", err)
		}
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("failed to run up migrations after reset: %w", err)
		}
	case "drop":
		// Close the migrator first to release any connections
		m.Close()
		if err := dropAllSchemas(databaseURL); err != nil {
			return fmt.Errorf("failed to drop schemas: %w", err)
		}
		// Recreate migrator for any subsequent operations
		m, err = migrate.New("file://migrations", databaseURL)
		if err != nil {
			return fmt.Errorf("failed to recreate migrator: %w", err)
		}
		defer m.Close()
	case "force":
		if err := m.Force(0); err != nil {
			return fmt.Errorf("failed to force version: %w", err)
		}
	default:
		return fmt.Errorf("unknown migration direction: %s", direction)
	}

	return nil
}

func GetMigrationVersion(databaseURL string) (uint, bool, error) {
	m, err := migrate.New(
		"file://migrations",
		databaseURL,
	)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrator: %w", err)
	}
	defer m.Close()

	return m.Version()
}

func dropAllSchemas(databaseURL string) error {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Drop all custom types first (in all schemas)
	rows, err := db.Query(`
		SELECT n.nspname, t.typname
		FROM pg_type t
		JOIN pg_namespace n ON t.typnamespace = n.oid
		WHERE t.typtype = 'e'
		AND n.nspname NOT IN ('pg_catalog', 'information_schema')
	`)
	if err == nil {
		var types []struct{ schema, name string }
		for rows.Next() {
			var schema, name string
			if err := rows.Scan(&schema, &name); err == nil {
				types = append(types, struct{ schema, name string }{schema, name})
			}
		}
		rows.Close()
		for _, t := range types {
			db.Exec(fmt.Sprintf("DROP TYPE IF EXISTS %s.%s CASCADE", t.schema, t.name))
		}
	}

	schemas := []string{
		"audit", "gl", "ap", "ar", "fa", "ic", "consol", "cost", "close", "fx",
		"workflow", "segment", "export", "docs",
	}

	for _, schema := range schemas {
		_, err := db.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
		if err != nil {
			return fmt.Errorf("failed to drop schema %s: %w", schema, err)
		}
	}

	// Drop public tables
	tables := []string{
		"schema_migrations", "user_entity_access", "users", "exchange_rates",
		"currencies", "entities",
	}
	for _, table := range tables {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	// Drop functions
	_, _ = db.Exec("DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE")

	return nil
}
