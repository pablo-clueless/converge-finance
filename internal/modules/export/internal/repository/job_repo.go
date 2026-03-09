package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/export/internal/domain"
	"converge-finance.com/m/internal/platform/database"
)

type JobRepository interface {
	Create(ctx context.Context, job *domain.ExportJob) error
	Update(ctx context.Context, job *domain.ExportJob) error
	GetByID(ctx context.Context, id common.ID) (*domain.ExportJob, error)
	GetByJobNumber(ctx context.Context, entityID common.ID, jobNumber string) (*domain.ExportJob, error)
	List(ctx context.Context, filter JobFilter) ([]domain.ExportJob, int, error)
	Delete(ctx context.Context, id common.ID) error
	GenerateJobNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)
	ListExpired(ctx context.Context) ([]domain.ExportJob, error)
}

type JobFilter struct {
	EntityID    common.ID
	TemplateID  common.ID
	ExportType  string
	Status      *domain.JobStatus
	RequestedBy common.ID
	DateFrom    *time.Time
	DateTo      *time.Time
	Limit       int
	Offset      int
}

type ScheduleRepository interface {
	Create(ctx context.Context, schedule *domain.ExportSchedule) error
	Update(ctx context.Context, schedule *domain.ExportSchedule) error
	GetByID(ctx context.Context, id common.ID) (*domain.ExportSchedule, error)
	List(ctx context.Context, filter ScheduleFilter) ([]domain.ExportSchedule, int, error)
	Delete(ctx context.Context, id common.ID) error
	ListDue(ctx context.Context) ([]domain.ExportSchedule, error)
}

type ScheduleFilter struct {
	EntityID   common.ID
	TemplateID common.ID
	IsActive   *bool
	Limit      int
	Offset     int
}

type PostgresJobRepo struct {
	db *database.PostgresDB
}

func NewPostgresJobRepo(db *database.PostgresDB) *PostgresJobRepo {
	return &PostgresJobRepo{db: db}
}

func (r *PostgresJobRepo) Create(ctx context.Context, job *domain.ExportJob) error {
	query := `
		INSERT INTO export.jobs (
			id, entity_id, job_number, template_id, export_type, format,
			parameters, status, file_name, file_path, file_size, mime_type,
			row_count, error_message, expires_at, requested_by,
			created_at, started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`

	paramsJSON, err := json.Marshal(job.Parameters)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query,
		job.ID,
		job.EntityID,
		job.JobNumber,
		job.TemplateID,
		job.ExportType,
		job.Format,
		paramsJSON,
		job.Status,
		job.FileName,
		job.FilePath,
		job.FileSize,
		job.MimeType,
		job.RowCount,
		job.ErrorMessage,
		job.ExpiresAt,
		job.RequestedBy,
		job.CreatedAt,
		job.StartedAt,
		job.CompletedAt,
	)

	return err
}

