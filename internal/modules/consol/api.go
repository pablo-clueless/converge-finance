package consol

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type API interface {
	GetConsolidationSet(ctx context.Context, id common.ID) (*ConsolidationSetResponse, error)
	ListConsolidationSets(ctx context.Context, parentEntityID common.ID) ([]ConsolidationSetResponse, error)
	InitiateConsolidationRun(ctx context.Context, req InitiateRunRequest) (*ConsolidationRunResponse, error)
	ExecuteConsolidation(ctx context.Context, runID common.ID) error
	PostConsolidation(ctx context.Context, runID common.ID) error
	GetConsolidationRun(ctx context.Context, runID common.ID) (*ConsolidationRunResponse, error)
	GetConsolidatedTrialBalance(ctx context.Context, runID common.ID) (*ConsolidatedTrialBalanceResponse, error)
	GetExchangeRate(ctx context.Context, fromCurrency, toCurrency money.Currency, date time.Time) (*ExchangeRateResponse, error)
	TranslateAmount(ctx context.Context, amount money.Money, toCurrency money.Currency, date time.Time) (money.Money, float64, error)
}

type InitiateRunRequest struct {
	ConsolidationSetID common.ID
	FiscalPeriodID     common.ID
	ConsolidationDate  time.Time
	ClosingRateDate    time.Time
	AverageRateDate    *time.Time
}

type ConsolidationSetResponse struct {
	ID                       common.ID
	SetCode                  string
	SetName                  string
	Description              string
	ParentEntityID           common.ID
	ReportingCurrency        string
	DefaultTranslationMethod string
	IsActive                 bool
	Members                  []ConsolidationSetMemberResponse
}

type ConsolidationSetMemberResponse struct {
	ID                  common.ID
	EntityID            common.ID
	EntityCode          string
	EntityName          string
	OwnershipPercent    float64
	MinorityPercent     float64
	ConsolidationMethod string
	FunctionalCurrency  string
	IsActive            bool
}

type ConsolidationRunResponse struct {
	ID                    common.ID
	RunNumber             string
	ConsolidationSetID    common.ID
	FiscalPeriodID        common.ID
	ReportingCurrency     string
	ConsolidationDate     time.Time
	ClosingRateDate       time.Time
	AverageRateDate       *time.Time
	EntityCount           int
	TotalAssets           money.Money
	TotalLiabilities      money.Money
	TotalEquity           money.Money
	TotalRevenue          money.Money
	TotalExpenses         money.Money
	NetIncome             money.Money
	TotalCTA              money.Money
	TotalMinorityInterest money.Money
	Status                string
	JournalEntryID        *common.ID
	CreatedAt             time.Time
	CompletedAt           *time.Time
	PostedAt              *time.Time
}

type ConsolidatedTrialBalanceResponse struct {
	RunID             common.ID
	ReportingCurrency string
	Accounts          []ConsolidatedAccountBalance
	TotalAssets       money.Money
	TotalLiabilities  money.Money
	TotalEquity       money.Money
	TotalRevenue      money.Money
	TotalExpenses     money.Money
	NetIncome         money.Money
	IsBalanced        bool
}

type ConsolidatedAccountBalance struct {
	AccountID             common.ID
	AccountCode           string
	AccountName           string
	AccountType           string
	OpeningBalance        money.Money
	PeriodDebit           money.Money
	PeriodCredit          money.Money
	EliminationDebit      money.Money
	EliminationCredit     money.Money
	TranslationAdjustment money.Money
	MinorityInterest      money.Money
	ClosingBalance        money.Money
}

type ExchangeRateResponse struct {
	ID             common.ID
	FromCurrency   string
	ToCurrency     string
	RateDate       time.Time
	ClosingRate    float64
	AverageRate    *float64
	HistoricalRate *float64
}
