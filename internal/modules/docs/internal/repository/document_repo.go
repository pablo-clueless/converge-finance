package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/docs/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/lib/pq"
)

type DocumentRepository interface {
	Create(ctx context.Context, doc *domain.Document) error
	Update(ctx context.Context, doc *domain.Document) error
	GetByID(ctx context.Context, id common.ID) (*domain.Document, error)
	GetByNumber(ctx context.Context, entityID common.ID, documentNumber string) (*domain.Document, error)
	List(ctx context.Context, filter DocumentFilter) ([]domain.Document, int, error)
	Delete(ctx context.Context, id common.ID) error
	GenerateDocumentNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)
}

type DocumentVersionRepository interface {
	Create(ctx context.Context, version *domain.DocumentVersion) error
	GetByID(ctx context.Context, id common.ID) (*domain.DocumentVersion, error)
	GetByDocumentID(ctx context.Context, documentID common.ID) ([]domain.DocumentVersion, error)
	GetLatestVersion(ctx context.Context, documentID common.ID) (*domain.DocumentVersion, error)
	Delete(ctx context.Context, id common.ID) error
}

type StorageConfigRepository interface {
	Create(ctx context.Context, config *domain.StorageConfig) error
	Update(ctx context.Context, config *domain.StorageConfig) error
	GetByID(ctx context.Context, id common.ID) (*domain.StorageConfig, error)
	GetDefault(ctx context.Context, entityID *common.ID) (*domain.StorageConfig, error)
	List(ctx context.Context, entityID *common.ID) ([]domain.StorageConfig, error)
	Delete(ctx context.Context, id common.ID) error
}

type RetentionPolicyRepository interface {
	Create(ctx context.Context, policy *domain.RetentionPolicy) error
	Update(ctx context.Context, policy *domain.RetentionPolicy) error
	GetByID(ctx context.Context, id common.ID) (*domain.RetentionPolicy, error)
	GetByCode(ctx context.Context, entityID *common.ID, code string) (*domain.RetentionPolicy, error)
	GetDefault(ctx context.Context, entityID *common.ID, documentType string) (*domain.RetentionPolicy, error)
	List(ctx context.Context, entityID *common.ID) ([]domain.RetentionPolicy, error)
	Delete(ctx context.Context, id common.ID) error
}

type DocumentFilter struct {
	EntityID     common.ID
	DocumentType string
	Status       *domain.DocumentStatus
	Tags         []string
	LegalHold    *bool
	DateFrom     *time.Time
	DateTo       *time.Time
	SearchQuery  string
	Limit        int
	Offset       int
}

type PostgresDocumentRepo struct {
	db *database.PostgresDB
}

func NewPostgresDocumentRepo(db *database.PostgresDB) *PostgresDocumentRepo {
	return &PostgresDocumentRepo{db: db}
}

func (r *PostgresDocumentRepo) Create(ctx context.Context, doc *domain.Document) error {
	query := `
		INSERT INTO docs.documents (
			id, entity_id, document_number, file_name, original_name, mime_type,
			file_size, checksum, storage_id, storage_path, document_type, description,
			tags, metadata, retention_policy_id, status, legal_hold, legal_hold_reason,
			archived_at, expires_at, deleted_at, uploaded_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)
	`

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		doc.ID,
		doc.EntityID,
		doc.DocumentNumber,
		doc.FileName,
		doc.OriginalName,
		doc.MimeType,
		doc.FileSize,
		doc.Checksum,
		doc.StorageID,
		doc.StoragePath,
		doc.DocumentType,
		doc.Description,
		pq.Array(doc.Tags),
		metadataJSON,
		doc.RetentionPolicyID,
		doc.Status,
		doc.LegalHold,
		doc.LegalHoldReason,
		doc.ArchivedAt,
		doc.ExpiresAt,
		doc.DeletedAt,
		doc.UploadedBy,
		doc.CreatedAt,
		doc.UpdatedAt,
	)

	return err
}

