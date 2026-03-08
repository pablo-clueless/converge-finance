package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/shopspring/decimal"
)

type BalanceRepository interface {
	Upsert(ctx context.Context, balance *domain.SegmentBalance) error
	GetByID(ctx context.Context, id common.ID) (*domain.SegmentBalance, error)
	Get(ctx context.Context, entityID, segmentID, fiscalPeriodID, accountID common.ID) (*domain.SegmentBalance, error)
	ListBySegment(ctx context.Context, segmentID, fiscalPeriodID common.ID) ([]domain.SegmentBalance, error)
	ListByPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]domain.SegmentBalance, error)
	ListByAccount(ctx context.Context, accountID, fiscalPeriodID common.ID) ([]domain.SegmentBalance, error)
	GetSummaryBySegment(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]domain.SegmentBalanceSummary, error)
	Delete(ctx context.Context, id common.ID) error
	DeleteByPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID) error
}

type PostgresBalanceRepo struct {
	db *database.PostgresDB
}

func NewPostgresBalanceRepo(db *database.PostgresDB) *PostgresBalanceRepo {
	return &PostgresBalanceRepo{db: db}
}

func (r *PostgresBalanceRepo) Upsert(ctx context.Context, balance *domain.SegmentBalance) error {
	query := `
		INSERT INTO segment.balances (
			id, entity_id, segment_id, fiscal_period_id, account_id,
			debit_amount, credit_amount, net_amount, currency_code, last_updated
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (entity_id, segment_id, fiscal_period_id, account_id)
		DO UPDATE SET
			debit_amount = $6,
			credit_amount = $7,
			net_amount = $8,
			last_updated = $10
	`

	_, err := r.db.ExecContext(ctx, query,
		balance.ID,
		balance.EntityID,
		balance.SegmentID,
		balance.FiscalPeriodID,
		balance.AccountID,
		balance.DebitAmount,
		balance.CreditAmount,
		balance.NetAmount,
		balance.CurrencyCode,
		balance.LastUpdated,
	)

	return err
}

func (r *PostgresBalanceRepo) GetByID(ctx context.Context, id common.ID) (*domain.SegmentBalance, error) {
	query := `
		SELECT id, entity_id, segment_id, fiscal_period_id, account_id,
			   debit_amount, credit_amount, net_amount, currency_code, last_updated
		FROM segment.balances
		WHERE id = $1
	`

	return r.scanBalance(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresBalanceRepo) Get(ctx context.Context, entityID, segmentID, fiscalPeriodID, accountID common.ID) (*domain.SegmentBalance, error) {
	query := `
		SELECT id, entity_id, segment_id, fiscal_period_id, account_id,
			   debit_amount, credit_amount, net_amount, currency_code, last_updated
		FROM segment.balances
		WHERE entity_id = $1 AND segment_id = $2 AND fiscal_period_id = $3 AND account_id = $4
	`

	return r.scanBalance(r.db.QueryRowContext(ctx, query, entityID, segmentID, fiscalPeriodID, accountID))
}

func (r *PostgresBalanceRepo) ListBySegment(ctx context.Context, segmentID, fiscalPeriodID common.ID) ([]domain.SegmentBalance, error) {
	query := `
		SELECT id, entity_id, segment_id, fiscal_period_id, account_id,
			   debit_amount, credit_amount, net_amount, currency_code, last_updated
		FROM segment.balances
		WHERE segment_id = $1 AND fiscal_period_id = $2
		ORDER BY account_id
	`

	return r.listBalances(ctx, query, segmentID, fiscalPeriodID)
}

func (r *PostgresBalanceRepo) ListByPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]domain.SegmentBalance, error) {
	query := `
		SELECT id, entity_id, segment_id, fiscal_period_id, account_id,
			   debit_amount, credit_amount, net_amount, currency_code, last_updated
		FROM segment.balances
		WHERE entity_id = $1 AND fiscal_period_id = $2
		ORDER BY segment_id, account_id
	`

	return r.listBalances(ctx, query, entityID, fiscalPeriodID)
}

func (r *PostgresBalanceRepo) ListByAccount(ctx context.Context, accountID, fiscalPeriodID common.ID) ([]domain.SegmentBalance, error) {
	query := `
		SELECT id, entity_id, segment_id, fiscal_period_id, account_id,
			   debit_amount, credit_amount, net_amount, currency_code, last_updated
		FROM segment.balances
		WHERE account_id = $1 AND fiscal_period_id = $2
		ORDER BY segment_id
	`

	return r.listBalances(ctx, query, accountID, fiscalPeriodID)
}

