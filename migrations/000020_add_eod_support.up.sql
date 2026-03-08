-- EOD (End of Day) Support
-- Adds daily close type and EOD-specific tables for daily processing

-- Add 'day' to close_type enum
ALTER TYPE close.close_type ADD VALUE IF NOT EXISTS 'day' BEFORE 'period';

-- Add EOD-specific rule types
ALTER TYPE close.close_rule_type ADD VALUE IF NOT EXISTS 'daily_accrual' AFTER 'currency_revaluation';
ALTER TYPE close.close_rule_type ADD VALUE IF NOT EXISTS 'daily_reconciliation' AFTER 'daily_accrual';
ALTER TYPE close.close_rule_type ADD VALUE IF NOT EXISTS 'daily_valuation' AFTER 'daily_reconciliation';

-- EOD Run Status Enum
CREATE TYPE close.eod_status AS ENUM ('pending', 'in_progress', 'completed', 'failed', 'skipped');

-- EOD Task Type Enum
CREATE TYPE close.eod_task_type AS ENUM (
    'validate_transactions',
    'post_pending_batches',
    'run_reconciliation',
    'calculate_accruals',
    'fx_rate_update',
    'generate_daily_reports',
    'validate_balances',
    'rollover_date',
    'custom'
);

-- Business Date Table
-- Tracks the current business date for each entity
CREATE TABLE close.business_dates (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    current_business_date DATE NOT NULL,
    last_eod_date DATE,
    last_eod_run_id CHAR(26),
    next_business_date DATE,
    is_holiday BOOLEAN NOT NULL DEFAULT false,
    holiday_name VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id)
);

-- EOD Configuration Table
-- Defines EOD settings per entity
CREATE TABLE close.eod_config (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    eod_cutoff_time TIME NOT NULL DEFAULT '17:00:00',
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    auto_rollover BOOLEAN NOT NULL DEFAULT false,
    require_zero_suspense BOOLEAN NOT NULL DEFAULT true,
    require_balanced_books BOOLEAN NOT NULL DEFAULT true,
    skip_weekends BOOLEAN NOT NULL DEFAULT true,
    skip_holidays BOOLEAN NOT NULL DEFAULT true,
    notify_on_completion BOOLEAN NOT NULL DEFAULT true,
    notify_on_failure BOOLEAN NOT NULL DEFAULT true,
    notification_emails JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id)
);

-- EOD Runs Table
-- Tracks each EOD execution
CREATE TABLE close.eod_runs (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    run_number VARCHAR(30) NOT NULL,
    business_date DATE NOT NULL,
    status close.eod_status NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    total_tasks INTEGER NOT NULL DEFAULT 0,
    completed_tasks INTEGER NOT NULL DEFAULT 0,
    failed_tasks INTEGER NOT NULL DEFAULT 0,
    skipped_tasks INTEGER NOT NULL DEFAULT 0,
    journal_entries_posted INTEGER NOT NULL DEFAULT 0,
    transactions_validated INTEGER NOT NULL DEFAULT 0,
    warnings JSONB NOT NULL DEFAULT '[]',
    errors JSONB NOT NULL DEFAULT '[]',
    initiated_by CHAR(26) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, run_number),
    UNIQUE(entity_id, business_date)
);

-- EOD Tasks Table
-- Defines tasks to run during EOD
CREATE TABLE close.eod_tasks (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    task_code VARCHAR(30) NOT NULL,
    task_name VARCHAR(100) NOT NULL,
    task_type close.eod_task_type NOT NULL,
    sequence_number INTEGER NOT NULL DEFAULT 1,
    is_required BOOLEAN NOT NULL DEFAULT true,
    is_active BOOLEAN NOT NULL DEFAULT true,
    configuration JSONB NOT NULL DEFAULT '{}',
    -- Configuration examples:
    -- validate_transactions: {"include_pending": true, "fail_on_unposted": true}
    -- run_reconciliation: {"accounts": ["1000", "1100"], "tolerance": 0.01}
    -- calculate_accruals: {"accrual_accounts": [...]}
    -- generate_daily_reports: {"reports": ["daily_activity", "cash_position"]}
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, task_code)
);