func (r *PostgresDocumentRepo) Update(ctx context.Context, doc *domain.Document) error {
	query := `
		UPDATE docs.documents SET
			file_name = $2,
			document_type = $3,
			description = $4,
			tags = $5,
			metadata = $6,
			retention_policy_id = $7,
			status = $8,
			legal_hold = $9,
			legal_hold_reason = $10,
			archived_at = $11,
			expires_at = $12,
			deleted_at = $13,
			updated_at = $14
		WHERE id = $1
	`

	metadataJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query,
		doc.ID,
		doc.FileName,
		doc.DocumentType,
		doc.Description,
		pq.Array(doc.Tags),
		metadataJSON,
		doc.RetentionPolicyID,
		doc.Status,
		doc.LegalHold,
		doc.LegalHoldReason,
		doc.ArchivedAt,
		doc.ExpiresAt,
		doc.DeletedAt,
		doc.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrDocumentNotFound
	}

	return nil
}

func (r *PostgresDocumentRepo) GetByID(ctx context.Context, id common.ID) (*domain.Document, error) {
	query := `
		SELECT id, entity_id, document_number, file_name, original_name, mime_type,
			   file_size, checksum, storage_id, storage_path, document_type, description,
			   tags, metadata, retention_policy_id, status, legal_hold, legal_hold_reason,
			   archived_at, expires_at, deleted_at, uploaded_by, created_at, updated_at
		FROM docs.documents
		WHERE id = $1
	`

	return r.scanDocument(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresDocumentRepo) GetByNumber(ctx context.Context, entityID common.ID, documentNumber string) (*domain.Document, error) {
	query := `
		SELECT id, entity_id, document_number, file_name, original_name, mime_type,
			   file_size, checksum, storage_id, storage_path, document_type, description,
			   tags, metadata, retention_policy_id, status, legal_hold, legal_hold_reason,
			   archived_at, expires_at, deleted_at, uploaded_by, created_at, updated_at
		FROM docs.documents
		WHERE entity_id = $1 AND document_number = $2
	`

	return r.scanDocument(r.db.QueryRowContext(ctx, query, entityID, documentNumber))
}

func (r *PostgresDocumentRepo) List(ctx context.Context, filter DocumentFilter) ([]domain.Document, int, error) {
	baseQuery := `FROM docs.documents WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if filter.DocumentType != "" {
		baseQuery += fmt.Sprintf(` AND document_type = $%d`, argIdx)
		args = append(args, filter.DocumentType)
		argIdx++
	}
	if filter.Status != nil {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if len(filter.Tags) > 0 {
		baseQuery += fmt.Sprintf(` AND tags && $%d`, argIdx)
		args = append(args, pq.Array(filter.Tags))
		argIdx++
	}
	if filter.LegalHold != nil {
		baseQuery += fmt.Sprintf(` AND legal_hold = $%d`, argIdx)
		args = append(args, *filter.LegalHold)
		argIdx++
	}
	if filter.DateFrom != nil {
		baseQuery += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		baseQuery += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}
	if filter.SearchQuery != "" {
		baseQuery += fmt.Sprintf(` AND (
			document_number ILIKE $%d OR
			file_name ILIKE $%d OR
			original_name ILIKE $%d OR
			description ILIKE $%d
		)`, argIdx, argIdx, argIdx, argIdx)
		searchPattern := "%" + filter.SearchQuery + "%"
		args = append(args, searchPattern)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, document_number, file_name, original_name, mime_type,
			   file_size, checksum, storage_id, storage_path, document_type, description,
			   tags, metadata, retention_policy_id, status, legal_hold, legal_hold_reason,
			   archived_at, expires_at, deleted_at, uploaded_by, created_at, updated_at
		` + baseQuery + ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		dataQuery += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		dataQuery += fmt.Sprintf(` OFFSET $%d`, argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var documents []domain.Document
	for rows.Next() {
		doc, err := r.scanDocumentRow(rows)
		if err != nil {
			return nil, 0, err
		}
		documents = append(documents, *doc)
	}

	return documents, total, rows.Err()
}

func (r *PostgresDocumentRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM docs.documents WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrDocumentNotFound
	}

	return nil
}

func (r *PostgresDocumentRepo) GenerateDocumentNumber(ctx context.Context, entityID common.ID, prefix string) (string, error) {
	query := `SELECT docs.generate_document_number($1, $2)`
	var documentNumber string
	err := r.db.QueryRowContext(ctx, query, entityID, prefix).Scan(&documentNumber)
	return documentNumber, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *PostgresDocumentRepo) scanDocument(row rowScanner) (*domain.Document, error) {
	var doc domain.Document
	var tags []string
	var metadataJSON []byte

	err := row.Scan(
		&doc.ID,
		&doc.EntityID,
		&doc.DocumentNumber,
		&doc.FileName,
		&doc.OriginalName,
		&doc.MimeType,
		&doc.FileSize,
		&doc.Checksum,
		&doc.StorageID,
		&doc.StoragePath,
		&doc.DocumentType,
		&doc.Description,
		pq.Array(&tags),
		&metadataJSON,
		&doc.RetentionPolicyID,
		&doc.Status,
		&doc.LegalHold,
		&doc.LegalHoldReason,
		&doc.ArchivedAt,
		&doc.ExpiresAt,
		&doc.DeletedAt,
		&doc.UploadedBy,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, err
	}

	doc.Tags = tags
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, err
		}
	} else {
		doc.Metadata = make(map[string]interface{})
	}

	return &doc, nil
}

