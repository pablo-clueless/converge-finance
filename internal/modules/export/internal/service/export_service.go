package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/export/internal/domain"
	"converge-finance.com/m/internal/modules/export/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"go.uber.org/zap"
)

type ExportService struct {
	templateRepo repository.TemplateRepository
	jobRepo      repository.JobRepository
	scheduleRepo repository.ScheduleRepository
	auditLogger  *audit.Logger
	logger       *zap.Logger
	exportPath   string
}

func NewExportService(
	templateRepo repository.TemplateRepository,
	jobRepo repository.JobRepository,
	scheduleRepo repository.ScheduleRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
	exportPath string,
) *ExportService {
	return &ExportService{
		templateRepo: templateRepo,
		jobRepo:      jobRepo,
		scheduleRepo: scheduleRepo,
		auditLogger:  auditLogger,
		logger:       logger,
		exportPath:   exportPath,
	}
}

type RequestExportInput struct {
	EntityID    common.ID
	TemplateID  *common.ID
	ExportType  string
	Format      domain.ExportFormat
	Parameters  map[string]any
	RequestedBy common.ID
}

func (s *ExportService) RequestExport(ctx context.Context, req RequestExportInput) (*domain.ExportJob, error) {
	if !req.Format.IsValid() {
		return nil, domain.ErrInvalidFormat
	}

	if req.TemplateID != nil {
		template, err := s.templateRepo.GetByID(ctx, *req.TemplateID)
		if err != nil {
			return nil, err
		}
		if !template.IsActive {
			return nil, domain.ErrTemplateInactive
		}
		req.ExportType = template.ExportType
	}

	if req.ExportType == "" {
		return nil, domain.ErrInvalidExportType
	}

	jobNumber, err := s.jobRepo.GenerateJobNumber(ctx, req.EntityID, "EXP-")
	if err != nil {
		return nil, fmt.Errorf("failed to generate job number: %w", err)
	}

	job := domain.NewExportJob(req.EntityID, jobNumber, req.ExportType, req.Format, req.RequestedBy)

	if req.TemplateID != nil {
		job.SetTemplate(*req.TemplateID)
	}
	if req.Parameters != nil {
		job.SetParameters(req.Parameters)
	}

	if err := s.jobRepo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to create export job: %w", err)
	}

	_ = s.auditLogger.Log(ctx, "export_job", job.ID, "requested", map[string]any{
		"entity_id":   req.EntityID,
		"job_number":  job.JobNumber,
		"export_type": job.ExportType,
		"format":      job.Format,
	})

	return job, nil
}

func (s *ExportService) GetJob(ctx context.Context, id common.ID) (*domain.ExportJob, error) {
	return s.jobRepo.GetByID(ctx, id)
}

func (s *ExportService) ListJobs(ctx context.Context, filter repository.JobFilter) ([]domain.ExportJob, int, error) {
	return s.jobRepo.List(ctx, filter)
}

