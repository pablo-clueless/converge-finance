package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/lib/pq"
)

// AccountCashFlowConfigRepository defines the interface for account cash flow config persistence
type AccountCashFlowConfigRepository interface {
	Create(ctx context.Context, config *domain.AccountCashFlowConfig) error
	Update(ctx context.Context, config *domain.AccountCashFlowConfig) error
	GetByID(ctx context.Context, id common.ID) (*domain.AccountCashFlowConfig, error)
	GetByAccountID(ctx context.Context, entityID, accountID common.ID) (*domain.AccountCashFlowConfig, error)
	ListByEntity(ctx context.Context, entityID common.ID) ([]domain.AccountCashFlowConfig, error)
	ListCashAccounts(ctx context.Context, entityID common.ID) ([]domain.AccountCashFlowConfig, error)
	Delete(ctx context.Context, id common.ID) error
}

// CashFlowTemplateRepository defines the interface for cash flow template persistence
type CashFlowTemplateRepository interface {
	Create(ctx context.Context, template *domain.CashFlowTemplate) error
	GetByID(ctx context.Context, id common.ID) (*domain.CashFlowTemplate, error)
	ListByEntity(ctx context.Context, entityID common.ID) ([]domain.CashFlowTemplate, error)
	ListSystemTemplates(ctx context.Context) ([]domain.CashFlowTemplate, error)
}

// CashFlowRunRepository defines the interface for cash flow run persistence
type CashFlowRunRepository interface {
	Create(ctx context.Context, run *domain.CashFlowRun) error
	Update(ctx context.Context, run *domain.CashFlowRun) error
	GetByID(ctx context.Context, id common.ID) (*domain.CashFlowRun, error)
	List(ctx context.Context, filter CashFlowRunFilter) ([]domain.CashFlowRun, int, error)
	GenerateRunNumber(ctx context.Context, entityID common.ID) (string, error)
}

// CashFlowLineRepository defines the interface for cash flow line persistence
type CashFlowLineRepository interface {
	CreateBatch(ctx context.Context, lines []domain.CashFlowLine) error
	GetByRunID(ctx context.Context, runID common.ID) ([]domain.CashFlowLine, error)
	DeleteByRunID(ctx context.Context, runID common.ID) error
}

// CashFlowRunFilter defines filtering options for cash flow run queries
type CashFlowRunFilter struct {
	EntityID       common.ID
	FiscalPeriodID common.ID
	FiscalYearID   common.ID
	Status         *domain.CashFlowRunStatus
	Limit          int
	Offset         int
}

// PostgresAccountCashFlowConfigRepo implements AccountCashFlowConfigRepository
type PostgresAccountCashFlowConfigRepo struct {
	db *database.PostgresDB
}

// NewPostgresAccountCashFlowConfigRepo creates a new repository
func NewPostgresAccountCashFlowConfigRepo(db *database.PostgresDB) *PostgresAccountCashFlowConfigRepo {
	return &PostgresAccountCashFlowConfigRepo{db: db}
}

func (r *PostgresAccountCashFlowConfigRepo) Create(ctx context.Context, config *domain.AccountCashFlowConfig) error {
	query := `
		INSERT INTO close.account_cashflow_config (
			id, entity_id, account_id, cashflow_category, line_item_code,
			is_cash_account, is_cash_equivalent, adjustment_type, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.EntityID,
		config.AccountID,
		config.CashFlowCategory,
		config.LineItemCode,
		config.IsCashAccount,
		config.IsCashEquivalent,
		config.AdjustmentType,
		config.CreatedAt,
		config.UpdatedAt,
	)

	return err
}

func (r *PostgresAccountCashFlowConfigRepo) Update(ctx context.Context, config *domain.AccountCashFlowConfig) error {
	query := `
		UPDATE close.account_cashflow_config SET
			cashflow_category = $2,
			line_item_code = $3,
			is_cash_account = $4,
			is_cash_equivalent = $5,
			adjustment_type = $6,
			updated_at = $7
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.CashFlowCategory,
		config.LineItemCode,
		config.IsCashAccount,
		config.IsCashEquivalent,
		config.AdjustmentType,
		config.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("account cash flow config not found")
	}

	return nil
}

