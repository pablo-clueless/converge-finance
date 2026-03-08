-- Drop Segment Reporting Schema

-- Drop RLS policies
DROP POLICY IF EXISTS segment_report_data_entity_isolation ON segment.report_data;
DROP POLICY IF EXISTS segment_reports_entity_isolation ON segment.reports;
DROP POLICY IF EXISTS segment_balances_entity_isolation ON segment.balances;
DROP POLICY IF EXISTS segment_intersegment_entity_isolation ON segment.intersegment_transactions;
DROP POLICY IF EXISTS segment_assignments_entity_isolation ON segment.assignments;
DROP POLICY IF EXISTS segment_hierarchy_entity_isolation ON segment.segment_hierarchy;
DROP POLICY IF EXISTS segment_segments_entity_isolation ON segment.segments;

-- Disable RLS
ALTER TABLE segment.report_data DISABLE ROW LEVEL SECURITY;
ALTER TABLE segment.reports DISABLE ROW LEVEL SECURITY;
ALTER TABLE segment.balances DISABLE ROW LEVEL SECURITY;
ALTER TABLE segment.intersegment_transactions DISABLE ROW LEVEL SECURITY;
ALTER TABLE segment.assignments DISABLE ROW LEVEL SECURITY;
ALTER TABLE segment.segment_hierarchy DISABLE ROW LEVEL SECURITY;
ALTER TABLE segment.segments DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_segment_assignments_updated_at ON segment.assignments;
DROP TRIGGER IF EXISTS update_segment_hierarchy_updated_at ON segment.segment_hierarchy;
DROP TRIGGER IF EXISTS update_segment_segments_updated_at ON segment.segments;

-- Drop functions
DROP FUNCTION IF EXISTS segment.generate_report_number(CHAR(26), VARCHAR(10));

-- Drop tables
DROP TABLE IF EXISTS segment.report_data;
DROP TABLE IF EXISTS segment.reports;
DROP TABLE IF EXISTS segment.balances;
DROP TABLE IF EXISTS segment.intersegment_transactions;
DROP TABLE IF EXISTS segment.assignments;
DROP TABLE IF EXISTS segment.segment_hierarchy;
DROP TABLE IF EXISTS segment.segments;

-- Drop enums
DROP TYPE IF EXISTS segment.allocation_basis;
DROP TYPE IF EXISTS segment.segment_type;

-- Drop schema
DROP SCHEMA IF EXISTS segment;