func (s *ExportService) ProcessJob(ctx context.Context, jobID common.ID, generator ExportGenerator) error {
	job, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}

	if err := job.Start(); err != nil {
		return err
	}

	if err := s.jobRepo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	result, err := generator.Generate(ctx, GenerateInput{
		ExportType: job.ExportType,
		Format:     job.Format,
		Parameters: job.Parameters,
		EntityID:   job.EntityID,
	})
	if err != nil {
		s.logger.Error("export generation failed", zap.Error(err), zap.String("job_id", job.ID.String()))
		if failErr := job.Fail(err.Error()); failErr != nil {
			s.logger.Error("failed to mark job as failed", zap.Error(failErr))
		}
		_ = s.jobRepo.Update(ctx, job)
		return fmt.Errorf("export generation failed: %w", err)
	}

	fileName := fmt.Sprintf("%s_%s%s", job.ExportType, job.JobNumber, job.Format.FileExtension())
	filePath := filepath.Join(s.exportPath, job.EntityID.String(), fileName)

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		if failErr := job.Fail(err.Error()); failErr != nil {
			s.logger.Error("failed to mark job as failed", zap.Error(failErr))
		}
		_ = s.jobRepo.Update(ctx, job)
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		if failErr := job.Fail(err.Error()); failErr != nil {
			s.logger.Error("failed to mark job as failed", zap.Error(failErr))
		}
		_ = s.jobRepo.Update(ctx, job)
		return fmt.Errorf("failed to create export file: %w", err)
	}
	defer func() { _ = file.Close() }()

	written, err := io.Copy(file, result.Data)
	if err != nil {
		if failErr := job.Fail(err.Error()); failErr != nil {
			s.logger.Error("failed to mark job as failed", zap.Error(failErr))
		}
		_ = s.jobRepo.Update(ctx, job)
		return fmt.Errorf("failed to write export file: %w", err)
	}

	if err := job.Complete(fileName, filePath, written, result.RowCount); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	if err := s.jobRepo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	_ = s.auditLogger.Log(ctx, "export_job", job.ID, "completed", map[string]any{
		"entity_id":  job.EntityID,
		"job_number": job.JobNumber,
		"file_size":  written,
		"row_count":  result.RowCount,
	})

	return nil
}

type GetJobFileResult struct {
	FilePath string
	FileName string
	MimeType string
	FileSize int64
}

func (s *ExportService) GetJobFile(ctx context.Context, jobID common.ID) (*GetJobFileResult, error) {
	job, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, err
	}

	if !job.IsDownloadable() {
		return nil, domain.ErrJobNotDownloadable
	}

	if job.IsExpired() {
		return nil, domain.ErrJobExpired
	}

	if _, err := os.Stat(*job.FilePath); os.IsNotExist(err) {
		return nil, domain.ErrFileNotFound
	}

	return &GetJobFileResult{
		FilePath: *job.FilePath,
		FileName: *job.FileName,
		MimeType: *job.MimeType,
		FileSize: *job.FileSize,
	}, nil
}

type CreateTemplateInput struct {
	EntityID      common.ID
	TemplateCode  string
	TemplateName  string
	Module        string
	ExportType    string
	Configuration map[string]any
}

func (s *ExportService) CreateTemplate(ctx context.Context, req CreateTemplateInput) (*domain.ExportTemplate, error) {
	if req.TemplateCode == "" {
		return nil, domain.ErrInvalidTemplateCode
	}
	if req.Module == "" {
		return nil, domain.ErrInvalidModule
	}
	if req.ExportType == "" {
		return nil, domain.ErrInvalidExportType
	}

	existing, err := s.templateRepo.GetByCode(ctx, req.EntityID, req.TemplateCode)
	if err == nil && existing != nil && !existing.IsSystem {
		return nil, domain.ErrTemplateAlreadyExists
	}

	template := domain.NewExportTemplate(req.EntityID, req.TemplateCode, req.TemplateName, req.Module, req.ExportType)
	if req.Configuration != nil {
		template.SetConfiguration(req.Configuration)
	}

	if err := s.templateRepo.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	_ = s.auditLogger.Log(ctx, "export_template", template.ID, "created", map[string]any{
		"entity_id":     req.EntityID,
		"template_code": template.TemplateCode,
		"template_name": template.TemplateName,
		"module":        template.Module,
		"export_type":   template.ExportType,
	})

	return template, nil
}

func (s *ExportService) GetTemplate(ctx context.Context, id common.ID) (*domain.ExportTemplate, error) {
	return s.templateRepo.GetByID(ctx, id)
}

func (s *ExportService) ListTemplates(ctx context.Context, entityID common.ID) ([]domain.ExportTemplate, error) {
	templates, _, err := s.templateRepo.List(ctx, repository.TemplateFilter{
		EntityID:      entityID,
		IncludeSystem: true,
	})
	return templates, err
}

func (s *ExportService) ListTemplatesWithFilter(ctx context.Context, filter repository.TemplateFilter) ([]domain.ExportTemplate, int, error) {
	return s.templateRepo.List(ctx, filter)
}

