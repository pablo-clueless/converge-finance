package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
)

type DepreciationEngine struct {
	db           *database.PostgresDB
	assetRepo    repository.AssetRepository
	categoryRepo repository.CategoryRepository
	depRepo      repository.DepreciationRepository
	glAPI        gl.API
	auditLogger  *audit.Logger
	calculator   *domain.DepreciationCalculator
}

func NewDepreciationEngine(
	db *database.PostgresDB,
	assetRepo repository.AssetRepository,
	categoryRepo repository.CategoryRepository,
	depRepo repository.DepreciationRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *DepreciationEngine {
	return &DepreciationEngine{
		db:           db,
		assetRepo:    assetRepo,
		categoryRepo: categoryRepo,
		depRepo:      depRepo,
		glAPI:        glAPI,
		auditLogger:  auditLogger,
		calculator:   domain.NewDepreciationCalculator(),
	}
}

func (e *DepreciationEngine) PreviewDepreciation(
	ctx context.Context,
	entityID common.ID,
	periodEndDate time.Time,
) ([]domain.DepreciationPreview, error) {
	assets, err := e.assetRepo.GetAssetsDueForDepreciation(ctx, entityID, periodEndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get depreciable assets: %w", err)
	}

	var previews []domain.DepreciationPreview
	for _, asset := range assets {
		category, err := e.categoryRepo.GetByID(ctx, asset.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to get category for asset %s: %w", asset.AssetCode, err)
		}
		asset.Category = category

		amount, basis, err := e.calculator.CalculateMonthlyDepreciation(&asset, periodEndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate depreciation for asset %s: %w", asset.AssetCode, err)
		}

		if amount.IsZero() {
			continue
		}

		previews = append(previews, domain.DepreciationPreview{
			Asset:              &asset,
			OpeningBookValue:   asset.BookValue,
			DepreciationAmount: amount,
			ClosingBookValue:   asset.BookValue.MustSubtract(amount),
			Method:             asset.DepreciationMethod,
			CalculationBasis:   basis,
		})
	}

	return previews, nil
}

func (e *DepreciationEngine) RunMonthlyDepreciation(
	ctx context.Context,
	entityID common.ID,
	fiscalPeriodID common.ID,
	periodEndDate time.Time,
	currency money.Currency,
) (*domain.DepreciationRun, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	existingRun, err := e.depRepo.GetRunByPeriod(ctx, entityID, fiscalPeriodID)
	if err == nil && existingRun != nil {
		if existingRun.Status == domain.DepreciationRunStatusPosted {
			return nil, fmt.Errorf("depreciation has already been posted for this period")
		}
		if existingRun.Status == domain.DepreciationRunStatusCalculated {
			return existingRun, nil
		}
	}

	runNumber, err := e.depRepo.GetNextRunNumber(ctx, entityID, "DEP")
	if err != nil {
		return nil, fmt.Errorf("failed to generate run number: %w", err)
	}

	run, err := domain.NewDepreciationRun(
		entityID,
		runNumber,
		fiscalPeriodID,
		periodEndDate,
		currency,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create depreciation run: %w", err)
	}

	assets, err := e.assetRepo.GetAssetsDueForDepreciation(ctx, entityID, periodEndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get depreciable assets: %w", err)
	}

	for _, asset := range assets {
		category, err := e.categoryRepo.GetByID(ctx, asset.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to get category for asset %s: %w", asset.AssetCode, err)
		}
		asset.Category = category

		amount, basis, err := e.calculator.CalculateMonthlyDepreciation(&asset, periodEndDate)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate depreciation for asset %s: %w", asset.AssetCode, err)
		}

		if amount.IsZero() {
			continue
		}

		monthsElapsed := asset.GetMonthsInService(periodEndDate)
		entry, err := domain.NewDepreciationEntry(run.ID, &asset, amount, monthsElapsed, basis)
		if err != nil {
			return nil, fmt.Errorf("failed to create entry for asset %s: %w", asset.AssetCode, err)
		}

		if err := run.AddEntry(*entry); err != nil {
			return nil, fmt.Errorf("failed to add entry for asset %s: %w", asset.AssetCode, err)
		}
	}

	if len(run.Entries) == 0 {
		return nil, fmt.Errorf("no assets require depreciation for this period")
	}

	if err := run.Calculate(); err != nil {
		return nil, fmt.Errorf("failed to finalize run: %w", err)
	}

	if err := e.depRepo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save depreciation run: %w", err)
	}

	if err := e.depRepo.CreateEntries(ctx, run.Entries); err != nil {
		return nil, fmt.Errorf("failed to save depreciation entries: %w", err)
	}

	if e.auditLogger != nil {
		_ = e.auditLogger.LogAction(ctx, "fa.depreciation_run", run.ID, "calculated", map[string]any{
			"run_number":         run.RunNumber,
			"asset_count":        run.AssetCount,
			"total_depreciation": run.TotalDepreciation.String(),
			"period_end_date":    periodEndDate.Format("2006-01-02"),
		})
	}

	return run, nil
}

