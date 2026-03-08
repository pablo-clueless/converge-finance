CREATE SCHEMA IF NOT EXISTS fa;

CREATE TYPE fa.asset_status AS ENUM ('draft', 'active', 'suspended', 'disposed', 'written_off');
CREATE TYPE fa.depreciation_method AS ENUM ('straight_line', 'declining_balance', 'sum_of_years_digits', 'units_of_production');
CREATE TYPE fa.disposal_type AS ENUM ('sale', 'scrapping', 'donation', 'trade_in', 'theft_loss');
CREATE TYPE fa.transfer_status AS ENUM ('pending', 'approved', 'completed', 'cancelled');
CREATE TYPE fa.depreciation_run_status AS ENUM ('draft', 'calculated', 'posted', 'reversed');

CREATE TABLE fa.asset_categories (
    id                              CHAR(26) PRIMARY KEY,
    entity_id                       CHAR(26) NOT NULL REFERENCES entities(id),
    code                            VARCHAR(20) NOT NULL,
    name                            VARCHAR(100) NOT NULL,
    description                     TEXT,
    depreciation_method             fa.depreciation_method NOT NULL DEFAULT 'straight_line',
    default_useful_life_years       INTEGER NOT NULL DEFAULT 5 CHECK (default_useful_life_years > 0),
    default_salvage_percent         DECIMAL(5,2) NOT NULL DEFAULT 0 CHECK (default_salvage_percent >= 0 AND default_salvage_percent <= 100),
    asset_account_id                CHAR(26) REFERENCES gl.accounts(id),
    accum_depreciation_account_id   CHAR(26) REFERENCES gl.accounts(id),
    depreciation_expense_account_id CHAR(26) REFERENCES gl.accounts(id),
    gain_loss_account_id            CHAR(26) REFERENCES gl.accounts(id),
    is_active                       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                      CHAR(26) NOT NULL,
    CONSTRAINT fa_asset_categories_uk UNIQUE (entity_id, code)
);

CREATE INDEX idx_fa_asset_categories_entity ON fa.asset_categories(entity_id);
CREATE INDEX idx_fa_asset_categories_active ON fa.asset_categories(entity_id, is_active) WHERE is_active = TRUE;

CREATE TABLE fa.assets (
    id                              CHAR(26) PRIMARY KEY,
    entity_id                       CHAR(26) NOT NULL REFERENCES entities(id),
    category_id                     CHAR(26) NOT NULL REFERENCES fa.asset_categories(id),
    asset_code                      VARCHAR(50) NOT NULL,
    asset_name                      VARCHAR(255) NOT NULL,
    description                     TEXT,
    serial_number                   VARCHAR(100),
    barcode                         VARCHAR(100),

    -- Acquisition
    acquisition_date                DATE NOT NULL,
    acquisition_cost                DECIMAL(18,4) NOT NULL CHECK (acquisition_cost >= 0),
    currency_code                   CHAR(3) NOT NULL REFERENCES currencies(code),
    vendor_id                       CHAR(26) REFERENCES ap.vendors(id),
    ap_invoice_id                   CHAR(26),
    po_number                       VARCHAR(100),

    -- Depreciation Config
    depreciation_method             fa.depreciation_method NOT NULL,
    useful_life_years               INTEGER NOT NULL CHECK (useful_life_years > 0),
    useful_life_units               INTEGER CHECK (useful_life_units IS NULL OR useful_life_units > 0),
    salvage_value                   DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (salvage_value >= 0),
    depreciation_start_date         DATE,

    -- Current Values (updated by depreciation runs)
    accumulated_depreciation        DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (accumulated_depreciation >= 0),
    book_value                      DECIMAL(18,4) NOT NULL CHECK (book_value >= 0),
    units_used                      INTEGER DEFAULT 0 CHECK (units_used IS NULL OR units_used >= 0),
    last_depreciation_date          DATE,
    depreciation_through_date       DATE,

    -- GL Accounts (override category defaults)
    asset_account_id                CHAR(26) REFERENCES gl.accounts(id),
    accum_depreciation_account_id   CHAR(26) REFERENCES gl.accounts(id),
    depreciation_expense_account_id CHAR(26) REFERENCES gl.accounts(id),

    -- Location & Assignment
    location_code                   VARCHAR(50),
    location_name                   VARCHAR(255),
    department_code                 VARCHAR(50),
    department_name                 VARCHAR(255),
    custodian_id                    CHAR(26),
    custodian_name                  VARCHAR(255),
    cost_center_id                  CHAR(26),

    -- Status
    status                          fa.asset_status NOT NULL DEFAULT 'draft',
    activated_at                    TIMESTAMPTZ,
    activated_by                    CHAR(26),
    suspended_at                    TIMESTAMPTZ,
    suspended_reason                TEXT,
    disposed_at                     TIMESTAMPTZ,
    disposal_type                   fa.disposal_type,
    disposal_proceeds               DECIMAL(18,4),
    disposal_cost                   DECIMAL(18,4) DEFAULT 0,
    disposal_gain_loss              DECIMAL(18,4),
    disposal_journal_id             CHAR(26),
    disposal_notes                  TEXT,

    -- Metadata
    warranty_expiry                 DATE,
    insurance_policy                VARCHAR(100),
    notes                           TEXT,
    tags                            TEXT[],
    custom_fields                   JSONB DEFAULT '{}',

    -- Audit
    created_by                      CHAR(26) NOT NULL,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fa_assets_uk UNIQUE (entity_id, asset_code),
    CONSTRAINT fa_assets_salvage_check CHECK (salvage_value <= acquisition_cost),
    CONSTRAINT fa_assets_book_value_check CHECK (book_value <= acquisition_cost)
);

