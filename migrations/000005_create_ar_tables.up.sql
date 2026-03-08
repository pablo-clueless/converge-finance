-- =============================================
-- ACCOUNTS RECEIVABLE TABLES
-- =============================================

-- Create AR schema
CREATE SCHEMA IF NOT EXISTS ar;

-- =============================================
-- ENUM TYPES
-- =============================================

-- Customer Types
CREATE TYPE ar.customer_status AS ENUM ('active', 'inactive', 'suspended', 'blocked');
CREATE TYPE ar.customer_type AS ENUM ('individual', 'business', 'government', 'nonprofit');
CREATE TYPE ar.payment_terms AS ENUM ('net_30', 'net_45', 'net_60', 'net_90', 'due_on_receipt', 'prepaid', 'custom');

-- Invoice Types
CREATE TYPE ar.invoice_status AS ENUM ('draft', 'pending', 'approved', 'sent', 'partial', 'paid', 'overdue', 'void', 'writeoff', 'disputed');
CREATE TYPE ar.invoice_type AS ENUM ('standard', 'credit', 'debit', 'proforma', 'recurring');

-- Receipt Types
CREATE TYPE ar.receipt_status AS ENUM ('draft', 'pending', 'confirmed', 'applied', 'reversed', 'void');
CREATE TYPE ar.receipt_method AS ENUM ('cash', 'check', 'ach', 'wire', 'card', 'online', 'lockbox');

-- Dunning Types
CREATE TYPE ar.dunning_level AS ENUM ('none', 'reminder', 'first_notice', 'second_notice', 'final_notice', 'legal_collections');
CREATE TYPE ar.dunning_action AS ENUM ('email', 'letter', 'phone', 'credit_hold', 'collection_agency', 'write_off');

-- =============================================
-- CUSTOMERS
-- =============================================

CREATE TABLE ar.customers (
    id                  CHAR(26) PRIMARY KEY,
    entity_id           CHAR(26) NOT NULL REFERENCES entities(id),
    customer_code       VARCHAR(50) NOT NULL,
    name                VARCHAR(255) NOT NULL,
    legal_name          VARCHAR(255),
    customer_type       ar.customer_type NOT NULL DEFAULT 'business',
    tax_id              VARCHAR(50),
    status              ar.customer_status NOT NULL DEFAULT 'active',

    -- Contact information
    email               VARCHAR(255),
    phone               VARCHAR(50),
    website             VARCHAR(255),

    -- Billing address (stored as JSONB for flexibility)
    billing_address     JSONB NOT NULL DEFAULT '{}',
    shipping_address    JSONB,

    -- Payment settings
    payment_terms       ar.payment_terms NOT NULL DEFAULT 'net_30',
    payment_terms_days  INTEGER DEFAULT 30,
    currency_code       CHAR(3) NOT NULL REFERENCES currencies(code),

    -- Credit management
    credit_limit        DECIMAL(18,4) NOT NULL DEFAULT 0,
    current_balance     DECIMAL(18,4) NOT NULL DEFAULT 0,
    available_credit    DECIMAL(18,4) NOT NULL DEFAULT 0,
    credit_hold_amount  DECIMAL(18,4) NOT NULL DEFAULT 0,
    on_credit_hold      BOOLEAN NOT NULL DEFAULT FALSE,
    credit_hold_reason  TEXT,
    credit_hold_date    TIMESTAMPTZ,
    credit_approved_by  CHAR(26),
    credit_approved_at  TIMESTAMPTZ,

    -- GL Account mappings
    default_revenue_account_id CHAR(26) REFERENCES gl.accounts(id),
    ar_account_id       CHAR(26) REFERENCES gl.accounts(id),

    -- Collection settings
    dunning_enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    last_dunning_date   TIMESTAMPTZ,
    dunning_level       INTEGER NOT NULL DEFAULT 0,

    -- Notes
    notes               TEXT,

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          CHAR(26) NOT NULL,

    CONSTRAINT ar_customers_uk_entity_code UNIQUE (entity_id, customer_code)
);

CREATE INDEX idx_ar_customers_entity ON ar.customers(entity_id);
CREATE INDEX idx_ar_customers_status ON ar.customers(entity_id, status);
CREATE INDEX idx_ar_customers_credit_hold ON ar.customers(entity_id, on_credit_hold) WHERE on_credit_hold = TRUE;
CREATE INDEX idx_ar_customers_search ON ar.customers USING gin(
    to_tsvector('english', name || ' ' || COALESCE(customer_code, '') || ' ' || COALESCE(email, ''))
);

