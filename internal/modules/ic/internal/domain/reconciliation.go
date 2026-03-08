package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type EntityPairBalance struct {
	ID                common.ID
	FromEntityID      common.ID
	ToEntityID        common.ID
	FiscalPeriodID    common.ID
	Currency          money.Currency
	OpeningBalance    money.Money
	PeriodDebits      money.Money
	PeriodCredits     money.Money
	ClosingBalance    money.Money
	IsReconciled      bool
	DiscrepancyAmount money.Money
	LastReconciledAt  *time.Time
	UpdatedAt         time.Time
}

func NewEntityPairBalance(
	fromEntityID common.ID,
	toEntityID common.ID,
	fiscalPeriodID common.ID,
	currency money.Currency,
) *EntityPairBalance {
	zero := money.Zero(currency)
	return &EntityPairBalance{
		ID:                common.NewID(),
		FromEntityID:      fromEntityID,
		ToEntityID:        toEntityID,
		FiscalPeriodID:    fiscalPeriodID,
		Currency:          currency,
		OpeningBalance:    zero,
		PeriodDebits:      zero,
		PeriodCredits:     zero,
		ClosingBalance:    zero,
		IsReconciled:      false,
		DiscrepancyAmount: zero,
		UpdatedAt:         time.Now(),
	}
}

func (b *EntityPairBalance) SetOpeningBalance(balance money.Money) {
	b.OpeningBalance = balance
	b.recalculateClosing()
}

func (b *EntityPairBalance) AddDebit(amount money.Money) {
	b.PeriodDebits = b.PeriodDebits.MustAdd(amount)
	b.recalculateClosing()
}

func (b *EntityPairBalance) AddCredit(amount money.Money) {
	b.PeriodCredits = b.PeriodCredits.MustAdd(amount)
	b.recalculateClosing()
}

func (b *EntityPairBalance) recalculateClosing() {
	b.ClosingBalance = b.OpeningBalance.MustAdd(b.PeriodDebits).MustSubtract(b.PeriodCredits)
	b.UpdatedAt = time.Now()
}

func (b *EntityPairBalance) SetDiscrepancy(discrepancy money.Money) {
	b.DiscrepancyAmount = discrepancy
	b.IsReconciled = discrepancy.IsZero()
	if b.IsReconciled {
		now := time.Now()
		b.LastReconciledAt = &now
	}
	b.UpdatedAt = time.Now()
}

func (b *EntityPairBalance) MarkReconciled() {
	now := time.Now()
	b.IsReconciled = true
	b.DiscrepancyAmount = money.Zero(b.Currency)
	b.LastReconciledAt = &now
	b.UpdatedAt = now
}

func (b *EntityPairBalance) GetNetMovement() money.Money {
	return b.PeriodDebits.MustSubtract(b.PeriodCredits)
}

func (b *EntityPairBalance) HasActivity() bool {
	return !b.PeriodDebits.IsZero() || !b.PeriodCredits.IsZero()
}

type ReconciliationDiscrepancy struct {
	FromEntityID   common.ID
	FromEntityCode string
	FromEntityName string
	ToEntityID     common.ID
	ToEntityCode   string
	ToEntityName   string
	FiscalPeriodID common.ID
	PeriodName     string
	Currency       money.Currency

	FromBalance money.Money

	ToBalance money.Money

	DiscrepancyAmount money.Money

	FromUnreconciledCount int
	ToUnreconciledCount   int

	FromLastActivityDate *time.Time
	ToLastActivityDate   *time.Time
}

func (d *ReconciliationDiscrepancy) IsReconciled() bool {
	return d.DiscrepancyAmount.IsZero()
}

func (d *ReconciliationDiscrepancy) GetExpectedToBalance() money.Money {
	return d.FromBalance.Negate()
}

type ReconciliationSummary struct {
	ParentEntityID common.ID
	FiscalPeriodID common.ID
	AsOfDate       time.Time

	TotalEntityPairs  int
	ReconciledPairs   int
	UnreconciledPairs int
	DisputedPairs     int

	TotalDiscrepancy   money.Money
	LargestDiscrepancy money.Money

	TotalTransactions   int
	ReconciledTxCount   int
	UnreconciledTxCount int
	DisputedTxCount     int

	Discrepancies []ReconciliationDiscrepancy
}

func (s *ReconciliationSummary) GetReconciliationPercentage() decimal.Decimal {
	if s.TotalEntityPairs == 0 {
		return decimal.NewFromInt(100)
	}
	return decimal.NewFromInt(int64(s.ReconciledPairs)).
		Div(decimal.NewFromInt(int64(s.TotalEntityPairs))).
		Mul(decimal.NewFromInt(100))
}

func (s *ReconciliationSummary) HasDiscrepancies() bool {
	return s.UnreconciledPairs > 0 || !s.TotalDiscrepancy.IsZero()
}

type EntityPairReconciliation struct {
	FromEntityID   common.ID
	FromEntityCode string
	FromEntityName string
	ToEntityID     common.ID
	ToEntityCode   string
	ToEntityName   string
	FiscalPeriodID common.ID

	FromOpeningBalance money.Money
	ToOpeningBalance   money.Money
	FromClosingBalance money.Money
	ToClosingBalance   money.Money
	DiscrepancyAmount  money.Money
	IsReconciled       bool

	FromTransactions []ICTransaction

	ToTransactions []ICTransaction

	MatchedTransactions []TransactionMatch

	UnmatchedFromTransactions []ICTransaction
	UnmatchedToTransactions   []ICTransaction
}

type TransactionMatch struct {
	FromTransaction ICTransaction
	ToTransaction   ICTransaction
	MatchType       MatchType
	Difference      money.Money
}

type MatchType string

const (
	MatchTypeExact     MatchType = "exact"
	MatchTypeAmount    MatchType = "amount"
	MatchTypeReference MatchType = "reference"
	MatchTypeManual    MatchType = "manual"
)

func (t MatchType) IsValid() bool {
	switch t {
	case MatchTypeExact, MatchTypeAmount, MatchTypeReference, MatchTypeManual:
		return true
	}
	return false
}

type EntityPairBalanceFilter struct {
	FromEntityID   *common.ID
	ToEntityID     *common.ID
	FiscalPeriodID *common.ID
	IsReconciled   *bool
	HasActivity    *bool
	Limit          int
	Offset         int
}
