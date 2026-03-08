package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type AssetStatus string

const (
	AssetStatusDraft     AssetStatus = "draft"
	AssetStatusActive    AssetStatus = "active"
	AssetStatusSuspended AssetStatus = "suspended"
	AssetStatusDisposed  AssetStatus = "disposed"
	AssetStatusWrittenOff AssetStatus = "written_off"
)

func (s AssetStatus) IsValid() bool {
	switch s {
	case AssetStatusDraft, AssetStatusActive, AssetStatusSuspended, AssetStatusDisposed, AssetStatusWrittenOff:
		return true
	}
	return false
}

func (s AssetStatus) String() string {
	return string(s)
}

func (s AssetStatus) IsDepreciable() bool {
	return s == AssetStatusActive
}

func (s AssetStatus) CanTransfer() bool {
	return s == AssetStatusActive
}

func (s AssetStatus) CanDispose() bool {
	return s == AssetStatusActive || s == AssetStatusSuspended
}

type DisposalType string

const (
	DisposalTypeSale      DisposalType = "sale"
	DisposalTypeScrapping DisposalType = "scrapping"
	DisposalTypeDonation  DisposalType = "donation"
	DisposalTypeTradeIn   DisposalType = "trade_in"
	DisposalTypeTheftLoss DisposalType = "theft_loss"
)

func (dt DisposalType) IsValid() bool {
	switch dt {
	case DisposalTypeSale, DisposalTypeScrapping, DisposalTypeDonation, DisposalTypeTradeIn, DisposalTypeTheftLoss:
		return true
	}
	return false
}

type Asset struct {
	ID         common.ID
	EntityID   common.ID
	CategoryID common.ID
	AssetCode  string
	AssetName  string
	Description string
	SerialNumber string
	Barcode      string

	// Acquisition
	AcquisitionDate time.Time
	AcquisitionCost money.Money
	Currency        money.Currency
	VendorID        *common.ID
	APInvoiceID     *common.ID
	PONumber        string

	// Depreciation Config
	DepreciationMethod    DepreciationMethod
	UsefulLifeYears       int
	UsefulLifeUnits       *int
	SalvageValue          money.Money
	DepreciationStartDate *time.Time

	// Current Values
	AccumulatedDepreciation  money.Money
	BookValue                money.Money
	UnitsUsed                int
	LastDepreciationDate     *time.Time
	DepreciationThroughDate  *time.Time

	// GL Accounts (override category defaults)
	AssetAccountID              *common.ID
	AccumDepreciationAccountID  *common.ID
	DepreciationExpenseAccountID *common.ID

	// Location & Assignment
	LocationCode    string
	LocationName    string
	DepartmentCode  string
	DepartmentName  string
	CustodianID     *common.ID
	CustodianName   string
	CostCenterID    *common.ID

	// Status
	Status         AssetStatus
	ActivatedAt    *time.Time
	ActivatedBy    *common.ID
	SuspendedAt    *time.Time
	SuspendedReason string

	// Disposal
	DisposedAt       *time.Time
	DisposalType     *DisposalType
	DisposalProceeds money.Money
	DisposalCost     money.Money
	DisposalGainLoss money.Money
	DisposalJournalID *common.ID
	DisposalNotes    string

	// Metadata
	WarrantyExpiry   *time.Time
	InsurancePolicy  string
	Notes            string
	Tags             []string
	CustomFields     map[string]interface{}

	// Audit
	CreatedBy common.ID
	CreatedAt time.Time
	UpdatedAt time.Time

	// Related data (for convenience)
	Category *AssetCategory
}

