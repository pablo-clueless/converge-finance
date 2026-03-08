-- =============================================
-- INTERCOMPANY (IC) TABLES
-- Multi-Entity Hierarchy and Intercompany Transactions
-- =============================================

-- Create IC schema
CREATE SCHEMA IF NOT EXISTS ic;

-- =============================================
-- EXTEND ENTITIES TABLE FOR HIERARCHY
-- =============================================

-- Entity type for hierarchy classification
DO $$ BEGIN
    CREATE TYPE entity_type AS ENUM ('operating', 'holding', 'elimination');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Consolidation method
DO $$ BEGIN
    CREATE TYPE consolidation_method AS ENUM ('full', 'proportional', 'equity', 'none');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Add hierarchy columns to entities table
ALTER TABLE entities ADD COLUMN IF NOT EXISTS parent_id CHAR(26) REFERENCES entities(id);
ALTER TABLE entities ADD COLUMN IF NOT EXISTS entity_type entity_type DEFAULT 'operating';
ALTER TABLE entities ADD COLUMN IF NOT EXISTS ownership_percent DECIMAL(5,2) DEFAULT 100.00
    CHECK (ownership_percent >= 0 AND ownership_percent <= 100);
ALTER TABLE entities ADD COLUMN IF NOT EXISTS consolidation_method consolidation_method DEFAULT 'full';
ALTER TABLE entities ADD COLUMN IF NOT EXISTS hierarchy_level INTEGER DEFAULT 0;
ALTER TABLE entities ADD COLUMN IF NOT EXISTS hierarchy_path TEXT;

-- Create indexes for hierarchy queries
CREATE INDEX IF NOT EXISTS idx_entities_parent ON entities(parent_id);
CREATE INDEX IF NOT EXISTS idx_entities_hierarchy_path ON entities(hierarchy_path);
CREATE INDEX IF NOT EXISTS idx_entities_hierarchy_level ON entities(hierarchy_level);

-- Add IC and elimination to journal entry source enum
ALTER TYPE gl.journal_entry_source ADD VALUE IF NOT EXISTS 'ic';
ALTER TYPE gl.journal_entry_source ADD VALUE IF NOT EXISTS 'elimination';

-- =============================================
-- IC TRANSACTION TYPES AND STATUS
-- =============================================

CREATE TYPE ic.transaction_type AS ENUM (
    'sale',           -- Intercompany sale of goods
    'service',        -- Intercompany services
    'loan',           -- Intercompany loan
    'allocation',     -- Cost allocation
    'dividend',       -- Dividend distribution
    'capital',        -- Capital contribution
    'recharge',       -- Expense recharge
    'transfer'        -- Asset/inventory transfer
);

CREATE TYPE ic.transaction_status AS ENUM (
    'draft',          -- Initial state
    'pending',        -- Awaiting counterparty confirmation
    'posted',         -- Journal entries created in both entities
    'reconciled',     -- Matched and reconciled
    'disputed'        -- Discrepancy identified
);

CREATE TYPE ic.elimination_type AS ENUM (
    'ic_receivable_payable',  -- Due To/Due From elimination
    'ic_revenue_expense',     -- Revenue/Expense elimination
    'ic_dividend',            -- Intercompany dividend elimination
    'ic_investment',          -- Investment in subsidiary elimination
    'ic_equity',              -- Equity elimination
    'unrealized_profit'       -- Unrealized profit in inventory/assets
);

CREATE TYPE ic.elimination_status AS ENUM (
    'draft',
    'posted',
    'reversed'
);

-- =============================================
-- ACCOUNT MAPPINGS
-- IC account pairs per entity relationship
-- =============================================

