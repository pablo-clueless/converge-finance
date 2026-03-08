package cost

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
	"converge-finance.com/m/internal/modules/cost/internal/service"
)

type apiImpl struct {
	costCenterService *service.CostCenterService
	allocationService *service.AllocationService
	budgetService     *service.BudgetService
}

func NewCostAPI(
	costCenterService *service.CostCenterService,
	allocationService *service.AllocationService,
	budgetService *service.BudgetService,
) API {
	return &apiImpl{
		costCenterService: costCenterService,
		allocationService: allocationService,
		budgetService:     budgetService,
	}
}

func (a *apiImpl) GetCostCenter(ctx context.Context, id common.ID) (*CostCenterResponse, error) {
	center, err := a.costCenterService.GetCostCenter(ctx, id)
	if err != nil {
		return nil, err
	}
	return toCostCenterResponse(center), nil
}

func (a *apiImpl) GetCostCenterByCode(ctx context.Context, entityID common.ID, code string) (*CostCenterResponse, error) {
	center, err := a.costCenterService.GetCostCenterByCode(ctx, entityID, code)
	if err != nil {
		return nil, err
	}
	return toCostCenterResponse(center), nil
}

func (a *apiImpl) ListCostCenters(ctx context.Context, entityID common.ID) ([]CostCenterResponse, error) {
	filter := domain.CostCenterFilter{EntityID: &entityID}
	centers, err := a.costCenterService.ListCostCenters(ctx, filter)
	if err != nil {
		return nil, err
	}

	response := make([]CostCenterResponse, len(centers))
	for i, c := range centers {
		response[i] = *toCostCenterResponse(&c)
	}
	return response, nil
}

func (a *apiImpl) ExecuteAllocation(ctx context.Context, entityID, fiscalPeriodID common.ID, date time.Time, currency money.Currency) (*AllocationRunResponse, error) {
	run, err := a.allocationService.InitiateAllocationRun(ctx, entityID, fiscalPeriodID, date, currency)
	if err != nil {
		return nil, err
	}

	if err := a.allocationService.ExecuteAllocation(ctx, run.ID); err != nil {
		return nil, err
	}

	run, err = a.allocationService.GetAllocationRun(ctx, run.ID)
	if err != nil {
		return nil, err
	}

	return toAllocationRunResponse(run), nil
}

func (a *apiImpl) GetBudget(ctx context.Context, id common.ID) (*BudgetResponse, error) {
	budget, err := a.budgetService.GetBudget(ctx, id)
	if err != nil {
		return nil, err
	}
	return toBudgetResponse(budget), nil
}