func (r *PostgresDocumentRepo) scanDocumentRow(rows *sql.Rows) (*domain.Document, error) {
	var doc domain.Document
	var tags []string
	var metadataJSON []byte

	err := rows.Scan(
		&doc.ID,
		&doc.EntityID,
		&doc.DocumentNumber,
		&doc.FileName,
		&doc.OriginalName,
		&doc.MimeType,
		&doc.FileSize,
		&doc.Checksum,
		&doc.StorageID,
		&doc.StoragePath,
		&doc.DocumentType,
		&doc.Description,
		pq.Array(&tags),
		&metadataJSON,
		&doc.RetentionPolicyID,
		&doc.Status,
		&doc.LegalHold,
		&doc.LegalHoldReason,
		&doc.ArchivedAt,
		&doc.ExpiresAt,
		&doc.DeletedAt,
		&doc.UploadedBy,
		&doc.CreatedAt,
		&doc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	doc.Tags = tags
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &doc.Metadata); err != nil {
			return nil, err
		}
	} else {
		doc.Metadata = make(map[string]interface{})
	}

	return &doc, nil
}

type PostgresDocumentVersionRepo struct {
	db *database.PostgresDB
}

func NewPostgresDocumentVersionRepo(db *database.PostgresDB) *PostgresDocumentVersionRepo {
	return &PostgresDocumentVersionRepo{db: db}
}

