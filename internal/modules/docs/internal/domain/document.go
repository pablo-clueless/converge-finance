package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type DocumentStatus string

const (
	DocumentStatusActive          DocumentStatus = "active"
	DocumentStatusArchived        DocumentStatus = "archived"
	DocumentStatusPendingDeletion DocumentStatus = "pending_deletion"
	DocumentStatusDeleted         DocumentStatus = "deleted"
)

type StorageType string

const (
	StorageTypeLocal     StorageType = "local"
	StorageTypeS3        StorageType = "s3"
	StorageTypeAzureBlob StorageType = "azure_blob"
	StorageTypeGCS       StorageType = "gcs"
)

type StorageConfig struct {
	ID            common.ID
	EntityID      *common.ID
	StorageType   StorageType
	Configuration map[string]interface{}
	IsDefault     bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Document struct {
	ID                common.ID
	EntityID          common.ID
	DocumentNumber    string
	FileName          string
	OriginalName      string
	MimeType          string
	FileSize          int64
	Checksum          string
	StorageID         common.ID
	StoragePath       string
	DocumentType      string
	Description       string
	Tags              []string
	Metadata          map[string]interface{}
	RetentionPolicyID *common.ID
	Status            DocumentStatus
	LegalHold         bool
	LegalHoldReason   string
	ArchivedAt        *time.Time
	ExpiresAt         *time.Time
	DeletedAt         *time.Time
	UploadedBy        common.ID
	CreatedAt         time.Time
	UpdatedAt         time.Time

	Versions []DocumentVersion
}

func NewDocument(
	entityID common.ID,
	documentNumber string,
	fileName, originalName, mimeType string,
	fileSize int64,
	checksum string,
	storageID common.ID,
	storagePath string,
	documentType string,
	uploadedBy common.ID,
) *Document {
	now := time.Now()
	return &Document{
		ID:             common.NewID(),
		EntityID:       entityID,
		DocumentNumber: documentNumber,
		FileName:       fileName,
		OriginalName:   originalName,
		MimeType:       mimeType,
		FileSize:       fileSize,
		Checksum:       checksum,
		StorageID:      storageID,
		StoragePath:    storagePath,
		DocumentType:   documentType,
		Tags:           []string{},
		Metadata:       make(map[string]interface{}),
		Status:         DocumentStatusActive,
		LegalHold:      false,
		UploadedBy:     uploadedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
		Versions:       []DocumentVersion{},
	}
}

func (d *Document) SetDescription(description string) {
	d.Description = description
	d.UpdatedAt = time.Now()
}

func (d *Document) SetTags(tags []string) {
	d.Tags = tags
	d.UpdatedAt = time.Now()
}

func (d *Document) SetMetadata(metadata map[string]interface{}) {
	d.Metadata = metadata
	d.UpdatedAt = time.Now()
}

func (d *Document) SetRetentionPolicy(policyID common.ID, expiresAt *time.Time) {
	d.RetentionPolicyID = &policyID
	d.ExpiresAt = expiresAt
	d.UpdatedAt = time.Now()
}

func (d *Document) SetLegalHold(hold bool, reason string) error {
	d.LegalHold = hold
	if hold {
		d.LegalHoldReason = reason
	} else {
		d.LegalHoldReason = ""
	}
	d.UpdatedAt = time.Now()
	return nil
}

func (d *Document) Archive() error {
	if d.Status != DocumentStatusActive {
		return ErrInvalidDocumentStatus
	}
	if d.LegalHold {
		return ErrDocumentOnLegalHold
	}
	now := time.Now()
	d.Status = DocumentStatusArchived
	d.ArchivedAt = &now
	d.UpdatedAt = now
	return nil
}

func (d *Document) MarkForDeletion() error {
	if d.LegalHold {
		return ErrDocumentOnLegalHold
	}
	if d.Status == DocumentStatusDeleted {
		return ErrDocumentDeleted
	}
	d.Status = DocumentStatusPendingDeletion
	d.UpdatedAt = time.Now()
	return nil
}

func (d *Document) Delete() error {
	if d.LegalHold {
		return ErrDocumentOnLegalHold
	}
	now := time.Now()
	d.Status = DocumentStatusDeleted
	d.DeletedAt = &now
	d.UpdatedAt = now
	return nil
}

func (d *Document) Restore() error {
	if d.Status != DocumentStatusArchived && d.Status != DocumentStatusPendingDeletion {
		return ErrInvalidDocumentStatus
	}
	d.Status = DocumentStatusActive
	d.ArchivedAt = nil
	d.UpdatedAt = time.Now()
	return nil
}

func (d *Document) IsActive() bool {
	return d.Status == DocumentStatusActive
}

func (d *Document) IsArchived() bool {
	return d.Status == DocumentStatusArchived
}

func (d *Document) IsDeleted() bool {
	return d.Status == DocumentStatusDeleted
}

func (d *Document) IsExpired() bool {
	if d.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*d.ExpiresAt)
}

func (d *Document) CanModify() bool {
	return d.IsActive() && !d.LegalHold
}

func (d *Document) AddVersion(version DocumentVersion) {
	d.Versions = append(d.Versions, version)
	d.UpdatedAt = time.Now()
}

type DocumentVersion struct {
	ID            common.ID
	DocumentID    common.ID
	VersionNumber int
	FileName      string
	FileSize      int64
	Checksum      string
	StoragePath   string
	ChangeNotes   string
	CreatedBy     common.ID
	CreatedAt     time.Time
}

func NewDocumentVersion(
	documentID common.ID,
	versionNumber int,
	fileName string,
	fileSize int64,
	checksum string,
	storagePath string,
	changeNotes string,
	createdBy common.ID,
) *DocumentVersion {
	return &DocumentVersion{
		ID:            common.NewID(),
		DocumentID:    documentID,
		VersionNumber: versionNumber,
		FileName:      fileName,
		FileSize:      fileSize,
		Checksum:      checksum,
		StoragePath:   storagePath,
		ChangeNotes:   changeNotes,
		CreatedBy:     createdBy,
		CreatedAt:     time.Now(),
	}
}
