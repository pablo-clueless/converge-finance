-- =============================================
-- COST ACCOUNTING & BUDGETING TABLES
-- Cost Centers, Allocations, Budgets, and Variance Analysis
-- =============================================

-- Create cost schema
CREATE SCHEMA IF NOT EXISTS cost;

-- =============================================
-- ENUMS
-- =============================================

-- Cost center type
CREATE TYPE cost.center_type AS ENUM (
    'production',      -- Direct production costs
    'service',         -- Internal service departments
    'administrative',  -- Administrative overhead
    'selling',         -- Sales & marketing
    'research',        -- R&D
    'support'          -- IT, HR, etc.
);

-- Allocation method
CREATE TYPE cost.allocation_method AS ENUM (
    'direct',          -- Direct assignment
    'headcount',       -- Based on employee count
    'square_footage',  -- Based on space used
    'revenue',         -- Based on revenue
    'usage',           -- Based on usage metrics
    'activity',        -- Activity-based costing
    'fixed_percent',   -- Fixed percentage split
    'step_down',       -- Step-down allocation
    'reciprocal'       -- Reciprocal allocation
);

-- Allocation run status
CREATE TYPE cost.allocation_status AS ENUM (
    'draft',
    'in_progress',
    'completed',
    'posted',
    'reversed'
);

-- Budget status
CREATE TYPE cost.budget_status AS ENUM (
    'draft',
    'submitted',
    'approved',
    'rejected',
    'active',
    'closed'
);

-- Budget type
CREATE TYPE cost.budget_type AS ENUM (
    'operating',       -- Operating budget
    'capital',         -- Capital expenditure budget
    'cash',            -- Cash flow budget
    'project',         -- Project-specific budget
    'rolling'          -- Rolling forecast
);

-- =============================================
-- COST CENTERS
-- Organizational units for cost tracking
-- =============================================

CREATE TABLE cost.cost_centers (
    id                      CHAR(26) PRIMARY KEY,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    code                    VARCHAR(50) NOT NULL,
    name                    VARCHAR(255) NOT NULL,
    description             TEXT,
    center_type             cost.center_type NOT NULL,

    -- Hierarchy
    parent_id               CHAR(26) REFERENCES cost.cost_centers(id),
    hierarchy_level         INTEGER NOT NULL DEFAULT 0,
    hierarchy_path          TEXT,

    -- Manager responsible
    manager_id              CHAR(26),
    manager_name            VARCHAR(255),

    -- Default GL accounts for this cost center
    default_expense_account_id CHAR(26) REFERENCES gl.accounts(id),

    -- Statistics for allocation
    headcount               INTEGER DEFAULT 0,
    square_footage          DECIMAL(12,2) DEFAULT 0,

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_centers_uk UNIQUE (entity_id, code)
);

CREATE INDEX idx_cost_centers_entity ON cost.cost_centers(entity_id);
CREATE INDEX idx_cost_centers_parent ON cost.cost_centers(parent_id);
CREATE INDEX idx_cost_centers_type ON cost.cost_centers(center_type);

-- =============================================
-- ALLOCATION RULES
-- Rules for allocating costs between centers
-- =============================================

CREATE TABLE cost.allocation_rules (
    id                      CHAR(26) PRIMARY KEY,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    rule_code               VARCHAR(50) NOT NULL,
    rule_name               VARCHAR(255) NOT NULL,
    description             TEXT,

    -- Source and method
    source_cost_center_id   CHAR(26) NOT NULL REFERENCES cost.cost_centers(id),
    allocation_method       cost.allocation_method NOT NULL,

    -- Account filter (which accounts to allocate)
    account_filter          JSONB DEFAULT '{}',

    -- Sequence for step-down allocation
    sequence_number         INTEGER NOT NULL DEFAULT 0,

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_allocation_rules_uk UNIQUE (entity_id, rule_code)
);

CREATE INDEX idx_cost_alloc_rules_entity ON cost.allocation_rules(entity_id);
CREATE INDEX idx_cost_alloc_rules_source ON cost.allocation_rules(source_cost_center_id);

-- =============================================
-- ALLOCATION TARGETS
-- Target cost centers for each rule
-- =============================================

