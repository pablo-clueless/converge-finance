-- Drop FX Revaluation Schema

-- Drop functions
DROP FUNCTION IF EXISTS fx.generate_revaluation_run_number(CHAR(26));

-- Drop RLS policies
DROP POLICY IF EXISTS fx_revaluation_details_entity_isolation ON fx.revaluation_details;
DROP POLICY IF EXISTS fx_revaluation_runs_entity_isolation ON fx.revaluation_runs;
DROP POLICY IF EXISTS fx_account_fx_config_entity_isolation ON fx.account_fx_config;

-- Disable RLS
ALTER TABLE fx.revaluation_details DISABLE ROW LEVEL SECURITY;
ALTER TABLE fx.revaluation_runs DISABLE ROW LEVEL SECURITY;
ALTER TABLE fx.account_fx_config DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_fx_revaluation_runs_updated_at ON fx.revaluation_runs;
DROP TRIGGER IF EXISTS update_fx_account_fx_config_updated_at ON fx.account_fx_config;

-- Drop tables (in dependency order)
DROP TABLE IF EXISTS fx.revaluation_details;
DROP TABLE IF EXISTS fx.revaluation_runs;
DROP TABLE IF EXISTS fx.account_fx_config;

-- Drop enums
DROP TYPE IF EXISTS fx.account_fx_treatment;
DROP TYPE IF EXISTS fx.revaluation_status;
DROP TYPE IF EXISTS fx.revaluation_type;
