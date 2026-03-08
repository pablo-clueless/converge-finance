package docs

import (
	"context"
	"io"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/docs/internal/domain"
	"converge-finance.com/m/internal/modules/docs/internal/repository"
	"converge-finance.com/m/internal/modules/docs/internal/service"
)

type docsAPI struct {
	documentService *service.DocumentService
}

func NewDocsAPI(documentService *service.DocumentService) API {
	return &docsAPI{
		documentService: documentService,
	}
}

func (a *docsAPI) Upload(ctx context.Context, req UploadRequest, fileReader io.Reader) (*DocumentResponse, error) {
	doc, err := a.documentService.Upload(ctx, service.UploadRequest{
		EntityID:      req.EntityID,
		FileName:      req.FileName,
		MimeType:      req.MimeType,
		FileSize:      req.FileSize,
		DocumentType:  req.DocumentType,
		Description:   req.Description,
		Tags:          req.Tags,
		Metadata:      req.Metadata,
		RetentionCode: req.RetentionCode,
		UploadedBy:    req.UploadedBy,
	}, fileReader)
	if err != nil {
		return nil, err
	}

	return mapDocumentToResponse(doc), nil
}

func (a *docsAPI) Download(ctx context.Context, documentID common.ID) (io.ReadCloser, *DocumentResponse, error) {
	reader, doc, err := a.documentService.Download(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}

	return reader, mapDocumentToResponse(doc), nil
}

func (a *docsAPI) GetDocument(ctx context.Context, documentID common.ID) (*DocumentResponse, error) {
	doc, err := a.documentService.GetDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}

	return mapDocumentToResponse(doc), nil
}

func (a *docsAPI) ListDocuments(ctx context.Context, req ListDocumentsRequest) (*ListDocumentsResponse, error) {
	filter := repository.DocumentFilter{
		EntityID:     req.EntityID,
		DocumentType: req.DocumentType,
		Tags:         req.Tags,
		LegalHold:    req.LegalHold,
		DateFrom:     req.DateFrom,
		DateTo:       req.DateTo,
		SearchQuery:  req.SearchQuery,
		Limit:        req.PageSize,
		Offset:       (req.Page - 1) * req.PageSize,
	}

	if req.Status != "" {
		status := domain.DocumentStatus(req.Status)
		filter.Status = &status
	}

	docs, total, err := a.documentService.ListDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]DocumentResponse, len(docs))
	for i, doc := range docs {
		responses[i] = *mapDocumentToResponse(&doc)
	}

	return &ListDocumentsResponse{
		Documents: responses,
		Total:     total,
		Page:      req.Page,
	}, nil
}

func (a *docsAPI) UpdateDocument(ctx context.Context, documentID common.ID, req UpdateDocumentRequest) (*DocumentResponse, error) {
	doc, err := a.documentService.UpdateDocument(ctx, documentID, service.UpdateDocumentRequest{
		Description:  req.Description,
		Tags:         req.Tags,
		Metadata:     req.Metadata,
		DocumentType: req.DocumentType,
	})
	if err != nil {
		return nil, err
	}

	return mapDocumentToResponse(doc), nil
}

func (a *docsAPI) Attach(ctx context.Context, documentID common.ID, refType string, refID common.ID, attachedBy common.ID) (*AttachmentResponse, error) {
	attachment, err := a.documentService.Attach(ctx, documentID, refType, refID, attachedBy)
	if err != nil {
		return nil, err
	}

	return &AttachmentResponse{
		ID:            attachment.ID,
		DocumentID:    attachment.DocumentID,
		EntityID:      attachment.EntityID,
		ReferenceType: attachment.ReferenceType,
		ReferenceID:   attachment.ReferenceID,
		IsPrimary:     attachment.IsPrimary,
		AttachedBy:    attachment.AttachedBy,
		AttachedAt:    attachment.AttachedAt,
	}, nil
}

func (a *docsAPI) Detach(ctx context.Context, documentID common.ID, refType string, refID common.ID, detachedBy common.ID) error {
	return a.documentService.Detach(ctx, documentID, refType, refID, detachedBy)
}