func (e *DepreciationEngine) PostDepreciationRun(ctx context.Context, runID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	return e.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		run, err := e.depRepo.WithTx(tx).GetRunByIDForUpdate(ctx, tx, runID)
		if err != nil {
			return fmt.Errorf("failed to get depreciation run: %w", err)
		}

		if !run.Status.CanPost() {
			return fmt.Errorf("cannot post run with status: %s", run.Status)
		}

		entries, err := e.depRepo.WithTx(tx).GetEntriesByRun(ctx, runID)
		if err != nil {
			return fmt.Errorf("failed to get depreciation entries: %w", err)
		}
		run.Entries = entries

		journalEntry, err := e.createGLEntry(ctx, run)
		if err != nil {
			return fmt.Errorf("failed to create GL entry: %w", err)
		}

		if err := e.glAPI.PostJournalEntry(ctx, journalEntry.ID); err != nil {
			return fmt.Errorf("failed to post GL entry: %w", err)
		}

		if err := run.Post(journalEntry.ID, common.ID(userID)); err != nil {
			return fmt.Errorf("failed to mark run as posted: %w", err)
		}

		if err := e.depRepo.WithTx(tx).UpdateRun(ctx, run); err != nil {
			return fmt.Errorf("failed to update depreciation run: %w", err)
		}

		for _, entry := range entries {
			asset, err := e.assetRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, entry.AssetID)
			if err != nil {
				return fmt.Errorf("failed to get asset %s: %w", entry.AssetID, err)
			}

			if err := asset.RecordDepreciation(entry.DepreciationAmount, run.DepreciationDate); err != nil {
				return fmt.Errorf("failed to record depreciation for asset %s: %w", asset.AssetCode, err)
			}

			if err := e.assetRepo.WithTx(tx).UpdateDepreciation(ctx, asset); err != nil {
				return fmt.Errorf("failed to update asset %s: %w", asset.AssetCode, err)
			}
		}

		if e.auditLogger != nil {
			_ = e.auditLogger.LogAction(ctx, "fa.depreciation_run", run.ID, "posted", map[string]any{
				"run_number":       run.RunNumber,
				"journal_entry_id": journalEntry.ID,
				"asset_count":      run.AssetCount,
				"total_amount":     run.TotalDepreciation.String(),
			})
		}

		return nil
	})
}

func (e *DepreciationEngine) createGLEntry(ctx context.Context, run *domain.DepreciationRun) (*gl.JournalEntryResponse, error) {
	expenseByAccount := make(map[common.ID]money.Money)
	accumByAccount := make(map[common.ID]money.Money)

	for _, entry := range run.Entries {
		asset, err := e.assetRepo.GetByID(ctx, entry.AssetID)
		if err != nil {
			return nil, fmt.Errorf("failed to get asset: %w", err)
		}

		category, err := e.categoryRepo.GetByID(ctx, asset.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("failed to get category: %w", err)
		}

		expenseAccountID := asset.GetEffectiveDepExpenseAccountID(category)
		accumAccountID := asset.GetEffectiveAccumDepAccountID(category)

		if expenseAccountID == nil {
			return nil, fmt.Errorf("no depreciation expense account configured for asset %s", asset.AssetCode)
		}
		if accumAccountID == nil {
			return nil, fmt.Errorf("no accumulated depreciation account configured for asset %s", asset.AssetCode)
		}

		if existing, ok := expenseByAccount[*expenseAccountID]; ok {
			expenseByAccount[*expenseAccountID] = existing.MustAdd(entry.DepreciationAmount)
		} else {
			expenseByAccount[*expenseAccountID] = entry.DepreciationAmount
		}

		if existing, ok := accumByAccount[*accumAccountID]; ok {
			accumByAccount[*accumAccountID] = existing.MustAdd(entry.DepreciationAmount)
		} else {
			accumByAccount[*accumAccountID] = entry.DepreciationAmount
		}
	}

	var lines []gl.JournalLineRequest

	for accountID, amount := range expenseByAccount {
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   accountID,
			Description: fmt.Sprintf("Depreciation expense - %s", run.RunNumber),
			Debit:       amount,
			Credit:      money.Zero(run.Currency),
		})
	}

	for accountID, amount := range accumByAccount {
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   accountID,
			Description: fmt.Sprintf("Accumulated depreciation - %s", run.RunNumber),
			Debit:       money.Zero(run.Currency),
			Credit:      amount,
		})
	}

	req := gl.CreateJournalEntryRequest{
		EntityID:     run.EntityID,
		EntryDate:    run.DepreciationDate,
		Description:  fmt.Sprintf("Fixed Asset Depreciation - %s", run.RunNumber),
		CurrencyCode: run.Currency.Code,
		Lines:        lines,
	}

	return e.glAPI.CreateJournalEntry(ctx, req)
}

