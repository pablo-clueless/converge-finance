package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
)

type ConsolidationRunRepository interface {
	WithTx(tx *sql.Tx) ConsolidationRunRepository

	Create(ctx context.Context, run *domain.ConsolidationRun) error
	Update(ctx context.Context, run *domain.ConsolidationRun) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.ConsolidationRun, error)
	GetByNumber(ctx context.Context, setID common.ID, runNumber string) (*domain.ConsolidationRun, error)
	GetLatest(ctx context.Context, setID common.ID) (*domain.ConsolidationRun, error)
	List(ctx context.Context, filter domain.ConsolidationRunFilter) ([]domain.ConsolidationRun, error)

	GetNextRunNumber(ctx context.Context, setID common.ID) (string, error)
}

type EntityBalanceRepository interface {
	WithTx(tx *sql.Tx) EntityBalanceRepository

	Create(ctx context.Context, balance *domain.EntityBalance) error
	CreateBatch(ctx context.Context, balances []domain.EntityBalance) error
	Delete(ctx context.Context, id common.ID) error
	DeleteByRun(ctx context.Context, runID common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.EntityBalance, error)
	GetByRun(ctx context.Context, runID common.ID) ([]domain.EntityBalance, error)
	GetByRunAndEntity(ctx context.Context, runID, entityID common.ID) ([]domain.EntityBalance, error)
	GetByRunAndAccount(ctx context.Context, runID, accountID common.ID) ([]domain.EntityBalance, error)
}

type ConsolidatedBalanceRepository interface {
	WithTx(tx *sql.Tx) ConsolidatedBalanceRepository

	Create(ctx context.Context, balance *domain.ConsolidatedBalance) error
	CreateBatch(ctx context.Context, balances []domain.ConsolidatedBalance) error
	Update(ctx context.Context, balance *domain.ConsolidatedBalance) error
	Delete(ctx context.Context, id common.ID) error
	DeleteByRun(ctx context.Context, runID common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.ConsolidatedBalance, error)
	GetByRun(ctx context.Context, runID common.ID) ([]domain.ConsolidatedBalance, error)
	GetByRunAndAccount(ctx context.Context, runID, accountID common.ID) (*domain.ConsolidatedBalance, error)

	GetTrialBalance(ctx context.Context, runID common.ID) ([]domain.ConsolidatedBalance, error)
}

type MinorityInterestRepository interface {
	WithTx(tx *sql.Tx) MinorityInterestRepository

	Create(ctx context.Context, mi *domain.MinorityInterest) error
	CreateBatch(ctx context.Context, mis []domain.MinorityInterest) error
	Update(ctx context.Context, mi *domain.MinorityInterest) error
	Delete(ctx context.Context, id common.ID) error
	DeleteByRun(ctx context.Context, runID common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.MinorityInterest, error)
	GetByRun(ctx context.Context, runID common.ID) ([]domain.MinorityInterest, error)
	GetByRunAndEntity(ctx context.Context, runID, entityID common.ID) (*domain.MinorityInterest, error)
}
