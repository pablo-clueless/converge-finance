-- Advanced Reporting (Drill-Down) Schema Extension
-- Adds drill-down capability and report bookmarks to the close schema

-- Report Drill-Down Links Table
CREATE TABLE close.report_drill_links (
    id CHAR(26) PRIMARY KEY,
    report_run_id CHAR(26) NOT NULL REFERENCES close.report_runs(id) ON DELETE CASCADE,
    report_row_id CHAR(26) NOT NULL REFERENCES close.report_data(id) ON DELETE CASCADE,
    link_type VARCHAR(50) NOT NULL,
    link_count INTEGER NOT NULL DEFAULT 0,
    filter_criteria JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Drill-Down Cache Table (for performance)
CREATE TABLE close.drill_down_cache (
    id CHAR(26) PRIMARY KEY,
    drill_link_id CHAR(26) NOT NULL REFERENCES close.report_drill_links(id) ON DELETE CASCADE,
    cache_key VARCHAR(100) NOT NULL,
    cached_data JSONB NOT NULL,
    row_count INTEGER NOT NULL,
    cached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    UNIQUE(drill_link_id, cache_key)
);

-- Report Bookmarks Table
CREATE TABLE close.report_bookmarks (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    user_id CHAR(26) NOT NULL,
    bookmark_name VARCHAR(100) NOT NULL,
    report_type close.report_type NOT NULL,
    parameters JSONB NOT NULL DEFAULT '{}',
    drill_path JSONB NOT NULL DEFAULT '[]',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_drill_links_report ON close.report_drill_links(report_run_id);
CREATE INDEX idx_drill_links_row ON close.report_drill_links(report_row_id);
CREATE INDEX idx_drill_links_type ON close.report_drill_links(link_type);

CREATE INDEX idx_drill_cache_link ON close.drill_down_cache(drill_link_id);
CREATE INDEX idx_drill_cache_key ON close.drill_down_cache(cache_key);
CREATE INDEX idx_drill_cache_expires ON close.drill_down_cache(expires_at);

CREATE INDEX idx_bookmarks_entity ON close.report_bookmarks(entity_id);
CREATE INDEX idx_bookmarks_user ON close.report_bookmarks(user_id);
CREATE INDEX idx_bookmarks_type ON close.report_bookmarks(report_type);
CREATE INDEX idx_bookmarks_default ON close.report_bookmarks(entity_id, user_id, report_type, is_default) WHERE is_default = true;

-- Triggers
CREATE TRIGGER update_report_bookmarks_updated_at
    BEFORE UPDATE ON close.report_bookmarks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to clean expired cache
CREATE OR REPLACE FUNCTION close.clean_expired_drill_cache()
RETURNS INTEGER AS $$
DECLARE
    v_deleted INTEGER;
BEGIN
    DELETE FROM close.drill_down_cache
    WHERE expires_at < NOW();

    GET DIAGNOSTICS v_deleted = ROW_COUNT;
    RETURN v_deleted;
END;
$$ LANGUAGE plpgsql;

-- Function to get drill-down journals for a report row
CREATE OR REPLACE FUNCTION close.get_drill_down_journals(
    p_report_row_id CHAR(26),
    p_limit INTEGER DEFAULT 100,
    p_offset INTEGER DEFAULT 0
)
RETURNS TABLE (
    journal_entry_id CHAR(26),
    entry_number VARCHAR(50),
    entry_date DATE,
    posting_date DATE,
    description TEXT,
    account_code VARCHAR(50),
    account_name VARCHAR(100),
    debit_amount DECIMAL(18,4),
    credit_amount DECIMAL(18,4),
    status VARCHAR(20)
) AS $$
DECLARE
    v_account_id CHAR(26);
    v_period_id CHAR(26);
    v_entity_id CHAR(26);
BEGIN
    -- Get the account and period from the report row
    SELECT rd.account_id, rr.fiscal_period_id, rr.entity_id
    INTO v_account_id, v_period_id, v_entity_id
    FROM close.report_data rd
    JOIN close.report_runs rr ON rr.id = rd.report_run_id
    WHERE rd.id = p_report_row_id;

    IF v_account_id IS NULL THEN
        RETURN;
    END IF;

    RETURN QUERY
    SELECT
        je.id AS journal_entry_id,
        je.entry_number,
        je.entry_date::DATE,
        je.posting_date::DATE,
        je.description,
        a.account_code,
        a.name AS account_name,
        jl.debit_amount,
        jl.credit_amount,
        je.status::VARCHAR(20)
    FROM gl.journal_entries je
    JOIN gl.journal_lines jl ON jl.journal_entry_id = je.id
    JOIN gl.accounts a ON a.id = jl.account_id
    WHERE je.entity_id = v_entity_id
    AND je.fiscal_period_id = v_period_id
    AND jl.account_id = v_account_id
    AND je.status = 'posted'
    ORDER BY je.posting_date DESC, je.entry_number DESC
    LIMIT p_limit
    OFFSET p_offset;
END;
$$ LANGUAGE plpgsql;

-- RLS Policies
ALTER TABLE close.report_drill_links ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.drill_down_cache ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.report_bookmarks ENABLE ROW LEVEL SECURITY;

CREATE POLICY drill_links_entity_isolation ON close.report_drill_links
    USING (report_run_id IN (
        SELECT id FROM close.report_runs
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY drill_cache_entity_isolation ON close.drill_down_cache
    USING (drill_link_id IN (
        SELECT dl.id FROM close.report_drill_links dl
        JOIN close.report_runs rr ON rr.id = dl.report_run_id
        WHERE rr.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY bookmarks_entity_isolation ON close.report_bookmarks
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));