CREATE TABLE cost.allocation_targets (
    id                      CHAR(26) PRIMARY KEY,
    allocation_rule_id      CHAR(26) NOT NULL REFERENCES cost.allocation_rules(id) ON DELETE CASCADE,
    target_cost_center_id   CHAR(26) NOT NULL REFERENCES cost.cost_centers(id),

    -- Allocation basis
    fixed_percent           DECIMAL(8,4),      -- For fixed_percent method
    driver_value            DECIMAL(18,4),     -- For driver-based methods

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_alloc_targets_uk UNIQUE (allocation_rule_id, target_cost_center_id)
);

CREATE INDEX idx_cost_alloc_targets_rule ON cost.allocation_targets(allocation_rule_id);
CREATE INDEX idx_cost_alloc_targets_center ON cost.allocation_targets(target_cost_center_id);

-- =============================================
-- ALLOCATION RUNS
-- Execution of cost allocation
-- =============================================

CREATE TABLE cost.allocation_runs (
    id                      CHAR(26) PRIMARY KEY,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    run_number              VARCHAR(30) NOT NULL,
    fiscal_period_id        CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),
    allocation_date         DATE NOT NULL,

    -- Summary
    rules_executed          INTEGER NOT NULL DEFAULT 0,
    total_allocated         DECIMAL(18,4) NOT NULL DEFAULT 0,

    status                  cost.allocation_status NOT NULL DEFAULT 'draft',

    -- Journal entry for posting
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

    CONSTRAINT cost_allocation_runs_uk UNIQUE (entity_id, run_number)
);

CREATE INDEX idx_cost_alloc_runs_entity ON cost.allocation_runs(entity_id);
CREATE INDEX idx_cost_alloc_runs_period ON cost.allocation_runs(fiscal_period_id);
CREATE INDEX idx_cost_alloc_runs_status ON cost.allocation_runs(status);

-- =============================================
-- ALLOCATION ENTRIES
-- Individual allocation journal lines
-- =============================================

CREATE TABLE cost.allocation_entries (
    id                      CHAR(26) PRIMARY KEY,
    allocation_run_id       CHAR(26) NOT NULL REFERENCES cost.allocation_runs(id) ON DELETE CASCADE,
    allocation_rule_id      CHAR(26) REFERENCES cost.allocation_rules(id),
    line_number             INTEGER NOT NULL,

    -- Source
    source_cost_center_id   CHAR(26) NOT NULL REFERENCES cost.cost_centers(id),
    source_account_id       CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    -- Target
    target_cost_center_id   CHAR(26) NOT NULL REFERENCES cost.cost_centers(id),
    target_account_id       CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    -- Amounts
    allocation_percent      DECIMAL(8,4) NOT NULL,
    allocated_amount        DECIMAL(18,4) NOT NULL,

    description             TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_alloc_entries_uk UNIQUE (allocation_run_id, line_number)
);

CREATE INDEX idx_cost_alloc_entries_run ON cost.allocation_entries(allocation_run_id);
CREATE INDEX idx_cost_alloc_entries_source ON cost.allocation_entries(source_cost_center_id);
CREATE INDEX idx_cost_alloc_entries_target ON cost.allocation_entries(target_cost_center_id);

-- =============================================
-- BUDGETS
-- Budget headers
-- =============================================

CREATE TABLE cost.budgets (
    id                      CHAR(26) PRIMARY KEY,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    budget_code             VARCHAR(50) NOT NULL,
    budget_name             VARCHAR(255) NOT NULL,
    description             TEXT,

    budget_type             cost.budget_type NOT NULL,
    fiscal_year_id          CHAR(26) NOT NULL REFERENCES gl.fiscal_years(id),

    -- Version control
    version_number          INTEGER NOT NULL DEFAULT 1,
    is_current_version      BOOLEAN NOT NULL DEFAULT TRUE,
    parent_version_id       CHAR(26) REFERENCES cost.budgets(id),

    -- Currency
    currency_code           CHAR(3) NOT NULL REFERENCES currencies(code),

    -- Totals
    total_revenue           DECIMAL(18,4) NOT NULL DEFAULT 0,
    total_expenses          DECIMAL(18,4) NOT NULL DEFAULT 0,
    net_budget              DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Status
    status                  cost.budget_status NOT NULL DEFAULT 'draft',

    -- Approval tracking
    submitted_by            CHAR(26),
    submitted_at            TIMESTAMPTZ,
    approved_by             CHAR(26),
    approved_at             TIMESTAMPTZ,
    rejected_by             CHAR(26),
    rejected_at             TIMESTAMPTZ,
    rejection_reason        TEXT,

    -- Audit
    created_by              CHAR(26) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_budgets_uk UNIQUE (entity_id, budget_code, version_number)
);

