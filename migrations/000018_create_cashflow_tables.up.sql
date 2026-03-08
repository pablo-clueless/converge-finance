-- Cash Flow Statement Tables
-- Extends the close schema with cash flow statement functionality

-- =============================================
-- ENUMS
-- =============================================

-- Method used for cash flow statement preparation
CREATE TYPE close.cashflow_method AS ENUM ('indirect', 'direct');

-- Cash flow statement categories
CREATE TYPE close.cashflow_category AS ENUM ('operating', 'investing', 'financing');

-- Line item types in cash flow statement
CREATE TYPE close.cashflow_line_type AS ENUM ('cash_receipt', 'cash_payment', 'adjustment', 'subtotal', 'total');

-- =============================================
-- TABLES
-- =============================================

-- Account Cash Flow Configuration
-- Maps GL accounts to cash flow statement categories and line items
CREATE TABLE close.account_cashflow_config (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    account_id CHAR(26) NOT NULL REFERENCES gl.accounts(id),
    cashflow_category close.cashflow_category NOT NULL,
    line_item_code VARCHAR(30) NOT NULL,
    is_cash_account BOOLEAN NOT NULL DEFAULT false,
    is_cash_equivalent BOOLEAN NOT NULL DEFAULT false,
    adjustment_type VARCHAR(30), -- 'add_back', 'deduct', 'operating_asset_change', 'operating_liability_change', etc.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, account_id)
);

-- Cash Flow Statement Templates
-- Defines the structure and configuration for cash flow statements
CREATE TABLE close.cashflow_templates (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) REFERENCES entities(id), -- NULL for system templates
    template_code VARCHAR(30) NOT NULL,
    template_name VARCHAR(100) NOT NULL,
    method close.cashflow_method NOT NULL,
    configuration JSONB NOT NULL DEFAULT '{}',
    -- Configuration can include:
    -- - line_items: [{code: "NET_INCOME", label: "Net Income", category: "operating", type: "subtotal"}]
    -- - adjustments: [{code: "DEPRECIATION", label: "Add: Depreciation", account_types: ["expense"]}]
    -- - working_capital_items: [{code: "AR_CHANGE", label: "Decrease/(Increase) in Accounts Receivable"}]
    -- - investing_items: [{code: "CAPEX", label: "Purchase of Property and Equipment"}]
    -- - financing_items: [{code: "DEBT_PROCEEDS", label: "Proceeds from Long-term Debt"}]
    is_system BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cash Flow Runs
-- Tracks each cash flow statement generation
CREATE TABLE close.cashflow_runs (
    id CHAR(26) PRIMARY KEY,
    entity_id CHAR(26) NOT NULL REFERENCES entities(id),
    run_number VARCHAR(30) NOT NULL,
    template_id CHAR(26) REFERENCES close.cashflow_templates(id),
    fiscal_period_id CHAR(26) NOT NULL,
    fiscal_year_id CHAR(26) NOT NULL,
    method close.cashflow_method NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    currency_code CHAR(3) NOT NULL,
    operating_net DECIMAL(19,4) NOT NULL DEFAULT 0,
    investing_net DECIMAL(19,4) NOT NULL DEFAULT 0,
    financing_net DECIMAL(19,4) NOT NULL DEFAULT 0,
    net_change DECIMAL(19,4) NOT NULL DEFAULT 0,
    opening_cash DECIMAL(19,4) NOT NULL DEFAULT 0,
    closing_cash DECIMAL(19,4) NOT NULL DEFAULT 0,
    fx_effect DECIMAL(19,4) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- 'pending', 'generating', 'completed', 'failed'
    generated_by CHAR(26) NOT NULL,
    generated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(entity_id, run_number)
);

