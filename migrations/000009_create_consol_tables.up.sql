-- =============================================
-- GROUP CONSOLIDATION TABLES
-- Consolidation, Currency Translation, and Minority Interest
-- =============================================

-- Create consolidation schema
CREATE SCHEMA IF NOT EXISTS consol;

-- =============================================
-- ENUMS
-- =============================================

-- Currency translation method
CREATE TYPE consol.translation_method AS ENUM (
    'current_rate',       -- All items at current rate (common for foreign subsidiaries)
    'temporal',           -- Monetary at current, non-monetary at historical
    'monetary_nonmonetary' -- Monetary vs non-monetary classification
);

-- Consolidation run status
CREATE TYPE consol.run_status AS ENUM (
    'draft',
    'in_progress',
    'completed',
    'posted',
    'reversed'
);

-- Translation adjustment type
CREATE TYPE consol.adjustment_type AS ENUM (
    'cta',                -- Cumulative Translation Adjustment
    'remeasurement_gain_loss', -- Remeasurement for temporal method
    'minority_interest',  -- NCI adjustments
    'elimination',        -- IC elimination adjustments
    'manual'              -- Manual adjustments
);

-- =============================================
-- CONSOLIDATION SETS
-- Define groups of entities to consolidate
-- =============================================

CREATE TABLE consol.consolidation_sets (
    id                      CHAR(26) PRIMARY KEY,
    set_code                VARCHAR(50) NOT NULL,
    set_name                VARCHAR(255) NOT NULL,
    description             TEXT,

    -- Parent/holding entity that owns the consolidation
    parent_entity_id        CHAR(26) NOT NULL REFERENCES entities(id),

    -- Reporting currency for consolidated statements
    reporting_currency      CHAR(3) NOT NULL REFERENCES currencies(code),

    -- Default translation method
    default_translation_method consol.translation_method NOT NULL DEFAULT 'current_rate',

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_sets_uk UNIQUE (parent_entity_id, set_code)
);

CREATE INDEX idx_consol_sets_parent ON consol.consolidation_sets(parent_entity_id);

-- =============================================
-- CONSOLIDATION SET MEMBERS
-- Entities included in a consolidation set
-- =============================================

CREATE TABLE consol.consolidation_set_members (
    id                      CHAR(26) PRIMARY KEY,
    consolidation_set_id    CHAR(26) NOT NULL REFERENCES consol.consolidation_sets(id) ON DELETE CASCADE,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),

    -- Ownership and consolidation details
    ownership_percent       DECIMAL(5,2) NOT NULL DEFAULT 100.00
        CHECK (ownership_percent > 0 AND ownership_percent <= 100),
    consolidation_method    consolidation_method NOT NULL DEFAULT 'full',

    -- Translation method for this entity (overrides set default)
    translation_method      consol.translation_method,

    -- Functional currency of this entity
    functional_currency     CHAR(3) NOT NULL REFERENCES currencies(code),

    -- Sequence for processing order
    sequence_number         INTEGER NOT NULL DEFAULT 0,

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_set_members_uk UNIQUE (consolidation_set_id, entity_id)
);

CREATE INDEX idx_consol_members_set ON consol.consolidation_set_members(consolidation_set_id);
CREATE INDEX idx_consol_members_entity ON consol.consolidation_set_members(entity_id);

-- =============================================
-- EXCHANGE RATES
-- Historical exchange rates for translation
-- =============================================

CREATE TABLE consol.exchange_rates (
    id                      CHAR(26) PRIMARY KEY,
    from_currency           CHAR(3) NOT NULL REFERENCES currencies(code),
    to_currency             CHAR(3) NOT NULL REFERENCES currencies(code),
    rate_date               DATE NOT NULL,

    -- Different rate types for translation
    closing_rate            DECIMAL(18,8) NOT NULL CHECK (closing_rate > 0),
    average_rate            DECIMAL(18,8) CHECK (average_rate > 0),
    historical_rate         DECIMAL(18,8) CHECK (historical_rate > 0),

    source                  VARCHAR(50),  -- e.g., 'ECB', 'MANUAL', 'API'

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_exchange_rates_uk UNIQUE (from_currency, to_currency, rate_date)
);

CREATE INDEX idx_consol_rates_date ON consol.exchange_rates(rate_date);
CREATE INDEX idx_consol_rates_pair ON consol.exchange_rates(from_currency, to_currency);

-- =============================================
-- CONSOLIDATION RUNS
-- Actual consolidation process execution
-- =============================================

