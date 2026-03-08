package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
)

type PeriodCloseService struct {
	db              *database.PostgresDB
	periodCloseRepo repository.PeriodCloseRepository
	closeRuleRepo   repository.CloseRuleRepository
	closeRunRepo    repository.CloseRunRepository
	closeEntryRepo  repository.CloseRunEntryRepository
	glAPI           gl.API
	auditLogger     *audit.Logger
}

func NewPeriodCloseService(
	db *database.PostgresDB,
	periodCloseRepo repository.PeriodCloseRepository,
	closeRuleRepo repository.CloseRuleRepository,
	closeRunRepo repository.CloseRunRepository,
	closeEntryRepo repository.CloseRunEntryRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *PeriodCloseService {
	return &PeriodCloseService{
		db:              db,
		periodCloseRepo: periodCloseRepo,
		closeRuleRepo:   closeRuleRepo,
		closeRunRepo:    closeRunRepo,
		closeEntryRepo:  closeEntryRepo,
		glAPI:           glAPI,
		auditLogger:     auditLogger,
	}
}

func (s *PeriodCloseService) GetPeriodCloseStatus(ctx context.Context, entityID, fiscalPeriodID common.ID) (*domain.PeriodClose, error) {
	return s.periodCloseRepo.GetByPeriod(ctx, entityID, fiscalPeriodID)
}

func (s *PeriodCloseService) InitializePeriodClose(ctx context.Context, entityID, fiscalPeriodID, fiscalYearID common.ID) (*domain.PeriodClose, error) {

	existing, err := s.periodCloseRepo.GetByPeriod(ctx, entityID, fiscalPeriodID)
	if err == nil && existing != nil {
		return existing, nil
	}

	pc := domain.NewPeriodClose(entityID, fiscalPeriodID, fiscalYearID)
	if err := s.periodCloseRepo.Create(ctx, pc); err != nil {
		return nil, fmt.Errorf("failed to create period close status: %w", err)
	}

	s.auditLogger.Log(ctx, "period_close", pc.ID, "create", map[string]interface{}{
		"fiscal_period_id": fiscalPeriodID,
		"fiscal_year_id":   fiscalYearID,
	})

	return pc, nil
}

func (s *PeriodCloseService) SoftClosePeriod(ctx context.Context, entityID, fiscalPeriodID common.ID, userID common.ID) (*domain.PeriodClose, error) {
	pc, err := s.periodCloseRepo.GetByPeriod(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get period close status: %w", err)
	}

	if err := pc.SoftClose(userID); err != nil {
		return nil, err
	}

	if err := s.periodCloseRepo.Update(ctx, pc); err != nil {
		return nil, fmt.Errorf("failed to update period close status: %w", err)
	}

	s.auditLogger.Log(ctx, "period_close", pc.ID, "soft_close", map[string]interface{}{
		"closed_by": userID,
	})

	return pc, nil
}

func (s *PeriodCloseService) HardClosePeriod(
	ctx context.Context,
	entityID, fiscalPeriodID, fiscalYearID common.ID,
	closeDate time.Time,
	currency money.Currency,
	userID common.ID,
) (*domain.CloseRun, error) {
	pc, err := s.periodCloseRepo.GetByPeriod(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get period close status: %w", err)
	}

	if pc.Status != domain.PeriodStatusSoftClosed {
		return nil, errors.New("period must be soft closed before hard close")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil {
			_ = s.auditLogger.Log(ctx, "period_close", pc.ID, "hard_close", map[string]interface{}{
				"error": rbErr.Error(),
			})
		}
	}()

	runNumber, err := s.closeRunRepo.WithTx(tx).GetNextRunNumber(ctx, entityID, "CLS")
	if err != nil {
		return nil, fmt.Errorf("failed to generate run number: %w", err)
	}

	run := domain.NewCloseRun(entityID, runNumber, domain.CloseTypePeriod, fiscalPeriodID, fiscalYearID, closeDate, currency, userID)
	if err := s.closeRunRepo.WithTx(tx).Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create close run: %w", err)
	}

	run, err = s.executeCloseRun(ctx, tx, run)
	if err != nil {
		_ = run.Fail(err.Error())
		_ = s.closeRunRepo.WithTx(tx).Update(ctx, run)
		_ = tx.Commit()
		return run, err
	}

	if err := pc.HardClose(userID, run.ClosingJournalEntryID); err != nil {
		return nil, err
	}

	if err := s.periodCloseRepo.WithTx(tx).Update(ctx, pc); err != nil {
		return nil, fmt.Errorf("failed to update period close status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.auditLogger.Log(ctx, "period_close", pc.ID, "hard_close", map[string]interface{}{
		"close_run_id": run.ID,
		"closed_by":    userID,
	})

	return run, nil
}