func NewAsset(
	entityID common.ID,
	categoryID common.ID,
	assetCode string,
	assetName string,
	acquisitionDate time.Time,
	acquisitionCost money.Money,
	depreciationMethod DepreciationMethod,
	usefulLifeYears int,
	salvageValue money.Money,
	createdBy common.ID,
) (*Asset, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if categoryID.IsZero() {
		return nil, fmt.Errorf("category ID is required")
	}
	if assetCode == "" {
		return nil, fmt.Errorf("asset code is required")
	}
	if assetName == "" {
		return nil, fmt.Errorf("asset name is required")
	}
	if acquisitionCost.IsNegative() {
		return nil, fmt.Errorf("acquisition cost cannot be negative")
	}
	if !depreciationMethod.IsValid() {
		return nil, fmt.Errorf("invalid depreciation method")
	}
	if usefulLifeYears <= 0 {
		return nil, fmt.Errorf("useful life must be at least 1 year")
	}
	if salvageValue.IsNegative() {
		return nil, fmt.Errorf("salvage value cannot be negative")
	}
	if salvageValue.GreaterThan(acquisitionCost) {
		return nil, fmt.Errorf("salvage value cannot exceed acquisition cost")
	}

	now := time.Now()
	return &Asset{
		ID:                      common.NewID(),
		EntityID:                entityID,
		CategoryID:              categoryID,
		AssetCode:               assetCode,
		AssetName:               assetName,
		AcquisitionDate:         acquisitionDate,
		AcquisitionCost:         acquisitionCost,
		Currency:                acquisitionCost.Currency,
		DepreciationMethod:      depreciationMethod,
		UsefulLifeYears:         usefulLifeYears,
		SalvageValue:            salvageValue,
		AccumulatedDepreciation: money.Zero(acquisitionCost.Currency),
		BookValue:               acquisitionCost,
		DisposalProceeds:        money.Zero(acquisitionCost.Currency),
		DisposalCost:            money.Zero(acquisitionCost.Currency),
		DisposalGainLoss:        money.Zero(acquisitionCost.Currency),
		Status:                  AssetStatusDraft,
		Tags:                    []string{},
		CustomFields:            make(map[string]interface{}),
		CreatedBy:               createdBy,
		CreatedAt:               now,
		UpdatedAt:               now,
	}, nil
}