CREATE TABLE consol.consolidation_runs (
    id                      CHAR(26) PRIMARY KEY,
    run_number              VARCHAR(30) NOT NULL,
    consolidation_set_id    CHAR(26) NOT NULL REFERENCES consol.consolidation_sets(id),
    fiscal_period_id        CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),

    -- Reporting details
    reporting_currency      CHAR(3) NOT NULL REFERENCES currencies(code),
    consolidation_date      DATE NOT NULL,

    -- Exchange rates used
    closing_rate_date       DATE NOT NULL,
    average_rate_date       DATE,

    -- Run statistics
    entity_count            INTEGER NOT NULL DEFAULT 0,
    total_assets            DECIMAL(18,4) DEFAULT 0,
    total_liabilities       DECIMAL(18,4) DEFAULT 0,
    total_equity            DECIMAL(18,4) DEFAULT 0,
    total_revenue           DECIMAL(18,4) DEFAULT 0,
    total_expenses          DECIMAL(18,4) DEFAULT 0,
    net_income              DECIMAL(18,4) DEFAULT 0,

    -- Translation adjustment totals
    total_cta               DECIMAL(18,4) DEFAULT 0,

    -- Minority interest
    total_minority_interest DECIMAL(18,4) DEFAULT 0,

    -- Status
    status                  consol.run_status NOT NULL DEFAULT 'draft',

    -- Journal entry for posting to GL (in parent entity)
    journal_entry_id        CHAR(26) REFERENCES gl.journal_entries(id),

    -- Audit
    created_by              CHAR(26) NOT NULL,
    completed_by            CHAR(26),
    posted_by               CHAR(26),
    reversed_by             CHAR(26),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at            TIMESTAMPTZ,
    posted_at               TIMESTAMPTZ,
    reversed_at             TIMESTAMPTZ,

    CONSTRAINT consol_runs_uk UNIQUE (consolidation_set_id, run_number)
);

CREATE INDEX idx_consol_runs_set ON consol.consolidation_runs(consolidation_set_id);
CREATE INDEX idx_consol_runs_period ON consol.consolidation_runs(fiscal_period_id);
CREATE INDEX idx_consol_runs_status ON consol.consolidation_runs(status);
CREATE INDEX idx_consol_runs_date ON consol.consolidation_runs(consolidation_date);

-- =============================================
-- ENTITY BALANCES (per run)
-- Translated balances for each entity in a run
-- =============================================

CREATE TABLE consol.entity_balances (
    id                      CHAR(26) PRIMARY KEY,
    consolidation_run_id    CHAR(26) NOT NULL REFERENCES consol.consolidation_runs(id) ON DELETE CASCADE,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    account_id              CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    -- Original functional currency amounts
    functional_currency     CHAR(3) NOT NULL REFERENCES currencies(code),
    functional_debit        DECIMAL(18,4) NOT NULL DEFAULT 0,
    functional_credit       DECIMAL(18,4) NOT NULL DEFAULT 0,
    functional_balance      DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Exchange rate used
    exchange_rate           DECIMAL(18,8) NOT NULL DEFAULT 1,
    rate_type               VARCHAR(20) NOT NULL DEFAULT 'closing', -- closing, average, historical

    -- Translated reporting currency amounts
    translated_debit        DECIMAL(18,4) NOT NULL DEFAULT 0,
    translated_credit       DECIMAL(18,4) NOT NULL DEFAULT 0,
    translated_balance      DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Translation difference
    translation_difference  DECIMAL(18,4) NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_entity_balances_uk
        UNIQUE (consolidation_run_id, entity_id, account_id)
);

CREATE INDEX idx_consol_entity_bal_run ON consol.entity_balances(consolidation_run_id);
CREATE INDEX idx_consol_entity_bal_entity ON consol.entity_balances(entity_id);
CREATE INDEX idx_consol_entity_bal_account ON consol.entity_balances(account_id);

-- =============================================
-- CONSOLIDATED BALANCES
-- Final consolidated trial balance
-- =============================================

CREATE TABLE consol.consolidated_balances (
    id                      CHAR(26) PRIMARY KEY,
    consolidation_run_id    CHAR(26) NOT NULL REFERENCES consol.consolidation_runs(id) ON DELETE CASCADE,
    account_id              CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    -- Aggregated balances (in reporting currency)
    opening_balance         DECIMAL(18,4) NOT NULL DEFAULT 0,
    period_debit            DECIMAL(18,4) NOT NULL DEFAULT 0,
    period_credit           DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Elimination adjustments
    elimination_debit       DECIMAL(18,4) NOT NULL DEFAULT 0,
    elimination_credit      DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Translation adjustments
    translation_adjustment  DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Minority interest portion (for BS accounts)
    minority_interest       DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Final consolidated balance
    closing_balance         DECIMAL(18,4) NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_consolidated_bal_uk
        UNIQUE (consolidation_run_id, account_id)
);

