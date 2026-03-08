package gl

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type API interface {
	CreateJournalEntry(ctx context.Context, req CreateJournalEntryRequest) (*JournalEntryResponse, error)
	PostJournalEntry(ctx context.Context, entryID common.ID) error
	ReverseJournalEntry(ctx context.Context, entryID common.ID, reversalDate time.Time) (*JournalEntryResponse, error)
	GetJournalEntry(ctx context.Context, entryID common.ID) (*JournalEntryResponse, error)

	GetAccountByID(ctx context.Context, accountID common.ID) (*AccountResponse, error)
	GetAccountByCode(ctx context.Context, entityID common.ID, code string) (*AccountResponse, error)
	ListAccounts(ctx context.Context, entityID common.ID, filter AccountFilterRequest) ([]AccountResponse, error)

	ValidatePeriodOpen(ctx context.Context, entityID common.ID, date time.Time) error
	GetFiscalPeriodForDate(ctx context.Context, entityID common.ID, date time.Time) (*FiscalPeriodResponse, error)
	GetOpenPeriods(ctx context.Context, entityID common.ID) ([]FiscalPeriodResponse, error)

	GetAccountBalance(ctx context.Context, accountID common.ID, periodID common.ID) (*AccountBalanceResponse, error)
	GetTrialBalance(ctx context.Context, entityID common.ID, periodID common.ID) (*TrialBalanceResponse, error)
}

type CreateJournalEntryRequest struct {
	EntityID    common.ID
	EntryDate   time.Time
	Description string

	CurrencyCode string

	Lines []JournalLineRequest
}

type JournalLineRequest struct {
	AccountID   common.ID
	Description string
	Debit       money.Money
	Credit      money.Money
}

type JournalEntryResponse struct {
	ID              common.ID
	EntityID        common.ID
	EntryNumber     string
	FiscalPeriodID  common.ID
	EntryDate       time.Time
	PostingDate     *time.Time
	Description     string
	Source          string
	SourceReference string
	Status          string
	CurrencyCode    string
	TotalDebit      money.Money
	TotalCredit     money.Money
	Lines           []JournalLineResponse
	CreatedAt       time.Time
	PostedAt        *time.Time
}

type JournalLineResponse struct {
	ID           common.ID
	LineNumber   int
	AccountID    common.ID
	AccountCode  string
	AccountName  string
	Description  string
	DebitAmount  money.Money
	CreditAmount money.Money
}

type AccountResponse struct {
	ID            common.ID
	EntityID      common.ID
	ParentID      *common.ID
	Code          string
	Name          string
	Type          string
	Subtype       string
	CurrencyCode  string
	IsControl     bool
	IsPosting     bool
	IsActive      bool
	NormalBalance string
}

type AccountFilterRequest struct {
	Type      *string
	IsActive  *bool
	IsPosting *bool
	Search    string
	Limit     int
	Offset    int
}

type FiscalPeriodResponse struct {
	ID           common.ID
	EntityID     common.ID
	FiscalYearID common.ID
	PeriodNumber int
	PeriodName   string
	StartDate    time.Time
	EndDate      time.Time
	Status       string
	IsAdjustment bool
}

type AccountBalanceResponse struct {
	AccountID     common.ID
	AccountCode   string
	AccountName   string
	PeriodID      common.ID
	OpeningDebit  money.Money
	OpeningCredit money.Money
	PeriodDebit   money.Money
	PeriodCredit  money.Money
	ClosingDebit  money.Money
	ClosingCredit money.Money
}

type TrialBalanceResponse struct {
	EntityID    common.ID
	PeriodID    common.ID
	PeriodName  string
	AsOfDate    time.Time
	Accounts    []TrialBalanceLineResponse
	TotalDebit  money.Money
	TotalCredit money.Money
	IsBalanced  bool
}

type TrialBalanceLineResponse struct {
	AccountID   common.ID
	AccountCode string
	AccountName string
	AccountType string
	Debit       money.Money
	Credit      money.Money
}
