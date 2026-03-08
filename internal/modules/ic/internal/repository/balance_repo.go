package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
)

type BalanceRepository interface {
	Create(ctx context.Context, balance *domain.EntityPairBalance) error

	Update(ctx context.Context, balance *domain.EntityPairBalance) error

	GetByID(ctx context.Context, id common.ID) (*domain.EntityPairBalance, error)

	GetByEntityPair(ctx context.Context, fromEntityID, toEntityID common.ID, fiscalPeriodID common.ID) (*domain.EntityPairBalance, error)

	GetByEntityPairForUpdate(ctx context.Context, dbTx *sql.Tx, fromEntityID, toEntityID common.ID, fiscalPeriodID common.ID) (*domain.EntityPairBalance, error)

	GetOrCreate(ctx context.Context, fromEntityID, toEntityID common.ID, fiscalPeriodID common.ID, currencyCode string) (*domain.EntityPairBalance, error)

	List(ctx context.Context, filter domain.EntityPairBalanceFilter) ([]domain.EntityPairBalance, error)

	Count(ctx context.Context, filter domain.EntityPairBalanceFilter) (int64, error)

	GetAllForEntity(ctx context.Context, entityID common.ID, fiscalPeriodID common.ID) ([]domain.EntityPairBalance, error)

	GetUnreconciled(ctx context.Context, parentEntityID common.ID, fiscalPeriodID common.ID) ([]domain.EntityPairBalance, error)

	GetWithDiscrepancies(ctx context.Context, parentEntityID common.ID, fiscalPeriodID common.ID) ([]domain.EntityPairBalance, error)

	RecalculateBalances(ctx context.Context, fromEntityID, toEntityID common.ID, fiscalPeriodID common.ID) error

	RecalculateAllBalances(ctx context.Context, fiscalPeriodID common.ID) error

	CarryForwardBalances(ctx context.Context, fromPeriodID, toPeriodID common.ID) error

	GetDiscrepancies(ctx context.Context, parentEntityID common.ID, fiscalPeriodID common.ID) ([]domain.ReconciliationDiscrepancy, error)

	GetReconciliationSummary(ctx context.Context, parentEntityID common.ID, fiscalPeriodID common.ID) (*domain.ReconciliationSummary, error)

	MarkReconciled(ctx context.Context, id common.ID) error

	WithTx(tx *sql.Tx) BalanceRepository
}