func (a *docsAPI) GetAttachments(ctx context.Context, refType string, refID common.ID) ([]DocumentResponse, error) {
	docs, err := a.documentService.GetAttachments(ctx, refType, refID)
	if err != nil {
		return nil, err
	}

	responses := make([]DocumentResponse, len(docs))
	for i, doc := range docs {
		responses[i] = *mapDocumentToResponse(&doc)
	}

	return responses, nil
}

func (a *docsAPI) SetLegalHold(ctx context.Context, documentID common.ID, hold bool, reason string, userID common.ID) error {
	return a.documentService.SetLegalHold(ctx, documentID, hold, reason, userID)
}

func (a *docsAPI) ArchiveDocument(ctx context.Context, documentID common.ID, userID common.ID) error {
	return a.documentService.ArchiveDocument(ctx, documentID, userID)
}

func (a *docsAPI) DeleteDocument(ctx context.Context, documentID common.ID, userID common.ID) error {
	return a.documentService.DeleteDocument(ctx, documentID, userID)
}

func (a *docsAPI) CreateVersion(ctx context.Context, req CreateVersionRequest, fileReader io.Reader) (*VersionResponse, error) {
	version, err := a.documentService.CreateVersion(ctx, service.CreateVersionRequest{
		DocumentID:  req.DocumentID,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		ChangeNotes: req.ChangeNotes,
		CreatedBy:   req.CreatedBy,
	}, fileReader)
	if err != nil {
		return nil, err
	}

	return &VersionResponse{
		ID:            version.ID,
		DocumentID:    version.DocumentID,
		VersionNumber: version.VersionNumber,
		FileName:      version.FileName,
		FileSize:      version.FileSize,
		Checksum:      version.Checksum,
		ChangeNotes:   version.ChangeNotes,
		CreatedBy:     version.CreatedBy,
		CreatedAt:     version.CreatedAt,
	}, nil
}

func (a *docsAPI) GetVersions(ctx context.Context, documentID common.ID) ([]VersionResponse, error) {
	versions, err := a.documentService.GetVersions(ctx, documentID)
	if err != nil {
		return nil, err
	}

	responses := make([]VersionResponse, len(versions))
	for i, v := range versions {
		responses[i] = VersionResponse{
			ID:            v.ID,
			DocumentID:    v.DocumentID,
			VersionNumber: v.VersionNumber,
			FileName:      v.FileName,
			FileSize:      v.FileSize,
			Checksum:      v.Checksum,
			ChangeNotes:   v.ChangeNotes,
			CreatedBy:     v.CreatedBy,
			CreatedAt:     v.CreatedAt,
		}
	}

	return responses, nil
}

func (a *docsAPI) DownloadVersion(ctx context.Context, versionID common.ID) (io.ReadCloser, *VersionResponse, error) {
	reader, version, err := a.documentService.DownloadVersion(ctx, versionID)
	if err != nil {
		return nil, nil, err
	}

	return reader, &VersionResponse{
		ID:            version.ID,
		DocumentID:    version.DocumentID,
		VersionNumber: version.VersionNumber,
		FileName:      version.FileName,
		FileSize:      version.FileSize,
		Checksum:      version.Checksum,
		ChangeNotes:   version.ChangeNotes,
		CreatedBy:     version.CreatedBy,
		CreatedAt:     version.CreatedAt,
	}, nil
}

func mapDocumentToResponse(doc *domain.Document) *DocumentResponse {
	return &DocumentResponse{
		ID:                doc.ID,
		EntityID:          doc.EntityID,
		DocumentNumber:    doc.DocumentNumber,
		FileName:          doc.FileName,
		OriginalName:      doc.OriginalName,
		MimeType:          doc.MimeType,
		FileSize:          doc.FileSize,
		Checksum:          doc.Checksum,
		StoragePath:       doc.StoragePath,
		DocumentType:      doc.DocumentType,
		Description:       doc.Description,
		Tags:              doc.Tags,
		Metadata:          doc.Metadata,
		RetentionPolicyID: doc.RetentionPolicyID,
		Status:            string(doc.Status),
		LegalHold:         doc.LegalHold,
		LegalHoldReason:   doc.LegalHoldReason,
		ExpiresAt:         doc.ExpiresAt,
		UploadedBy:        doc.UploadedBy,
		CreatedAt:         doc.CreatedAt,
		UpdatedAt:         doc.UpdatedAt,
	}
}
