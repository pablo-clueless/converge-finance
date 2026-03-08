-- =============================================
-- DROP COST ACCOUNTING & BUDGETING TABLES
-- =============================================

-- Drop RLS policies
DROP POLICY IF EXISTS cost_actuals_entity_isolation ON cost.budget_actuals;
DROP POLICY IF EXISTS cost_budget_transfers_entity_isolation ON cost.budget_transfers;
DROP POLICY IF EXISTS cost_budget_lines_entity_isolation ON cost.budget_lines;
DROP POLICY IF EXISTS cost_budgets_entity_isolation ON cost.budgets;
DROP POLICY IF EXISTS cost_alloc_entries_entity_isolation ON cost.allocation_entries;
DROP POLICY IF EXISTS cost_alloc_runs_entity_isolation ON cost.allocation_runs;
DROP POLICY IF EXISTS cost_alloc_targets_entity_isolation ON cost.allocation_targets;
DROP POLICY IF EXISTS cost_alloc_rules_entity_isolation ON cost.allocation_rules;
DROP POLICY IF EXISTS cost_centers_entity_isolation ON cost.cost_centers;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_budget_lines_totals ON cost.budget_lines;
DROP TRIGGER IF EXISTS trg_cost_budget_lines_updated_at ON cost.budget_lines;
DROP TRIGGER IF EXISTS trg_cost_budgets_updated_at ON cost.budgets;
DROP TRIGGER IF EXISTS trg_cost_alloc_runs_updated_at ON cost.allocation_runs;
DROP TRIGGER IF EXISTS trg_cost_alloc_rules_updated_at ON cost.allocation_rules;
DROP TRIGGER IF EXISTS trg_cost_centers_updated_at ON cost.cost_centers;
DROP TRIGGER IF EXISTS trg_cost_centers_hierarchy ON cost.cost_centers;

-- Drop functions
DROP FUNCTION IF EXISTS cost.recalculate_budget_totals();
DROP FUNCTION IF EXISTS cost.update_cost_center_hierarchy();
DROP FUNCTION IF EXISTS cost.generate_transfer_number(CHAR(26));
DROP FUNCTION IF EXISTS cost.generate_allocation_run_number(CHAR(26));

-- Drop tables (in reverse dependency order)
DROP TABLE IF EXISTS cost.budget_actuals;
DROP TABLE IF EXISTS cost.budget_transfers;
DROP TABLE IF EXISTS cost.budget_lines;
DROP TABLE IF EXISTS cost.budgets;
DROP TABLE IF EXISTS cost.allocation_entries;
DROP TABLE IF EXISTS cost.allocation_runs;
DROP TABLE IF EXISTS cost.allocation_targets;
DROP TABLE IF EXISTS cost.allocation_rules;
DROP TABLE IF EXISTS cost.cost_centers;

-- Drop enums
DROP TYPE IF EXISTS cost.budget_type;
DROP TYPE IF EXISTS cost.budget_status;
DROP TYPE IF EXISTS cost.allocation_status;
DROP TYPE IF EXISTS cost.allocation_method;
DROP TYPE IF EXISTS cost.center_type;

-- Drop schema
DROP SCHEMA IF EXISTS cost;
