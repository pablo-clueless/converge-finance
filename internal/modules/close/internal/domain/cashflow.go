package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

// CashFlowMethod defines the method used for cash flow statement
type CashFlowMethod string

const (
	CashFlowMethodIndirect CashFlowMethod = "indirect"
	CashFlowMethodDirect   CashFlowMethod = "direct"
)

// CashFlowCategory defines the cash flow statement category
type CashFlowCategory string

const (
	CashFlowCategoryOperating CashFlowCategory = "operating"
	CashFlowCategoryInvesting CashFlowCategory = "investing"
	CashFlowCategoryFinancing CashFlowCategory = "financing"
)

// CashFlowLineType defines the type of line item
type CashFlowLineType string

const (
	CashFlowLineTypeCashReceipt  CashFlowLineType = "cash_receipt"
	CashFlowLineTypeCashPayment  CashFlowLineType = "cash_payment"
	CashFlowLineTypeAdjustment   CashFlowLineType = "adjustment"
	CashFlowLineTypeSubtotal     CashFlowLineType = "subtotal"
	CashFlowLineTypeTotal        CashFlowLineType = "total"
)

// CashFlowRunStatus defines the status of a cash flow run
type CashFlowRunStatus string

const (
	CashFlowRunStatusPending    CashFlowRunStatus = "pending"
	CashFlowRunStatusGenerating CashFlowRunStatus = "generating"
	CashFlowRunStatusCompleted  CashFlowRunStatus = "completed"
	CashFlowRunStatusFailed     CashFlowRunStatus = "failed"
)

