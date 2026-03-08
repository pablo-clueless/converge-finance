CREATE SCHEMA IF NOT EXISTS ap;

CREATE TYPE ap.vendor_status AS ENUM ('active', 'inactive', 'blocked');
CREATE TYPE ap.payment_terms AS ENUM ('net_30', 'net_45', 'net_60', 'net_90', 'due_on_receipt', 'custom');
CREATE TYPE ap.payment_method AS ENUM ('check', 'ach', 'wire', 'card');
CREATE TYPE ap.invoice_status AS ENUM ('draft', 'pending', 'approved', 'partial', 'paid', 'void', 'disputed');
CREATE TYPE ap.payment_status AS ENUM ('draft', 'pending', 'approved', 'scheduled', 'processing', 'completed', 'failed', 'void');
CREATE TYPE ap.payment_type AS ENUM ('regular', 'advance', 'refund');

CREATE TABLE ap.vendors (
    id                          CHAR(26) PRIMARY KEY,
    entity_id                   CHAR(26) NOT NULL REFERENCES entities(id),
    vendor_code                 VARCHAR(50) NOT NULL,
    name                        VARCHAR(255) NOT NULL,
    legal_name                  VARCHAR(255),
    tax_id                      VARCHAR(50),
    status                      ap.vendor_status NOT NULL DEFAULT 'active',
    email                       VARCHAR(255),
    phone                       VARCHAR(50),
    website                     VARCHAR(255),
    billing_address             JSONB NOT NULL DEFAULT '{}',
    remit_to_address            JSONB,
    payment_terms               ap.payment_terms NOT NULL DEFAULT 'net_30',
    payment_terms_days          INTEGER DEFAULT 30,
    payment_method              ap.payment_method NOT NULL DEFAULT 'check',
    currency_code               CHAR(3) NOT NULL REFERENCES currencies(code),
    bank_info                   JSONB,
    credit_limit                DECIMAL(18,4) NOT NULL DEFAULT 0,
    current_balance             DECIMAL(18,4) NOT NULL DEFAULT 0,
    default_expense_account_id  CHAR(26) REFERENCES gl.accounts(id),
    ap_account_id               CHAR(26) REFERENCES gl.accounts(id),
    is_1099_vendor              BOOLEAN NOT NULL DEFAULT FALSE,
    w9_on_file                  BOOLEAN NOT NULL DEFAULT FALSE,
    w9_received_date            DATE,
    notes                       TEXT,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by                  CHAR(26) NOT NULL,
    CONSTRAINT ap_vendors_uk_entity_code UNIQUE (entity_id, vendor_code)
);

CREATE INDEX idx_ap_vendors_entity ON ap.vendors(entity_id);
CREATE INDEX idx_ap_vendors_status ON ap.vendors(entity_id, status);
CREATE INDEX idx_ap_vendors_1099 ON ap.vendors(entity_id, is_1099_vendor) WHERE is_1099_vendor = TRUE;
CREATE INDEX idx_ap_vendors_search ON ap.vendors USING gin(
    to_tsvector('english', name || ' ' || COALESCE(vendor_code, '') || ' ' || COALESCE(email, ''))
);