CREATE TABLE ic.account_mappings (
    id                      CHAR(26) PRIMARY KEY,
    from_entity_id          CHAR(26) NOT NULL REFERENCES entities(id),
    to_entity_id            CHAR(26) NOT NULL REFERENCES entities(id),
    transaction_type        ic.transaction_type NOT NULL,

    -- FROM entity accounts (the initiating entity)
    from_due_to_account_id      CHAR(26) REFERENCES gl.accounts(id),    -- Liability: Due to TO_entity
    from_due_from_account_id    CHAR(26) REFERENCES gl.accounts(id),    -- Asset: Due from TO_entity
    from_revenue_account_id     CHAR(26) REFERENCES gl.accounts(id),    -- IC Revenue account
    from_expense_account_id     CHAR(26) REFERENCES gl.accounts(id),    -- IC Expense account

    -- TO entity accounts (the counterparty)
    to_due_to_account_id        CHAR(26) REFERENCES gl.accounts(id),    -- Liability: Due to FROM_entity
    to_due_from_account_id      CHAR(26) REFERENCES gl.accounts(id),    -- Asset: Due from FROM_entity
    to_revenue_account_id       CHAR(26) REFERENCES gl.accounts(id),    -- IC Revenue account
    to_expense_account_id       CHAR(26) REFERENCES gl.accounts(id),    -- IC Expense account

    description             TEXT,
    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ic_account_mappings_different_entities
        CHECK (from_entity_id != to_entity_id),
    CONSTRAINT ic_account_mappings_uk
        UNIQUE (from_entity_id, to_entity_id, transaction_type)
);

CREATE INDEX idx_ic_account_mappings_from ON ic.account_mappings(from_entity_id);
CREATE INDEX idx_ic_account_mappings_to ON ic.account_mappings(to_entity_id);
CREATE INDEX idx_ic_account_mappings_type ON ic.account_mappings(transaction_type);

-- =============================================
-- IC TRANSACTIONS
-- Intercompany transaction headers
-- =============================================

CREATE TABLE ic.transactions (
    id                      CHAR(26) PRIMARY KEY,
    transaction_number      VARCHAR(30) NOT NULL,
    transaction_type        ic.transaction_type NOT NULL,

    -- Entity pair
    from_entity_id          CHAR(26) NOT NULL REFERENCES entities(id),
    to_entity_id            CHAR(26) NOT NULL REFERENCES entities(id),

    -- Dates
    transaction_date        DATE NOT NULL,
    due_date                DATE,

    -- Amounts
    amount                  DECIMAL(18,4) NOT NULL CHECK (amount > 0),
    currency_code           CHAR(3) NOT NULL REFERENCES currencies(code),
    exchange_rate           DECIMAL(18,8) NOT NULL DEFAULT 1 CHECK (exchange_rate > 0),
    base_amount             DECIMAL(18,4) NOT NULL,

    -- Description
    description             TEXT NOT NULL,
    reference               VARCHAR(100),

    -- Status tracking
    status                  ic.transaction_status NOT NULL DEFAULT 'draft',

    -- Fiscal period references
    from_fiscal_period_id   CHAR(26) REFERENCES gl.fiscal_periods(id),
    to_fiscal_period_id     CHAR(26) REFERENCES gl.fiscal_periods(id),

    -- Journal entry references (one in each entity)
    from_journal_entry_id   CHAR(26) REFERENCES gl.journal_entries(id),
    to_journal_entry_id     CHAR(26) REFERENCES gl.journal_entries(id),

    -- Audit fields
    created_by              CHAR(26) NOT NULL,
    posted_by               CHAR(26),
    reconciled_by           CHAR(26),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    posted_at               TIMESTAMPTZ,
    reconciled_at           TIMESTAMPTZ,

    CONSTRAINT ic_transactions_different_entities
        CHECK (from_entity_id != to_entity_id)
);

CREATE UNIQUE INDEX idx_ic_transactions_number ON ic.transactions(from_entity_id, transaction_number);
CREATE INDEX idx_ic_transactions_from ON ic.transactions(from_entity_id);
CREATE INDEX idx_ic_transactions_to ON ic.transactions(to_entity_id);
CREATE INDEX idx_ic_transactions_date ON ic.transactions(transaction_date);
CREATE INDEX idx_ic_transactions_status ON ic.transactions(status);
CREATE INDEX idx_ic_transactions_type ON ic.transactions(transaction_type);
CREATE INDEX idx_ic_transactions_entity_pair ON ic.transactions(from_entity_id, to_entity_id);