func (a *Asset) Validate() error {
	ve := common.NewValidationError()

	if a.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if a.CategoryID.IsZero() {
		ve.Add("category_id", "required", "Category ID is required")
	}
	if a.AssetCode == "" {
		ve.Add("asset_code", "required", "Asset code is required")
	}
	if len(a.AssetCode) > 50 {
		ve.Add("asset_code", "max_length", "Asset code cannot exceed 50 characters")
	}
	if a.AssetName == "" {
		ve.Add("asset_name", "required", "Asset name is required")
	}
	if len(a.AssetName) > 255 {
		ve.Add("asset_name", "max_length", "Asset name cannot exceed 255 characters")
	}
	if a.AcquisitionCost.IsNegative() {
		ve.Add("acquisition_cost", "min", "Acquisition cost cannot be negative")
	}
	if !a.DepreciationMethod.IsValid() {
		ve.Add("depreciation_method", "invalid", "Invalid depreciation method")
	}
	if a.UsefulLifeYears <= 0 {
		ve.Add("useful_life_years", "min", "Useful life must be at least 1 year")
	}
	if a.SalvageValue.IsNegative() {
		ve.Add("salvage_value", "min", "Salvage value cannot be negative")
	}
	if a.SalvageValue.GreaterThan(a.AcquisitionCost) {
		ve.Add("salvage_value", "max", "Salvage value cannot exceed acquisition cost")
	}
	if a.DepreciationMethod == DepreciationMethodUnitsOfProduction && a.UsefulLifeUnits == nil {
		ve.Add("useful_life_units", "required", "Units of production method requires useful life units")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (a *Asset) Activate(startDate time.Time, activatedBy common.ID) error {
	if a.Status != AssetStatusDraft {
		return fmt.Errorf("can only activate assets in draft status, current status: %s", a.Status)
	}
	if startDate.IsZero() {
		return fmt.Errorf("depreciation start date is required")
	}

	now := time.Now()
	a.Status = AssetStatusActive
	a.DepreciationStartDate = &startDate
	a.ActivatedAt = &now
	a.ActivatedBy = &activatedBy
	a.UpdatedAt = now
	return nil
}

func (a *Asset) Suspend(reason string) error {
	if a.Status != AssetStatusActive {
		return fmt.Errorf("can only suspend active assets, current status: %s", a.Status)
	}

	now := time.Now()
	a.Status = AssetStatusSuspended
	a.SuspendedAt = &now
	a.SuspendedReason = reason
	a.UpdatedAt = now
	return nil
}

func (a *Asset) Reactivate() error {
	if a.Status != AssetStatusSuspended {
		return fmt.Errorf("can only reactivate suspended assets, current status: %s", a.Status)
	}

	a.Status = AssetStatusActive
	a.SuspendedAt = nil
	a.SuspendedReason = ""
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Asset) Dispose(disposalType DisposalType, proceeds money.Money, cost money.Money, notes string) error {
	if !a.Status.CanDispose() {
		return fmt.Errorf("cannot dispose asset with status: %s", a.Status)
	}
	if !disposalType.IsValid() {
		return fmt.Errorf("invalid disposal type: %s", disposalType)
	}

	now := time.Now()
	gainLoss := proceeds.MustSubtract(cost).MustSubtract(a.BookValue)

	a.Status = AssetStatusDisposed
	a.DisposedAt = &now
	a.DisposalType = &disposalType
	a.DisposalProceeds = proceeds
	a.DisposalCost = cost
	a.DisposalGainLoss = gainLoss
	a.DisposalNotes = notes
	a.UpdatedAt = now
	return nil
}

func (a *Asset) WriteOff(notes string) error {
	if !a.Status.CanDispose() {
		return fmt.Errorf("cannot write off asset with status: %s", a.Status)
	}

	now := time.Now()
	a.Status = AssetStatusWrittenOff
	a.DisposedAt = &now
	a.DisposalNotes = notes
	a.DisposalGainLoss = a.BookValue.Negate()
	a.UpdatedAt = now
	return nil
}

func (a *Asset) RecordDepreciation(amount money.Money, throughDate time.Time) error {
	if !a.Status.IsDepreciable() {
		return fmt.Errorf("cannot depreciate asset with status: %s", a.Status)
	}
	if amount.IsNegative() {
		return fmt.Errorf("depreciation amount cannot be negative")
	}
	if amount.IsZero() {
		return nil
	}

	newAccumulated := a.AccumulatedDepreciation.MustAdd(amount)
	depreciableAmount := a.GetDepreciableAmount()

	if newAccumulated.GreaterThan(depreciableAmount) {
		return fmt.Errorf("depreciation would exceed depreciable amount")
	}

	a.AccumulatedDepreciation = newAccumulated
	a.BookValue = a.AcquisitionCost.MustSubtract(newAccumulated)
	a.LastDepreciationDate = &throughDate
	a.DepreciationThroughDate = &throughDate
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Asset) RecordUnits(units int) error {
	if a.DepreciationMethod != DepreciationMethodUnitsOfProduction {
		return fmt.Errorf("asset does not use units of production depreciation")
	}
	if units < 0 {
		return fmt.Errorf("units cannot be negative")
	}
	if a.UsefulLifeUnits != nil && a.UnitsUsed+units > *a.UsefulLifeUnits {
		return fmt.Errorf("total units would exceed useful life units")
	}

	a.UnitsUsed += units
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Asset) GetDepreciableAmount() money.Money {
	return a.AcquisitionCost.MustSubtract(a.SalvageValue)
}

func (a *Asset) GetRemainingDepreciableAmount() money.Money {
	return a.GetDepreciableAmount().MustSubtract(a.AccumulatedDepreciation)
}

func (a *Asset) IsFullyDepreciated() bool {
	return a.AccumulatedDepreciation.GreaterThanOrEqual(a.GetDepreciableAmount())
}

func (a *Asset) GetMonthsInService(asOf time.Time) int {
	if a.DepreciationStartDate == nil {
		return 0
	}

	startYear, startMonth, _ := a.DepreciationStartDate.Date()
	asOfYear, asOfMonth, _ := asOf.Date()

	months := (asOfYear-startYear)*12 + int(asOfMonth-startMonth) + 1
	if months < 0 {
		return 0
	}
	return months
}

func (a *Asset) GetRemainingLifeMonths(asOf time.Time) int {
	totalMonths := a.UsefulLifeYears * 12
	elapsed := a.GetMonthsInService(asOf)
	remaining := totalMonths - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (a *Asset) GetEffectiveAssetAccountID(category *AssetCategory) *common.ID {
	if a.AssetAccountID != nil {
		return a.AssetAccountID
	}
	if category != nil {
		return category.AssetAccountID
	}
	return nil
}

func (a *Asset) GetEffectiveAccumDepAccountID(category *AssetCategory) *common.ID {
	if a.AccumDepreciationAccountID != nil {
		return a.AccumDepreciationAccountID
	}
	if category != nil {
		return category.AccumDepreciationAccountID
	}
	return nil
}

func (a *Asset) GetEffectiveDepExpenseAccountID(category *AssetCategory) *common.ID {
	if a.DepreciationExpenseAccountID != nil {
		return a.DepreciationExpenseAccountID
	}
	if category != nil {
		return category.DepreciationExpenseAccountID
	}
	return nil
}

func (a *Asset) CalculateMonthlyDepreciation() money.Money {
	if a.IsFullyDepreciated() {
		return money.Zero(a.Currency)
	}

	depreciableAmount := a.GetDepreciableAmount()
	remainingAmount := a.GetRemainingDepreciableAmount()

	switch a.DepreciationMethod {
	case DepreciationMethodStraightLine:
		totalMonths := decimal.NewFromInt(int64(a.UsefulLifeYears * 12))
		monthlyDep := depreciableAmount.Amount.Div(totalMonths)
		result := money.NewFromDecimal(monthlyDep, a.Currency)
		if result.GreaterThan(remainingAmount) {
			return remainingAmount
		}
		return result

	case DepreciationMethodDecliningBalance:
		rate := decimal.NewFromFloat(2.0 / float64(a.UsefulLifeYears))
		annualDep := a.BookValue.Amount.Mul(rate)
		monthlyDep := annualDep.Div(decimal.NewFromInt(12))
		result := money.NewFromDecimal(monthlyDep, a.Currency)
		if result.GreaterThan(remainingAmount) {
			return remainingAmount
		}
		return result

	case DepreciationMethodSumOfYearsDigits:
		if a.DepreciationStartDate == nil {
			return money.Zero(a.Currency)
		}
		yearsElapsed := a.GetMonthsInService(time.Now()) / 12
		currentYear := yearsElapsed + 1
		if currentYear > a.UsefulLifeYears {
			return money.Zero(a.Currency)
		}
		n := a.UsefulLifeYears
		sumOfYears := n * (n + 1) / 2
		remainingYears := n - yearsElapsed
		yearFraction := decimal.NewFromInt(int64(remainingYears)).Div(decimal.NewFromInt(int64(sumOfYears)))
		annualDep := depreciableAmount.Amount.Mul(yearFraction)
		monthlyDep := annualDep.Div(decimal.NewFromInt(12))
		result := money.NewFromDecimal(monthlyDep, a.Currency)
		if result.GreaterThan(remainingAmount) {
			return remainingAmount
		}
		return result

	case DepreciationMethodUnitsOfProduction:
		return money.Zero(a.Currency)

	default:
		return money.Zero(a.Currency)
	}
}

func (a *Asset) CalculateUnitsDepreciation(unitsThisPeriod int) money.Money {
	if a.DepreciationMethod != DepreciationMethodUnitsOfProduction {
		return money.Zero(a.Currency)
	}
	if a.UsefulLifeUnits == nil || *a.UsefulLifeUnits == 0 {
		return money.Zero(a.Currency)
	}
	if a.IsFullyDepreciated() {
		return money.Zero(a.Currency)
	}

	depreciableAmount := a.GetDepreciableAmount()
	remainingAmount := a.GetRemainingDepreciableAmount()

	costPerUnit := depreciableAmount.Amount.Div(decimal.NewFromInt(int64(*a.UsefulLifeUnits)))
	periodDep := costPerUnit.Mul(decimal.NewFromInt(int64(unitsThisPeriod)))
	result := money.NewFromDecimal(periodDep, a.Currency)

	if result.GreaterThan(remainingAmount) {
		return remainingAmount
	}
	return result
}

type AssetFilter struct {
	EntityID      common.ID
	CategoryID    *common.ID
	Status        *AssetStatus
	LocationCode  *string
	DepartmentCode *string
	CustodianID   *common.ID
	VendorID      *common.ID
	Search        string
	AcquiredFrom  *time.Time
	AcquiredTo    *time.Time
	Limit         int
	Offset        int
}
