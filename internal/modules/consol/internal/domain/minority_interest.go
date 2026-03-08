package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type MinorityInterest struct {
	ID                  common.ID
	ConsolidationRunID  common.ID
	EntityID            common.ID
	OwnershipPercent    float64
	MinorityPercent     float64
	TotalEquity         money.Money
	MinorityShareEquity money.Money
	NetIncome           money.Money
	MinorityShareIncome money.Money
	DividendsToMinority money.Money
	OpeningNCI          money.Money
	ClosingNCI          money.Money
	CreatedAt           time.Time

	EntityCode string
	EntityName string
}

func NewMinorityInterest(
	consolidationRunID common.ID,
	entityID common.ID,
	ownershipPercent float64,
	reportingCurrency money.Currency,
) *MinorityInterest {
	minorityPercent := 100 - ownershipPercent

	return &MinorityInterest{
		ID:                  common.NewID(),
		ConsolidationRunID:  consolidationRunID,
		EntityID:            entityID,
		OwnershipPercent:    ownershipPercent,
		MinorityPercent:     minorityPercent,
		TotalEquity:         money.Zero(reportingCurrency),
		MinorityShareEquity: money.Zero(reportingCurrency),
		NetIncome:           money.Zero(reportingCurrency),
		MinorityShareIncome: money.Zero(reportingCurrency),
		DividendsToMinority: money.Zero(reportingCurrency),
		OpeningNCI:          money.Zero(reportingCurrency),
		ClosingNCI:          money.Zero(reportingCurrency),
		CreatedAt:           time.Now(),
	}
}

func (m *MinorityInterest) Calculate(totalEquity, netIncome, openingNCI money.Money) {
	m.TotalEquity = totalEquity
	m.NetIncome = netIncome
	m.OpeningNCI = openingNCI

	minorityRatio := decimal.NewFromFloat(m.MinorityPercent / 100)

	m.MinorityShareEquity = totalEquity.Multiply(minorityRatio)
	m.MinorityShareIncome = netIncome.Multiply(minorityRatio)

	m.ClosingNCI = openingNCI.MustAdd(m.MinorityShareIncome).MustSubtract(m.DividendsToMinority)
}

func (m *MinorityInterest) SetDividends(dividends money.Money) {
	m.DividendsToMinority = dividends
}

func (m *MinorityInterest) HasMinorityInterest() bool {
	return m.MinorityPercent > 0
}

type MinorityInterestFilter struct {
	ConsolidationRunID *common.ID
	EntityID           *common.ID
}
