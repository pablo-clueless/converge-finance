package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
)

type AllocationRuleRepository interface {
	WithTx(tx *sql.Tx) AllocationRuleRepository

	Create(ctx context.Context, rule *domain.AllocationRule) error
	Update(ctx context.Context, rule *domain.AllocationRule) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.AllocationRule, error)
	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.AllocationRule, error)
	List(ctx context.Context, filter domain.AllocationRuleFilter) ([]domain.AllocationRule, error)

	GetActiveByEntity(ctx context.Context, entityID common.ID) ([]domain.AllocationRule, error)
	GetBySourceCostCenter(ctx context.Context, sourceCostCenterID common.ID) ([]domain.AllocationRule, error)

	AddTarget(ctx context.Context, target *domain.AllocationTarget) error
	UpdateTarget(ctx context.Context, target *domain.AllocationTarget) error
	RemoveTarget(ctx context.Context, ruleID, targetCostCenterID common.ID) error
	GetTargets(ctx context.Context, ruleID common.ID) ([]domain.AllocationTarget, error)
}

type AllocationRunRepository interface {
	WithTx(tx *sql.Tx) AllocationRunRepository

	Create(ctx context.Context, run *domain.AllocationRun) error
	Update(ctx context.Context, run *domain.AllocationRun) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.AllocationRun, error)
	GetByNumber(ctx context.Context, entityID common.ID, runNumber string) (*domain.AllocationRun, error)
	GetLatest(ctx context.Context, entityID common.ID) (*domain.AllocationRun, error)
	List(ctx context.Context, filter domain.AllocationRunFilter) ([]domain.AllocationRun, error)

	GetNextRunNumber(ctx context.Context, entityID common.ID) (string, error)

	CreateEntry(ctx context.Context, entry *domain.AllocationEntry) error
	CreateEntries(ctx context.Context, entries []domain.AllocationEntry) error
	GetEntries(ctx context.Context, runID common.ID) ([]domain.AllocationEntry, error)
	DeleteEntries(ctx context.Context, runID common.ID) error
}
