-- =============================================
-- DROP GROUP CONSOLIDATION TABLES
-- =============================================

-- Drop RLS policies
DROP POLICY IF EXISTS consol_acc_map_entity_isolation ON consol.account_mappings;
DROP POLICY IF EXISTS consol_mi_entity_isolation ON consol.minority_interest;
DROP POLICY IF EXISTS consol_trans_adj_entity_isolation ON consol.translation_adjustments;
DROP POLICY IF EXISTS consol_consolidated_bal_entity_isolation ON consol.consolidated_balances;
DROP POLICY IF EXISTS consol_entity_bal_entity_isolation ON consol.entity_balances;
DROP POLICY IF EXISTS consol_runs_entity_isolation ON consol.consolidation_runs;
DROP POLICY IF EXISTS consol_rates_all_access ON consol.exchange_rates;
DROP POLICY IF EXISTS consol_members_entity_isolation ON consol.consolidation_set_members;
DROP POLICY IF EXISTS consol_sets_entity_isolation ON consol.consolidation_sets;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_consol_acc_mappings_updated_at ON consol.account_mappings;
DROP TRIGGER IF EXISTS trg_consol_runs_updated_at ON consol.consolidation_runs;
DROP TRIGGER IF EXISTS trg_consol_rates_updated_at ON consol.exchange_rates;
DROP TRIGGER IF EXISTS trg_consol_members_updated_at ON consol.consolidation_set_members;
DROP TRIGGER IF EXISTS trg_consol_sets_updated_at ON consol.consolidation_sets;

-- Drop functions
DROP FUNCTION IF EXISTS consol.get_exchange_rate(CHAR(3), CHAR(3), DATE, VARCHAR(20));
DROP FUNCTION IF EXISTS consol.generate_run_number(CHAR(26));

-- Drop tables (in reverse dependency order)
DROP TABLE IF EXISTS consol.account_mappings;
DROP TABLE IF EXISTS consol.minority_interest;
DROP TABLE IF EXISTS consol.translation_adjustments;
DROP TABLE IF EXISTS consol.consolidated_balances;
DROP TABLE IF EXISTS consol.entity_balances;
DROP TABLE IF EXISTS consol.consolidation_runs;
DROP TABLE IF EXISTS consol.exchange_rates;
DROP TABLE IF EXISTS consol.consolidation_set_members;
DROP TABLE IF EXISTS consol.consolidation_sets;

-- Drop enums
DROP TYPE IF EXISTS consol.adjustment_type;
DROP TYPE IF EXISTS consol.run_status;
DROP TYPE IF EXISTS consol.translation_method;

-- Drop schema
DROP SCHEMA IF EXISTS consol;