-- EOD Task Runs Table
-- Tracks execution of each task within an EOD run
CREATE TABLE close.eod_task_runs (
    id CHAR(26) PRIMARY KEY,
    eod_run_id CHAR(26) NOT NULL REFERENCES close.eod_runs(id) ON DELETE CASCADE,
    eod_task_id CHAR(26) NOT NULL REFERENCES close.eod_tasks(id),
    task_code VARCHAR(30) NOT NULL,
    task_name VARCHAR(100) NOT NULL,
    sequence_number INTEGER NOT NULL,
    status close.eod_status NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,
    records_processed INTEGER NOT NULL DEFAULT 0,
    records_failed INTEGER NOT NULL DEFAULT 0,
    result_summary JSONB NOT NULL DEFAULT '{}',
    error_message TEXT,
    warnings JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Holiday Calendar Table
-- Defines holidays for EOD scheduling
CREATE TABLE close.holiday_calendar (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    holiday_date DATE NOT NULL,
    holiday_name VARCHAR(100) NOT NULL,
    holiday_type VARCHAR(20) NOT NULL DEFAULT 'bank', -- bank, national, company
    is_recurring BOOLEAN NOT NULL DEFAULT false,
    recurring_month INTEGER CHECK (recurring_month BETWEEN 1 AND 12),
    recurring_day INTEGER CHECK (recurring_day BETWEEN 1 AND 31),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, holiday_date)
);

