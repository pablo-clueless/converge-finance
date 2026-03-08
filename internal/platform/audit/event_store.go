package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type Event struct {
	ID            common.ID
	AggregateType string
	AggregateID   common.ID
	EventType     string
	EventVersion  int
	EventData     map[string]any
	Metadata      EventMetadata
	CreatedAt     time.Time
}

type EventMetadata struct {
	EntityID      common.ID `json:"entity_id"`
	UserID        common.ID `json:"user_id"`
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	Source        string    `json:"source,omitempty"`
}

type EventStore interface {
	Append(ctx context.Context, event Event) error

	AppendMultiple(ctx context.Context, events []Event) error

	GetByAggregate(ctx context.Context, aggregateType string, aggregateID common.ID) ([]Event, error)

	GetByEntity(ctx context.Context, entityID common.ID, from, to time.Time) ([]Event, error)

	GetByUser(ctx context.Context, userID common.ID, from, to time.Time) ([]Event, error)

	GetLatestVersion(ctx context.Context, aggregateType string, aggregateID common.ID) (int, error)
}

type PostgresEventStore struct {
	db *sql.DB
}

func NewPostgresEventStore(db *sql.DB) *PostgresEventStore {
	return &PostgresEventStore{db: db}
}

func (s *PostgresEventStore) Append(ctx context.Context, event Event) error {

	if event.ID.IsZero() {
		event.ID = common.NewID()
	}

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	if event.EventVersion == 0 {
		version, err := s.GetLatestVersion(ctx, event.AggregateType, event.AggregateID)
		if err != nil {
			return fmt.Errorf("failed to get latest version: %w", err)
		}
		event.EventVersion = version + 1
	}

	eventDataJSON, err := json.Marshal(event.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO audit_events (
			id, aggregate_type, aggregate_id, event_type, event_version,
			event_data, metadata, entity_id, user_id, ip_address, user_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err = s.db.ExecContext(ctx, query,
		event.ID,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.EventVersion,
		eventDataJSON,
		metadataJSON,
		event.Metadata.EntityID,
		event.Metadata.UserID,
		event.Metadata.IPAddress,
		event.Metadata.UserAgent,
		event.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert audit event: %w", err)
	}

	return nil
}

func (s *PostgresEventStore) AppendMultiple(ctx context.Context, events []Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for i := range events {
		if events[i].ID.IsZero() {
			events[i].ID = common.NewID()
		}
		if events[i].CreatedAt.IsZero() {
			events[i].CreatedAt = time.Now()
		}

		eventDataJSON, err := json.Marshal(events[i].EventData)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}

		metadataJSON, err := json.Marshal(events[i].Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}

		query := `
			INSERT INTO audit_events (
				id, aggregate_type, aggregate_id, event_type, event_version,
				event_data, metadata, entity_id, user_id, ip_address, user_agent, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		`

		_, err = tx.ExecContext(ctx, query,
			events[i].ID,
			events[i].AggregateType,
			events[i].AggregateID,
			events[i].EventType,
			events[i].EventVersion,
			eventDataJSON,
			metadataJSON,
			events[i].Metadata.EntityID,
			events[i].Metadata.UserID,
			events[i].Metadata.IPAddress,
			events[i].Metadata.UserAgent,
			events[i].CreatedAt,
		)

		if err != nil {
			return fmt.Errorf("failed to insert audit event: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *PostgresEventStore) GetByAggregate(ctx context.Context, aggregateType string, aggregateID common.ID) ([]Event, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, event_version,
			   event_data, metadata, entity_id, user_id, ip_address, user_agent, created_at
		FROM audit_events
		WHERE aggregate_type = $1 AND aggregate_id = $2
		ORDER BY event_version ASC
	`

	return s.queryEvents(ctx, query, aggregateType, aggregateID)
}

func (s *PostgresEventStore) GetByEntity(ctx context.Context, entityID common.ID, from, to time.Time) ([]Event, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, event_version,
			   event_data, metadata, entity_id, user_id, ip_address, user_agent, created_at
		FROM audit_events
		WHERE entity_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at ASC
	`

	return s.queryEvents(ctx, query, entityID, from, to)
}

func (s *PostgresEventStore) GetByUser(ctx context.Context, userID common.ID, from, to time.Time) ([]Event, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, event_version,
			   event_data, metadata, entity_id, user_id, ip_address, user_agent, created_at
		FROM audit_events
		WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		ORDER BY created_at ASC
	`

	return s.queryEvents(ctx, query, userID, from, to)
}

func (s *PostgresEventStore) GetLatestVersion(ctx context.Context, aggregateType string, aggregateID common.ID) (int, error) {
	query := `
		SELECT COALESCE(MAX(event_version), 0)
		FROM audit_events
		WHERE aggregate_type = $1 AND aggregate_id = $2
	`

	var version int
	err := s.db.QueryRowContext(ctx, query, aggregateType, aggregateID).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest version: %w", err)
	}

	return version, nil
}

func (s *PostgresEventStore) queryEvents(ctx context.Context, query string, args ...any) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var eventDataJSON, metadataJSON []byte
		var entityID, userID string
		var ipAddress, userAgent sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&event.EventVersion,
			&eventDataJSON,
			&metadataJSON,
			&entityID,
			&userID,
			&ipAddress,
			&userAgent,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if err := json.Unmarshal(eventDataJSON, &event.EventData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		event.Metadata.EntityID = common.ID(entityID)
		event.Metadata.UserID = common.ID(userID)
		if ipAddress.Valid {
			event.Metadata.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			event.Metadata.UserAgent = userAgent.String
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	return events, nil
}
