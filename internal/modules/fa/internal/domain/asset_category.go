package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

type DepreciationMethod string

const (
	DepreciationMethodStraightLine      DepreciationMethod = "straight_line"
	DepreciationMethodDecliningBalance  DepreciationMethod = "declining_balance"
	DepreciationMethodSumOfYearsDigits  DepreciationMethod = "sum_of_years_digits"
	DepreciationMethodUnitsOfProduction DepreciationMethod = "units_of_production"
)

func (dm DepreciationMethod) IsValid() bool {
	switch dm {
	case DepreciationMethodStraightLine,
		DepreciationMethodDecliningBalance,
		DepreciationMethodSumOfYearsDigits,
		DepreciationMethodUnitsOfProduction:
		return true
	}
	return false
}

func (dm DepreciationMethod) String() string {
	return string(dm)
}

func (dm DepreciationMethod) DisplayName() string {
	switch dm {
	case DepreciationMethodStraightLine:
		return "Straight Line"
	case DepreciationMethodDecliningBalance:
		return "Declining Balance"
	case DepreciationMethodSumOfYearsDigits:
		return "Sum of Years' Digits"
	case DepreciationMethodUnitsOfProduction:
		return "Units of Production"
	default:
		return string(dm)
	}
}

type AssetCategory struct {
	ID                           common.ID
	EntityID                     common.ID
	Code                         string
	Name                         string
	Description                  string
	DepreciationMethod           DepreciationMethod
	DefaultUsefulLifeYears       int
	DefaultSalvagePercent        decimal.Decimal
	AssetAccountID               *common.ID
	AccumDepreciationAccountID   *common.ID
	DepreciationExpenseAccountID *common.ID
	GainLossAccountID            *common.ID
	IsActive                     bool
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
	CreatedBy                    common.ID
}

func NewAssetCategory(
	entityID common.ID,
	code string,
	name string,
	createdBy common.ID,
) (*AssetCategory, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if code == "" {
		return nil, fmt.Errorf("category code is required")
	}
	if name == "" {
		return nil, fmt.Errorf("category name is required")
	}
	if createdBy.IsZero() {
		return nil, fmt.Errorf("created by is required")
	}

	now := time.Now()
	return &AssetCategory{
		ID:                     common.NewID(),
		EntityID:               entityID,
		Code:                   code,
		Name:                   name,
		DepreciationMethod:     DepreciationMethodStraightLine,
		DefaultUsefulLifeYears: 5,
		DefaultSalvagePercent:  decimal.Zero,
		IsActive:               true,
		CreatedAt:              now,
		UpdatedAt:              now,
		CreatedBy:              createdBy,
	}, nil
}

func (c *AssetCategory) Validate() error {
	ve := common.NewValidationError()

	if c.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if c.Code == "" {
		ve.Add("code", "required", "Category code is required")
	}
	if len(c.Code) > 20 {
		ve.Add("code", "max_length", "Category code cannot exceed 20 characters")
	}
	if c.Name == "" {
		ve.Add("name", "required", "Category name is required")
	}
	if len(c.Name) > 100 {
		ve.Add("name", "max_length", "Category name cannot exceed 100 characters")
	}
	if !c.DepreciationMethod.IsValid() {
		ve.Add("depreciation_method", "invalid", "Invalid depreciation method")
	}
	if c.DefaultUsefulLifeYears <= 0 {
		ve.Add("default_useful_life_years", "min", "Default useful life must be at least 1 year")
	}
	if c.DefaultSalvagePercent.LessThan(decimal.Zero) || c.DefaultSalvagePercent.GreaterThan(decimal.NewFromInt(100)) {
		ve.Add("default_salvage_percent", "range", "Default salvage percent must be between 0 and 100")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (c *AssetCategory) SetDepreciationMethod(method DepreciationMethod) error {
	if !method.IsValid() {
		return fmt.Errorf("invalid depreciation method: %s", method)
	}
	c.DepreciationMethod = method
	c.UpdatedAt = time.Now()
	return nil
}

func (c *AssetCategory) SetDefaultUsefulLife(years int) error {
	if years <= 0 {
		return fmt.Errorf("useful life must be at least 1 year")
	}
	c.DefaultUsefulLifeYears = years
	c.UpdatedAt = time.Now()
	return nil
}

func (c *AssetCategory) SetDefaultSalvagePercent(percent decimal.Decimal) error {
	if percent.LessThan(decimal.Zero) || percent.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("salvage percent must be between 0 and 100")
	}
	c.DefaultSalvagePercent = percent
	c.UpdatedAt = time.Now()
	return nil
}

func (c *AssetCategory) SetGLAccounts(
	assetAccountID *common.ID,
	accumDepAccountID *common.ID,
	depExpenseAccountID *common.ID,
	gainLossAccountID *common.ID,
) {
	c.AssetAccountID = assetAccountID
	c.AccumDepreciationAccountID = accumDepAccountID
	c.DepreciationExpenseAccountID = depExpenseAccountID
	c.GainLossAccountID = gainLossAccountID
	c.UpdatedAt = time.Now()
}

func (c *AssetCategory) Activate() {
	c.IsActive = true
	c.UpdatedAt = time.Now()
}

func (c *AssetCategory) Deactivate() {
	c.IsActive = false
	c.UpdatedAt = time.Now()
}

func (c *AssetCategory) HasGLAccountsConfigured() bool {
	return c.AssetAccountID != nil &&
		c.AccumDepreciationAccountID != nil &&
		c.DepreciationExpenseAccountID != nil
}

type AssetCategoryFilter struct {
	EntityID common.ID
	IsActive *bool
	Search   string
	Limit    int
	Offset   int
}