-- Daily Reconciliation Results Table
-- Stores results of daily reconciliation checks
CREATE TABLE close.daily_reconciliation (
    id CHAR(26) PRIMARY KEY,
    eod_run_id CHAR(26) NOT NULL REFERENCES close.eod_runs(id) ON DELETE CASCADE,
    account_id CHAR(26) NOT NULL,
    account_code VARCHAR(50) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    expected_balance DECIMAL(19,4) NOT NULL,
    actual_balance DECIMAL(19,4) NOT NULL,
    difference DECIMAL(19,4) NOT NULL,
    currency_code CHAR(3) NOT NULL,
    is_reconciled BOOLEAN NOT NULL,
    reconciliation_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_business_dates_entity ON close.business_dates(entity_id);
CREATE INDEX idx_business_dates_current ON close.business_dates(current_business_date);

CREATE INDEX idx_eod_config_entity ON close.eod_config(entity_id);

CREATE INDEX idx_eod_runs_entity ON close.eod_runs(entity_id);
CREATE INDEX idx_eod_runs_date ON close.eod_runs(business_date);
CREATE INDEX idx_eod_runs_status ON close.eod_runs(status);
CREATE INDEX idx_eod_runs_entity_date ON close.eod_runs(entity_id, business_date);

CREATE INDEX idx_eod_tasks_entity ON close.eod_tasks(entity_id);
CREATE INDEX idx_eod_tasks_sequence ON close.eod_tasks(entity_id, sequence_number);
CREATE INDEX idx_eod_tasks_active ON close.eod_tasks(entity_id, is_active) WHERE is_active = true;

CREATE INDEX idx_eod_task_runs_eod ON close.eod_task_runs(eod_run_id);
CREATE INDEX idx_eod_task_runs_task ON close.eod_task_runs(eod_task_id);
CREATE INDEX idx_eod_task_runs_status ON close.eod_task_runs(status);
CREATE INDEX idx_eod_task_runs_sequence ON close.eod_task_runs(eod_run_id, sequence_number);

CREATE INDEX idx_holiday_calendar_entity ON close.holiday_calendar(entity_id);
CREATE INDEX idx_holiday_calendar_date ON close.holiday_calendar(holiday_date);
CREATE INDEX idx_holiday_calendar_entity_date ON close.holiday_calendar(entity_id, holiday_date);

CREATE INDEX idx_daily_reconciliation_eod ON close.daily_reconciliation(eod_run_id);
CREATE INDEX idx_daily_reconciliation_account ON close.daily_reconciliation(account_id);
CREATE INDEX idx_daily_reconciliation_reconciled ON close.daily_reconciliation(is_reconciled);

-- Triggers
CREATE TRIGGER update_business_dates_updated_at BEFORE UPDATE ON close.business_dates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_eod_config_updated_at BEFORE UPDATE ON close.eod_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_eod_runs_updated_at BEFORE UPDATE ON close.eod_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_eod_tasks_updated_at BEFORE UPDATE ON close.eod_tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_eod_task_runs_updated_at BEFORE UPDATE ON close.eod_task_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- RLS Policies
ALTER TABLE close.business_dates ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.eod_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.eod_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.eod_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.eod_task_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.holiday_calendar ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.daily_reconciliation ENABLE ROW LEVEL SECURITY;

CREATE POLICY business_dates_entity_isolation ON close.business_dates
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY eod_config_entity_isolation ON close.eod_config
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY eod_runs_entity_isolation ON close.eod_runs
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY eod_tasks_entity_isolation ON close.eod_tasks
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY eod_task_runs_entity_isolation ON close.eod_task_runs
    USING (eod_run_id IN (
        SELECT id FROM close.eod_runs
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY holiday_calendar_entity_isolation ON close.holiday_calendar
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY daily_reconciliation_entity_isolation ON close.daily_reconciliation
    USING (eod_run_id IN (
        SELECT id FROM close.eod_runs
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

-- EOD Run Number Generation Function
CREATE OR REPLACE FUNCTION close.generate_eod_run_number(p_entity_id CHAR(26), p_business_date DATE)
RETURNS VARCHAR(30) AS $$
DECLARE
    v_run_number VARCHAR(30);
BEGIN
    v_run_number := 'EOD' || TO_CHAR(p_business_date, 'YYYYMMDD');
    RETURN v_run_number;
END;
$$ LANGUAGE plpgsql;

-- Function to get next business date
CREATE OR REPLACE FUNCTION close.get_next_business_date(
    p_entity_id CHAR(26),
    p_from_date DATE
) RETURNS DATE AS $$
DECLARE
    v_next_date DATE;
    v_config RECORD;
    v_is_holiday BOOLEAN;
BEGIN
    -- Get EOD config
    SELECT * INTO v_config FROM close.eod_config WHERE entity_id = p_entity_id;

    v_next_date := p_from_date + 1;

    -- Skip weekends and holidays if configured
    LOOP
        v_is_holiday := FALSE;

        -- Check if weekend
        IF v_config.skip_weekends AND EXTRACT(DOW FROM v_next_date) IN (0, 6) THEN
            v_next_date := v_next_date + 1;
            CONTINUE;
        END IF;

        -- Check if holiday
        IF v_config.skip_holidays THEN
            SELECT TRUE INTO v_is_holiday
            FROM close.holiday_calendar
            WHERE entity_id = p_entity_id AND holiday_date = v_next_date;

            IF v_is_holiday THEN
                v_next_date := v_next_date + 1;
                CONTINUE;
            END IF;
        END IF;

        EXIT;
    END LOOP;

    RETURN v_next_date;
END;
$$ LANGUAGE plpgsql;

-- Insert default EOD tasks for new entities (can be customized per entity)
-- This is a template that can be copied when setting up a new entity
COMMENT ON TABLE close.eod_tasks IS 'Default EOD tasks should be created when entity is set up. Typical tasks:
1. validate_transactions - Ensure all transactions are valid
2. post_pending_batches - Post any pending batch entries
3. run_reconciliation - Reconcile key accounts
4. calculate_accruals - Calculate daily accruals
5. fx_rate_update - Update FX rates if needed
6. validate_balances - Validate trial balance
7. generate_daily_reports - Generate daily activity reports
8. rollover_date - Roll business date forward';
