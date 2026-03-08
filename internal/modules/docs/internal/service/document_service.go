package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/docs/internal/domain"
	"converge-finance.com/m/internal/modules/docs/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"go.uber.org/zap"
)

type DocumentService struct {
	documentRepo   repository.DocumentRepository
	versionRepo    repository.DocumentVersionRepository
	attachmentRepo repository.AttachmentRepository
	storageRepo    repository.StorageConfigRepository
	retentionRepo  repository.RetentionPolicyRepository
	auditLogger    *audit.Logger
	logger         *zap.Logger
}

func NewDocumentService(
	documentRepo repository.DocumentRepository,
	versionRepo repository.DocumentVersionRepository,
	attachmentRepo repository.AttachmentRepository,
	storageRepo repository.StorageConfigRepository,
	retentionRepo repository.RetentionPolicyRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *DocumentService {
	return &DocumentService{
		documentRepo:   documentRepo,
		versionRepo:    versionRepo,
		attachmentRepo: attachmentRepo,
		storageRepo:    storageRepo,
		retentionRepo:  retentionRepo,
		auditLogger:    auditLogger,
		logger:         logger,
	}
}

type UploadRequest struct {
	EntityID       common.ID
	FileName       string
	MimeType       string
	FileSize       int64
	DocumentType   string
	Description    string
	Tags           []string
	Metadata       map[string]interface{}
	RetentionCode  string
	UploadedBy     common.ID
}

type UploadResult struct {
	Document    *domain.Document
	StoragePath string
}

func (s *DocumentService) Upload(ctx context.Context, req UploadRequest, fileReader io.Reader) (*domain.Document, error) {
	storageConfig, err := s.storageRepo.GetDefault(ctx, &req.EntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	hasher := sha256.New()
	content, err := io.ReadAll(io.TeeReader(fileReader, hasher))
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	fileID := common.NewID().String()
	ext := filepath.Ext(req.FileName)
	storedFileName := fileID + ext
	storagePath := s.generateStoragePath(req.EntityID, req.DocumentType, storedFileName)

	if err := s.storeFile(ctx, storageConfig, storagePath, content); err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	docNumber, err := s.documentRepo.GenerateDocumentNumber(ctx, req.EntityID, "DOC")
	if err != nil {
		return nil, fmt.Errorf("failed to generate document number: %w", err)
	}

	doc := domain.NewDocument(
		req.EntityID,
		docNumber,
		storedFileName,
		req.FileName,
		req.MimeType,
		req.FileSize,
		checksum,
		storageConfig.ID,
		storagePath,
		req.DocumentType,
		req.UploadedBy,
	)

	if req.Description != "" {
		doc.SetDescription(req.Description)
	}
	if len(req.Tags) > 0 {
		doc.SetTags(req.Tags)
	}
	if req.Metadata != nil {
		doc.SetMetadata(req.Metadata)
	}

	if req.RetentionCode != "" {
		policy, err := s.retentionRepo.GetByCode(ctx, &req.EntityID, req.RetentionCode)
		if err == nil {
			expiresAt := policy.CalculateExpiryDate(doc.CreatedAt)
			doc.SetRetentionPolicy(policy.ID, &expiresAt)
		}
	} else {
		policy, _ := s.retentionRepo.GetDefault(ctx, &req.EntityID, req.DocumentType)
		if policy != nil {
			expiresAt := policy.CalculateExpiryDate(doc.CreatedAt)
			doc.SetRetentionPolicy(policy.ID, &expiresAt)
		}
	}

	if err := s.documentRepo.Create(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to create document: %w", err)
	}

	s.auditLogger.Log(ctx, "document", doc.ID, "uploaded", map[string]any{
		"entity_id":       req.EntityID,
		"uploaded_by":     req.UploadedBy,
		"document_number": doc.DocumentNumber,
		"file_name":       req.FileName,
		"file_size":       req.FileSize,
		"document_type":   req.DocumentType,
	})

	return doc, nil
}

func (s *DocumentService) Download(ctx context.Context, documentID common.ID) (io.ReadCloser, *domain.Document, error) {
	doc, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}

	if !doc.IsActive() && !doc.IsArchived() {
		return nil, nil, domain.ErrDocumentDeleted
	}

	storageConfig, err := s.storageRepo.GetByID(ctx, doc.StorageID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	reader, err := s.retrieveFile(ctx, storageConfig, doc.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve file: %w", err)
	}

	return reader, doc, nil
}

func (s *DocumentService) GetDocument(ctx context.Context, id common.ID) (*domain.Document, error) {
	doc, err := s.documentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	versions, err := s.versionRepo.GetByDocumentID(ctx, id)
	if err != nil {
		s.logger.Warn("failed to load document versions", zap.Error(err))
	} else {
		doc.Versions = versions
	}

	return doc, nil
}

func (s *DocumentService) ListDocuments(ctx context.Context, filter repository.DocumentFilter) ([]domain.Document, int, error) {
	return s.documentRepo.List(ctx, filter)
}

func (s *DocumentService) UpdateDocument(ctx context.Context, id common.ID, updates UpdateDocumentRequest) (*domain.Document, error) {
	doc, err := s.documentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !doc.CanModify() {
		if doc.LegalHold {
			return nil, domain.ErrDocumentOnLegalHold
		}
		return nil, domain.ErrInvalidDocumentStatus
	}

	if updates.Description != nil {
		doc.SetDescription(*updates.Description)
	}
	if updates.Tags != nil {
		doc.SetTags(updates.Tags)
	}
	if updates.Metadata != nil {
		doc.SetMetadata(updates.Metadata)
	}
	if updates.DocumentType != nil {
		doc.DocumentType = *updates.DocumentType
		doc.UpdatedAt = time.Now()
	}

	if err := s.documentRepo.Update(ctx, doc); err != nil {
		return nil, err
	}

	return doc, nil
}

type UpdateDocumentRequest struct {
	Description  *string
	Tags         []string
	Metadata     map[string]interface{}
	DocumentType *string
}

func (s *DocumentService) Attach(ctx context.Context, documentID common.ID, refType string, refID common.ID, attachedBy common.ID) (*domain.Attachment, error) {
	doc, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return nil, err
	}

	if !doc.IsActive() {
		return nil, domain.ErrInvalidDocumentStatus
	}

	existing, err := s.attachmentRepo.GetByDocumentAndReference(ctx, documentID, refType, refID)
	if err == nil && existing != nil {
		return nil, domain.ErrAttachmentAlreadyExists
	}

	attachment := domain.NewAttachment(documentID, doc.EntityID, refType, refID, attachedBy)

	if err := s.attachmentRepo.Create(ctx, attachment); err != nil {
		return nil, fmt.Errorf("failed to create attachment: %w", err)
	}

	s.auditLogger.Log(ctx, "document", documentID, "attached", map[string]any{
		"entity_id":      doc.EntityID,
		"attached_by":    attachedBy,
		"reference_type": refType,
		"reference_id":   refID.String(),
	})

	return attachment, nil
}

func (s *DocumentService) Detach(ctx context.Context, documentID common.ID, refType string, refID common.ID, detachedBy common.ID) error {
	doc, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}

	if err := s.attachmentRepo.DeleteByDocumentAndReference(ctx, documentID, refType, refID); err != nil {
		return err
	}

	s.auditLogger.Log(ctx, "document", documentID, "detached", map[string]any{
		"entity_id":      doc.EntityID,
		"detached_by":    detachedBy,
		"reference_type": refType,
		"reference_id":   refID.String(),
	})

	return nil
}