type UpdateTemplateInput struct {
	ID            common.ID
	TemplateName  *string
	Configuration map[string]any
	IsActive      *bool
}

func (s *ExportService) UpdateTemplate(ctx context.Context, req UpdateTemplateInput) (*domain.ExportTemplate, error) {
	template, err := s.templateRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	if template.IsSystem {
		return nil, domain.ErrTemplateNotFound
	}

	template.Update(req.TemplateName, req.Configuration)
	if req.IsActive != nil {
		if *req.IsActive {
			template.Activate()
		} else {
			template.Deactivate()
		}
	}

	if err := s.templateRepo.Update(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	return template, nil
}

func (s *ExportService) DeleteTemplate(ctx context.Context, id common.ID) error {
	return s.templateRepo.Delete(ctx, id)
}

type CreateScheduleInput struct {
	EntityID       common.ID
	ScheduleName   string
	TemplateID     common.ID
	Format         domain.ExportFormat
	Parameters     map[string]any
	CronExpression string
	Recipients     []string
	CreatedBy      common.ID
}

func (s *ExportService) CreateSchedule(ctx context.Context, req CreateScheduleInput) (*domain.ExportSchedule, error) {
	if !req.Format.IsValid() {
		return nil, domain.ErrInvalidFormat
	}

	template, err := s.templateRepo.GetByID(ctx, req.TemplateID)
	if err != nil {
		return nil, err
	}
	if !template.IsActive {
		return nil, domain.ErrTemplateInactive
	}

	if len(req.Recipients) == 0 {
		return nil, domain.ErrNoRecipients
	}

	schedule := domain.NewExportSchedule(req.EntityID, req.ScheduleName, req.TemplateID, req.Format, req.CronExpression, req.CreatedBy)
	if req.Parameters != nil {
		schedule.SetParameters(req.Parameters)
	}
	schedule.SetRecipients(req.Recipients)

	if err := s.scheduleRepo.Create(ctx, schedule); err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	s.auditLogger.Log(ctx, "export_schedule", schedule.ID, "created", map[string]any{
		"entity_id":       req.EntityID,
		"created_by":      req.CreatedBy,
		"schedule_name":   schedule.ScheduleName,
		"template_id":     schedule.TemplateID,
		"cron_expression": schedule.CronExpression,
	})

	return schedule, nil
}

func (s *ExportService) GetSchedule(ctx context.Context, id common.ID) (*domain.ExportSchedule, error) {
	return s.scheduleRepo.GetByID(ctx, id)
}

func (s *ExportService) ListSchedules(ctx context.Context, filter repository.ScheduleFilter) ([]domain.ExportSchedule, int, error) {
	return s.scheduleRepo.List(ctx, filter)
}

type UpdateScheduleInput struct {
	ID             common.ID
	ScheduleName   *string
	CronExpression *string
	Format         *domain.ExportFormat
	Parameters     map[string]any
	Recipients     []string
	IsActive       *bool
}

func (s *ExportService) UpdateSchedule(ctx context.Context, req UpdateScheduleInput) (*domain.ExportSchedule, error) {
	schedule, err := s.scheduleRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	schedule.Update(req.ScheduleName, req.CronExpression, req.Format, req.Parameters, req.Recipients)
	if req.IsActive != nil {
		if *req.IsActive {
			schedule.Activate()
		} else {
			schedule.Deactivate()
		}
	}

	if err := s.scheduleRepo.Update(ctx, schedule); err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	return schedule, nil
}

func (s *ExportService) DeleteSchedule(ctx context.Context, id common.ID) error {
	return s.scheduleRepo.Delete(ctx, id)
}

type ExportGenerator interface {
	Generate(ctx context.Context, input GenerateInput) (*GenerateResult, error)
}

type GenerateInput struct {
	ExportType string
	Format     domain.ExportFormat
	Parameters map[string]any
	EntityID   common.ID
}

type GenerateResult struct {
	Data     io.Reader
	RowCount int
}
