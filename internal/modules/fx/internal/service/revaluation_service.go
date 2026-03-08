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

type RevaluationService struct {
	accountConfigRepo repository.AccountFXConfigRepository
	runRepo           repository.RevaluationRunRepository
	detailRepo        repository.RevaluationDetailRepository
	rateService       money.ExchangeRateService
	auditLogger       *audit.Logger
	logger            *zap.Logger
}

func NewRevaluationService(
	accountConfigRepo repository.AccountFXConfigRepository,
	runRepo repository.RevaluationRunRepository,
	detailRepo repository.RevaluationDetailRepository,
	rateService money.ExchangeRateService,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *RevaluationService {
	return &RevaluationService{
		accountConfigRepo: accountConfigRepo,
		runRepo:           runRepo,
		detailRepo:        detailRepo,
		rateService:       rateService,
		auditLogger:       auditLogger,
		logger:            logger,
	}
}

type ConfigureAccountFXRequest struct {
	EntityID                 common.ID
	AccountID                common.ID
	FXTreatment              domain.AccountFXTreatment
	RevaluationGainAccountID common.ID
	RevaluationLossAccountID common.ID
}

func (s *RevaluationService) ConfigureAccountFX(ctx context.Context, req ConfigureAccountFXRequest) (*domain.AccountFXConfig, error) {
	existing, err := s.accountConfigRepo.GetByAccountID(ctx, req.EntityID, req.AccountID)
	if err != nil && err != domain.ErrAccountFXConfigNotFound {
		return nil, fmt.Errorf("failed to check existing config: %w", err)
	}

	if existing != nil {
		existing.SetTreatment(req.FXTreatment)
		if !req.RevaluationGainAccountID.IsZero() && !req.RevaluationLossAccountID.IsZero() {
			existing.SetGainLossAccounts(req.RevaluationGainAccountID, req.RevaluationLossAccountID)
		}
		if err := s.accountConfigRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update config: %w", err)
		}
		return existing, nil
	}

	config := domain.NewAccountFXConfig(req.EntityID, req.AccountID, req.FXTreatment)
	if !req.RevaluationGainAccountID.IsZero() && !req.RevaluationLossAccountID.IsZero() {
		config.SetGainLossAccounts(req.RevaluationGainAccountID, req.RevaluationLossAccountID)
	}

	if err := s.accountConfigRepo.Create(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	return config, nil
}

func (s *RevaluationService) GetAccountFXConfig(ctx context.Context, entityID, accountID common.ID) (*domain.AccountFXConfig, error) {
	return s.accountConfigRepo.GetByAccountID(ctx, entityID, accountID)
}

func (s *RevaluationService) ListAccountFXConfigs(ctx context.Context, entityID common.ID, treatment *domain.AccountFXTreatment) ([]domain.AccountFXConfig, error) {
	return s.accountConfigRepo.ListByEntity(ctx, entityID, treatment)
}

type CreateRevaluationRunRequest struct {
	EntityID           common.ID
	FiscalPeriodID     common.ID
	RevaluationDate    time.Time
	RateDate           time.Time
	FunctionalCurrency money.Currency
	CreatedBy          common.ID
}

func (s *RevaluationService) CreateRevaluationRun(ctx context.Context, req CreateRevaluationRunRequest) (*domain.RevaluationRun, error) {
	runNumber, err := s.runRepo.GenerateRunNumber(ctx, req.EntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate run number: %w", err)
	}

	run := domain.NewRevaluationRun(
		req.EntityID,
		runNumber,
		req.FiscalPeriodID,
		req.RevaluationDate,
		req.RateDate,
		req.FunctionalCurrency,
		req.CreatedBy,
	)

	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create revaluation run: %w", err)
	}

	err = s.auditLogger.Log(ctx, "fx_revaluation", run.ID, "revaluation.run.created", map[string]any{
		"entity_id":  req.EntityID,
		"run_number": runNumber,
	})
	if err != nil {
		s.logger.Warn("failed to log audit event", zap.Error(err))
	}

	return run, nil
}

