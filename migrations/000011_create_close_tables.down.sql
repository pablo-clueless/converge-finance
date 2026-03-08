-- Drop Period Close & Reporting Schema

-- Drop RLS policies
DROP POLICY IF EXISTS year_end_checklist_items_entity_isolation ON close.year_end_checklist_items;
DROP POLICY IF EXISTS year_end_checklists_entity_isolation ON close.year_end_checklists;
DROP POLICY IF EXISTS scheduled_reports_entity_isolation ON close.scheduled_reports;
DROP POLICY IF EXISTS report_data_entity_isolation ON close.report_data;
DROP POLICY IF EXISTS report_runs_entity_isolation ON close.report_runs;
DROP POLICY IF EXISTS report_templates_entity_isolation ON close.report_templates;
DROP POLICY IF EXISTS close_run_entries_entity_isolation ON close.close_run_entries;
DROP POLICY IF EXISTS close_runs_entity_isolation ON close.close_runs;
DROP POLICY IF EXISTS close_rules_entity_isolation ON close.close_rules;
DROP POLICY IF EXISTS period_close_status_entity_isolation ON close.period_close_status;

-- Disable RLS
ALTER TABLE close.year_end_checklist_items DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.year_end_checklists DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.scheduled_reports DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.report_data DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.report_runs DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.report_templates DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.close_run_entries DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.close_runs DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.close_rules DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.period_close_status DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_year_end_checklist_items_updated_at ON close.year_end_checklist_items;
DROP TRIGGER IF EXISTS update_year_end_checklists_updated_at ON close.year_end_checklists;
DROP TRIGGER IF EXISTS update_scheduled_reports_updated_at ON close.scheduled_reports;
DROP TRIGGER IF EXISTS update_report_templates_updated_at ON close.report_templates;
DROP TRIGGER IF EXISTS update_close_runs_updated_at ON close.close_runs;
DROP TRIGGER IF EXISTS update_close_rules_updated_at ON close.close_rules;
DROP TRIGGER IF EXISTS update_period_close_status_updated_at ON close.period_close_status;

-- Drop functions
DROP FUNCTION IF EXISTS close.generate_report_number(CHAR(26), VARCHAR(10));
DROP FUNCTION IF EXISTS close.generate_close_run_number(CHAR(26), VARCHAR(10));

-- Drop tables (in dependency order)
DROP TABLE IF EXISTS close.year_end_checklist_items;
DROP TABLE IF EXISTS close.year_end_checklists;
DROP TABLE IF EXISTS close.scheduled_reports;
DROP TABLE IF EXISTS close.report_data;
DROP TABLE IF EXISTS close.report_runs;
DROP TABLE IF EXISTS close.report_templates;
DROP TABLE IF EXISTS close.close_run_entries;
DROP TABLE IF EXISTS close.close_runs;
DROP TABLE IF EXISTS close.close_rules;
DROP TABLE IF EXISTS close.period_close_status;

-- Drop enums
DROP TYPE IF EXISTS close.report_status;
DROP TYPE IF EXISTS close.report_format;
DROP TYPE IF EXISTS close.report_type;
DROP TYPE IF EXISTS close.close_run_status;
DROP TYPE IF EXISTS close.close_rule_type;
DROP TYPE IF EXISTS close.close_type;
DROP TYPE IF EXISTS close.period_status;

-- Drop schema
DROP SCHEMA IF EXISTS close;
