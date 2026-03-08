package fa

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/modules/fa/internal/service"
)

type faAPI struct {
	assetRepo    repository.AssetRepository
	categoryRepo repository.CategoryRepository
	depRepo      repository.DepreciationRepository
	depEngine    *service.DepreciationEngine
	assetService *service.AssetService
}

func NewFAAPI(
	assetRepo repository.AssetRepository,
	categoryRepo repository.CategoryRepository,
	depRepo repository.DepreciationRepository,
	depEngine *service.DepreciationEngine,
	assetService *service.AssetService,
) API {
	return &faAPI{
		assetRepo:    assetRepo,
		categoryRepo: categoryRepo,
		depRepo:      depRepo,
		depEngine:    depEngine,
		assetService: assetService,
	}
}

func (a *faAPI) GetAssetByID(ctx context.Context, assetID common.ID) (*AssetResponse, error) {
	asset, err := a.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}
	return toAssetResponse(asset), nil
}

func (a *faAPI) GetAssetByCode(ctx context.Context, entityID common.ID, code string) (*AssetResponse, error) {
	asset, err := a.assetRepo.GetByCode(ctx, entityID, code)
	if err != nil {
		return nil, fmt.Errorf("asset not found: %w", err)
	}
	return toAssetResponse(asset), nil
}

func (a *faAPI) ListAssets(ctx context.Context, entityID common.ID, filter AssetFilterRequest) ([]AssetResponse, error) {
	domainFilter := domain.AssetFilter{
		EntityID: entityID,
		Limit:    filter.Limit,
		Offset:   filter.Offset,
	}

	if filter.CategoryID != nil {
		domainFilter.CategoryID = filter.CategoryID
	}

	if filter.Status != nil {
		status := domain.AssetStatus(*filter.Status)
		domainFilter.Status = &status
	}

	assets, err := a.assetRepo.List(ctx, domainFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}

	responses := make([]AssetResponse, len(assets))
	for i, asset := range assets {
		responses[i] = *toAssetResponse(&asset)
	}
	return responses, nil
}

func (a *faAPI) RunDepreciation(ctx context.Context, entityID common.ID, fiscalPeriodID common.ID, periodDate time.Time, currency money.Currency) (*DepreciationRunResponse, error) {
	run, err := a.depEngine.RunMonthlyDepreciation(ctx, entityID, fiscalPeriodID, periodDate, currency)
	if err != nil {
		return nil, fmt.Errorf("failed to run depreciation: %w", err)
	}
	return toDepreciationRunResponse(run), nil
}

func (a *faAPI) GetDepreciationRun(ctx context.Context, runID common.ID) (*DepreciationRunResponse, error) {
	run, err := a.depRepo.GetRunByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("depreciation run not found: %w", err)
	}
	return toDepreciationRunResponse(run), nil
}

func (a *faAPI) LinkAPInvoice(ctx context.Context, assetID, invoiceID common.ID) error {
	asset, err := a.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("asset not found: %w", err)
	}

	asset.APInvoiceID = &invoiceID

	if err := a.assetRepo.Update(ctx, asset); err != nil {
		return fmt.Errorf("failed to link invoice: %w", err)
	}

	return nil
}

func (a *faAPI) GetCategoryByID(ctx context.Context, categoryID common.ID) (*CategoryResponse, error) {
	category, err := a.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("category not found: %w", err)
	}
	return toCategoryResponse(category), nil
}