func (r *PostgresAccountCashFlowConfigRepo) GetByID(ctx context.Context, id common.ID) (*domain.AccountCashFlowConfig, error) {
	query := `
		SELECT id, entity_id, account_id, cashflow_category, line_item_code,
			   is_cash_account, is_cash_equivalent, adjustment_type, created_at, updated_at
		FROM close.account_cashflow_config
		WHERE id = $1
	`

	var config domain.AccountCashFlowConfig
	var adjustmentType sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&config.ID,
		&config.EntityID,
		&config.AccountID,
		&config.CashFlowCategory,
		&config.LineItemCode,
		&config.IsCashAccount,
		&config.IsCashEquivalent,
		&adjustmentType,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("account cash flow config not found")
	}
	if err != nil {
		return nil, err
	}

	if adjustmentType.Valid {
		config.AdjustmentType = adjustmentType.String
	}

	return &config, nil
}

func (r *PostgresAccountCashFlowConfigRepo) GetByAccountID(ctx context.Context, entityID, accountID common.ID) (*domain.AccountCashFlowConfig, error) {
	query := `
		SELECT id, entity_id, account_id, cashflow_category, line_item_code,
			   is_cash_account, is_cash_equivalent, adjustment_type, created_at, updated_at
		FROM close.account_cashflow_config
		WHERE entity_id = $1 AND account_id = $2
	`

	var config domain.AccountCashFlowConfig
	var adjustmentType sql.NullString

	err := r.db.QueryRowContext(ctx, query, entityID, accountID).Scan(
		&config.ID,
		&config.EntityID,
		&config.AccountID,
		&config.CashFlowCategory,
		&config.LineItemCode,
		&config.IsCashAccount,
		&config.IsCashEquivalent,
		&adjustmentType,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("account cash flow config not found")
	}
	if err != nil {
		return nil, err
	}

	if adjustmentType.Valid {
		config.AdjustmentType = adjustmentType.String
	}

	return &config, nil
}

