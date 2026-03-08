package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/platform/database"
)

type ReportRepository interface {
	Create(ctx context.Context, report *domain.SegmentReport) error
	Update(ctx context.Context, report *domain.SegmentReport) error
	GetByID(ctx context.Context, id common.ID) (*domain.SegmentReport, error)
	GetByReportNumber(ctx context.Context, entityID common.ID, reportNumber string) (*domain.SegmentReport, error)
	List(ctx context.Context, filter ReportFilter) ([]domain.SegmentReport, int, error)
	Delete(ctx context.Context, id common.ID) error
	GenerateReportNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)
}

type ReportDataRepository interface {
	CreateBatch(ctx context.Context, data []domain.ReportData) error
	GetByReportID(ctx context.Context, reportID common.ID) ([]domain.ReportData, error)
	DeleteByReportID(ctx context.Context, reportID common.ID) error
}

type ReportFilter struct {
	EntityID       common.ID
	FiscalPeriodID *common.ID
	FiscalYearID   *common.ID
	SegmentType    *domain.SegmentType
	Status         *domain.ReportStatus
	Limit          int
	Offset         int
}

type PostgresReportRepo struct {
	db *database.PostgresDB
}

func NewPostgresReportRepo(db *database.PostgresDB) *PostgresReportRepo {
	return &PostgresReportRepo{db: db}
}

func (r *PostgresReportRepo) Create(ctx context.Context, report *domain.SegmentReport) error {
	query := `
		INSERT INTO segment.reports (
			id, entity_id, report_number, report_name, fiscal_period_id,
			fiscal_year_id, as_of_date, segment_type, hierarchy_id,
			include_intersegment, currency_code, status, generated_by, generated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err := r.db.ExecContext(ctx, query,
		report.ID,
		report.EntityID,
		report.ReportNumber,
		report.ReportName,
		report.FiscalPeriodID,
		report.FiscalYearID,
		report.AsOfDate,
		report.SegmentType,
		report.HierarchyID,
		report.IncludeIntersegment,
		report.CurrencyCode,
		report.Status,
		report.GeneratedBy,
		report.GeneratedAt,
	)

	return err
}

func (r *PostgresReportRepo) Update(ctx context.Context, report *domain.SegmentReport) error {
	query := `
		UPDATE segment.reports SET
			report_name = $2,
			hierarchy_id = $3,
			include_intersegment = $4,
			status = $5
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		report.ID,
		report.ReportName,
		report.HierarchyID,
		report.IncludeIntersegment,
		report.Status,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrReportNotFound
	}

	return nil
}

func (r *PostgresReportRepo) GetByID(ctx context.Context, id common.ID) (*domain.SegmentReport, error) {
	query := `
		SELECT id, entity_id, report_number, report_name, fiscal_period_id,
			   fiscal_year_id, as_of_date, segment_type, hierarchy_id,
			   include_intersegment, currency_code, status, generated_by, generated_at
		FROM segment.reports
		WHERE id = $1
	`

	return r.scanReport(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresReportRepo) GetByReportNumber(ctx context.Context, entityID common.ID, reportNumber string) (*domain.SegmentReport, error) {
	query := `
		SELECT id, entity_id, report_number, report_name, fiscal_period_id,
			   fiscal_year_id, as_of_date, segment_type, hierarchy_id,
			   include_intersegment, currency_code, status, generated_by, generated_at
		FROM segment.reports
		WHERE entity_id = $1 AND report_number = $2
	`

	return r.scanReport(r.db.QueryRowContext(ctx, query, entityID, reportNumber))
}

func (r *PostgresReportRepo) List(ctx context.Context, filter ReportFilter) ([]domain.SegmentReport, int, error) {
	baseQuery := `FROM segment.reports WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if filter.FiscalPeriodID != nil {
		baseQuery += fmt.Sprintf(` AND fiscal_period_id = $%d`, argIdx)
		args = append(args, *filter.FiscalPeriodID)
		argIdx++
	}
	if filter.FiscalYearID != nil {
		baseQuery += fmt.Sprintf(` AND fiscal_year_id = $%d`, argIdx)
		args = append(args, *filter.FiscalYearID)
		argIdx++
	}
	if filter.SegmentType != nil {
		baseQuery += fmt.Sprintf(` AND segment_type = $%d`, argIdx)
		args = append(args, *filter.SegmentType)
		argIdx++
	}
	if filter.Status != nil {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, report_number, report_name, fiscal_period_id,
			   fiscal_year_id, as_of_date, segment_type, hierarchy_id,
			   include_intersegment, currency_code, status, generated_by, generated_at
		` + baseQuery + ` ORDER BY generated_at DESC`

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

	var reports []domain.SegmentReport
	for rows.Next() {
		report, err := r.scanReportRow(rows)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, *report)
	}

	return reports, total, rows.Err()
}