-- Cash Flow Lines
-- Individual line items in a generated cash flow statement
CREATE TABLE close.cashflow_lines (
    id CHAR(26) PRIMARY KEY,
    cashflow_run_id CHAR(26) NOT NULL REFERENCES close.cashflow_runs(id) ON DELETE CASCADE,
    line_number INTEGER NOT NULL,
    category close.cashflow_category NOT NULL,
    line_type close.cashflow_line_type NOT NULL,
    line_code VARCHAR(30) NOT NULL,
    description VARCHAR(200) NOT NULL,
    amount DECIMAL(19,4) NOT NULL DEFAULT 0,
    indent_level INTEGER NOT NULL DEFAULT 0,
    is_bold BOOLEAN NOT NULL DEFAULT false,
    source_accounts TEXT[], -- Array of account IDs that contributed to this line
    calculation TEXT, -- Description of how this line was calculated
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================
-- INDEXES
-- =============================================

-- Account Cash Flow Config Indexes
CREATE INDEX idx_account_cashflow_config_entity ON close.account_cashflow_config(entity_id);
CREATE INDEX idx_account_cashflow_config_account ON close.account_cashflow_config(account_id);
CREATE INDEX idx_account_cashflow_config_category ON close.account_cashflow_config(cashflow_category);
CREATE INDEX idx_account_cashflow_config_cash ON close.account_cashflow_config(entity_id, is_cash_account) WHERE is_cash_account = true;

-- Cash Flow Templates Indexes
CREATE INDEX idx_cashflow_templates_entity ON close.cashflow_templates(entity_id);
CREATE INDEX idx_cashflow_templates_method ON close.cashflow_templates(method);
CREATE INDEX idx_cashflow_templates_system ON close.cashflow_templates(is_system);
CREATE INDEX idx_cashflow_templates_active ON close.cashflow_templates(is_active) WHERE is_active = true;

-- Cash Flow Runs Indexes
CREATE INDEX idx_cashflow_runs_entity ON close.cashflow_runs(entity_id);
CREATE INDEX idx_cashflow_runs_template ON close.cashflow_runs(template_id);
CREATE INDEX idx_cashflow_runs_period ON close.cashflow_runs(fiscal_period_id);
CREATE INDEX idx_cashflow_runs_year ON close.cashflow_runs(fiscal_year_id);
CREATE INDEX idx_cashflow_runs_status ON close.cashflow_runs(status);
CREATE INDEX idx_cashflow_runs_dates ON close.cashflow_runs(entity_id, period_start, period_end);

-- Cash Flow Lines Indexes
CREATE INDEX idx_cashflow_lines_run ON close.cashflow_lines(cashflow_run_id);
CREATE INDEX idx_cashflow_lines_category ON close.cashflow_lines(cashflow_run_id, category);
CREATE INDEX idx_cashflow_lines_number ON close.cashflow_lines(cashflow_run_id, line_number);

-- =============================================
-- TRIGGERS
-- =============================================

-- Updated_at triggers
CREATE TRIGGER update_account_cashflow_config_updated_at BEFORE UPDATE ON close.account_cashflow_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_cashflow_templates_updated_at BEFORE UPDATE ON close.cashflow_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_cashflow_runs_updated_at BEFORE UPDATE ON close.cashflow_runs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- RUN NUMBER GENERATION FUNCTION
-- =============================================

CREATE OR REPLACE FUNCTION close.generate_cashflow_run_number(p_entity_id CHAR(26), p_prefix VARCHAR(10))
RETURNS VARCHAR(30) AS $$
DECLARE
    v_year TEXT;
    v_sequence INTEGER;
    v_run_number VARCHAR(30);
BEGIN
    v_year := TO_CHAR(CURRENT_DATE, 'YYYY');

    SELECT COALESCE(MAX(
        CAST(SUBSTRING(run_number FROM LENGTH(p_prefix) + 6 FOR 6) AS INTEGER)
    ), 0) + 1
    INTO v_sequence
    FROM close.cashflow_runs
    WHERE entity_id = p_entity_id
    AND run_number LIKE p_prefix || v_year || '%';

    v_run_number := p_prefix || v_year || '-' || LPAD(v_sequence::TEXT, 6, '0');

    RETURN v_run_number;
END;
$$ LANGUAGE plpgsql;

-- =============================================
-- ROW LEVEL SECURITY
-- =============================================

-- Enable RLS
ALTER TABLE close.account_cashflow_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.cashflow_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.cashflow_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE close.cashflow_lines ENABLE ROW LEVEL SECURITY;

-- RLS Policies for account_cashflow_config
CREATE POLICY account_cashflow_config_entity_isolation ON close.account_cashflow_config
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- RLS Policies for cashflow_templates (system templates visible to all)
CREATE POLICY cashflow_templates_entity_isolation ON close.cashflow_templates
    USING (is_system = true OR entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- RLS Policies for cashflow_runs
CREATE POLICY cashflow_runs_entity_isolation ON close.cashflow_runs
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- RLS Policies for cashflow_lines (through parent)
CREATE POLICY cashflow_lines_entity_isolation ON close.cashflow_lines
    USING (cashflow_run_id IN (
        SELECT id FROM close.cashflow_runs
        WHERE entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));

-- =============================================
-- DEFAULT SYSTEM TEMPLATES
-- =============================================

-- Indirect Method Template (most common)
INSERT INTO close.cashflow_templates (id, entity_id, template_code, template_name, method, is_system, configuration) VALUES
    ('01SYSTEM0CFINDIRECT000001', NULL, 'SYS-CF-INDIRECT', 'Cash Flow Statement - Indirect Method', 'indirect', true,
     '{
        "sections": {
            "operating": {
                "label": "Cash Flows from Operating Activities",
                "start_with": "net_income",
                "adjustments": [
                    {"code": "DEPRECIATION", "label": "Depreciation and Amortization", "type": "add_back"},
                    {"code": "LOSS_GAIN_DISPOSAL", "label": "Loss (Gain) on Disposal of Assets", "type": "adjustment"},
                    {"code": "DEFERRED_TAX", "label": "Deferred Income Taxes", "type": "adjustment"}
                ],
                "working_capital_changes": [
                    {"code": "AR_CHANGE", "label": "Decrease (Increase) in Accounts Receivable"},
                    {"code": "INVENTORY_CHANGE", "label": "Decrease (Increase) in Inventory"},
                    {"code": "PREPAID_CHANGE", "label": "Decrease (Increase) in Prepaid Expenses"},
                    {"code": "AP_CHANGE", "label": "Increase (Decrease) in Accounts Payable"},
                    {"code": "ACCRUED_CHANGE", "label": "Increase (Decrease) in Accrued Liabilities"}
                ]
            },
            "investing": {
                "label": "Cash Flows from Investing Activities",
                "items": [
                    {"code": "CAPEX", "label": "Purchase of Property and Equipment", "type": "cash_payment"},
                    {"code": "ASSET_PROCEEDS", "label": "Proceeds from Sale of Assets", "type": "cash_receipt"},
                    {"code": "INVESTMENT_PURCHASE", "label": "Purchase of Investments", "type": "cash_payment"},
                    {"code": "INVESTMENT_PROCEEDS", "label": "Proceeds from Sale of Investments", "type": "cash_receipt"}
                ]
            },
            "financing": {
                "label": "Cash Flows from Financing Activities",
                "items": [
                    {"code": "DEBT_PROCEEDS", "label": "Proceeds from Long-term Debt", "type": "cash_receipt"},
                    {"code": "DEBT_REPAYMENT", "label": "Repayment of Long-term Debt", "type": "cash_payment"},
                    {"code": "EQUITY_ISSUED", "label": "Proceeds from Issuance of Common Stock", "type": "cash_receipt"},
                    {"code": "DIVIDENDS_PAID", "label": "Dividends Paid", "type": "cash_payment"},
                    {"code": "TREASURY_STOCK", "label": "Purchase of Treasury Stock", "type": "cash_payment"}
                ]
            }
        },
        "supplemental": {
            "show_interest_paid": true,
            "show_taxes_paid": true
        }
     }');

-- Direct Method Template
INSERT INTO close.cashflow_templates (id, entity_id, template_code, template_name, method, is_system, configuration) VALUES
    ('01SYSTEM0CFDIRECT00000001', NULL, 'SYS-CF-DIRECT', 'Cash Flow Statement - Direct Method', 'direct', true,
     '{
        "sections": {
            "operating": {
                "label": "Cash Flows from Operating Activities",
                "receipts": [
                    {"code": "CUSTOMER_RECEIPTS", "label": "Cash Received from Customers", "type": "cash_receipt"},
                    {"code": "INTEREST_RECEIVED", "label": "Interest Received", "type": "cash_receipt"},
                    {"code": "DIVIDENDS_RECEIVED", "label": "Dividends Received", "type": "cash_receipt"},
                    {"code": "OTHER_OPERATING_RECEIPTS", "label": "Other Operating Cash Receipts", "type": "cash_receipt"}
                ],
                "payments": [
                    {"code": "SUPPLIER_PAYMENTS", "label": "Cash Paid to Suppliers", "type": "cash_payment"},
                    {"code": "EMPLOYEE_PAYMENTS", "label": "Cash Paid to Employees", "type": "cash_payment"},
                    {"code": "INTEREST_PAID", "label": "Interest Paid", "type": "cash_payment"},
                    {"code": "TAXES_PAID", "label": "Income Taxes Paid", "type": "cash_payment"},
                    {"code": "OTHER_OPERATING_PAYMENTS", "label": "Other Operating Cash Payments", "type": "cash_payment"}
                ]
            },
            "investing": {
                "label": "Cash Flows from Investing Activities",
                "items": [
                    {"code": "CAPEX", "label": "Purchase of Property and Equipment", "type": "cash_payment"},
                    {"code": "ASSET_PROCEEDS", "label": "Proceeds from Sale of Assets", "type": "cash_receipt"},
                    {"code": "INVESTMENT_PURCHASE", "label": "Purchase of Investments", "type": "cash_payment"},
                    {"code": "INVESTMENT_PROCEEDS", "label": "Proceeds from Sale of Investments", "type": "cash_receipt"}
                ]
            },
            "financing": {
                "label": "Cash Flows from Financing Activities",
                "items": [
                    {"code": "DEBT_PROCEEDS", "label": "Proceeds from Long-term Debt", "type": "cash_receipt"},
                    {"code": "DEBT_REPAYMENT", "label": "Repayment of Long-term Debt", "type": "cash_payment"},
                    {"code": "EQUITY_ISSUED", "label": "Proceeds from Issuance of Common Stock", "type": "cash_receipt"},
                    {"code": "DIVIDENDS_PAID", "label": "Dividends Paid", "type": "cash_payment"},
                    {"code": "TREASURY_STOCK", "label": "Purchase of Treasury Stock", "type": "cash_payment"}
                ]
            }
        },
        "reconciliation": {
            "show_indirect_reconciliation": true,
            "label": "Reconciliation of Net Income to Net Cash from Operating Activities"
        }
     }');
