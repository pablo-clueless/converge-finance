package service

import (
	"context"
	"database/sql"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
)

type PostingEngine struct {
	db          *database.PostgresDB
	journalRepo repository.JournalRepository
	accountRepo repository.AccountRepository
	periodRepo  repository.PeriodRepository
	balanceRepo repository.AccountBalanceRepository
	auditLogger *audit.Logger
}

func NewPostingEngine(
	db *database.PostgresDB,
	journalRepo repository.JournalRepository,
	accountRepo repository.AccountRepository,
	periodRepo repository.PeriodRepository,
	balanceRepo repository.AccountBalanceRepository,
	auditLogger *audit.Logger,
) *PostingEngine {
	return &PostingEngine{
		db:          db,
		journalRepo: journalRepo,
		accountRepo: accountRepo,
		periodRepo:  periodRepo,
		balanceRepo: balanceRepo,
		auditLogger: auditLogger,
	}
}

func (e *PostingEngine) PostEntry(ctx context.Context, entryID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	return e.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		entry, err := e.journalRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, entryID)
		if err != nil {
			return fmt.Errorf("failed to get journal entry: %w", err)
		}

		if err := e.validateForPosting(ctx, tx, entry); err != nil {
			return err
		}

		if err := entry.Post(common.ID(userID)); err != nil {
			return fmt.Errorf("failed to post entry: %w", err)
		}

		if err := e.journalRepo.WithTx(tx).Update(ctx, entry); err != nil {
			return fmt.Errorf("failed to update entry: %w", err)
		}

		if err := e.updateBalances(ctx, tx, entry); err != nil {
			return fmt.Errorf("failed to update balances: %w", err)
		}

		if e.auditLogger != nil {
			_ = e.auditLogger.LogAction(ctx, "gl.journal_entry", entry.ID, "posted", map[string]any{
				"entry_number":  entry.EntryNumber,
				"total_debits":  entry.TotalDebits().String(),
				"total_credits": entry.TotalCredits().String(),
				"line_count":    len(entry.Lines),
				"fiscal_period": entry.FiscalPeriodID,
			})
		}

		return nil
	})
}

func (e *PostingEngine) validateForPosting(ctx context.Context, tx *sql.Tx, entry *domain.JournalEntry) error {

	if !entry.IsBalanced() {
		return fmt.Errorf("journal entry is not balanced: debits=%s, credits=%s",
			entry.TotalDebits().String(), entry.TotalCredits().String())
	}

	if len(entry.Lines) < 2 {
		return fmt.Errorf("journal entry must have at least 2 lines")
	}

	period, err := e.periodRepo.WithTx(tx).GetPeriodByID(ctx, entry.FiscalPeriodID)
	if err != nil {
		return fmt.Errorf("failed to get fiscal period: %w", err)
	}
	if !period.CanPost() {
		return fmt.Errorf("fiscal period %s is %s, cannot post", period.PeriodName, period.Status)
	}

	accountRepo := e.accountRepo.WithTx(tx)
	for _, line := range entry.Lines {
		account, err := accountRepo.GetByID(ctx, line.AccountID)
		if err != nil {
			return fmt.Errorf("failed to get account %s: %w", line.AccountID, err)
		}
		if !account.CanPost() {
			return fmt.Errorf("account %s (%s) cannot receive postings", account.Code, account.Name)
		}
	}

	return nil
}

