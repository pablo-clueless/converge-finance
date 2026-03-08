package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type ExportSchedule struct {
	ID             common.ID
	EntityID       common.ID
	ScheduleName   string
	TemplateID     common.ID
	Format         ExportFormat
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

func NewExportSchedule(
	entityID common.ID,
	scheduleName string,
	templateID common.ID,
	format ExportFormat,
	cronExpression string,
	createdBy common.ID,
) *ExportSchedule {
	now := time.Now()
	return &ExportSchedule{
		ID:             common.NewID(),
		EntityID:       entityID,
		ScheduleName:   scheduleName,
		TemplateID:     templateID,
		Format:         format,
		Parameters:     make(map[string]any),
		CronExpression: cronExpression,
		Recipients:     []string{},
		IsActive:       true,
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (s *ExportSchedule) SetParameters(params map[string]any) {
	s.Parameters = params
	s.UpdatedAt = time.Now()
}

func (s *ExportSchedule) SetRecipients(recipients []string) {
	s.Recipients = recipients
	s.UpdatedAt = time.Now()
}

func (s *ExportSchedule) UpdateNextRun(nextRun time.Time) {
	s.NextRunAt = &nextRun
	s.UpdatedAt = time.Now()
}

func (s *ExportSchedule) RecordRun(runTime time.Time, nextRun time.Time) {
	s.LastRunAt = &runTime
	s.NextRunAt = &nextRun
	s.UpdatedAt = time.Now()
}

func (s *ExportSchedule) Activate() {
	s.IsActive = true
	s.UpdatedAt = time.Now()
}

func (s *ExportSchedule) Deactivate() {
	s.IsActive = false
	s.UpdatedAt = time.Now()
}

func (s *ExportSchedule) Update(
	scheduleName *string,
	cronExpression *string,
	format *ExportFormat,
	parameters map[string]any,
	recipients []string,
) {
	if scheduleName != nil {
		s.ScheduleName = *scheduleName
	}
	if cronExpression != nil {
		s.CronExpression = *cronExpression
	}
	if format != nil {
		s.Format = *format
	}
	if parameters != nil {
		s.Parameters = parameters
	}
	if recipients != nil {
		s.Recipients = recipients
	}
	s.UpdatedAt = time.Now()
}

func (s *ExportSchedule) IsDueForRun() bool {
	if !s.IsActive {
		return false
	}
	if s.NextRunAt == nil {
		return false
	}
	return time.Now().After(*s.NextRunAt)
}
