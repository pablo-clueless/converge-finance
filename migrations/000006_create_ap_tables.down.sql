DROP FUNCTION IF EXISTS ap.get_next_payment_number(CHAR(26), VARCHAR(10));
DROP FUNCTION IF EXISTS ap.get_next_invoice_number(CHAR(26), VARCHAR(10));
DROP FUNCTION IF EXISTS ap.get_next_vendor_number(CHAR(26), VARCHAR(10));

DROP TABLE IF EXISTS ap.payment_sequences;
DROP TABLE IF EXISTS ap.invoice_sequences;
DROP TABLE IF EXISTS ap.vendor_sequences;

DROP POLICY IF EXISTS ap_payment_batches_entity_isolation ON ap.payment_batches;
DROP POLICY IF EXISTS ap_payment_allocations_entity_isolation ON ap.payment_allocations;
DROP POLICY IF EXISTS ap_payments_entity_isolation ON ap.payments;
DROP POLICY IF EXISTS ap_invoice_lines_entity_isolation ON ap.invoice_lines;
DROP POLICY IF EXISTS ap_invoices_entity_isolation ON ap.invoices;
DROP POLICY IF EXISTS ap_vendors_entity_isolation ON ap.vendors;

DROP TRIGGER IF EXISTS trg_ap_vendor_balance_invoice ON ap.invoices;
DROP TRIGGER IF EXISTS trg_ap_invoice_payment_update ON ap.payment_allocations;
DROP TRIGGER IF EXISTS trg_ap_payment_allocations_update_amounts ON ap.payment_allocations;
DROP TRIGGER IF EXISTS trg_ap_payment_batches_updated_at ON ap.payment_batches;
DROP TRIGGER IF EXISTS trg_ap_payments_updated_at ON ap.payments;
DROP TRIGGER IF EXISTS trg_ap_invoices_updated_at ON ap.invoices;
DROP TRIGGER IF EXISTS trg_ap_vendors_updated_at ON ap.vendors;

DROP FUNCTION IF EXISTS ap.update_vendor_balance();
DROP FUNCTION IF EXISTS ap.update_invoice_on_payment();
DROP FUNCTION IF EXISTS ap.recalculate_payment_amounts();

DROP TABLE IF EXISTS ap.payment_batches;
DROP TABLE IF EXISTS ap.payment_allocations;
DROP TABLE IF EXISTS ap.payments;
DROP TABLE IF EXISTS ap.invoice_lines;
DROP TABLE IF EXISTS ap.invoices;
DROP TABLE IF EXISTS ap.vendors;

DROP TYPE IF EXISTS ap.payment_type;
DROP TYPE IF EXISTS ap.payment_status;
DROP TYPE IF EXISTS ap.invoice_status;
DROP TYPE IF EXISTS ap.payment_method;
DROP TYPE IF EXISTS ap.payment_terms;
DROP TYPE IF EXISTS ap.vendor_status;

DROP SCHEMA IF EXISTS ap;
