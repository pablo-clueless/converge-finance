package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/docs/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/lib/pq"
)

type AttachmentRepository interface {
	Create(ctx context.Context, attachment *domain.Attachment) error
	GetByID(ctx context.Context, id common.ID) (*domain.Attachment, error)
	GetByDocumentAndReference(ctx context.Context, documentID common.ID, refType string, refID common.ID) (*domain.Attachment, error)
	ListByDocument(ctx context.Context, documentID common.ID) ([]domain.Attachment, error)
	ListByReference(ctx context.Context, refType string, refID common.ID) ([]domain.Attachment, error)
	ListByReferenceWithDocuments(ctx context.Context, refType string, refID common.ID) ([]domain.AttachmentWithDocument, error)
	Delete(ctx context.Context, id common.ID) error
	DeleteByDocumentAndReference(ctx context.Context, documentID common.ID, refType string, refID common.ID) error
	SetPrimary(ctx context.Context, id common.ID, isPrimary bool) error
}

type PostgresAttachmentRepo struct {
	db *database.PostgresDB
}

func NewPostgresAttachmentRepo(db *database.PostgresDB) *PostgresAttachmentRepo {
	return &PostgresAttachmentRepo{db: db}
}

func (r *PostgresAttachmentRepo) Create(ctx context.Context, attachment *domain.Attachment) error {
	query := `
		INSERT INTO docs.attachments (
			id, document_id, entity_id, reference_type, reference_id, is_primary, attached_by, attached_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		attachment.ID,
		attachment.DocumentID,
		attachment.EntityID,
		attachment.ReferenceType,
		attachment.ReferenceID,
		attachment.IsPrimary,
		attachment.AttachedBy,
		attachment.AttachedAt,
	)

	return err
}

func (r *PostgresAttachmentRepo) GetByID(ctx context.Context, id common.ID) (*domain.Attachment, error) {
	query := `
		SELECT id, document_id, entity_id, reference_type, reference_id, is_primary, attached_by, attached_at
		FROM docs.attachments
		WHERE id = $1
	`

	return r.scanAttachment(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresAttachmentRepo) GetByDocumentAndReference(ctx context.Context, documentID common.ID, refType string, refID common.ID) (*domain.Attachment, error) {
	query := `
		SELECT id, document_id, entity_id, reference_type, reference_id, is_primary, attached_by, attached_at
		FROM docs.attachments
		WHERE document_id = $1 AND reference_type = $2 AND reference_id = $3
	`

	return r.scanAttachment(r.db.QueryRowContext(ctx, query, documentID, refType, refID))
}

func (r *PostgresAttachmentRepo) ListByDocument(ctx context.Context, documentID common.ID) ([]domain.Attachment, error) {
	query := `
		SELECT id, document_id, entity_id, reference_type, reference_id, is_primary, attached_by, attached_at
		FROM docs.attachments
		WHERE document_id = $1
		ORDER BY attached_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			panic(err)
		}
	}()

	var attachments []domain.Attachment
	for rows.Next() {
		attachment, err := r.scanAttachmentRow(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, *attachment)
	}

	return attachments, rows.Err()
}