CREATE INDEX idx_cost_budgets_entity ON cost.budgets(entity_id);
CREATE INDEX idx_cost_budgets_year ON cost.budgets(fiscal_year_id);
CREATE INDEX idx_cost_budgets_status ON cost.budgets(status);
CREATE INDEX idx_cost_budgets_type ON cost.budgets(budget_type);

-- =============================================
-- BUDGET LINES
-- Detailed budget by account and period
-- =============================================

CREATE TABLE cost.budget_lines (
    id                      CHAR(26) PRIMARY KEY,
    budget_id               CHAR(26) NOT NULL REFERENCES cost.budgets(id) ON DELETE CASCADE,
    account_id              CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    cost_center_id          CHAR(26) REFERENCES cost.cost_centers(id),
    fiscal_period_id        CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),

    -- Budget amounts
    budget_amount           DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Optional breakdown
    quantity                DECIMAL(18,4),
    unit_cost               DECIMAL(18,4),

    -- Notes
    notes                   TEXT,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_budget_lines_uk
        UNIQUE (budget_id, account_id, cost_center_id, fiscal_period_id)
);

CREATE INDEX idx_cost_budget_lines_budget ON cost.budget_lines(budget_id);
CREATE INDEX idx_cost_budget_lines_account ON cost.budget_lines(account_id);
CREATE INDEX idx_cost_budget_lines_center ON cost.budget_lines(cost_center_id);
CREATE INDEX idx_cost_budget_lines_period ON cost.budget_lines(fiscal_period_id);

-- =============================================
-- BUDGET TRANSFERS
-- Track budget reallocations
-- =============================================

CREATE TABLE cost.budget_transfers (
    id                      CHAR(26) PRIMARY KEY,
    budget_id               CHAR(26) NOT NULL REFERENCES cost.budgets(id),
    transfer_number         VARCHAR(30) NOT NULL,
    transfer_date           DATE NOT NULL,

    -- From
    from_account_id         CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    from_cost_center_id     CHAR(26) REFERENCES cost.cost_centers(id),
    from_period_id          CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),

    -- To
    to_account_id           CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    to_cost_center_id       CHAR(26) REFERENCES cost.cost_centers(id),
    to_period_id            CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),

    -- Amount
    transfer_amount         DECIMAL(18,4) NOT NULL CHECK (transfer_amount > 0),

    -- Justification
    reason                  TEXT NOT NULL,

    -- Approval
    requested_by            CHAR(26) NOT NULL,
    approved_by             CHAR(26),
    approved_at             TIMESTAMPTZ,
    is_approved             BOOLEAN NOT NULL DEFAULT FALSE,

    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_budget_transfers_uk UNIQUE (budget_id, transfer_number)
);

CREATE INDEX idx_cost_budget_transfers_budget ON cost.budget_transfers(budget_id);

-- =============================================
-- BUDGET ACTUALS (for variance analysis)
-- Captures actual amounts for comparison
-- =============================================

CREATE TABLE cost.budget_actuals (
    id                      CHAR(26) PRIMARY KEY,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    account_id              CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    cost_center_id          CHAR(26) REFERENCES cost.cost_centers(id),
    fiscal_period_id        CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),

    -- Actual amounts (pulled from GL)
    actual_amount           DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Snapshot timestamp
    snapshot_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cost_budget_actuals_uk
        UNIQUE (entity_id, account_id, cost_center_id, fiscal_period_id)
);

CREATE INDEX idx_cost_actuals_entity ON cost.budget_actuals(entity_id);
CREATE INDEX idx_cost_actuals_account ON cost.budget_actuals(account_id);
CREATE INDEX idx_cost_actuals_center ON cost.budget_actuals(cost_center_id);
CREATE INDEX idx_cost_actuals_period ON cost.budget_actuals(fiscal_period_id);

-- =============================================
-- FUNCTIONS
-- =============================================

