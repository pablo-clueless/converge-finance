package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type DepreciationRunStatus string

const (
	DepreciationRunStatusDraft      DepreciationRunStatus = "draft"
	DepreciationRunStatusCalculated DepreciationRunStatus = "calculated"
	DepreciationRunStatusPosted     DepreciationRunStatus = "posted"
	DepreciationRunStatusReversed   DepreciationRunStatus = "reversed"
)

func (s DepreciationRunStatus) IsValid() bool {
	switch s {
	case DepreciationRunStatusDraft, DepreciationRunStatusCalculated, DepreciationRunStatusPosted, DepreciationRunStatusReversed:
		return true
	}
	return false
}

func (s DepreciationRunStatus) CanPost() bool {
	return s == DepreciationRunStatusCalculated
}

func (s DepreciationRunStatus) CanReverse() bool {
	return s == DepreciationRunStatusPosted
}

func (s DepreciationRunStatus) CanAddEntries() bool {
	return s == DepreciationRunStatusDraft
}

type DepreciationRun struct {
	ID                common.ID
	EntityID          common.ID
	RunNumber         string
	FiscalPeriodID    common.ID
	DepreciationDate  time.Time
	AssetCount        int
	TotalDepreciation money.Money
	Currency          money.Currency
	Status            DepreciationRunStatus
	JournalEntryID    *common.ID
	Notes             string

	PostedAt      *time.Time
	PostedBy      *common.ID
	ReversedAt    *time.Time
	ReversedBy    *common.ID
	ReversalRunID *common.ID

	CreatedBy common.ID
	CreatedAt time.Time
	UpdatedAt time.Time

	Entries []DepreciationEntry
}

