package audit

import (
	"context"
	"net/http"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/platform/auth"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	httpRequestKey    contextKey = "http_request"
	correlationIDKey  contextKey = "correlation_id"
)

type Logger struct {
	eventStore EventStore
}

func NewLogger(eventStore EventStore) *Logger {
	return &Logger{eventStore: eventStore}
}

func (l *Logger) Log(ctx context.Context, aggregateType string, aggregateID common.ID, eventType string, data map[string]any) error {
	metadata := l.extractMetadata(ctx)

	event := Event{
		ID:            common.NewID(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		EventData:     data,
		Metadata:      metadata,
	}

	return l.eventStore.Append(ctx, event)
}

func (l *Logger) LogCreate(ctx context.Context, aggregateType string, aggregateID common.ID, data map[string]any) error {
	return l.Log(ctx, aggregateType, aggregateID, "created", data)
}

func (l *Logger) LogUpdate(ctx context.Context, aggregateType string, aggregateID common.ID, changes map[string]any) error {
	return l.Log(ctx, aggregateType, aggregateID, "updated", changes)
}

func (l *Logger) LogDelete(ctx context.Context, aggregateType string, aggregateID common.ID, data map[string]any) error {
	return l.Log(ctx, aggregateType, aggregateID, "deleted", data)
}

func (l *Logger) LogAction(ctx context.Context, aggregateType string, aggregateID common.ID, action string, data map[string]any) error {
	return l.Log(ctx, aggregateType, aggregateID, action, data)
}

func (l *Logger) WithAudit(ctx context.Context, aggregateType string, aggregateID common.ID, eventType string, fn func() (map[string]any, error)) error {
	data, err := fn()
	if err != nil {
		_ = l.Log(ctx, aggregateType, aggregateID, eventType+"_failed", map[string]any{
			"error": err.Error(),
		})
		return err
	}

	return l.Log(ctx, aggregateType, aggregateID, eventType, data)
}

func (l *Logger) extractMetadata(ctx context.Context) EventMetadata {
	metadata := EventMetadata{
		Source: "api",
	}

	claims := auth.GetClaimsFromContext(ctx)
	if claims != nil {
		metadata.UserID = common.ID(claims.UserID)
		metadata.EntityID = common.ID(claims.EntityID)
	}

	if correlationID, ok := ctx.Value(correlationIDKey).(string); ok {
		metadata.CorrelationID = correlationID
	}

	if req, ok := ctx.Value(httpRequestKey).(*http.Request); ok {
		metadata.IPAddress = getClientIP(req)
		metadata.UserAgent = req.UserAgent()
	}

	return metadata
}

func getClientIP(r *http.Request) string {

	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	return r.RemoteAddr
}

func ContextWithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey, r)
}

func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}