func (r *PostgresJobRepo) Update(ctx context.Context, job *domain.ExportJob) error {
	query := `
		UPDATE export.jobs SET
			status = $2,
			file_name = $3,
			file_path = $4,
			file_size = $5,
			mime_type = $6,
			row_count = $7,
			error_message = $8,
			started_at = $9,
			completed_at = $10
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		job.ID,
		job.Status,
		job.FileName,
		job.FilePath,
		job.FileSize,
		job.MimeType,
		job.RowCount,
		job.ErrorMessage,
		job.StartedAt,
		job.CompletedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrJobNotFound
	}

	return nil
}

func (r *PostgresJobRepo) GetByID(ctx context.Context, id common.ID) (*domain.ExportJob, error) {
	query := `
		SELECT id, entity_id, job_number, template_id, export_type, format,
			   parameters, status, file_name, file_path, file_size, mime_type,
			   row_count, error_message, expires_at, requested_by,
			   created_at, started_at, completed_at
		FROM export.jobs
		WHERE id = $1
	`

	return r.scanJob(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresJobRepo) GetByJobNumber(ctx context.Context, entityID common.ID, jobNumber string) (*domain.ExportJob, error) {
	query := `
		SELECT id, entity_id, job_number, template_id, export_type, format,
			   parameters, status, file_name, file_path, file_size, mime_type,
			   row_count, error_message, expires_at, requested_by,
			   created_at, started_at, completed_at
		FROM export.jobs
		WHERE entity_id = $1 AND job_number = $2
	`

	return r.scanJob(r.db.QueryRowContext(ctx, query, entityID, jobNumber))
}

func (r *PostgresJobRepo) List(ctx context.Context, filter JobFilter) ([]domain.ExportJob, int, error) {
	baseQuery := `FROM export.jobs WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if !filter.TemplateID.IsZero() {
		baseQuery += fmt.Sprintf(` AND template_id = $%d`, argIdx)
		args = append(args, filter.TemplateID)
		argIdx++
	}
	if filter.ExportType != "" {
		baseQuery += fmt.Sprintf(` AND export_type = $%d`, argIdx)
		args = append(args, filter.ExportType)
		argIdx++
	}
	if filter.Status != nil {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if !filter.RequestedBy.IsZero() {
		baseQuery += fmt.Sprintf(` AND requested_by = $%d`, argIdx)
		args = append(args, filter.RequestedBy)
		argIdx++
	}
	if filter.DateFrom != nil {
		baseQuery += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		baseQuery += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, job_number, template_id, export_type, format,
			   parameters, status, file_name, file_path, file_size, mime_type,
			   row_count, error_message, expires_at, requested_by,
			   created_at, started_at, completed_at
		` + baseQuery + ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		dataQuery += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		dataQuery += fmt.Sprintf(` OFFSET $%d`, argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var jobs []domain.ExportJob
	for rows.Next() {
		job, err := r.scanJobRow(rows)
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, *job)
	}

	return jobs, total, rows.Err()
}

func (r *PostgresJobRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM export.jobs WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrJobNotFound
	}

	return nil
}

func (r *PostgresJobRepo) GenerateJobNumber(ctx context.Context, entityID common.ID, prefix string) (string, error) {
	query := `SELECT export.generate_job_number($1, $2)`
	var jobNumber string
	err := r.db.QueryRowContext(ctx, query, entityID, prefix).Scan(&jobNumber)
	return jobNumber, err
}

func (r *PostgresJobRepo) ListExpired(ctx context.Context) ([]domain.ExportJob, error) {
	query := `
		SELECT id, entity_id, job_number, template_id, export_type, format,
			   parameters, status, file_name, file_path, file_size, mime_type,
			   row_count, error_message, expires_at, requested_by,
			   created_at, started_at, completed_at
		FROM export.jobs
		WHERE status = 'completed' AND expires_at < NOW()
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var jobs []domain.ExportJob
	for rows.Next() {
		job, err := r.scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}

	return jobs, rows.Err()
}

func (r *PostgresJobRepo) scanJob(row rowScanner) (*domain.ExportJob, error) {
	var job domain.ExportJob
	var paramsJSON []byte

	err := row.Scan(
		&job.ID,
		&job.EntityID,
		&job.JobNumber,
		&job.TemplateID,
		&job.ExportType,
		&job.Format,
		&paramsJSON,
		&job.Status,
		&job.FileName,
		&job.FilePath,
		&job.FileSize,
		&job.MimeType,
		&job.RowCount,
		&job.ErrorMessage,
		&job.ExpiresAt,
		&job.RequestedBy,
		&job.CreatedAt,
		&job.StartedAt,
		&job.CompletedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(paramsJSON, &job.Parameters); err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *PostgresJobRepo) scanJobRow(rows *sql.Rows) (*domain.ExportJob, error) {
	var job domain.ExportJob
	var paramsJSON []byte

	err := rows.Scan(
		&job.ID,
		&job.EntityID,
		&job.JobNumber,
		&job.TemplateID,
		&job.ExportType,
		&job.Format,
		&paramsJSON,
		&job.Status,
		&job.FileName,
		&job.FilePath,
		&job.FileSize,
		&job.MimeType,
		&job.RowCount,
		&job.ErrorMessage,
		&job.ExpiresAt,
		&job.RequestedBy,
		&job.CreatedAt,
		&job.StartedAt,
		&job.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(paramsJSON, &job.Parameters); err != nil {
		return nil, err
	}

	return &job, nil
}

type PostgresScheduleRepo struct {
	db *database.PostgresDB
}

func NewPostgresScheduleRepo(db *database.PostgresDB) *PostgresScheduleRepo {
	return &PostgresScheduleRepo{db: db}
}

func (r *PostgresScheduleRepo) Create(ctx context.Context, schedule *domain.ExportSchedule) error {
	query := `
		INSERT INTO export.schedules (
			id, entity_id, schedule_name, template_id, format, parameters,
			cron_expression, recipients, is_active, last_run_at, next_run_at,
			created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	paramsJSON, err := json.Marshal(schedule.Parameters)
	if err != nil {
		return err
	}

	recipientsJSON, err := json.Marshal(schedule.Recipients)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query,
		schedule.ID,
		schedule.EntityID,
		schedule.ScheduleName,
		schedule.TemplateID,
		schedule.Format,
		paramsJSON,
		schedule.CronExpression,
		recipientsJSON,
		schedule.IsActive,
		schedule.LastRunAt,
		schedule.NextRunAt,
		schedule.CreatedBy,
		schedule.CreatedAt,
		schedule.UpdatedAt,
	)

	return err
}

func (r *PostgresScheduleRepo) Update(ctx context.Context, schedule *domain.ExportSchedule) error {
	query := `
		UPDATE export.schedules SET
			schedule_name = $2,
			format = $3,
			parameters = $4,
			cron_expression = $5,
			recipients = $6,
			is_active = $7,
			last_run_at = $8,
			next_run_at = $9,
			updated_at = $10
		WHERE id = $1
	`

	paramsJSON, err := json.Marshal(schedule.Parameters)
	if err != nil {
		return err
	}

	recipientsJSON, err := json.Marshal(schedule.Recipients)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, query,
		schedule.ID,
		schedule.ScheduleName,
		schedule.Format,
		paramsJSON,
		schedule.CronExpression,
		recipientsJSON,
		schedule.IsActive,
		schedule.LastRunAt,
		schedule.NextRunAt,
		schedule.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrScheduleNotFound
	}

	return nil
}

func (r *PostgresScheduleRepo) GetByID(ctx context.Context, id common.ID) (*domain.ExportSchedule, error) {
	query := `
		SELECT id, entity_id, schedule_name, template_id, format, parameters,
			   cron_expression, recipients, is_active, last_run_at, next_run_at,
			   created_by, created_at, updated_at
		FROM export.schedules
		WHERE id = $1
	`

	return r.scanSchedule(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresScheduleRepo) List(ctx context.Context, filter ScheduleFilter) ([]domain.ExportSchedule, int, error) {
	baseQuery := `FROM export.schedules WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if !filter.TemplateID.IsZero() {
		baseQuery += fmt.Sprintf(` AND template_id = $%d`, argIdx)
		args = append(args, filter.TemplateID)
		argIdx++
	}
	if filter.IsActive != nil {
		baseQuery += fmt.Sprintf(` AND is_active = $%d`, argIdx)
		args = append(args, *filter.IsActive)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, schedule_name, template_id, format, parameters,
			   cron_expression, recipients, is_active, last_run_at, next_run_at,
			   created_by, created_at, updated_at
		` + baseQuery + ` ORDER BY schedule_name`

	if filter.Limit > 0 {
		dataQuery += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		dataQuery += fmt.Sprintf(` OFFSET $%d`, argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var schedules []domain.ExportSchedule
	for rows.Next() {
		schedule, err := r.scanScheduleRow(rows)
		if err != nil {
			return nil, 0, err
		}
		schedules = append(schedules, *schedule)
	}

	return schedules, total, rows.Err()
}

func (r *PostgresScheduleRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM export.schedules WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrScheduleNotFound
	}

	return nil
}

func (r *PostgresScheduleRepo) ListDue(ctx context.Context) ([]domain.ExportSchedule, error) {
	query := `
		SELECT id, entity_id, schedule_name, template_id, format, parameters,
			   cron_expression, recipients, is_active, last_run_at, next_run_at,
			   created_by, created_at, updated_at
		FROM export.schedules
		WHERE is_active = true AND next_run_at <= NOW()
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var schedules []domain.ExportSchedule
	for rows.Next() {
		schedule, err := r.scanScheduleRow(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, *schedule)
	}

	return schedules, rows.Err()
}

func (r *PostgresScheduleRepo) scanSchedule(row rowScanner) (*domain.ExportSchedule, error) {
	var schedule domain.ExportSchedule
	var paramsJSON, recipientsJSON []byte

	err := row.Scan(
		&schedule.ID,
		&schedule.EntityID,
		&schedule.ScheduleName,
		&schedule.TemplateID,
		&schedule.Format,
		&paramsJSON,
		&schedule.CronExpression,
		&recipientsJSON,
		&schedule.IsActive,
		&schedule.LastRunAt,
		&schedule.NextRunAt,
		&schedule.CreatedBy,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrScheduleNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(paramsJSON, &schedule.Parameters); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(recipientsJSON, &schedule.Recipients); err != nil {
		return nil, err
	}

	return &schedule, nil
}

func (r *PostgresScheduleRepo) scanScheduleRow(rows *sql.Rows) (*domain.ExportSchedule, error) {
	var schedule domain.ExportSchedule
	var paramsJSON, recipientsJSON []byte

	err := rows.Scan(
		&schedule.ID,
		&schedule.EntityID,
		&schedule.ScheduleName,
		&schedule.TemplateID,
		&schedule.Format,
		&paramsJSON,
		&schedule.CronExpression,
		&recipientsJSON,
		&schedule.IsActive,
		&schedule.LastRunAt,
		&schedule.NextRunAt,
		&schedule.CreatedBy,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(paramsJSON, &schedule.Parameters); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(recipientsJSON, &schedule.Recipients); err != nil {
		return nil, err
	}

	return &schedule, nil
}
