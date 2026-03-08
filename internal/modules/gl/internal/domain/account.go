package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type AccountType string

const (
	AccountTypeAsset     AccountType = "asset"
	AccountTypeLiability AccountType = "liability"
	AccountTypeEquity    AccountType = "equity"
	AccountTypeRevenue   AccountType = "revenue"
	AccountTypeExpense   AccountType = "expense"
)

func (t AccountType) IsValid() bool {
	switch t {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeRevenue, AccountTypeExpense:
		return true
	}
	return false
}

func (t AccountType) NormalBalance() BalanceType {
	switch t {
	case AccountTypeAsset, AccountTypeExpense:
		return BalanceTypeDebit
	default:
		return BalanceTypeCredit
	}
}

type AccountSubtype string

const (
	AccountSubtypeCurrentAsset      AccountSubtype = "current_asset"
	AccountSubtypeFixedAsset        AccountSubtype = "fixed_asset"
	AccountSubtypeOtherAsset        AccountSubtype = "other_asset"
	AccountSubtypeCurrentLiability  AccountSubtype = "current_liability"
	AccountSubtypeLongTermLiability AccountSubtype = "long_term_liability"
	AccountSubtypeOtherLiability    AccountSubtype = "other_liability"
	AccountSubtypeRetainedEarnings  AccountSubtype = "retained_earnings"
	AccountSubtypeOtherEquity       AccountSubtype = "other_equity"
	AccountSubtypeOperatingRevenue  AccountSubtype = "operating_revenue"
	AccountSubtypeOtherRevenue      AccountSubtype = "other_revenue"
	AccountSubtypeOperatingExpense  AccountSubtype = "operating_expense"
	AccountSubtypeOtherExpense      AccountSubtype = "other_expense"
)

type BalanceType string

const (
	BalanceTypeDebit  BalanceType = "debit"
	BalanceTypeCredit BalanceType = "credit"
)

type Account struct {
	ID            common.ID
	EntityID      common.ID
	ParentID      *common.ID
	Code          string
	Name          string
	Type          AccountType
	Subtype       AccountSubtype
	Currency      money.Currency
	IsControl     bool
	IsPosting     bool
	IsActive      bool
	Description   string
	NormalBalance BalanceType
	Children      []Account
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewAccount(entityID common.ID, code, name string, accountType AccountType, currency money.Currency) (*Account, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if code == "" {
		return nil, fmt.Errorf("account code is required")
	}
	if name == "" {
		return nil, fmt.Errorf("account name is required")
	}
	if !accountType.IsValid() {
		return nil, fmt.Errorf("invalid account type: %s", accountType)
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}

	now := time.Now()
	return &Account{
		ID:            common.NewID(),
		EntityID:      entityID,
		Code:          code,
		Name:          name,
		Type:          accountType,
		Currency:      currency,
		IsPosting:     true,
		IsActive:      true,
		NormalBalance: accountType.NormalBalance(),
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (a *Account) Validate() error {
	ve := common.NewValidationError()

	if a.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if a.Code == "" {
		ve.Add("code", "required", "Account code is required")
	}
	if len(a.Code) > 50 {
		ve.Add("code", "max_length", "Account code must be 50 characters or less")
	}
	if a.Name == "" {
		ve.Add("name", "required", "Account name is required")
	}
	if len(a.Name) > 255 {
		ve.Add("name", "max_length", "Account name must be 255 characters or less")
	}
	if !a.Type.IsValid() {
		ve.Add("type", "invalid", "Invalid account type")
	}
	if a.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}

	if a.IsControl && a.IsPosting {
		ve.Add("is_posting", "invalid", "Control accounts cannot be posting accounts")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (a *Account) SetParent(parentID common.ID) error {
	if parentID == a.ID {
		return fmt.Errorf("account cannot be its own parent")
	}
	a.ParentID = &parentID
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) Activate() {
	a.IsActive = true
	a.UpdatedAt = time.Now()
}

func (a *Account) Deactivate() error {

	a.IsActive = false
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Account) MakeControl() {
	a.IsControl = true
	a.IsPosting = false
	a.UpdatedAt = time.Now()
}

func (a *Account) CanPost() bool {
	return a.IsPosting && a.IsActive
}

func (a *Account) IsDebitNormal() bool {
	return a.NormalBalance == BalanceTypeDebit
}

func (a *Account) IsCreditNormal() bool {
	return a.NormalBalance == BalanceTypeCredit
}

func (a *Account) IsBalanceSheet() bool {
	switch a.Type {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity:
		return true
	}
	return false
}

func (a *Account) IsIncomeStatement() bool {
	switch a.Type {
	case AccountTypeRevenue, AccountTypeExpense:
		return true
	}
	return false
}

func (a *Account) HasChildren() bool {
	return len(a.Children) > 0
}

type AccountFilter struct {
	EntityID    common.ID
	ParentID    *common.ID
	Type        *AccountType
	Subtype     *AccountSubtype
	IsControl   *bool
	IsPosting   *bool
	IsActive    *bool
	SearchQuery string
	Limit       int
	Offset      int
}
