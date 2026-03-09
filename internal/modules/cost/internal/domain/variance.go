package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type BudgetActual struct {
	ID             common.ID
	EntityID       common.ID
	AccountID      common.ID
	CostCenterID   *common.ID
	FiscalPeriodID common.ID
	ActualAmount   money.Money
	SnapshotAt     time.Time

	AccountCode      string
	AccountName      string
	CostCenterCode   string
	CostCenterName   string
	FiscalPeriodName string
}

func NewBudgetActual(
	entityID common.ID,
	accountID common.ID,
	fiscalPeriodID common.ID,
	actualAmount money.Money,
) *BudgetActual {
	return &BudgetActual{
		ID:             common.NewID(),
		EntityID:       entityID,
		AccountID:      accountID,
		FiscalPeriodID: fiscalPeriodID,
		ActualAmount:   actualAmount,
		SnapshotAt:     time.Now(),
	}
}

func (a *BudgetActual) SetCostCenter(costCenterID common.ID) {
	a.CostCenterID = &costCenterID
}

type VarianceAnalysis struct {
	AccountID        common.ID
	AccountCode      string
	AccountName      string
	AccountType      string
	CostCenterID     *common.ID
	CostCenterCode   string
	CostCenterName   string
	FiscalPeriodID   common.ID
	FiscalPeriodName string

	BudgetAmount   money.Money
	ActualAmount   money.Money
	VarianceAmount money.Money
	VariancePercent float64

	IsFavorable bool
}

func NewVarianceAnalysis(
	accountID common.ID,
	accountCode string,
	accountName string,
	accountType string,
	fiscalPeriodID common.ID,
	budgetAmount money.Money,
	actualAmount money.Money,
) *VarianceAnalysis {
	varianceAmount := actualAmount.MustSubtract(budgetAmount)

	var variancePercent float64
	if !budgetAmount.IsZero() {
		variancePercent = varianceAmount.Amount.InexactFloat64() / budgetAmount.Amount.InexactFloat64() * 100
	}

	var isFavorable bool
	switch accountType {
	case "revenue":
		isFavorable = varianceAmount.IsPositive()
	case "expense":
		isFavorable = varianceAmount.IsNegative()
	}

	return &VarianceAnalysis{
		AccountID:       accountID,
		AccountCode:     accountCode,
		AccountName:     accountName,
		AccountType:     accountType,
		FiscalPeriodID:  fiscalPeriodID,
		BudgetAmount:    budgetAmount,
		ActualAmount:    actualAmount,
		VarianceAmount:  varianceAmount,
		VariancePercent: variancePercent,
		IsFavorable:     isFavorable,
	}
}

func (v *VarianceAnalysis) SetCostCenter(costCenterID common.ID, code, name string) {
	v.CostCenterID = &costCenterID
	v.CostCenterCode = code
	v.CostCenterName = name
}

type VarianceSummary struct {
	EntityID         common.ID
	FiscalPeriodID   common.ID
	FiscalPeriodName string
	BudgetID         common.ID
	BudgetName       string

	TotalBudgetRevenue    money.Money
	TotalActualRevenue    money.Money
	RevenueVariance       money.Money
	RevenueVariancePercent float64

	TotalBudgetExpenses   money.Money
	TotalActualExpenses   money.Money
	ExpenseVariance       money.Money
	ExpenseVariancePercent float64

	BudgetNetIncome       money.Money
	ActualNetIncome       money.Money
	NetIncomeVariance     money.Money
	NetIncomeVariancePercent float64

	FavorableVariances   int
	UnfavorableVariances int
}

func NewVarianceSummary(
	entityID common.ID,
	fiscalPeriodID common.ID,
	budgetID common.ID,
	currency money.Currency,
) *VarianceSummary {
	return &VarianceSummary{
		EntityID:              entityID,
		FiscalPeriodID:        fiscalPeriodID,
		BudgetID:              budgetID,
		TotalBudgetRevenue:    money.Zero(currency),
		TotalActualRevenue:    money.Zero(currency),
		RevenueVariance:       money.Zero(currency),
		TotalBudgetExpenses:   money.Zero(currency),
		TotalActualExpenses:   money.Zero(currency),
		ExpenseVariance:       money.Zero(currency),
		BudgetNetIncome:       money.Zero(currency),
		ActualNetIncome:       money.Zero(currency),
		NetIncomeVariance:     money.Zero(currency),
	}
}

func (s *VarianceSummary) Calculate(analyses []VarianceAnalysis) {
	for _, a := range analyses {
		switch a.AccountType {
		case "revenue":
			s.TotalBudgetRevenue = s.TotalBudgetRevenue.MustAdd(a.BudgetAmount)
			s.TotalActualRevenue = s.TotalActualRevenue.MustAdd(a.ActualAmount)
		case "expense":
			s.TotalBudgetExpenses = s.TotalBudgetExpenses.MustAdd(a.BudgetAmount)
			s.TotalActualExpenses = s.TotalActualExpenses.MustAdd(a.ActualAmount)
		}

		if a.IsFavorable {
			s.FavorableVariances++
		} else if !a.VarianceAmount.IsZero() {
			s.UnfavorableVariances++
		}
	}

	s.RevenueVariance = s.TotalActualRevenue.MustSubtract(s.TotalBudgetRevenue)
	if !s.TotalBudgetRevenue.IsZero() {
		s.RevenueVariancePercent = s.RevenueVariance.Amount.InexactFloat64() / s.TotalBudgetRevenue.Amount.InexactFloat64() * 100
	}

	s.ExpenseVariance = s.TotalActualExpenses.MustSubtract(s.TotalBudgetExpenses)
	if !s.TotalBudgetExpenses.IsZero() {
		s.ExpenseVariancePercent = s.ExpenseVariance.Amount.InexactFloat64() / s.TotalBudgetExpenses.Amount.InexactFloat64() * 100
	}

	s.BudgetNetIncome = s.TotalBudgetRevenue.MustSubtract(s.TotalBudgetExpenses)
	s.ActualNetIncome = s.TotalActualRevenue.MustSubtract(s.TotalActualExpenses)
	s.NetIncomeVariance = s.ActualNetIncome.MustSubtract(s.BudgetNetIncome)
	if !s.BudgetNetIncome.IsZero() {
		s.NetIncomeVariancePercent = s.NetIncomeVariance.Amount.InexactFloat64() / s.BudgetNetIncome.Amount.InexactFloat64() * 100
	}
}

type BudgetActualFilter struct {
	EntityID       *common.ID
	AccountID      *common.ID
	CostCenterID   *common.ID
	FiscalPeriodID *common.ID
	Limit          int
	Offset         int
}

type VarianceFilter struct {
	EntityID       *common.ID
	BudgetID       *common.ID
	FiscalPeriodID *common.ID
	CostCenterID   *common.ID
	AccountType    *string
	OnlyUnfavorable bool
	Limit          int
	Offset         int
}
