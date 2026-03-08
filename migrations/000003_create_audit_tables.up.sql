-- Audit Events (Immutable Event Store)
CREATE TABLE audit.audit_events (
    id              CHAR(26) PRIMARY KEY,
    aggregate_type  VARCHAR(50) NOT NULL,
    aggregate_id    CHAR(26) NOT NULL,
    event_type      VARCHAR(100) NOT NULL,
    event_version   INTEGER NOT NULL DEFAULT 1,
    event_data      JSONB NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    entity_id       CHAR(26) NOT NULL,
    user_id         CHAR(26) NOT NULL,
    ip_address      INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT audit_events_uk UNIQUE (aggregate_type, aggregate_id, event_version)
);

-- Indexes for common query patterns
CREATE INDEX idx_audit_events_aggregate ON audit.audit_events(aggregate_type, aggregate_id);
CREATE INDEX idx_audit_events_entity ON audit.audit_events(entity_id);
CREATE INDEX idx_audit_events_user ON audit.audit_events(user_id);
CREATE INDEX idx_audit_events_timestamp ON audit.audit_events(created_at);
CREATE INDEX idx_audit_events_type ON audit.audit_events(event_type);

-- Immutability enforcement via trigger (works without superuser privileges)
CREATE OR REPLACE FUNCTION audit.prevent_audit_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Audit events are immutable and cannot be modified or deleted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_events_immutable
    BEFORE UPDATE OR DELETE ON audit.audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit.prevent_audit_modification();

-- Comment for documentation
COMMENT ON TABLE audit.audit_events IS 'Immutable audit trail using event sourcing pattern. No updates or deletes allowed.';
COMMENT ON COLUMN audit.audit_events.aggregate_type IS 'Type of entity being audited (e.g., gl.journal_entry, ap.invoice)';
COMMENT ON COLUMN audit.audit_events.aggregate_id IS 'ID of the entity being audited';
COMMENT ON COLUMN audit.audit_events.event_type IS 'Type of event (e.g., created, updated, posted, reversed)';
COMMENT ON COLUMN audit.audit_events.event_version IS 'Sequence number for this aggregate (for ordering events)';
COMMENT ON COLUMN audit.audit_events.event_data IS 'JSON payload containing event-specific data';
COMMENT ON COLUMN audit.audit_events.metadata IS 'Additional context (correlation_id, source, etc.)';
