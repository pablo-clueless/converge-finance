package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

type SegmentBalance struct {
	ID             common.ID
	EntityID       common.ID
	SegmentID      common.ID
	FiscalPeriodID common.ID
	AccountID      common.ID
	DebitAmount    decimal.Decimal
	CreditAmount   decimal.Decimal
	NetAmount      decimal.Decimal
	CurrencyCode   string
	LastUpdated    time.Time
}

func NewSegmentBalance(
	entityID, segmentID, fiscalPeriodID, accountID common.ID,
	currencyCode string,
) *SegmentBalance {
	return &SegmentBalance{
		ID:             common.NewID(),
		EntityID:       entityID,
		SegmentID:      segmentID,
		FiscalPeriodID: fiscalPeriodID,
		AccountID:      accountID,
		DebitAmount:    decimal.Zero,
		CreditAmount:   decimal.Zero,
		NetAmount:      decimal.Zero,
		CurrencyCode:   currencyCode,
		LastUpdated:    time.Now(),
	}
}

func (b *SegmentBalance) AddDebit(amount decimal.Decimal) {
	b.DebitAmount = b.DebitAmount.Add(amount)
	b.NetAmount = b.DebitAmount.Sub(b.CreditAmount)
	b.LastUpdated = time.Now()
}

func (b *SegmentBalance) AddCredit(amount decimal.Decimal) {
	b.CreditAmount = b.CreditAmount.Add(amount)
	b.NetAmount = b.DebitAmount.Sub(b.CreditAmount)
	b.LastUpdated = time.Now()
}

func (b *SegmentBalance) SetAmounts(debit, credit decimal.Decimal) {
	b.DebitAmount = debit
	b.CreditAmount = credit
	b.NetAmount = debit.Sub(credit)
	b.LastUpdated = time.Now()
}

func (b *SegmentBalance) IsZero() bool {
	return b.DebitAmount.IsZero() && b.CreditAmount.IsZero()
}

type SegmentBalanceSummary struct {
	SegmentID      common.ID
	SegmentCode    string
	SegmentName    string
	TotalDebit     decimal.Decimal
	TotalCredit    decimal.Decimal
	NetAmount      decimal.Decimal
	AccountCount   int
}

type AccountBalanceBySegment struct {
	AccountID    common.ID
	AccountCode  string
	AccountName  string
	Balances     []SegmentAccountBalance
}

type SegmentAccountBalance struct {
	SegmentID    common.ID
	SegmentCode  string
	SegmentName  string
	DebitAmount  decimal.Decimal
	CreditAmount decimal.Decimal
	NetAmount    decimal.Decimal
}
