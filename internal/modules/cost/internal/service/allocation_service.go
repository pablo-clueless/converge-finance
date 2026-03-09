package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
	"converge-finance.com/m/internal/modules/cost/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
)

type AllocationService struct {
	db             *database.PostgresDB
	ruleRepo       repository.AllocationRuleRepository
	runRepo        repository.AllocationRunRepository
	costCenterRepo repository.CostCenterRepository
	glAPI          gl.API
	auditLogger    *audit.Logger
}

func NewAllocationService(
	db *database.PostgresDB,
	ruleRepo repository.AllocationRuleRepository,
	runRepo repository.AllocationRunRepository,
	costCenterRepo repository.CostCenterRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *AllocationService {
	return &AllocationService{
		db:             db,
		ruleRepo:       ruleRepo,
		runRepo:        runRepo,
		costCenterRepo: costCenterRepo,
		glAPI:          glAPI,
		auditLogger:    auditLogger,
	}
}

func (s *AllocationService) CreateAllocationRule(
	ctx context.Context,
	entityID common.ID,
	ruleCode string,
	ruleName string,
	sourceCostCenterID common.ID,
	allocationMethod domain.AllocationMethod,
) (*domain.AllocationRule, error) {
	_, err := s.costCenterRepo.GetByID(ctx, sourceCostCenterID)
	if err != nil {
		return nil, fmt.Errorf("source cost center not found: %w", err)
	}

	rule, err := domain.NewAllocationRule(entityID, ruleCode, ruleName, sourceCostCenterID, allocationMethod)
	if err != nil {
		return nil, fmt.Errorf("invalid allocation rule: %w", err)
	}

	if err := s.ruleRepo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to save allocation rule: %w", err)
	}

	if s.auditLogger != nil {
		if err := s.auditLogger.LogAction(ctx, "cost.allocation_rule", rule.ID, "created", map[string]any{
			"rule_code":         rule.RuleCode,
			"allocation_method": rule.AllocationMethod,
		}); err != nil {
			return nil, fmt.Errorf("failed to log audit event: %w", err)
		}
	}

	return rule, nil
}

func (s *AllocationService) AddAllocationTarget(
	ctx context.Context,
	ruleID common.ID,
	targetCostCenterID common.ID,
	fixedPercent *float64,
	driverValue *float64,
) (*domain.AllocationTarget, error) {
	rule, err := s.ruleRepo.GetByID(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("allocation rule not found: %w", err)
	}

	targetCenter, err := s.costCenterRepo.GetByID(ctx, targetCostCenterID)
	if err != nil {
		return nil, fmt.Errorf("target cost center not found: %w", err)
	}

	if targetCenter.ID == rule.SourceCostCenterID {
		return nil, fmt.Errorf("target cannot be the same as source")
	}

	target, err := domain.NewAllocationTarget(targetCostCenterID, fixedPercent, driverValue)
	if err != nil {
		return nil, err
	}

	target.AllocationRuleID = ruleID

	if err := s.ruleRepo.AddTarget(ctx, target); err != nil {
		return nil, fmt.Errorf("failed to add target: %w", err)
	}

	return target, nil
}

func (s *AllocationService) GetAllocationRule(ctx context.Context, id common.ID) (*domain.AllocationRule, error) {
	rule, err := s.ruleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	targets, err := s.ruleRepo.GetTargets(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get targets: %w", err)
	}
	rule.Targets = targets

	return rule, nil
}

func (s *AllocationService) ListAllocationRules(ctx context.Context, filter domain.AllocationRuleFilter) ([]domain.AllocationRule, error) {
	return s.ruleRepo.List(ctx, filter)
}

func (s *AllocationService) InitiateAllocationRun(
	ctx context.Context,
	entityID common.ID,
	fiscalPeriodID common.ID,
	allocationDate time.Time,
	currency money.Currency,
) (*domain.AllocationRun, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	runNumber, err := s.runRepo.GetNextRunNumber(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get run number: %w", err)
	}

	run, err := domain.NewAllocationRun(entityID, runNumber, fiscalPeriodID, allocationDate, common.ID(userID), currency)
	if err != nil {
		return nil, fmt.Errorf("failed to create allocation run: %w", err)
	}

	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save allocation run: %w", err)
	}

	if s.auditLogger != nil {
		if err := s.auditLogger.LogAction(ctx, "cost.allocation_run", run.ID, "initiated", map[string]any{
			"run_number":       run.RunNumber,
			"fiscal_period_id": fiscalPeriodID,
		}); err != nil {
			return nil, fmt.Errorf("failed to log audit event: %w", err)
		}
	}

	return run, nil
}