func (r *PostgresBalanceRepo) GetSummaryBySegment(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]domain.SegmentBalanceSummary, error) {
	query := `
		SELECT
			b.segment_id,
			s.segment_code,
			s.segment_name,
			SUM(b.debit_amount) as total_debit,
			SUM(b.credit_amount) as total_credit,
			SUM(b.net_amount) as net_amount,
			COUNT(DISTINCT b.account_id) as account_count
		FROM segment.balances b
		JOIN segment.segments s ON b.segment_id = s.id
		WHERE b.entity_id = $1 AND b.fiscal_period_id = $2
		GROUP BY b.segment_id, s.segment_code, s.segment_name
		ORDER BY s.segment_code
	`

	rows, err := r.db.QueryContext(ctx, query, entityID, fiscalPeriodID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var summaries []domain.SegmentBalanceSummary
	for rows.Next() {
		var summary domain.SegmentBalanceSummary
		err := rows.Scan(
			&summary.SegmentID,
			&summary.SegmentCode,
			&summary.SegmentName,
			&summary.TotalDebit,
			&summary.TotalCredit,
			&summary.NetAmount,
			&summary.AccountCount,
		)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	return summaries, rows.Err()
}

func (r *PostgresBalanceRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM segment.balances WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrBalanceNotFound
	}

	return nil
}

func (r *PostgresBalanceRepo) DeleteByPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID) error {
	query := `DELETE FROM segment.balances WHERE entity_id = $1 AND fiscal_period_id = $2`
	_, err := r.db.ExecContext(ctx, query, entityID, fiscalPeriodID)
	return err
}

func (r *PostgresBalanceRepo) listBalances(ctx context.Context, query string, args ...any) ([]domain.SegmentBalance, error) {
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

	var balances []domain.SegmentBalance
	for rows.Next() {
		balance, err := r.scanBalanceRow(rows)
		if err != nil {
			return nil, err
		}
		balances = append(balances, *balance)
	}

	return balances, rows.Err()
}

func (r *PostgresBalanceRepo) scanBalance(row rowScanner) (*domain.SegmentBalance, error) {
	var balance domain.SegmentBalance

	err := row.Scan(
		&balance.ID,
		&balance.EntityID,
		&balance.SegmentID,
		&balance.FiscalPeriodID,
		&balance.AccountID,
		&balance.DebitAmount,
		&balance.CreditAmount,
		&balance.NetAmount,
		&balance.CurrencyCode,
		&balance.LastUpdated,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrBalanceNotFound
	}
	if err != nil {
		return nil, err
	}

	return &balance, nil
}

func (r *PostgresBalanceRepo) scanBalanceRow(rows *sql.Rows) (*domain.SegmentBalance, error) {
	var balance domain.SegmentBalance

	err := rows.Scan(
		&balance.ID,
		&balance.EntityID,
		&balance.SegmentID,
		&balance.FiscalPeriodID,
		&balance.AccountID,
		&balance.DebitAmount,
		&balance.CreditAmount,
		&balance.NetAmount,
		&balance.CurrencyCode,
		&balance.LastUpdated,
	)
	if err != nil {
		return nil, err
	}

	return &balance, nil
}

type AccountBalanceData struct {
	AccountID    common.ID
	AccountCode  string
	AccountName  string
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
	NetAmount    decimal.Decimal
	CurrencyCode string
}

type SegmentAllocationData struct {
	SegmentID         common.ID
	AllocationPercent decimal.Decimal
}

type BalanceCalculationProvider interface {
	GetAccountBalances(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]AccountBalanceData, error)
	GetSegmentAllocations(ctx context.Context, entityID common.ID, accountID common.ID, effectiveDate interface{}) ([]SegmentAllocationData, error)
}

func BuildBalanceUpsertQuery(balances []domain.SegmentBalance) (string, []any) {
	if len(balances) == 0 {
		return "", nil
	}

	query := `
		INSERT INTO segment.balances (
			id, entity_id, segment_id, fiscal_period_id, account_id,
			debit_amount, credit_amount, net_amount, currency_code, last_updated
		) VALUES
	`

	var args []any
	for i, balance := range balances {
		if i > 0 {
			query += ", "
		}
		offset := i * 10
		query += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			offset+1, offset+2, offset+3, offset+4, offset+5,
			offset+6, offset+7, offset+8, offset+9, offset+10)
		args = append(args,
			balance.ID,
			balance.EntityID,
			balance.SegmentID,
			balance.FiscalPeriodID,
			balance.AccountID,
			balance.DebitAmount,
			balance.CreditAmount,
			balance.NetAmount,
			balance.CurrencyCode,
			balance.LastUpdated,
		)
	}

	query += `
		ON CONFLICT (entity_id, segment_id, fiscal_period_id, account_id)
		DO UPDATE SET
			debit_amount = EXCLUDED.debit_amount,
			credit_amount = EXCLUDED.credit_amount,
			net_amount = EXCLUDED.net_amount,
			last_updated = EXCLUDED.last_updated
	`

	return query, args
}
