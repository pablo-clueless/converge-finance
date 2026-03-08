-- =============================================
-- DROP DOCUMENTS MANAGEMENT TABLES
-- =============================================

-- Drop sequence functions
DROP FUNCTION IF EXISTS docs.generate_document_number(CHAR(26), VARCHAR(10));

-- Drop sequence tables
DROP TABLE IF EXISTS docs.document_sequences;

-- Drop RLS policies
DROP POLICY IF EXISTS docs_document_versions_entity_isolation ON docs.document_versions;
DROP POLICY IF EXISTS docs_attachments_entity_isolation ON docs.attachments;
DROP POLICY IF EXISTS docs_documents_entity_isolation ON docs.documents;
DROP POLICY IF EXISTS docs_retention_policies_entity_isolation ON docs.retention_policies;
DROP POLICY IF EXISTS docs_storage_config_entity_isolation ON docs.storage_config;

-- Drop triggers
DROP TRIGGER IF EXISTS trg_docs_documents_updated_at ON docs.documents;
DROP TRIGGER IF EXISTS trg_docs_retention_policies_updated_at ON docs.retention_policies;
DROP TRIGGER IF EXISTS trg_docs_storage_config_updated_at ON docs.storage_config;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS docs.document_versions;
DROP TABLE IF EXISTS docs.attachments;
DROP TABLE IF EXISTS docs.documents;
DROP TABLE IF EXISTS docs.retention_policies;
DROP TABLE IF EXISTS docs.storage_config;

-- Drop enum types
DROP TYPE IF EXISTS docs.document_status;
DROP TYPE IF EXISTS docs.storage_type;

-- Drop schema
DROP SCHEMA IF EXISTS docs;