CREATE INDEX idx_consol_bal_run ON consol.consolidated_balances(consolidation_run_id);
CREATE INDEX idx_consol_bal_account ON consol.consolidated_balances(account_id);

-- =============================================
-- TRANSLATION ADJUSTMENTS
-- Track CTA and other translation adjustments
-- =============================================

CREATE TABLE consol.translation_adjustments (
    id                      CHAR(26) PRIMARY KEY,
    consolidation_run_id    CHAR(26) NOT NULL REFERENCES consol.consolidation_runs(id) ON DELETE CASCADE,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),

    adjustment_type         consol.adjustment_type NOT NULL,

    -- Account for the adjustment
    account_id              CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    description             TEXT,

    -- Amount
    debit_amount            DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),
    credit_amount           DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),

    -- Currency info
    functional_currency     CHAR(3) NOT NULL REFERENCES currencies(code),
    reporting_currency      CHAR(3) NOT NULL REFERENCES currencies(code),
    exchange_rate           DECIMAL(18,8) NOT NULL,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_trans_adj_debit_or_credit
        CHECK ((debit_amount = 0 AND credit_amount > 0) OR
               (debit_amount > 0 AND credit_amount = 0) OR
               (debit_amount = 0 AND credit_amount = 0))
);

CREATE INDEX idx_consol_trans_adj_run ON consol.translation_adjustments(consolidation_run_id);
CREATE INDEX idx_consol_trans_adj_entity ON consol.translation_adjustments(entity_id);
CREATE INDEX idx_consol_trans_adj_type ON consol.translation_adjustments(adjustment_type);

-- =============================================
-- MINORITY INTEREST (Non-Controlling Interest)
-- Track NCI for partially-owned subsidiaries
-- =============================================

CREATE TABLE consol.minority_interest (
    id                      CHAR(26) PRIMARY KEY,
    consolidation_run_id    CHAR(26) NOT NULL REFERENCES consol.consolidation_runs(id) ON DELETE CASCADE,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),

    -- Ownership
    ownership_percent       DECIMAL(5,2) NOT NULL,
    minority_percent        DECIMAL(5,2) NOT NULL,  -- 100 - ownership_percent

    -- Balances
    total_equity            DECIMAL(18,4) NOT NULL DEFAULT 0,
    minority_share_equity   DECIMAL(18,4) NOT NULL DEFAULT 0,

    net_income              DECIMAL(18,4) NOT NULL DEFAULT 0,
    minority_share_income   DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Dividends to minority
    dividends_to_minority   DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Accumulated NCI balance
    opening_nci             DECIMAL(18,4) NOT NULL DEFAULT 0,
    closing_nci             DECIMAL(18,4) NOT NULL DEFAULT 0,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_minority_interest_uk
        UNIQUE (consolidation_run_id, entity_id)
);

CREATE INDEX idx_consol_mi_run ON consol.minority_interest(consolidation_run_id);
CREATE INDEX idx_consol_mi_entity ON consol.minority_interest(entity_id);

-- =============================================
-- ACCOUNT MAPPINGS FOR CONSOLIDATION
-- Map subsidiary accounts to consolidated COA
-- =============================================

CREATE TABLE consol.account_mappings (
    id                      CHAR(26) PRIMARY KEY,
    consolidation_set_id    CHAR(26) NOT NULL REFERENCES consol.consolidation_sets(id) ON DELETE CASCADE,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),

    -- Source account in subsidiary
    source_account_id       CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    -- Target account in consolidated COA
    target_account_id       CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    -- Rate type to use for translation
    rate_type               VARCHAR(20) NOT NULL DEFAULT 'closing',

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT consol_account_mappings_uk
        UNIQUE (consolidation_set_id, entity_id, source_account_id)
);

CREATE INDEX idx_consol_acc_map_set ON consol.account_mappings(consolidation_set_id);
CREATE INDEX idx_consol_acc_map_entity ON consol.account_mappings(entity_id);
CREATE INDEX idx_consol_acc_map_source ON consol.account_mappings(source_account_id);

-- =============================================
-- FUNCTIONS
-- =============================================

