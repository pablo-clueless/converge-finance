-- Drop Export Schema

-- Drop RLS policies
DROP POLICY IF EXISTS schedules_entity_isolation ON export.schedules;
DROP POLICY IF EXISTS jobs_entity_isolation ON export.jobs;
DROP POLICY IF EXISTS templates_entity_isolation ON export.templates;

-- Disable RLS
ALTER TABLE export.schedules DISABLE ROW LEVEL SECURITY;
ALTER TABLE export.jobs DISABLE ROW LEVEL SECURITY;
ALTER TABLE export.templates DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_export_schedules_updated_at ON export.schedules;
DROP TRIGGER IF EXISTS update_export_templates_updated_at ON export.templates;

-- Drop functions
DROP FUNCTION IF EXISTS export.generate_job_number(CHAR(26), VARCHAR(10));

-- Drop tables (in dependency order)
DROP TABLE IF EXISTS export.schedules;
DROP TABLE IF EXISTS export.jobs;
DROP TABLE IF EXISTS export.templates;

-- Drop enums
DROP TYPE IF EXISTS export.status;
DROP TYPE IF EXISTS export.format;

-- Drop schema
DROP SCHEMA IF EXISTS export;