-- =============================================
-- IC TRANSACTION LINES
-- Line items for IC transactions
-- =============================================

CREATE TABLE ic.transaction_lines (
    id                      CHAR(26) PRIMARY KEY,
    transaction_id          CHAR(26) NOT NULL REFERENCES ic.transactions(id) ON DELETE CASCADE,
    line_number             INTEGER NOT NULL,

    description             TEXT,
    quantity                DECIMAL(18,4) DEFAULT 1,
    unit_price              DECIMAL(18,4),
    amount                  DECIMAL(18,4) NOT NULL CHECK (amount > 0),

    -- Optional cost center / project reference
    cost_center_code        VARCHAR(50),
    project_code            VARCHAR(50),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ic_transaction_lines_uk UNIQUE (transaction_id, line_number)
);

CREATE INDEX idx_ic_transaction_lines_tx ON ic.transaction_lines(transaction_id);

-- =============================================
-- ENTITY PAIR BALANCES
-- Materialized balances for reconciliation
-- =============================================

CREATE TABLE ic.entity_pair_balances (
    id                      CHAR(26) PRIMARY KEY,
    from_entity_id          CHAR(26) NOT NULL REFERENCES entities(id),
    to_entity_id            CHAR(26) NOT NULL REFERENCES entities(id),
    fiscal_period_id        CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),
    currency_code           CHAR(3) NOT NULL REFERENCES currencies(code),

    -- Balances from FROM entity's perspective
    -- Positive = FROM owes TO (Due To), Negative = TO owes FROM (Due From)
    opening_balance         DECIMAL(18,4) NOT NULL DEFAULT 0,
    period_debits           DECIMAL(18,4) NOT NULL DEFAULT 0,
    period_credits          DECIMAL(18,4) NOT NULL DEFAULT 0,
    closing_balance         DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Reconciliation status
    is_reconciled           BOOLEAN NOT NULL DEFAULT FALSE,
    discrepancy_amount      DECIMAL(18,4) DEFAULT 0,

    last_reconciled_at      TIMESTAMPTZ,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ic_entity_pair_balances_different
        CHECK (from_entity_id != to_entity_id),
    CONSTRAINT ic_entity_pair_balances_uk
        UNIQUE (from_entity_id, to_entity_id, fiscal_period_id, currency_code)
);

CREATE INDEX idx_ic_pair_balances_from ON ic.entity_pair_balances(from_entity_id);
CREATE INDEX idx_ic_pair_balances_to ON ic.entity_pair_balances(to_entity_id);
CREATE INDEX idx_ic_pair_balances_period ON ic.entity_pair_balances(fiscal_period_id);

-- =============================================
-- ELIMINATION RULES
-- Rules for generating elimination entries
-- =============================================

CREATE TABLE ic.elimination_rules (
    id                      CHAR(26) PRIMARY KEY,
    parent_entity_id        CHAR(26) NOT NULL REFERENCES entities(id),
    rule_code               VARCHAR(50) NOT NULL,
    rule_name               VARCHAR(255) NOT NULL,
    elimination_type        ic.elimination_type NOT NULL,

    description             TEXT,

    -- Rule configuration (JSON for flexibility)
    -- Can include: account patterns, entity filters, percentage, etc.
    rule_config             JSONB NOT NULL DEFAULT '{}',

    -- Execution order
    sequence_number         INTEGER NOT NULL DEFAULT 0,

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ic_elimination_rules_uk UNIQUE (parent_entity_id, rule_code)
);

CREATE INDEX idx_ic_elimination_rules_parent ON ic.elimination_rules(parent_entity_id);
CREATE INDEX idx_ic_elimination_rules_type ON ic.elimination_rules(elimination_type);

-- =============================================
-- ELIMINATION RUNS
-- Elimination run headers
-- =============================================

