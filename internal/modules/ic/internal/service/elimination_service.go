package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
	"converge-finance.com/m/internal/modules/ic/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
)

type EliminationService struct {
	db            *database.PostgresDB
	elimRepo      repository.EliminationRepository
	balanceRepo   repository.BalanceRepository
	hierarchyRepo repository.EntityHierarchyRepository
	mappingRepo   repository.AccountMappingRepository
	glAPI         gl.API
	auditLogger   *audit.Logger
}

func NewEliminationService(
	db *database.PostgresDB,
	elimRepo repository.EliminationRepository,
	balanceRepo repository.BalanceRepository,
	hierarchyRepo repository.EntityHierarchyRepository,
	mappingRepo repository.AccountMappingRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *EliminationService {
	return &EliminationService{
		db:            db,
		elimRepo:      elimRepo,
		balanceRepo:   balanceRepo,
		hierarchyRepo: hierarchyRepo,
		mappingRepo:   mappingRepo,
		glAPI:         glAPI,
		auditLogger:   auditLogger,
	}
}

type CreateRuleRequest struct {
	ParentEntityID  common.ID
	RuleCode        string
	RuleName        string
	EliminationType domain.EliminationType
	Description     string
	Config          domain.EliminationRuleConfig
	SequenceNumber  int
}

func (s *EliminationService) CreateRule(ctx context.Context, req CreateRuleRequest) (*domain.EliminationRule, error) {

	existing, _ := s.elimRepo.GetRuleByCode(ctx, req.ParentEntityID, req.RuleCode)
	if existing != nil {
		return nil, fmt.Errorf("rule code already exists: %s", req.RuleCode)
	}

	rule, err := domain.NewEliminationRule(
		req.ParentEntityID,
		req.RuleCode,
		req.RuleName,
		req.EliminationType,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	rule.Description = req.Description
	rule.SetConfig(req.Config)
	rule.SetSequence(req.SequenceNumber)

	if err := rule.Validate(); err != nil {
		return nil, err
	}

	if err := s.elimRepo.CreateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to save rule: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "ic.elimination_rule", rule.ID, "created", map[string]any{
			"rule_code":        rule.RuleCode,
			"rule_name":        rule.RuleName,
			"elimination_type": rule.EliminationType,
		})
	}

	return rule, nil
}

func (s *EliminationService) UpdateRule(ctx context.Context, ruleID common.ID, req CreateRuleRequest) (*domain.EliminationRule, error) {
	rule, err := s.elimRepo.GetRuleByID(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}

	rule.RuleName = req.RuleName
	rule.Description = req.Description
	rule.SetConfig(req.Config)
	rule.SetSequence(req.SequenceNumber)

	if err := rule.Validate(); err != nil {
		return nil, err
	}

	if err := s.elimRepo.UpdateRule(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.elimination_rule", rule.ID, "updated", map[string]any{
			"rule_code": rule.RuleCode,
			"rule_name": rule.RuleName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to log updated rule action: %w", err)
		}
	}

	return rule, nil
}

func (s *EliminationService) DeleteRule(ctx context.Context, ruleID common.ID) error {
	rule, err := s.elimRepo.GetRuleByID(ctx, ruleID)
	if err != nil {
		return fmt.Errorf("rule not found: %w", err)
	}

	if err := s.elimRepo.DeleteRule(ctx, ruleID); err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.elimination_rule", rule.ID, "deleted", map[string]any{
			"rule_code": rule.RuleCode,
		})
		if err != nil {
			return fmt.Errorf("failed to log deleted rule action: %w", err)
		}
	}

	return nil
}

func (s *EliminationService) GetRule(ctx context.Context, ruleID common.ID) (*domain.EliminationRule, error) {
	rule, err := s.elimRepo.GetRuleByID(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}
	return rule, nil
}

func (s *EliminationService) ListRules(ctx context.Context, parentEntityID common.ID) ([]domain.EliminationRule, error) {
	rules, err := s.elimRepo.GetActiveRules(ctx, parentEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to list rules: %w", err)
	}
	return rules, nil
}

type GenerateEliminationsRequest struct {
	ParentEntityID  common.ID
	FiscalPeriodID  common.ID
	EliminationDate time.Time
	CurrencyCode    string
}

