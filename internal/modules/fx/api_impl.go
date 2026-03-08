package fx

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fx/internal/domain"
	"converge-finance.com/m/internal/modules/fx/internal/repository"
	"converge-finance.com/m/internal/modules/fx/internal/service"
)

type fxAPI struct {
	triangulationService *service.TriangulationService
	revaluationService   *service.RevaluationService
}

func NewFXAPI(triangulationService *service.TriangulationService, revaluationService *service.RevaluationService) API {
	return &fxAPI{
		triangulationService: triangulationService,
		revaluationService:   revaluationService,
	}
}

func (a *fxAPI) Convert(ctx context.Context, req ConvertRequest) (*ConvertResponse, error) {
	result, err := a.triangulationService.Convert(ctx, service.ConvertRequest{
		EntityID:      req.EntityID,
		Amount:        req.Amount,
		ToCurrency:    req.ToCurrency,
		Date:          req.Date,
		RateType:      req.RateType,
		ReferenceType: req.ReferenceType,
		ReferenceID:   req.ReferenceID,
		CreatedBy:     req.CreatedBy,
	})
	if err != nil {
		return nil, err
	}

	legs := make([]ConversionLeg, len(result.Legs))
	for i, leg := range result.Legs {
		legs[i] = ConversionLeg{
			FromCurrency: leg.FromCurrency,
			ToCurrency:   leg.ToCurrency,
			Rate:         leg.Rate,
			RateType:     leg.RateType,
			RateDate:     leg.RateDate,
		}
	}

	return &ConvertResponse{
		FromCurrency:   result.FromCurrency,
		ToCurrency:     result.ToCurrency,
		OriginalAmount: result.OriginalAmount,
		ResultAmount:   result.ResultAmount,
		EffectiveRate:  result.EffectiveRate,
		Legs:           legs,
		LegsCount:      len(legs),
		Method:         string(result.Method),
		ConversionDate: result.ConversionDate,
		RateType:       result.RateType,
	}, nil
}

func (a *fxAPI) FindConversionPath(ctx context.Context, entityID common.ID, from, to money.Currency, date time.Time, rateType money.RateType) (*ConversionPathResponse, error) {
	path, err := a.triangulationService.FindConversionPath(ctx, entityID, from, to, date, rateType)
	if err != nil {
		return nil, err
	}

	return &ConversionPathResponse{
		Currencies:    path.Currencies,
		Rates:         path.Rates,
		EffectiveRate: path.EffectiveRate,
		LegsCount:     path.LegsCount,
	}, nil
}

func (a *fxAPI) GetConfig(ctx context.Context, entityID common.ID) (*TriangulationConfigResponse, error) {
	config, err := a.triangulationService.GetConfig(ctx, entityID)
	if err != nil {
		return nil, err
	}

	return &TriangulationConfigResponse{
		ID:                 config.ID,
		EntityID:           config.EntityID,
		BaseCurrency:       config.BaseCurrency.Code,
		FallbackCurrencies: config.FallbackCurrencies,
		MaxLegs:            config.MaxLegs,
		AllowInverseRates:  config.AllowInverseRates,
		RateTolerance:      config.RateTolerance,
		IsActive:           config.IsActive,
	}, nil
}

func (a *fxAPI) GetCurrencyPairConfig(ctx context.Context, entityID common.ID, from, to string) (*CurrencyPairConfigResponse, error) {
	config, err := a.triangulationService.GetCurrencyPairConfig(ctx, entityID, from, to)
	if err != nil {
		return nil, err
	}

	resp := &CurrencyPairConfigResponse{
		ID:              config.ID,
		EntityID:        config.EntityID,
		FromCurrency:    config.FromCurrency.Code,
		ToCurrency:      config.ToCurrency.Code,
		PreferredMethod: string(config.PreferredMethod),
		SpreadMarkup:    config.SpreadMarkup,
		Priority:        config.Priority,
		IsActive:        config.IsActive,
	}

	if config.ViaCurrency != nil {
		via := config.ViaCurrency.Code
		resp.ViaCurrency = &via
	}

	return resp, nil
}

func (a *fxAPI) GetRevaluationRun(ctx context.Context, id common.ID) (*RevaluationRunResponse, error) {
	run, err := a.revaluationService.GetRevaluationRun(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapRevaluationRunToResponse(run), nil
}

func (a *fxAPI) ListRevaluationRuns(ctx context.Context, req ListRevaluationRunsRequest) (*ListRevaluationRunsResponse, error) {
	filter := repository.RevaluationRunFilter{
		EntityID:       req.EntityID,
		FiscalPeriodID: req.FiscalPeriodID,
		DateFrom:       req.DateFrom,
		DateTo:         req.DateTo,
		Limit:          req.PageSize,
		Offset:         (req.Page - 1) * req.PageSize,
	}

	if req.Status != "" {
		status := domain.RevaluationStatus(req.Status)
		filter.Status = &status
	}

	runs, total, err := a.revaluationService.ListRevaluationRuns(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]RevaluationRunResponse, len(runs))
	for i, run := range runs {
		responses[i] = *a.mapRevaluationRunToResponse(&run)
	}

	return &ListRevaluationRunsResponse{
		Runs:  responses,
		Total: total,
		Page:  req.Page,
	}, nil
}

func (a *fxAPI) ApproveRevaluation(ctx context.Context, runID, approverID common.ID) (*RevaluationRunResponse, error) {
	run, err := a.revaluationService.ApproveRevaluation(ctx, runID, approverID)
	if err != nil {
		return nil, err
	}

	return a.mapRevaluationRunToResponse(run), nil
}

func (a *fxAPI) mapRevaluationRunToResponse(run *domain.RevaluationRun) *RevaluationRunResponse {
	resp := &RevaluationRunResponse{
		ID:                  run.ID,
		EntityID:            run.EntityID,
		RunNumber:           run.RunNumber,
		FiscalPeriodID:      run.FiscalPeriodID,
		RevaluationDate:     run.RevaluationDate,
		RateDate:            run.RateDate,
		FunctionalCurrency:  run.FunctionalCurrency.Code,
		Status:              string(run.Status),
		TotalUnrealizedGain: run.TotalUnrealizedGain,
		TotalUnrealizedLoss: run.TotalUnrealizedLoss,
		NetRevaluation:      run.NetRevaluation,
		AccountsProcessed:   run.AccountsProcessed,
		JournalEntryID:      run.JournalEntryID,
		CreatedBy:           run.CreatedBy,
		ApprovedBy:          run.ApprovedBy,
		PostedBy:            run.PostedBy,
		CreatedAt:           run.CreatedAt,
		ApprovedAt:          run.ApprovedAt,
		PostedAt:            run.PostedAt,
		Details:             make([]RevaluationDetailResponse, len(run.Details)),
	}

	for i, detail := range run.Details {
		resp.Details[i] = RevaluationDetailResponse{
			ID:                       detail.ID,
			AccountID:                detail.AccountID,
			AccountCode:              detail.AccountCode,
			AccountName:              detail.AccountName,
			OriginalCurrency:         detail.OriginalCurrency.Code,
			OriginalBalance:          detail.OriginalBalance,
			OriginalRate:             detail.OriginalRate,
			OriginalFunctionalAmount: detail.OriginalFunctionalAmount,
			NewRate:                  detail.NewRate,
			NewFunctionalAmount:      detail.NewFunctionalAmount,
			RevaluationAmount:        detail.RevaluationAmount,
			IsGain:                   detail.IsGain(),
			GainLossAccountID:        detail.GainLossAccountID,
		}
	}

	return resp
}