func (r *PostgresDocumentVersionRepo) Create(ctx context.Context, version *domain.DocumentVersion) error {
	query := `
		INSERT INTO docs.document_versions (
			id, document_id, version_number, file_name, file_size,
			checksum, storage_path, change_notes, created_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		version.ID,
		version.DocumentID,
		version.VersionNumber,
		version.FileName,
		version.FileSize,
		version.Checksum,
		version.StoragePath,
		version.ChangeNotes,
		version.CreatedBy,
		version.CreatedAt,
	)

	return err
}

func (r *PostgresDocumentVersionRepo) GetByID(ctx context.Context, id common.ID) (*domain.DocumentVersion, error) {
	query := `
		SELECT id, document_id, version_number, file_name, file_size,
			   checksum, storage_path, change_notes, created_by, created_at
		FROM docs.document_versions
		WHERE id = $1
	`

	return r.scanVersion(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresDocumentVersionRepo) GetByDocumentID(ctx context.Context, documentID common.ID) ([]domain.DocumentVersion, error) {
	query := `
		SELECT id, document_id, version_number, file_name, file_size,
			   checksum, storage_path, change_notes, created_by, created_at
		FROM docs.document_versions
		WHERE document_id = $1
		ORDER BY version_number DESC
	`

	rows, err := r.db.QueryContext(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []domain.DocumentVersion
	for rows.Next() {
		version, err := r.scanVersionRow(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, *version)
	}

	return versions, rows.Err()
}

func (r *PostgresDocumentVersionRepo) GetLatestVersion(ctx context.Context, documentID common.ID) (*domain.DocumentVersion, error) {
	query := `
		SELECT id, document_id, version_number, file_name, file_size,
			   checksum, storage_path, change_notes, created_by, created_at
		FROM docs.document_versions
		WHERE document_id = $1
		ORDER BY version_number DESC
		LIMIT 1
	`

	return r.scanVersion(r.db.QueryRowContext(ctx, query, documentID))
}

func (r *PostgresDocumentVersionRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM docs.document_versions WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrVersionNotFound
	}

	return nil
}

func (r *PostgresDocumentVersionRepo) scanVersion(row rowScanner) (*domain.DocumentVersion, error) {
	var version domain.DocumentVersion

	err := row.Scan(
		&version.ID,
		&version.DocumentID,
		&version.VersionNumber,
		&version.FileName,
		&version.FileSize,
		&version.Checksum,
		&version.StoragePath,
		&version.ChangeNotes,
		&version.CreatedBy,
		&version.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrVersionNotFound
	}
	if err != nil {
		return nil, err
	}

	return &version, nil
}

func (r *PostgresDocumentVersionRepo) scanVersionRow(rows *sql.Rows) (*domain.DocumentVersion, error) {
	var version domain.DocumentVersion

	err := rows.Scan(
		&version.ID,
		&version.DocumentID,
		&version.VersionNumber,
		&version.FileName,
		&version.FileSize,
		&version.Checksum,
		&version.StoragePath,
		&version.ChangeNotes,
		&version.CreatedBy,
		&version.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

type PostgresStorageConfigRepo struct {
	db *database.PostgresDB
}

func NewPostgresStorageConfigRepo(db *database.PostgresDB) *PostgresStorageConfigRepo {
	return &PostgresStorageConfigRepo{db: db}
}

func (r *PostgresStorageConfigRepo) Create(ctx context.Context, config *domain.StorageConfig) error {
	query := `
		INSERT INTO docs.storage_config (
			id, entity_id, storage_type, configuration, is_default, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	configJSON, err := json.Marshal(config.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query,
		config.ID,
		config.EntityID,
		config.StorageType,
		configJSON,
		config.IsDefault,
		config.IsActive,
		config.CreatedAt,
		config.UpdatedAt,
	)

	return err
}

func (r *PostgresStorageConfigRepo) Update(ctx context.Context, config *domain.StorageConfig) error {
	query := `
		UPDATE docs.storage_config SET
			storage_type = $2,
			configuration = $3,
			is_default = $4,
			is_active = $5,
			updated_at = $6
		WHERE id = $1
	`

	configJSON, err := json.Marshal(config.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	result, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.StorageType,
		configJSON,
		config.IsDefault,
		config.IsActive,
		config.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrStorageConfigNotFound
	}

	return nil
}

func (r *PostgresStorageConfigRepo) GetByID(ctx context.Context, id common.ID) (*domain.StorageConfig, error) {
	query := `
		SELECT id, entity_id, storage_type, configuration, is_default, is_active, created_at, updated_at
		FROM docs.storage_config
		WHERE id = $1
	`

	return r.scanConfig(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresStorageConfigRepo) GetDefault(ctx context.Context, entityID *common.ID) (*domain.StorageConfig, error) {
	query := `
		SELECT id, entity_id, storage_type, configuration, is_default, is_active, created_at, updated_at
		FROM docs.storage_config
		WHERE (entity_id = $1 OR entity_id IS NULL) AND is_default = true AND is_active = true
		ORDER BY entity_id NULLS LAST
		LIMIT 1
	`

	return r.scanConfig(r.db.QueryRowContext(ctx, query, entityID))
}

func (r *PostgresStorageConfigRepo) List(ctx context.Context, entityID *common.ID) ([]domain.StorageConfig, error) {
	query := `
		SELECT id, entity_id, storage_type, configuration, is_default, is_active, created_at, updated_at
		FROM docs.storage_config
		WHERE entity_id = $1 OR entity_id IS NULL
		ORDER BY entity_id NULLS LAST, created_at
	`

	rows, err := r.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []domain.StorageConfig
	for rows.Next() {
		config, err := r.scanConfigRow(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *config)
	}

	return configs, rows.Err()
}

func (r *PostgresStorageConfigRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM docs.storage_config WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrStorageConfigNotFound
	}

	return nil
}

func (r *PostgresStorageConfigRepo) scanConfig(row rowScanner) (*domain.StorageConfig, error) {
	var config domain.StorageConfig
	var configJSON []byte

	err := row.Scan(
		&config.ID,
		&config.EntityID,
		&config.StorageType,
		&configJSON,
		&config.IsDefault,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrStorageConfigNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &config.Configuration); err != nil {
			return nil, err
		}
	} else {
		config.Configuration = make(map[string]interface{})
	}

	return &config, nil
}