func (s *EliminationService) GenerateEliminations(ctx context.Context, req GenerateEliminationsRequest) (*domain.EliminationRun, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	parent, err := s.hierarchyRepo.GetByID(ctx, req.ParentEntityID)
	if err != nil {
		return nil, fmt.Errorf("parent entity not found: %w", err)
	}

	currency := money.MustGetCurrency(req.CurrencyCode)

	runNumber, err := s.elimRepo.GetNextRunNumber(ctx, req.ParentEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate run number: %w", err)
	}

	run, err := domain.NewEliminationRun(
		req.ParentEntityID,
		req.FiscalPeriodID,
		req.EliminationDate,
		currency,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create elimination run: %w", err)
	}
	run.SetRunNumber(runNumber)

	rules, err := s.elimRepo.GetActiveRules(ctx, req.ParentEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get elimination rules: %w", err)
	}

	entities, err := s.hierarchyRepo.GetConsolidationGroup(ctx, req.ParentEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get consolidation group: %w", err)
	}

	for _, rule := range rules {
		entries, err := s.generateEntriesForRule(ctx, run, &rule, entities, req.FiscalPeriodID, currency)
		if err != nil {
			return nil, fmt.Errorf("failed to generate entries for rule %s: %w", rule.RuleCode, err)
		}
		for _, entry := range entries {
			if err := run.AddEntry(entry); err != nil {
				return nil, fmt.Errorf("failed to add entry: %w", err)
			}
		}
	}

	if len(run.Entries) > 0 && !run.IsBalanced() {
		return nil, fmt.Errorf("elimination entries are not balanced: debits=%s, credits=%s",
			run.GetTotalDebits().String(), run.GetTotalCredits().String())
	}

	if err := s.elimRepo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to save elimination run: %w", err)
	}

	if len(run.Entries) > 0 {
		if err := s.elimRepo.CreateEntries(ctx, run.Entries); err != nil {
			return nil, fmt.Errorf("failed to save elimination entries: %w", err)
		}
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "ic.elimination_run", run.ID, "generated", map[string]any{
			"run_number":    run.RunNumber,
			"parent_entity": parent.Code,
			"entry_count":   run.EntryCount,
			"total_amount":  run.TotalEliminations.String(),
		})
	}

	return run, nil
}

func (s *EliminationService) generateEntriesForRule(
	ctx context.Context,
	run *domain.EliminationRun,
	rule *domain.EliminationRule,
	entities []domain.EntityHierarchy,
	fiscalPeriodID common.ID,
	currency money.Currency,
) ([]domain.EliminationEntry, error) {
	var entries []domain.EliminationEntry

	switch rule.EliminationType {
	case domain.EliminationTypeICReceivablePayable:

		for i := 0; i < len(entities); i++ {
			for j := i + 1; j < len(entities); j++ {
				fromEntity := entities[i]
				toEntity := entities[j]

				balance, err := s.balanceRepo.GetByEntityPair(ctx, fromEntity.ID, toEntity.ID, fiscalPeriodID)
				if err != nil {
					continue
				}

				if !balance.ClosingBalance.IsZero() {

					absBalance := balance.ClosingBalance.Abs()

					if balance.ClosingBalance.IsPositive() {

						receivableEntry, _ := domain.NewEliminationEntry(
							rule.EliminationType,
							rule.ID,
							fmt.Sprintf("Eliminate IC receivable %s -> %s", fromEntity.Code, toEntity.Code),
							money.Zero(currency),
							absBalance,
						)
						receivableEntry.SetSourceEntities(&fromEntity.ID, &toEntity.ID)
						receivableEntry.SetRuleID(rule.ID)
						entries = append(entries, *receivableEntry)

						payableEntry, _ := domain.NewEliminationEntry(
							rule.EliminationType,
							rule.ID,
							fmt.Sprintf("Eliminate IC payable %s -> %s", toEntity.Code, fromEntity.Code),
							absBalance,
							money.Zero(currency),
						)
						payableEntry.SetSourceEntities(&toEntity.ID, &fromEntity.ID)
						payableEntry.SetRuleID(rule.ID)
						entries = append(entries, *payableEntry)
					}
				}
			}
		}

	case domain.EliminationTypeICRevenueExpense:

	case domain.EliminationTypeICDividend:

	case domain.EliminationTypeICInvestment:

	case domain.EliminationTypeICEquity:

	case domain.EliminationTypeUnrealizedProfit:

	}

	return entries, nil
}

