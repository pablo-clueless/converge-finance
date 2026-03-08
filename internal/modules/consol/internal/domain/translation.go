package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type RateType string

const (
	RateTypeClosing    RateType = "closing"
	RateTypeAverage    RateType = "average"
	RateTypeHistorical RateType = "historical"
)

func (r RateType) IsValid() bool {
	switch r {
	case RateTypeClosing, RateTypeAverage, RateTypeHistorical:
		return true
	}
	return false
}

type ExchangeRate struct {
	ID             common.ID
	FromCurrency   money.Currency
	ToCurrency     money.Currency
	RateDate       time.Time
	ClosingRate    float64
	AverageRate    *float64
	HistoricalRate *float64
	Source         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewExchangeRate(
	fromCurrency money.Currency,
	toCurrency money.Currency,
	rateDate time.Time,
	closingRate float64,
) (*ExchangeRate, error) {
	if fromCurrency == toCurrency {
		return nil, fmt.Errorf("from and to currencies must be different")
	}
	if closingRate <= 0 {
		return nil, fmt.Errorf("closing rate must be positive")
	}

	return &ExchangeRate{
		ID:           common.NewID(),
		FromCurrency: fromCurrency,
		ToCurrency:   toCurrency,
		RateDate:     rateDate,
		ClosingRate:  closingRate,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (r *ExchangeRate) SetAverageRate(rate float64) error {
	if rate <= 0 {
		return fmt.Errorf("average rate must be positive")
	}
	r.AverageRate = &rate
	r.UpdatedAt = time.Now()
	return nil
}

func (r *ExchangeRate) SetHistoricalRate(rate float64) error {
	if rate <= 0 {
		return fmt.Errorf("historical rate must be positive")
	}
	r.HistoricalRate = &rate
	r.UpdatedAt = time.Now()
	return nil
}

func (r *ExchangeRate) GetRate(rateType RateType) float64 {
	switch rateType {
	case RateTypeAverage:
		if r.AverageRate != nil {
			return *r.AverageRate
		}
		return r.ClosingRate
	case RateTypeHistorical:
		if r.HistoricalRate != nil {
			return *r.HistoricalRate
		}
		return r.ClosingRate
	default:
		return r.ClosingRate
	}
}

func (r *ExchangeRate) Translate(amount money.Money, rateType RateType) (money.Money, error) {
	if !amount.Currency.Equals(r.FromCurrency) {
		return money.Money{}, fmt.Errorf("amount currency %s does not match rate from currency %s",
			amount.Currency.Code, r.FromCurrency.Code)
	}

	rate := r.GetRate(rateType)
	return amount.Convert(r.ToCurrency, decimal.NewFromFloat(rate)), nil
}

func (r *ExchangeRate) InverseRate() *ExchangeRate {
	inverse := &ExchangeRate{
		ID:           common.NewID(),
		FromCurrency: r.ToCurrency,
		ToCurrency:   r.FromCurrency,
		RateDate:     r.RateDate,
		ClosingRate:  1 / r.ClosingRate,
		Source:       r.Source,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if r.AverageRate != nil {
		avg := 1 / *r.AverageRate
		inverse.AverageRate = &avg
	}
	if r.HistoricalRate != nil {
		hist := 1 / *r.HistoricalRate
		inverse.HistoricalRate = &hist
	}

	return inverse
}

type ExchangeRateFilter struct {
	FromCurrency *money.Currency
	ToCurrency   *money.Currency
	DateFrom     *time.Time
	DateTo       *time.Time
	Limit        int
	Offset       int
}

type AdjustmentType string

const (
	AdjustmentTypeCTA                AdjustmentType = "cta"
	AdjustmentTypeRemeasurementGL    AdjustmentType = "remeasurement_gain_loss"
	AdjustmentTypeMinorityInterest   AdjustmentType = "minority_interest"
	AdjustmentTypeElimination        AdjustmentType = "elimination"
	AdjustmentTypeManual             AdjustmentType = "manual"
)

func (t AdjustmentType) IsValid() bool {
	switch t {
	case AdjustmentTypeCTA, AdjustmentTypeRemeasurementGL, AdjustmentTypeMinorityInterest,
		AdjustmentTypeElimination, AdjustmentTypeManual:
		return true
	}
	return false
}

type TranslationAdjustment struct {
	ID                 common.ID
	ConsolidationRunID common.ID
	EntityID           common.ID
	AdjustmentType     AdjustmentType
	AccountID          common.ID
	Description        string
	DebitAmount        money.Money
	CreditAmount       money.Money
	FunctionalCurrency money.Currency
	ReportingCurrency  money.Currency
	ExchangeRate       float64
	CreatedAt          time.Time

	AccountCode string
	AccountName string
}

func NewTranslationAdjustment(
	consolidationRunID common.ID,
	entityID common.ID,
	adjustmentType AdjustmentType,
	accountID common.ID,
	description string,
	amount money.Money,
	isDebit bool,
	functionalCurrency money.Currency,
	reportingCurrency money.Currency,
	exchangeRate float64,
) (*TranslationAdjustment, error) {
	if !adjustmentType.IsValid() {
		return nil, fmt.Errorf("invalid adjustment type")
	}

	adj := &TranslationAdjustment{
		ID:                 common.NewID(),
		ConsolidationRunID: consolidationRunID,
		EntityID:           entityID,
		AdjustmentType:     adjustmentType,
		AccountID:          accountID,
		Description:        description,
		FunctionalCurrency: functionalCurrency,
		ReportingCurrency:  reportingCurrency,
		ExchangeRate:       exchangeRate,
		CreatedAt:          time.Now(),
	}

	if isDebit {
		adj.DebitAmount = amount
		adj.CreditAmount = money.Zero(amount.Currency)
	} else {
		adj.DebitAmount = money.Zero(amount.Currency)
		adj.CreditAmount = amount
	}

	return adj, nil
}

func (a *TranslationAdjustment) Amount() money.Money {
	if !a.DebitAmount.IsZero() {
		return a.DebitAmount
	}
	return a.CreditAmount
}

func (a *TranslationAdjustment) IsDebit() bool {
	return !a.DebitAmount.IsZero()
}
