-- Segment Reporting Schema (IFRS 8 / ASC 280)
-- Handles segment definitions, assignments, and compliance reporting

-- Create schema
CREATE SCHEMA IF NOT EXISTS segment;

-- Enums
CREATE TYPE segment.segment_type AS ENUM ('operating', 'geographic', 'product', 'customer', 'custom');
CREATE TYPE segment.allocation_basis AS ENUM ('direct', 'revenue', 'headcount', 'square_footage', 'custom');

-- Segments Table
CREATE TABLE segment.segments (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    segment_code VARCHAR(50) NOT NULL,
    segment_name VARCHAR(100) NOT NULL,
    segment_type segment.segment_type NOT NULL,
    parent_id CHAR(26) REFERENCES segment.segments(id),
    description TEXT,
    manager_id CHAR(26),
    is_reportable BOOLEAN NOT NULL DEFAULT true,
    is_active BOOLEAN NOT NULL DEFAULT true,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, segment_code)
);

-- Segment Hierarchy Table
CREATE TABLE segment.segment_hierarchy (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    hierarchy_code VARCHAR(50) NOT NULL,
    hierarchy_name VARCHAR(100) NOT NULL,
    segment_type segment.segment_type NOT NULL,
    description TEXT,
    is_primary BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, hierarchy_code)
);

-- Segment Assignments Table
CREATE TABLE segment.assignments (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    segment_id CHAR(26) NOT NULL REFERENCES segment.segments(id) ON DELETE CASCADE,
    assignment_type VARCHAR(50) NOT NULL,
    assignment_id CHAR(26) NOT NULL,
    allocation_percent DECIMAL(5,2) NOT NULL DEFAULT 100.00 CHECK (allocation_percent > 0 AND allocation_percent <= 100),
    effective_from DATE NOT NULL,
    effective_to DATE,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (effective_to IS NULL OR effective_to >= effective_from)
);

-- Intersegment Transactions Table
CREATE TABLE segment.intersegment_transactions (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    fiscal_period_id CHAR(26) NOT NULL,
    from_segment_id CHAR(26) NOT NULL REFERENCES segment.segments(id),
    to_segment_id CHAR(26) NOT NULL REFERENCES segment.segments(id),
    journal_entry_id CHAR(26),
    transaction_date DATE NOT NULL,
    description TEXT,
    amount DECIMAL(18,4) NOT NULL,
    currency_code CHAR(3) NOT NULL REFERENCES currencies(code),
    is_eliminated BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (from_segment_id != to_segment_id)
);

-- Segment Balances Table (materialized for reporting)
CREATE TABLE segment.balances (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    segment_id CHAR(26) NOT NULL REFERENCES segment.segments(id),
    fiscal_period_id CHAR(26) NOT NULL,
    account_id CHAR(26) NOT NULL,
    debit_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
    credit_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
    net_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
    currency_code CHAR(3) NOT NULL REFERENCES currencies(code),
    last_updated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, segment_id, fiscal_period_id, account_id)
);

-- Segment Reports Table
CREATE TABLE segment.reports (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    report_number VARCHAR(30) NOT NULL,
    report_name VARCHAR(100) NOT NULL,
    fiscal_period_id CHAR(26) NOT NULL,
    fiscal_year_id CHAR(26) NOT NULL,
    as_of_date DATE NOT NULL,
    segment_type segment.segment_type NOT NULL,
    hierarchy_id CHAR(26) REFERENCES segment.segment_hierarchy(id),
    include_intersegment BOOLEAN NOT NULL DEFAULT true,
    currency_code CHAR(3) NOT NULL REFERENCES currencies(code),
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    generated_by CHAR(26) NOT NULL,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, report_number)
);

