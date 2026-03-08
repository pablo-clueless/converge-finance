package close

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type API interface {
	GetPeriodCloseStatus(ctx context.Context, entityID, fiscalPeriodID common.ID) (*PeriodCloseStatusResponse, error)
	IsPeriodOpen(ctx context.Context, entityID, fiscalPeriodID common.ID) (bool, error)
	IsPeriodClosed(ctx context.Context, entityID, fiscalPeriodID common.ID) (bool, error)

	SoftClosePeriod(ctx context.Context, entityID, fiscalPeriodID common.ID, userID common.ID) (*PeriodCloseStatusResponse, error)
	HardClosePeriod(ctx context.Context, entityID, fiscalPeriodID, fiscalYearID common.ID, closeDate time.Time, currency money.Currency, userID common.ID) (*CloseRunResponse, error)

	GenerateTrialBalance(ctx context.Context, entityID, fiscalPeriodID common.ID, asOfDate time.Time, userID common.ID) (*ReportRunResponse, error)
	GenerateIncomeStatement(ctx context.Context, entityID, fiscalPeriodID, fiscalYearID common.ID, asOfDate time.Time, userID common.ID) (*ReportRunResponse, error)
	GenerateBalanceSheet(ctx context.Context, entityID, fiscalPeriodID, fiscalYearID common.ID, asOfDate time.Time, userID common.ID) (*ReportRunResponse, error)
	GetReportRun(ctx context.Context, id common.ID) (*ReportRunResponse, error)

	// Cash Flow Statement methods
	ConfigureAccountCashFlow(ctx context.Context, req ConfigureAccountCashFlowRequest) (*AccountCashFlowConfigResponse, error)
	ListAccountCashFlowConfigs(ctx context.Context, entityID common.ID) ([]AccountCashFlowConfigResponse, error)
	GenerateCashFlowStatement(ctx context.Context, req GenerateCashFlowRequest) (*CashFlowRunResponse, error)
	GetCashFlowRun(ctx context.Context, id common.ID) (*CashFlowRunResponse, error)
	ListCashFlowRuns(ctx context.Context, req ListCashFlowRunsRequest) (*ListCashFlowRunsResponse, error)
}

type PeriodCloseStatusResponse struct {
	ID                    common.ID
	EntityID              common.ID
	FiscalPeriodID        common.ID
	FiscalYearID          common.ID
	Status                string
	SoftClosedAt          *time.Time
	HardClosedAt          *time.Time
	ClosingJournalEntryID *common.ID
}

type CloseRunResponse struct {
	ID                    common.ID
	EntityID              common.ID
	RunNumber             string
	CloseType             string
	FiscalPeriodID        common.ID
	CloseDate             time.Time
	Status                string
	RulesExecuted         int
	EntriesCreated        int
	TotalDebits           money.Money
	TotalCredits          money.Money
	ClosingJournalEntryID *common.ID
	CompletedAt           *time.Time
}

type ReportRunResponse struct {
	ID             common.ID
	EntityID       common.ID
	ReportNumber   string
	ReportType     string
	ReportFormat   string
	ReportName     string
	FiscalPeriodID *common.ID
	AsOfDate       time.Time
	Status         string
	GeneratedAt    *time.Time
	CompletedAt    *time.Time
	DataRows       []ReportDataRowResponse
}

type ReportDataRowResponse struct {
	RowNumber    int
	RowType      string
	IndentLevel  int
	AccountCode  string
	AccountName  string
	Description  string
	Amount1      *float64
	Amount2      *float64
	Amount3      *float64
	CurrencyCode string
	IsBold       bool
	IsUnderlined bool
}

// Cash Flow types

type ConfigureAccountCashFlowRequest struct {
	EntityID         common.ID
	AccountID        common.ID
	Category         string // operating, investing, financing
	LineItemCode     string
	IsCashAccount    bool
	IsCashEquivalent bool
	AdjustmentType   string // add_back, subtract
}

type AccountCashFlowConfigResponse struct {
	ID               common.ID
	EntityID         common.ID
	AccountID        common.ID
	Category         string
	LineItemCode     string
	IsCashAccount    bool
	IsCashEquivalent bool
	AdjustmentType   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type GenerateCashFlowRequest struct {
	EntityID       common.ID
	FiscalPeriodID common.ID
	FiscalYearID   common.ID
	Method         string // indirect, direct
	PeriodStart    time.Time
	PeriodEnd      time.Time
	CurrencyCode   string
	UserID         common.ID
}

type CashFlowRunResponse struct {
	ID             common.ID
	EntityID       common.ID
	RunNumber      string
	Method         string
	FiscalPeriodID common.ID
	FiscalYearID   common.ID
	PeriodStart    time.Time
	PeriodEnd      time.Time
	CurrencyCode   string
	OperatingNet   float64
	InvestingNet   float64
	FinancingNet   float64
	NetChange      float64
	OpeningCash    float64
	ClosingCash    float64
	FXEffect       float64
	Status         string
	GeneratedBy    common.ID
	GeneratedAt    *time.Time
	Lines          []CashFlowLineResponse
}

type CashFlowLineResponse struct {
	ID             common.ID
	LineNumber     int
	Category       string
	LineType       string
	LineCode       string
	Description    string
	Amount         float64
	IndentLevel    int
	IsBold         bool
	SourceAccounts []string
	Calculation    string
}

type ListCashFlowRunsRequest struct {
	EntityID       common.ID
	FiscalPeriodID common.ID
	FiscalYearID   common.ID
	Status         *string
	Limit          int
	Offset         int
}

type ListCashFlowRunsResponse struct {
	Runs  []CashFlowRunResponse
	Total int
}