func (s *DocumentService) GetAttachments(ctx context.Context, refType string, refID common.ID) ([]domain.Document, error) {
	attachmentsWithDocs, err := s.attachmentRepo.ListByReferenceWithDocuments(ctx, refType, refID)
	if err != nil {
		return nil, err
	}

	documents := make([]domain.Document, 0, len(attachmentsWithDocs))
	for _, awd := range attachmentsWithDocs {
		if awd.Document != nil {
			documents = append(documents, *awd.Document)
		}
	}

	return documents, nil
}

func (s *DocumentService) SetLegalHold(ctx context.Context, documentID common.ID, hold bool, reason string, userID common.ID) error {
	doc, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}

	if err := doc.SetLegalHold(hold, reason); err != nil {
		return err
	}

	if err := s.documentRepo.Update(ctx, doc); err != nil {
		return err
	}

	action := "document.legal_hold_set"
	if !hold {
		action = "document.legal_hold_removed"
	}

	s.auditLogger.Log(ctx, "document", documentID, action, map[string]any{
		"entity_id":  doc.EntityID,
		"user_id":    userID,
		"legal_hold": hold,
		"reason":     reason,
	})

	return nil
}

func (s *DocumentService) ArchiveDocument(ctx context.Context, documentID common.ID, userID common.ID) error {
	doc, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}

	if err := doc.Archive(); err != nil {
		return err
	}

	if err := s.documentRepo.Update(ctx, doc); err != nil {
		return err
	}

	s.auditLogger.Log(ctx, "document", documentID, "archived", map[string]any{
		"entity_id": doc.EntityID,
		"user_id":   userID,
	})

	return nil
}