func (a *faAPI) ListCategories(ctx context.Context, entityID common.ID) ([]CategoryResponse, error) {
	categories, err := a.categoryRepo.List(ctx, domain.AssetCategoryFilter{
		EntityID: entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list categories: %w", err)
	}

	responses := make([]CategoryResponse, len(categories))
	for i, cat := range categories {
		responses[i] = *toCategoryResponse(&cat)
	}
	return responses, nil
}

func toAssetResponse(asset *domain.Asset) *AssetResponse {
	resp := &AssetResponse{
		ID:                      asset.ID,
		EntityID:                asset.EntityID,
		CategoryID:              asset.CategoryID,
		AssetCode:               asset.AssetCode,
		AssetName:               asset.AssetName,
		Description:             asset.Description,
		SerialNumber:            asset.SerialNumber,
		AcquisitionDate:         asset.AcquisitionDate,
		AcquisitionCost:         asset.AcquisitionCost,
		CurrencyCode:            asset.Currency.Code,
		DepreciationMethod:      string(asset.DepreciationMethod),
		UsefulLifeYears:         asset.UsefulLifeYears,
		SalvageValue:            asset.SalvageValue,
		AccumulatedDepreciation: asset.AccumulatedDepreciation,
		BookValue:               asset.BookValue,
		UnitsUsed:               asset.UnitsUsed,
		LocationCode:            asset.LocationCode,
		DepartmentCode:          asset.DepartmentCode,
		Status:                  string(asset.Status),
		CreatedAt:               asset.CreatedAt,
		UpdatedAt:               asset.UpdatedAt,
	}

	if asset.UsefulLifeUnits != nil {
		resp.UsefulLifeUnits = asset.UsefulLifeUnits
	}

	if asset.DepreciationStartDate != nil {
		resp.DepreciationStartDate = asset.DepreciationStartDate
	}

	if asset.LastDepreciationDate != nil {
		resp.LastDepreciationDate = asset.LastDepreciationDate
	}

	if asset.ActivatedAt != nil {
		resp.ActivatedAt = asset.ActivatedAt
	}

	if asset.DisposedAt != nil {
		resp.DisposedAt = asset.DisposedAt
	}

	if asset.DisposalType != nil {
		dt := string(*asset.DisposalType)
		resp.DisposalType = &dt
	}

	if !asset.DisposalProceeds.IsZero() {
		resp.DisposalProceeds = &asset.DisposalProceeds
	}

	return resp
}

func toCategoryResponse(cat *domain.AssetCategory) *CategoryResponse {
	resp := &CategoryResponse{
		ID:                     cat.ID,
		EntityID:               cat.EntityID,
		Code:                   cat.Code,
		Name:                   cat.Name,
		Description:            cat.Description,
		DepreciationMethod:     string(cat.DepreciationMethod),
		DefaultUsefulLifeYears: cat.DefaultUsefulLifeYears,
		DefaultSalvagePercent:  cat.DefaultSalvagePercent.InexactFloat64(),
		IsActive:               cat.IsActive,
		CreatedAt:              cat.CreatedAt,
		UpdatedAt:              cat.UpdatedAt,
	}

	if cat.AssetAccountID != nil {
		resp.AssetAccountID = cat.AssetAccountID
	}
	if cat.AccumDepreciationAccountID != nil {
		resp.AccumDepreciationAccountID = cat.AccumDepreciationAccountID
	}
	if cat.DepreciationExpenseAccountID != nil {
		resp.DepreciationExpenseAccountID = cat.DepreciationExpenseAccountID
	}
	if cat.GainLossAccountID != nil {
		resp.GainLossAccountID = cat.GainLossAccountID
	}

	return resp
}

func toDepreciationRunResponse(run *domain.DepreciationRun) *DepreciationRunResponse {
	resp := &DepreciationRunResponse{
		ID:                run.ID,
		EntityID:          run.EntityID,
		RunNumber:         run.RunNumber,
		FiscalPeriodID:    run.FiscalPeriodID,
		DepreciationDate:  run.DepreciationDate,
		AssetCount:        run.AssetCount,
		TotalDepreciation: run.TotalDepreciation,
		CurrencyCode:      run.Currency.Code,
		Status:            string(run.Status),
		CreatedAt:         run.CreatedAt,
	}

	if run.JournalEntryID != nil {
		resp.JournalEntryID = run.JournalEntryID
	}

	if run.PostedAt != nil {
		resp.PostedAt = run.PostedAt
	}

	return resp
}
