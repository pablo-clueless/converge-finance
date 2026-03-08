package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type BudgetStatus string

const (
	BudgetStatusDraft     BudgetStatus = "draft"
	BudgetStatusSubmitted BudgetStatus = "submitted"
	BudgetStatusApproved  BudgetStatus = "approved"
	BudgetStatusRejected  BudgetStatus = "rejected"
	BudgetStatusActive    BudgetStatus = "active"
	BudgetStatusClosed    BudgetStatus = "closed"
)

func (s BudgetStatus) IsValid() bool {
	switch s {
	case BudgetStatusDraft, BudgetStatusSubmitted, BudgetStatusApproved,
		BudgetStatusRejected, BudgetStatusActive, BudgetStatusClosed:
		return true
	}
	return false
}

type BudgetType string

const (
	BudgetTypeOperating BudgetType = "operating"
	BudgetTypeCapital   BudgetType = "capital"
	BudgetTypeCash      BudgetType = "cash"
	BudgetTypeProject   BudgetType = "project"
	BudgetTypeRolling   BudgetType = "rolling"
)

func (t BudgetType) IsValid() bool {
	switch t {
	case BudgetTypeOperating, BudgetTypeCapital, BudgetTypeCash,
		BudgetTypeProject, BudgetTypeRolling:
		return true
	}
	return false
}

type Budget struct {
	ID               common.ID
	EntityID         common.ID
	BudgetCode       string
	BudgetName       string
	Description      string
	BudgetType       BudgetType
	FiscalYearID     common.ID
	VersionNumber    int
	IsCurrentVersion bool
	ParentVersionID  *common.ID
	Currency         money.Currency
	TotalRevenue     money.Money
	TotalExpenses    money.Money
	NetBudget        money.Money
	Status           BudgetStatus
	SubmittedBy      *common.ID
	SubmittedAt      *time.Time
	ApprovedBy       *common.ID
	ApprovedAt       *time.Time
	RejectedBy       *common.ID
	RejectedAt       *time.Time
	RejectionReason  string
	CreatedBy        common.ID
	CreatedAt        time.Time
	UpdatedAt        time.Time

	FiscalYearName string
	Lines          []BudgetLine
}

