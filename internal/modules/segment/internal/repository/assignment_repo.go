package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/platform/database"
)

type AssignmentRepository interface {
	Create(ctx context.Context, assignment *domain.Assignment) error
	Update(ctx context.Context, assignment *domain.Assignment) error
	GetByID(ctx context.Context, id common.ID) (*domain.Assignment, error)
	List(ctx context.Context, filter AssignmentFilter) ([]domain.Assignment, int, error)
	ListBySegment(ctx context.Context, segmentID common.ID) ([]domain.Assignment, error)
	ListByAssignment(ctx context.Context, assignmentType string, assignmentID common.ID) ([]domain.Assignment, error)
	ListEffective(ctx context.Context, entityID common.ID, assignmentType string, assignmentID common.ID, date time.Time) ([]domain.Assignment, error)
	Delete(ctx context.Context, id common.ID) error
	GetTotalAllocation(ctx context.Context, assignmentType string, assignmentID common.ID, excludeID *common.ID, effectiveDate time.Time) (float64, error)
}

type AssignmentFilter struct {
	EntityID       common.ID
	SegmentID      *common.ID
	AssignmentType string
	AssignmentID   *common.ID
	IsActive       *bool
	EffectiveDate  *time.Time
	Limit          int
	Offset         int
}

type PostgresAssignmentRepo struct {
	db *database.PostgresDB
}

func NewPostgresAssignmentRepo(db *database.PostgresDB) *PostgresAssignmentRepo {
	return &PostgresAssignmentRepo{db: db}
}

func (r *PostgresAssignmentRepo) Create(ctx context.Context, assignment *domain.Assignment) error {
	query := `
		INSERT INTO segment.assignments (
			id, entity_id, segment_id, assignment_type, assignment_id,
			allocation_percent, effective_from, effective_to, is_active,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := r.db.ExecContext(ctx, query,
		assignment.ID,
		assignment.EntityID,
		assignment.SegmentID,
		assignment.AssignmentType,
		assignment.AssignmentID,
		assignment.AllocationPercent,
		assignment.EffectiveFrom,
		assignment.EffectiveTo,
		assignment.IsActive,
		assignment.CreatedAt,
		assignment.UpdatedAt,
	)

	return err
}

func (r *PostgresAssignmentRepo) Update(ctx context.Context, assignment *domain.Assignment) error {
	query := `
		UPDATE segment.assignments SET
			segment_id = $2,
			allocation_percent = $3,
			effective_from = $4,
			effective_to = $5,
			is_active = $6,
			updated_at = $7
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		assignment.ID,
		assignment.SegmentID,
		assignment.AllocationPercent,
		assignment.EffectiveFrom,
		assignment.EffectiveTo,
		assignment.IsActive,
		assignment.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAssignmentNotFound
	}

	return nil
}

