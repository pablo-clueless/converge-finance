package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusExpired    JobStatus = "expired"
)

func (s JobStatus) IsValid() bool {
	switch s {
	case JobStatusPending, JobStatusProcessing, JobStatusCompleted, JobStatusFailed, JobStatusExpired:
		return true
	}
	return false
}

func (s JobStatus) IsTerminal() bool {
	switch s {
	case JobStatusCompleted, JobStatusFailed, JobStatusExpired:
		return true
	}
	return false
}

type ExportJob struct {
	ID           common.ID
	EntityID     common.ID
	JobNumber    string
	TemplateID   *common.ID
	ExportType   string
	Format       ExportFormat
	Parameters   map[string]any
	Status       JobStatus
	FileName     *string
	FilePath     *string
	FileSize     *int64
	MimeType     *string
	RowCount     *int
	ErrorMessage *string
	ExpiresAt    *time.Time
	RequestedBy  common.ID
	CreatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
}

func NewExportJob(
	entityID common.ID,
	jobNumber string,
	exportType string,
	format ExportFormat,
	requestedBy common.ID,
) *ExportJob {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour) // Default expiration of 24 hours

	return &ExportJob{
		ID:          common.NewID(),
		EntityID:    entityID,
		JobNumber:   jobNumber,
		ExportType:  exportType,
		Format:      format,
		Parameters:  make(map[string]any),
		Status:      JobStatusPending,
		ExpiresAt:   &expiresAt,
		RequestedBy: requestedBy,
		CreatedAt:   now,
	}
}

func (j *ExportJob) SetTemplate(templateID common.ID) {
	j.TemplateID = &templateID
}

func (j *ExportJob) SetParameters(params map[string]any) {
	j.Parameters = params
}

func (j *ExportJob) Start() error {
	if j.Status != JobStatusPending {
		return ErrInvalidJobStatus
	}
	now := time.Now()
	j.Status = JobStatusProcessing
	j.StartedAt = &now
	return nil
}

func (j *ExportJob) Complete(fileName, filePath string, fileSize int64, rowCount int) error {
	if j.Status != JobStatusProcessing {
		return ErrInvalidJobStatus
	}
	now := time.Now()
	mimeType := j.Format.MimeType()

	j.Status = JobStatusCompleted
	j.FileName = &fileName
	j.FilePath = &filePath
	j.FileSize = &fileSize
	j.MimeType = &mimeType
	j.RowCount = &rowCount
	j.CompletedAt = &now
	return nil
}

func (j *ExportJob) Fail(errorMessage string) error {
	if j.Status != JobStatusPending && j.Status != JobStatusProcessing {
		return ErrInvalidJobStatus
	}
	now := time.Now()
	j.Status = JobStatusFailed
	j.ErrorMessage = &errorMessage
	j.CompletedAt = &now
	return nil
}

func (j *ExportJob) Expire() error {
	if j.Status.IsTerminal() && j.Status != JobStatusCompleted {
		return ErrInvalidJobStatus
	}
	j.Status = JobStatusExpired
	return nil
}

func (j *ExportJob) IsDownloadable() bool {
	return j.Status == JobStatusCompleted && j.FilePath != nil
}

func (j *ExportJob) IsExpired() bool {
	if j.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*j.ExpiresAt)
}
