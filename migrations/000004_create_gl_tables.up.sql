-- =============================================
-- GENERAL LEDGER TABLES
-- =============================================

-- Account Types
CREATE TYPE gl.account_type AS ENUM (
    'asset', 'liability', 'equity', 'revenue', 'expense'
);

CREATE TYPE gl.account_subtype AS ENUM (
    'current_asset', 'fixed_asset', 'other_asset',
    'current_liability', 'long_term_liability', 'other_liability',
    'retained_earnings', 'other_equity',
    'operating_revenue', 'other_revenue',
    'operating_expense', 'other_expense'
);

-- Chart of Accounts
CREATE TABLE gl.accounts (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    parent_id       CHAR(26) REFERENCES gl.accounts(id),
    account_code    VARCHAR(50) NOT NULL,
    account_name    VARCHAR(255) NOT NULL,
    account_type    gl.account_type NOT NULL,
    account_subtype gl.account_subtype,
    currency_code   CHAR(3) NOT NULL REFERENCES currencies(code),
    is_control      BOOLEAN NOT NULL DEFAULT FALSE,
    is_posting      BOOLEAN NOT NULL DEFAULT TRUE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    description     TEXT,
    normal_balance  VARCHAR(10) NOT NULL DEFAULT 'debit' CHECK (normal_balance IN ('debit', 'credit')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT gl_accounts_uk_entity_code UNIQUE (entity_id, account_code)
);

CREATE INDEX idx_gl_accounts_entity ON gl.accounts(entity_id);
CREATE INDEX idx_gl_accounts_parent ON gl.accounts(parent_id);
CREATE INDEX idx_gl_accounts_type ON gl.accounts(account_type);
CREATE INDEX idx_gl_accounts_active ON gl.accounts(entity_id, is_active) WHERE is_active = TRUE;

-- Fiscal Year Status
CREATE TYPE gl.fiscal_year_status AS ENUM ('open', 'closing', 'closed');

-- Fiscal Years
CREATE TABLE gl.fiscal_years (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    year_code       VARCHAR(10) NOT NULL,
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    status          gl.fiscal_year_status NOT NULL DEFAULT 'open',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT gl_fiscal_years_uk UNIQUE (entity_id, year_code),
    CONSTRAINT gl_fiscal_years_dates CHECK (end_date > start_date)
);

CREATE INDEX idx_gl_fiscal_years_entity ON gl.fiscal_years(entity_id);

-- Fiscal Period Status
CREATE TYPE gl.fiscal_period_status AS ENUM ('future', 'open', 'closing', 'closed');

-- Fiscal Periods
CREATE TABLE gl.fiscal_periods (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    fiscal_year_id  CHAR(26) NOT NULL REFERENCES gl.fiscal_years(id),
    period_number   INTEGER NOT NULL CHECK (period_number BETWEEN 1 AND 13),
    period_name     VARCHAR(50) NOT NULL,
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    status          gl.fiscal_period_status NOT NULL DEFAULT 'future',
    is_adjustment   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT gl_fiscal_periods_uk UNIQUE (entity_id, fiscal_year_id, period_number),
    CONSTRAINT gl_fiscal_periods_dates CHECK (end_date >= start_date)
);

CREATE INDEX idx_gl_fiscal_periods_entity ON gl.fiscal_periods(entity_id);
CREATE INDEX idx_gl_fiscal_periods_year ON gl.fiscal_periods(fiscal_year_id);
CREATE INDEX idx_gl_fiscal_periods_dates ON gl.fiscal_periods(entity_id, start_date, end_date);

-- Journal Entry Status
CREATE TYPE gl.journal_entry_status AS ENUM ('draft', 'pending', 'posted', 'reversed');

-- Journal Entry Source
CREATE TYPE gl.journal_entry_source AS ENUM ('manual', 'ap', 'ar', 'fa', 'recurring', 'closing', 'system');

-- Journal Entries (Header)
CREATE TABLE gl.journal_entries (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    entry_number    VARCHAR(30) NOT NULL,
    fiscal_period_id CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),
    entry_date      DATE NOT NULL,
    posting_date    DATE,
    description     TEXT NOT NULL,
    source          gl.journal_entry_source NOT NULL DEFAULT 'manual',
    source_reference VARCHAR(100),
    status          gl.journal_entry_status NOT NULL DEFAULT 'draft',
    currency_code   CHAR(3) NOT NULL REFERENCES currencies(code),
    exchange_rate   DECIMAL(18,8) NOT NULL DEFAULT 1 CHECK (exchange_rate > 0),
    is_reversing    BOOLEAN NOT NULL DEFAULT FALSE,
    reversal_of_id  CHAR(26) REFERENCES gl.journal_entries(id),
    reversed_by_id  CHAR(26) REFERENCES gl.journal_entries(id),
    created_by      CHAR(26) NOT NULL,
    approved_by     CHAR(26),
    posted_by       CHAR(26),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    posted_at       TIMESTAMPTZ,

    CONSTRAINT gl_journal_entries_uk UNIQUE (entity_id, entry_number)
);