func (r *PostgresAssignmentRepo) GetByID(ctx context.Context, id common.ID) (*domain.Assignment, error) {
	query := `
		SELECT id, entity_id, segment_id, assignment_type, assignment_id,
			   allocation_percent, effective_from, effective_to, is_active,
			   created_at, updated_at
		FROM segment.assignments
		WHERE id = $1
	`

	return r.scanAssignment(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresAssignmentRepo) List(ctx context.Context, filter AssignmentFilter) ([]domain.Assignment, int, error) {
	baseQuery := `FROM segment.assignments WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if filter.SegmentID != nil {
		baseQuery += fmt.Sprintf(` AND segment_id = $%d`, argIdx)
		args = append(args, *filter.SegmentID)
		argIdx++
	}
	if filter.AssignmentType != "" {
		baseQuery += fmt.Sprintf(` AND assignment_type = $%d`, argIdx)
		args = append(args, filter.AssignmentType)
		argIdx++
	}
	if filter.AssignmentID != nil {
		baseQuery += fmt.Sprintf(` AND assignment_id = $%d`, argIdx)
		args = append(args, *filter.AssignmentID)
		argIdx++
	}
	if filter.IsActive != nil {
		baseQuery += fmt.Sprintf(` AND is_active = $%d`, argIdx)
		args = append(args, *filter.IsActive)
		argIdx++
	}
	if filter.EffectiveDate != nil {
		baseQuery += fmt.Sprintf(` AND effective_from <= $%d AND (effective_to IS NULL OR effective_to >= $%d)`, argIdx, argIdx)
		args = append(args, *filter.EffectiveDate)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, segment_id, assignment_type, assignment_id,
			   allocation_percent, effective_from, effective_to, is_active,
			   created_at, updated_at
		` + baseQuery + ` ORDER BY effective_from DESC, created_at DESC`

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
	defer rows.Close()

	var assignments []domain.Assignment
	for rows.Next() {
		assignment, err := r.scanAssignmentRow(rows)
		if err != nil {
			return nil, 0, err
		}
		assignments = append(assignments, *assignment)
	}

	return assignments, total, rows.Err()
}

func (r *PostgresAssignmentRepo) ListBySegment(ctx context.Context, segmentID common.ID) ([]domain.Assignment, error) {
	query := `
		SELECT id, entity_id, segment_id, assignment_type, assignment_id,
			   allocation_percent, effective_from, effective_to, is_active,
			   created_at, updated_at
		FROM segment.assignments
		WHERE segment_id = $1 AND is_active = true
		ORDER BY effective_from DESC
	`

	rows, err := r.db.QueryContext(ctx, query, segmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []domain.Assignment
	for rows.Next() {
		assignment, err := r.scanAssignmentRow(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, *assignment)
	}

	return assignments, rows.Err()
}

func (r *PostgresAssignmentRepo) ListByAssignment(ctx context.Context, assignmentType string, assignmentID common.ID) ([]domain.Assignment, error) {
	query := `
		SELECT id, entity_id, segment_id, assignment_type, assignment_id,
			   allocation_percent, effective_from, effective_to, is_active,
			   created_at, updated_at
		FROM segment.assignments
		WHERE assignment_type = $1 AND assignment_id = $2 AND is_active = true
		ORDER BY effective_from DESC
	`

	rows, err := r.db.QueryContext(ctx, query, assignmentType, assignmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []domain.Assignment
	for rows.Next() {
		assignment, err := r.scanAssignmentRow(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, *assignment)
	}

	return assignments, rows.Err()
}

func (r *PostgresAssignmentRepo) ListEffective(ctx context.Context, entityID common.ID, assignmentType string, assignmentID common.ID, date time.Time) ([]domain.Assignment, error) {
	query := `
		SELECT id, entity_id, segment_id, assignment_type, assignment_id,
			   allocation_percent, effective_from, effective_to, is_active,
			   created_at, updated_at
		FROM segment.assignments
		WHERE entity_id = $1 AND assignment_type = $2 AND assignment_id = $3
		  AND is_active = true
		  AND effective_from <= $4
		  AND (effective_to IS NULL OR effective_to >= $4)
		ORDER BY allocation_percent DESC
	`

	rows, err := r.db.QueryContext(ctx, query, entityID, assignmentType, assignmentID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []domain.Assignment
	for rows.Next() {
		assignment, err := r.scanAssignmentRow(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, *assignment)
	}

	return assignments, rows.Err()
}

func (r *PostgresAssignmentRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM segment.assignments WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAssignmentNotFound
	}

	return nil
}

func (r *PostgresAssignmentRepo) GetTotalAllocation(ctx context.Context, assignmentType string, assignmentID common.ID, excludeID *common.ID, effectiveDate time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(allocation_percent), 0)
		FROM segment.assignments
		WHERE assignment_type = $1 AND assignment_id = $2
		  AND is_active = true
		  AND effective_from <= $3
		  AND (effective_to IS NULL OR effective_to >= $3)
	`
	args := []any{assignmentType, assignmentID, effectiveDate}

	if excludeID != nil {
		query += ` AND id != $4`
		args = append(args, *excludeID)
	}

	var total float64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, err
	}

	return total, nil
}

func (r *PostgresAssignmentRepo) scanAssignment(row rowScanner) (*domain.Assignment, error) {
	var assignment domain.Assignment

	err := row.Scan(
		&assignment.ID,
		&assignment.EntityID,
		&assignment.SegmentID,
		&assignment.AssignmentType,
		&assignment.AssignmentID,
		&assignment.AllocationPercent,
		&assignment.EffectiveFrom,
		&assignment.EffectiveTo,
		&assignment.IsActive,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAssignmentNotFound
	}
	if err != nil {
		return nil, err
	}

	return &assignment, nil
}

func (r *PostgresAssignmentRepo) scanAssignmentRow(rows *sql.Rows) (*domain.Assignment, error) {
	var assignment domain.Assignment

	err := rows.Scan(
		&assignment.ID,
		&assignment.EntityID,
		&assignment.SegmentID,
		&assignment.AssignmentType,
		&assignment.AssignmentID,
		&assignment.AllocationPercent,
		&assignment.EffectiveFrom,
		&assignment.EffectiveTo,
		&assignment.IsActive,
		&assignment.CreatedAt,
		&assignment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &assignment, nil
}
