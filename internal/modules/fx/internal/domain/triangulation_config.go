package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type TriangulationMethod string

const (
	TriangulationMethodDirect  TriangulationMethod = "direct"
	TriangulationMethodViaBase TriangulationMethod = "via_base"
	TriangulationMethodViaUSD  TriangulationMethod = "via_usd"
	TriangulationMethodViaEUR  TriangulationMethod = "via_eur"
	TriangulationMethodCustom  TriangulationMethod = "custom"
)

type TriangulationConfig struct {
	ID                 common.ID
	EntityID           common.ID
	BaseCurrency       money.Currency
	FallbackCurrencies []string
	MaxLegs            int
	AllowInverseRates  bool
	RateTolerance      decimal.Decimal
	IsActive           bool
	CreatedBy          common.ID
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func NewTriangulationConfig(
	entityID common.ID,
	baseCurrency money.Currency,
	createdBy common.ID,
) *TriangulationConfig {
	now := time.Now()
	return &TriangulationConfig{
		ID:                 common.NewID(),
		EntityID:           entityID,
		BaseCurrency:       baseCurrency,
		FallbackCurrencies: []string{"USD", "EUR"},
		MaxLegs:            3,
		AllowInverseRates:  true,
		RateTolerance:      decimal.NewFromFloat(0.0001),
		IsActive:           true,
		CreatedBy:          createdBy,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func (c *TriangulationConfig) Update(
	baseCurrency *money.Currency,
	fallbackCurrencies []string,
	maxLegs *int,
	allowInverseRates *bool,
	rateTolerance *decimal.Decimal,
) {
	if baseCurrency != nil {
		c.BaseCurrency = *baseCurrency
	}
	if fallbackCurrencies != nil {
		c.FallbackCurrencies = fallbackCurrencies
	}
	if maxLegs != nil && *maxLegs >= 2 && *maxLegs <= 5 {
		c.MaxLegs = *maxLegs
	}
	if allowInverseRates != nil {
		c.AllowInverseRates = *allowInverseRates
	}
	if rateTolerance != nil {
		c.RateTolerance = *rateTolerance
	}
	c.UpdatedAt = time.Now()
}

func (c *TriangulationConfig) Activate() {
	c.IsActive = true
	c.UpdatedAt = time.Now()
}

func (c *TriangulationConfig) Deactivate() {
	c.IsActive = false
	c.UpdatedAt = time.Now()
}

type CurrencyPairConfig struct {
	ID              common.ID
	EntityID        common.ID
	FromCurrency    money.Currency
	ToCurrency      money.Currency
	PreferredMethod TriangulationMethod
	ViaCurrency     *money.Currency
	SpreadMarkup    decimal.Decimal
	Priority        int
	IsActive        bool
	CreatedBy       common.ID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewCurrencyPairConfig(
	entityID common.ID,
	fromCurrency, toCurrency money.Currency,
	method TriangulationMethod,
	createdBy common.ID,
) (*CurrencyPairConfig, error) {
	if fromCurrency.Equals(toCurrency) {
		return nil, ErrSameCurrency
	}

	now := time.Now()
	return &CurrencyPairConfig{
		ID:              common.NewID(),
		EntityID:        entityID,
		FromCurrency:    fromCurrency,
		ToCurrency:      toCurrency,
		PreferredMethod: method,
		SpreadMarkup:    decimal.Zero,
		Priority:        0,
		IsActive:        true,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (c *CurrencyPairConfig) SetViaCurrency(via money.Currency) error {
	if via.Equals(c.FromCurrency) || via.Equals(c.ToCurrency) {
		return ErrInvalidViaCurrency
	}
	c.ViaCurrency = &via
	c.PreferredMethod = TriangulationMethodCustom
	c.UpdatedAt = time.Now()
	return nil
}

func (c *CurrencyPairConfig) SetSpreadMarkup(markup decimal.Decimal) error {
	if markup.LessThan(decimal.Zero) {
		return ErrInvalidSpreadMarkup
	}
	c.SpreadMarkup = markup
	c.UpdatedAt = time.Now()
	return nil
}
