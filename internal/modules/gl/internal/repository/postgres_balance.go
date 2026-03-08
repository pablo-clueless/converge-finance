package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
)

type PostgresBalanceRepository struct {
	db *sql.DB
}

func NewPostgresBalanceRepository(db *sql.DB) *PostgresBalanceRepository {
	return &PostgresBalanceRepository{db: db}
}

func (r *PostgresBalanceRepository) GetByAccountAndPeriod(ctx context.Context, accountID, periodID common.ID) (*AccountBalance, error) {
	return nil, nil
}

func (r *PostgresBalanceRepository) GetByPeriod(ctx context.Context, periodID common.ID) ([]AccountBalance, error) {
	return []AccountBalance{}, nil
}

func (r *PostgresBalanceRepository) UpsertBalance(ctx context.Context, balance *AccountBalance) error {
	return nil
}

func (r *PostgresBalanceRepository) RecalculateBalance(ctx context.Context, accountID, periodID common.ID) error {
	return nil
}

func (r *PostgresBalanceRepository) RollForward(ctx context.Context, fromPeriodID, toPeriodID common.ID) error {
	return nil
}

func (r *PostgresBalanceRepository) WithTx(tx *sql.Tx) AccountBalanceRepository {
	return r
}