CREATE TABLE ap.invoices (
    id                  CHAR(26) PRIMARY KEY,
    entity_id           CHAR(26) NOT NULL REFERENCES entities(id),
    vendor_id           CHAR(26) NOT NULL REFERENCES ap.vendors(id),
    invoice_number      VARCHAR(100) NOT NULL,
    internal_number     VARCHAR(50),
    po_number           VARCHAR(100),
    invoice_date        DATE NOT NULL,
    received_date       DATE NOT NULL,
    due_date            DATE NOT NULL,
    posting_date        DATE,
    status              ap.invoice_status NOT NULL DEFAULT 'draft',
    currency_code       CHAR(3) NOT NULL REFERENCES currencies(code),
    exchange_rate       DECIMAL(18,8) NOT NULL DEFAULT 1 CHECK (exchange_rate > 0),
    subtotal            DECIMAL(18,4) NOT NULL DEFAULT 0,
    tax_amount          DECIMAL(18,4) NOT NULL DEFAULT 0,
    shipping_amount     DECIMAL(18,4) NOT NULL DEFAULT 0,
    discount_amount     DECIMAL(18,4) NOT NULL DEFAULT 0,
    total_amount        DECIMAL(18,4) NOT NULL DEFAULT 0,
    paid_amount         DECIMAL(18,4) NOT NULL DEFAULT 0,
    balance_due         DECIMAL(18,4) NOT NULL DEFAULT 0,
    discount_terms      VARCHAR(50),
    discount_percent    DECIMAL(5,2) DEFAULT 0,
    discount_due_date   DATE,
    journal_entry_id    CHAR(26) REFERENCES gl.journal_entries(id),
    approved_by         CHAR(26),
    approved_at         TIMESTAMPTZ,
    approval_notes      TEXT,
    description         TEXT,
    notes               TEXT,
    attachments         TEXT[],
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by          CHAR(26) NOT NULL,
    CONSTRAINT ap_invoices_uk_entity_vendor_number UNIQUE (entity_id, vendor_id, invoice_number),
    CONSTRAINT ap_invoices_due_after_invoice CHECK (due_date >= invoice_date)
);

CREATE INDEX idx_ap_invoices_entity ON ap.invoices(entity_id);
CREATE INDEX idx_ap_invoices_vendor ON ap.invoices(vendor_id);
CREATE INDEX idx_ap_invoices_status ON ap.invoices(entity_id, status);
CREATE INDEX idx_ap_invoices_due_date ON ap.invoices(entity_id, due_date);
CREATE INDEX idx_ap_invoices_overdue ON ap.invoices(entity_id, due_date, status)
    WHERE status IN ('approved', 'partial') AND balance_due > 0;
CREATE INDEX idx_ap_invoices_journal ON ap.invoices(journal_entry_id);

CREATE TABLE ap.invoice_lines (
    id              CHAR(26) PRIMARY KEY,
    invoice_id      CHAR(26) NOT NULL REFERENCES ap.invoices(id) ON DELETE CASCADE,
    line_number     INTEGER NOT NULL,
    account_id      CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    description     TEXT NOT NULL,
    quantity        DECIMAL(18,4) NOT NULL DEFAULT 1 CHECK (quantity != 0),
    unit_price      DECIMAL(18,4) NOT NULL DEFAULT 0,
    amount          DECIMAL(18,4) NOT NULL DEFAULT 0,
    tax_code        VARCHAR(20),
    tax_amount      DECIMAL(18,4) DEFAULT 0,
    item_code       VARCHAR(50),
    project_id      CHAR(26),
    cost_center_id  CHAR(26),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ap_invoice_lines_uk UNIQUE (invoice_id, line_number)
);

CREATE INDEX idx_ap_invoice_lines_invoice ON ap.invoice_lines(invoice_id);
CREATE INDEX idx_ap_invoice_lines_account ON ap.invoice_lines(account_id);

CREATE TABLE ap.payments (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    vendor_id       CHAR(26) NOT NULL REFERENCES ap.vendors(id),
    payment_number  VARCHAR(50) NOT NULL,
    check_number    VARCHAR(50),
    payment_date    DATE NOT NULL,
    scheduled_date  DATE,
    cleared_date    DATE,
    status          ap.payment_status NOT NULL DEFAULT 'draft',
    payment_type    ap.payment_type NOT NULL DEFAULT 'regular',
    payment_method  ap.payment_method NOT NULL,
    currency_code   CHAR(3) NOT NULL REFERENCES currencies(code),
    exchange_rate   DECIMAL(18,8) NOT NULL DEFAULT 1 CHECK (exchange_rate > 0),
    amount          DECIMAL(18,4) NOT NULL DEFAULT 0,
    discount_taken  DECIMAL(18,4) NOT NULL DEFAULT 0,
    bank_account_id CHAR(26) REFERENCES gl.accounts(id),
    bank_reference  VARCHAR(100),
    journal_entry_id CHAR(26) REFERENCES gl.journal_entries(id),
    approved_by     CHAR(26),
    approved_at     TIMESTAMPTZ,
    failure_reason  TEXT,
    memo            TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      CHAR(26) NOT NULL,
    CONSTRAINT ap_payments_uk_entity_number UNIQUE (entity_id, payment_number)
);