func (r *PostgresAccountCashFlowConfigRepo) ListByEntity(ctx context.Context, entityID common.ID) ([]domain.AccountCashFlowConfig, error) {
	query := `
		SELECT id, entity_id, account_id, cashflow_category, line_item_code,
			   is_cash_account, is_cash_equivalent, adjustment_type, created_at, updated_at
		FROM close.account_cashflow_config
		WHERE entity_id = $1
		ORDER BY cashflow_category, line_item_code
	`

	rows, err := r.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []domain.AccountCashFlowConfig
	for rows.Next() {
		var config domain.AccountCashFlowConfig
		var adjustmentType sql.NullString

		err := rows.Scan(
			&config.ID,
			&config.EntityID,
			&config.AccountID,
			&config.CashFlowCategory,
			&config.LineItemCode,
			&config.IsCashAccount,
			&config.IsCashEquivalent,
			&adjustmentType,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if adjustmentType.Valid {
			config.AdjustmentType = adjustmentType.String
		}

		configs = append(configs, config)
	}

	return configs, rows.Err()
}

func (r *PostgresAccountCashFlowConfigRepo) ListCashAccounts(ctx context.Context, entityID common.ID) ([]domain.AccountCashFlowConfig, error) {
	query := `
		SELECT id, entity_id, account_id, cashflow_category, line_item_code,
			   is_cash_account, is_cash_equivalent, adjustment_type, created_at, updated_at
		FROM close.account_cashflow_config
		WHERE entity_id = $1 AND (is_cash_account = true OR is_cash_equivalent = true)
		ORDER BY line_item_code
	`

	rows, err := r.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []domain.AccountCashFlowConfig
	for rows.Next() {
		var config domain.AccountCashFlowConfig
		var adjustmentType sql.NullString

		err := rows.Scan(
			&config.ID,
			&config.EntityID,
			&config.AccountID,
			&config.CashFlowCategory,
			&config.LineItemCode,
			&config.IsCashAccount,
			&config.IsCashEquivalent,
			&adjustmentType,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if adjustmentType.Valid {
			config.AdjustmentType = adjustmentType.String
		}

		configs = append(configs, config)
	}

	return configs, rows.Err()
}

func (r *PostgresAccountCashFlowConfigRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM close.account_cashflow_config WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("account cash flow config not found")
	}

	return nil
}

// PostgresCashFlowTemplateRepo implements CashFlowTemplateRepository
type PostgresCashFlowTemplateRepo struct {
	db *database.PostgresDB
}

// NewPostgresCashFlowTemplateRepo creates a new repository
func NewPostgresCashFlowTemplateRepo(db *database.PostgresDB) *PostgresCashFlowTemplateRepo {
	return &PostgresCashFlowTemplateRepo{db: db}
}

func (r *PostgresCashFlowTemplateRepo) Create(ctx context.Context, template *domain.CashFlowTemplate) error {
	query := `
		INSERT INTO close.cashflow_templates (
			id, entity_id, template_code, template_name, method,
			configuration, is_system, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
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
		template.Method,
		configJSON,
		template.IsSystem,
		template.IsActive,
		template.CreatedAt,
		template.UpdatedAt,
	)

	return err
}

func (r *PostgresCashFlowTemplateRepo) GetByID(ctx context.Context, id common.ID) (*domain.CashFlowTemplate, error) {
	query := `
		SELECT id, entity_id, template_code, template_name, method,
			   configuration, is_system, is_active, created_at, updated_at
		FROM close.cashflow_templates
		WHERE id = $1
	`

	var template domain.CashFlowTemplate
	var configJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&template.ID,
		&template.EntityID,
		&template.TemplateCode,
		&template.TemplateName,
		&template.Method,
		&configJSON,
		&template.IsSystem,
		&template.IsActive,
		&template.CreatedAt,
		&template.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("cash flow template not found")
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &template.Configuration); err != nil {
		return nil, err
	}

	return &template, nil
}

func (r *PostgresCashFlowTemplateRepo) ListByEntity(ctx context.Context, entityID common.ID) ([]domain.CashFlowTemplate, error) {
	query := `
		SELECT id, entity_id, template_code, template_name, method,
			   configuration, is_system, is_active, created_at, updated_at
		FROM close.cashflow_templates
		WHERE (entity_id = $1 OR is_system = true) AND is_active = true
		ORDER BY is_system DESC, template_name
	`

	return r.scanTemplates(r.db.QueryContext(ctx, query, entityID))
}

func (r *PostgresCashFlowTemplateRepo) ListSystemTemplates(ctx context.Context) ([]domain.CashFlowTemplate, error) {
	query := `
		SELECT id, entity_id, template_code, template_name, method,
			   configuration, is_system, is_active, created_at, updated_at
		FROM close.cashflow_templates
		WHERE is_system = true AND is_active = true
		ORDER BY template_name
	`

	return r.scanTemplates(r.db.QueryContext(ctx, query))
}

func (r *PostgresCashFlowTemplateRepo) scanTemplates(rows *sql.Rows, err error) ([]domain.CashFlowTemplate, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []domain.CashFlowTemplate
	for rows.Next() {
		var template domain.CashFlowTemplate
		var configJSON []byte

		err := rows.Scan(
			&template.ID,
			&template.EntityID,
			&template.TemplateCode,
			&template.TemplateName,
			&template.Method,
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

		templates = append(templates, template)
	}

	return templates, rows.Err()
}

// PostgresCashFlowRunRepo implements CashFlowRunRepository
type PostgresCashFlowRunRepo struct {
	db *database.PostgresDB
}

// NewPostgresCashFlowRunRepo creates a new repository
func NewPostgresCashFlowRunRepo(db *database.PostgresDB) *PostgresCashFlowRunRepo {
	return &PostgresCashFlowRunRepo{db: db}
}

func (r *PostgresCashFlowRunRepo) Create(ctx context.Context, run *domain.CashFlowRun) error {
	query := `
		INSERT INTO close.cashflow_runs (
			id, entity_id, run_number, template_id, fiscal_period_id, fiscal_year_id,
			method, period_start, period_end, currency_code,
			operating_net, investing_net, financing_net, net_change,
			opening_cash, closing_cash, fx_effect, status,
			generated_by, generated_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
	`

	_, err := r.db.ExecContext(ctx, query,
		run.ID,
		run.EntityID,
		run.RunNumber,
		run.TemplateID,
		run.FiscalPeriodID,
		run.FiscalYearID,
		run.Method,
		run.PeriodStart,
		run.PeriodEnd,
		run.CurrencyCode,
		run.OperatingNet,
		run.InvestingNet,
		run.FinancingNet,
		run.NetChange,
		run.OpeningCash,
		run.ClosingCash,
		run.FXEffect,
		run.Status,
		run.GeneratedBy,
		run.GeneratedAt,
		run.CreatedAt,
		run.UpdatedAt,
	)

	return err
}

func (r *PostgresCashFlowRunRepo) Update(ctx context.Context, run *domain.CashFlowRun) error {
	query := `
		UPDATE close.cashflow_runs SET
			operating_net = $2,
			investing_net = $3,
			financing_net = $4,
			net_change = $5,
			opening_cash = $6,
			closing_cash = $7,
			fx_effect = $8,
			status = $9,
			generated_at = $10,
			updated_at = $11
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		run.ID,
		run.OperatingNet,
		run.InvestingNet,
		run.FinancingNet,
		run.NetChange,
		run.OpeningCash,
		run.ClosingCash,
		run.FXEffect,
		run.Status,
		run.GeneratedAt,
		run.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("cash flow run not found")
	}

	return nil
}

func (r *PostgresCashFlowRunRepo) GetByID(ctx context.Context, id common.ID) (*domain.CashFlowRun, error) {
	query := `
		SELECT id, entity_id, run_number, template_id, fiscal_period_id, fiscal_year_id,
			   method, period_start, period_end, currency_code,
			   operating_net, investing_net, financing_net, net_change,
			   opening_cash, closing_cash, fx_effect, status,
			   generated_by, generated_at, created_at, updated_at
		FROM close.cashflow_runs
		WHERE id = $1
	`

	var run domain.CashFlowRun

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&run.ID,
		&run.EntityID,
		&run.RunNumber,
		&run.TemplateID,
		&run.FiscalPeriodID,
		&run.FiscalYearID,
		&run.Method,
		&run.PeriodStart,
		&run.PeriodEnd,
		&run.CurrencyCode,
		&run.OperatingNet,
		&run.InvestingNet,
		&run.FinancingNet,
		&run.NetChange,
		&run.OpeningCash,
		&run.ClosingCash,
		&run.FXEffect,
		&run.Status,
		&run.GeneratedBy,
		&run.GeneratedAt,
		&run.CreatedAt,
		&run.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("cash flow run not found")
	}
	if err != nil {
		return nil, err
	}

	return &run, nil
}