func (s *RevaluationService) GetRevaluationRun(ctx context.Context, id common.ID) (*domain.RevaluationRun, error) {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	details, err := s.detailRepo.GetByRunID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load details: %w", err)
	}
	run.Details = details

	return run, nil
}

func (s *RevaluationService) ListRevaluationRuns(ctx context.Context, filter repository.RevaluationRunFilter) ([]domain.RevaluationRun, int, error) {
	return s.runRepo.List(ctx, filter)
}

type AccountBalanceProvider interface {
	GetForeignCurrencyBalances(ctx context.Context, entityID, periodID common.ID, functionalCurrency string) ([]repository.AccountBalance, error)
}

type ExecuteRevaluationRequest struct {
	RunID                common.ID
	BalanceProvider      AccountBalanceProvider
	DefaultGainAccountID common.ID
	DefaultLossAccountID common.ID
}

func (s *RevaluationService) ExecuteRevaluation(ctx context.Context, req ExecuteRevaluationRequest) (*domain.RevaluationRun, error) {
	run, err := s.runRepo.GetByID(ctx, req.RunID)
	if err != nil {
		return nil, err
	}

	if !run.CanEdit() {
		return nil, domain.ErrInvalidRevaluationStatus
	}

	if err := s.detailRepo.DeleteByRunID(ctx, run.ID); err != nil {
		return nil, fmt.Errorf("failed to clear existing details: %w", err)
	}

	monetaryTreatment := domain.AccountFXTreatmentMonetary
	configs, err := s.accountConfigRepo.ListByEntity(ctx, run.EntityID, &monetaryTreatment)
	if err != nil {
		return nil, fmt.Errorf("failed to list account configs: %w", err)
	}

	configMap := make(map[common.ID]*domain.AccountFXConfig)
	for i := range configs {
		configMap[configs[i].AccountID] = &configs[i]
	}

	balances, err := req.BalanceProvider.GetForeignCurrencyBalances(
		ctx,
		run.EntityID,
		run.FiscalPeriodID,
		run.FunctionalCurrency.Code,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balances: %w", err)
	}

	var details []domain.RevaluationDetail
	totalGain := decimal.Zero
	totalLoss := decimal.Zero

	for _, balance := range balances {

		if balance.Balance.IsZero() {
			continue
		}

		config, hasConfig := configMap[balance.AccountID]
		if !hasConfig {
			continue
		}

		foreignCurrency, err := money.GetCurrency(balance.CurrencyCode)
		if err != nil {
			s.logger.Warn("unknown currency, skipping account",
				zap.String("currency", balance.CurrencyCode),
				zap.String("account", balance.AccountCode))
			continue
		}

		newRate, err := s.rateService.GetRate(ctx, foreignCurrency, run.FunctionalCurrency, run.RateDate, money.RateTypeSpot)
		if err != nil {
			s.logger.Warn("failed to get rate, skipping account",
				zap.String("currency", balance.CurrencyCode),
				zap.String("account", balance.AccountCode),
				zap.Error(err))
			continue
		}

		originalFunctionalAmount := balance.Balance.Mul(balance.OriginalFunctionalRate)
		newFunctionalAmount := balance.Balance.Mul(newRate.Rate)
		revaluationAmount := newFunctionalAmount.Sub(originalFunctionalAmount)

		if revaluationAmount.IsZero() {
			continue
		}

		var gainLossAccountID common.ID
		if revaluationAmount.GreaterThan(decimal.Zero) {
			if config.RevaluationGainAccountID != nil {
				gainLossAccountID = *config.RevaluationGainAccountID
			} else {
				gainLossAccountID = req.DefaultGainAccountID
			}
			totalGain = totalGain.Add(revaluationAmount)
		} else {
			if config.RevaluationLossAccountID != nil {
				gainLossAccountID = *config.RevaluationLossAccountID
			} else {
				gainLossAccountID = req.DefaultLossAccountID
			}
			totalLoss = totalLoss.Add(revaluationAmount.Abs())
		}

		detail := domain.NewRevaluationDetail(
			run.ID,
			balance.AccountID,
			balance.AccountCode,
			balance.AccountName,
			foreignCurrency,
			balance.Balance,
			balance.OriginalFunctionalRate,
			originalFunctionalAmount,
			newRate.Rate,
			newFunctionalAmount,
			gainLossAccountID,
		)

		details = append(details, *detail)
	}

	if len(details) == 0 {
		return nil, domain.ErrNoMonetaryAccounts
	}

	if err := s.detailRepo.CreateBatch(ctx, details); err != nil {
		return nil, fmt.Errorf("failed to save revaluation details: %w", err)
	}

	run.Details = details
	run.TotalUnrealizedGain = totalGain
	run.TotalUnrealizedLoss = totalLoss
	run.NetRevaluation = totalGain.Sub(totalLoss)
	run.AccountsProcessed = len(details)
	run.UpdatedAt = time.Now()

	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update run: %w", err)
	}

	err = s.auditLogger.Log(ctx, "fx_revaluation", run.ID, "revaluation.run.executed", map[string]any{
		"entity_id":          run.EntityID,
		"run_number":         run.RunNumber,
		"accounts_processed": len(details),
	})
	if err != nil {
		s.logger.Warn("failed to log audit event", zap.Error(err))
	}

	return run, nil
}

