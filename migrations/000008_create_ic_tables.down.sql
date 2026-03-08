-- =============================================
-- DROP INTERCOMPANY (IC) TABLES
-- Reverse migration for IC module
-- =============================================

-- Drop RLS policies
DROP POLICY IF EXISTS ic_elimination_entries_entity_isolation ON ic.elimination_entries;
DROP POLICY IF EXISTS ic_elimination_runs_entity_isolation ON ic.elimination_runs;
DROP POLICY IF EXISTS ic_elimination_rules_entity_isolation ON ic.elimination_rules;
DROP POLICY IF EXISTS ic_entity_pair_balances_entity_isolation ON ic.entity_pair_balances;
DROP POLICY IF EXISTS ic_transaction_lines_entity_isolation ON ic.transaction_lines;
DROP POLICY IF EXISTS ic_transactions_entity_isolation ON ic.transactions;
DROP POLICY IF EXISTS ic_account_mappings_entity_isolation ON ic.account_mappings;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_ic_elimination_runs_updated_at ON ic.elimination_runs;
DROP TRIGGER IF EXISTS trg_ic_elimination_rules_updated_at ON ic.elimination_rules;
DROP TRIGGER IF EXISTS trg_ic_entity_pair_balances_updated_at ON ic.entity_pair_balances;
DROP TRIGGER IF EXISTS trg_ic_transactions_updated_at ON ic.transactions;
DROP TRIGGER IF EXISTS trg_ic_account_mappings_updated_at ON ic.account_mappings;

-- Drop entity hierarchy triggers
DROP TRIGGER IF EXISTS trg_entities_no_circular ON entities;
DROP TRIGGER IF EXISTS trg_entities_hierarchy ON entities;

-- Drop functions
DROP FUNCTION IF EXISTS ic.generate_elimination_run_number(CHAR(26));
DROP FUNCTION IF EXISTS ic.generate_transaction_number(CHAR(26));
DROP FUNCTION IF EXISTS check_entity_circular_reference();
DROP FUNCTION IF EXISTS update_entity_hierarchy();

-- Drop IC tables (in reverse order of dependencies)
DROP TABLE IF EXISTS ic.elimination_entries;
DROP TABLE IF EXISTS ic.elimination_runs;
DROP TABLE IF EXISTS ic.elimination_rules;
DROP TABLE IF EXISTS ic.entity_pair_balances;
DROP TABLE IF EXISTS ic.transaction_lines;
DROP TABLE IF EXISTS ic.transactions;
DROP TABLE IF EXISTS ic.account_mappings;

-- Drop IC types
DROP TYPE IF EXISTS ic.elimination_status;
DROP TYPE IF EXISTS ic.elimination_type;
DROP TYPE IF EXISTS ic.transaction_status;
DROP TYPE IF EXISTS ic.transaction_type;

-- Drop IC schema
DROP SCHEMA IF EXISTS ic;

-- Remove hierarchy columns from entities table
-- Note: Cannot easily remove enum values from gl.journal_entry_source
ALTER TABLE entities DROP COLUMN IF EXISTS hierarchy_path;
ALTER TABLE entities DROP COLUMN IF EXISTS hierarchy_level;
ALTER TABLE entities DROP COLUMN IF EXISTS consolidation_method;
ALTER TABLE entities DROP COLUMN IF EXISTS ownership_percent;
ALTER TABLE entities DROP COLUMN IF EXISTS entity_type;
ALTER TABLE entities DROP COLUMN IF EXISTS parent_id;

-- Drop indexes on entities
DROP INDEX IF EXISTS idx_entities_hierarchy_level;
DROP INDEX IF EXISTS idx_entities_hierarchy_path;
DROP INDEX IF EXISTS idx_entities_parent;

-- Drop entity-related types
DROP TYPE IF EXISTS consolidation_method;
DROP TYPE IF EXISTS entity_type;
