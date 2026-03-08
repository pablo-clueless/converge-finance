-- =============================================
-- DROP ACCOUNTS RECEIVABLE TABLES
-- =============================================

-- Drop sequence functions
DROP FUNCTION IF EXISTS ar.get_next_receipt_number(CHAR(26), VARCHAR(10));
DROP FUNCTION IF EXISTS ar.get_next_invoice_number(CHAR(26), VARCHAR(10));
DROP FUNCTION IF EXISTS ar.get_next_customer_number(CHAR(26), VARCHAR(10));

-- Drop sequence tables
DROP TABLE IF EXISTS ar.receipt_sequences;
DROP TABLE IF EXISTS ar.invoice_sequences;
DROP TABLE IF EXISTS ar.customer_sequences;

-- Drop RLS policies
DROP POLICY IF EXISTS ar_collection_cases_entity_isolation ON ar.collection_cases;
DROP POLICY IF EXISTS ar_dunning_history_entity_isolation ON ar.dunning_history;
DROP POLICY IF EXISTS ar_dunning_level_configs_entity_isolation ON ar.dunning_level_configs;
DROP POLICY IF EXISTS ar_dunning_profiles_entity_isolation ON ar.dunning_profiles;
DROP POLICY IF EXISTS ar_receipt_batches_entity_isolation ON ar.receipt_batches;
DROP POLICY IF EXISTS ar_receipt_applications_entity_isolation ON ar.receipt_applications;
DROP POLICY IF EXISTS ar_receipts_entity_isolation ON ar.receipts;
DROP POLICY IF EXISTS ar_invoice_lines_entity_isolation ON ar.invoice_lines;
DROP POLICY IF EXISTS ar_invoices_entity_isolation ON ar.invoices;
DROP POLICY IF EXISTS ar_customer_contacts_entity_isolation ON ar.customer_contacts;
DROP POLICY IF EXISTS ar_customers_entity_isolation ON ar.customers;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_ar_customer_balance_invoice ON ar.invoices;
DROP TRIGGER IF EXISTS trg_ar_invoice_payment_update ON ar.receipt_applications;
DROP TRIGGER IF EXISTS trg_ar_receipt_applications_update_amounts ON ar.receipt_applications;
DROP TRIGGER IF EXISTS trg_ar_collection_cases_updated_at ON ar.collection_cases;
DROP TRIGGER IF EXISTS trg_ar_dunning_profiles_updated_at ON ar.dunning_profiles;
DROP TRIGGER IF EXISTS trg_ar_receipt_batches_updated_at ON ar.receipt_batches;
DROP TRIGGER IF EXISTS trg_ar_receipts_updated_at ON ar.receipts;
DROP TRIGGER IF EXISTS trg_ar_invoices_updated_at ON ar.invoices;
DROP TRIGGER IF EXISTS trg_ar_customers_updated_at ON ar.customers;

-- Drop functions
DROP FUNCTION IF EXISTS ar.update_customer_balance();
DROP FUNCTION IF EXISTS ar.update_invoice_on_payment();
DROP FUNCTION IF EXISTS ar.recalculate_receipt_amounts();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS ar.collection_cases;
DROP TABLE IF EXISTS ar.dunning_history;
DROP TABLE IF EXISTS ar.dunning_level_configs;
DROP TABLE IF EXISTS ar.dunning_profiles;
DROP TABLE IF EXISTS ar.receipt_batches;
DROP TABLE IF EXISTS ar.receipt_applications;
DROP TABLE IF EXISTS ar.receipts;
DROP TABLE IF EXISTS ar.invoice_lines;
DROP TABLE IF EXISTS ar.invoices;
DROP TABLE IF EXISTS ar.customer_contacts;
DROP TABLE IF EXISTS ar.customers;

-- Drop enum types
DROP TYPE IF EXISTS ar.dunning_action;
DROP TYPE IF EXISTS ar.dunning_level;
DROP TYPE IF EXISTS ar.receipt_method;
DROP TYPE IF EXISTS ar.receipt_status;
DROP TYPE IF EXISTS ar.invoice_type;
DROP TYPE IF EXISTS ar.invoice_status;
DROP TYPE IF EXISTS ar.payment_terms;
DROP TYPE IF EXISTS ar.customer_type;
DROP TYPE IF EXISTS ar.customer_status;

-- Drop schema
DROP SCHEMA IF EXISTS ar;
