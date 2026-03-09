package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/platform/database"
)

type SegmentRepository interface {
	Create(ctx context.Context, segment *domain.Segment) error
	Update(ctx context.Context, segment *domain.Segment) error
	GetByID(ctx context.Context, id common.ID) (*domain.Segment, error)
	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Segment, error)
	List(ctx context.Context, filter SegmentFilter) ([]domain.Segment, int, error)
	Delete(ctx context.Context, id common.ID) error
	GetTree(ctx context.Context, entityID common.ID, segmentType domain.SegmentType) (*domain.SegmentTree, error)
	GetChildren(ctx context.Context, parentID common.ID) ([]domain.Segment, error)
}

type SegmentHierarchyRepository interface {
	Create(ctx context.Context, hierarchy *domain.SegmentHierarchy) error
	Update(ctx context.Context, hierarchy *domain.SegmentHierarchy) error
	GetByID(ctx context.Context, id common.ID) (*domain.SegmentHierarchy, error)
	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.SegmentHierarchy, error)
	GetPrimary(ctx context.Context, entityID common.ID, segmentType domain.SegmentType) (*domain.SegmentHierarchy, error)
	List(ctx context.Context, entityID common.ID, segmentType *domain.SegmentType) ([]domain.SegmentHierarchy, error)
	Delete(ctx context.Context, id common.ID) error
}

type IntersegmentTransactionRepository interface {
	Create(ctx context.Context, txn *domain.IntersegmentTransaction) error
	GetByID(ctx context.Context, id common.ID) (*domain.IntersegmentTransaction, error)
	List(ctx context.Context, filter IntersegmentFilter) ([]domain.IntersegmentTransaction, int, error)
	Eliminate(ctx context.Context, id common.ID) error
}

type SegmentFilter struct {
	EntityID     common.ID
	SegmentType  *domain.SegmentType
	ParentID     *common.ID
	IsReportable *bool
	IsActive     *bool
	Limit        int
	Offset       int
}

type IntersegmentFilter struct {
	EntityID       common.ID
	FiscalPeriodID common.ID
	FromSegmentID  *common.ID
	ToSegmentID    *common.ID
	IsEliminated   *bool
	Limit          int
	Offset         int
}

type rowScanner interface {
	Scan(dest ...any) error
}

type PostgresSegmentRepo struct {
	db *database.PostgresDB
}

func NewPostgresSegmentRepo(db *database.PostgresDB) *PostgresSegmentRepo {
	return &PostgresSegmentRepo{db: db}
}