CREATE INDEX idx_ap_payments_entity ON ap.payments(entity_id);
CREATE INDEX idx_ap_payments_vendor ON ap.payments(vendor_id);
CREATE INDEX idx_ap_payments_status ON ap.payments(entity_id, status);
CREATE INDEX idx_ap_payments_date ON ap.payments(entity_id, payment_date);
CREATE INDEX idx_ap_payments_scheduled ON ap.payments(entity_id, scheduled_date)
    WHERE status = 'scheduled' AND scheduled_date IS NOT NULL;
CREATE INDEX idx_ap_payments_journal ON ap.payments(journal_entry_id);

CREATE TABLE ap.payment_allocations (
    id              CHAR(26) PRIMARY KEY,
    payment_id      CHAR(26) NOT NULL REFERENCES ap.payments(id) ON DELETE CASCADE,
    invoice_id      CHAR(26) NOT NULL REFERENCES ap.invoices(id),
    amount          DECIMAL(18,4) NOT NULL CHECK (amount > 0),
    discount_taken  DECIMAL(18,4) NOT NULL DEFAULT 0 CHECK (discount_taken >= 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ap_payment_allocations_uk UNIQUE (payment_id, invoice_id)
);

CREATE INDEX idx_ap_payment_allocations_payment ON ap.payment_allocations(payment_id);
CREATE INDEX idx_ap_payment_allocations_invoice ON ap.payment_allocations(invoice_id);

CREATE TABLE ap.payment_batches (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    batch_number    VARCHAR(50) NOT NULL,
    payment_method  ap.payment_method NOT NULL,
    payment_date    DATE NOT NULL,
    status          ap.payment_status NOT NULL DEFAULT 'pending',
    total_amount    DECIMAL(18,4) NOT NULL DEFAULT 0,
    payment_count   INTEGER NOT NULL DEFAULT 0,
    bank_account_id CHAR(26) REFERENCES gl.accounts(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by      CHAR(26) NOT NULL,
    processed_at    TIMESTAMPTZ,
    processed_by    CHAR(26),
    CONSTRAINT ap_payment_batches_uk UNIQUE (entity_id, batch_number)
);

CREATE INDEX idx_ap_payment_batches_entity ON ap.payment_batches(entity_id);
CREATE INDEX idx_ap_payment_batches_status ON ap.payment_batches(entity_id, status);

CREATE TRIGGER trg_ap_vendors_updated_at
    BEFORE UPDATE ON ap.vendors
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ap_invoices_updated_at
    BEFORE UPDATE ON ap.invoices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ap_payments_updated_at
    BEFORE UPDATE ON ap.payments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_ap_payment_batches_updated_at
    BEFORE UPDATE ON ap.payment_batches
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE OR REPLACE FUNCTION ap.recalculate_payment_amounts()
RETURNS TRIGGER AS $$
DECLARE
    total_allocated DECIMAL(18,4);
    total_discount DECIMAL(18,4);
BEGIN
    SELECT COALESCE(SUM(amount), 0), COALESCE(SUM(discount_taken), 0)
    INTO total_allocated, total_discount
    FROM ap.payment_allocations
    WHERE payment_id = COALESCE(NEW.payment_id, OLD.payment_id);

    UPDATE ap.payments
    SET amount = total_allocated,
        discount_taken = total_discount,
        updated_at = NOW()
    WHERE id = COALESCE(NEW.payment_id, OLD.payment_id);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ap_payment_allocations_update_amounts
    AFTER INSERT OR UPDATE OR DELETE ON ap.payment_allocations
    FOR EACH ROW EXECUTE FUNCTION ap.recalculate_payment_amounts();

CREATE OR REPLACE FUNCTION ap.update_invoice_on_payment()
RETURNS TRIGGER AS $$
DECLARE
    total_paid DECIMAL(18,4);
    inv_total DECIMAL(18,4);
BEGIN
    SELECT COALESCE(SUM(pa.amount), 0)
    INTO total_paid
    FROM ap.payment_allocations pa
    JOIN ap.payments p ON p.id = pa.payment_id
    WHERE pa.invoice_id = COALESCE(NEW.invoice_id, OLD.invoice_id)
    AND p.status NOT IN ('failed', 'void');

    SELECT total_amount INTO inv_total
    FROM ap.invoices
    WHERE id = COALESCE(NEW.invoice_id, OLD.invoice_id);

    UPDATE ap.invoices
    SET paid_amount = total_paid,
        balance_due = total_amount - total_paid,
        status = CASE
            WHEN total_paid >= total_amount THEN 'paid'::ap.invoice_status
            WHEN total_paid > 0 THEN 'partial'::ap.invoice_status
            ELSE status
        END,
        updated_at = NOW()
    WHERE id = COALESCE(NEW.invoice_id, OLD.invoice_id);

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ap_invoice_payment_update
    AFTER INSERT OR UPDATE OR DELETE ON ap.payment_allocations
    FOR EACH ROW EXECUTE FUNCTION ap.update_invoice_on_payment();

CREATE OR REPLACE FUNCTION ap.update_vendor_balance()
RETURNS TRIGGER AS $$
DECLARE
    v_id CHAR(26);
    total_balance DECIMAL(18,4);
BEGIN
    v_id := COALESCE(NEW.vendor_id, OLD.vendor_id);

    SELECT COALESCE(SUM(balance_due), 0)
    INTO total_balance
    FROM ap.invoices
    WHERE vendor_id = v_id
    AND status NOT IN ('void', 'paid');

    UPDATE ap.vendors
    SET current_balance = total_balance,
        updated_at = NOW()
    WHERE id = v_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ap_vendor_balance_invoice
    AFTER INSERT OR UPDATE OF balance_due, status OR DELETE ON ap.invoices
    FOR EACH ROW EXECUTE FUNCTION ap.update_vendor_balance();

ALTER TABLE ap.vendors ENABLE ROW LEVEL SECURITY;
ALTER TABLE ap.invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE ap.invoice_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE ap.payments ENABLE ROW LEVEL SECURITY;
ALTER TABLE ap.payment_allocations ENABLE ROW LEVEL SECURITY;
ALTER TABLE ap.payment_batches ENABLE ROW LEVEL SECURITY;

CREATE POLICY ap_vendors_entity_isolation ON ap.vendors
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ap_invoices_entity_isolation ON ap.invoices
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ap_invoice_lines_entity_isolation ON ap.invoice_lines
    USING (EXISTS (
        SELECT 1 FROM ap.invoices i
        WHERE i.id = invoice_id
        AND i.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY ap_payments_entity_isolation ON ap.payments
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE POLICY ap_payment_allocations_entity_isolation ON ap.payment_allocations
    USING (EXISTS (
        SELECT 1 FROM ap.payments p
        WHERE p.id = payment_id
        AND p.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

CREATE POLICY ap_payment_batches_entity_isolation ON ap.payment_batches
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

CREATE TABLE ap.vendor_sequences (
    entity_id   CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION ap.get_next_vendor_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'VND')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO ap.vendor_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = ap.vendor_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 6, '0');
END;
$$ LANGUAGE plpgsql;

CREATE TABLE ap.invoice_sequences (
    entity_id   CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION ap.get_next_invoice_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'APINV')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO ap.invoice_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = ap.invoice_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 8, '0');
END;
$$ LANGUAGE plpgsql;

CREATE TABLE ap.payment_sequences (
    entity_id   CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION ap.get_next_payment_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'PMT')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO ap.payment_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = ap.payment_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 8, '0');
END;
$$ LANGUAGE plpgsql;