func (r *PostgresReportRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM segment.reports WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrReportNotFound
	}

	return nil
}

func (r *PostgresReportRepo) GenerateReportNumber(ctx context.Context, entityID common.ID, prefix string) (string, error) {
	query := `SELECT segment.generate_report_number($1, $2)`
	var reportNumber string
	err := r.db.QueryRowContext(ctx, query, entityID, prefix).Scan(&reportNumber)
	return reportNumber, err
}

func (r *PostgresReportRepo) scanReport(row rowScanner) (*domain.SegmentReport, error) {
	var report domain.SegmentReport

	err := row.Scan(
		&report.ID,
		&report.EntityID,
		&report.ReportNumber,
		&report.ReportName,
		&report.FiscalPeriodID,
		&report.FiscalYearID,
		&report.AsOfDate,
		&report.SegmentType,
		&report.HierarchyID,
		&report.IncludeIntersegment,
		&report.CurrencyCode,
		&report.Status,
		&report.GeneratedBy,
		&report.GeneratedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrReportNotFound
	}
	if err != nil {
		return nil, err
	}

	report.Data = []domain.ReportData{}

	return &report, nil
}

func (r *PostgresReportRepo) scanReportRow(rows *sql.Rows) (*domain.SegmentReport, error) {
	var report domain.SegmentReport

	err := rows.Scan(
		&report.ID,
		&report.EntityID,
		&report.ReportNumber,
		&report.ReportName,
		&report.FiscalPeriodID,
		&report.FiscalYearID,
		&report.AsOfDate,
		&report.SegmentType,
		&report.HierarchyID,
		&report.IncludeIntersegment,
		&report.CurrencyCode,
		&report.Status,
		&report.GeneratedBy,
		&report.GeneratedAt,
	)
	if err != nil {
		return nil, err
	}

	report.Data = []domain.ReportData{}

	return &report, nil
}

type PostgresReportDataRepo struct {
	db *database.PostgresDB
}

func NewPostgresReportDataRepo(db *database.PostgresDB) *PostgresReportDataRepo {
	return &PostgresReportDataRepo{db: db}
}

func (r *PostgresReportDataRepo) CreateBatch(ctx context.Context, data []domain.ReportData) error {
	if len(data) == 0 {
		return nil
	}

	query := `
		INSERT INTO segment.report_data (
			id, report_id, segment_id, row_type, line_item,
			amount, intersegment_amount, external_amount,
			percentage_of_total, row_order, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	for _, d := range data {
		_, err := r.db.ExecContext(ctx, query,
			d.ID,
			d.ReportID,
			d.SegmentID,
			d.RowType,
			d.LineItem,
			d.Amount,
			d.IntersegmentAmount,
			d.ExternalAmount,
			d.PercentageOfTotal,
			d.RowOrder,
			d.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresReportDataRepo) GetByReportID(ctx context.Context, reportID common.ID) ([]domain.ReportData, error) {
	query := `
		SELECT id, report_id, segment_id, row_type, line_item,
			   amount, intersegment_amount, external_amount,
			   percentage_of_total, row_order, created_at
		FROM segment.report_data
		WHERE report_id = $1
		ORDER BY segment_id, row_order
	`

	rows, err := r.db.QueryContext(ctx, query, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dataList []domain.ReportData
	for rows.Next() {
		var data domain.ReportData
		err := rows.Scan(
			&data.ID,
			&data.ReportID,
			&data.SegmentID,
			&data.RowType,
			&data.LineItem,
			&data.Amount,
			&data.IntersegmentAmount,
			&data.ExternalAmount,
			&data.PercentageOfTotal,
			&data.RowOrder,
			&data.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		dataList = append(dataList, data)
	}

	return dataList, rows.Err()
}

func (r *PostgresReportDataRepo) DeleteByReportID(ctx context.Context, reportID common.ID) error {
	query := `DELETE FROM segment.report_data WHERE report_id = $1`
	_, err := r.db.ExecContext(ctx, query, reportID)
	return err
}
