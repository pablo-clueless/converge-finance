package money

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

type RateType string

const (
	RateTypeSpot    RateType = "spot"
	RateTypeAverage RateType = "average"
	RateTypeBudget  RateType = "budget"
	RateTypeClosing RateType = "closing"
)

type ExchangeRate struct {
	FromCurrency  Currency
	ToCurrency    Currency
	Rate          decimal.Decimal
	RateType      RateType
	EffectiveDate time.Time
}

func (er ExchangeRate) Convert(amount decimal.Decimal) decimal.Decimal {
	return amount.Mul(er.Rate)
}

func (er ExchangeRate) Invert() ExchangeRate {
	return ExchangeRate{
		FromCurrency:  er.ToCurrency,
		ToCurrency:    er.FromCurrency,
		Rate:          decimal.NewFromInt(1).Div(er.Rate),
		RateType:      er.RateType,
		EffectiveDate: er.EffectiveDate,
	}
}

type ExchangeRateService interface {
	GetRate(ctx context.Context, from, to Currency, date time.Time, rateType RateType) (ExchangeRate, error)

	Convert(ctx context.Context, amount Money, to Currency, date time.Time, rateType RateType) (Money, ExchangeRate, error)

	GetLatestRate(ctx context.Context, from, to Currency, rateType RateType) (ExchangeRate, error)
}

type ExchangeRateRepository interface {
	Save(ctx context.Context, rate ExchangeRate) error

	GetByDate(ctx context.Context, from, to Currency, date time.Time, rateType RateType) (ExchangeRate, error)

	GetLatest(ctx context.Context, from, to Currency, rateType RateType) (ExchangeRate, error)

	GetRange(ctx context.Context, from, to Currency, startDate, endDate time.Time, rateType RateType) ([]ExchangeRate, error)
}

type SimpleExchangeRateService struct {
	repo ExchangeRateRepository
}

func NewSimpleExchangeRateService(repo ExchangeRateRepository) *SimpleExchangeRateService {
	return &SimpleExchangeRateService{repo: repo}
}

func (s *SimpleExchangeRateService) GetRate(ctx context.Context, from, to Currency, date time.Time, rateType RateType) (ExchangeRate, error) {
	if from.Equals(to) {
		return ExchangeRate{
			FromCurrency:  from,
			ToCurrency:    to,
			Rate:          decimal.NewFromInt(1),
			RateType:      rateType,
			EffectiveDate: date,
		}, nil
	}

	rate, err := s.repo.GetByDate(ctx, from, to, date, rateType)
	if err == nil {
		return rate, nil
	}

	inverseRate, err := s.repo.GetByDate(ctx, to, from, date, rateType)
	if err == nil {
		return inverseRate.Invert(), nil
	}

	return ExchangeRate{}, fmt.Errorf("exchange rate not found: %s to %s on %s", from.Code, to.Code, date.Format("2006-01-02"))
}

func (s *SimpleExchangeRateService) Convert(ctx context.Context, amount Money, to Currency, date time.Time, rateType RateType) (Money, ExchangeRate, error) {
	rate, err := s.GetRate(ctx, amount.Currency, to, date, rateType)
	if err != nil {
		return Money{}, ExchangeRate{}, err
	}

	converted := amount.Convert(to, rate.Rate)
	return converted, rate, nil
}

func (s *SimpleExchangeRateService) GetLatestRate(ctx context.Context, from, to Currency, rateType RateType) (ExchangeRate, error) {
	if from.Equals(to) {
		return ExchangeRate{
			FromCurrency:  from,
			ToCurrency:    to,
			Rate:          decimal.NewFromInt(1),
			RateType:      rateType,
			EffectiveDate: time.Now(),
		}, nil
	}

	rate, err := s.repo.GetLatest(ctx, from, to, rateType)
	if err == nil {
		return rate, nil
	}

	inverseRate, err := s.repo.GetLatest(ctx, to, from, rateType)
	if err == nil {
		return inverseRate.Invert(), nil
	}

	return ExchangeRate{}, fmt.Errorf("exchange rate not found: %s to %s", from.Code, to.Code)
}

func Triangulate(rate1, rate2 ExchangeRate) (ExchangeRate, error) {
	if !rate1.ToCurrency.Equals(rate2.FromCurrency) {
		return ExchangeRate{}, fmt.Errorf("currencies do not match for triangulation")
	}

	return ExchangeRate{
		FromCurrency:  rate1.FromCurrency,
		ToCurrency:    rate2.ToCurrency,
		Rate:          rate1.Rate.Mul(rate2.Rate),
		RateType:      rate1.RateType,
		EffectiveDate: rate1.EffectiveDate,
	}, nil
}
