package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fx/internal/domain"
	"converge-finance.com/m/internal/modules/fx/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type TriangulationService struct {
	configRepo     repository.TriangulationConfigRepository
	pairConfigRepo repository.CurrencyPairConfigRepository
	logRepo        repository.TriangulationLogRepository
	rateService    money.ExchangeRateService
	auditLogger    *audit.Logger
	logger         *zap.Logger
}

func NewTriangulationService(
	configRepo repository.TriangulationConfigRepository,
	pairConfigRepo repository.CurrencyPairConfigRepository,
	logRepo repository.TriangulationLogRepository,
	rateService money.ExchangeRateService,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *TriangulationService {
	return &TriangulationService{
		configRepo:     configRepo,
		pairConfigRepo: pairConfigRepo,
		logRepo:        logRepo,
		rateService:    rateService,
		auditLogger:    auditLogger,
		logger:         logger,
	}
}

type ConvertRequest struct {
	EntityID      common.ID
	Amount        money.Money
	ToCurrency    money.Currency
	Date          time.Time
	RateType      money.RateType
	ReferenceType string
	ReferenceID   common.ID
	CreatedBy     common.ID
}

func (s *TriangulationService) Convert(ctx context.Context, req ConvertRequest) (*domain.TriangulationResult, error) {

	if req.Amount.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidAmount
	}

	if req.Amount.Currency.Equals(req.ToCurrency) {
		return &domain.TriangulationResult{
			FromCurrency:   req.Amount.Currency,
			ToCurrency:     req.ToCurrency,
			OriginalAmount: req.Amount,
			ResultAmount:   req.Amount,
			EffectiveRate:  decimal.NewFromInt(1),
			Legs:           []domain.TriangulationLeg{},
			Method:         domain.TriangulationMethodDirect,
			ConversionDate: req.Date,
			RateType:       req.RateType,
		}, nil
	}

	config, err := s.configRepo.GetByEntityID(ctx, req.EntityID)
	if err != nil && err != domain.ErrConfigNotFound {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	if config == nil {
		config = &domain.TriangulationConfig{
			FallbackCurrencies: []string{"USD", "EUR"},
			MaxLegs:            3,
			AllowInverseRates:  true,
		}
	}

	pairConfig, _ := s.pairConfigRepo.GetByPair(ctx, req.EntityID, req.Amount.Currency.Code, req.ToCurrency.Code)

	result, err := s.findAndExecuteConversion(ctx, req, config, pairConfig)
	if err != nil {
		return nil, err
	}

	if req.CreatedBy != "" {
		logEntry := domain.NewTriangulationLog(
			req.EntityID,
			result,
			req.ReferenceType,
			req.ReferenceID,
			req.CreatedBy,
		)
		if err := s.logRepo.Create(ctx, logEntry); err != nil {
			s.logger.Warn("failed to log conversion", zap.Error(err))
		}
	}

	return result, nil
}

func (s *TriangulationService) findAndExecuteConversion(
	ctx context.Context,
	req ConvertRequest,
	config *domain.TriangulationConfig,
	pairConfig *domain.CurrencyPairConfig,
) (*domain.TriangulationResult, error) {
	fromCurrency := req.Amount.Currency
	toCurrency := req.ToCurrency

	rate, err := s.rateService.GetRate(ctx, fromCurrency, toCurrency, req.Date, req.RateType)
	if err == nil {
		resultAmount := req.Amount.Convert(toCurrency, rate.Rate)

		if pairConfig != nil && pairConfig.SpreadMarkup.GreaterThan(decimal.Zero) {
			markup := decimal.NewFromInt(1).Add(pairConfig.SpreadMarkup)
			resultAmount = money.NewFromDecimal(resultAmount.Amount.Mul(markup), toCurrency)
		}

		return &domain.TriangulationResult{
			FromCurrency:   fromCurrency,
			ToCurrency:     toCurrency,
			OriginalAmount: req.Amount,
			ResultAmount:   resultAmount,
			EffectiveRate:  rate.Rate,
			Legs: []domain.TriangulationLeg{
				{
					FromCurrency: fromCurrency.Code,
					ToCurrency:   toCurrency.Code,
					Rate:         rate.Rate,
					RateType:     string(req.RateType),
					RateDate:     req.Date,
				},
			},
			Method:         domain.TriangulationMethodDirect,
			ConversionDate: req.Date,
			RateType:       req.RateType,
		}, nil
	}

	if pairConfig != nil && pairConfig.ViaCurrency != nil {
		result, err := s.convertVia(ctx, req, *pairConfig.ViaCurrency, pairConfig.SpreadMarkup)
		if err == nil {
			result.Method = domain.TriangulationMethodCustom
			return result, nil
		}
	}

	for _, fallbackCode := range config.FallbackCurrencies {
		if fallbackCode == fromCurrency.Code || fallbackCode == toCurrency.Code {
			continue
		}

		fallbackCurrency, err := money.GetCurrency(fallbackCode)
		if err != nil {
			continue
		}

		result, err := s.convertVia(ctx, req, fallbackCurrency, decimal.Zero)
		if err == nil {
			switch fallbackCode {
			case "USD":
				result.Method = domain.TriangulationMethodViaUSD
			case "EUR":
				result.Method = domain.TriangulationMethodViaEUR
			default:
				result.Method = domain.TriangulationMethodViaBase
			}
			return result, nil
		}
	}

	return nil, domain.ErrNoConversionPath
}

func (s *TriangulationService) convertVia(
	ctx context.Context,
	req ConvertRequest,
	viaCurrency money.Currency,
	spreadMarkup decimal.Decimal,
) (*domain.TriangulationResult, error) {
	fromCurrency := req.Amount.Currency
	toCurrency := req.ToCurrency

	rate1, err := s.rateService.GetRate(ctx, fromCurrency, viaCurrency, req.Date, req.RateType)
	if err != nil {
		return nil, err
	}

	rate2, err := s.rateService.GetRate(ctx, viaCurrency, toCurrency, req.Date, req.RateType)
	if err != nil {
		return nil, err
	}

	effectiveRate := rate1.Rate.Mul(rate2.Rate)

	if spreadMarkup.GreaterThan(decimal.Zero) {
		markup := decimal.NewFromInt(1).Add(spreadMarkup)
		effectiveRate = effectiveRate.Mul(markup)
	}

	resultAmount := money.NewFromDecimal(req.Amount.Amount.Mul(effectiveRate), toCurrency)

	return &domain.TriangulationResult{
		FromCurrency:   fromCurrency,
		ToCurrency:     toCurrency,
		OriginalAmount: req.Amount,
		ResultAmount:   resultAmount,
		EffectiveRate:  effectiveRate,
		Legs: []domain.TriangulationLeg{
			{
				FromCurrency: fromCurrency.Code,
				ToCurrency:   viaCurrency.Code,
				Rate:         rate1.Rate,
				RateType:     string(req.RateType),
				RateDate:     req.Date,
			},
			{
				FromCurrency: viaCurrency.Code,
				ToCurrency:   toCurrency.Code,
				Rate:         rate2.Rate,
				RateType:     string(req.RateType),
				RateDate:     req.Date,
			},
		},
		ConversionDate: req.Date,
		RateType:       req.RateType,
	}, nil
}

func (s *TriangulationService) FindConversionPath(
	ctx context.Context,
	entityID common.ID,
	from, to money.Currency,
	date time.Time,
	rateType money.RateType,
) (*domain.ConversionPath, error) {
	if from.Equals(to) {
		return &domain.ConversionPath{
			Currencies:    []string{from.Code, to.Code},
			Rates:         []decimal.Decimal{decimal.NewFromInt(1)},
			EffectiveRate: decimal.NewFromInt(1),
			LegsCount:     1,
		}, nil
	}

	rate, err := s.rateService.GetRate(ctx, from, to, date, rateType)
	if err == nil {
		return &domain.ConversionPath{
			Currencies:    []string{from.Code, to.Code},
			Rates:         []decimal.Decimal{rate.Rate},
			EffectiveRate: rate.Rate,
			LegsCount:     1,
		}, nil
	}

	config, _ := s.configRepo.GetByEntityID(ctx, entityID)
	fallbacks := []string{"USD", "EUR"}
	if config != nil {
		fallbacks = config.FallbackCurrencies
	}

	for _, viaCode := range fallbacks {
		if viaCode == from.Code || viaCode == to.Code {
			continue
		}

		viaCurrency, err := money.GetCurrency(viaCode)
		if err != nil {
			continue
		}

		rate1, err := s.rateService.GetRate(ctx, from, viaCurrency, date, rateType)
		if err != nil {
			continue
		}

		rate2, err := s.rateService.GetRate(ctx, viaCurrency, to, date, rateType)
		if err != nil {
			continue
		}

		return &domain.ConversionPath{
			Currencies:    []string{from.Code, viaCode, to.Code},
			Rates:         []decimal.Decimal{rate1.Rate, rate2.Rate},
			EffectiveRate: rate1.Rate.Mul(rate2.Rate),
			LegsCount:     2,
		}, nil
	}

	return nil, domain.ErrNoConversionPath
}

func (s *TriangulationService) GetConfig(ctx context.Context, entityID common.ID) (*domain.TriangulationConfig, error) {
	return s.configRepo.GetByEntityID(ctx, entityID)
}

func (s *TriangulationService) CreateOrUpdateConfig(ctx context.Context, config *domain.TriangulationConfig) error {
	existing, err := s.configRepo.GetByEntityID(ctx, config.EntityID)
	if err == domain.ErrConfigNotFound {
		return s.configRepo.Create(ctx, config)
	}
	if err != nil {
		return err
	}

	config.ID = existing.ID
	config.CreatedAt = existing.CreatedAt
	config.CreatedBy = existing.CreatedBy
	return s.configRepo.Update(ctx, config)
}

func (s *TriangulationService) GetCurrencyPairConfig(
	ctx context.Context,
	entityID common.ID,
	from, to string,
) (*domain.CurrencyPairConfig, error) {
	return s.pairConfigRepo.GetByPair(ctx, entityID, from, to)
}

func (s *TriangulationService) SetCurrencyPairConfig(ctx context.Context, config *domain.CurrencyPairConfig) error {
	existing, err := s.pairConfigRepo.GetByPair(ctx, config.EntityID, config.FromCurrency.Code, config.ToCurrency.Code)
	if err == domain.ErrPairConfigNotFound {
		return s.pairConfigRepo.Create(ctx, config)
	}
	if err != nil {
		return err
	}

	config.ID = existing.ID
	config.CreatedAt = existing.CreatedAt
	config.CreatedBy = existing.CreatedBy
	return s.pairConfigRepo.Update(ctx, config)
}

func (s *TriangulationService) ListCurrencyPairConfigs(ctx context.Context, entityID common.ID) ([]domain.CurrencyPairConfig, error) {
	return s.pairConfigRepo.List(ctx, entityID)
}

func (s *TriangulationService) GetConversionLog(ctx context.Context, id common.ID) (*domain.TriangulationLog, error) {
	return s.logRepo.GetByID(ctx, id)
}

func (s *TriangulationService) ListConversionLogs(
	ctx context.Context,
	filter repository.TriangulationLogFilter,
) ([]domain.TriangulationLog, int, error) {
	return s.logRepo.List(ctx, filter)
}