func (r *PostgresCashFlowRunRepo) List(ctx context.Context, filter CashFlowRunFilter) ([]domain.CashFlowRun, int, error) {
	baseQuery := `FROM close.cashflow_runs WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if !filter.FiscalPeriodID.IsZero() {
		baseQuery += fmt.Sprintf(` AND fiscal_period_id = $%d`, argIdx)
		args = append(args, filter.FiscalPeriodID)
		argIdx++
	}
	if !filter.FiscalYearID.IsZero() {
		baseQuery += fmt.Sprintf(` AND fiscal_year_id = $%d`, argIdx)
		args = append(args, filter.FiscalYearID)
		argIdx++
	}
	if filter.Status != nil {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}

	// Count query
	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query
	dataQuery := `
		SELECT id, entity_id, run_number, template_id, fiscal_period_id, fiscal_year_id,
			   method, period_start, period_end, currency_code,
			   operating_net, investing_net, financing_net, net_change,
			   opening_cash, closing_cash, fx_effect, status,
			   generated_by, generated_at, created_at, updated_at
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
	defer rows.Close()

	var runs []domain.CashFlowRun
	for rows.Next() {
		var run domain.CashFlowRun

		err := rows.Scan(
			&run.ID,
			&run.EntityID,
			&run.RunNumber,
			&run.TemplateID,
			&run.FiscalPeriodID,
			&run.FiscalYearID,
			&run.Method,
			&run.PeriodStart,
			&run.PeriodEnd,
			&run.CurrencyCode,
			&run.OperatingNet,
			&run.InvestingNet,
			&run.FinancingNet,
			&run.NetChange,
			&run.OpeningCash,
			&run.ClosingCash,
			&run.FXEffect,
			&run.Status,
			&run.GeneratedBy,
			&run.GeneratedAt,
			&run.CreatedAt,
			&run.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		runs = append(runs, run)
	}

	return runs, total, rows.Err()
}

func (r *PostgresCashFlowRunRepo) GenerateRunNumber(ctx context.Context, entityID common.ID) (string, error) {
	query := `SELECT close.generate_cashflow_run_number($1, 'CF')`
	var runNumber string
	err := r.db.QueryRowContext(ctx, query, entityID).Scan(&runNumber)
	return runNumber, err
}

// PostgresCashFlowLineRepo implements CashFlowLineRepository
type PostgresCashFlowLineRepo struct {
	db *database.PostgresDB
}

// NewPostgresCashFlowLineRepo creates a new repository
func NewPostgresCashFlowLineRepo(db *database.PostgresDB) *PostgresCashFlowLineRepo {
	return &PostgresCashFlowLineRepo{db: db}
}

func (r *PostgresCashFlowLineRepo) CreateBatch(ctx context.Context, lines []domain.CashFlowLine) error {
	if len(lines) == 0 {
		return nil
	}

	query := `
		INSERT INTO close.cashflow_lines (
			id, cashflow_run_id, line_number, category, line_type,
			line_code, description, amount, indent_level, is_bold,
			source_accounts, calculation, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	for _, line := range lines {
		var calculation sql.NullString
		if line.Calculation != "" {
			calculation = sql.NullString{String: line.Calculation, Valid: true}
		}

		_, err := r.db.ExecContext(ctx, query,
			line.ID,
			line.CashFlowRunID,
			line.LineNumber,
			line.Category,
			line.LineType,
			line.LineCode,
			line.Description,
			line.Amount,
			line.IndentLevel,
			line.IsBold,
			pq.Array(line.SourceAccounts),
			calculation,
			line.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresCashFlowLineRepo) GetByRunID(ctx context.Context, runID common.ID) ([]domain.CashFlowLine, error) {
	query := `
		SELECT id, cashflow_run_id, line_number, category, line_type,
			   line_code, description, amount, indent_level, is_bold,
			   source_accounts, calculation, created_at
		FROM close.cashflow_lines
		WHERE cashflow_run_id = $1
		ORDER BY line_number
	`

	rows, err := r.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []domain.CashFlowLine
	for rows.Next() {
		var line domain.CashFlowLine
		var calculation sql.NullString

		err := rows.Scan(
			&line.ID,
			&line.CashFlowRunID,
			&line.LineNumber,
			&line.Category,
			&line.LineType,
			&line.LineCode,
			&line.Description,
			&line.Amount,
			&line.IndentLevel,
			&line.IsBold,
			pq.Array(&line.SourceAccounts),
			&calculation,
			&line.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if calculation.Valid {
			line.Calculation = calculation.String
		}

		lines = append(lines, line)
	}

	return lines, rows.Err()
}

func (r *PostgresCashFlowLineRepo) DeleteByRunID(ctx context.Context, runID common.ID) error {
	query := `DELETE FROM close.cashflow_lines WHERE cashflow_run_id = $1`
	_, err := r.db.ExecContext(ctx, query, runID)
	return err
}
