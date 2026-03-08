-- Drop Cash Flow Statement Tables
-- Reverses the cash flow statement schema additions

-- =============================================
-- DROP RLS POLICIES
-- =============================================

DROP POLICY IF EXISTS cashflow_lines_entity_isolation ON close.cashflow_lines;
DROP POLICY IF EXISTS cashflow_runs_entity_isolation ON close.cashflow_runs;
DROP POLICY IF EXISTS cashflow_templates_entity_isolation ON close.cashflow_templates;
DROP POLICY IF EXISTS account_cashflow_config_entity_isolation ON close.account_cashflow_config;

-- =============================================
-- DISABLE RLS
-- =============================================

ALTER TABLE close.cashflow_lines DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.cashflow_runs DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.cashflow_templates DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.account_cashflow_config DISABLE ROW LEVEL SECURITY;

-- =============================================
-- DROP TRIGGERS
-- =============================================

DROP TRIGGER IF EXISTS update_cashflow_runs_updated_at ON close.cashflow_runs;
DROP TRIGGER IF EXISTS update_cashflow_templates_updated_at ON close.cashflow_templates;
DROP TRIGGER IF EXISTS update_account_cashflow_config_updated_at ON close.account_cashflow_config;

-- =============================================
-- DROP FUNCTIONS
-- =============================================

DROP FUNCTION IF EXISTS close.generate_cashflow_run_number(CHAR(26), VARCHAR(10));

-- =============================================
-- DROP TABLES (in dependency order)
-- =============================================

DROP TABLE IF EXISTS close.cashflow_lines;
DROP TABLE IF EXISTS close.cashflow_runs;
DROP TABLE IF EXISTS close.cashflow_templates;
DROP TABLE IF EXISTS close.account_cashflow_config;

-- =============================================
-- DROP ENUMS
-- =============================================

DROP TYPE IF EXISTS close.cashflow_line_type;
DROP TYPE IF EXISTS close.cashflow_category;
DROP TYPE IF EXISTS close.cashflow_method;
