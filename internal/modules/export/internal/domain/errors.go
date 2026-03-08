package domain

import "errors"

var (
	ErrTemplateNotFound      = errors.New("export template not found")
	ErrTemplateAlreadyExists = errors.New("export template already exists")
	ErrTemplateInactive      = errors.New("export template is inactive")
	ErrInvalidTemplateCode   = errors.New("invalid template code")
	ErrInvalidExportType     = errors.New("invalid export type")
	ErrInvalidModule         = errors.New("invalid module")

	ErrJobNotFound       = errors.New("export job not found")
	ErrInvalidJobStatus  = errors.New("invalid job status for this operation")
	ErrJobExpired        = errors.New("export job has expired")
	ErrJobNotDownloadable = errors.New("export job is not downloadable")
	ErrJobAlreadyStarted = errors.New("export job has already started")
	ErrJobProcessing     = errors.New("export job is still processing")
	ErrFileNotFound      = errors.New("export file not found")

	ErrScheduleNotFound      = errors.New("export schedule not found")
	ErrScheduleInactive      = errors.New("export schedule is inactive")
	ErrInvalidCronExpression = errors.New("invalid cron expression")
	ErrNoRecipients          = errors.New("no recipients specified for scheduled export")

	ErrInvalidFormat     = errors.New("invalid export format")
	ErrInvalidParameters = errors.New("invalid export parameters")
	ErrExportFailed      = errors.New("export generation failed")
)
