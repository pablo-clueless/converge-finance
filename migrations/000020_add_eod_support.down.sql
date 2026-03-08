-- Rollback EOD Support

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS close.daily_reconciliation;
DROP TABLE IF EXISTS close.eod_task_runs;
DROP TABLE IF EXISTS close.eod_tasks;
DROP TABLE IF EXISTS close.eod_runs;
DROP TABLE IF EXISTS close.holiday_calendar;
DROP TABLE IF EXISTS close.eod_config;
DROP TABLE IF EXISTS close.business_dates;

-- Drop functions
DROP FUNCTION IF EXISTS close.get_next_business_date(CHAR(26), DATE);
DROP FUNCTION IF EXISTS close.generate_eod_run_number(CHAR(26), DATE);

-- Drop enums
DROP TYPE IF EXISTS close.eod_task_type;
DROP TYPE IF EXISTS close.eod_status;

-- Note: Cannot remove enum values in PostgreSQL
-- The 'day' value in close.close_type and new rule types will remain
-- This is a PostgreSQL limitation