CREATE INDEX idx_fa_assets_entity ON fa.assets(entity_id);
CREATE INDEX idx_fa_assets_category ON fa.assets(category_id);
CREATE INDEX idx_fa_assets_status ON fa.assets(entity_id, status);
CREATE INDEX idx_fa_assets_location ON fa.assets(entity_id, location_code) WHERE location_code IS NOT NULL;
CREATE INDEX idx_fa_assets_department ON fa.assets(entity_id, department_code) WHERE department_code IS NOT NULL;
CREATE INDEX idx_fa_assets_vendor ON fa.assets(vendor_id) WHERE vendor_id IS NOT NULL;
CREATE INDEX idx_fa_assets_depreciable ON fa.assets(entity_id, status, depreciation_start_date)
    WHERE status = 'active' AND depreciation_start_date IS NOT NULL;
CREATE INDEX idx_fa_assets_search ON fa.assets USING gin(
    to_tsvector('english', asset_name || ' ' || COALESCE(asset_code, '') || ' ' || COALESCE(serial_number, ''))
);

CREATE TABLE fa.depreciation_runs (
    id                      CHAR(26) PRIMARY KEY,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    run_number              VARCHAR(30) NOT NULL,
    fiscal_period_id        CHAR(26) NOT NULL REFERENCES gl.fiscal_periods(id),
    depreciation_date       DATE NOT NULL,
    asset_count             INTEGER NOT NULL DEFAULT 0 CHECK (asset_count >= 0),
    total_depreciation      DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (total_depreciation >= 0),
    currency_code           CHAR(3) NOT NULL REFERENCES currencies(code),
    status                  fa.depreciation_run_status NOT NULL DEFAULT 'draft',
    journal_entry_id        CHAR(26) REFERENCES gl.journal_entries(id),
    notes                   TEXT,
    posted_at               TIMESTAMPTZ,
    posted_by               CHAR(26),
    reversed_at             TIMESTAMPTZ,
    reversed_by             CHAR(26),
    reversal_run_id         CHAR(26) REFERENCES fa.depreciation_runs(id),
    created_by              CHAR(26) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fa_depreciation_runs_uk UNIQUE (entity_id, run_number)
);

CREATE INDEX idx_fa_depreciation_runs_entity ON fa.depreciation_runs(entity_id);
CREATE INDEX idx_fa_depreciation_runs_period ON fa.depreciation_runs(fiscal_period_id);
CREATE INDEX idx_fa_depreciation_runs_status ON fa.depreciation_runs(entity_id, status);
CREATE INDEX idx_fa_depreciation_runs_date ON fa.depreciation_runs(entity_id, depreciation_date);
CREATE INDEX idx_fa_depreciation_runs_journal ON fa.depreciation_runs(journal_entry_id) WHERE journal_entry_id IS NOT NULL;

CREATE TABLE fa.depreciation_entries (
    id                      CHAR(26) PRIMARY KEY,
    depreciation_run_id     CHAR(26) NOT NULL REFERENCES fa.depreciation_runs(id) ON DELETE CASCADE,
    asset_id                CHAR(26) NOT NULL REFERENCES fa.assets(id),
    opening_book_value      DECIMAL(18,4) NOT NULL,
    depreciation_amount     DECIMAL(18,4) NOT NULL CHECK (depreciation_amount >= 0),
    closing_book_value      DECIMAL(18,4) NOT NULL CHECK (closing_book_value >= 0),
    accumulated_before      DECIMAL(18,4) NOT NULL,
    accumulated_after       DECIMAL(18,4) NOT NULL,
    depreciation_method     fa.depreciation_method NOT NULL,
    useful_life_years       INTEGER NOT NULL,
    months_elapsed          INTEGER NOT NULL,
    calculation_basis       TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fa_depreciation_entries_uk UNIQUE (depreciation_run_id, asset_id)
);

