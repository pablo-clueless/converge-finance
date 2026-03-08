-- Drop FX Schema for Triangulation

-- Drop functions
DROP FUNCTION IF EXISTS fx.find_conversion_path(CHAR(26), CHAR(3), CHAR(3), DATE, VARCHAR(20));

-- Drop RLS policies
DROP POLICY IF EXISTS fx_triangulation_log_entity_isolation ON fx.triangulation_log;
DROP POLICY IF EXISTS fx_currency_pair_config_entity_isolation ON fx.currency_pair_config;
DROP POLICY IF EXISTS fx_triangulation_config_entity_isolation ON fx.triangulation_config;

-- Disable RLS
ALTER TABLE fx.triangulation_log DISABLE ROW LEVEL SECURITY;
ALTER TABLE fx.currency_pair_config DISABLE ROW LEVEL SECURITY;
ALTER TABLE fx.triangulation_config DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_fx_currency_pair_config_updated_at ON fx.currency_pair_config;
DROP TRIGGER IF EXISTS update_fx_triangulation_config_updated_at ON fx.triangulation_config;

-- Drop tables
DROP TABLE IF EXISTS fx.triangulation_log;
DROP TABLE IF EXISTS fx.currency_pair_config;
DROP TABLE IF EXISTS fx.triangulation_config;

-- Drop enums
DROP TYPE IF EXISTS fx.triangulation_method;

-- Drop schema (only if empty - will fail if revaluation tables exist)
-- DROP SCHEMA IF EXISTS fx;