func (r *PostgresStorageConfigRepo) scanConfigRow(rows *sql.Rows) (*domain.StorageConfig, error) {
	var config domain.StorageConfig
	var configJSON []byte

	err := rows.Scan(
		&config.ID,
		&config.EntityID,
		&config.StorageType,
		&configJSON,
		&config.IsDefault,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &config.Configuration); err != nil {
			return nil, err
		}
	} else {
		config.Configuration = make(map[string]interface{})
	}

	return &config, nil
}

type PostgresRetentionPolicyRepo struct {
	db *database.PostgresDB
}

func NewPostgresRetentionPolicyRepo(db *database.PostgresDB) *PostgresRetentionPolicyRepo {
	return &PostgresRetentionPolicyRepo{db: db}
}

func (r *PostgresRetentionPolicyRepo) Create(ctx context.Context, policy *domain.RetentionPolicy) error {
	query := `
		INSERT INTO docs.retention_policies (
			id, entity_id, policy_code, policy_name, document_type, retention_days,
			archive_after_days, delete_after_archive_days, legal_hold_override,
			is_default, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.ExecContext(ctx, query,
		policy.ID,
		policy.EntityID,
		policy.PolicyCode,
		policy.PolicyName,
		policy.DocumentType,
		policy.RetentionDays,
		policy.ArchiveAfterDays,
		policy.DeleteAfterArchiveDays,
		policy.LegalHoldOverride,
		policy.IsDefault,
		policy.IsActive,
		policy.CreatedAt,
		policy.UpdatedAt,
	)

	return err
}

func (r *PostgresRetentionPolicyRepo) Update(ctx context.Context, policy *domain.RetentionPolicy) error {
	query := `
		UPDATE docs.retention_policies SET
			policy_name = $2,
			document_type = $3,
			retention_days = $4,
			archive_after_days = $5,
			delete_after_archive_days = $6,
			legal_hold_override = $7,
			is_default = $8,
			is_active = $9,
			updated_at = $10
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		policy.ID,
		policy.PolicyName,
		policy.DocumentType,
		policy.RetentionDays,
		policy.ArchiveAfterDays,
		policy.DeleteAfterArchiveDays,
		policy.LegalHoldOverride,
		policy.IsDefault,
		policy.IsActive,
		policy.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrRetentionPolicyNotFound
	}

	return nil
}

