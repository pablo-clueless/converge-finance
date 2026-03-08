package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fx/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/shopspring/decimal"
)

type AccountFXConfigRepository interface {
	Create(ctx context.Context, config *domain.AccountFXConfig) error
	Update(ctx context.Context, config *domain.AccountFXConfig) error
	GetByID(ctx context.Context, id common.ID) (*domain.AccountFXConfig, error)
	GetByAccountID(ctx context.Context, entityID, accountID common.ID) (*domain.AccountFXConfig, error)
	ListByEntity(ctx context.Context, entityID common.ID, treatment *domain.AccountFXTreatment) ([]domain.AccountFXConfig, error)
	Delete(ctx context.Context, id common.ID) error
}

type RevaluationRunRepository interface {
	Create(ctx context.Context, run *domain.RevaluationRun) error
	Update(ctx context.Context, run *domain.RevaluationRun) error
	GetByID(ctx context.Context, id common.ID) (*domain.RevaluationRun, error)
	GetByRunNumber(ctx context.Context, entityID common.ID, runNumber string) (*domain.RevaluationRun, error)
	List(ctx context.Context, filter RevaluationRunFilter) ([]domain.RevaluationRun, int, error)
	Delete(ctx context.Context, id common.ID) error
	GenerateRunNumber(ctx context.Context, entityID common.ID) (string, error)
}

type RevaluationDetailRepository interface {
	CreateBatch(ctx context.Context, details []domain.RevaluationDetail) error
	GetByRunID(ctx context.Context, runID common.ID) ([]domain.RevaluationDetail, error)
	DeleteByRunID(ctx context.Context, runID common.ID) error
}

type RevaluationRunFilter struct {
	EntityID       common.ID
	FiscalPeriodID common.ID
	Status         *domain.RevaluationStatus
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

type PostgresAccountFXConfigRepo struct {
	db *database.PostgresDB
}

func NewPostgresAccountFXConfigRepo(db *database.PostgresDB) *PostgresAccountFXConfigRepo {
	return &PostgresAccountFXConfigRepo{db: db}
}

func (r *PostgresAccountFXConfigRepo) Create(ctx context.Context, config *domain.AccountFXConfig) error {
	query := `
		INSERT INTO fx.account_fx_config (
			id, entity_id, account_id, fx_treatment,
			revaluation_gain_account_id, revaluation_loss_account_id,
			is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.EntityID,
		config.AccountID,
		config.FXTreatment,
		config.RevaluationGainAccountID,
		config.RevaluationLossAccountID,
		config.IsActive,
		config.CreatedAt,
		config.UpdatedAt,
	)

	return err
}

func (r *PostgresAccountFXConfigRepo) Update(ctx context.Context, config *domain.AccountFXConfig) error {
	query := `
		UPDATE fx.account_fx_config SET
			fx_treatment = $2,
			revaluation_gain_account_id = $3,
			revaluation_loss_account_id = $4,
			is_active = $5,
			updated_at = $6
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		config.ID,
		config.FXTreatment,
		config.RevaluationGainAccountID,
		config.RevaluationLossAccountID,
		config.IsActive,
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
		return domain.ErrAccountFXConfigNotFound
	}

	return nil
}

func (r *PostgresAccountFXConfigRepo) GetByID(ctx context.Context, id common.ID) (*domain.AccountFXConfig, error) {
	query := `
		SELECT id, entity_id, account_id, fx_treatment,
			   revaluation_gain_account_id, revaluation_loss_account_id,
			   is_active, created_at, updated_at
		FROM fx.account_fx_config
		WHERE id = $1
	`

	return r.scanConfig(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresAccountFXConfigRepo) GetByAccountID(ctx context.Context, entityID, accountID common.ID) (*domain.AccountFXConfig, error) {
	query := `
		SELECT id, entity_id, account_id, fx_treatment,
			   revaluation_gain_account_id, revaluation_loss_account_id,
			   is_active, created_at, updated_at
		FROM fx.account_fx_config
		WHERE entity_id = $1 AND account_id = $2
	`

	return r.scanConfig(r.db.QueryRowContext(ctx, query, entityID, accountID))
}

func (r *PostgresAccountFXConfigRepo) ListByEntity(ctx context.Context, entityID common.ID, treatment *domain.AccountFXTreatment) ([]domain.AccountFXConfig, error) {
	query := `
		SELECT id, entity_id, account_id, fx_treatment,
			   revaluation_gain_account_id, revaluation_loss_account_id,
			   is_active, created_at, updated_at
		FROM fx.account_fx_config
		WHERE entity_id = $1 AND is_active = true
	`

	args := []any{entityID}
	if treatment != nil {
		query += ` AND fx_treatment = $2`
		args = append(args, *treatment)
	}

	query += ` ORDER BY created_at`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("error closing rows: %v", err)
		}
	}()

	var configs []domain.AccountFXConfig
	for rows.Next() {
		config, err := r.scanConfigRow(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *config)
	}

	return configs, rows.Err()
}

func (r *PostgresAccountFXConfigRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM fx.account_fx_config WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrAccountFXConfigNotFound
	}

	return nil
}

func (r *PostgresAccountFXConfigRepo) scanConfig(row rowScanner) (*domain.AccountFXConfig, error) {
	var config domain.AccountFXConfig

	err := row.Scan(
		&config.ID,
		&config.EntityID,
		&config.AccountID,
		&config.FXTreatment,
		&config.RevaluationGainAccountID,
		&config.RevaluationLossAccountID,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAccountFXConfigNotFound
	}
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (r *PostgresAccountFXConfigRepo) scanConfigRow(rows *sql.Rows) (*domain.AccountFXConfig, error) {
	var config domain.AccountFXConfig

	err := rows.Scan(
		&config.ID,
		&config.EntityID,
		&config.AccountID,
		&config.FXTreatment,
		&config.RevaluationGainAccountID,
		&config.RevaluationLossAccountID,
		&config.IsActive,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

type PostgresRevaluationRunRepo struct {
	db *database.PostgresDB
}

func NewPostgresRevaluationRunRepo(db *database.PostgresDB) *PostgresRevaluationRunRepo {
	return &PostgresRevaluationRunRepo{db: db}
}

func (r *PostgresRevaluationRunRepo) Create(ctx context.Context, run *domain.RevaluationRun) error {
	query := `
		INSERT INTO fx.revaluation_runs (
			id, entity_id, run_number, fiscal_period_id, revaluation_date, rate_date,
			functional_currency, status, total_unrealized_gain, total_unrealized_loss,
			net_revaluation, accounts_processed, journal_entry_id, reversal_journal_entry_id,
			created_by, approved_by, posted_by, reversed_by,
			created_at, updated_at, approved_at, posted_at, reversed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
	`

	_, err := r.db.ExecContext(ctx, query,
		run.ID,
		run.EntityID,
		run.RunNumber,
		run.FiscalPeriodID,
		run.RevaluationDate,
		run.RateDate,
		run.FunctionalCurrency.Code,
		run.Status,
		run.TotalUnrealizedGain,
		run.TotalUnrealizedLoss,
		run.NetRevaluation,
		run.AccountsProcessed,
		run.JournalEntryID,
		run.ReversalJournalEntryID,
		run.CreatedBy,
		run.ApprovedBy,
		run.PostedBy,
		run.ReversedBy,
		run.CreatedAt,
		run.UpdatedAt,
		run.ApprovedAt,
		run.PostedAt,
		run.ReversedAt,
	)

	return err
}

func (r *PostgresRevaluationRunRepo) Update(ctx context.Context, run *domain.RevaluationRun) error {
	query := `
		UPDATE fx.revaluation_runs SET
			status = $2,
			total_unrealized_gain = $3,
			total_unrealized_loss = $4,
			net_revaluation = $5,
			accounts_processed = $6,
			journal_entry_id = $7,
			reversal_journal_entry_id = $8,
			approved_by = $9,
			posted_by = $10,
			reversed_by = $11,
			updated_at = $12,
			approved_at = $13,
			posted_at = $14,
			reversed_at = $15
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		run.ID,
		run.Status,
		run.TotalUnrealizedGain,
		run.TotalUnrealizedLoss,
		run.NetRevaluation,
		run.AccountsProcessed,
		run.JournalEntryID,
		run.ReversalJournalEntryID,
		run.ApprovedBy,
		run.PostedBy,
		run.ReversedBy,
		run.UpdatedAt,
		run.ApprovedAt,
		run.PostedAt,
		run.ReversedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrRevaluationRunNotFound
	}

	return nil
}

func (r *PostgresRevaluationRunRepo) GetByID(ctx context.Context, id common.ID) (*domain.RevaluationRun, error) {
	query := `
		SELECT id, entity_id, run_number, fiscal_period_id, revaluation_date, rate_date,
			   functional_currency, status, total_unrealized_gain, total_unrealized_loss,
			   net_revaluation, accounts_processed, journal_entry_id, reversal_journal_entry_id,
			   created_by, approved_by, posted_by, reversed_by,
			   created_at, updated_at, approved_at, posted_at, reversed_at
		FROM fx.revaluation_runs
		WHERE id = $1
	`

	return r.scanRun(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresRevaluationRunRepo) GetByRunNumber(ctx context.Context, entityID common.ID, runNumber string) (*domain.RevaluationRun, error) {
	query := `
		SELECT id, entity_id, run_number, fiscal_period_id, revaluation_date, rate_date,
			   functional_currency, status, total_unrealized_gain, total_unrealized_loss,
			   net_revaluation, accounts_processed, journal_entry_id, reversal_journal_entry_id,
			   created_by, approved_by, posted_by, reversed_by,
			   created_at, updated_at, approved_at, posted_at, reversed_at
		FROM fx.revaluation_runs
		WHERE entity_id = $1 AND run_number = $2
	`

	return r.scanRun(r.db.QueryRowContext(ctx, query, entityID, runNumber))
}

func (r *PostgresRevaluationRunRepo) List(ctx context.Context, filter RevaluationRunFilter) ([]domain.RevaluationRun, int, error) {
	baseQuery := `FROM fx.revaluation_runs WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if !filter.FiscalPeriodID.IsZero() {
		baseQuery += fmt.Sprintf(` AND fiscal_period_id = $%d`, argIdx)
		args = append(args, filter.FiscalPeriodID)
		argIdx++
	}
	if filter.Status != nil {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.DateFrom != nil {
		baseQuery += fmt.Sprintf(` AND revaluation_date >= $%d`, argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		baseQuery += fmt.Sprintf(` AND revaluation_date <= $%d`, argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, run_number, fiscal_period_id, revaluation_date, rate_date,
			   functional_currency, status, total_unrealized_gain, total_unrealized_loss,
			   net_revaluation, accounts_processed, journal_entry_id, reversal_journal_entry_id,
			   created_by, approved_by, posted_by, reversed_by,
			   created_at, updated_at, approved_at, posted_at, reversed_at
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
			log.Printf("error closing rows: %v", err)
		}
	}()

	var runs []domain.RevaluationRun
	for rows.Next() {
		run, err := r.scanRunRow(rows)
		if err != nil {
			return nil, 0, err
		}
		runs = append(runs, *run)
	}

	return runs, total, rows.Err()
}

func (r *PostgresRevaluationRunRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM fx.revaluation_runs WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrRevaluationRunNotFound
	}

	return nil
}

func (r *PostgresRevaluationRunRepo) GenerateRunNumber(ctx context.Context, entityID common.ID) (string, error) {
	query := `SELECT fx.generate_revaluation_run_number($1)`
	var runNumber string
	err := r.db.QueryRowContext(ctx, query, entityID).Scan(&runNumber)
	return runNumber, err
}

func (r *PostgresRevaluationRunRepo) scanRun(row rowScanner) (*domain.RevaluationRun, error) {
	var run domain.RevaluationRun
	var functionalCurrencyCode string

	err := row.Scan(
		&run.ID,
		&run.EntityID,
		&run.RunNumber,
		&run.FiscalPeriodID,
		&run.RevaluationDate,
		&run.RateDate,
		&functionalCurrencyCode,
		&run.Status,
		&run.TotalUnrealizedGain,
		&run.TotalUnrealizedLoss,
		&run.NetRevaluation,
		&run.AccountsProcessed,
		&run.JournalEntryID,
		&run.ReversalJournalEntryID,
		&run.CreatedBy,
		&run.ApprovedBy,
		&run.PostedBy,
		&run.ReversedBy,
		&run.CreatedAt,
		&run.UpdatedAt,
		&run.ApprovedAt,
		&run.PostedAt,
		&run.ReversedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrRevaluationRunNotFound
	}
	if err != nil {
		return nil, err
	}

	run.FunctionalCurrency = money.MustGetCurrency(functionalCurrencyCode)

	return &run, nil
}

func (r *PostgresRevaluationRunRepo) scanRunRow(rows *sql.Rows) (*domain.RevaluationRun, error) {
	var run domain.RevaluationRun
	var functionalCurrencyCode string

	err := rows.Scan(
		&run.ID,
		&run.EntityID,
		&run.RunNumber,
		&run.FiscalPeriodID,
		&run.RevaluationDate,
		&run.RateDate,
		&functionalCurrencyCode,
		&run.Status,
		&run.TotalUnrealizedGain,
		&run.TotalUnrealizedLoss,
		&run.NetRevaluation,
		&run.AccountsProcessed,
		&run.JournalEntryID,
		&run.ReversalJournalEntryID,
		&run.CreatedBy,
		&run.ApprovedBy,
		&run.PostedBy,
		&run.ReversedBy,
		&run.CreatedAt,
		&run.UpdatedAt,
		&run.ApprovedAt,
		&run.PostedAt,
		&run.ReversedAt,
	)
	if err != nil {
		return nil, err
	}

	run.FunctionalCurrency = money.MustGetCurrency(functionalCurrencyCode)

	return &run, nil
}

type PostgresRevaluationDetailRepo struct {
	db *database.PostgresDB
}

func NewPostgresRevaluationDetailRepo(db *database.PostgresDB) *PostgresRevaluationDetailRepo {
	return &PostgresRevaluationDetailRepo{db: db}
}

func (r *PostgresRevaluationDetailRepo) CreateBatch(ctx context.Context, details []domain.RevaluationDetail) error {
	if len(details) == 0 {
		return nil
	}

	query := `
		INSERT INTO fx.revaluation_details (
			id, revaluation_run_id, account_id, account_code, account_name,
			original_currency, original_balance, original_rate, original_functional_amount,
			new_rate, new_functional_amount, revaluation_amount, revaluation_type,
			gain_loss_account_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	for _, detail := range details {
		_, err := r.db.ExecContext(ctx, query,
			detail.ID,
			detail.RevaluationRunID,
			detail.AccountID,
			detail.AccountCode,
			detail.AccountName,
			detail.OriginalCurrency.Code,
			detail.OriginalBalance,
			detail.OriginalRate,
			detail.OriginalFunctionalAmount,
			detail.NewRate,
			detail.NewFunctionalAmount,
			detail.RevaluationAmount,
			detail.RevaluationType,
			detail.GainLossAccountID,
			detail.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *PostgresRevaluationDetailRepo) GetByRunID(ctx context.Context, runID common.ID) ([]domain.RevaluationDetail, error) {
	query := `
		SELECT id, revaluation_run_id, account_id, account_code, account_name,
			   original_currency, original_balance, original_rate, original_functional_amount,
			   new_rate, new_functional_amount, revaluation_amount, revaluation_type,
			   gain_loss_account_id, created_at
		FROM fx.revaluation_details
		WHERE revaluation_run_id = $1
		ORDER BY account_code
	`

	rows, err := r.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("error closing rows: %v", err)
		}
	}()

	var details []domain.RevaluationDetail
	for rows.Next() {
		var detail domain.RevaluationDetail
		var originalCurrencyCode string

		err := rows.Scan(
			&detail.ID,
			&detail.RevaluationRunID,
			&detail.AccountID,
			&detail.AccountCode,
			&detail.AccountName,
			&originalCurrencyCode,
			&detail.OriginalBalance,
			&detail.OriginalRate,
			&detail.OriginalFunctionalAmount,
			&detail.NewRate,
			&detail.NewFunctionalAmount,
			&detail.RevaluationAmount,
			&detail.RevaluationType,
			&detail.GainLossAccountID,
			&detail.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		detail.OriginalCurrency = money.MustGetCurrency(originalCurrencyCode)
		details = append(details, detail)
	}

	return details, rows.Err()
}

func (r *PostgresRevaluationDetailRepo) DeleteByRunID(ctx context.Context, runID common.ID) error {
	query := `DELETE FROM fx.revaluation_details WHERE revaluation_run_id = $1`
	_, err := r.db.ExecContext(ctx, query, runID)
	return err
}

type AccountBalance struct {
	AccountID              common.ID
	AccountCode            string
	AccountName            string
	CurrencyCode           string
	Balance                decimal.Decimal
	OriginalFunctionalRate decimal.Decimal
}