func (e *PostingEngine) updateBalances(ctx context.Context, tx *sql.Tx, entry *domain.JournalEntry) error {
	balanceRepo := e.balanceRepo.WithTx(tx)

	for _, line := range entry.Lines {

		balance, err := balanceRepo.GetByAccountAndPeriod(ctx, line.AccountID, entry.FiscalPeriodID)
		if err != nil {

			balance = &repository.AccountBalance{
				ID:             common.NewID(),
				EntityID:       entry.EntityID,
				AccountID:      line.AccountID,
				FiscalPeriodID: entry.FiscalPeriodID,
			}
		}

		if line.IsDebit() {
			balance.PeriodDebit += line.BaseDebit.Amount.InexactFloat64()
		} else {
			balance.PeriodCredit += line.BaseCredit.Amount.InexactFloat64()
		}

		balance.ClosingDebit = balance.OpeningDebit + balance.PeriodDebit
		balance.ClosingCredit = balance.OpeningCredit + balance.PeriodCredit

		if err := balanceRepo.UpsertBalance(ctx, balance); err != nil {
			return fmt.Errorf("failed to update balance for account %s: %w", line.AccountID, err)
		}
	}

	return nil
}

func (e *PostingEngine) ReverseEntry(ctx context.Context, entryID common.ID, reversalDate string) (*domain.JournalEntry, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	var reversal *domain.JournalEntry

	err := e.db.WithTransaction(ctx, func(tx *sql.Tx) error {

		entry, err := e.journalRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, entryID)
		if err != nil {
			return fmt.Errorf("failed to get journal entry: %w", err)
		}

		if !entry.CanReverse() {
			return fmt.Errorf("entry cannot be reversed: status=%s, already_reversed=%v",
				entry.Status, entry.ReversedByID != nil)
		}

		reversalNumber, err := e.journalRepo.WithTx(tx).GetNextEntryNumber(ctx, entry.EntityID, "REV")
		if err != nil {
			return fmt.Errorf("failed to generate reversal number: %w", err)
		}

		reversalTime := entry.EntryDate

		reversal, err = entry.Reverse(reversalTime, reversalNumber, common.ID(userID))
		if err != nil {
			return fmt.Errorf("failed to create reversal: %w", err)
		}

		if err := e.journalRepo.WithTx(tx).Create(ctx, reversal); err != nil {
			return fmt.Errorf("failed to save reversal: %w", err)
		}

		if err := e.journalRepo.WithTx(tx).Update(ctx, entry); err != nil {
			return fmt.Errorf("failed to update original entry: %w", err)
		}

		if err := reversal.Post(common.ID(userID)); err != nil {
			return fmt.Errorf("failed to post reversal: %w", err)
		}

		if err := e.journalRepo.WithTx(tx).Update(ctx, reversal); err != nil {
			return fmt.Errorf("failed to save posted reversal: %w", err)
		}

		if err := e.updateBalances(ctx, tx, reversal); err != nil {
			return fmt.Errorf("failed to update balances for reversal: %w", err)
		}

		if e.auditLogger != nil {
			_ = e.auditLogger.LogAction(ctx, "gl.journal_entry", entry.ID, "reversed", map[string]any{
				"reversal_entry_id":     reversal.ID,
				"reversal_entry_number": reversal.EntryNumber,
			})
			_ = e.auditLogger.LogAction(ctx, "gl.journal_entry", reversal.ID, "posted", map[string]any{
				"is_reversal":    true,
				"reversal_of_id": entry.ID,
				"total_debits":   reversal.TotalDebits().String(),
				"total_credits":  reversal.TotalCredits().String(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return reversal, nil
}

func (e *PostingEngine) ValidateEntry(ctx context.Context, entry *domain.JournalEntry) error {

	if err := entry.Validate(); err != nil {
		return err
	}

	period, err := e.periodRepo.GetPeriodByID(ctx, entry.FiscalPeriodID)
	if err != nil {
		return fmt.Errorf("invalid fiscal period: %w", err)
	}
	if !period.CanPost() {
		return fmt.Errorf("fiscal period %s is not open for posting", period.PeriodName)
	}

	for _, line := range entry.Lines {
		account, err := e.accountRepo.GetByID(ctx, line.AccountID)
		if err != nil {
			return fmt.Errorf("invalid account %s: %w", line.AccountID, err)
		}
		if !account.CanPost() {
			return fmt.Errorf("account %s cannot receive postings", account.Code)
		}
	}

	return nil
}