CREATE INDEX idx_fa_depreciation_entries_run ON fa.depreciation_entries(depreciation_run_id);
CREATE INDEX idx_fa_depreciation_entries_asset ON fa.depreciation_entries(asset_id);

CREATE TABLE fa.asset_transfers (
    id                  CHAR(26) PRIMARY KEY,
    entity_id           CHAR(26) NOT NULL REFERENCES entities(id),
    transfer_number     VARCHAR(30) NOT NULL,
    asset_id            CHAR(26) NOT NULL REFERENCES fa.assets(id),
    transfer_date       DATE NOT NULL,
    effective_date      DATE NOT NULL,
    from_location_code  VARCHAR(50),
    from_location_name  VARCHAR(255),
    to_location_code    VARCHAR(50),
    to_location_name    VARCHAR(255),
    from_department_code VARCHAR(50),
    from_department_name VARCHAR(255),
    to_department_code  VARCHAR(50),
    to_department_name  VARCHAR(255),
    from_custodian_id   CHAR(26),
    from_custodian_name VARCHAR(255),
    to_custodian_id     CHAR(26),
    to_custodian_name   VARCHAR(255),
    from_cost_center_id CHAR(26),
    to_cost_center_id   CHAR(26),
    reason              TEXT,
    status              fa.transfer_status NOT NULL DEFAULT 'pending',
    approved_by         CHAR(26),
    approved_at         TIMESTAMPTZ,
    completed_by        CHAR(26),
    completed_at        TIMESTAMPTZ,
    created_by          CHAR(26) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fa_asset_transfers_uk UNIQUE (entity_id, transfer_number)
);

CREATE INDEX idx_fa_asset_transfers_entity ON fa.asset_transfers(entity_id);
CREATE INDEX idx_fa_asset_transfers_asset ON fa.asset_transfers(asset_id);
CREATE INDEX idx_fa_asset_transfers_status ON fa.asset_transfers(entity_id, status);
CREATE INDEX idx_fa_asset_transfers_date ON fa.asset_transfers(entity_id, transfer_date);

-- Triggers for updated_at
CREATE TRIGGER trg_fa_asset_categories_updated_at
    BEFORE UPDATE ON fa.asset_categories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_fa_assets_updated_at
    BEFORE UPDATE ON fa.assets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_fa_depreciation_runs_updated_at
    BEFORE UPDATE ON fa.depreciation_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_fa_asset_transfers_updated_at
    BEFORE UPDATE ON fa.asset_transfers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Row Level Security
ALTER TABLE fa.asset_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE fa.assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE fa.depreciation_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE fa.depreciation_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE fa.asset_transfers ENABLE ROW LEVEL SECURITY;

CREATE POLICY fa_asset_categories_entity_isolation ON fa.asset_categories
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY fa_assets_entity_isolation ON fa.assets
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY fa_depreciation_runs_entity_isolation ON fa.depreciation_runs
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY fa_depreciation_entries_entity_isolation ON fa.depreciation_entries
    USING (EXISTS (
        SELECT 1 FROM fa.depreciation_runs r
        WHERE r.id = depreciation_run_id
        AND r.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY fa_asset_transfers_entity_isolation ON fa.asset_transfers
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Sequence generators
CREATE TABLE fa.asset_sequences (
    entity_id   CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION fa.get_next_asset_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'FA')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO fa.asset_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = fa.asset_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 8, '0');
END;
$$ LANGUAGE plpgsql;

CREATE TABLE fa.depreciation_run_sequences (
    entity_id   CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION fa.get_next_depreciation_run_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'DEP')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO fa.depreciation_run_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = fa.depreciation_run_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 8, '0');
END;
$$ LANGUAGE plpgsql;

CREATE TABLE fa.transfer_sequences (
    entity_id   CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION fa.get_next_transfer_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'TRF')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO fa.transfer_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = fa.transfer_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 8, '0');
END;
$$ LANGUAGE plpgsql;

-- Function to apply transfer to asset
CREATE OR REPLACE FUNCTION fa.apply_asset_transfer()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'completed' AND OLD.status != 'completed' THEN
        UPDATE fa.assets
        SET location_code = NEW.to_location_code,
            location_name = NEW.to_location_name,
            department_code = NEW.to_department_code,
            department_name = NEW.to_department_name,
            custodian_id = NEW.to_custodian_id,
            custodian_name = NEW.to_custodian_name,
            cost_center_id = NEW.to_cost_center_id,
            updated_at = NOW()
        WHERE id = NEW.asset_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_fa_apply_asset_transfer
    AFTER UPDATE ON fa.asset_transfers
    FOR EACH ROW EXECUTE FUNCTION fa.apply_asset_transfer();
