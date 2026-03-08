package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
)

type BudgetRepository interface {
	WithTx(tx *sql.Tx) BudgetRepository

	Create(ctx context.Context, budget *domain.Budget) error
	Update(ctx context.Context, budget *domain.Budget) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.Budget, error)
	GetByCode(ctx context.Context, entityID common.ID, code string, version int) (*domain.Budget, error)
	GetCurrentVersion(ctx context.Context, entityID common.ID, code string) (*domain.Budget, error)
	List(ctx context.Context, filter domain.BudgetFilter) ([]domain.Budget, error)

	GetVersionHistory(ctx context.Context, budgetCode string, entityID common.ID) ([]domain.Budget, error)
	GetActiveBudget(ctx context.Context, entityID, fiscalYearID common.ID, budgetType domain.BudgetType) (*domain.Budget, error)
}

type BudgetLineRepository interface {
	WithTx(tx *sql.Tx) BudgetLineRepository

	Create(ctx context.Context, line *domain.BudgetLine) error
	CreateBatch(ctx context.Context, lines []domain.BudgetLine) error
	Update(ctx context.Context, line *domain.BudgetLine) error
	Delete(ctx context.Context, id common.ID) error
	DeleteByBudget(ctx context.Context, budgetID common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.BudgetLine, error)
	GetByBudget(ctx context.Context, budgetID common.ID) ([]domain.BudgetLine, error)
	GetByBudgetAndPeriod(ctx context.Context, budgetID, fiscalPeriodID common.ID) ([]domain.BudgetLine, error)
	GetByBudgetAccountAndPeriod(ctx context.Context, budgetID, accountID, fiscalPeriodID common.ID, costCenterID *common.ID) (*domain.BudgetLine, error)
	List(ctx context.Context, filter domain.BudgetLineFilter) ([]domain.BudgetLine, error)

	CopyFromBudget(ctx context.Context, sourceBudgetID, targetBudgetID common.ID) error
}

type BudgetTransferRepository interface {
	WithTx(tx *sql.Tx) BudgetTransferRepository

	Create(ctx context.Context, transfer *domain.BudgetTransfer) error
	Update(ctx context.Context, transfer *domain.BudgetTransfer) error

	GetByID(ctx context.Context, id common.ID) (*domain.BudgetTransfer, error)
	GetByBudget(ctx context.Context, budgetID common.ID) ([]domain.BudgetTransfer, error)
	GetPendingApproval(ctx context.Context, budgetID common.ID) ([]domain.BudgetTransfer, error)

	GetNextTransferNumber(ctx context.Context, budgetID common.ID) (string, error)
}

type BudgetActualRepository interface {
	WithTx(tx *sql.Tx) BudgetActualRepository

	Upsert(ctx context.Context, actual *domain.BudgetActual) error
	UpsertBatch(ctx context.Context, actuals []domain.BudgetActual) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.BudgetActual, error)
	Get(ctx context.Context, entityID, accountID, fiscalPeriodID common.ID, costCenterID *common.ID) (*domain.BudgetActual, error)
	List(ctx context.Context, filter domain.BudgetActualFilter) ([]domain.BudgetActual, error)

	GetByPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]domain.BudgetActual, error)
	RefreshFromGL(ctx context.Context, entityID, fiscalPeriodID common.ID) error
}
