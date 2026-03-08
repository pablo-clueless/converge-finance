-- =============================================
-- DOCUMENTS MANAGEMENT TABLES
-- =============================================

-- Create docs schema
CREATE SCHEMA IF NOT EXISTS docs;

-- =============================================
-- ENUM TYPES
-- =============================================

-- Storage Types
CREATE TYPE docs.storage_type AS ENUM ('local', 's3', 'azure_blob', 'gcs');

-- Document Status
CREATE TYPE docs.document_status AS ENUM ('active', 'archived', 'pending_deletion', 'deleted');

-- =============================================
-- STORAGE CONFIGURATION
-- =============================================

CREATE TABLE docs.storage_config (
    id              CHAR(26) PRIMARY KEY,
    entity_id       CHAR(26) REFERENCES entities(id),  -- NULL for global config
    storage_type    docs.storage_type NOT NULL,
    configuration   JSONB NOT NULL DEFAULT '{}',  -- bucket, region, endpoint, credentials_ref
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_docs_storage_config_entity ON docs.storage_config(entity_id);
CREATE INDEX idx_docs_storage_config_type ON docs.storage_config(storage_type);
CREATE INDEX idx_docs_storage_config_default ON docs.storage_config(entity_id, is_default) WHERE is_default = TRUE;
CREATE INDEX idx_docs_storage_config_active ON docs.storage_config(entity_id, is_active) WHERE is_active = TRUE;

-- =============================================
-- RETENTION POLICIES
-- =============================================

CREATE TABLE docs.retention_policies (
    id                          CHAR(26) PRIMARY KEY,
    entity_id                   CHAR(26) REFERENCES entities(id),  -- NULL for global policy
    policy_code                 VARCHAR(50) NOT NULL,
    policy_name                 VARCHAR(255) NOT NULL,
    document_type               VARCHAR(50) NOT NULL,  -- invoice, receipt, contract, journal, etc.
    retention_days              INTEGER NOT NULL CHECK (retention_days >= 0),
    archive_after_days          INTEGER CHECK (archive_after_days >= 0),
    delete_after_archive_days   INTEGER CHECK (delete_after_archive_days >= 0),
    legal_hold_override         BOOLEAN NOT NULL DEFAULT TRUE,
    is_default                  BOOLEAN NOT NULL DEFAULT FALSE,
    is_active                   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT docs_retention_policies_uk UNIQUE (entity_id, policy_code)
);

CREATE INDEX idx_docs_retention_policies_entity ON docs.retention_policies(entity_id);
CREATE INDEX idx_docs_retention_policies_doc_type ON docs.retention_policies(document_type);
CREATE INDEX idx_docs_retention_policies_default ON docs.retention_policies(entity_id, document_type, is_default) WHERE is_default = TRUE;
CREATE INDEX idx_docs_retention_policies_active ON docs.retention_policies(entity_id, is_active) WHERE is_active = TRUE;

-- =============================================
-- DOCUMENTS
-- =============================================

CREATE TABLE docs.documents (
    id                  CHAR(26) PRIMARY KEY,
    entity_id           CHAR(26) NOT NULL REFERENCES entities(id),
    document_number     VARCHAR(50) NOT NULL,
    file_name           VARCHAR(255) NOT NULL,
    original_name       VARCHAR(255) NOT NULL,
    mime_type           VARCHAR(100) NOT NULL,
    file_size           BIGINT NOT NULL CHECK (file_size >= 0),
    checksum            VARCHAR(64) NOT NULL,  -- SHA-256 hash

    -- Storage reference
    storage_id          CHAR(26) NOT NULL REFERENCES docs.storage_config(id),
    storage_path        VARCHAR(1000) NOT NULL,

    -- Classification
    document_type       VARCHAR(50) NOT NULL,
    description         TEXT,
    tags                TEXT[] NOT NULL DEFAULT '{}',
    metadata            JSONB NOT NULL DEFAULT '{}',

    -- Retention
    retention_policy_id CHAR(26) REFERENCES docs.retention_policies(id),
    status              docs.document_status NOT NULL DEFAULT 'active',
    legal_hold          BOOLEAN NOT NULL DEFAULT FALSE,
    legal_hold_reason   TEXT,

    -- Lifecycle dates
    archived_at         TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ,
    deleted_at          TIMESTAMPTZ,

    -- Audit
    uploaded_by         CHAR(26) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT docs_documents_uk_entity_number UNIQUE (entity_id, document_number)
);

CREATE INDEX idx_docs_documents_entity ON docs.documents(entity_id);
CREATE INDEX idx_docs_documents_storage ON docs.documents(storage_id);
CREATE INDEX idx_docs_documents_type ON docs.documents(entity_id, document_type);
CREATE INDEX idx_docs_documents_status ON docs.documents(entity_id, status);
CREATE INDEX idx_docs_documents_legal_hold ON docs.documents(entity_id, legal_hold) WHERE legal_hold = TRUE;
CREATE INDEX idx_docs_documents_retention ON docs.documents(retention_policy_id);
CREATE INDEX idx_docs_documents_expires ON docs.documents(expires_at) WHERE expires_at IS NOT NULL AND status = 'active';
CREATE INDEX idx_docs_documents_archived ON docs.documents(archived_at) WHERE status = 'archived';
CREATE INDEX idx_docs_documents_tags ON docs.documents USING gin(tags);
CREATE INDEX idx_docs_documents_metadata ON docs.documents USING gin(metadata);
CREATE INDEX idx_docs_documents_search ON docs.documents USING gin(
    to_tsvector('english', COALESCE(document_number, '') || ' ' || COALESCE(file_name, '') || ' ' || COALESCE(original_name, '') || ' ' || COALESCE(description, ''))
);

-- =============================================
-- ATTACHMENTS
-- =============================================

CREATE TABLE docs.attachments (
    id              CHAR(26) PRIMARY KEY,
    document_id     CHAR(26) NOT NULL REFERENCES docs.documents(id) ON DELETE CASCADE,
    entity_id       CHAR(26) NOT NULL REFERENCES entities(id),
    reference_type  VARCHAR(50) NOT NULL,  -- ap_invoice, ar_invoice, journal_entry, etc.
    reference_id    CHAR(26) NOT NULL,
    is_primary      BOOLEAN NOT NULL DEFAULT FALSE,
    attached_by     CHAR(26) NOT NULL,
    attached_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT docs_attachments_uk UNIQUE (document_id, reference_type, reference_id)
);

CREATE INDEX idx_docs_attachments_document ON docs.attachments(document_id);
CREATE INDEX idx_docs_attachments_entity ON docs.attachments(entity_id);
CREATE INDEX idx_docs_attachments_reference ON docs.attachments(reference_type, reference_id);
CREATE INDEX idx_docs_attachments_primary ON docs.attachments(reference_type, reference_id, is_primary) WHERE is_primary = TRUE;

-- =============================================
-- DOCUMENT VERSIONS
-- =============================================

CREATE TABLE docs.document_versions (
    id              CHAR(26) PRIMARY KEY,
    document_id     CHAR(26) NOT NULL REFERENCES docs.documents(id) ON DELETE CASCADE,
    version_number  INTEGER NOT NULL CHECK (version_number >= 1),
    file_name       VARCHAR(255) NOT NULL,
    file_size       BIGINT NOT NULL CHECK (file_size >= 0),
    checksum        VARCHAR(64) NOT NULL,  -- SHA-256 hash
    storage_path    VARCHAR(1000) NOT NULL,
    change_notes    TEXT,
    created_by      CHAR(26) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT docs_document_versions_uk UNIQUE (document_id, version_number)
);

CREATE INDEX idx_docs_document_versions_document ON docs.document_versions(document_id);
CREATE INDEX idx_docs_document_versions_created ON docs.document_versions(document_id, created_at DESC);

-- =============================================
-- TRIGGERS
-- =============================================

-- Updated_at triggers
CREATE TRIGGER trg_docs_storage_config_updated_at
    BEFORE UPDATE ON docs.storage_config
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_docs_retention_policies_updated_at
    BEFORE UPDATE ON docs.retention_policies
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_docs_documents_updated_at
    BEFORE UPDATE ON docs.documents
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- =============================================
-- FUNCTIONS
-- =============================================

-- Document number sequence per entity
CREATE TABLE docs.document_sequences (
    entity_id       CHAR(26) PRIMARY KEY REFERENCES entities(id),
    last_number     INTEGER NOT NULL DEFAULT 0
);

CREATE OR REPLACE FUNCTION docs.generate_document_number(p_entity_id CHAR(26), p_prefix VARCHAR(10) DEFAULT 'DOC')
RETURNS VARCHAR(50) AS $$
DECLARE
    next_num INTEGER;
BEGIN
    INSERT INTO docs.document_sequences (entity_id, last_number)
    VALUES (p_entity_id, 1)
    ON CONFLICT (entity_id)
    DO UPDATE SET last_number = docs.document_sequences.last_number + 1
    RETURNING last_number INTO next_num;

    RETURN p_prefix || LPAD(next_num::TEXT, 10, '0');
END;
$$ LANGUAGE plpgsql;

-- =============================================
-- ROW LEVEL SECURITY
-- =============================================

ALTER TABLE docs.storage_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs.retention_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs.documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs.attachments ENABLE ROW LEVEL SECURITY;
ALTER TABLE docs.document_versions ENABLE ROW LEVEL SECURITY;

-- RLS Policies

-- Storage config: allow access if entity matches OR if entity_id is NULL (global config)
CREATE POLICY docs_storage_config_entity_isolation ON docs.storage_config
    USING (entity_id IS NULL OR entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Retention policies: allow access if entity matches OR if entity_id is NULL (global policy)
CREATE POLICY docs_retention_policies_entity_isolation ON docs.retention_policies
    USING (entity_id IS NULL OR entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Documents: standard entity isolation
CREATE POLICY docs_documents_entity_isolation ON docs.documents
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Attachments: standard entity isolation
CREATE POLICY docs_attachments_entity_isolation ON docs.attachments
    USING (entity_id = current_setting('app.current_entity_id', true)::CHAR(26));

-- Document versions: inherit from parent document
CREATE POLICY docs_document_versions_entity_isolation ON docs.document_versions
    USING (EXISTS (
        SELECT 1 FROM docs.documents d
        WHERE d.id = document_id
        AND d.entity_id = current_setting('app.current_entity_id', true)::CHAR(26)
    ));
