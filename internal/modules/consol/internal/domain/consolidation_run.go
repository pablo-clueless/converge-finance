package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type RunStatus string

const (
	RunStatusDraft      RunStatus = "draft"
	RunStatusInProgress RunStatus = "in_progress"
	RunStatusCompleted  RunStatus = "completed"
	RunStatusPosted     RunStatus = "posted"
	RunStatusReversed   RunStatus = "reversed"
)

func (s RunStatus) IsValid() bool {
	switch s {
	case RunStatusDraft, RunStatusInProgress, RunStatusCompleted, RunStatusPosted, RunStatusReversed:
		return true
	}
	return false
}

type ConsolidationRun struct {
	ID                  common.ID
	RunNumber           string
	ConsolidationSetID  common.ID
	FiscalPeriodID      common.ID
	ReportingCurrency   money.Currency
	ConsolidationDate   time.Time
	ClosingRateDate     time.Time
	AverageRateDate     *time.Time
	EntityCount         int
	TotalAssets         money.Money
	TotalLiabilities    money.Money
	TotalEquity         money.Money
	TotalRevenue        money.Money
	TotalExpenses       money.Money
	NetIncome           money.Money
	TotalCTA            money.Money
	TotalMinorityInterest money.Money
	Status              RunStatus
	JournalEntryID      *common.ID
	CreatedBy           common.ID
	CompletedBy         *common.ID
	PostedBy            *common.ID
	ReversedBy          *common.ID
	CreatedAt           time.Time
	UpdatedAt           time.Time
	CompletedAt         *time.Time
	PostedAt            *time.Time
	ReversedAt          *time.Time

	ConsolidationSetCode string
	ConsolidationSetName string
	FiscalPeriodName     string
}

func NewConsolidationRun(
	runNumber string,
	consolidationSetID common.ID,
	fiscalPeriodID common.ID,
	reportingCurrency money.Currency,
	consolidationDate time.Time,
	closingRateDate time.Time,
	createdBy common.ID,
) (*ConsolidationRun, error) {
	if runNumber == "" {
		return nil, fmt.Errorf("run number is required")
	}

	return &ConsolidationRun{
		ID:                 common.NewID(),
		RunNumber:          runNumber,
		ConsolidationSetID: consolidationSetID,
		FiscalPeriodID:     fiscalPeriodID,
		ReportingCurrency:  reportingCurrency,
		ConsolidationDate:  consolidationDate,
		ClosingRateDate:    closingRateDate,
		TotalAssets:        money.Zero(reportingCurrency),
		TotalLiabilities:   money.Zero(reportingCurrency),
		TotalEquity:        money.Zero(reportingCurrency),
		TotalRevenue:       money.Zero(reportingCurrency),
		TotalExpenses:      money.Zero(reportingCurrency),
		NetIncome:          money.Zero(reportingCurrency),
		TotalCTA:           money.Zero(reportingCurrency),
		TotalMinorityInterest: money.Zero(reportingCurrency),
		Status:             RunStatusDraft,
		CreatedBy:          createdBy,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}, nil
}

func (r *ConsolidationRun) StartProcessing() error {
	if r.Status != RunStatusDraft {
		return fmt.Errorf("can only start processing from draft status")
	}
	r.Status = RunStatusInProgress
	r.UpdatedAt = time.Now()
	return nil
}

func (r *ConsolidationRun) Complete(completedBy common.ID) error {
	if r.Status != RunStatusInProgress {
		return fmt.Errorf("can only complete from in_progress status")
	}
	r.Status = RunStatusCompleted
	r.CompletedBy = &completedBy
	now := time.Now()
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *ConsolidationRun) Post(postedBy common.ID, journalEntryID common.ID) error {
	if r.Status != RunStatusCompleted {
		return fmt.Errorf("can only post from completed status")
	}
	r.Status = RunStatusPosted
	r.PostedBy = &postedBy
	r.JournalEntryID = &journalEntryID
	now := time.Now()
	r.PostedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *ConsolidationRun) Reverse(reversedBy common.ID) error {
	if r.Status != RunStatusPosted {
		return fmt.Errorf("can only reverse from posted status")
	}
	r.Status = RunStatusReversed
	r.ReversedBy = &reversedBy
	now := time.Now()
	r.ReversedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *ConsolidationRun) SetAverageRateDate(date time.Time) {
	r.AverageRateDate = &date
	r.UpdatedAt = time.Now()
}

func (r *ConsolidationRun) UpdateTotals(
	entityCount int,
	assets, liabilities, equity, revenue, expenses money.Money,
	cta, minorityInterest money.Money,
) {
	r.EntityCount = entityCount
	r.TotalAssets = assets
	r.TotalLiabilities = liabilities
	r.TotalEquity = equity
	r.TotalRevenue = revenue
	r.TotalExpenses = expenses
	r.NetIncome = revenue.MustSubtract(expenses)
	r.TotalCTA = cta
	r.TotalMinorityInterest = minorityInterest
	r.UpdatedAt = time.Now()
}

func (r *ConsolidationRun) CanModify() bool {
	return r.Status == RunStatusDraft || r.Status == RunStatusInProgress
}