func (r *PostgresSegmentRepo) Create(ctx context.Context, segment *domain.Segment) error {
	metadata, err := json.Marshal(segment.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO segment.segments (
			id, entity_id, segment_code, segment_name, segment_type,
			parent_id, description, manager_id, is_reportable, is_active,
			metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.db.ExecContext(ctx, query,
		segment.ID,
		segment.EntityID,
		segment.SegmentCode,
		segment.SegmentName,
		segment.SegmentType,
		segment.ParentID,
		segment.Description,
		segment.ManagerID,
		segment.IsReportable,
		segment.IsActive,
		metadata,
		segment.CreatedAt,
		segment.UpdatedAt,
	)

	return err
}

func (r *PostgresSegmentRepo) Update(ctx context.Context, segment *domain.Segment) error {
	metadata, err := json.Marshal(segment.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE segment.segments SET
			segment_name = $2,
			segment_type = $3,
			parent_id = $4,
			description = $5,
			manager_id = $6,
			is_reportable = $7,
			is_active = $8,
			metadata = $9,
			updated_at = $10
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		segment.ID,
		segment.SegmentName,
		segment.SegmentType,
		segment.ParentID,
		segment.Description,
		segment.ManagerID,
		segment.IsReportable,
		segment.IsActive,
		metadata,
		segment.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrSegmentNotFound
	}

	return nil
}

func (r *PostgresSegmentRepo) GetByID(ctx context.Context, id common.ID) (*domain.Segment, error) {
	query := `
		SELECT id, entity_id, segment_code, segment_name, segment_type,
			   parent_id, description, manager_id, is_reportable, is_active,
			   metadata, created_at, updated_at
		FROM segment.segments
		WHERE id = $1
	`

	return r.scanSegment(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresSegmentRepo) GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Segment, error) {
	query := `
		SELECT id, entity_id, segment_code, segment_name, segment_type,
			   parent_id, description, manager_id, is_reportable, is_active,
			   metadata, created_at, updated_at
		FROM segment.segments
		WHERE entity_id = $1 AND segment_code = $2
	`

	return r.scanSegment(r.db.QueryRowContext(ctx, query, entityID, code))
}

func (r *PostgresSegmentRepo) List(ctx context.Context, filter SegmentFilter) ([]domain.Segment, int, error) {
	baseQuery := `FROM segment.segments WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if filter.SegmentType != nil {
		baseQuery += fmt.Sprintf(` AND segment_type = $%d`, argIdx)
		args = append(args, *filter.SegmentType)
		argIdx++
	}
	if filter.ParentID != nil {
		baseQuery += fmt.Sprintf(` AND parent_id = $%d`, argIdx)
		args = append(args, *filter.ParentID)
		argIdx++
	}
	if filter.IsReportable != nil {
		baseQuery += fmt.Sprintf(` AND is_reportable = $%d`, argIdx)
		args = append(args, *filter.IsReportable)
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
		SELECT id, entity_id, segment_code, segment_name, segment_type,
			   parent_id, description, manager_id, is_reportable, is_active,
			   metadata, created_at, updated_at
		` + baseQuery + ` ORDER BY segment_code`

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

	var segments []domain.Segment
	for rows.Next() {
		segment, err := r.scanSegmentRow(rows)
		if err != nil {
			return nil, 0, err
		}
		segments = append(segments, *segment)
	}

	return segments, total, rows.Err()
}

func (r *PostgresSegmentRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM segment.segments WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrSegmentNotFound
	}

	return nil
}

func (r *PostgresSegmentRepo) GetTree(ctx context.Context, entityID common.ID, segmentType domain.SegmentType) (*domain.SegmentTree, error) {
	query := `
		SELECT id, entity_id, segment_code, segment_name, segment_type,
			   parent_id, description, manager_id, is_reportable, is_active,
			   metadata, created_at, updated_at
		FROM segment.segments
		WHERE entity_id = $1 AND segment_type = $2 AND is_active = true
		ORDER BY segment_code
	`

	rows, err := r.db.QueryContext(ctx, query, entityID, segmentType)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var allSegments []domain.Segment
	segmentMap := make(map[common.ID]*domain.Segment)

	for rows.Next() {
		segment, err := r.scanSegmentRow(rows)
		if err != nil {
			return nil, err
		}
		allSegments = append(allSegments, *segment)
		segmentMap[segment.ID] = segment
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var rootSegments []domain.Segment
	for i := range allSegments {
		if allSegments[i].ParentID == nil {
			rootSegments = append(rootSegments, allSegments[i])
		} else {
			if parent, ok := segmentMap[*allSegments[i].ParentID]; ok {
				parent.Children = append(parent.Children, allSegments[i])
			}
		}
	}

	return &domain.SegmentTree{
		Segments:    rootSegments,
		SegmentType: segmentType,
		TotalCount:  len(allSegments),
	}, nil
}

func (r *PostgresSegmentRepo) GetChildren(ctx context.Context, parentID common.ID) ([]domain.Segment, error) {
	query := `
		SELECT id, entity_id, segment_code, segment_name, segment_type,
			   parent_id, description, manager_id, is_reportable, is_active,
			   metadata, created_at, updated_at
		FROM segment.segments
		WHERE parent_id = $1 AND is_active = true
		ORDER BY segment_code
	`

	rows, err := r.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var segments []domain.Segment
	for rows.Next() {
		segment, err := r.scanSegmentRow(rows)
		if err != nil {
			return nil, err
		}
		segments = append(segments, *segment)
	}

	return segments, rows.Err()
}

func (r *PostgresSegmentRepo) scanSegment(row rowScanner) (*domain.Segment, error) {
	var segment domain.Segment
	var metadataJSON []byte

	err := row.Scan(
		&segment.ID,
		&segment.EntityID,
		&segment.SegmentCode,
		&segment.SegmentName,
		&segment.SegmentType,
		&segment.ParentID,
		&segment.Description,
		&segment.ManagerID,
		&segment.IsReportable,
		&segment.IsActive,
		&metadataJSON,
		&segment.CreatedAt,
		&segment.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSegmentNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &segment.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	} else {
		segment.Metadata = make(map[string]any)
	}

	segment.Children = []domain.Segment{}

	return &segment, nil
}

func (r *PostgresSegmentRepo) scanSegmentRow(rows *sql.Rows) (*domain.Segment, error) {
	var segment domain.Segment
	var metadataJSON []byte

	err := rows.Scan(
		&segment.ID,
		&segment.EntityID,
		&segment.SegmentCode,
		&segment.SegmentName,
		&segment.SegmentType,
		&segment.ParentID,
		&segment.Description,
		&segment.ManagerID,
		&segment.IsReportable,
		&segment.IsActive,
		&metadataJSON,
		&segment.CreatedAt,
		&segment.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &segment.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	} else {
		segment.Metadata = make(map[string]any)
	}

	segment.Children = []domain.Segment{}

	return &segment, nil
}

type PostgresSegmentHierarchyRepo struct {
	db *database.PostgresDB
}

func NewPostgresSegmentHierarchyRepo(db *database.PostgresDB) *PostgresSegmentHierarchyRepo {
	return &PostgresSegmentHierarchyRepo{db: db}
}

func (r *PostgresSegmentHierarchyRepo) Create(ctx context.Context, hierarchy *domain.SegmentHierarchy) error {
	query := `
		INSERT INTO segment.segment_hierarchy (
			id, entity_id, hierarchy_code, hierarchy_name, segment_type,
			description, is_primary, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		hierarchy.ID,
		hierarchy.EntityID,
		hierarchy.HierarchyCode,
		hierarchy.HierarchyName,
		hierarchy.SegmentType,
		hierarchy.Description,
		hierarchy.IsPrimary,
		hierarchy.IsActive,
		hierarchy.CreatedAt,
		hierarchy.UpdatedAt,
	)

	return err
}

func (r *PostgresSegmentHierarchyRepo) Update(ctx context.Context, hierarchy *domain.SegmentHierarchy) error {
	query := `
		UPDATE segment.segment_hierarchy SET
			hierarchy_name = $2,
			description = $3,
			is_primary = $4,
			is_active = $5,
			updated_at = $6
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		hierarchy.ID,
		hierarchy.HierarchyName,
		hierarchy.Description,
		hierarchy.IsPrimary,
		hierarchy.IsActive,
		hierarchy.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrHierarchyNotFound
	}

	return nil
}

func (r *PostgresSegmentHierarchyRepo) GetByID(ctx context.Context, id common.ID) (*domain.SegmentHierarchy, error) {
	query := `
		SELECT id, entity_id, hierarchy_code, hierarchy_name, segment_type,
			   description, is_primary, is_active, created_at, updated_at
		FROM segment.segment_hierarchy
		WHERE id = $1
	`

	return r.scanHierarchy(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresSegmentHierarchyRepo) GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.SegmentHierarchy, error) {
	query := `
		SELECT id, entity_id, hierarchy_code, hierarchy_name, segment_type,
			   description, is_primary, is_active, created_at, updated_at
		FROM segment.segment_hierarchy
		WHERE entity_id = $1 AND hierarchy_code = $2
	`

	return r.scanHierarchy(r.db.QueryRowContext(ctx, query, entityID, code))
}

func (r *PostgresSegmentHierarchyRepo) GetPrimary(ctx context.Context, entityID common.ID, segmentType domain.SegmentType) (*domain.SegmentHierarchy, error) {
	query := `
		SELECT id, entity_id, hierarchy_code, hierarchy_name, segment_type,
			   description, is_primary, is_active, created_at, updated_at
		FROM segment.segment_hierarchy
		WHERE entity_id = $1 AND segment_type = $2 AND is_primary = true AND is_active = true
	`

	return r.scanHierarchy(r.db.QueryRowContext(ctx, query, entityID, segmentType))
}

func (r *PostgresSegmentHierarchyRepo) List(ctx context.Context, entityID common.ID, segmentType *domain.SegmentType) ([]domain.SegmentHierarchy, error) {
	query := `
		SELECT id, entity_id, hierarchy_code, hierarchy_name, segment_type,
			   description, is_primary, is_active, created_at, updated_at
		FROM segment.segment_hierarchy
		WHERE entity_id = $1
	`
	args := []any{entityID}

	if segmentType != nil {
		query += ` AND segment_type = $2`
		args = append(args, *segmentType)
	}

	query += ` ORDER BY hierarchy_code`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var hierarchies []domain.SegmentHierarchy
	for rows.Next() {
		var hierarchy domain.SegmentHierarchy
		err := rows.Scan(
			&hierarchy.ID,
			&hierarchy.EntityID,
			&hierarchy.HierarchyCode,
			&hierarchy.HierarchyName,
			&hierarchy.SegmentType,
			&hierarchy.Description,
			&hierarchy.IsPrimary,
			&hierarchy.IsActive,
			&hierarchy.CreatedAt,
			&hierarchy.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		hierarchies = append(hierarchies, hierarchy)
	}

	return hierarchies, rows.Err()
}

func (r *PostgresSegmentHierarchyRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM segment.segment_hierarchy WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrHierarchyNotFound
	}

	return nil
}

func (r *PostgresSegmentHierarchyRepo) scanHierarchy(row rowScanner) (*domain.SegmentHierarchy, error) {
	var hierarchy domain.SegmentHierarchy

	err := row.Scan(
		&hierarchy.ID,
		&hierarchy.EntityID,
		&hierarchy.HierarchyCode,
		&hierarchy.HierarchyName,
		&hierarchy.SegmentType,
		&hierarchy.Description,
		&hierarchy.IsPrimary,
		&hierarchy.IsActive,
		&hierarchy.CreatedAt,
		&hierarchy.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrHierarchyNotFound
	}
	if err != nil {
		return nil, err
	}

	return &hierarchy, nil
}

type PostgresIntersegmentTransactionRepo struct {
	db *database.PostgresDB
}

func NewPostgresIntersegmentTransactionRepo(db *database.PostgresDB) *PostgresIntersegmentTransactionRepo {
	return &PostgresIntersegmentTransactionRepo{db: db}
}

func (r *PostgresIntersegmentTransactionRepo) Create(ctx context.Context, txn *domain.IntersegmentTransaction) error {
	query := `
		INSERT INTO segment.intersegment_transactions (
			id, entity_id, fiscal_period_id, from_segment_id, to_segment_id,
			journal_entry_id, transaction_date, description, amount, currency_code,
			is_eliminated, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.ExecContext(ctx, query,
		txn.ID,
		txn.EntityID,
		txn.FiscalPeriodID,
		txn.FromSegmentID,
		txn.ToSegmentID,
		txn.JournalEntryID,
		txn.TransactionDate,
		txn.Description,
		txn.Amount,
		txn.CurrencyCode,
		txn.IsEliminated,
		txn.CreatedAt,
	)

	return err
}

func (r *PostgresIntersegmentTransactionRepo) GetByID(ctx context.Context, id common.ID) (*domain.IntersegmentTransaction, error) {
	query := `
		SELECT id, entity_id, fiscal_period_id, from_segment_id, to_segment_id,
			   journal_entry_id, transaction_date, description, amount, currency_code,
			   is_eliminated, created_at
		FROM segment.intersegment_transactions
		WHERE id = $1
	`

	var txn domain.IntersegmentTransaction
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&txn.ID,
		&txn.EntityID,
		&txn.FiscalPeriodID,
		&txn.FromSegmentID,
		&txn.ToSegmentID,
		&txn.JournalEntryID,
		&txn.TransactionDate,
		&txn.Description,
		&txn.Amount,
		&txn.CurrencyCode,
		&txn.IsEliminated,
		&txn.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrIntersegmentTransactionNotFound
	}
	if err != nil {
		return nil, err
	}

	return &txn, nil
}

func (r *PostgresIntersegmentTransactionRepo) List(ctx context.Context, filter IntersegmentFilter) ([]domain.IntersegmentTransaction, int, error) {
	baseQuery := `FROM segment.intersegment_transactions WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if !filter.FiscalPeriodID.IsZero() {
		baseQuery += fmt.Sprintf(` AND fiscal_period_id = $%d`, argIdx)
		args = append(args, filter.FiscalPeriodID)
		argIdx++
	}
	if filter.FromSegmentID != nil {
		baseQuery += fmt.Sprintf(` AND from_segment_id = $%d`, argIdx)
		args = append(args, *filter.FromSegmentID)
		argIdx++
	}
	if filter.ToSegmentID != nil {
		baseQuery += fmt.Sprintf(` AND to_segment_id = $%d`, argIdx)
		args = append(args, *filter.ToSegmentID)
		argIdx++
	}
	if filter.IsEliminated != nil {
		baseQuery += fmt.Sprintf(` AND is_eliminated = $%d`, argIdx)
		args = append(args, *filter.IsEliminated)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, fiscal_period_id, from_segment_id, to_segment_id,
			   journal_entry_id, transaction_date, description, amount, currency_code,
			   is_eliminated, created_at
		` + baseQuery + ` ORDER BY transaction_date DESC, created_at DESC`

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

	var transactions []domain.IntersegmentTransaction
	for rows.Next() {
		var txn domain.IntersegmentTransaction
		err := rows.Scan(
			&txn.ID,
			&txn.EntityID,
			&txn.FiscalPeriodID,
			&txn.FromSegmentID,
			&txn.ToSegmentID,
			&txn.JournalEntryID,
			&txn.TransactionDate,
			&txn.Description,
			&txn.Amount,
			&txn.CurrencyCode,
			&txn.IsEliminated,
			&txn.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, txn)
	}

	return transactions, total, rows.Err()
}

func (r *PostgresIntersegmentTransactionRepo) Eliminate(ctx context.Context, id common.ID) error {
	query := `UPDATE segment.intersegment_transactions SET is_eliminated = true WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrIntersegmentTransactionNotFound
	}

	return nil
}
