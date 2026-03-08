-- Export Schema
-- Handles data exports, export templates, and scheduled exports

-- Create schema
CREATE SCHEMA IF NOT EXISTS export;

-- Enums
CREATE TYPE export.format AS ENUM ('csv', 'xlsx', 'pdf', 'json');
CREATE TYPE export.status AS ENUM ('pending', 'processing', 'completed', 'failed', 'expired');

-- Export Templates Table
-- Reusable export configurations
CREATE TABLE export.templates (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) REFERENCES entities(id), -- NULL for system templates
    template_code VARCHAR(30) NOT NULL,
    template_name VARCHAR(100) NOT NULL,
    module VARCHAR(20) NOT NULL, -- gl, ap, ar, etc.
    export_type VARCHAR(50) NOT NULL, -- trial_balance, invoice_list, etc.
    configuration JSONB NOT NULL DEFAULT '{}',
    -- Configuration can include:
    -- - columns: [{field: "account_code", header: "Account", width: 20}]
    -- - filters: {date_range: true, account_types: [...]}
    -- - styling: {header_color: "#000", font_size: 10}
    is_system BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Export Jobs Table
-- Export job queue
CREATE TABLE export.jobs (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    job_number VARCHAR(30) NOT NULL,
    template_id CHAR(26) REFERENCES export.templates(id),
    export_type VARCHAR(50) NOT NULL,
    format export.format NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}',
    status export.status NOT NULL DEFAULT 'pending',
    file_name VARCHAR(255),
    file_path VARCHAR(500),
    file_size BIGINT,
    mime_type VARCHAR(100),
    row_count INTEGER,
    error_message TEXT,
    expires_at TIMESTAMPTZ,
    requested_by CHAR(26) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    UNIQUE(entity_id, job_number)
);

-- Export Schedules Table
-- Scheduled exports
CREATE TABLE export.schedules (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    schedule_name VARCHAR(100) NOT NULL,
    template_id CHAR(26) NOT NULL REFERENCES export.templates(id),
    format export.format NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}',
    cron_expression VARCHAR(100) NOT NULL,
    recipients JSONB NOT NULL DEFAULT '[]', -- email addresses
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ,
    created_by CHAR(26) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_export_templates_entity ON export.templates(entity_id);
CREATE INDEX idx_export_templates_module ON export.templates(module);
CREATE INDEX idx_export_templates_export_type ON export.templates(export_type);
CREATE INDEX idx_export_templates_system ON export.templates(is_system);
CREATE INDEX idx_export_templates_active ON export.templates(entity_id, is_active);

CREATE INDEX idx_export_jobs_entity ON export.jobs(entity_id);
CREATE INDEX idx_export_jobs_template ON export.jobs(template_id);
CREATE INDEX idx_export_jobs_status ON export.jobs(status);
CREATE INDEX idx_export_jobs_requested_by ON export.jobs(requested_by);
CREATE INDEX idx_export_jobs_created_at ON export.jobs(created_at);
CREATE INDEX idx_export_jobs_expires_at ON export.jobs(expires_at);

CREATE INDEX idx_export_schedules_entity ON export.schedules(entity_id);
CREATE INDEX idx_export_schedules_template ON export.schedules(template_id);
CREATE INDEX idx_export_schedules_next_run ON export.schedules(next_run_at);
CREATE INDEX idx_export_schedules_active ON export.schedules(entity_id, is_active);

-- Triggers for updated_at
CREATE TRIGGER update_export_templates_updated_at BEFORE UPDATE ON export.templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_export_schedules_updated_at BEFORE UPDATE ON export.schedules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Job Number Generation Function
CREATE OR REPLACE FUNCTION export.generate_job_number(p_entity_id CHAR(26), p_prefix VARCHAR(10))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_year TEXT;
    v_sequence INTEGER;
    v_job_number VARCHAR(30);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COALESCE(MAX(
        CAST(SUBSTRING(job_number FROM LENGTH(p_prefix) + 6 FOR 6) AS INTEGER)
    ), 0) + 1
    INTO v_sequence
    FROM export.jobs
    WHERE entity_id = p_entity_id
    AND job_number LIKE p_prefix || v_year || '%';

    v_job_number := p_prefix || v_year || '-' || LPAD(v_sequence::TEXT, 6, '0');

    RETURN v_job_number;
END;
$$ LANGUAGE plpgsql;

-- RLS Policies
ALTER TABLE export.templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE export.jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE export.schedules ENABLE ROW LEVEL SECURITY;

-- RLS Policies for templates (system templates visible to all)
CREATE POLICY templates_entity_isolation ON export.templates
    USING (is_system = true OR entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- RLS Policies for jobs
CREATE POLICY jobs_entity_isolation ON export.jobs
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- RLS Policies for schedules
CREATE POLICY schedules_entity_isolation ON export.schedules
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));
