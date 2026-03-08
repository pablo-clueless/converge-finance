package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type PostgresDB struct {
	*sql.DB
}

func NewPostgresDB(databaseURL string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{DB: db}, nil
}

func (db *PostgresDB) SetEntityContext(ctx context.Context, entityID string) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf("SET app.current_entity_id = '%s'", entityID))
	return err
}

func (db *PostgresDB) ClearEntityContext(ctx context.Context) error {
	_, err := db.ExecContext(ctx, "RESET app.current_entity_id")
	return err
}

func (db *PostgresDB) Health(ctx context.Context) error {
	return db.PingContext(ctx)
}

// WithEntityContext executes a function with the entity context set on the same connection.
// This ensures RLS policies work correctly with connection pooling.
func (db *PostgresDB) WithEntityContext(ctx context.Context, entityID string, fn func(conn *sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Set the entity context on this specific connection
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_entity_id = '%s'", entityID)); err != nil {
		return fmt.Errorf("failed to set entity context: %w", err)
	}

	return fn(conn)
}

// QueryWithEntity executes a query with entity context set, returning rows.
func (db *PostgresDB) QueryWithEntity(ctx context.Context, entityID, query string, args ...any) (*sql.Rows, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection: %w", err)
	}
	// Note: caller must close rows, which will release the connection

	// Set the entity context on this specific connection
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_entity_id = '%s'", entityID)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to set entity context: %w", err)
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	return rows, nil
}

// QueryRowWithEntity executes a query with entity context set, returning a single row.
func (db *PostgresDB) QueryRowWithEntity(ctx context.Context, entityID, query string, args ...any) (*sql.Row, func(), error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to acquire connection: %w", err)
	}

	cleanup := func() { _ = conn.Close() }

	// Set the entity context on this specific connection
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_entity_id = '%s'", entityID)); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("failed to set entity context: %w", err)
	}

	row := conn.QueryRowContext(ctx, query, args...)
	return row, cleanup, nil
}