func (s *RevaluationService) SubmitForApproval(ctx context.Context, runID common.ID) (*domain.RevaluationRun, error) {
	run, err := s.GetRevaluationRun(ctx, runID)
	if err != nil {
		return nil, err
	}

	if err := run.SubmitForApproval(); err != nil {
		return nil, err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update run: %w", err)
	}

	err = s.auditLogger.Log(ctx, "fx_revaluation", run.ID, "revaluation.run.submitted", map[string]any{
		"entity_id":  run.EntityID,
		"run_number": run.RunNumber,
	})
	if err != nil {
		s.logger.Warn("failed to log audit event", zap.Error(err))
	}

	return run, nil
}

func (s *RevaluationService) ApproveRevaluation(ctx context.Context, runID, approverID common.ID) (*domain.RevaluationRun, error) {
	run, err := s.GetRevaluationRun(ctx, runID)
	if err != nil {
		return nil, err
	}

	if err := run.Approve(approverID); err != nil {
		return nil, err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update run: %w", err)
	}

	s.auditLogger.Log(ctx, "fx_revaluation", run.ID, "revaluation.run.approved", map[string]any{
		"entity_id":   run.EntityID,
		"run_number":  run.RunNumber,
		"approved_by": approverID,
	})

	return run, nil
}

type PostRevaluationRequest struct {
	RunID          common.ID
	PosterID       common.ID
	JournalEntryID common.ID
}

func (s *RevaluationService) PostRevaluation(ctx context.Context, req PostRevaluationRequest) (*domain.RevaluationRun, error) {
	run, err := s.GetRevaluationRun(ctx, req.RunID)
	if err != nil {
		return nil, err
	}

	if err := run.Post(req.PosterID, req.JournalEntryID); err != nil {
		return nil, err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update run: %w", err)
	}

	s.auditLogger.Log(ctx, "fx_revaluation", run.ID, "revaluation.run.posted", map[string]any{
		"entity_id":        run.EntityID,
		"run_number":       run.RunNumber,
		"posted_by":        req.PosterID,
		"journal_entry_id": req.JournalEntryID,
	})

	return run, nil
}

type ReverseRevaluationRequest struct {
	RunID             common.ID
	ReversedBy        common.ID
	ReversalJournalID common.ID
}

func (s *RevaluationService) ReverseRevaluation(ctx context.Context, req ReverseRevaluationRequest) (*domain.RevaluationRun, error) {
	run, err := s.GetRevaluationRun(ctx, req.RunID)
	if err != nil {
		return nil, err
	}

	if err := run.Reverse(req.ReversedBy, req.ReversalJournalID); err != nil {
		return nil, err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update run: %w", err)
	}

	s.auditLogger.Log(ctx, "fx_revaluation", run.ID, "revaluation.run.reversed", map[string]any{
		"entity_id":           run.EntityID,
		"run_number":          run.RunNumber,
		"reversed_by":         req.ReversedBy,
		"reversal_journal_id": req.ReversalJournalID,
	})

	return run, nil
}