CREATE TABLE ic.elimination_runs (
    id                      CHAR(26) PRIMARY KEY,
    run_number              VARCHAR(30) NOT NULL,
    parent_entity_id        CHAR(26) NOT NULL REFERENCES entities(id),
    fiscal_period_id        CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),
    elimination_date        DATE NOT NULL,

    currency_code           CHAR(3) NOT NULL REFERENCES currencies(code),

    -- Summary
    entry_count             INTEGER NOT NULL DEFAULT 0,
    total_eliminations      DECIMAL(18,4) NOT NULL DEFAULT 0,

    status                  ic.elimination_status NOT NULL DEFAULT 'draft',

    -- Journal entry in parent (elimination) entity
    journal_entry_id        CHAR(26) REFERENCES gl.journal_entries(id),

    -- Audit fields
    created_by              CHAR(26) NOT NULL,
    posted_by               CHAR(26),
    reversed_by             CHAR(26),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    posted_at               TIMESTAMPTZ,
    reversed_at             TIMESTAMPTZ,

    CONSTRAINT ic_elimination_runs_uk UNIQUE (parent_entity_id, run_number)
);

CREATE INDEX idx_ic_elimination_runs_parent ON ic.elimination_runs(parent_entity_id);
CREATE INDEX idx_ic_elimination_runs_period ON ic.elimination_runs(fiscal_period_id);
CREATE INDEX idx_ic_elimination_runs_status ON ic.elimination_runs(status);

-- =============================================
-- ELIMINATION ENTRIES
-- Individual elimination journal lines
-- =============================================

CREATE TABLE ic.elimination_entries (
    id                      CHAR(26) PRIMARY KEY,
    elimination_run_id      CHAR(26) NOT NULL REFERENCES ic.elimination_runs(id) ON DELETE CASCADE,
    elimination_rule_id     CHAR(26) REFERENCES ic.elimination_rules(id),

    line_number             INTEGER NOT NULL,
    elimination_type        ic.elimination_type NOT NULL,

    -- Source entities being eliminated
    from_entity_id          CHAR(26) REFERENCES entities(id),
    to_entity_id            CHAR(26) REFERENCES entities(id),

    -- Account and amounts
    account_id              CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    description             TEXT,
    debit_amount            DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),
    credit_amount           DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ic_elimination_entries_debit_or_credit
        CHECK ((debit_amount = 0 AND credit_amount > 0) OR
               (debit_amount > 0 AND credit_amount = 0)),
    CONSTRAINT ic_elimination_entries_uk UNIQUE (elimination_run_id, line_number)
);

CREATE INDEX idx_ic_elimination_entries_run ON ic.elimination_entries(elimination_run_id);
CREATE INDEX idx_ic_elimination_entries_rule ON ic.elimination_entries(elimination_rule_id);
CREATE INDEX idx_ic_elimination_entries_account ON ic.elimination_entries(account_id);

-- =============================================
-- FUNCTIONS AND TRIGGERS
-- =============================================

-- Function to calculate and update hierarchy path and level
CREATE OR REPLACE FUNCTION update_entity_hierarchy()
RETURNS TRIGGER AS $$
DECLARE
    parent_path TEXT;
    parent_level INTEGER;
BEGIN
    IF NEW.parent_id IS NULL THEN
        NEW.hierarchy_level := 0;
        NEW.hierarchy_path := NEW.id;
    ELSE
        SELECT hierarchy_path, hierarchy_level
        INTO parent_path, parent_level
        FROM entities
        WHERE id = NEW.parent_id;

        NEW.hierarchy_level := COALESCE(parent_level, 0) + 1;
        NEW.hierarchy_path := COALESCE(parent_path, NEW.parent_id) || '/' || NEW.id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_entities_hierarchy
    BEFORE INSERT OR UPDATE OF parent_id ON entities
    FOR EACH ROW
    EXECUTE FUNCTION update_entity_hierarchy();

