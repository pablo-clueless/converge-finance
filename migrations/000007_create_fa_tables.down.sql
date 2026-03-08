-- Drop triggers first
DROP TRIGGER IF EXISTS trg_fa_apply_asset_transfer ON fa.asset_transfers;
DROP TRIGGER IF EXISTS trg_fa_asset_transfers_updated_at ON fa.asset_transfers;
DROP TRIGGER IF EXISTS trg_fa_depreciation_runs_updated_at ON fa.depreciation_runs;
DROP TRIGGER IF EXISTS trg_fa_assets_updated_at ON fa.assets;
DROP TRIGGER IF EXISTS trg_fa_asset_categories_updated_at ON fa.asset_categories;

-- Drop functions
DROP FUNCTION IF EXISTS fa.apply_asset_transfer();
DROP FUNCTION IF EXISTS fa.get_next_transfer_number(CHAR(26), VARCHAR(10));
DROP FUNCTION IF EXISTS fa.get_next_depreciation_run_number(CHAR(26), VARCHAR(10));
DROP FUNCTION IF EXISTS fa.get_next_asset_number(CHAR(26), VARCHAR(10));

-- Drop policies
DROP POLICY IF EXISTS fa_asset_transfers_entity_isolation ON fa.asset_transfers;
DROP POLICY IF EXISTS fa_depreciation_entries_entity_isolation ON fa.depreciation_entries;
DROP POLICY IF EXISTS fa_depreciation_runs_entity_isolation ON fa.depreciation_runs;
DROP POLICY IF EXISTS fa_assets_entity_isolation ON fa.assets;
DROP POLICY IF EXISTS fa_asset_categories_entity_isolation ON fa.asset_categories;

-- Drop sequence tables
DROP TABLE IF EXISTS fa.transfer_sequences;
DROP TABLE IF EXISTS fa.depreciation_run_sequences;
DROP TABLE IF EXISTS fa.asset_sequences;

-- Drop main tables in reverse order of dependencies
DROP TABLE IF EXISTS fa.asset_transfers;
DROP TABLE IF EXISTS fa.depreciation_entries;
DROP TABLE IF EXISTS fa.depreciation_runs;
DROP TABLE IF EXISTS fa.assets;
DROP TABLE IF EXISTS fa.asset_categories;

-- Drop types
DROP TYPE IF EXISTS fa.depreciation_run_status;
DROP TYPE IF EXISTS fa.transfer_status;
DROP TYPE IF EXISTS fa.disposal_type;
DROP TYPE IF EXISTS fa.depreciation_method;
DROP TYPE IF EXISTS fa.asset_status;

-- Drop schema
DROP SCHEMA IF EXISTS fa;
