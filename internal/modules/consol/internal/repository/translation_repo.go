package repository

import (
	"context"
	"database/sql"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
)

type ExchangeRateRepository interface {
	WithTx(tx *sql.Tx) ExchangeRateRepository

	Create(ctx context.Context, rate *domain.ExchangeRate) error
	Update(ctx context.Context, rate *domain.ExchangeRate) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.ExchangeRate, error)
	GetRate(ctx context.Context, fromCurrency, toCurrency money.Currency, date time.Time) (*domain.ExchangeRate, error)
	GetLatestRate(ctx context.Context, fromCurrency, toCurrency money.Currency) (*domain.ExchangeRate, error)
	List(ctx context.Context, filter domain.ExchangeRateFilter) ([]domain.ExchangeRate, error)

	GetClosingRate(ctx context.Context, fromCurrency, toCurrency money.Currency, date time.Time) (float64, error)
	GetAverageRate(ctx context.Context, fromCurrency, toCurrency money.Currency, date time.Time) (float64, error)
	GetHistoricalRate(ctx context.Context, fromCurrency, toCurrency money.Currency, date time.Time) (float64, error)
}

type TranslationAdjustmentRepository interface {
	WithTx(tx *sql.Tx) TranslationAdjustmentRepository

	Create(ctx context.Context, adjustment *domain.TranslationAdjustment) error
	CreateBatch(ctx context.Context, adjustments []domain.TranslationAdjustment) error
	Delete(ctx context.Context, id common.ID) error
	DeleteByRun(ctx context.Context, runID common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.TranslationAdjustment, error)
	GetByRun(ctx context.Context, runID common.ID) ([]domain.TranslationAdjustment, error)
	GetByRunAndEntity(ctx context.Context, runID, entityID common.ID) ([]domain.TranslationAdjustment, error)
	GetByRunAndType(ctx context.Context, runID common.ID, adjustmentType domain.AdjustmentType) ([]domain.TranslationAdjustment, error)

	SumByType(ctx context.Context, runID common.ID, adjustmentType domain.AdjustmentType) (money.Money, error)
}