type ConsolidationRunFilter struct {
	ConsolidationSetID *common.ID
	FiscalPeriodID     *common.ID
	Status             *RunStatus
	DateFrom           *time.Time
	DateTo             *time.Time
	Limit              int
	Offset             int
}

type EntityBalance struct {
	ID                   common.ID
	ConsolidationRunID   common.ID
	EntityID             common.ID
	AccountID            common.ID
	FunctionalCurrency   money.Currency
	FunctionalDebit      money.Money
	FunctionalCredit     money.Money
	FunctionalBalance    money.Money
	ExchangeRate         float64
	RateType             RateType
	TranslatedDebit      money.Money
	TranslatedCredit     money.Money
	TranslatedBalance    money.Money
	TranslationDifference money.Money
	CreatedAt            time.Time

	EntityCode    string
	EntityName    string
	AccountCode   string
	AccountName   string
	AccountType   string
}

func NewEntityBalance(
	consolidationRunID common.ID,
	entityID common.ID,
	accountID common.ID,
	functionalCurrency money.Currency,
	reportingCurrency money.Currency,
) *EntityBalance {
	return &EntityBalance{
		ID:                   common.NewID(),
		ConsolidationRunID:   consolidationRunID,
		EntityID:             entityID,
		AccountID:            accountID,
		FunctionalCurrency:   functionalCurrency,
		FunctionalDebit:      money.Zero(functionalCurrency),
		FunctionalCredit:     money.Zero(functionalCurrency),
		FunctionalBalance:    money.Zero(functionalCurrency),
		ExchangeRate:         1.0,
		RateType:             RateTypeClosing,
		TranslatedDebit:      money.Zero(reportingCurrency),
		TranslatedCredit:     money.Zero(reportingCurrency),
		TranslatedBalance:    money.Zero(reportingCurrency),
		TranslationDifference: money.Zero(reportingCurrency),
		CreatedAt:            time.Now(),
	}
}

func (b *EntityBalance) SetFunctionalAmounts(debit, credit money.Money) {
	b.FunctionalDebit = debit
	b.FunctionalCredit = credit
	b.FunctionalBalance = debit.MustSubtract(credit)
}

func (b *EntityBalance) Translate(rate float64, rateType RateType, reportingCurrency money.Currency) {
	b.ExchangeRate = rate
	b.RateType = rateType

	rateDecimal := decimal.NewFromFloat(rate)
	b.TranslatedDebit = b.FunctionalDebit.Convert(reportingCurrency, rateDecimal)
	b.TranslatedCredit = b.FunctionalCredit.Convert(reportingCurrency, rateDecimal)
	b.TranslatedBalance = b.TranslatedDebit.MustSubtract(b.TranslatedCredit)
}

type ConsolidatedBalance struct {
	ID                   common.ID
	ConsolidationRunID   common.ID
	AccountID            common.ID
	OpeningBalance       money.Money
	PeriodDebit          money.Money
	PeriodCredit         money.Money
	EliminationDebit     money.Money
	EliminationCredit    money.Money
	TranslationAdjustment money.Money
	MinorityInterest     money.Money
	ClosingBalance       money.Money
	CreatedAt            time.Time

	AccountCode string
	AccountName string
	AccountType string
}

func NewConsolidatedBalance(
	consolidationRunID common.ID,
	accountID common.ID,
	reportingCurrency money.Currency,
) *ConsolidatedBalance {
	return &ConsolidatedBalance{
		ID:                   common.NewID(),
		ConsolidationRunID:   consolidationRunID,
		AccountID:            accountID,
		OpeningBalance:       money.Zero(reportingCurrency),
		PeriodDebit:          money.Zero(reportingCurrency),
		PeriodCredit:         money.Zero(reportingCurrency),
		EliminationDebit:     money.Zero(reportingCurrency),
		EliminationCredit:    money.Zero(reportingCurrency),
		TranslationAdjustment: money.Zero(reportingCurrency),
		MinorityInterest:     money.Zero(reportingCurrency),
		ClosingBalance:       money.Zero(reportingCurrency),
		CreatedAt:            time.Now(),
	}
}

func (b *ConsolidatedBalance) AddEntityBalance(entityBalance EntityBalance) {
	b.PeriodDebit = b.PeriodDebit.MustAdd(entityBalance.TranslatedDebit)
	b.PeriodCredit = b.PeriodCredit.MustAdd(entityBalance.TranslatedCredit)
}

func (b *ConsolidatedBalance) AddElimination(debit, credit money.Money) {
	b.EliminationDebit = b.EliminationDebit.MustAdd(debit)
	b.EliminationCredit = b.EliminationCredit.MustAdd(credit)
}

func (b *ConsolidatedBalance) SetTranslationAdjustment(amount money.Money) {
	b.TranslationAdjustment = amount
}

func (b *ConsolidatedBalance) SetMinorityInterest(amount money.Money) {
	b.MinorityInterest = amount
}

func (b *ConsolidatedBalance) CalculateClosingBalance() {
	netDebit := b.PeriodDebit.MustAdd(b.EliminationDebit)
	netCredit := b.PeriodCredit.MustAdd(b.EliminationCredit)
	netBalance := netDebit.MustSubtract(netCredit)

	b.ClosingBalance = netBalance.
		MustAdd(b.TranslationAdjustment).
		MustSubtract(b.MinorityInterest)
}
