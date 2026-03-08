package cost

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type API interface {
	// Cost Centers
	GetCostCenter(ctx context.Context, id common.ID) (*CostCenterResponse, error)
	GetCostCenterByCode(ctx context.Context, entityID common.ID, code string) (*CostCenterResponse, error)
	ListCostCenters(ctx context.Context, entityID common.ID) ([]CostCenterResponse, error)

	// Allocations
	ExecuteAllocation(ctx context.Context, entityID, fiscalPeriodID common.ID, date time.Time, currency money.Currency) (*AllocationRunResponse, error)

	// Budgets
	GetBudget(ctx context.Context, id common.ID) (*BudgetResponse, error)
	GetActiveBudget(ctx context.Context, entityID, fiscalYearID common.ID, budgetType string) (*BudgetResponse, error)
	GetBudgetAmount(ctx context.Context, budgetID, accountID, fiscalPeriodID common.ID, costCenterID *common.ID) (money.Money, error)

	// Variance Analysis
	GetVarianceAnalysis(ctx context.Context, budgetID, fiscalPeriodID common.ID) (*VarianceSummaryResponse, []VarianceResponse, error)
}

type CostCenterResponse struct {
	ID            common.ID
	EntityID      common.ID
	Code          string
	Name          string
	CenterType    string
	ParentID      *common.ID
	ManagerID     *common.ID
	ManagerName   string
	Headcount     int
	SquareFootage float64
	IsActive      bool
}

type AllocationRunResponse struct {
	ID             common.ID
	RunNumber      string
	FiscalPeriodID common.ID
	AllocationDate time.Time
	RulesExecuted  int
	TotalAllocated money.Money
	Status         string
	JournalEntryID *common.ID
}

type BudgetResponse struct {
	ID               common.ID
	EntityID         common.ID
	BudgetCode       string
	BudgetName       string
	BudgetType       string
	FiscalYearID     common.ID
	VersionNumber    int
	IsCurrentVersion bool
	Currency         string
	TotalRevenue     money.Money
	TotalExpenses    money.Money
	NetBudget        money.Money
	Status           string
}

type VarianceSummaryResponse struct {
	EntityID               common.ID
	FiscalPeriodID         common.ID
	BudgetID               common.ID
	BudgetName             string
	TotalBudgetRevenue     money.Money
	TotalActualRevenue     money.Money
	RevenueVariance        money.Money
	RevenueVariancePercent float64
	TotalBudgetExpenses    money.Money
	TotalActualExpenses    money.Money
	ExpenseVariance        money.Money
	ExpenseVariancePercent float64
	BudgetNetIncome        money.Money
	ActualNetIncome        money.Money
	NetIncomeVariance      money.Money
	FavorableVariances     int
	UnfavorableVariances   int
}

type VarianceResponse struct {
	AccountID       common.ID
	AccountCode     string
	AccountName     string
	CostCenterID    *common.ID
	CostCenterCode  string
	BudgetAmount    money.Money
	ActualAmount    money.Money
	VarianceAmount  money.Money
	VariancePercent float64
	IsFavorable     bool
}
