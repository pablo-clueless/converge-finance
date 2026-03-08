package docs

import (
	"context"
	"io"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type API interface {
	Upload(ctx context.Context, req UploadRequest, fileReader io.Reader) (*DocumentResponse, error)

	Download(ctx context.Context, documentID common.ID) (io.ReadCloser, *DocumentResponse, error)

	GetDocument(ctx context.Context, documentID common.ID) (*DocumentResponse, error)

	ListDocuments(ctx context.Context, req ListDocumentsRequest) (*ListDocumentsResponse, error)

	UpdateDocument(ctx context.Context, documentID common.ID, req UpdateDocumentRequest) (*DocumentResponse, error)

	Attach(ctx context.Context, documentID common.ID, refType string, refID common.ID, attachedBy common.ID) (*AttachmentResponse, error)

	Detach(ctx context.Context, documentID common.ID, refType string, refID common.ID, detachedBy common.ID) error

	GetAttachments(ctx context.Context, refType string, refID common.ID) ([]DocumentResponse, error)

	SetLegalHold(ctx context.Context, documentID common.ID, hold bool, reason string, userID common.ID) error

	ArchiveDocument(ctx context.Context, documentID common.ID, userID common.ID) error

	DeleteDocument(ctx context.Context, documentID common.ID, userID common.ID) error

	CreateVersion(ctx context.Context, req CreateVersionRequest, fileReader io.Reader) (*VersionResponse, error)

	GetVersions(ctx context.Context, documentID common.ID) ([]VersionResponse, error)

	DownloadVersion(ctx context.Context, versionID common.ID) (io.ReadCloser, *VersionResponse, error)
}

type UploadRequest struct {
	EntityID      common.ID
	FileName      string
	MimeType      string
	FileSize      int64
	DocumentType  string
	Description   string
	Tags          []string
	Metadata      map[string]interface{}
	RetentionCode string
	UploadedBy    common.ID
}

type DocumentResponse struct {
	ID                common.ID
	EntityID          common.ID
	DocumentNumber    string
	FileName          string
	OriginalName      string
	MimeType          string
	FileSize          int64
	Checksum          string
	StoragePath       string
	DocumentType      string
	Description       string
	Tags              []string
	Metadata          map[string]interface{}
	RetentionPolicyID *common.ID
	Status            string
	LegalHold         bool
	LegalHoldReason   string
	ExpiresAt         *time.Time
	UploadedBy        common.ID
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ListDocumentsRequest struct {
	EntityID     common.ID
	DocumentType string
	Status       string
	Tags         []string
	LegalHold    *bool
	DateFrom     *time.Time
	DateTo       *time.Time
	SearchQuery  string
	Page         int
	PageSize     int
}

type ListDocumentsResponse struct {
	Documents []DocumentResponse
	Total     int
	Page      int
}

type UpdateDocumentRequest struct {
	Description  *string
	Tags         []string
	Metadata     map[string]interface{}
	DocumentType *string
}

type AttachmentResponse struct {
	ID            common.ID
	DocumentID    common.ID
	EntityID      common.ID
	ReferenceType string
	ReferenceID   common.ID
	IsPrimary     bool
	AttachedBy    common.ID
	AttachedAt    time.Time
}

type CreateVersionRequest struct {
	DocumentID  common.ID
	FileName    string
	FileSize    int64
	ChangeNotes string
	CreatedBy   common.ID
}

type VersionResponse struct {
	ID            common.ID
	DocumentID    common.ID
	VersionNumber int
	FileName      string
	FileSize      int64
	Checksum      string
	ChangeNotes   string
	CreatedBy     common.ID
	CreatedAt     time.Time
}