func (s *DocumentService) DeleteDocument(ctx context.Context, documentID common.ID, userID common.ID) error {
	doc, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return err
	}

	if err := doc.Delete(); err != nil {
		return err
	}

	if err := s.documentRepo.Update(ctx, doc); err != nil {
		return err
	}

	s.auditLogger.Log(ctx, "document", documentID, "deleted", map[string]any{
		"entity_id": doc.EntityID,
		"user_id":   userID,
	})

	return nil
}

type CreateVersionRequest struct {
	DocumentID  common.ID
	FileName    string
	FileSize    int64
	ChangeNotes string
	CreatedBy   common.ID
}

func (s *DocumentService) CreateVersion(ctx context.Context, req CreateVersionRequest, fileReader io.Reader) (*domain.DocumentVersion, error) {
	doc, err := s.documentRepo.GetByID(ctx, req.DocumentID)
	if err != nil {
		return nil, err
	}

	if !doc.CanModify() {
		if doc.LegalHold {
			return nil, domain.ErrDocumentOnLegalHold
		}
		return nil, domain.ErrInvalidDocumentStatus
	}

	storageConfig, err := s.storageRepo.GetByID(ctx, doc.StorageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	hasher := sha256.New()
	content, err := io.ReadAll(io.TeeReader(fileReader, hasher))
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	latestVersion, _ := s.versionRepo.GetLatestVersion(ctx, req.DocumentID)
	versionNumber := 1
	if latestVersion != nil {
		versionNumber = latestVersion.VersionNumber + 1
	}

	fileID := common.NewID().String()
	ext := filepath.Ext(req.FileName)
	storedFileName := fmt.Sprintf("%s_v%d%s", fileID, versionNumber, ext)
	storagePath := s.generateStoragePath(doc.EntityID, doc.DocumentType, storedFileName)

	if err := s.storeFile(ctx, storageConfig, storagePath, content); err != nil {
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	version := domain.NewDocumentVersion(
		req.DocumentID,
		versionNumber,
		req.FileName,
		req.FileSize,
		checksum,
		storagePath,
		req.ChangeNotes,
		req.CreatedBy,
	)

	if err := s.versionRepo.Create(ctx, version); err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}

	s.auditLogger.Log(ctx, "document", doc.ID, "version_created", map[string]any{
		"entity_id":      doc.EntityID,
		"created_by":     req.CreatedBy,
		"version_number": versionNumber,
		"file_name":      req.FileName,
	})

	return version, nil
}

func (s *DocumentService) GetVersions(ctx context.Context, documentID common.ID) ([]domain.DocumentVersion, error) {
	return s.versionRepo.GetByDocumentID(ctx, documentID)
}

func (s *DocumentService) DownloadVersion(ctx context.Context, versionID common.ID) (io.ReadCloser, *domain.DocumentVersion, error) {
	version, err := s.versionRepo.GetByID(ctx, versionID)
	if err != nil {
		return nil, nil, err
	}

	doc, err := s.documentRepo.GetByID(ctx, version.DocumentID)
	if err != nil {
		return nil, nil, err
	}

	storageConfig, err := s.storageRepo.GetByID(ctx, doc.StorageID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get storage config: %w", err)
	}

	reader, err := s.retrieveFile(ctx, storageConfig, version.StoragePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to retrieve file: %w", err)
	}

	return reader, version, nil
}

func (s *DocumentService) generateStoragePath(entityID common.ID, documentType, fileName string) string {
	now := time.Now()
	return fmt.Sprintf("%s/%s/%d/%02d/%02d/%s",
		entityID.String(),
		strings.ToLower(documentType),
		now.Year(),
		now.Month(),
		now.Day(),
		fileName,
	)
}

func (s *DocumentService) storeFile(ctx context.Context, config *domain.StorageConfig, path string, content []byte) error {
	s.logger.Info("storing file",
		zap.String("storage_type", string(config.StorageType)),
		zap.String("path", path),
		zap.Int("size", len(content)),
	)
	return nil
}

func (s *DocumentService) retrieveFile(ctx context.Context, config *domain.StorageConfig, path string) (io.ReadCloser, error) {
	s.logger.Info("retrieving file",
		zap.String("storage_type", string(config.StorageType)),
		zap.String("path", path),
	)
	return nil, domain.ErrStorageError
}

func (s *DocumentService) GetRetentionPolicies(ctx context.Context, entityID *common.ID) ([]domain.RetentionPolicy, error) {
	return s.retentionRepo.List(ctx, entityID)
}

func (s *DocumentService) GetStorageConfigs(ctx context.Context, entityID *common.ID) ([]domain.StorageConfig, error) {
	return s.storageRepo.List(ctx, entityID)
}
