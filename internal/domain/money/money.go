package money

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

type Money struct {
	Amount   decimal.Decimal
	Currency Currency
}

func New(amount float64, currencyCode string) Money {
	return Money{
		Amount:   decimal.NewFromFloat(amount),
		Currency: MustGetCurrency(currencyCode),
	}
}

func NewFromDecimal(amount decimal.Decimal, currency Currency) Money {
	return Money{
		Amount:   amount,
		Currency: currency,
	}
}

func NewFromString(amount string, currencyCode string) (Money, error) {
	d, err := decimal.NewFromString(amount)
	if err != nil {
		return Money{}, fmt.Errorf("invalid amount: %w", err)
	}
	currency, err := GetCurrency(currencyCode)
	if err != nil {
		return Money{}, err
	}
	return Money{Amount: d, Currency: currency}, nil
}

func NewFromInt(minorUnits int64, currencyCode string) Money {
	currency := MustGetCurrency(currencyCode)
	divisor := decimal.NewFromInt(1)
	for i := 0; i < currency.DecimalPlaces; i++ {
		divisor = divisor.Mul(decimal.NewFromInt(10))
	}
	return Money{
		Amount:   decimal.NewFromInt(minorUnits).Div(divisor),
		Currency: currency,
	}
}

func Zero(currency Currency) Money {
	return Money{
		Amount:   decimal.Zero,
		Currency: currency,
	}
}

func ZeroUSD() Money {
	return Zero(USD)
}

func (m Money) Add(other Money) (Money, error) {
	if err := m.validateSameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{
		Amount:   m.Amount.Add(other.Amount),
		Currency: m.Currency,
	}, nil
}

func (m Money) MustAdd(other Money) Money {
	result, err := m.Add(other)
	if err != nil {
		panic(err)
	}
	return result
}

func (m Money) Subtract(other Money) (Money, error) {
	if err := m.validateSameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{
		Amount:   m.Amount.Sub(other.Amount),
		Currency: m.Currency,
	}, nil
}

func (m Money) MustSubtract(other Money) Money {
	result, err := m.Subtract(other)
	if err != nil {
		panic(err)
	}
	return result
}

func (m Money) Multiply(multiplier decimal.Decimal) Money {
	return Money{
		Amount:   m.Amount.Mul(multiplier),
		Currency: m.Currency,
	}
}

func (m Money) MultiplyFloat(multiplier float64) Money {
	return m.Multiply(decimal.NewFromFloat(multiplier))
}

func (m Money) Divide(divisor decimal.Decimal) (Money, error) {
	if divisor.IsZero() {
		return Money{}, fmt.Errorf("cannot divide by zero")
	}
	return Money{
		Amount:   m.Amount.Div(divisor),
		Currency: m.Currency,
	}, nil
}

func (m Money) Negate() Money {
	return Money{
		Amount:   m.Amount.Neg(),
		Currency: m.Currency,
	}
}

func (m Money) Abs() Money {
	return Money{
		Amount:   m.Amount.Abs(),
		Currency: m.Currency,
	}
}

func (m Money) Round() Money {
	return Money{
		Amount:   m.Amount.Round(int32(m.Currency.DecimalPlaces)),
		Currency: m.Currency,
	}
}

func (m Money) IsZero() bool {
	return m.Amount.IsZero()
}

func (m Money) IsPositive() bool {
	return m.Amount.IsPositive()
}

func (m Money) IsNegative() bool {
	return m.Amount.IsNegative()
}

func (m Money) Equals(other Money) bool {
	return m.Currency.Equals(other.Currency) && m.Amount.Equal(other.Amount)
}

func (m Money) GreaterThan(other Money) bool {
	if !m.Currency.Equals(other.Currency) {
		return false
	}
	return m.Amount.GreaterThan(other.Amount)
}

func (m Money) LessThan(other Money) bool {
	if !m.Currency.Equals(other.Currency) {
		return false
	}
	return m.Amount.LessThan(other.Amount)
}

func (m Money) GreaterThanOrEqual(other Money) bool {
	if !m.Currency.Equals(other.Currency) {
		return false
	}
	return m.Amount.GreaterThanOrEqual(other.Amount)
}

func (m Money) LessThanOrEqual(other Money) bool {
	if !m.Currency.Equals(other.Currency) {
		return false
	}
	return m.Amount.LessThanOrEqual(other.Amount)
}

func (m Money) Convert(toCurrency Currency, rate decimal.Decimal) Money {
	return Money{
		Amount:   m.Amount.Mul(rate).Round(int32(toCurrency.DecimalPlaces)),
		Currency: toCurrency,
	}
}

func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.Amount.StringFixed(int32(m.Currency.DecimalPlaces)), m.Currency.Code)
}

func (m Money) FormattedString() string {
	return fmt.Sprintf("%s%s", m.Currency.Symbol, m.Amount.StringFixed(int32(m.Currency.DecimalPlaces)))
}

func (m Money) ToMinorUnits() int64 {
	multiplier := decimal.NewFromInt(1)
	for i := 0; i < m.Currency.DecimalPlaces; i++ {
		multiplier = multiplier.Mul(decimal.NewFromInt(10))
	}
	return m.Amount.Mul(multiplier).IntPart()
}

func (m Money) validateSameCurrency(other Money) error {
	if !m.Currency.Equals(other.Currency) {
		return fmt.Errorf("currency mismatch: %s vs %s", m.Currency.Code, other.Currency.Code)
	}
	return nil
}

func (m Money) Value() (driver.Value, error) {
	return m.Amount.String(), nil
}

type moneyJSON struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(moneyJSON{
		Amount:   m.Amount.String(),
		Currency: m.Currency.Code,
	})
}

func (m *Money) UnmarshalJSON(data []byte) error {
	var mj moneyJSON
	if err := json.Unmarshal(data, &mj); err != nil {
		return err
	}

	amount, err := decimal.NewFromString(mj.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}

	currency, err := GetCurrency(mj.Currency)
	if err != nil {
		currency = Currency{Code: mj.Currency, DecimalPlaces: 2}
	}

	m.Amount = amount
	m.Currency = currency
	return nil
}

func (m Money) Allocate(parts int) []Money {
	if parts <= 0 {
		return nil
	}

	total := m.ToMinorUnits()
	baseAmount := total / int64(parts)
	remainder := total % int64(parts)

	result := make([]Money, parts)
	for i := 0; i < parts; i++ {
		amount := baseAmount
		if int64(i) < remainder {
			amount++
		}
		result[i] = NewFromInt(amount, m.Currency.Code)
	}

	return result
}

func (m Money) AllocateByRatios(ratios []decimal.Decimal) []Money {
	if len(ratios) == 0 {
		return nil
	}

	total := decimal.Zero
	for _, r := range ratios {
		total = total.Add(r)
	}

	if total.IsZero() {
		return nil
	}

	result := make([]Money, len(ratios))
	allocated := decimal.Zero

	for i, ratio := range ratios {
		if i == len(ratios)-1 {
			result[i] = Money{
				Amount:   m.Amount.Sub(allocated),
				Currency: m.Currency,
			}
		} else {
			part := m.Amount.Mul(ratio).Div(total).Round(int32(m.Currency.DecimalPlaces))
			result[i] = Money{
				Amount:   part,
				Currency: m.Currency,
			}
			allocated = allocated.Add(part)
		}
	}

	return result
}
