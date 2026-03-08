package export

import (
	"context"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/export/internal/domain"
	"converge-finance.com/m/internal/modules/export/internal/repository"
	"converge-finance.com/m/internal/modules/export/internal/service"
)

type exportAPI struct {
	exportService *service.ExportService
}

func NewExportAPI(exportService *service.ExportService) API {
	return &exportAPI{
		exportService: exportService,
	}
}

func (a *exportAPI) RequestExport(ctx context.Context, req ExportRequest) (*ExportJobResponse, error) {
	format := domain.ExportFormat(req.Format)
	if !format.IsValid() {
		return nil, domain.ErrInvalidFormat
	}

	job, err := a.exportService.RequestExport(ctx, service.RequestExportInput{
		EntityID:    req.EntityID,
		TemplateID:  req.TemplateID,
		ExportType:  req.ExportType,
		Format:      format,
		Parameters:  req.Parameters,
		RequestedBy: req.RequestedBy,
	})
	if err != nil {
		return nil, err
	}

	return a.mapJobToResponse(job), nil
}

func (a *exportAPI) GetJob(ctx context.Context, jobID common.ID) (*ExportJobResponse, error) {
	job, err := a.exportService.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	return a.mapJobToResponse(job), nil
}

func (a *exportAPI) ListJobs(ctx context.Context, req ListJobsRequest) (*ListJobsResponse, error) {
	filter := repository.JobFilter{
		EntityID:    req.EntityID,
		TemplateID:  req.TemplateID,
		ExportType:  req.ExportType,
		RequestedBy: req.RequestedBy,
		DateFrom:    req.DateFrom,
		DateTo:      req.DateTo,
		Limit:       req.PageSize,
		Offset:      (req.Page - 1) * req.PageSize,
	}

	if req.Status != "" {
		status := domain.JobStatus(req.Status)
		filter.Status = &status
	}

	jobs, total, err := a.exportService.ListJobs(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]ExportJobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = *a.mapJobToResponse(&job)
	}

	return &ListJobsResponse{
		Jobs:  responses,
		Total: total,
		Page:  req.Page,
	}, nil
}

func (a *exportAPI) CreateTemplate(ctx context.Context, req CreateTemplateRequest) (*TemplateResponse, error) {
	template, err := a.exportService.CreateTemplate(ctx, service.CreateTemplateInput{
		EntityID:      req.EntityID,
		TemplateCode:  req.TemplateCode,
		TemplateName:  req.TemplateName,
		Module:        req.Module,
		ExportType:    req.ExportType,
		Configuration: req.Configuration,
	})
	if err != nil {
		return nil, err
	}

	return a.mapTemplateToResponse(template), nil
}

func (a *exportAPI) GetTemplate(ctx context.Context, templateID common.ID) (*TemplateResponse, error) {
	template, err := a.exportService.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}

	return a.mapTemplateToResponse(template), nil
}

func (a *exportAPI) ListTemplates(ctx context.Context, entityID common.ID) ([]TemplateResponse, error) {
	templates, err := a.exportService.ListTemplates(ctx, entityID)
	if err != nil {
		return nil, err
	}

	responses := make([]TemplateResponse, len(templates))
	for i, template := range templates {
		responses[i] = *a.mapTemplateToResponse(&template)
	}

	return responses, nil
}

func (a *exportAPI) CreateSchedule(ctx context.Context, req CreateScheduleRequest) (*ScheduleResponse, error) {
	format := domain.ExportFormat(req.Format)
	if !format.IsValid() {
		return nil, domain.ErrInvalidFormat
	}

	schedule, err := a.exportService.CreateSchedule(ctx, service.CreateScheduleInput{
		EntityID:       req.EntityID,
		ScheduleName:   req.ScheduleName,
		TemplateID:     req.TemplateID,
		Format:         format,
		Parameters:     req.Parameters,
		CronExpression: req.CronExpression,
		Recipients:     req.Recipients,
		CreatedBy:      req.CreatedBy,
	})
	if err != nil {
		return nil, err
	}

	return a.mapScheduleToResponse(schedule), nil
}

func (a *exportAPI) GetSchedule(ctx context.Context, scheduleID common.ID) (*ScheduleResponse, error) {
	schedule, err := a.exportService.GetSchedule(ctx, scheduleID)
	if err != nil {
		return nil, err
	}

	return a.mapScheduleToResponse(schedule), nil
}

func (a *exportAPI) ListSchedules(ctx context.Context, req ListSchedulesRequest) (*ListSchedulesResponse, error) {
	filter := repository.ScheduleFilter{
		EntityID:   req.EntityID,
		TemplateID: req.TemplateID,
		IsActive:   req.IsActive,
		Limit:      req.PageSize,
		Offset:     (req.Page - 1) * req.PageSize,
	}

	schedules, total, err := a.exportService.ListSchedules(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]ScheduleResponse, len(schedules))
	for i, schedule := range schedules {
		responses[i] = *a.mapScheduleToResponse(&schedule)
	}

	return &ListSchedulesResponse{
		Schedules: responses,
		Total:     total,
		Page:      req.Page,
	}, nil
}

func (a *exportAPI) mapJobToResponse(job *domain.ExportJob) *ExportJobResponse {
	return &ExportJobResponse{
		ID:           job.ID,
		EntityID:     job.EntityID,
		JobNumber:    job.JobNumber,
		TemplateID:   job.TemplateID,
		ExportType:   job.ExportType,
		Format:       string(job.Format),
		Parameters:   job.Parameters,
		Status:       string(job.Status),
		FileName:     job.FileName,
		FilePath:     job.FilePath,
		FileSize:     job.FileSize,
		RowCount:     job.RowCount,
		ErrorMessage: job.ErrorMessage,
		ExpiresAt:    job.ExpiresAt,
		RequestedBy:  job.RequestedBy,
		CreatedAt:    job.CreatedAt,
		StartedAt:    job.StartedAt,
		CompletedAt:  job.CompletedAt,
	}
}

func (a *exportAPI) mapTemplateToResponse(template *domain.ExportTemplate) *TemplateResponse {
	return &TemplateResponse{
		ID:            template.ID,
		EntityID:      template.EntityID,
		TemplateCode:  template.TemplateCode,
		TemplateName:  template.TemplateName,
		Module:        template.Module,
		ExportType:    template.ExportType,
		Configuration: template.Configuration,
		IsSystem:      template.IsSystem,
		IsActive:      template.IsActive,
		CreatedAt:     template.CreatedAt,
		UpdatedAt:     template.UpdatedAt,
	}
}

func (a *exportAPI) mapScheduleToResponse(schedule *domain.ExportSchedule) *ScheduleResponse {
	return &ScheduleResponse{
		ID:             schedule.ID,
		EntityID:       schedule.EntityID,
		ScheduleName:   schedule.ScheduleName,
		TemplateID:     schedule.TemplateID,
		Format:         string(schedule.Format),
		Parameters:     schedule.Parameters,
		CronExpression: schedule.CronExpression,
		Recipients:     schedule.Recipients,
		IsActive:       schedule.IsActive,
		LastRunAt:      schedule.LastRunAt,
		NextRunAt:      schedule.NextRunAt,
		CreatedBy:      schedule.CreatedBy,
		CreatedAt:      schedule.CreatedAt,
		UpdatedAt:      schedule.UpdatedAt,
	}
}
