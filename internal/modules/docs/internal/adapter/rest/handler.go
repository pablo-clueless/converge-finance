package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/docs/internal/domain"
	"converge-finance.com/m/internal/modules/docs/internal/repository"
	"converge-finance.com/m/internal/modules/docs/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type DocumentHandler struct {
	service *service.DocumentService
	logger  *zap.Logger
}

func NewDocumentHandler(svc *service.DocumentService, logger *zap.Logger) *DocumentHandler {
	return &DocumentHandler{
		service: svc,
		logger:  logger,
	}
}

type UploadRequest struct {
	DocumentType  string                 `json:"document_type"`
	Description   string                 `json:"description,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	RetentionCode string                 `json:"retention_code,omitempty"`
}

type DocumentResponse struct {
	ID                common.ID              `json:"id"`
	EntityID          common.ID              `json:"entity_id"`
	DocumentNumber    string                 `json:"document_number"`
	FileName          string                 `json:"file_name"`
	OriginalName      string                 `json:"original_name"`
	MimeType          string                 `json:"mime_type"`
	FileSize          int64                  `json:"file_size"`
	Checksum          string                 `json:"checksum"`
	StoragePath       string                 `json:"storage_path"`
	DocumentType      string                 `json:"document_type"`
	Description       string                 `json:"description,omitempty"`
	Tags              []string               `json:"tags,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	RetentionPolicyID *common.ID             `json:"retention_policy_id,omitempty"`
	Status            string                 `json:"status"`
	LegalHold         bool                   `json:"legal_hold"`
	LegalHoldReason   string                 `json:"legal_hold_reason,omitempty"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	UploadedBy        common.ID              `json:"uploaded_by"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

func toDocumentResponse(doc *domain.Document) *DocumentResponse {
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

func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	documentType := r.FormValue("document_type")
	if documentType == "" {
		documentType = "general"
	}

	var tags []string
	if tagsStr := r.FormValue("tags"); tagsStr != "" {
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil {
			tags = []string{tagsStr}
		}
	}

	var metadata map[string]interface{}
	if metadataStr := r.FormValue("metadata"); metadataStr != "" {
		_ = json.Unmarshal([]byte(metadataStr), &metadata)
	}

	doc, err := h.service.Upload(r.Context(), service.UploadRequest{
		EntityID:      entityID,
		FileName:      header.Filename,
		MimeType:      header.Header.Get("Content-Type"),
		FileSize:      header.Size,
		DocumentType:  documentType,
		Description:   r.FormValue("description"),
		Tags:          tags,
		Metadata:      metadata,
		RetentionCode: r.FormValue("retention_code"),
		UploadedBy:    userID,
	}, file)

	if err != nil {
		h.logger.Error("upload failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, toDocumentResponse(doc))
}

func (h *DocumentHandler) Download(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	reader, doc, err := h.service.Download(r.Context(), id)
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err == domain.ErrDocumentDeleted {
		h.writeError(w, http.StatusGone, "document has been deleted")
		return
	}
	if err != nil {
		h.logger.Error("download failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", doc.MimeType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+doc.OriginalName+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(doc.FileSize, 10))

	if _, err := io.Copy(w, reader); err != nil {
		h.logger.Error("failed to stream file", zap.Error(err))
	}
}

func (h *DocumentHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	doc, err := h.service.GetDocument(r.Context(), id)
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, toDocumentResponse(doc))
}

func (h *DocumentHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.DocumentFilter{
		EntityID:     entityID,
		DocumentType: r.URL.Query().Get("document_type"),
		SearchQuery:  r.URL.Query().Get("q"),
		Limit:        50,
		Offset:       0,
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := domain.DocumentStatus(statusStr)
		filter.Status = &status
	}

	if legalHoldStr := r.URL.Query().Get("legal_hold"); legalHoldStr != "" {
		legalHold := legalHoldStr == "true"
		filter.LegalHold = &legalHold
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	if dateFromStr := r.URL.Query().Get("date_from"); dateFromStr != "" {
		if dateFrom, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filter.DateFrom = &dateFrom
		}
	}

	if dateToStr := r.URL.Query().Get("date_to"); dateToStr != "" {
		if dateTo, err := time.Parse("2006-01-02", dateToStr); err == nil {
			filter.DateTo = &dateTo
		}
	}

	docs, total, err := h.service.ListDocuments(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]*DocumentResponse, len(docs))
	for i, doc := range docs {
		responses[i] = toDocumentResponse(&doc)
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   responses,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

type UpdateDocumentRequest struct {
	Description  *string                `json:"description,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	DocumentType *string                `json:"document_type,omitempty"`
}

func (h *DocumentHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	var req UpdateDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	doc, err := h.service.UpdateDocument(r.Context(), id, service.UpdateDocumentRequest{
		Description:  req.Description,
		Tags:         req.Tags,
		Metadata:     req.Metadata,
		DocumentType: req.DocumentType,
	})
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err == domain.ErrDocumentOnLegalHold {
		h.writeError(w, http.StatusForbidden, "document is on legal hold")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, toDocumentResponse(doc))
}

func (h *DocumentHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	err := h.service.DeleteDocument(r.Context(), id, userID)
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err == domain.ErrDocumentOnLegalHold {
		h.writeError(w, http.StatusForbidden, "document is on legal hold")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type AttachRequest struct {
	ReferenceType string `json:"reference_type"`
	ReferenceID   string `json:"reference_id"`
}

func (h *DocumentHandler) Attach(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	documentID := common.ID(idStr)
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	var req AttachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ReferenceType == "" || req.ReferenceID == "" {
		h.writeError(w, http.StatusBadRequest, "reference_type and reference_id are required")
		return
	}

	attachment, err := h.service.Attach(r.Context(), documentID, req.ReferenceType, common.ID(req.ReferenceID), userID)
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err == domain.ErrAttachmentAlreadyExists {
		h.writeError(w, http.StatusConflict, "attachment already exists")
		return
	}
	if err != nil {
		h.logger.Error("attach failed", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, attachment)
}

func (h *DocumentHandler) Detach(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	documentID := common.ID(idStr)
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	refType := r.URL.Query().Get("reference_type")
	refID := r.URL.Query().Get("reference_id")

	if refType == "" || refID == "" {
		h.writeError(w, http.StatusBadRequest, "reference_type and reference_id are required")
		return
	}

	err := h.service.Detach(r.Context(), documentID, refType, common.ID(refID), userID)
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err == domain.ErrAttachmentNotFound {
		h.writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DocumentHandler) GetAttachments(w http.ResponseWriter, r *http.Request) {
	refType := chi.URLParam(r, "refType")
	refID := chi.URLParam(r, "refID")

	if refType == "" || refID == "" {
		h.writeError(w, http.StatusBadRequest, "reference_type and reference_id are required")
		return
	}

	docs, err := h.service.GetAttachments(r.Context(), refType, common.ID(refID))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]*DocumentResponse, len(docs))
	for i, doc := range docs {
		responses[i] = toDocumentResponse(&doc)
	}

	h.writeJSON(w, http.StatusOK, responses)
}

type SetLegalHoldRequest struct {
	Hold   bool   `json:"hold"`
	Reason string `json:"reason,omitempty"`
}

func (h *DocumentHandler) SetLegalHold(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	documentID := common.ID(idStr)
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	var req SetLegalHoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Hold && req.Reason == "" {
		h.writeError(w, http.StatusBadRequest, "reason is required when setting legal hold")
		return
	}

	err := h.service.SetLegalHold(r.Context(), documentID, req.Hold, req.Reason, userID)
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	doc, err := h.service.GetDocument(r.Context(), documentID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, toDocumentResponse(doc))
}

func (h *DocumentHandler) ArchiveDocument(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	documentID := common.ID(idStr)
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	err := h.service.ArchiveDocument(r.Context(), documentID, userID)
	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err == domain.ErrDocumentOnLegalHold {
		h.writeError(w, http.StatusForbidden, "document is on legal hold")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	doc, err := h.service.GetDocument(r.Context(), documentID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, toDocumentResponse(doc))
}

type VersionResponse struct {
	ID            common.ID `json:"id"`
	DocumentID    common.ID `json:"document_id"`
	VersionNumber int       `json:"version_number"`
	FileName      string    `json:"file_name"`
	FileSize      int64     `json:"file_size"`
	Checksum      string    `json:"checksum"`
	ChangeNotes   string    `json:"change_notes,omitempty"`
	CreatedBy     common.ID `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
}

func (h *DocumentHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	documentID := common.ID(idStr)
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	version, err := h.service.CreateVersion(r.Context(), service.CreateVersionRequest{
		DocumentID:  documentID,
		FileName:    header.Filename,
		FileSize:    header.Size,
		ChangeNotes: r.FormValue("change_notes"),
		CreatedBy:   userID,
	}, file)

	if err == domain.ErrDocumentNotFound {
		h.writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err == domain.ErrDocumentOnLegalHold {
		h.writeError(w, http.StatusForbidden, "document is on legal hold")
		return
	}
	if err != nil {
		h.logger.Error("create version failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, VersionResponse{
		ID:            version.ID,
		DocumentID:    version.DocumentID,
		VersionNumber: version.VersionNumber,
		FileName:      version.FileName,
		FileSize:      version.FileSize,
		Checksum:      version.Checksum,
		ChangeNotes:   version.ChangeNotes,
		CreatedBy:     version.CreatedBy,
		CreatedAt:     version.CreatedAt,
	})
}

func (h *DocumentHandler) GetVersions(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	documentID := common.ID(idStr)

	versions, err := h.service.GetVersions(r.Context(), documentID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
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

	h.writeJSON(w, http.StatusOK, responses)
}

func (h *DocumentHandler) DownloadVersion(w http.ResponseWriter, r *http.Request) {
	versionIDStr := chi.URLParam(r, "versionId")
	versionID := common.ID(versionIDStr)

	reader, version, err := h.service.DownloadVersion(r.Context(), versionID)
	if err == domain.ErrVersionNotFound {
		h.writeError(w, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		h.logger.Error("download version failed", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+version.FileName+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(version.FileSize, 10))

	if _, err := io.Copy(w, reader); err != nil {
		h.logger.Error("failed to stream file", zap.Error(err))
	}
}

func (h *DocumentHandler) GetRetentionPolicies(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	policies, err := h.service.GetRetentionPolicies(r.Context(), &entityID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, policies)
}

func (h *DocumentHandler) GetStorageConfigs(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	configs, err := h.service.GetStorageConfigs(r.Context(), &entityID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, configs)
}

func (h *DocumentHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *DocumentHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