func (r *PostgresRetentionPolicyRepo) GetByID(ctx context.Context, id common.ID) (*domain.RetentionPolicy, error) {
	query := `
		SELECT id, entity_id, policy_code, policy_name, document_type, retention_days,
			   archive_after_days, delete_after_archive_days, legal_hold_override,
			   is_default, is_active, created_at, updated_at
		FROM docs.retention_policies
		WHERE id = $1
	`

	return r.scanPolicy(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresRetentionPolicyRepo) GetByCode(ctx context.Context, entityID *common.ID, code string) (*domain.RetentionPolicy, error) {
	query := `
		SELECT id, entity_id, policy_code, policy_name, document_type, retention_days,
			   archive_after_days, delete_after_archive_days, legal_hold_override,
			   is_default, is_active, created_at, updated_at
		FROM docs.retention_policies
		WHERE (entity_id = $1 OR entity_id IS NULL) AND policy_code = $2
		ORDER BY entity_id NULLS LAST
		LIMIT 1
	`

	return r.scanPolicy(r.db.QueryRowContext(ctx, query, entityID, code))
}

func (r *PostgresRetentionPolicyRepo) GetDefault(ctx context.Context, entityID *common.ID, documentType string) (*domain.RetentionPolicy, error) {
	query := `
		SELECT id, entity_id, policy_code, policy_name, document_type, retention_days,
			   archive_after_days, delete_after_archive_days, legal_hold_override,
			   is_default, is_active, created_at, updated_at
		FROM docs.retention_policies
		WHERE (entity_id = $1 OR entity_id IS NULL) AND document_type = $2 AND is_default = true AND is_active = true
		ORDER BY entity_id NULLS LAST
		LIMIT 1
	`

	return r.scanPolicy(r.db.QueryRowContext(ctx, query, entityID, documentType))
}

func (r *PostgresRetentionPolicyRepo) List(ctx context.Context, entityID *common.ID) ([]domain.RetentionPolicy, error) {
	query := `
		SELECT id, entity_id, policy_code, policy_name, document_type, retention_days,
			   archive_after_days, delete_after_archive_days, legal_hold_override,
			   is_default, is_active, created_at, updated_at
		FROM docs.retention_policies
		WHERE entity_id = $1 OR entity_id IS NULL
		ORDER BY entity_id NULLS LAST, document_type, policy_name
	`

	rows, err := r.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []domain.RetentionPolicy
	for rows.Next() {
		policy, err := r.scanPolicyRow(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, *policy)
	}

	return policies, rows.Err()
}

func (r *PostgresRetentionPolicyRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM docs.retention_policies WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrRetentionPolicyNotFound
	}

	return nil
}

func (r *PostgresRetentionPolicyRepo) scanPolicy(row rowScanner) (*domain.RetentionPolicy, error) {
	var policy domain.RetentionPolicy

	err := row.Scan(
		&policy.ID,
		&policy.EntityID,
		&policy.PolicyCode,
		&policy.PolicyName,
		&policy.DocumentType,
		&policy.RetentionDays,
		&policy.ArchiveAfterDays,
		&policy.DeleteAfterArchiveDays,
		&policy.LegalHoldOverride,
		&policy.IsDefault,
		&policy.IsActive,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrRetentionPolicyNotFound
	}
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (r *PostgresRetentionPolicyRepo) scanPolicyRow(rows *sql.Rows) (*domain.RetentionPolicy, error) {
	var policy domain.RetentionPolicy

	err := rows.Scan(
		&policy.ID,
		&policy.EntityID,
		&policy.PolicyCode,
		&policy.PolicyName,
		&policy.DocumentType,
		&policy.RetentionDays,
		&policy.ArchiveAfterDays,
		&policy.DeleteAfterArchiveDays,
		&policy.LegalHoldOverride,
		&policy.IsDefault,
		&policy.IsActive,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}