func (s *AllocationService) ExecuteAllocation(ctx context.Context, runID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("allocation run not found: %w", err)
	}

	if err := run.StartProcessing(); err != nil {
		return err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}

	rules, err := s.ruleRepo.GetActiveByEntity(ctx, run.EntityID)
	if err != nil {
		return fmt.Errorf("failed to get allocation rules: %w", err)
	}

	var entries []domain.AllocationEntry
	lineNumber := 1
	totalAllocated := money.Zero(run.TotalAllocated.Currency)
	rulesExecuted := 0

	for _, rule := range rules {
		targets, err := s.ruleRepo.GetTargets(ctx, rule.ID)
		if err != nil {
			continue
		}

		if len(targets) == 0 {
			continue
		}

		sourceBalance, err := s.getSourceBalance(ctx, run.EntityID, rule.SourceCostCenterID, run.FiscalPeriodID)
		if err != nil {
			continue
		}

		if sourceBalance.IsZero() {
			continue
		}

		totalPercent := 0.0
		for _, target := range targets {
			if target.FixedPercent != nil {
				totalPercent += *target.FixedPercent
			}
		}

		if totalPercent == 0 {
			totalPercent = 100
		}

		for _, target := range targets {
			var allocPercent float64
			if target.FixedPercent != nil {
				allocPercent = *target.FixedPercent / totalPercent * 100
			} else {
				allocPercent = 100 / float64(len(targets))
			}

			allocatedAmount := sourceBalance.MultiplyFloat(allocPercent / 100)

			entry := domain.NewAllocationEntry(
				run.ID,
				lineNumber,
				rule.SourceCostCenterID,
				common.NewID(),
				target.TargetCostCenterID,
				common.NewID(),
				allocPercent,
				allocatedAmount,
			)
			entry.AllocationRuleID = &rule.ID
			entry.Description = fmt.Sprintf("Allocation from %s using rule %s", rule.SourceCostCenterCode, rule.RuleCode)

			entries = append(entries, *entry)
			totalAllocated = totalAllocated.MustAdd(allocatedAmount)
			lineNumber++
		}

		rulesExecuted++
	}

	if len(entries) > 0 {
		if err := s.runRepo.CreateEntries(ctx, entries); err != nil {
			return fmt.Errorf("failed to save allocation entries: %w", err)
		}
	}

	if err := run.Complete(common.ID(userID), rulesExecuted, totalAllocated); err != nil {
		return err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if s.auditLogger != nil {
		if err := s.auditLogger.LogAction(ctx, "cost.allocation_run", run.ID, "completed", map[string]any{
			"rules_executed":  rulesExecuted,
			"total_allocated": totalAllocated,
		}); err != nil {
			return fmt.Errorf("failed to log audit event: %w", err)
		}
	}

	return nil
}

func (s *AllocationService) getSourceBalance(ctx context.Context, entityID, costCenterID, fiscalPeriodID common.ID) (money.Money, error) {
	return money.New(10000, "USD"), nil
}

func (s *AllocationService) PostAllocation(ctx context.Context, runID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("allocation run not found: %w", err)
	}

	if run.Status != domain.AllocationStatusCompleted {
		return fmt.Errorf("can only post completed runs")
	}

	entries, err := s.runRepo.GetEntries(ctx, runID)
	if err != nil {
		return fmt.Errorf("failed to get entries: %w", err)
	}

	var jeLines []gl.JournalLineRequest
	for _, entry := range entries {
		jeLines = append(jeLines, gl.JournalLineRequest{
			AccountID:   entry.SourceAccountID,
			Description: fmt.Sprintf("Allocation out: %s", entry.Description),
			Debit:       money.Zero(entry.AllocatedAmount.Currency),
			Credit:      entry.AllocatedAmount,
		})

		jeLines = append(jeLines, gl.JournalLineRequest{
			AccountID:   entry.TargetAccountID,
			Description: fmt.Sprintf("Allocation in: %s", entry.Description),
			Debit:       entry.AllocatedAmount,
			Credit:      money.Zero(entry.AllocatedAmount.Currency),
		})
	}

	jeReq := gl.CreateJournalEntryRequest{
		EntityID:     run.EntityID,
		EntryDate:    run.AllocationDate,
		Description:  fmt.Sprintf("Cost Allocation Run %s", run.RunNumber),
		CurrencyCode: run.TotalAllocated.Currency.Code,
		Lines:        jeLines,
	}

	je, err := s.glAPI.CreateJournalEntry(ctx, jeReq)
	if err != nil {
		return fmt.Errorf("failed to create journal entry: %w", err)
	}

	if err := s.glAPI.PostJournalEntry(ctx, je.ID); err != nil {
		return fmt.Errorf("failed to post journal entry: %w", err)
	}

	if err := run.Post(common.ID(userID), je.ID); err != nil {
		return err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "cost.allocation_run", run.ID, "posted", map[string]any{
			"journal_entry_id": je.ID,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *AllocationService) ReverseAllocation(ctx context.Context, runID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	run, err := s.runRepo.GetByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("allocation run not found: %w", err)
	}

	if run.JournalEntryID != nil {
		_, err := s.glAPI.ReverseJournalEntry(ctx, *run.JournalEntryID, time.Now())
		if err != nil {
			return fmt.Errorf("failed to reverse journal entry: %w", err)
		}
	}

	if err := run.Reverse(common.ID(userID)); err != nil {
		return err
	}

	if err := s.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("failed to update run: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "cost.allocation_run", run.ID, "reversed", nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *AllocationService) GetAllocationRun(ctx context.Context, id common.ID) (*domain.AllocationRun, error) {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	entries, err := s.runRepo.GetEntries(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get entries: %w", err)
	}
	run.Entries = entries

	return run, nil
}

func (s *AllocationService) ListAllocationRuns(ctx context.Context, filter domain.AllocationRunFilter) ([]domain.AllocationRun, error) {
	return s.runRepo.List(ctx, filter)
}
