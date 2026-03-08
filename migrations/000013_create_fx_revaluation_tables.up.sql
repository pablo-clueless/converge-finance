-- FX Revaluation Schema
-- Handles foreign currency revaluation for monetary accounts
-- Extends fx schema (created in 000012)

-- Enums
CREATE TYPE fx.revaluation_type AS ENUM ('unrealized', 'realized');
CREATE TYPE fx.revaluation_status AS ENUM ('draft', 'pending_approval', 'approved', 'posted', 'reversed');
CREATE TYPE fx.account_fx_treatment AS ENUM ('monetary', 'nonmonetary', 'excluded');

-- Account FX Configuration Table
-- Defines how each account should be treated for FX revaluation
CREATE TABLE fx.account_fx_config (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    account_id CHAR(26) NOT NULL REFERENCES gl.accounts(id) ON DELETE CASCADE,
    fx_treatment fx.account_fx_treatment NOT NULL DEFAULT 'monetary',
    revaluation_gain_account_id CHAR(26) REFERENCES gl.accounts(id),
    revaluation_loss_account_id CHAR(26) REFERENCES gl.accounts(id),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, account_id)
);

-- Revaluation Runs Table
-- Tracks each FX revaluation batch process
CREATE TABLE fx.revaluation_runs (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    run_number VARCHAR(30) NOT NULL,
    fiscal_period_id CHAR(26) NOT NULL,
    revaluation_date DATE NOT NULL,
    rate_date DATE NOT NULL,
    functional_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    status fx.revaluation_status NOT NULL DEFAULT 'draft',
    total_unrealized_gain DECIMAL(19,4) NOT NULL DEFAULT 0,
    total_unrealized_loss DECIMAL(19,4) NOT NULL DEFAULT 0,
    net_revaluation DECIMAL(19,4) NOT NULL DEFAULT 0,
    accounts_processed INTEGER NOT NULL DEFAULT 0,
    journal_entry_id CHAR(26),
    reversal_journal_entry_id CHAR(26),
    created_by CHAR(26) NOT NULL,
    approved_by CHAR(26),
    posted_by CHAR(26),
    reversed_by CHAR(26),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at TIMESTAMPTZ,
    posted_at TIMESTAMPTZ,
    reversed_at TIMESTAMPTZ,
    UNIQUE(entity_id, run_number)
);

-- Revaluation Details Table
-- Per-account revaluation calculations
CREATE TABLE fx.revaluation_details (
    id CHAR(26) PRIMARY KEY,
    revaluation_run_id CHAR(26) NOT NULL REFERENCES fx.revaluation_runs(id) ON DELETE CASCADE,
    account_id CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    account_code VARCHAR(50) NOT NULL,
    account_name VARCHAR(255) NOT NULL,
    original_currency CHAR(3) NOT NULL REFERENCES currencies(code),
    original_balance DECIMAL(19,4) NOT NULL,
    original_rate DECIMAL(18,8) NOT NULL,
    original_functional_amount DECIMAL(19,4) NOT NULL,
    new_rate DECIMAL(18,8) NOT NULL,
    new_functional_amount DECIMAL(19,4) NOT NULL,
    revaluation_amount DECIMAL(19,4) NOT NULL,
    revaluation_type fx.revaluation_type NOT NULL DEFAULT 'unrealized',
    gain_loss_account_id CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for account_fx_config
CREATE INDEX idx_fx_account_config_entity ON fx.account_fx_config(entity_id);
CREATE INDEX idx_fx_account_config_account ON fx.account_fx_config(account_id);
CREATE INDEX idx_fx_account_config_treatment ON fx.account_fx_config(fx_treatment);
CREATE INDEX idx_fx_account_config_active ON fx.account_fx_config(entity_id, is_active) WHERE is_active = true;
CREATE INDEX idx_fx_account_config_gain_account ON fx.account_fx_config(revaluation_gain_account_id);
CREATE INDEX idx_fx_account_config_loss_account ON fx.account_fx_config(revaluation_loss_account_id);

-- Indexes for revaluation_runs
CREATE INDEX idx_fx_revaluation_runs_entity ON fx.revaluation_runs(entity_id);
CREATE INDEX idx_fx_revaluation_runs_period ON fx.revaluation_runs(fiscal_period_id);
CREATE INDEX idx_fx_revaluation_runs_date ON fx.revaluation_runs(revaluation_date);
CREATE INDEX idx_fx_revaluation_runs_rate_date ON fx.revaluation_runs(rate_date);
CREATE INDEX idx_fx_revaluation_runs_status ON fx.revaluation_runs(status);
CREATE INDEX idx_fx_revaluation_runs_currency ON fx.revaluation_runs(functional_currency);
CREATE INDEX idx_fx_revaluation_runs_journal ON fx.revaluation_runs(journal_entry_id);
CREATE INDEX idx_fx_revaluation_runs_reversal ON fx.revaluation_runs(reversal_journal_entry_id);
CREATE INDEX idx_fx_revaluation_runs_created_by ON fx.revaluation_runs(created_by);
CREATE INDEX idx_fx_revaluation_runs_approved_by ON fx.revaluation_runs(approved_by);
CREATE INDEX idx_fx_revaluation_runs_posted_by ON fx.revaluation_runs(posted_by);

-- Indexes for revaluation_details
CREATE INDEX idx_fx_revaluation_details_run ON fx.revaluation_details(revaluation_run_id);
CREATE INDEX idx_fx_revaluation_details_account ON fx.revaluation_details(account_id);
CREATE INDEX idx_fx_revaluation_details_currency ON fx.revaluation_details(original_currency);
CREATE INDEX idx_fx_revaluation_details_type ON fx.revaluation_details(revaluation_type);
CREATE INDEX idx_fx_revaluation_details_gain_loss ON fx.revaluation_details(gain_loss_account_id);

-- Triggers for updated_at
CREATE TRIGGER update_fx_account_fx_config_updated_at
    BEFORE UPDATE ON fx.account_fx_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_fx_revaluation_runs_updated_at
    BEFORE UPDATE ON fx.revaluation_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- RLS Policies
ALTER TABLE fx.account_fx_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE fx.revaluation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE fx.revaluation_details ENABLE ROW LEVEL SECURITY;

CREATE POLICY fx_account_fx_config_entity_isolation ON fx.account_fx_config
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY fx_revaluation_runs_entity_isolation ON fx.revaluation_runs
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY fx_revaluation_details_entity_isolation ON fx.revaluation_details
    USING (revaluation_run_id IN (
        SELECT id FROM fx.revaluation_runs
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

-- Run Number Generation Function
CREATE OR REPLACE FUNCTION fx.generate_revaluation_run_number(p_entity_id CHAR(26))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_year TEXT;
    v_sequence INTEGER;
    v_run_number VARCHAR(30);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COALESCE(MAX(
        CAST(SUBSTRING(run_number FROM 9 FOR 6) AS INTEGER)
    ), 0) + 1
    INTO v_sequence
    FROM fx.revaluation_runs
    WHERE entity_id = p_entity_id
    AND run_number LIKE 'FXRV' || v_year || '%';

    v_run_number := 'FXRV' || v_year || '-' || LPAD(v_sequence::TEXT, 6, '0');

    RETURN v_run_number;
END;
$$ LANGUAGE plpgsql;
