-- Drop RLS policies
DROP POLICY IF EXISTS gl_journal_lines_entity_isolation ON gl.journal_lines;
DROP POLICY IF EXISTS gl_recurring_entries_entity_isolation ON gl.recurring_entries;
DROP POLICY IF EXISTS gl_account_balances_entity_isolation ON gl.account_balances;
DROP POLICY IF EXISTS gl_journal_entries_entity_isolation ON gl.journal_entries;
DROP POLICY IF EXISTS gl_fiscal_periods_entity_isolation ON gl.fiscal_periods;
DROP POLICY IF EXISTS gl_fiscal_years_entity_isolation ON gl.fiscal_years;
DROP POLICY IF EXISTS gl_accounts_entity_isolation ON gl.accounts;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_gl_recurring_entries_updated_at ON gl.recurring_entries;
DROP TRIGGER IF EXISTS trg_gl_journal_entries_updated_at ON gl.journal_entries;
DROP TRIGGER IF EXISTS trg_gl_fiscal_periods_updated_at ON gl.fiscal_periods;
DROP TRIGGER IF EXISTS trg_gl_fiscal_years_updated_at ON gl.fiscal_years;
DROP TRIGGER IF EXISTS trg_gl_accounts_updated_at ON gl.accounts;
DROP TRIGGER IF EXISTS trg_gl_journal_entries_period ON gl.journal_entries;
DROP TRIGGER IF EXISTS trg_gl_journal_entries_balance ON gl.journal_entries;

-- Drop functions
DROP FUNCTION IF EXISTS gl.check_period_open();
DROP FUNCTION IF EXISTS gl.check_journal_balance();

-- Drop tables
DROP TABLE IF EXISTS gl.recurring_entry_lines;
DROP TABLE IF EXISTS gl.recurring_entries;
DROP TABLE IF EXISTS gl.account_balances;
DROP TABLE IF EXISTS gl.journal_lines;
DROP TABLE IF EXISTS gl.journal_entries;
DROP TABLE IF EXISTS gl.fiscal_periods;
DROP TABLE IF EXISTS gl.fiscal_years;
DROP TABLE IF EXISTS gl.accounts;

-- Drop types
DROP TYPE IF EXISTS gl.journal_entry_source;
DROP TYPE IF EXISTS gl.journal_entry_status;
DROP TYPE IF EXISTS gl.fiscal_period_status;
DROP TYPE IF EXISTS gl.fiscal_year_status;
DROP TYPE IF EXISTS gl.account_subtype;
DROP TYPE IF EXISTS gl.account_type;