func (s *PeriodCloseService) executeCloseRun(ctx context.Context, tx *sql.Tx, run *domain.CloseRun) (*domain.CloseRun, error) {
	if err := run.StartProcessing(); err != nil {
		return run, err
	}
	_ = s.closeRunRepo.WithTx(tx).Update(ctx, run)

	rules, err := s.closeRuleRepo.GetActiveRulesForCloseType(ctx, run.EntityID, run.CloseType)
	if err != nil {
		return run, fmt.Errorf("failed to get close rules: %w", err)
	}

	if len(rules) == 0 {

		_ = run.Complete(common.NewID(), 0, 0, money.Zero(run.Currency), money.Zero(run.Currency))
		return run, nil
	}

	var journalLines []gl.JournalLineRequest
	var entries []domain.CloseRunEntry
	totalDebits := money.Zero(run.Currency)
	totalCredits := money.Zero(run.Currency)
	sequence := 0

	for _, rule := range rules {
		ruleEntries, ruleDebits, ruleCredits, err := s.generateEntriesForRule(ctx, run, &rule, &sequence)
		if err != nil {
			return run, fmt.Errorf("failed to generate entries for rule %s: %w", rule.RuleCode, err)
		}

		entries = append(entries, ruleEntries...)
		totalDebits = totalDebits.MustAdd(ruleDebits)
		totalCredits = totalCredits.MustAdd(ruleCredits)

		for _, entry := range ruleEntries {
			if entry.Amount.IsPositive() {

				journalLines = append(journalLines,
					gl.JournalLineRequest{
						AccountID:   entry.SourceAccountID,
						Description: entry.Description,
						Debit:       money.Zero(run.Currency),
						Credit:      entry.Amount,
					},
					gl.JournalLineRequest{
						AccountID:   entry.TargetAccountID,
						Description: entry.Description,
						Debit:       entry.Amount,
						Credit:      money.Zero(run.Currency),
					},
				)
			} else {

				absAmount := entry.Amount.Negate()
				journalLines = append(journalLines,
					gl.JournalLineRequest{
						AccountID:   entry.SourceAccountID,
						Description: entry.Description,
						Debit:       absAmount,
						Credit:      money.Zero(run.Currency),
					},
					gl.JournalLineRequest{
						AccountID:   entry.TargetAccountID,
						Description: entry.Description,
						Debit:       money.Zero(run.Currency),
						Credit:      absAmount,
					},
				)
			}
		}
	}

	if len(entries) == 0 {
		_ = run.Complete(common.NewID(), len(rules), 0, totalDebits, totalCredits)
		return run, nil
	}

	if err := s.closeEntryRepo.WithTx(tx).CreateBatch(ctx, entries); err != nil {
		return run, fmt.Errorf("failed to create close run entries: %w", err)
	}

	jeReq := gl.CreateJournalEntryRequest{
		EntityID:     run.EntityID,
		EntryDate:    run.CloseDate,
		Description:  fmt.Sprintf("Period closing entry - %s", run.RunNumber),
		CurrencyCode: run.Currency.Code,
		Lines:        journalLines,
	}

	je, err := s.glAPI.CreateJournalEntry(ctx, jeReq)
	if err != nil {
		return run, fmt.Errorf("failed to create closing journal entry: %w", err)
	}

	if err := s.glAPI.PostJournalEntry(ctx, je.ID); err != nil {
		return run, fmt.Errorf("failed to post closing journal entry: %w", err)
	}

	if err := run.Complete(je.ID, len(rules), len(entries), totalDebits, totalCredits); err != nil {
		return run, err
	}

	if err := s.closeRunRepo.WithTx(tx).Update(ctx, run); err != nil {
		return run, fmt.Errorf("failed to update close run: %w", err)
	}

	return run, nil
}

func (s *PeriodCloseService) generateEntriesForRule(
	ctx context.Context,
	run *domain.CloseRun,
	rule *domain.CloseRule,
	sequence *int,
) ([]domain.CloseRunEntry, money.Money, money.Money, error) {
	var entries []domain.CloseRunEntry
	totalDebits := money.Zero(run.Currency)
	totalCredits := money.Zero(run.Currency)

	var accountFilter gl.AccountFilterRequest
	if rule.SourceAccountType != "" {
		accountFilter.Type = &rule.SourceAccountType
	}
	isPosting := true
	accountFilter.IsPosting = &isPosting
	isActive := true
	accountFilter.IsActive = &isActive

	accounts, err := s.glAPI.ListAccounts(ctx, run.EntityID, accountFilter)
	if err != nil {
		return nil, totalDebits, totalCredits, err
	}

	targetAccount, err := s.glAPI.GetAccountByID(ctx, rule.TargetAccountID)
	if err != nil {
		return nil, totalDebits, totalCredits, fmt.Errorf("failed to get target account: %w", err)
	}

	for _, account := range accounts {

		if rule.SourceAccountID != nil && account.ID != *rule.SourceAccountID {
			continue
		}

		balance, err := s.glAPI.GetAccountBalance(ctx, account.ID, run.FiscalPeriodID)
		if err != nil {
			continue
		}

		var netAmount money.Money
		if balance.ClosingDebit.IsPositive() {
			netAmount = balance.ClosingDebit
		} else if balance.ClosingCredit.IsPositive() {
			netAmount = balance.ClosingCredit.Negate()
		} else {
			continue
		}

		*sequence++
		entry := domain.NewCloseRunEntry(
			run.ID,
			rule.ID,
			*sequence,
			account.ID,
			account.Code,
			account.Name,
			targetAccount.ID,
			targetAccount.Code,
			targetAccount.Name,
			netAmount,
			fmt.Sprintf("Close %s to %s", account.Code, targetAccount.Code),
		)

		entries = append(entries, *entry)

		if netAmount.IsPositive() {
			totalDebits = totalDebits.MustAdd(netAmount)
		} else {
			totalCredits = totalCredits.MustAdd(netAmount.Negate())
		}
	}

	return entries, totalDebits, totalCredits, nil
}