-- Function to prevent circular hierarchy
CREATE OR REPLACE FUNCTION check_entity_circular_reference()
RETURNS TRIGGER AS $$
DECLARE
    current_parent CHAR(26);
BEGIN
    IF NEW.parent_id IS NOT NULL THEN
        current_parent := NEW.parent_id;
        WHILE current_parent IS NOT NULL LOOP
            IF current_parent = NEW.id THEN
                RAISE EXCEPTION 'Circular reference detected in entity hierarchy';
            END IF;
            SELECT parent_id INTO current_parent FROM entities WHERE id = current_parent;
        END LOOP;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_entities_no_circular
    BEFORE INSERT OR UPDATE OF parent_id ON entities
    FOR EACH ROW
    EXECUTE FUNCTION check_entity_circular_reference();

-- Function to generate IC transaction number
CREATE OR REPLACE FUNCTION ic.generate_transaction_number(p_entity_id CHAR(26))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_count INTEGER;
    v_year VARCHAR(4);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COUNT(*) + 1 INTO v_count
    FROM ic.transactions
    WHERE from_entity_id = p_entity_id
    AND transaction_number LIKE 'IC-' || v_year || '-%';

    RETURN 'IC-' || v_year || '-' || LPAD(v_count::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

-- Function to generate elimination run number
CREATE OR REPLACE FUNCTION ic.generate_elimination_run_number(p_parent_id CHAR(26))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_count INTEGER;
    v_year VARCHAR(4);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COUNT(*) + 1 INTO v_count
    FROM ic.elimination_runs
    WHERE parent_entity_id = p_parent_id
    AND run_number LIKE 'ELIM-' || v_year || '-%';

    RETURN 'ELIM-' || v_year || '-' || LPAD(v_count::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

-- Updated_at triggers for IC tables
CREATE TRIGGER trg_ic_account_mappings_updated_at
    BEFORE UPDATE ON ic.account_mappings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ic_transactions_updated_at
    BEFORE UPDATE ON ic.transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ic_entity_pair_balances_updated_at
    BEFORE UPDATE ON ic.entity_pair_balances
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ic_elimination_rules_updated_at
    BEFORE UPDATE ON ic.elimination_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ic_elimination_runs_updated_at
    BEFORE UPDATE ON ic.elimination_runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- ROW LEVEL SECURITY
-- =============================================

ALTER TABLE ic.account_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE ic.transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE ic.transaction_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE ic.entity_pair_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE ic.elimination_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE ic.elimination_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE ic.elimination_entries ENABLE ROW LEVEL SECURITY;

-- IC tables need access if current entity is EITHER from or to entity
CREATE POLICY ic_account_mappings_entity_isolation ON ic.account_mappings
    USING (
        from_entity_id = current_setting('app.current_entity_id', true)::CHAR(26) OR
        to_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    );

CREATE POLICY ic_transactions_entity_isolation ON ic.transactions
    USING (
        from_entity_id = current_setting('app.current_entity_id', true)::CHAR(26) OR
        to_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    );

CREATE POLICY ic_transaction_lines_entity_isolation ON ic.transaction_lines
    USING (EXISTS (
        SELECT 1 FROM ic.transactions t
        WHERE t.id = transaction_id
        AND (t.from_entity_id = current_setting('app.current_entity_id', true)::CHAR(26) OR
             t.to_entity_id = current_setting('app.current_entity_id', true)::CHAR(26))
    ));

CREATE POLICY ic_entity_pair_balances_entity_isolation ON ic.entity_pair_balances
    USING (
        from_entity_id = current_setting('app.current_entity_id', true)::CHAR(26) OR
        to_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    );

CREATE POLICY ic_elimination_rules_entity_isolation ON ic.elimination_rules
    USING (parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ic_elimination_runs_entity_isolation ON ic.elimination_runs
    USING (parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ic_elimination_entries_entity_isolation ON ic.elimination_entries
    USING (EXISTS (
        SELECT 1 FROM ic.elimination_runs r
        WHERE r.id = elimination_run_id
        AND r.parent_entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));