CREATE INDEX idx_gl_journal_entries_entity ON gl.journal_entries(entity_id);
CREATE INDEX idx_gl_journal_entries_period ON gl.journal_entries(fiscal_period_id);
CREATE INDEX idx_gl_journal_entries_date ON gl.journal_entries(entity_id, entry_date);
CREATE INDEX idx_gl_journal_entries_status ON gl.journal_entries(entity_id, status);
CREATE INDEX idx_gl_journal_entries_source ON gl.journal_entries(source, source_reference);

-- Journal Entry Lines
CREATE TABLE gl.journal_lines (
    id              CHAR(26) PRIMARY KEY,
    journal_entry_id CHAR(26) NOT NULL REFERENCES gl.journal_entries(id) ON DELETE CASCADE,
    line_number     INTEGER NOT NULL,
    account_id      CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    description     TEXT,
    debit_amount    DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (debit_amount >= 0),
    credit_amount   DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    base_debit      DECIMAL(18,4) NOT NULL DEFAULT 0,
    base_credit     DECIMAL(18,4) NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Either debit OR credit, never both, never zero
    CONSTRAINT gl_journal_lines_debit_or_credit
        CHECK ((debit_amount = 0 AND credit_amount > 0) OR
               (debit_amount > 0 AND credit_amount = 0)),
    CONSTRAINT gl_journal_lines_uk UNIQUE (journal_entry_id, line_number)
);

CREATE INDEX idx_gl_journal_lines_entry ON gl.journal_lines(journal_entry_id);
CREATE INDEX idx_gl_journal_lines_account ON gl.journal_lines(account_id);

-- Function to check journal entry balance before posting
CREATE OR REPLACE FUNCTION gl.check_journal_balance()
RETURNS TRIGGER AS $$
DECLARE
    total_debits DECIMAL(18,4);
    total_credits DECIMAL(18,4);
    line_count INTEGER;
BEGIN
    -- Only check when posting
    IF NEW.status = 'posted' AND (OLD.status IS NULL OR OLD.status != 'posted') THEN
        SELECT
            COALESCE(SUM(debit_amount), 0),
            COALESCE(SUM(credit_amount), 0),
            COUNT(*)
        INTO total_debits, total_credits, line_count
        FROM gl.journal_lines
        WHERE journal_entry_id = NEW.id;

        -- Check minimum lines
        IF line_count < 2 THEN
            RAISE EXCEPTION 'Journal entry must have at least 2 lines';
        END IF;

        -- Check balance
        IF total_debits != total_credits THEN
            RAISE EXCEPTION 'Journal entry is not balanced: debits=%, credits=%',
                total_debits, total_credits;
        END IF;

        -- Check non-zero
        IF total_debits = 0 THEN
            RAISE EXCEPTION 'Journal entry cannot have zero amounts';
        END IF;

        -- Set posting timestamp
        NEW.posting_date := COALESCE(NEW.posting_date, NEW.entry_date);
        NEW.posted_at := NOW();
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_gl_journal_entries_balance
    BEFORE UPDATE ON gl.journal_entries
    FOR EACH ROW
    EXECUTE FUNCTION gl.check_journal_balance();

-- Function to prevent posting to closed periods
CREATE OR REPLACE FUNCTION gl.check_period_open()
RETURNS TRIGGER AS $$
DECLARE
    period_status gl.fiscal_period_status;