-- Customer Contacts
CREATE TABLE ar.customer_contacts (
    id              CHAR(26) PRIMARY KEY,
    customer_id     CHAR(26) NOT NULL REFERENCES ar.customers(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    title           VARCHAR(100),
    email           VARCHAR(255),
    phone           VARCHAR(50),
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ar_customer_contacts_customer ON ar.customer_contacts(customer_id);

-- =============================================
-- INVOICES
-- =============================================

CREATE TABLE ar.invoices (
    id                  CHAR(26) PRIMARY KEY,
    entity_id           CHAR(26) NOT NULL REFERENCES entities(id),
    customer_id         CHAR(26) NOT NULL REFERENCES ar.customers(id),

    -- Invoice identifiers
    invoice_number      VARCHAR(50) NOT NULL,
    po_number           VARCHAR(100),
    sales_order_id      CHAR(26),

    -- Type and status
    invoice_type        ar.invoice_type NOT NULL DEFAULT 'standard',
    status              ar.invoice_status NOT NULL DEFAULT 'draft',

    -- Dates
    invoice_date        DATE NOT NULL,
    due_date            DATE NOT NULL,
    ship_date           DATE,
    sent_date           TIMESTAMPTZ,
    posting_date        DATE,

    -- Amounts (all in transaction currency)
    currency_code       CHAR(3) NOT NULL REFERENCES currencies(code),
    exchange_rate       DECIMAL(18,8) NOT NULL DEFAULT 1 CHECK (exchange_rate > 0),
    subtotal            DECIMAL(18,4) NOT NULL DEFAULT 0,
    tax_amount          DECIMAL(18,4) NOT NULL DEFAULT 0,
    shipping_amount     DECIMAL(18,4) NOT NULL DEFAULT 0,
    discount_amount     DECIMAL(18,4) NOT NULL DEFAULT 0,
    total_amount        DECIMAL(18,4) NOT NULL DEFAULT 0,
    paid_amount         DECIMAL(18,4) NOT NULL DEFAULT 0,
    balance_due         DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Early payment discount
    discount_terms      VARCHAR(50),
    discount_percent    DECIMAL(5,2) DEFAULT 0,
    discount_due_date   DATE,

    -- GL Integration
    journal_entry_id    CHAR(26) REFERENCES gl.journal_entries(id),

    -- Addresses (stored as JSONB)
    bill_to_address     JSONB NOT NULL DEFAULT '{}',
    ship_to_address     JSONB,

    -- Approval workflow
    approved_by         CHAR(26),
    approved_at         TIMESTAMPTZ,

    -- Dunning/Collections
    dunning_level       INTEGER NOT NULL DEFAULT 0,
    last_dunning_date   TIMESTAMPTZ,
    collection_status   VARCHAR(50),

    -- Write-off
    write_off_date      TIMESTAMPTZ,
    write_off_amount    DECIMAL(18,4) DEFAULT 0,
    write_off_reason    TEXT,

    -- Metadata
    description         TEXT,
    notes               TEXT,
    terms_text          TEXT,
    footer_text         TEXT,

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          CHAR(26) NOT NULL,

    CONSTRAINT ar_invoices_uk_entity_number UNIQUE (entity_id, invoice_number),
    CONSTRAINT ar_invoices_due_after_invoice CHECK (due_date >= invoice_date)
);

CREATE INDEX idx_ar_invoices_entity ON ar.invoices(entity_id);
CREATE INDEX idx_ar_invoices_customer ON ar.invoices(customer_id);
CREATE INDEX idx_ar_invoices_status ON ar.invoices(entity_id, status);
CREATE INDEX idx_ar_invoices_due_date ON ar.invoices(entity_id, due_date);
CREATE INDEX idx_ar_invoices_overdue ON ar.invoices(entity_id, due_date, status)
    WHERE status IN ('sent', 'partial') AND balance_due > 0;
CREATE INDEX idx_ar_invoices_journal ON ar.invoices(journal_entry_id);

-- Invoice Lines
CREATE TABLE ar.invoice_lines (
    id                  CHAR(26) PRIMARY KEY,
    invoice_id          CHAR(26) NOT NULL REFERENCES ar.invoices(id) ON DELETE CASCADE,
    line_number         INTEGER NOT NULL,

    -- Account mapping
    revenue_account_id  CHAR(26) NOT NULL REFERENCES gl.accounts(id),

    -- Item details
    item_code           VARCHAR(50),
    description         TEXT NOT NULL,
    quantity            DECIMAL(18,4) NOT NULL DEFAULT 1 CHECK (quantity != 0),
    unit_price          DECIMAL(18,4) NOT NULL DEFAULT 0,
    amount              DECIMAL(18,4) NOT NULL DEFAULT 0,
    discount_pct        DECIMAL(5,2) DEFAULT 0,
    discount_amt        DECIMAL(18,4) DEFAULT 0,

    -- Tax
    tax_code            VARCHAR(20),
    tax_rate            DECIMAL(5,2) DEFAULT 0,
    tax_amount          DECIMAL(18,4) DEFAULT 0,

    -- Project/cost center tracking
    project_id          CHAR(26),
    cost_center_id      CHAR(26),

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ar_invoice_lines_uk UNIQUE (invoice_id, line_number)
);

CREATE INDEX idx_ar_invoice_lines_invoice ON ar.invoice_lines(invoice_id);
CREATE INDEX idx_ar_invoice_lines_account ON ar.invoice_lines(revenue_account_id);

-- =============================================
-- RECEIPTS
-- =============================================

CREATE TABLE ar.receipts (
    id                  CHAR(26) PRIMARY KEY,
    entity_id           CHAR(26) NOT NULL REFERENCES entities(id),
    customer_id         CHAR(26) NOT NULL REFERENCES ar.customers(id),

    -- Receipt identifiers
    receipt_number      VARCHAR(50) NOT NULL,
    check_number        VARCHAR(50),
    reference_number    VARCHAR(100),

    -- Dates
    receipt_date        DATE NOT NULL,
    deposit_date        DATE,
    cleared_date        DATE,

    -- Status and method
    status              ar.receipt_status NOT NULL DEFAULT 'draft',
    receipt_method      ar.receipt_method NOT NULL,

    -- Amounts
    currency_code       CHAR(3) NOT NULL REFERENCES currencies(code),
    exchange_rate       DECIMAL(18,8) NOT NULL DEFAULT 1 CHECK (exchange_rate > 0),
    amount              DECIMAL(18,4) NOT NULL CHECK (amount > 0),
    applied_amount      DECIMAL(18,4) NOT NULL DEFAULT 0,
    unapplied_amount    DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Bank account
    bank_account_id     CHAR(26) REFERENCES gl.accounts(id),
    bank_reference      VARCHAR(100),

    -- GL Integration
    journal_entry_id    CHAR(26) REFERENCES gl.journal_entries(id),

    -- Reversal information
    reversed_date       TIMESTAMPTZ,
    reversed_by         CHAR(26),
    reversal_reason     TEXT,
    reversal_entry_id   CHAR(26) REFERENCES gl.journal_entries(id),

    -- Metadata
    memo                TEXT,
    notes               TEXT,

    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          CHAR(26) NOT NULL,

    CONSTRAINT ar_receipts_uk_entity_number UNIQUE (entity_id, receipt_number),
    CONSTRAINT ar_receipts_amounts_check CHECK (applied_amount + unapplied_amount = amount)
);

CREATE INDEX idx_ar_receipts_entity ON ar.receipts(entity_id);
CREATE INDEX idx_ar_receipts_customer ON ar.receipts(customer_id);
CREATE INDEX idx_ar_receipts_status ON ar.receipts(entity_id, status);
CREATE INDEX idx_ar_receipts_date ON ar.receipts(entity_id, receipt_date);
CREATE INDEX idx_ar_receipts_unapplied ON ar.receipts(entity_id)
    WHERE status IN ('confirmed', 'draft') AND unapplied_amount > 0;
CREATE INDEX idx_ar_receipts_undeposited ON ar.receipts(entity_id, receipt_method)
    WHERE deposit_date IS NULL AND status != 'void';
CREATE INDEX idx_ar_receipts_journal ON ar.receipts(journal_entry_id);

-- Receipt Applications
CREATE TABLE ar.receipt_applications (
    id              CHAR(26) PRIMARY KEY,
    receipt_id      CHAR(26) NOT NULL REFERENCES ar.receipts(id) ON DELETE CASCADE,
    invoice_id      CHAR(26) NOT NULL REFERENCES ar.invoices(id),

    amount          DECIMAL(18,4) NOT NULL CHECK (amount > 0),
    discount_taken  DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (discount_taken >= 0),

    applied_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT ar_receipt_applications_uk UNIQUE (receipt_id, invoice_id)
);

CREATE INDEX idx_ar_receipt_applications_receipt ON ar.receipt_applications(receipt_id);
CREATE INDEX idx_ar_receipt_applications_invoice ON ar.receipt_applications(invoice_id);

-- Receipt Batches (for lockbox/batch deposits)
CREATE TABLE ar.receipt_batches (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    batch_number    VARCHAR(50) NOT NULL,
    receipt_method  ar.receipt_method NOT NULL,
    deposit_date    DATE NOT NULL,
    status          ar.receipt_status NOT NULL DEFAULT 'pending',

    total_amount    DECIMAL(18,4) NOT NULL DEFAULT 0,
    receipt_count   INTEGER NOT NULL DEFAULT 0,

    bank_account_id CHAR(26) REFERENCES gl.accounts(id),

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      CHAR(26) NOT NULL,
    deposited_at    TIMESTAMPTZ,
    deposited_by    CHAR(26),

    CONSTRAINT ar_receipt_batches_uk UNIQUE (entity_id, batch_number)
);

CREATE INDEX idx_ar_receipt_batches_entity ON ar.receipt_batches(entity_id);
CREATE INDEX idx_ar_receipt_batches_status ON ar.receipt_batches(entity_id, status);

-- =============================================
-- DUNNING
-- =============================================

-- Dunning Profiles
CREATE TABLE ar.dunning_profiles (
    id                      CHAR(26) PRIMARY KEY,
    entity_id               CHAR(26) NOT NULL REFERENCES entities(id),
    name                    VARCHAR(100) NOT NULL,
    description             TEXT,
    is_default              BOOLEAN NOT NULL DEFAULT FALSE,

    -- Grace period before first dunning
    grace_period_days       INTEGER NOT NULL DEFAULT 0,

    -- Auto-actions
    auto_credit_hold_days   INTEGER DEFAULT 0,
    auto_collections_days   INTEGER DEFAULT 0,
    auto_write_off_days     INTEGER DEFAULT 0,
    auto_write_off_threshold DECIMAL(18,4) DEFAULT 0,

    is_active               BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ar_dunning_profiles_entity ON ar.dunning_profiles(entity_id);

-- Dunning Level Configurations (stored in profile as JSONB or separate table)
CREATE TABLE ar.dunning_level_configs (
    id                  CHAR(26) PRIMARY KEY,
    profile_id          CHAR(26) NOT NULL REFERENCES ar.dunning_profiles(id) ON DELETE CASCADE,
    level               INTEGER NOT NULL,
    days_after_due      INTEGER NOT NULL,
    actions             TEXT[] NOT NULL DEFAULT '{}',
    email_template_id   CHAR(26),
    letter_template_id  CHAR(26),
    charge_late_fee     BOOLEAN NOT NULL DEFAULT FALSE,
    late_fee_amount     DECIMAL(18,4) DEFAULT 0,
    late_fee_percent    DECIMAL(5,2) DEFAULT 0,

    CONSTRAINT ar_dunning_level_configs_uk UNIQUE (profile_id, level)
);

CREATE INDEX idx_ar_dunning_level_configs_profile ON ar.dunning_level_configs(profile_id);

-- Dunning History
CREATE TABLE ar.dunning_history (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    customer_id     CHAR(26) NOT NULL REFERENCES ar.customers(id),
    invoice_id      CHAR(26) NOT NULL REFERENCES ar.invoices(id),
    run_id          CHAR(26),

    level           INTEGER NOT NULL,
    action          ar.dunning_action NOT NULL,
    action_date     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- For communications
    email_sent      BOOLEAN NOT NULL DEFAULT FALSE,
    email_address   VARCHAR(255),
    email_sent_at   TIMESTAMPTZ,
    letter_sent     BOOLEAN NOT NULL DEFAULT FALSE,
    letter_sent_at  TIMESTAMPTZ,

    -- For phone calls
    phone_called    BOOLEAN NOT NULL DEFAULT FALSE,
    phone_number    VARCHAR(50),
    phone_called_at TIMESTAMPTZ,
    phone_notes     TEXT,

    -- For fees
    late_fee_charged DECIMAL(18,4) DEFAULT 0,

    -- Response/outcome
    response_received BOOLEAN NOT NULL DEFAULT FALSE,
    response_date    TIMESTAMPTZ,
    response_notes   TEXT,
    promised_amount  DECIMAL(18,4) DEFAULT 0,
    promised_date    DATE,

    notes           TEXT,
    created_by      CHAR(26) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ar_dunning_history_entity ON ar.dunning_history(entity_id);
CREATE INDEX idx_ar_dunning_history_customer ON ar.dunning_history(customer_id);
CREATE INDEX idx_ar_dunning_history_invoice ON ar.dunning_history(invoice_id);
CREATE INDEX idx_ar_dunning_history_date ON ar.dunning_history(entity_id, action_date);

-- Collection Cases
CREATE TABLE ar.collection_cases (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    customer_id     CHAR(26) NOT NULL REFERENCES ar.customers(id),

    -- Case details
    case_number     VARCHAR(50) NOT NULL,
    agency_name     VARCHAR(255),
    agency_contact  VARCHAR(255),
    agency_phone    VARCHAR(50),
    agency_email    VARCHAR(255),

    -- Amounts
    currency_code   CHAR(3) NOT NULL REFERENCES currencies(code),
    original_amount DECIMAL(18,4) NOT NULL,
    current_amount  DECIMAL(18,4) NOT NULL,
    recovered_amount DECIMAL(18,4) NOT NULL DEFAULT 0,
    agency_fees     DECIMAL(18,4) NOT NULL DEFAULT 0,

    -- Status
    status          VARCHAR(50) NOT NULL DEFAULT 'open',
    opened_date     DATE NOT NULL,
    closed_date     DATE,
    closed_reason   TEXT,

    -- Associated invoices (stored as array)
    invoice_ids     CHAR(26)[] NOT NULL DEFAULT '{}',

    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      CHAR(26) NOT NULL,

    CONSTRAINT ar_collection_cases_uk UNIQUE (entity_id, case_number)
);

CREATE INDEX idx_ar_collection_cases_entity ON ar.collection_cases(entity_id);
CREATE INDEX idx_ar_collection_cases_customer ON ar.collection_cases(customer_id);
CREATE INDEX idx_ar_collection_cases_status ON ar.collection_cases(entity_id, status);

-- =============================================
-- TRIGGERS
-- =============================================

-- Updated_at triggers
CREATE TRIGGER trg_ar_customers_updated_at
    BEFORE UPDATE ON ar.customers
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ar_invoices_updated_at
    BEFORE UPDATE ON ar.invoices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ar_receipts_updated_at
    BEFORE UPDATE ON ar.receipts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ar_receipt_batches_updated_at
    BEFORE UPDATE ON ar.receipt_batches
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ar_dunning_profiles_updated_at
    BEFORE UPDATE ON ar.dunning_profiles
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ar_collection_cases_updated_at
    BEFORE UPDATE ON ar.collection_cases
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- FUNCTIONS
-- =============================================

-- Function to recalculate receipt amounts
CREATE OR REPLACE FUNCTION ar.recalculate_receipt_amounts()
RETURNS TRIGGER AS $$
DECLARE
    total_applied DECIMAL(18,4);
    receipt_total DECIMAL(18,4);
BEGIN
    SELECT COALESCE(SUM(amount), 0)
    INTO total_applied
    FROM ar.receipt_applications
    WHERE receipt_id = COALESCE(NEW.receipt_id, OLD.receipt_id);

    SELECT amount INTO receipt_total
    FROM ar.receipts
    WHERE id = COALESCE(NEW.receipt_id, OLD.receipt_id);

    UPDATE ar.receipts
    SET applied_amount = total_applied,
        unapplied_amount = receipt_total - total_applied,
        updated_at = NOW()
    WHERE id = COALESCE(NEW.receipt_id, OLD.receipt_id);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ar_receipt_applications_update_amounts
    AFTER INSERT OR UPDATE OR DELETE ON ar.receipt_applications
    FOR EACH ROW
    EXECUTE FUNCTION ar.recalculate_receipt_amounts();

-- Function to update invoice status on payment
CREATE OR REPLACE FUNCTION ar.update_invoice_on_payment()
RETURNS TRIGGER AS $$
DECLARE
    inv_record RECORD;
    total_paid DECIMAL(18,4);
BEGIN
    -- Get total payments for the invoice
    SELECT COALESCE(SUM(ra.amount), 0)
    INTO total_paid
    FROM ar.receipt_applications ra
    JOIN ar.receipts r ON r.id = ra.receipt_id
    WHERE ra.invoice_id = COALESCE(NEW.invoice_id, OLD.invoice_id)
    AND r.status NOT IN ('reversed', 'void');

    -- Get invoice details
    SELECT * INTO inv_record
    FROM ar.invoices
    WHERE id = COALESCE(NEW.invoice_id, OLD.invoice_id);

    -- Update invoice
    UPDATE ar.invoices
    SET paid_amount = total_paid,
        balance_due = total_amount - total_paid,
        status = CASE
            WHEN total_paid >= total_amount THEN 'paid'::ar.invoice_status
            WHEN total_paid > 0 THEN 'partial'::ar.invoice_status
            ELSE status  -- Keep existing status
        END,
        updated_at = NOW()
    WHERE id = COALESCE(NEW.invoice_id, OLD.invoice_id);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ar_invoice_payment_update
    AFTER INSERT OR UPDATE OR DELETE ON ar.receipt_applications
    FOR EACH ROW
    EXECUTE FUNCTION ar.update_invoice_on_payment();

-- Function to update customer balance
CREATE OR REPLACE FUNCTION ar.update_customer_balance()
RETURNS TRIGGER AS $$
DECLARE
    cust_id CHAR(26);
    total_balance DECIMAL(18,4);
    cust_credit_limit DECIMAL(18,4);
BEGIN
    -- Get customer ID
    cust_id := COALESCE(NEW.customer_id, OLD.customer_id);

    -- Calculate total outstanding balance
    SELECT COALESCE(SUM(balance_due), 0)
    INTO total_balance
    FROM ar.invoices
    WHERE customer_id = cust_id
    AND status NOT IN ('void', 'paid', 'writeoff');

    -- Get credit limit
    SELECT credit_limit INTO cust_credit_limit
    FROM ar.customers
    WHERE id = cust_id;

    -- Update customer
    UPDATE ar.customers
    SET current_balance = total_balance,
        available_credit = GREATEST(cust_credit_limit - total_balance, 0),
        updated_at = NOW()
    WHERE id = cust_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ar_customer_balance_invoice
    AFTER INSERT OR UPDATE OF balance_due, status OR DELETE ON ar.invoices
    FOR EACH ROW
    EXECUTE FUNCTION ar.update_customer_balance();

-- =============================================
-- ROW LEVEL SECURITY
-- =============================================

ALTER TABLE ar.customers ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.customer_contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.invoice_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.receipt_applications ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.receipt_batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.dunning_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.dunning_level_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.dunning_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE ar.collection_cases ENABLE ROW LEVEL SECURITY;

-- RLS Policies
CREATE POLICY ar_customers_entity_isolation ON ar.customers
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ar_customer_contacts_entity_isolation ON ar.customer_contacts
    USING (EXISTS (
        SELECT 1 FROM ar.customers c
        WHERE c.id = customer_id
        AND c.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY ar_invoices_entity_isolation ON ar.invoices
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ar_invoice_lines_entity_isolation ON ar.invoice_lines
    USING (EXISTS (
        SELECT 1 FROM ar.invoices i
        WHERE i.id = invoice_id
        AND i.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY ar_receipts_entity_isolation ON ar.receipts
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ar_receipt_applications_entity_isolation ON ar.receipt_applications
    USING (EXISTS (
        SELECT 1 FROM ar.receipts r
        WHERE r.id = receipt_id
        AND r.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY ar_receipt_batches_entity_isolation ON ar.receipt_batches
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ar_dunning_profiles_entity_isolation ON ar.dunning_profiles
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ar_dunning_level_configs_entity_isolation ON ar.dunning_level_configs
    USING (EXISTS (
        SELECT 1 FROM ar.dunning_profiles p
        WHERE p.id = profile_id
        AND p.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY ar_dunning_history_entity_isolation ON ar.dunning_history
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ar_collection_cases_entity_isolation ON ar.collection_cases
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- =============================================
-- SEQUENCE FUNCTIONS FOR NUMBERS
-- =============================================

-- Customer number sequence per entity
CREATE TABLE ar.customer_sequences (
    entity_id       CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number     INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION ar.get_next_customer_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'CUST')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO ar.customer_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = ar.customer_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

-- Invoice number sequence per entity
CREATE TABLE ar.invoice_sequences (
    entity_id       CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number     INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION ar.get_next_invoice_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'INV')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO ar.invoice_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = ar.invoice_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 8, '0');
END;
$$ LANGUAGE plpgsql;

-- Receipt number sequence per entity
CREATE TABLE ar.receipt_sequences (
    entity_id       CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number     INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION ar.get_next_receipt_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'RCP')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO ar.receipt_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = ar.receipt_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 8, '0');
END;
$$ LANGUAGE plpgsql;