-- Segment Report Data Table
CREATE TABLE segment.report_data (
    id CHAR(26) PRIMARY KEY,
    report_id CHAR(26) NOT NULL REFERENCES segment.reports(id) ON DELETE CASCADE,
    segment_id CHAR(26) NOT NULL REFERENCES segment.segments(id),
    row_type VARCHAR(20) NOT NULL,
    line_item VARCHAR(100) NOT NULL,
    amount DECIMAL(18,4) NOT NULL,
    intersegment_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
    external_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
    percentage_of_total DECIMAL(5,2),
    row_order INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_segment_segments_entity ON segment.segments(entity_id);
CREATE INDEX idx_segment_segments_type ON segment.segments(segment_type);
CREATE INDEX idx_segment_segments_parent ON segment.segments(parent_id);
CREATE INDEX idx_segment_segments_active ON segment.segments(entity_id, is_active) WHERE is_active = true;
CREATE INDEX idx_segment_segments_reportable ON segment.segments(entity_id, is_reportable) WHERE is_reportable = true;

CREATE INDEX idx_segment_hierarchy_entity ON segment.segment_hierarchy(entity_id);
CREATE INDEX idx_segment_hierarchy_type ON segment.segment_hierarchy(segment_type);
CREATE INDEX idx_segment_hierarchy_primary ON segment.segment_hierarchy(entity_id, is_primary) WHERE is_primary = true;

CREATE INDEX idx_segment_assignments_entity ON segment.assignments(entity_id);
CREATE INDEX idx_segment_assignments_segment ON segment.assignments(segment_id);
CREATE INDEX idx_segment_assignments_ref ON segment.assignments(assignment_type, assignment_id);
CREATE INDEX idx_segment_assignments_active ON segment.assignments(entity_id, is_active) WHERE is_active = true;
CREATE INDEX idx_segment_assignments_effective ON segment.assignments(effective_from, effective_to);

CREATE INDEX idx_segment_intersegment_entity ON segment.intersegment_transactions(entity_id);
CREATE INDEX idx_segment_intersegment_period ON segment.intersegment_transactions(fiscal_period_id);
CREATE INDEX idx_segment_intersegment_from ON segment.intersegment_transactions(from_segment_id);
CREATE INDEX idx_segment_intersegment_to ON segment.intersegment_transactions(to_segment_id);

CREATE INDEX idx_segment_balances_entity ON segment.balances(entity_id);
CREATE INDEX idx_segment_balances_segment ON segment.balances(segment_id);
CREATE INDEX idx_segment_balances_period ON segment.balances(fiscal_period_id);
CREATE INDEX idx_segment_balances_account ON segment.balances(account_id);

CREATE INDEX idx_segment_reports_entity ON segment.reports(entity_id);
CREATE INDEX idx_segment_reports_period ON segment.reports(fiscal_period_id);
CREATE INDEX idx_segment_reports_type ON segment.reports(segment_type);

CREATE INDEX idx_segment_report_data_report ON segment.report_data(report_id);
CREATE INDEX idx_segment_report_data_segment ON segment.report_data(segment_id);

-- Triggers
CREATE TRIGGER update_segment_segments_updated_at
    BEFORE UPDATE ON segment.segments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_segment_hierarchy_updated_at
    BEFORE UPDATE ON segment.segment_hierarchy
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_segment_assignments_updated_at
    BEFORE UPDATE ON segment.assignments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Report Number Generation Function
CREATE OR REPLACE FUNCTION segment.generate_report_number(p_entity_id CHAR(26), p_prefix VARCHAR(10))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_year TEXT;
    v_sequence INTEGER;
    v_report_number VARCHAR(30);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COALESCE(MAX(
        CAST(SUBSTRING(report_number FROM LENGTH(p_prefix) + 6 FOR 6) AS INTEGER)
    ), 0) + 1
    INTO v_sequence
    FROM segment.reports
    WHERE entity_id = p_entity_id
    AND report_number LIKE p_prefix || v_year || '%';

    v_report_number := p_prefix || v_year || '-' || LPAD(v_sequence::TEXT, 6, '0');

    RETURN v_report_number;
END;
$$ LANGUAGE plpgsql;

-- RLS Policies
ALTER TABLE segment.segments ENABLE ROW LEVEL SECURITY;
ALTER TABLE segment.segment_hierarchy ENABLE ROW LEVEL SECURITY;
ALTER TABLE segment.assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE segment.intersegment_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE segment.balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE segment.reports ENABLE ROW LEVEL SECURITY;
ALTER TABLE segment.report_data ENABLE ROW LEVEL SECURITY;

CREATE POLICY segment_segments_entity_isolation ON segment.segments
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY segment_hierarchy_entity_isolation ON segment.segment_hierarchy
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY segment_assignments_entity_isolation ON segment.assignments
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY segment_intersegment_entity_isolation ON segment.intersegment_transactions
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY segment_balances_entity_isolation ON segment.balances
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY segment_reports_entity_isolation ON segment.reports
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY segment_report_data_entity_isolation ON segment.report_data
    USING (report_id IN (
        SELECT id FROM segment.reports
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));