func (r *PostgresAttachmentRepo) ListByReference(ctx context.Context, refType string, refID common.ID) ([]domain.Attachment, error) {
	query := `
		SELECT id, document_id, entity_id, reference_type, reference_id, is_primary, attached_by, attached_at
		FROM docs.attachments
		WHERE reference_type = $1 AND reference_id = $2
		ORDER BY is_primary DESC, attached_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, refType, refID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			panic(err)
		}
	}()

	var attachments []domain.Attachment
	for rows.Next() {
		attachment, err := r.scanAttachmentRow(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, *attachment)
	}

	return attachments, rows.Err()
}

func (r *PostgresAttachmentRepo) ListByReferenceWithDocuments(ctx context.Context, refType string, refID common.ID) ([]domain.AttachmentWithDocument, error) {
	query := `
		SELECT
			a.id, a.document_id, a.entity_id, a.reference_type, a.reference_id, a.is_primary, a.attached_by, a.attached_at,
			d.id, d.entity_id, d.document_number, d.file_name, d.original_name, d.mime_type,
			d.file_size, d.checksum, d.storage_id, d.storage_path, d.document_type, d.description,
			d.tags, d.metadata, d.retention_policy_id, d.status, d.legal_hold, d.legal_hold_reason,
			d.archived_at, d.expires_at, d.deleted_at, d.uploaded_by, d.created_at, d.updated_at
		FROM docs.attachments a
		JOIN docs.documents d ON a.document_id = d.id
		WHERE a.reference_type = $1 AND a.reference_id = $2
		ORDER BY a.is_primary DESC, a.attached_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, refType, refID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var results []domain.AttachmentWithDocument
	for rows.Next() {
		result, err := r.scanAttachmentWithDocumentRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}

	return results, rows.Err()
}

func (r *PostgresAttachmentRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM docs.attachments WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAttachmentNotFound
	}

	return nil
}

func (r *PostgresAttachmentRepo) DeleteByDocumentAndReference(ctx context.Context, documentID common.ID, refType string, refID common.ID) error {
	query := `DELETE FROM docs.attachments WHERE document_id = $1 AND reference_type = $2 AND reference_id = $3`
	result, err := r.db.ExecContext(ctx, query, documentID, refType, refID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAttachmentNotFound
	}

	return nil
}

func (r *PostgresAttachmentRepo) SetPrimary(ctx context.Context, id common.ID, isPrimary bool) error {
	attachment, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if isPrimary {
		clearQuery := `UPDATE docs.attachments SET is_primary = false WHERE reference_type = $1 AND reference_id = $2 AND id != $3`
		if _, err := r.db.ExecContext(ctx, clearQuery, attachment.ReferenceType, attachment.ReferenceID, id); err != nil {
			return fmt.Errorf("failed to clear existing primary: %w", err)
		}
	}

	updateQuery := `UPDATE docs.attachments SET is_primary = $1 WHERE id = $2`
	result, err := r.db.ExecContext(ctx, updateQuery, isPrimary, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAttachmentNotFound
	}

	return nil
}

func (r *PostgresAttachmentRepo) scanAttachment(row rowScanner) (*domain.Attachment, error) {
	var attachment domain.Attachment

	err := row.Scan(
		&attachment.ID,
		&attachment.DocumentID,
		&attachment.EntityID,
		&attachment.ReferenceType,
		&attachment.ReferenceID,
		&attachment.IsPrimary,
		&attachment.AttachedBy,
		&attachment.AttachedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAttachmentNotFound
	}
	if err != nil {
		return nil, err
	}

	return &attachment, nil
}

func (r *PostgresAttachmentRepo) scanAttachmentRow(rows *sql.Rows) (*domain.Attachment, error) {
	var attachment domain.Attachment

	err := rows.Scan(
		&attachment.ID,
		&attachment.DocumentID,
		&attachment.EntityID,
		&attachment.ReferenceType,
		&attachment.ReferenceID,
		&attachment.IsPrimary,
		&attachment.AttachedBy,
		&attachment.AttachedAt,
	)
	if err != nil {
		return nil, err
	}

	return &attachment, nil
}

func (r *PostgresAttachmentRepo) scanAttachmentWithDocumentRow(rows *sql.Rows) (*domain.AttachmentWithDocument, error) {
	var result domain.AttachmentWithDocument
	var doc domain.Document
	var tags []string
	var metadataJSON []byte

	err := rows.Scan(
		&result.ID,
		&result.DocumentID,
		&result.EntityID,
		&result.ReferenceType,
		&result.ReferenceID,
		&result.IsPrimary,
		&result.AttachedBy,
		&result.AttachedAt,
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
		_ = json.Unmarshal(metadataJSON, &doc.Metadata)
	} else {
		doc.Metadata = make(map[string]interface{})
	}

	result.Document = &doc
	return &result, nil
}