func (s *EliminationService) PostEliminationRun(ctx context.Context, runID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	return s.db.WithTransaction(ctx, func(dbTx *sql.Tx) error {
		run, err := s.elimRepo.GetRunByIDForUpdate(ctx, dbTx, runID)
		if err != nil {
			return fmt.Errorf("elimination run not found: %w", err)
		}

		if !run.Status.CanPost() {
			return fmt.Errorf("elimination run cannot be posted, current status: %s", run.Status)
		}

		entries, err := s.elimRepo.GetEntriesByRun(ctx, runID)
		if err != nil {
			return fmt.Errorf("failed to load entries: %w", err)
		}
		run.Entries = entries

		if len(run.Entries) == 0 {
			return fmt.Errorf("elimination run has no entries")
		}

		if err := s.glAPI.ValidatePeriodOpen(ctx, run.ParentEntityID, run.EliminationDate); err != nil {
			return fmt.Errorf("period not open: %w", err)
		}

		var lines []gl.JournalLineRequest
		for _, entry := range run.Entries {
			lines = append(lines, gl.JournalLineRequest{
				AccountID:   entry.AccountID,
				Description: entry.Description,
				Debit:       entry.DebitAmount,
				Credit:      entry.CreditAmount,
			})
		}

		jeReq := gl.CreateJournalEntryRequest{
			EntityID:     run.ParentEntityID,
			EntryDate:    run.EliminationDate,
			Description:  fmt.Sprintf("Consolidation Eliminations - %s", run.RunNumber),
			CurrencyCode: run.Currency.Code,
			Lines:        lines,
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

		if err := s.elimRepo.WithTx(dbTx).UpdateRun(ctx, run); err != nil {
			return fmt.Errorf("failed to update elimination run: %w", err)
		}

		if s.auditLogger != nil {
			s.auditLogger.LogAction(ctx, "ic.elimination_run", run.ID, "posted", map[string]any{
				"run_number":       run.RunNumber,
				"journal_entry_id": je.ID,
			})
		}

		return nil
	})
}

func (s *EliminationService) ReverseEliminationRun(ctx context.Context, runID common.ID) (*domain.EliminationRun, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	run, err := s.elimRepo.GetRunByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("elimination run not found: %w", err)
	}

	if !run.Status.CanReverse() {
		return nil, fmt.Errorf("elimination run cannot be reversed, current status: %s", run.Status)
	}

	if run.JournalEntryID != nil {
		_, err := s.glAPI.ReverseJournalEntry(ctx, *run.JournalEntryID, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to reverse journal entry: %w", err)
		}
	}

	if err := run.Reverse(common.ID(userID)); err != nil {
		return nil, err
	}

	if err := s.elimRepo.UpdateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update elimination run: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "ic.elimination_run", run.ID, "reversed", map[string]any{
			"run_number": run.RunNumber,
		})
	}

	return run, nil
}

func (s *EliminationService) GetEliminationRun(ctx context.Context, runID common.ID) (*domain.EliminationRun, error) {
	run, err := s.elimRepo.GetRunByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("elimination run not found: %w", err)
	}

	entries, err := s.elimRepo.GetEntriesByRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load entries: %w", err)
	}
	run.Entries = entries

	return run, nil
}

func (s *EliminationService) ListEliminationRuns(ctx context.Context, filter domain.EliminationRunFilter) ([]domain.EliminationRun, int64, error) {
	runs, err := s.elimRepo.ListRuns(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list runs: %w", err)
	}

	count, err := s.elimRepo.CountRuns(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count runs: %w", err)
	}

	return runs, count, nil
}

func (s *EliminationService) DeleteEliminationRun(ctx context.Context, runID common.ID) error {
	run, err := s.elimRepo.GetRunByID(ctx, runID)
	if err != nil {
		return fmt.Errorf("elimination run not found: %w", err)
	}

	if run.Status != domain.EliminationStatusDraft {
		return fmt.Errorf("can only delete draft elimination runs")
	}

	if err := s.elimRepo.DeleteEntriesByRun(ctx, runID); err != nil {
		return fmt.Errorf("failed to delete entries: %w", err)
	}

	if err := s.elimRepo.DeleteRun(ctx, runID); err != nil {
		return fmt.Errorf("failed to delete run: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "ic.elimination_run", run.ID, "deleted", map[string]any{
			"run_number": run.RunNumber,
		})
	}

	return nil
}