BEGIN
    -- Only check when posting
    IF NEW.status = 'posted' AND (OLD.status IS NULL OR OLD.status != 'posted') THEN
        SELECT status INTO period_status
        FROM gl.fiscal_periods
        WHERE id = NEW.fiscal_period_id;

        IF period_status != 'open' THEN
            RAISE EXCEPTION 'Cannot post to a % period', period_status;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_gl_journal_entries_period
    BEFORE UPDATE ON gl.journal_entries
    FOR EACH ROW
    EXECUTE FUNCTION gl.check_period_open();

-- Account Balances (Materialized for performance)
CREATE TABLE gl.account_balances (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    account_id      CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    fiscal_period_id CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),
    opening_debit   DECIMAL(18,4) NOT NULL DEFAULT 0,
    opening_credit  DECIMAL(18,4) NOT NULL DEFAULT 0,
    period_debit    DECIMAL(18,4) NOT NULL DEFAULT 0,
    period_credit   DECIMAL(18,4) NOT NULL DEFAULT 0,
    closing_debit   DECIMAL(18,4) NOT NULL DEFAULT 0,
    closing_credit  DECIMAL(18,4) NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT gl_account_balances_uk UNIQUE (entity_id, account_id, fiscal_period_id)
);

CREATE INDEX idx_gl_account_balances_account ON gl.account_balances(account_id);
CREATE INDEX idx_gl_account_balances_period ON gl.account_balances(fiscal_period_id);

-- Recurring Journal Templates
CREATE TABLE gl.recurring_entries (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    template_name   VARCHAR(100) NOT NULL,
    description     TEXT NOT NULL,
    frequency       VARCHAR(20) NOT NULL CHECK (frequency IN ('monthly', 'quarterly', 'yearly')),
    next_run_date   DATE NOT NULL,
    end_date        DATE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    auto_post       BOOLEAN NOT NULL DEFAULT FALSE,
    created_by      CHAR(26) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_gl_recurring_entries_entity ON gl.recurring_entries(entity_id);
CREATE INDEX idx_gl_recurring_entries_next ON gl.recurring_entries(next_run_date) WHERE is_active = TRUE;

-- Recurring Entry Lines
CREATE TABLE gl.recurring_entry_lines (
    id              CHAR(26) PRIMARY KEY,
    recurring_entry_id CHAR(26) NOT NULL REFERENCES gl.recurring_entries(id) ON DELETE CASCADE,
    line_number     INTEGER NOT NULL,
    account_id      CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    description     TEXT,
    debit_amount    DECIMAL(18,4) NOT NULL DEFAULT 0,
    credit_amount   DECIMAL(18,4) NOT NULL DEFAULT 0,

    CONSTRAINT gl_recurring_entry_lines_uk UNIQUE (recurring_entry_id, line_number)
);

-- Updated_at triggers for GL tables
CREATE TRIGGER trg_gl_accounts_updated_at
    BEFORE UPDATE ON gl.accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_gl_fiscal_years_updated_at
    BEFORE UPDATE ON gl.fiscal_years
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_gl_fiscal_periods_updated_at
    BEFORE UPDATE ON gl.fiscal_periods
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_gl_journal_entries_updated_at
    BEFORE UPDATE ON gl.journal_entries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_gl_recurring_entries_updated_at
    BEFORE UPDATE ON gl.recurring_entries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Row Level Security for GL tables
ALTER TABLE gl.accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl.fiscal_years ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl.fiscal_periods ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl.journal_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl.journal_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl.account_balances ENABLE ROW LEVEL SECURITY;
ALTER TABLE gl.recurring_entries ENABLE ROW LEVEL SECURITY;

-- RLS Policies (using app.current_entity_id session variable)
CREATE POLICY gl_accounts_entity_isolation ON gl.accounts
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY gl_fiscal_years_entity_isolation ON gl.fiscal_years
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY gl_fiscal_periods_entity_isolation ON gl.fiscal_periods
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY gl_journal_entries_entity_isolation ON gl.journal_entries
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY gl_account_balances_entity_isolation ON gl.account_balances
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY gl_recurring_entries_entity_isolation ON gl.recurring_entries
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Journal lines inherit from journal entries
CREATE POLICY gl_journal_lines_entity_isolation ON gl.journal_lines
    USING (EXISTS (
        SELECT 1 FROM gl.journal_entries je
        WHERE je.id = journal_entry_id
        AND je.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));