func (s *PeriodCloseService) ReopenPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID, userID common.ID, reason string) (*domain.PeriodClose, error) {
	pc, err := s.periodCloseRepo.GetByPeriod(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get period close status: %w", err)
	}

	if pc.Status == domain.PeriodStatusHardClosed && pc.ClosingJournalEntryID != nil {

		_, err := s.glAPI.ReverseJournalEntry(ctx, *pc.ClosingJournalEntryID, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to reverse closing journal entry: %w", err)
		}
	}

	if err := pc.Reopen(userID, reason); err != nil {
		return nil, err
	}

	if err := s.periodCloseRepo.Update(ctx, pc); err != nil {
		return nil, fmt.Errorf("failed to update period close status: %w", err)
	}

	s.auditLogger.Log(ctx, "period_close", pc.ID, "reopen", map[string]interface{}{
		"reopened_by": userID,
		"reason":      reason,
	})

	return pc, nil
}

func (s *PeriodCloseService) ListPeriodCloseStatuses(ctx context.Context, filter domain.PeriodCloseFilter) ([]domain.PeriodClose, error) {
	return s.periodCloseRepo.List(ctx, filter)
}

func (s *PeriodCloseService) CreateCloseRule(
	ctx context.Context,
	entityID common.ID,
	ruleCode, ruleName string,
	ruleType domain.CloseRuleType,
	closeType domain.CloseType,
	targetAccountID common.ID,
) (*domain.CloseRule, error) {
	rule := domain.NewCloseRule(entityID, ruleCode, ruleName, ruleType, closeType, targetAccountID)

	if err := s.closeRuleRepo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to create close rule: %w", err)
	}

	s.auditLogger.Log(ctx, "close_rule", rule.ID, "create", map[string]interface{}{
		"rule_code":         ruleCode,
		"rule_type":         ruleType,
		"close_type":        closeType,
		"target_account_id": targetAccountID,
	})

	return rule, nil
}

func (s *PeriodCloseService) GetCloseRule(ctx context.Context, id common.ID) (*domain.CloseRule, error) {
	return s.closeRuleRepo.GetByID(ctx, id)
}

func (s *PeriodCloseService) ListCloseRules(ctx context.Context, filter domain.CloseRuleFilter) ([]domain.CloseRule, error) {
	return s.closeRuleRepo.List(ctx, filter)
}

func (s *PeriodCloseService) GetCloseRun(ctx context.Context, id common.ID) (*domain.CloseRun, error) {
	run, err := s.closeRunRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	entries, err := s.closeEntryRepo.GetByRunID(ctx, id)
	if err == nil {
		run.Entries = entries
	}

	return run, nil
}

func (s *PeriodCloseService) ListCloseRuns(ctx context.Context, filter domain.CloseRunFilter) ([]domain.CloseRun, error) {
	return s.closeRunRepo.List(ctx, filter)
}

func (s *PeriodCloseService) ReverseCloseRun(ctx context.Context, runID common.ID, userID common.ID) (*domain.CloseRun, error) {
	run, err := s.closeRunRepo.GetByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get close run: %w", err)
	}

	if run.ClosingJournalEntryID == nil {
		return nil, errors.New("close run has no journal entry to reverse")
	}

	reversalJE, err := s.glAPI.ReverseJournalEntry(ctx, *run.ClosingJournalEntryID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to reverse journal entry: %w", err)
	}

	if err := run.Reverse(userID, reversalJE.ID); err != nil {
		return nil, err
	}

	if err := s.closeRunRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update close run: %w", err)
	}

	s.auditLogger.Log(ctx, "close_run", run.ID, "reverse", map[string]interface{}{
		"reversed_by":       userID,
		"reversal_entry_id": reversalJE.ID,
	})

	return run, nil
}