func (e *DepreciationEngine) ReverseDepreciationRun(ctx context.Context, runID common.ID) (*domain.DepreciationRun, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	var reversalRun *domain.DepreciationRun

	err := e.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		run, err := e.depRepo.WithTx(tx).GetRunByIDForUpdate(ctx, tx, runID)
		if err != nil {
			return fmt.Errorf("failed to get depreciation run: %w", err)
		}

		if !run.Status.CanReverse() {
			return fmt.Errorf("cannot reverse run with status: %s", run.Status)
		}

		entries, err := e.depRepo.WithTx(tx).GetEntriesByRun(ctx, runID)
		if err != nil {
			return fmt.Errorf("failed to get depreciation entries: %w", err)
		}

		if run.JournalEntryID != nil {
			_, err := e.glAPI.ReverseJournalEntry(ctx, *run.JournalEntryID, time.Now())
			if err != nil {
				return fmt.Errorf("failed to reverse GL entry: %w", err)
			}
		}

		for _, entry := range entries {
			asset, err := e.assetRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, entry.AssetID)
			if err != nil {
				return fmt.Errorf("failed to get asset %s: %w", entry.AssetID, err)
			}

			asset.AccumulatedDepreciation = asset.AccumulatedDepreciation.MustSubtract(entry.DepreciationAmount)
			asset.BookValue = asset.AcquisitionCost.MustSubtract(asset.AccumulatedDepreciation)

			if err := e.assetRepo.WithTx(tx).UpdateDepreciation(ctx, asset); err != nil {
				return fmt.Errorf("failed to update asset %s: %w", asset.AssetCode, err)
			}
		}

		if err := run.Reverse(common.ID(userID)); err != nil {
			return fmt.Errorf("failed to mark run as reversed: %w", err)
		}

		if err := e.depRepo.WithTx(tx).UpdateRun(ctx, run); err != nil {
			return fmt.Errorf("failed to update depreciation run: %w", err)
		}

		reversalRun = run

		if e.auditLogger != nil {
			_ = e.auditLogger.LogAction(ctx, "fa.depreciation_run", run.ID, "reversed", map[string]any{
				"run_number":         run.RunNumber,
				"asset_count":        run.AssetCount,
				"total_depreciation": run.TotalDepreciation.String(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return reversalRun, nil
}

func (e *DepreciationEngine) GetDepreciationSchedule(
	ctx context.Context,
	assetID common.ID,
) ([]domain.DepreciationEntry, error) {
	return e.depRepo.GetEntriesByAsset(ctx, assetID)
}

func (e *DepreciationEngine) RecordUnitsDepreciation(
	ctx context.Context,
	assetID common.ID,
	unitsUsed int,
	fiscalPeriodID common.ID,
	periodDate time.Time,
) (*domain.DepreciationEntry, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	asset, err := e.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	if asset.DepreciationMethod != domain.DepreciationMethodUnitsOfProduction {
		return nil, fmt.Errorf("asset does not use units of production depreciation")
	}

	amount, basis, err := e.calculator.CalculateUnitsDepreciation(asset, unitsUsed)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate depreciation: %w", err)
	}

	runNumber, err := e.depRepo.GetNextRunNumber(ctx, asset.EntityID, "UDEP")
	if err != nil {
		return nil, fmt.Errorf("failed to generate run number: %w", err)
	}

	run, err := domain.NewDepreciationRun(
		asset.EntityID,
		runNumber,
		fiscalPeriodID,
		periodDate,
		asset.Currency,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create depreciation run: %w", err)
	}

	monthsElapsed := asset.GetMonthsInService(periodDate)
	entry, err := domain.NewDepreciationEntry(run.ID, asset, amount, monthsElapsed, basis)
	if err != nil {
		return nil, fmt.Errorf("failed to create depreciation entry: %w", err)
	}

	if err := run.AddEntry(*entry); err != nil {
		return nil, fmt.Errorf("failed to add entry: %w", err)
	}

	if err := run.Calculate(); err != nil {
		return nil, fmt.Errorf("failed to calculate run: %w", err)
	}

	if err := e.depRepo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save run: %w", err)
	}

	if err := e.depRepo.CreateEntry(ctx, entry); err != nil {
		return nil, fmt.Errorf("failed to save entry: %w", err)
	}

	if err := asset.RecordUnits(unitsUsed); err != nil {
		return nil, fmt.Errorf("failed to record units: %w", err)
	}

	if err := e.assetRepo.Update(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to update asset: %w", err)
	}

	return entry, nil
}
