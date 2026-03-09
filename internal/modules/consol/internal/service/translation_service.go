package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
	"converge-finance.com/m/internal/modules/consol/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
)

type TranslationService struct {
	rateRepo    repository.ExchangeRateRepository
	auditLogger *audit.Logger
}

func NewTranslationService(
	rateRepo repository.ExchangeRateRepository,
	auditLogger *audit.Logger,
) *TranslationService {
	return &TranslationService{
		rateRepo:    rateRepo,
		auditLogger: auditLogger,
	}
}

func (s *TranslationService) CreateExchangeRate(
	ctx context.Context,
	fromCurrency money.Currency,
	toCurrency money.Currency,
	rateDate time.Time,
	closingRate float64,
	averageRate *float64,
	historicalRate *float64,
	source string,
) (*domain.ExchangeRate, error) {
	rate, err := domain.NewExchangeRate(fromCurrency, toCurrency, rateDate, closingRate)
	if err != nil {
		return nil, fmt.Errorf("invalid exchange rate: %w", err)
	}

	if averageRate != nil {
		if err := rate.SetAverageRate(*averageRate); err != nil {
			return nil, fmt.Errorf("invalid average rate: %w", err)
		}
	}

	if historicalRate != nil {
		if err := rate.SetHistoricalRate(*historicalRate); err != nil {
			return nil, fmt.Errorf("invalid historical rate: %w", err)
		}
	}

	rate.Source = source

	if err := s.rateRepo.Create(ctx, rate); err != nil {
		return nil, fmt.Errorf("failed to save exchange rate: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "consol.exchange_rate", rate.ID, "created", map[string]any{
			"from_currency": fromCurrency.Code,
			"to_currency":   toCurrency.Code,
			"rate_date":     rateDate,
			"closing_rate":  closingRate,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to log audit event: %w", err)
		}
	}

	return rate, nil
}

func (s *TranslationService) UpdateExchangeRate(
	ctx context.Context,
	rateID common.ID,
	closingRate *float64,
	averageRate *float64,
	historicalRate *float64,
) (*domain.ExchangeRate, error) {
	rate, err := s.rateRepo.GetByID(ctx, rateID)
	if err != nil {
		return nil, fmt.Errorf("exchange rate not found: %w", err)
	}

	if closingRate != nil {
		if *closingRate <= 0 {
			return nil, fmt.Errorf("closing rate must be positive")
		}
		rate.ClosingRate = *closingRate
	}

	if averageRate != nil {
		if err := rate.SetAverageRate(*averageRate); err != nil {
			return nil, err
		}
	}

	if historicalRate != nil {
		if err := rate.SetHistoricalRate(*historicalRate); err != nil {
			return nil, err
		}
	}

	rate.UpdatedAt = time.Now()

	if err := s.rateRepo.Update(ctx, rate); err != nil {
		return nil, fmt.Errorf("failed to update exchange rate: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "consol.exchange_rate", rate.ID, "updated", map[string]any{
			"closing_rate":    rate.ClosingRate,
			"average_rate":    rate.AverageRate,
			"historical_rate": rate.HistoricalRate,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to log audit event: %w", err)
		}
	}

	return rate, nil
}

func (s *TranslationService) GetExchangeRate(
	ctx context.Context,
	fromCurrency money.Currency,
	toCurrency money.Currency,
	date time.Time,
) (*domain.ExchangeRate, error) {
	if fromCurrency.Equals(toCurrency) {
		return &domain.ExchangeRate{
			ID:           common.NewID(),
			FromCurrency: fromCurrency,
			ToCurrency:   toCurrency,
			RateDate:     date,
			ClosingRate:  1.0,
		}, nil
	}

	rate, err := s.rateRepo.GetRate(ctx, fromCurrency, toCurrency, date)
	if err != nil {
		rate, err = s.rateRepo.GetLatestRate(ctx, fromCurrency, toCurrency)
		if err != nil {
			return nil, fmt.Errorf("no exchange rate found for %s/%s", fromCurrency.Code, toCurrency.Code)
		}
	}

	return rate, nil
}

func (s *TranslationService) ListExchangeRates(
	ctx context.Context,
	filter domain.ExchangeRateFilter,
) ([]domain.ExchangeRate, error) {
	return s.rateRepo.List(ctx, filter)
}

func (s *TranslationService) TranslateAmount(
	ctx context.Context,
	amount money.Money,
	toCurrency money.Currency,
	date time.Time,
	rateType domain.RateType,
) (money.Money, float64, error) {
	if amount.Currency.Equals(toCurrency) {
		return amount, 1.0, nil
	}

	exchangeRate, err := s.GetExchangeRate(ctx, amount.Currency, toCurrency, date)
	if err != nil {
		return money.Money{}, 0, err
	}

	translated, err := exchangeRate.Translate(amount, rateType)
	if err != nil {
		return money.Money{}, 0, err
	}

	return translated, exchangeRate.GetRate(rateType), nil
}

func (s *TranslationService) BulkCreateRates(
	ctx context.Context,
	rates []CreateRateRequest,
) (int, error) {
	created := 0

	for _, req := range rates {
		_, err := s.CreateExchangeRate(
			ctx,
			req.FromCurrency,
			req.ToCurrency,
			req.RateDate,
			req.ClosingRate,
			req.AverageRate,
			req.HistoricalRate,
			req.Source,
		)
		if err != nil {
			continue
		}
		created++
	}

	return created, nil
}

type CreateRateRequest struct {
	FromCurrency   money.Currency
	ToCurrency     money.Currency
	RateDate       time.Time
	ClosingRate    float64
	AverageRate    *float64
	HistoricalRate *float64
	Source         string
}

func (s *TranslationService) GetRateHistory(
	ctx context.Context,
	fromCurrency money.Currency,
	toCurrency money.Currency,
	startDate time.Time,
	endDate time.Time,
) ([]domain.ExchangeRate, error) {
	filter := domain.ExchangeRateFilter{
		FromCurrency: &fromCurrency,
		ToCurrency:   &toCurrency,
		DateFrom:     &startDate,
		DateTo:       &endDate,
	}

	return s.rateRepo.List(ctx, filter)
}

func (s *TranslationService) DeleteExchangeRate(ctx context.Context, rateID common.ID) error {
	if err := s.rateRepo.Delete(ctx, rateID); err != nil {
		return fmt.Errorf("failed to delete exchange rate: %w", err)
	}

	if s.auditLogger != nil {
		err := s.auditLogger.LogAction(ctx, "consol.exchange_rate", rateID, "deleted", nil)
		if err != nil {
			return fmt.Errorf("failed to log audit event: %w", err)
		}
	}

	return nil
}