func NewDepreciationRun(
	entityID common.ID,
	runNumber string,
	fiscalPeriodID common.ID,
	depreciationDate time.Time,
	currency money.Currency,
	createdBy common.ID,
) (*DepreciationRun, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if runNumber == "" {
		return nil, fmt.Errorf("run number is required")
	}
	if fiscalPeriodID.IsZero() {
		return nil, fmt.Errorf("fiscal period ID is required")
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}

	now := time.Now()
	return &DepreciationRun{
		ID:                common.NewID(),
		EntityID:          entityID,
		RunNumber:         runNumber,
		FiscalPeriodID:    fiscalPeriodID,
		DepreciationDate:  depreciationDate,
		AssetCount:        0,
		TotalDepreciation: money.Zero(currency),
		Currency:          currency,
		Status:            DepreciationRunStatusDraft,
		Entries:           make([]DepreciationEntry, 0),
		CreatedBy:         createdBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

func (r *DepreciationRun) AddEntry(entry DepreciationEntry) error {
	if !r.Status.CanAddEntries() {
		return fmt.Errorf("cannot add entries to run with status: %s", r.Status)
	}

	r.Entries = append(r.Entries, entry)
	r.AssetCount = len(r.Entries)
	r.TotalDepreciation = r.TotalDepreciation.MustAdd(entry.DepreciationAmount)
	r.UpdatedAt = time.Now()
	return nil
}

func (r *DepreciationRun) Calculate() error {
	if r.Status != DepreciationRunStatusDraft {
		return fmt.Errorf("can only calculate runs in draft status, current status: %s", r.Status)
	}
	if len(r.Entries) == 0 {
		return fmt.Errorf("cannot calculate run with no entries")
	}

	r.Status = DepreciationRunStatusCalculated
	r.UpdatedAt = time.Now()
	return nil
}

func (r *DepreciationRun) Post(journalEntryID common.ID, postedBy common.ID) error {
	if !r.Status.CanPost() {
		return fmt.Errorf("cannot post run with status: %s", r.Status)
	}

	now := time.Now()
	r.Status = DepreciationRunStatusPosted
	r.JournalEntryID = &journalEntryID
	r.PostedAt = &now
	r.PostedBy = &postedBy
	r.UpdatedAt = now
	return nil
}

func (r *DepreciationRun) Reverse(reversedBy common.ID) error {
	if !r.Status.CanReverse() {
		return fmt.Errorf("cannot reverse run with status: %s", r.Status)
	}

	now := time.Now()
	r.Status = DepreciationRunStatusReversed
	r.ReversedAt = &now
	r.ReversedBy = &reversedBy
	r.UpdatedAt = now
	return nil
}

func (r *DepreciationRun) SetReversalRun(reversalRunID common.ID) {
	r.ReversalRunID = &reversalRunID
	r.UpdatedAt = time.Now()
}

func (r *DepreciationRun) Validate() error {
	ve := common.NewValidationError()

	if r.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if r.RunNumber == "" {
		ve.Add("run_number", "required", "Run number is required")
	}
	if r.FiscalPeriodID.IsZero() {
		ve.Add("fiscal_period_id", "required", "Fiscal period ID is required")
	}
	if r.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

type DepreciationEntry struct {
	ID                common.ID
	DepreciationRunID common.ID
	AssetID           common.ID
	OpeningBookValue  money.Money
	DepreciationAmount money.Money
	ClosingBookValue  money.Money
	AccumulatedBefore money.Money
	AccumulatedAfter  money.Money
	DepreciationMethod DepreciationMethod
	UsefulLifeYears   int
	MonthsElapsed     int
	CalculationBasis  string
	CreatedAt         time.Time

	Asset *Asset
}

func NewDepreciationEntry(
	runID common.ID,
	asset *Asset,
	depreciationAmount money.Money,
	monthsElapsed int,
	calculationBasis string,
) (*DepreciationEntry, error) {
	if runID.IsZero() {
		return nil, fmt.Errorf("run ID is required")
	}
	if asset == nil {
		return nil, fmt.Errorf("asset is required")
	}
	if depreciationAmount.IsNegative() {
		return nil, fmt.Errorf("depreciation amount cannot be negative")
	}

	openingBookValue := asset.BookValue
	closingBookValue := openingBookValue.MustSubtract(depreciationAmount)
	accumulatedBefore := asset.AccumulatedDepreciation
	accumulatedAfter := accumulatedBefore.MustAdd(depreciationAmount)

	return &DepreciationEntry{
		ID:                 common.NewID(),
		DepreciationRunID:  runID,
		AssetID:            asset.ID,
		OpeningBookValue:   openingBookValue,
		DepreciationAmount: depreciationAmount,
		ClosingBookValue:   closingBookValue,
		AccumulatedBefore:  accumulatedBefore,
		AccumulatedAfter:   accumulatedAfter,
		DepreciationMethod: asset.DepreciationMethod,
		UsefulLifeYears:    asset.UsefulLifeYears,
		MonthsElapsed:      monthsElapsed,
		CalculationBasis:   calculationBasis,
		CreatedAt:          time.Now(),
		Asset:              asset,
	}, nil
}

type DepreciationRunFilter struct {
	EntityID       common.ID
	FiscalPeriodID *common.ID
	Status         *DepreciationRunStatus
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}

type DepreciationPreview struct {
	Asset              *Asset
	OpeningBookValue   money.Money
	DepreciationAmount money.Money
	ClosingBookValue   money.Money
	Method             DepreciationMethod
	CalculationBasis   string
}

type DepreciationCalculator struct{}

func NewDepreciationCalculator() *DepreciationCalculator {
	return &DepreciationCalculator{}
}

func (c *DepreciationCalculator) CalculateMonthlyDepreciation(
	asset *Asset,
	periodEndDate time.Time,
) (money.Money, string, error) {
	if asset == nil {
		return money.Money{}, "", fmt.Errorf("asset is required")
	}
	if !asset.Status.IsDepreciable() {
		return money.Zero(asset.Currency), "Asset not depreciable", nil
	}
	if asset.DepreciationStartDate == nil {
		return money.Zero(asset.Currency), "No depreciation start date", nil
	}
	if asset.IsFullyDepreciated() {
		return money.Zero(asset.Currency), "Fully depreciated", nil
	}
	if asset.DepreciationThroughDate != nil && !asset.DepreciationThroughDate.Before(periodEndDate) {
		return money.Zero(asset.Currency), "Already depreciated through this period", nil
	}

	switch asset.DepreciationMethod {
	case DepreciationMethodStraightLine:
		return c.straightLine(asset, periodEndDate)
	case DepreciationMethodDecliningBalance:
		return c.decliningBalance(asset, periodEndDate)
	case DepreciationMethodSumOfYearsDigits:
		return c.sumOfYearsDigits(asset, periodEndDate)
	case DepreciationMethodUnitsOfProduction:
		return money.Zero(asset.Currency), "Units of production requires explicit units", nil
	default:
		return money.Zero(asset.Currency), "", fmt.Errorf("unsupported depreciation method: %s", asset.DepreciationMethod)
	}
}

func (c *DepreciationCalculator) straightLine(asset *Asset, periodEndDate time.Time) (money.Money, string, error) {
	depreciableAmount := asset.GetDepreciableAmount()
	remainingAmount := asset.GetRemainingDepreciableAmount()
	totalMonths := asset.UsefulLifeYears * 12

	monthlyDepAmount := depreciableAmount.Amount.Div(decimal.NewFromInt(int64(totalMonths)))
	monthlyDep := money.NewFromDecimal(monthlyDepAmount, asset.Currency)
	basis := fmt.Sprintf("Straight-line: %s / %d months = %s/month",
		depreciableAmount.String(), totalMonths, monthlyDep.String())

	if monthlyDep.GreaterThan(remainingAmount) {
		monthlyDep = remainingAmount
		basis += " (capped at remaining amount)"
	}

	return monthlyDep, basis, nil
}

func (c *DepreciationCalculator) decliningBalance(asset *Asset, periodEndDate time.Time) (money.Money, string, error) {
	remainingAmount := asset.GetRemainingDepreciableAmount()
	rate := 2.0 / float64(asset.UsefulLifeYears)

	annualDep := asset.BookValue.MultiplyFloat(rate)
	monthlyDepAmount := annualDep.Amount.Div(decimal.NewFromInt(12))
	monthlyDep := money.NewFromDecimal(monthlyDepAmount, asset.Currency)

	basis := fmt.Sprintf("Double declining: %s * %.2f%% / 12 = %s/month",
		asset.BookValue.String(), rate*100, monthlyDep.String())

	slMonthlyDepAmount := asset.GetDepreciableAmount().Amount.Div(decimal.NewFromInt(int64(asset.UsefulLifeYears * 12)))
	slMonthlyDep := money.NewFromDecimal(slMonthlyDepAmount, asset.Currency)
	if monthlyDep.LessThan(slMonthlyDep) {
		monthlyDep = slMonthlyDep
		basis += " (switched to straight-line)"
	}

	if monthlyDep.GreaterThan(remainingAmount) {
		monthlyDep = remainingAmount
		basis += " (capped at remaining amount)"
	}

	return monthlyDep, basis, nil
}

func (c *DepreciationCalculator) sumOfYearsDigits(asset *Asset, periodEndDate time.Time) (money.Money, string, error) {
	if asset.DepreciationStartDate == nil {
		return money.Zero(asset.Currency), "", fmt.Errorf("depreciation start date is required")
	}

	depreciableAmount := asset.GetDepreciableAmount()
	remainingAmount := asset.GetRemainingDepreciableAmount()
	n := asset.UsefulLifeYears
	sumOfYears := n * (n + 1) / 2

	monthsElapsed := asset.GetMonthsInService(periodEndDate)
	currentYear := (monthsElapsed-1)/12 + 1
	if currentYear > n {
		return money.Zero(asset.Currency), "Beyond useful life", nil
	}

	remainingYears := n - currentYear + 1
	yearFraction := float64(remainingYears) / float64(sumOfYears)
	annualDep := depreciableAmount.MultiplyFloat(yearFraction)
	monthlyDepAmount := annualDep.Amount.Div(decimal.NewFromInt(12))
	monthlyDep := money.NewFromDecimal(monthlyDepAmount, asset.Currency)

	basis := fmt.Sprintf("SYD: %s * (%d/%d) / 12 = %s/month (year %d of %d)",
		depreciableAmount.String(), remainingYears, sumOfYears, monthlyDep.String(), currentYear, n)

	if monthlyDep.GreaterThan(remainingAmount) {
		monthlyDep = remainingAmount
		basis += " (capped at remaining amount)"
	}

	return monthlyDep, basis, nil
}

func (c *DepreciationCalculator) CalculateUnitsDepreciation(
	asset *Asset,
	unitsThisPeriod int,
) (money.Money, string, error) {
	if asset == nil {
		return money.Money{}, "", fmt.Errorf("asset is required")
	}
	if asset.DepreciationMethod != DepreciationMethodUnitsOfProduction {
		return money.Money{}, "", fmt.Errorf("asset does not use units of production method")
	}
	if asset.UsefulLifeUnits == nil || *asset.UsefulLifeUnits == 0 {
		return money.Zero(asset.Currency), "", fmt.Errorf("useful life units not set")
	}
	if asset.IsFullyDepreciated() {
		return money.Zero(asset.Currency), "Fully depreciated", nil
	}
	if unitsThisPeriod <= 0 {
		return money.Zero(asset.Currency), "No units used", nil
	}

	depreciableAmount := asset.GetDepreciableAmount()
	remainingAmount := asset.GetRemainingDepreciableAmount()
	totalUnits := *asset.UsefulLifeUnits

	costPerUnitAmount := depreciableAmount.Amount.Div(decimal.NewFromInt(int64(totalUnits)))
	periodDepAmount := costPerUnitAmount.Mul(decimal.NewFromInt(int64(unitsThisPeriod)))
	periodDep := money.NewFromDecimal(periodDepAmount, asset.Currency)

	basis := fmt.Sprintf("Units: %s / %d units * %d units = %s",
		depreciableAmount.String(), totalUnits, unitsThisPeriod, periodDep.String())

	if periodDep.GreaterThan(remainingAmount) {
		periodDep = remainingAmount
		basis += " (capped at remaining amount)"
	}

	return periodDep, basis, nil
}