type RevaluationPreview struct {
	Details             []RevaluationPreviewDetail
	TotalUnrealizedGain decimal.Decimal
	TotalUnrealizedLoss decimal.Decimal
	NetRevaluation      decimal.Decimal
	AccountsProcessed   int
}

type RevaluationPreviewDetail struct {
	AccountID                common.ID
	AccountCode              string
	AccountName              string
	OriginalCurrency         string
	OriginalBalance          decimal.Decimal
	OriginalRate             decimal.Decimal
	OriginalFunctionalAmount decimal.Decimal
	NewRate                  decimal.Decimal
	NewFunctionalAmount      decimal.Decimal
	RevaluationAmount        decimal.Decimal
	IsGain                   bool
}

type PreviewRevaluationRequest struct {
	EntityID           common.ID
	FiscalPeriodID     common.ID
	RateDate           time.Time
	FunctionalCurrency money.Currency
	BalanceProvider    AccountBalanceProvider
}

func (s *RevaluationService) PreviewRevaluation(ctx context.Context, req PreviewRevaluationRequest) (*RevaluationPreview, error) {

	monetaryTreatment := domain.AccountFXTreatmentMonetary
	configs, err := s.accountConfigRepo.ListByEntity(ctx, req.EntityID, &monetaryTreatment)
	if err != nil {
		return nil, fmt.Errorf("failed to list account configs: %w", err)
	}

	configMap := make(map[common.ID]bool)
	for _, cfg := range configs {
		configMap[cfg.AccountID] = true
	}

	balances, err := req.BalanceProvider.GetForeignCurrencyBalances(
		ctx,
		req.EntityID,
		req.FiscalPeriodID,
		req.FunctionalCurrency.Code,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get account balances: %w", err)
	}

	preview := &RevaluationPreview{
		Details:             []RevaluationPreviewDetail{},
		TotalUnrealizedGain: decimal.Zero,
		TotalUnrealizedLoss: decimal.Zero,
		NetRevaluation:      decimal.Zero,
		AccountsProcessed:   0,
	}

	for _, balance := range balances {
		if balance.Balance.IsZero() {
			continue
		}

		if !configMap[balance.AccountID] {
			continue
		}

		foreignCurrency, err := money.GetCurrency(balance.CurrencyCode)
		if err != nil {
			continue
		}

		newRate, err := s.rateService.GetRate(ctx, foreignCurrency, req.FunctionalCurrency, req.RateDate, money.RateTypeSpot)
		if err != nil {
			continue
		}

		originalFunctionalAmount := balance.Balance.Mul(balance.OriginalFunctionalRate)
		newFunctionalAmount := balance.Balance.Mul(newRate.Rate)
		revaluationAmount := newFunctionalAmount.Sub(originalFunctionalAmount)

		if revaluationAmount.IsZero() {
			continue
		}

		isGain := revaluationAmount.GreaterThan(decimal.Zero)
		if isGain {
			preview.TotalUnrealizedGain = preview.TotalUnrealizedGain.Add(revaluationAmount)
		} else {
			preview.TotalUnrealizedLoss = preview.TotalUnrealizedLoss.Add(revaluationAmount.Abs())
		}

		preview.Details = append(preview.Details, RevaluationPreviewDetail{
			AccountID:                balance.AccountID,
			AccountCode:              balance.AccountCode,
			AccountName:              balance.AccountName,
			OriginalCurrency:         balance.CurrencyCode,
			OriginalBalance:          balance.Balance,
			OriginalRate:             balance.OriginalFunctionalRate,
			OriginalFunctionalAmount: originalFunctionalAmount,
			NewRate:                  newRate.Rate,
			NewFunctionalAmount:      newFunctionalAmount,
			RevaluationAmount:        revaluationAmount,
			IsGain:                   isGain,
		})
	}

	preview.NetRevaluation = preview.TotalUnrealizedGain.Sub(preview.TotalUnrealizedLoss)
	preview.AccountsProcessed = len(preview.Details)

	return preview, nil
}