-- Function to generate consolidation run number
CREATE OR REPLACE FUNCTION consol.generate_run_number(p_set_id CHAR(26))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_count INTEGER;
    v_year VARCHAR(4);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COUNT(*) + 1 INTO v_count
    FROM consol.consolidation_runs
    WHERE consolidation_set_id = p_set_id
    AND run_number LIKE 'CONSOL-' || v_year || '-%';

    RETURN 'CONSOL-' || v_year || '-' || LPAD(v_count::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

-- Function to get exchange rate
CREATE OR REPLACE FUNCTION consol.get_exchange_rate(
    p_from_currency CHAR(3),
    p_to_currency CHAR(3),
    p_rate_date DATE,
    p_rate_type VARCHAR(20) DEFAULT 'closing'
)
RETURNS DECIMAL(18,8) AS $$
DECLARE
    v_rate DECIMAL(18,8);
BEGIN
    IF p_from_currency = p_to_currency THEN
        RETURN 1.0;
    END IF;

    SELECT
        CASE p_rate_type
            WHEN 'average' THEN COALESCE(average_rate, closing_rate)
            WHEN 'historical' THEN COALESCE(historical_rate, closing_rate)
            ELSE closing_rate
        END
    INTO v_rate
    FROM consol.exchange_rates
    WHERE from_currency = p_from_currency
    AND to_currency = p_to_currency
    AND rate_date <= p_rate_date
    ORDER BY rate_date DESC
    LIMIT 1;

    RETURN COALESCE(v_rate, 1.0);
END;
$$ LANGUAGE plpgsql;

-- =============================================
-- TRIGGERS
-- =============================================

-- Updated_at triggers
CREATE TRIGGER trg_consol_sets_updated_at
    BEFORE UPDATE ON consol.consolidation_sets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_consol_members_updated_at
    BEFORE UPDATE ON consol.consolidation_set_members
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_consol_rates_updated_at
    BEFORE UPDATE ON consol.exchange_rates
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_consol_runs_updated_at
    BEFORE UPDATE ON consol.consolidation_runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_consol_acc_mappings_updated_at
    BEFORE UPDATE ON consol.account_mappings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- ROW LEVEL SECURITY
-- =============================================

ALTER TABLE consol.consolidation_sets ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.consolidation_set_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.exchange_rates ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.consolidation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.entity_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.consolidated_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.translation_adjustments ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.minority_interest ENABLE ROW LEVEL SECURITY;
ALTER TABLE consol.account_mappings ENABLE ROW LEVEL SECURITY;

-- RLS Policies - consolidation data accessible by parent entity
CREATE POLICY consol_sets_entity_isolation ON consol.consolidation_sets
    USING (parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY consol_members_entity_isolation ON consol.consolidation_set_members
    USING (EXISTS (
        SELECT 1 FROM consol.consolidation_sets s
        WHERE s.id = consolidation_set_id
        AND s.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

-- Exchange rates are global (no entity isolation)
CREATE POLICY consol_rates_all_access ON consol.exchange_rates
    USING (TRUE);

CREATE POLICY consol_runs_entity_isolation ON consol.consolidation_runs
    USING (EXISTS (
        SELECT 1 FROM consol.consolidation_sets s
        WHERE s.id = consolidation_set_id
        AND s.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY consol_entity_bal_entity_isolation ON consol.entity_balances
    USING (EXISTS (
        SELECT 1 FROM consol.consolidation_runs r
        JOIN consol.consolidation_sets s ON s.id = r.consolidation_set_id
        WHERE r.id = consolidation_run_id
        AND s.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY consol_consolidated_bal_entity_isolation ON consol.consolidated_balances
    USING (EXISTS (
        SELECT 1 FROM consol.consolidation_runs r
        JOIN consol.consolidation_sets s ON s.id = r.consolidation_set_id
        WHERE r.id = consolidation_run_id
        AND s.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY consol_trans_adj_entity_isolation ON consol.translation_adjustments
    USING (EXISTS (
        SELECT 1 FROM consol.consolidation_runs r
        JOIN consol.consolidation_sets s ON s.id = r.consolidation_set_id
        WHERE r.id = consolidation_run_id
        AND s.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY consol_mi_entity_isolation ON consol.minority_interest
    USING (EXISTS (
        SELECT 1 FROM consol.consolidation_runs r
        JOIN consol.consolidation_sets s ON s.id = r.consolidation_set_id
        WHERE r.id = consolidation_run_id
        AND s.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY consol_acc_map_entity_isolation ON consol.account_mappings
    USING (EXISTS (
        SELECT 1 FROM consol.consolidation_sets s
        WHERE s.id = consolidation_set_id
        AND s.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

-- Add consolidation to journal entry source enum
ALTER TYPE gl.journal_entry_source ADD VALUE IF NOT EXISTS 'consolidation';