// AccountCashFlowConfig maps a GL account to cash flow categories
type AccountCashFlowConfig struct {
	ID               common.ID
	EntityID         common.ID
	AccountID        common.ID
	CashFlowCategory CashFlowCategory
	LineItemCode     string
	IsCashAccount    bool
	IsCashEquivalent bool
	AdjustmentType   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewAccountCashFlowConfig creates a new account cash flow configuration
func NewAccountCashFlowConfig(
	entityID, accountID common.ID,
	category CashFlowCategory,
	lineItemCode string,
) *AccountCashFlowConfig {
	now := time.Now()
	return &AccountCashFlowConfig{
		ID:               common.NewID(),
		EntityID:         entityID,
		AccountID:        accountID,
		CashFlowCategory: category,
		LineItemCode:     lineItemCode,
		IsCashAccount:    false,
		IsCashEquivalent: false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// SetCashAccount marks the account as a cash account
func (c *AccountCashFlowConfig) SetCashAccount(isCash, isCashEquivalent bool) {
	c.IsCashAccount = isCash
	c.IsCashEquivalent = isCashEquivalent
	c.UpdatedAt = time.Now()
}

// SetAdjustmentType sets the adjustment type for indirect method
func (c *AccountCashFlowConfig) SetAdjustmentType(adjustmentType string) {
	c.AdjustmentType = adjustmentType
	c.UpdatedAt = time.Now()
}

// CashFlowTemplate defines the structure for a cash flow statement
type CashFlowTemplate struct {
	ID            common.ID
	EntityID      *common.ID
	TemplateCode  string
	TemplateName  string
	Method        CashFlowMethod
	Configuration map[string]interface{}
	IsSystem      bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewCashFlowTemplate creates a new cash flow template
func NewCashFlowTemplate(
	entityID common.ID,
	templateCode, templateName string,
	method CashFlowMethod,
) *CashFlowTemplate {
	now := time.Now()
	return &CashFlowTemplate{
		ID:            common.NewID(),
		EntityID:      &entityID,
		TemplateCode:  templateCode,
		TemplateName:  templateName,
		Method:        method,
		Configuration: make(map[string]interface{}),
		IsSystem:      false,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CashFlowRun represents a generated cash flow statement
type CashFlowRun struct {
	ID             common.ID
	EntityID       common.ID
	RunNumber      string
	TemplateID     *common.ID
	FiscalPeriodID common.ID
	FiscalYearID   common.ID
	Method         CashFlowMethod
	PeriodStart    time.Time
	PeriodEnd      time.Time
	CurrencyCode   string
	OperatingNet   decimal.Decimal
	InvestingNet   decimal.Decimal
	FinancingNet   decimal.Decimal
	NetChange      decimal.Decimal
	OpeningCash    decimal.Decimal
	ClosingCash    decimal.Decimal
	FXEffect       decimal.Decimal
	Status         CashFlowRunStatus
	GeneratedBy    common.ID
	GeneratedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time

	Lines []CashFlowLine
}

// NewCashFlowRun creates a new cash flow run
func NewCashFlowRun(
	entityID common.ID,
	runNumber string,
	fiscalPeriodID, fiscalYearID common.ID,
	method CashFlowMethod,
	periodStart, periodEnd time.Time,
	currencyCode string,
	generatedBy common.ID,
) *CashFlowRun {
	now := time.Now()
	return &CashFlowRun{
		ID:             common.NewID(),
		EntityID:       entityID,
		RunNumber:      runNumber,
		FiscalPeriodID: fiscalPeriodID,
		FiscalYearID:   fiscalYearID,
		Method:         method,
		PeriodStart:    periodStart,
		PeriodEnd:      periodEnd,
		CurrencyCode:   currencyCode,
		OperatingNet:   decimal.Zero,
		InvestingNet:   decimal.Zero,
		FinancingNet:   decimal.Zero,
		NetChange:      decimal.Zero,
		OpeningCash:    decimal.Zero,
		ClosingCash:    decimal.Zero,
		FXEffect:       decimal.Zero,
		Status:         CashFlowRunStatusPending,
		GeneratedBy:    generatedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
		Lines:          []CashFlowLine{},
	}
}

// StartGeneration transitions the run to generating status
func (r *CashFlowRun) StartGeneration() {
	r.Status = CashFlowRunStatusGenerating
	now := time.Now()
	r.GeneratedAt = &now
	r.UpdatedAt = now
}

// Complete marks the run as completed
func (r *CashFlowRun) Complete() {
	r.Status = CashFlowRunStatusCompleted
	r.UpdatedAt = time.Now()
}

// Fail marks the run as failed
func (r *CashFlowRun) Fail() {
	r.Status = CashFlowRunStatusFailed
	r.UpdatedAt = time.Now()
}

// SetTotals sets the category totals
func (r *CashFlowRun) SetTotals(operating, investing, financing decimal.Decimal) {
	r.OperatingNet = operating
	r.InvestingNet = investing
	r.FinancingNet = financing
	r.NetChange = operating.Add(investing).Add(financing)
	r.UpdatedAt = time.Now()
}

// SetCashBalances sets the opening and closing cash balances
func (r *CashFlowRun) SetCashBalances(opening, closing, fxEffect decimal.Decimal) {
	r.OpeningCash = opening
	r.ClosingCash = closing
	r.FXEffect = fxEffect
	r.UpdatedAt = time.Now()
}

// AddLine adds a line to the cash flow run
func (r *CashFlowRun) AddLine(line CashFlowLine) {
	r.Lines = append(r.Lines, line)
}

// CashFlowLine represents a line item in a cash flow statement
type CashFlowLine struct {
	ID             common.ID
	CashFlowRunID  common.ID
	LineNumber     int
	Category       CashFlowCategory
	LineType       CashFlowLineType
	LineCode       string
	Description    string
	Amount         decimal.Decimal
	IndentLevel    int
	IsBold         bool
	SourceAccounts []string
	Calculation    string
	CreatedAt      time.Time
}

// NewCashFlowLine creates a new cash flow line
func NewCashFlowLine(
	cashFlowRunID common.ID,
	lineNumber int,
	category CashFlowCategory,
	lineType CashFlowLineType,
	lineCode, description string,
	amount decimal.Decimal,
) *CashFlowLine {
	return &CashFlowLine{
		ID:             common.NewID(),
		CashFlowRunID:  cashFlowRunID,
		LineNumber:     lineNumber,
		Category:       category,
		LineType:       lineType,
		LineCode:       lineCode,
		Description:    description,
		Amount:         amount,
		IndentLevel:    0,
		IsBold:         false,
		SourceAccounts: []string{},
		CreatedAt:      time.Now(),
	}
}

// SetFormatting sets the display formatting
func (l *CashFlowLine) SetFormatting(indentLevel int, isBold bool) {
	l.IndentLevel = indentLevel
	l.IsBold = isBold
}

// SetSourceAccounts sets the source accounts for audit trail
func (l *CashFlowLine) SetSourceAccounts(accounts []string) {
	l.SourceAccounts = accounts
}

// SetCalculation sets the calculation description
func (l *CashFlowLine) SetCalculation(calculation string) {
	l.Calculation = calculation
}