func (a *apiImpl) GetActiveBudget(ctx context.Context, entityID, fiscalYearID common.ID, budgetType string) (*BudgetResponse, error) {
	filter := domain.BudgetFilter{
		EntityID:     &entityID,
		FiscalYearID: &fiscalYearID,
		CurrentOnly:  true,
	}
	bt := domain.BudgetType(budgetType)
	filter.BudgetType = &bt
	status := domain.BudgetStatusActive
	filter.Status = &status

	budgets, err := a.budgetService.ListBudgets(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(budgets) == 0 {
		return nil, nil
	}

	return toBudgetResponse(&budgets[0]), nil
}

func (a *apiImpl) GetBudgetAmount(ctx context.Context, budgetID, accountID, fiscalPeriodID common.ID, costCenterID *common.ID) (money.Money, error) {
	lines, err := a.budgetService.GetBudgetLines(ctx, budgetID)
	if err != nil {
		return money.Money{}, err
	}

	for _, line := range lines {
		if line.AccountID == accountID && line.FiscalPeriodID == fiscalPeriodID {
			if costCenterID == nil && line.CostCenterID == nil {
				return line.BudgetAmount, nil
			}
			if costCenterID != nil && line.CostCenterID != nil && *costCenterID == *line.CostCenterID {
				return line.BudgetAmount, nil
			}
		}
	}

	budget, err := a.budgetService.GetBudget(ctx, budgetID)
	if err != nil {
		return money.Money{}, err
	}

	return money.Zero(budget.Currency), nil
}

func (a *apiImpl) GetVarianceAnalysis(ctx context.Context, budgetID, fiscalPeriodID common.ID) (*VarianceSummaryResponse, []VarianceResponse, error) {
	summary, analyses, err := a.budgetService.GetVarianceAnalysis(ctx, budgetID, fiscalPeriodID)
	if err != nil {
		return nil, nil, err
	}

	summaryResp := &VarianceSummaryResponse{
		EntityID:               summary.EntityID,
		FiscalPeriodID:         summary.FiscalPeriodID,
		BudgetID:               summary.BudgetID,
		BudgetName:             summary.BudgetName,
		TotalBudgetRevenue:     summary.TotalBudgetRevenue,
		TotalActualRevenue:     summary.TotalActualRevenue,
		RevenueVariance:        summary.RevenueVariance,
		RevenueVariancePercent: summary.RevenueVariancePercent,
		TotalBudgetExpenses:    summary.TotalBudgetExpenses,
		TotalActualExpenses:    summary.TotalActualExpenses,
		ExpenseVariance:        summary.ExpenseVariance,
		ExpenseVariancePercent: summary.ExpenseVariancePercent,
		BudgetNetIncome:        summary.BudgetNetIncome,
		ActualNetIncome:        summary.ActualNetIncome,
		NetIncomeVariance:      summary.NetIncomeVariance,
		FavorableVariances:     summary.FavorableVariances,
		UnfavorableVariances:   summary.UnfavorableVariances,
	}

	variances := make([]VarianceResponse, len(analyses))
	for i, a := range analyses {
		variances[i] = VarianceResponse{
			AccountID:       a.AccountID,
			AccountCode:     a.AccountCode,
			AccountName:     a.AccountName,
			CostCenterID:    a.CostCenterID,
			CostCenterCode:  a.CostCenterCode,
			BudgetAmount:    a.BudgetAmount,
			ActualAmount:    a.ActualAmount,
			VarianceAmount:  a.VarianceAmount,
			VariancePercent: a.VariancePercent,
			IsFavorable:     a.IsFavorable,
		}
	}

	return summaryResp, variances, nil
}

func toCostCenterResponse(c *domain.CostCenter) *CostCenterResponse {
	return &CostCenterResponse{
		ID:            c.ID,
		EntityID:      c.EntityID,
		Code:          c.Code,
		Name:          c.Name,
		CenterType:    string(c.CenterType),
		ParentID:      c.ParentID,
		ManagerID:     c.ManagerID,
		ManagerName:   c.ManagerName,
		Headcount:     c.Headcount,
		SquareFootage: c.SquareFootage,
		IsActive:      c.IsActive,
	}
}

func toAllocationRunResponse(r *domain.AllocationRun) *AllocationRunResponse {
	return &AllocationRunResponse{
		ID:             r.ID,
		RunNumber:      r.RunNumber,
		FiscalPeriodID: r.FiscalPeriodID,
		AllocationDate: r.AllocationDate,
		RulesExecuted:  r.RulesExecuted,
		TotalAllocated: r.TotalAllocated,
		Status:         string(r.Status),
		JournalEntryID: r.JournalEntryID,
	}
}

func toBudgetResponse(b *domain.Budget) *BudgetResponse {
	return &BudgetResponse{
		ID:               b.ID,
		EntityID:         b.EntityID,
		BudgetCode:       b.BudgetCode,
		BudgetName:       b.BudgetName,
		BudgetType:       string(b.BudgetType),
		FiscalYearID:     b.FiscalYearID,
		VersionNumber:    b.VersionNumber,
		IsCurrentVersion: b.IsCurrentVersion,
		Currency:         b.Currency.Code,
		TotalRevenue:     b.TotalRevenue,
		TotalExpenses:    b.TotalExpenses,
		NetBudget:        b.NetBudget,
		Status:           string(b.Status),
	}
}
