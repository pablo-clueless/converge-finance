package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/export/internal/domain"
	"converge-finance.com/m/internal/platform/database"
)

type TemplateRepository interface {
	Create(ctx context.Context, template *domain.ExportTemplate) error
	Update(ctx context.Context, template *domain.ExportTemplate) error
	GetByID(ctx context.Context, id common.ID) (*domain.ExportTemplate, error)
	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.ExportTemplate, error)
	List(ctx context.Context, filter TemplateFilter) ([]domain.ExportTemplate, int, error)
	Delete(ctx context.Context, id common.ID) error
}

type TemplateFilter struct {
	EntityID     common.ID
	Module       string
	ExportType   string
	IsActive     *bool
	IncludeSystem bool
	Limit        int
	Offset       int
}

type PostgresTemplateRepo struct {
	db *database.PostgresDB
}

func NewPostgresTemplateRepo(db *database.PostgresDB) *PostgresTemplateRepo {
	return &PostgresTemplateRepo{db: db}
}

func (r *PostgresTemplateRepo) Create(ctx context.Context, template *domain.ExportTemplate) error {
	query := `
		INSERT INTO export.templates (
			id, entity_id, template_code, template_name, module, export_type,
			configuration, is_system, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	configJSON, err := json.Marshal(template.Configuration)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, query,
		template.ID,
		template.EntityID,
		template.TemplateCode,
		template.TemplateName,
		template.Module,
		template.ExportType,
		configJSON,
		template.IsSystem,
		template.IsActive,
		template.CreatedAt,
		template.UpdatedAt,
	)

	return err
}

func (r *PostgresTemplateRepo) Update(ctx context.Context, template *domain.ExportTemplate) error {
	query := `
		UPDATE export.templates SET
			template_name = $2,
			configuration = $3,
			is_active = $4,
			updated_at = $5
		WHERE id = $1
	`

	configJSON, err := json.Marshal(template.Configuration)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, query,
		template.ID,
		template.TemplateName,
		configJSON,
		template.IsActive,
		template.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrTemplateNotFound
	}

	return nil
}

func (r *PostgresTemplateRepo) GetByID(ctx context.Context, id common.ID) (*domain.ExportTemplate, error) {
	query := `
		SELECT id, entity_id, template_code, template_name, module, export_type,
			   configuration, is_system, is_active, created_at, updated_at
		FROM export.templates
		WHERE id = $1
	`

	return r.scanTemplate(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresTemplateRepo) GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.ExportTemplate, error) {
	query := `
		SELECT id, entity_id, template_code, template_name, module, export_type,
			   configuration, is_system, is_active, created_at, updated_at
		FROM export.templates
		WHERE (entity_id = $1 OR is_system = true) AND template_code = $2
		ORDER BY is_system ASC
		LIMIT 1
	`

	return r.scanTemplate(r.db.QueryRowContext(ctx, query, entityID, code))
}

func (r *PostgresTemplateRepo) List(ctx context.Context, filter TemplateFilter) ([]domain.ExportTemplate, int, error) {
	baseQuery := `FROM export.templates WHERE (entity_id = $1 OR is_system = true)`
	args := []any{filter.EntityID}
	argIdx := 2

	if !filter.IncludeSystem {
		baseQuery = `FROM export.templates WHERE entity_id = $1`
	}

	if filter.Module != "" {
		baseQuery += ` AND module = $` + itoa(argIdx)
		args = append(args, filter.Module)
		argIdx++
	}
	if filter.ExportType != "" {
		baseQuery += ` AND export_type = $` + itoa(argIdx)
		args = append(args, filter.ExportType)
		argIdx++
	}
	if filter.IsActive != nil {
		baseQuery += ` AND is_active = $` + itoa(argIdx)
		args = append(args, *filter.IsActive)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, template_code, template_name, module, export_type,
			   configuration, is_system, is_active, created_at, updated_at
		` + baseQuery + ` ORDER BY is_system DESC, template_name`

	if filter.Limit > 0 {
		dataQuery += ` LIMIT $` + itoa(argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		dataQuery += ` OFFSET $` + itoa(argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var templates []domain.ExportTemplate
	for rows.Next() {
		template, err := r.scanTemplateRow(rows)
		if err != nil {
			return nil, 0, err
		}
		templates = append(templates, *template)
	}

	return templates, total, rows.Err()
}

func (r *PostgresTemplateRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM export.templates WHERE id = $1 AND is_system = false`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrTemplateNotFound
	}

	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *PostgresTemplateRepo) scanTemplate(row rowScanner) (*domain.ExportTemplate, error) {
	var template domain.ExportTemplate
	var configJSON []byte

	err := row.Scan(
		&template.ID,
		&template.EntityID,
		&template.TemplateCode,
		&template.TemplateName,
		&template.Module,
		&template.ExportType,
		&configJSON,
		&template.IsSystem,
		&template.IsActive,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &template.Configuration); err != nil {
		return nil, err
	}

	return &template, nil
}

func (r *PostgresTemplateRepo) scanTemplateRow(rows *sql.Rows) (*domain.ExportTemplate, error) {
	var template domain.ExportTemplate
	var configJSON []byte

	err := rows.Scan(
		&template.ID,
		&template.EntityID,
		&template.TemplateCode,
		&template.TemplateName,
		&template.Module,
		&template.ExportType,
		&configJSON,
		&template.IsSystem,
		&template.IsActive,
		&template.CreatedAt,
		&template.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &template.Configuration); err != nil {
		return nil, err
	}

	return &template, nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
