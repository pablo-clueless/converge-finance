-- Drop Advanced Reporting (Drill-Down) Extension

-- Drop RLS policies
DROP POLICY IF EXISTS bookmarks_entity_isolation ON close.report_bookmarks;
DROP POLICY IF EXISTS drill_cache_entity_isolation ON close.drill_down_cache;
DROP POLICY IF EXISTS drill_links_entity_isolation ON close.report_drill_links;

-- Disable RLS
ALTER TABLE close.report_bookmarks DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.drill_down_cache DISABLE ROW LEVEL SECURITY;
ALTER TABLE close.report_drill_links DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_report_bookmarks_updated_at ON close.report_bookmarks;

-- Drop functions
DROP FUNCTION IF EXISTS close.get_drill_down_journals(CHAR(26), INTEGER, INTEGER);
DROP FUNCTION IF EXISTS close.clean_expired_drill_cache();

-- Drop tables
DROP TABLE IF EXISTS close.report_bookmarks;
DROP TABLE IF EXISTS close.drill_down_cache;
DROP TABLE IF EXISTS close.report_drill_links;
