package rest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/export/internal/domain"
	"converge-finance.com/m/internal/modules/export/internal/repository"
	"converge-finance.com/m/internal/modules/export/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ExportHandler struct {
	service *service.ExportService
	logger  *zap.Logger
}

func NewExportHandler(svc *service.ExportService, logger *zap.Logger) *ExportHandler {
	return &ExportHandler{
		service: svc,
		logger:  logger,
	}
}

type RequestExportRequest struct {
	TemplateID string         `json:"template_id,omitempty"`
	ExportType string         `json:"export_type"`
	Format     string         `json:"format"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type ExportJobResponse struct {
	ID          string         `json:"id"`
	JobNumber   string         `json:"job_number"`
	TemplateID  *string        `json:"template_id,omitempty"`
	ExportType  string         `json:"export_type"`
	Format      string         `json:"format"`
	Parameters  map[string]any `json:"parameters"`
	Status      string         `json:"status"`
	FileName    *string        `json:"file_name,omitempty"`
	FileSize    *int64         `json:"file_size,omitempty"`
	RowCount    *int           `json:"row_count,omitempty"`
	Error       *string        `json:"error,omitempty"`
	ExpiresAt   *string        `json:"expires_at,omitempty"`
	RequestedBy string         `json:"requested_by"`
	CreatedAt   string         `json:"created_at"`
	StartedAt   *string        `json:"started_at,omitempty"`
	CompletedAt *string        `json:"completed_at,omitempty"`
}

func (h *ExportHandler) RequestExport(w http.ResponseWriter, r *http.Request) {
	var req RequestExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	format := domain.ExportFormat(req.Format)
	if !format.IsValid() {
		h.writeError(w, http.StatusBadRequest, "invalid format")
		return
	}

	input := service.RequestExportInput{
		EntityID:    entityID,
		ExportType:  req.ExportType,
		Format:      format,
		Parameters:  req.Parameters,
		RequestedBy: userID,
	}

	if req.TemplateID != "" {
		templateID := common.ID(req.TemplateID)
		input.TemplateID = &templateID
	}

	job, err := h.service.RequestExport(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to request export", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, h.mapJobToResponse(job))
}

func (h *ExportHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	job, err := h.service.GetJob(r.Context(), id)
	if err == domain.ErrJobNotFound {
		h.writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.mapJobToResponse(job))
}

func (h *ExportHandler) ListJobs(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.JobFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if exportType := r.URL.Query().Get("export_type"); exportType != "" {
		filter.ExportType = exportType
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := domain.JobStatus(statusStr)
		filter.Status = &status
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

	jobs, total, err := h.service.ListJobs(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]ExportJobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = h.mapJobToResponse(&job)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   responses,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *ExportHandler) DownloadJob(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	result, err := h.service.GetJobFile(r.Context(), id)
	if err == domain.ErrJobNotFound {
		h.writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err == domain.ErrJobNotDownloadable {
		h.writeError(w, http.StatusBadRequest, "job is not downloadable")
		return
	}
	if err == domain.ErrJobExpired {
		h.writeError(w, http.StatusGone, "export has expired")
		return
	}
	if err == domain.ErrFileNotFound {
		h.writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	file, err := os.Open(result.FilePath)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer func() { _ = file.Close() }()

	w.Header().Set("Content-Type", result.MimeType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+result.FileName+"\"")
	w.Header().Set("Content-Length", strconv.FormatInt(result.FileSize, 10))

	if _, err := io.Copy(w, file); err != nil {
		h.logger.Error("failed to stream file", zap.Error(err))
	}
}

type CreateTemplateRequest struct {
	TemplateCode  string         `json:"template_code"`
	TemplateName  string         `json:"template_name"`
	Module        string         `json:"module"`
	ExportType    string         `json:"export_type"`
	Configuration map[string]any `json:"configuration,omitempty"`
}

type TemplateResponse struct {
	ID            string         `json:"id"`
	EntityID      string         `json:"entity_id"`
	TemplateCode  string         `json:"template_code"`
	TemplateName  string         `json:"template_name"`
	Module        string         `json:"module"`
	ExportType    string         `json:"export_type"`
	Configuration map[string]any `json:"configuration"`
	IsSystem      bool           `json:"is_system"`
	IsActive      bool           `json:"is_active"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

func (h *ExportHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req CreateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	template, err := h.service.CreateTemplate(r.Context(), service.CreateTemplateInput{
		EntityID:      entityID,
		TemplateCode:  req.TemplateCode,
		TemplateName:  req.TemplateName,
		Module:        req.Module,
		ExportType:    req.ExportType,
		Configuration: req.Configuration,
	})
	if err != nil {
		h.logger.Error("failed to create template", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, h.mapTemplateToResponse(template))
}

func (h *ExportHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	template, err := h.service.GetTemplate(r.Context(), id)
	if err == domain.ErrTemplateNotFound {
		h.writeError(w, http.StatusNotFound, "template not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.mapTemplateToResponse(template))
}

func (h *ExportHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.TemplateFilter{
		EntityID:      entityID,
		IncludeSystem: true,
		Limit:         100,
		Offset:        0,
	}

	if module := r.URL.Query().Get("module"); module != "" {
		filter.Module = module
	}

	if exportType := r.URL.Query().Get("export_type"); exportType != "" {
		filter.ExportType = exportType
	}

	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		filter.IsActive = &active
	}

	templates, total, err := h.service.ListTemplatesWithFilter(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]TemplateResponse, len(templates))
	for i, template := range templates {
		responses[i] = h.mapTemplateToResponse(&template)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   responses,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

type UpdateTemplateRequest struct {
	TemplateName  *string        `json:"template_name,omitempty"`
	Configuration map[string]any `json:"configuration,omitempty"`
	IsActive      *bool          `json:"is_active,omitempty"`
}

func (h *ExportHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	var req UpdateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	template, err := h.service.UpdateTemplate(r.Context(), service.UpdateTemplateInput{
		ID:            id,
		TemplateName:  req.TemplateName,
		Configuration: req.Configuration,
		IsActive:      req.IsActive,
	})
	if err == domain.ErrTemplateNotFound {
		h.writeError(w, http.StatusNotFound, "template not found")
		return
	}
	if err != nil {
		h.logger.Error("failed to update template", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.mapTemplateToResponse(template))
}

func (h *ExportHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	err := h.service.DeleteTemplate(r.Context(), id)
	if err == domain.ErrTemplateNotFound {
		h.writeError(w, http.StatusNotFound, "template not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type CreateScheduleRequest struct {
	ScheduleName   string         `json:"schedule_name"`
	TemplateID     string         `json:"template_id"`
	Format         string         `json:"format"`
	Parameters     map[string]any `json:"parameters,omitempty"`
	CronExpression string         `json:"cron_expression"`
	Recipients     []string       `json:"recipients"`
}

type ScheduleResponse struct {
	ID             string         `json:"id"`
	EntityID       string         `json:"entity_id"`
	ScheduleName   string         `json:"schedule_name"`
	TemplateID     string         `json:"template_id"`
	Format         string         `json:"format"`
	Parameters     map[string]any `json:"parameters"`
	CronExpression string         `json:"cron_expression"`
	Recipients     []string       `json:"recipients"`
	IsActive       bool           `json:"is_active"`
	LastRunAt      *string        `json:"last_run_at,omitempty"`
	NextRunAt      *string        `json:"next_run_at,omitempty"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

func (h *ExportHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	format := domain.ExportFormat(req.Format)
	if !format.IsValid() {
		h.writeError(w, http.StatusBadRequest, "invalid format")
		return
	}

	schedule, err := h.service.CreateSchedule(r.Context(), service.CreateScheduleInput{
		EntityID:       entityID,
		ScheduleName:   req.ScheduleName,
		TemplateID:     common.ID(req.TemplateID),
		Format:         format,
		Parameters:     req.Parameters,
		CronExpression: req.CronExpression,
		Recipients:     req.Recipients,
		CreatedBy:      userID,
	})
	if err != nil {
		h.logger.Error("failed to create schedule", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, h.mapScheduleToResponse(schedule))
}

func (h *ExportHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	schedule, err := h.service.GetSchedule(r.Context(), id)
	if err == domain.ErrScheduleNotFound {
		h.writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.mapScheduleToResponse(schedule))
}

func (h *ExportHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.ScheduleFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if templateID := r.URL.Query().Get("template_id"); templateID != "" {
		filter.TemplateID = common.ID(templateID)
	}

	if activeStr := r.URL.Query().Get("active"); activeStr != "" {
		active := activeStr == "true"
		filter.IsActive = &active
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

	schedules, total, err := h.service.ListSchedules(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	responses := make([]ScheduleResponse, len(schedules))
	for i, schedule := range schedules {
		responses[i] = h.mapScheduleToResponse(&schedule)
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   responses,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

type UpdateScheduleRequest struct {
	ScheduleName   *string        `json:"schedule_name,omitempty"`
	Format         *string        `json:"format,omitempty"`
	Parameters     map[string]any `json:"parameters,omitempty"`
	CronExpression *string        `json:"cron_expression,omitempty"`
	Recipients     []string       `json:"recipients,omitempty"`
	IsActive       *bool          `json:"is_active,omitempty"`
}

func (h *ExportHandler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	var req UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	input := service.UpdateScheduleInput{
		ID:             id,
		ScheduleName:   req.ScheduleName,
		CronExpression: req.CronExpression,
		Parameters:     req.Parameters,
		Recipients:     req.Recipients,
		IsActive:       req.IsActive,
	}

	if req.Format != nil {
		format := domain.ExportFormat(*req.Format)
		if !format.IsValid() {
			h.writeError(w, http.StatusBadRequest, "invalid format")
			return
		}
		input.Format = &format
	}

	schedule, err := h.service.UpdateSchedule(r.Context(), input)
	if err == domain.ErrScheduleNotFound {
		h.writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	if err != nil {
		h.logger.Error("failed to update schedule", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, h.mapScheduleToResponse(schedule))
}

func (h *ExportHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	err := h.service.DeleteSchedule(r.Context(), id)
	if err == domain.ErrScheduleNotFound {
		h.writeError(w, http.StatusNotFound, "schedule not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ExportHandler) mapJobToResponse(job *domain.ExportJob) ExportJobResponse {
	resp := ExportJobResponse{
		ID:          job.ID.String(),
		JobNumber:   job.JobNumber,
		ExportType:  job.ExportType,
		Format:      string(job.Format),
		Parameters:  job.Parameters,
		Status:      string(job.Status),
		FileName:    job.FileName,
		FileSize:    job.FileSize,
		RowCount:    job.RowCount,
		Error:       job.ErrorMessage,
		RequestedBy: job.RequestedBy.String(),
		CreatedAt:   job.CreatedAt.Format(time.RFC3339),
	}

	if job.TemplateID != nil {
		templateID := job.TemplateID.String()
		resp.TemplateID = &templateID
	}
	if job.ExpiresAt != nil {
		expiresAt := job.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &expiresAt
	}
	if job.StartedAt != nil {
		startedAt := job.StartedAt.Format(time.RFC3339)
		resp.StartedAt = &startedAt
	}
	if job.CompletedAt != nil {
		completedAt := job.CompletedAt.Format(time.RFC3339)
		resp.CompletedAt = &completedAt
	}

	return resp
}

func (h *ExportHandler) mapTemplateToResponse(template *domain.ExportTemplate) TemplateResponse {
	return TemplateResponse{
		ID:            template.ID.String(),
		EntityID:      template.EntityID.String(),
		TemplateCode:  template.TemplateCode,
		TemplateName:  template.TemplateName,
		Module:        template.Module,
		ExportType:    template.ExportType,
		Configuration: template.Configuration,
		IsSystem:      template.IsSystem,
		IsActive:      template.IsActive,
		CreatedAt:     template.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     template.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *ExportHandler) mapScheduleToResponse(schedule *domain.ExportSchedule) ScheduleResponse {
	resp := ScheduleResponse{
		ID:             schedule.ID.String(),
		EntityID:       schedule.EntityID.String(),
		ScheduleName:   schedule.ScheduleName,
		TemplateID:     schedule.TemplateID.String(),
		Format:         string(schedule.Format),
		Parameters:     schedule.Parameters,
		CronExpression: schedule.CronExpression,
		Recipients:     schedule.Recipients,
		IsActive:       schedule.IsActive,
		CreatedBy:      schedule.CreatedBy.String(),
		CreatedAt:      schedule.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      schedule.UpdatedAt.Format(time.RFC3339),
	}

	if schedule.LastRunAt != nil {
		lastRunAt := schedule.LastRunAt.Format(time.RFC3339)
		resp.LastRunAt = &lastRunAt
	}
	if schedule.NextRunAt != nil {
		nextRunAt := schedule.NextRunAt.Format(time.RFC3339)
		resp.NextRunAt = &nextRunAt
	}

	return resp
}

func (h *ExportHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *ExportHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
