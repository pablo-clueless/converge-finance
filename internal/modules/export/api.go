package export

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type API interface {
	// RequestExport creates a new export job
	RequestExport(ctx context.Context, req ExportRequest) (*ExportJobResponse, error)

	// GetJob retrieves an export job by ID
	GetJob(ctx context.Context, jobID common.ID) (*ExportJobResponse, error)

	// ListJobs lists export jobs with filtering
	ListJobs(ctx context.Context, req ListJobsRequest) (*ListJobsResponse, error)

	// CreateTemplate creates a new export template
	CreateTemplate(ctx context.Context, req CreateTemplateRequest) (*TemplateResponse, error)

	// GetTemplate retrieves an export template by ID
	GetTemplate(ctx context.Context, templateID common.ID) (*TemplateResponse, error)

	// ListTemplates lists export templates for an entity
	ListTemplates(ctx context.Context, entityID common.ID) ([]TemplateResponse, error)

	// CreateSchedule creates a new export schedule
	CreateSchedule(ctx context.Context, req CreateScheduleRequest) (*ScheduleResponse, error)

	// GetSchedule retrieves an export schedule by ID
	GetSchedule(ctx context.Context, scheduleID common.ID) (*ScheduleResponse, error)

	// ListSchedules lists export schedules with filtering
	ListSchedules(ctx context.Context, req ListSchedulesRequest) (*ListSchedulesResponse, error)
}

type ExportRequest struct {
	EntityID    common.ID
	TemplateID  *common.ID
	ExportType  string
	Format      string
	Parameters  map[string]any
	RequestedBy common.ID
}

type ExportJobResponse struct {
	ID          common.ID
	EntityID    common.ID
	JobNumber   string
	TemplateID  *common.ID
	ExportType  string
	Format      string
	Parameters  map[string]any
	Status      string
	FileName    *string
	FilePath    *string
	FileSize    *int64
	RowCount    *int
	ErrorMessage *string
	ExpiresAt   *time.Time
	RequestedBy common.ID
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type ListJobsRequest struct {
	EntityID    common.ID
	TemplateID  common.ID
	ExportType  string
	Status      string
	RequestedBy common.ID
	DateFrom    *time.Time
	DateTo      *time.Time
	Page        int
	PageSize    int
}

type ListJobsResponse struct {
	Jobs  []ExportJobResponse
	Total int
	Page  int
}

type CreateTemplateRequest struct {
	EntityID      common.ID
	TemplateCode  string
	TemplateName  string
	Module        string
	ExportType    string
	Configuration map[string]any
}

type TemplateResponse struct {
	ID            common.ID
	EntityID      common.ID
	TemplateCode  string
	TemplateName  string
	Module        string
	ExportType    string
	Configuration map[string]any
	IsSystem      bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateScheduleRequest struct {
	EntityID       common.ID
	ScheduleName   string
	TemplateID     common.ID
	Format         string
	Parameters     map[string]any
	CronExpression string
	Recipients     []string
	CreatedBy      common.ID
}

type ScheduleResponse struct {
	ID             common.ID
	EntityID       common.ID
	ScheduleName   string
	TemplateID     common.ID
	Format         string
	Parameters     map[string]any
	CronExpression string
	Recipients     []string
	IsActive       bool
	LastRunAt      *time.Time
	NextRunAt      *time.Time
	CreatedBy      common.ID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ListSchedulesRequest struct {
	EntityID   common.ID
	TemplateID common.ID
	IsActive   *bool
	Page       int
	PageSize   int
}

type ListSchedulesResponse struct {
	Schedules []ScheduleResponse
	Total     int
	Page      int
}