-- Function to generate allocation run number
CREATE OR REPLACE FUNCTION cost.generate_allocation_run_number(p_entity_id CHAR(26))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_count INTEGER;
    v_year VARCHAR(4);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COUNT(*) + 1 INTO v_count
    FROM cost.allocation_runs
    WHERE entity_id = p_entity_id
    AND run_number LIKE 'ALLOC-' || v_year || '-%';

    RETURN 'ALLOC-' || v_year || '-' || LPAD(v_count::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

-- Function to generate budget transfer number
CREATE OR REPLACE FUNCTION cost.generate_transfer_number(p_budget_id CHAR(26))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_count INTEGER;
BEGIN
    SELECT COUNT(*) + 1 INTO v_count
    FROM cost.budget_transfers
    WHERE budget_id = p_budget_id;

    RETURN 'TRF-' || LPAD(v_count::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

-- Function to update cost center hierarchy
CREATE OR REPLACE FUNCTION cost.update_cost_center_hierarchy()
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
        FROM cost.cost_centers
        WHERE id = NEW.parent_id;

        NEW.hierarchy_level := COALESCE(parent_level, 0) + 1;
        NEW.hierarchy_path := COALESCE(parent_path, NEW.parent_id) || '/' || NEW.id;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_cost_centers_hierarchy
    BEFORE INSERT OR UPDATE OF parent_id ON cost.cost_centers
    FOR EACH ROW
    EXECUTE FUNCTION cost.update_cost_center_hierarchy();

-- Function to recalculate budget totals
CREATE OR REPLACE FUNCTION cost.recalculate_budget_totals()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE cost.budgets b
    SET
        total_revenue = COALESCE((
            SELECT SUM(bl.budget_amount)
            FROM cost.budget_lines bl
            JOIN gl.accounts a ON a.id = bl.account_id
            WHERE bl.budget_id = b.id
            AND a.account_type = 'revenue'
        ), 0),
        total_expenses = COALESCE((
            SELECT SUM(bl.budget_amount)
            FROM cost.budget_lines bl
            JOIN gl.accounts a ON a.id = bl.account_id
            WHERE bl.budget_id = b.id
            AND a.account_type = 'expense'
        ), 0),
        updated_at = NOW()
    WHERE b.id = COALESCE(NEW.budget_id, OLD.budget_id);

    UPDATE cost.budgets
    SET net_budget = total_revenue - total_expenses
    WHERE id = COALESCE(NEW.budget_id, OLD.budget_id);

    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_budget_lines_totals
    AFTER INSERT OR UPDATE OR DELETE ON cost.budget_lines
    FOR EACH ROW
    EXECUTE FUNCTION cost.recalculate_budget_totals();

-- =============================================
-- TRIGGERS
-- =============================================

CREATE TRIGGER trg_cost_centers_updated_at
    BEFORE UPDATE ON cost.cost_centers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_cost_alloc_rules_updated_at
    BEFORE UPDATE ON cost.allocation_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_cost_alloc_runs_updated_at
    BEFORE UPDATE ON cost.allocation_runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_cost_budgets_updated_at
    BEFORE UPDATE ON cost.budgets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_cost_budget_lines_updated_at
    BEFORE UPDATE ON cost.budget_lines
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- ROW LEVEL SECURITY
-- =============================================

ALTER TABLE cost.cost_centers ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.allocation_rules ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.allocation_targets ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.allocation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.allocation_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.budgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.budget_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.budget_transfers ENABLE ROW LEVEL SECURITY;
ALTER TABLE cost.budget_actuals ENABLE ROW LEVEL SECURITY;

-- RLS Policies
CREATE POLICY cost_centers_entity_isolation ON cost.cost_centers
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY cost_alloc_rules_entity_isolation ON cost.allocation_rules
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY cost_alloc_targets_entity_isolation ON cost.allocation_targets
    USING (EXISTS (
        SELECT 1 FROM cost.allocation_rules r
        WHERE r.id = allocation_rule_id
        AND r.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY cost_alloc_runs_entity_isolation ON cost.allocation_runs
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY cost_alloc_entries_entity_isolation ON cost.allocation_entries
    USING (EXISTS (
        SELECT 1 FROM cost.allocation_runs r
        WHERE r.id = allocation_run_id
        AND r.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY cost_budgets_entity_isolation ON cost.budgets
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY cost_budget_lines_entity_isolation ON cost.budget_lines
    USING (EXISTS (
        SELECT 1 FROM cost.budgets b
        WHERE b.id = budget_id
        AND b.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY cost_budget_transfers_entity_isolation ON cost.budget_transfers
    USING (EXISTS (
        SELECT 1 FROM cost.budgets b
        WHERE b.id = budget_id
        AND b.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY cost_actuals_entity_isolation ON cost.budget_actuals
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Add cost allocation to journal entry source enum
ALTER TYPE gl.journal_entry_source ADD VALUE IF NOT EXISTS 'cost_allocation';