func NewBudget(
	entityID common.ID,
	budgetCode string,
	budgetName string,
	budgetType BudgetType,
	fiscalYearID common.ID,
	currency money.Currency,
	createdBy common.ID,
) (*Budget, error) {
	if budgetCode == "" {
		return nil, fmt.Errorf("budget code is required")
	}
	if budgetName == "" {
		return nil, fmt.Errorf("budget name is required")
	}
	if !budgetType.IsValid() {
		return nil, fmt.Errorf("invalid budget type")
	}

	return &Budget{
		ID:               common.NewID(),
		EntityID:         entityID,
		BudgetCode:       budgetCode,
		BudgetName:       budgetName,
		BudgetType:       budgetType,
		FiscalYearID:     fiscalYearID,
		VersionNumber:    1,
		IsCurrentVersion: true,
		Currency:         currency,
		TotalRevenue:     money.Zero(currency),
		TotalExpenses:    money.Zero(currency),
		NetBudget:        money.Zero(currency),
		Status:           BudgetStatusDraft,
		CreatedBy:        createdBy,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

func (b *Budget) Submit(submittedBy common.ID) error {
	if b.Status != BudgetStatusDraft && b.Status != BudgetStatusRejected {
		return fmt.Errorf("can only submit from draft or rejected status")
	}
	b.Status = BudgetStatusSubmitted
	b.SubmittedBy = &submittedBy
	now := time.Now()
	b.SubmittedAt = &now
	b.UpdatedAt = now
	return nil
}

func (b *Budget) Approve(approvedBy common.ID) error {
	if b.Status != BudgetStatusSubmitted {
		return fmt.Errorf("can only approve from submitted status")
	}
	b.Status = BudgetStatusApproved
	b.ApprovedBy = &approvedBy
	now := time.Now()
	b.ApprovedAt = &now
	b.UpdatedAt = now
	return nil
}

func (b *Budget) Reject(rejectedBy common.ID, reason string) error {
	if b.Status != BudgetStatusSubmitted {
		return fmt.Errorf("can only reject from submitted status")
	}
	b.Status = BudgetStatusRejected
	b.RejectedBy = &rejectedBy
	b.RejectionReason = reason
	now := time.Now()
	b.RejectedAt = &now
	b.UpdatedAt = now
	return nil
}

func (b *Budget) Activate() error {
	if b.Status != BudgetStatusApproved {
		return fmt.Errorf("can only activate from approved status")
	}
	b.Status = BudgetStatusActive
	b.UpdatedAt = time.Now()
	return nil
}

func (b *Budget) Close() error {
	if b.Status != BudgetStatusActive {
		return fmt.Errorf("can only close from active status")
	}
	b.Status = BudgetStatusClosed
	b.UpdatedAt = time.Now()
	return nil
}

func (b *Budget) CreateNewVersion() *Budget {
	newVersion := &Budget{
		ID:               common.NewID(),
		EntityID:         b.EntityID,
		BudgetCode:       b.BudgetCode,
		BudgetName:       b.BudgetName,
		Description:      b.Description,
		BudgetType:       b.BudgetType,
		FiscalYearID:     b.FiscalYearID,
		VersionNumber:    b.VersionNumber + 1,
		IsCurrentVersion: true,
		ParentVersionID:  &b.ID,
		Currency:         b.Currency,
		TotalRevenue:     money.Zero(b.Currency),
		TotalExpenses:    money.Zero(b.Currency),
		NetBudget:        money.Zero(b.Currency),
		Status:           BudgetStatusDraft,
		CreatedBy:        b.CreatedBy,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	b.IsCurrentVersion = false
	b.UpdatedAt = time.Now()

	return newVersion
}

func (b *Budget) AddLine(line BudgetLine) {
	line.BudgetID = b.ID
	b.Lines = append(b.Lines, line)
	b.recalculateTotals()
}

func (b *Budget) recalculateTotals() {
	b.UpdatedAt = time.Now()
}

func (b *Budget) CanModify() bool {
	return b.Status == BudgetStatusDraft || b.Status == BudgetStatusRejected
}

type BudgetLine struct {
	ID             common.ID
	BudgetID       common.ID
	AccountID      common.ID
	CostCenterID   *common.ID
	FiscalPeriodID common.ID
	BudgetAmount   money.Money
	Quantity       *float64
	UnitCost       *float64
	Notes          string
	CreatedAt      time.Time
	UpdatedAt      time.Time

	AccountCode      string
	AccountName      string
	CostCenterCode   string
	CostCenterName   string
	FiscalPeriodName string
}

func NewBudgetLine(
	accountID common.ID,
	fiscalPeriodID common.ID,
	budgetAmount money.Money,
) *BudgetLine {
	return &BudgetLine{
		ID:             common.NewID(),
		AccountID:      accountID,
		FiscalPeriodID: fiscalPeriodID,
		BudgetAmount:   budgetAmount,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func (l *BudgetLine) SetCostCenter(costCenterID common.ID) {
	l.CostCenterID = &costCenterID
	l.UpdatedAt = time.Now()
}

func (l *BudgetLine) SetQuantityAndUnitCost(quantity, unitCost float64) {
	l.Quantity = &quantity
	l.UnitCost = &unitCost
	l.UpdatedAt = time.Now()
}

type BudgetTransfer struct {
	ID               common.ID
	BudgetID         common.ID
	TransferNumber   string
	TransferDate     time.Time
	FromAccountID    common.ID
	FromCostCenterID *common.ID
	FromPeriodID     common.ID
	ToAccountID      common.ID
	ToCostCenterID   *common.ID
	ToPeriodID       common.ID
	TransferAmount   money.Money
	Reason           string
	RequestedBy      common.ID
	ApprovedBy       *common.ID
	ApprovedAt       *time.Time
	IsApproved       bool
	CreatedAt        time.Time

	FromAccountCode    string
	ToAccountCode      string
	FromCostCenterCode string
	ToCostCenterCode   string
}

func NewBudgetTransfer(
	budgetID common.ID,
	transferNumber string,
	transferDate time.Time,
	fromAccountID common.ID,
	fromPeriodID common.ID,
	toAccountID common.ID,
	toPeriodID common.ID,
	transferAmount money.Money,
	reason string,
	requestedBy common.ID,
) (*BudgetTransfer, error) {
	if !transferAmount.IsPositive() {
		return nil, fmt.Errorf("transfer amount must be positive")
	}
	if reason == "" {
		return nil, fmt.Errorf("transfer reason is required")
	}

	return &BudgetTransfer{
		ID:             common.NewID(),
		BudgetID:       budgetID,
		TransferNumber: transferNumber,
		TransferDate:   transferDate,
		FromAccountID:  fromAccountID,
		FromPeriodID:   fromPeriodID,
		ToAccountID:    toAccountID,
		ToPeriodID:     toPeriodID,
		TransferAmount: transferAmount,
		Reason:         reason,
		RequestedBy:    requestedBy,
		IsApproved:     false,
		CreatedAt:      time.Now(),
	}, nil
}

func (t *BudgetTransfer) Approve(approvedBy common.ID) {
	t.ApprovedBy = &approvedBy
	now := time.Now()
	t.ApprovedAt = &now
	t.IsApproved = true
}

type BudgetFilter struct {
	EntityID     *common.ID
	FiscalYearID *common.ID
	BudgetType   *BudgetType
	Status       *BudgetStatus
	CurrentOnly  bool
	Search       string
	Limit        int
	Offset       int
}

type BudgetLineFilter struct {
	BudgetID       *common.ID
	AccountID      *common.ID
	CostCenterID   *common.ID
	FiscalPeriodID *common.ID
	Limit          int
	Offset         int
}
