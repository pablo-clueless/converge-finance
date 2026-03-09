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
	"github.com/shopspring/decimal"
)

type ICTransactionService struct {
	db          *database.PostgresDB
	txRepo      repository.TransactionRepository
	mappingRepo repository.AccountMappingRepository
	balanceRepo repository.BalanceRepository
	glAPI       gl.API
	auditLogger *audit.Logger
}

func NewICTransactionService(
	db *database.PostgresDB,
	txRepo repository.TransactionRepository,
	mappingRepo repository.AccountMappingRepository,
	balanceRepo repository.BalanceRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *ICTransactionService {
	return &ICTransactionService{
		db:          db,
		txRepo:      txRepo,
		mappingRepo: mappingRepo,
		balanceRepo: balanceRepo,
		glAPI:       glAPI,
		auditLogger: auditLogger,
	}
}

type CreateTransactionRequest struct {
	FromEntityID    common.ID
	ToEntityID      common.ID
	TransactionType domain.TransactionType
	TransactionDate time.Time
	DueDate         *time.Time
	Amount          money.Money
	Description     string
	Reference       string
	Lines           []TransactionLineRequest
}

type TransactionLineRequest struct {
	Description    string
	Quantity       decimal.Decimal
	UnitPrice      decimal.Decimal
	Amount         money.Money
	CostCenterCode string
	ProjectCode    string
}

func (s *ICTransactionService) CreateTransaction(ctx context.Context, req CreateTransactionRequest) (*domain.ICTransaction, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	if req.FromEntityID == req.ToEntityID {
		return nil, fmt.Errorf("from and to entities must be different")
	}

	mapping, err := s.mappingRepo.GetByEntityPair(ctx, req.FromEntityID, req.ToEntityID, req.TransactionType)
	if err != nil {
		return nil, fmt.Errorf("no account mapping found for this entity pair and transaction type: %w", err)
	}
	if !mapping.IsActive {
		return nil, fmt.Errorf("account mapping is not active")
	}

	txNumber, err := s.txRepo.GetNextTransactionNumber(ctx, req.FromEntityID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate transaction number: %w", err)
	}

	tx, err := domain.NewICTransaction(
		req.FromEntityID,
		req.ToEntityID,
		req.TransactionType,
		req.TransactionDate,
		req.Amount,
		req.Description,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	tx.SetTransactionNumber(txNumber)
	if req.DueDate != nil {
		tx.SetDueDate(*req.DueDate)
	}
	if req.Reference != "" {
		tx.SetReference(req.Reference)
	}

	for _, line := range req.Lines {
		if err := tx.AddLine(line.Description, line.Quantity, line.UnitPrice, line.Amount); err != nil {
			return nil, fmt.Errorf("failed to add line: %w", err)
		}
	}

	if err := tx.Validate(); err != nil {
		return nil, err
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to save transaction: %w", err)
	}

	for _, line := range tx.Lines {
		if err := s.txRepo.CreateLine(ctx, &line); err != nil {
			return nil, fmt.Errorf("failed to save transaction line: %w", err)
		}
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.transaction", tx.ID, "created", map[string]any{
			"transaction_number": tx.TransactionNumber,
			"from_entity_id":     tx.FromEntityID,
			"to_entity_id":       tx.ToEntityID,
			"amount":             tx.Amount.String(),
			"transaction_type":   tx.TransactionType,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to log posted run action: %w", err)
		}
	}

	return tx, nil
}

func (s *ICTransactionService) SubmitTransaction(ctx context.Context, txID common.ID) error {
	tx, err := s.txRepo.GetByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if err := tx.Submit(); err != nil {
		return err
	}

	if err := s.txRepo.UpdateStatus(ctx, tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.transaction", tx.ID, "submitted", map[string]any{
			"transaction_number": tx.TransactionNumber,
		})
		if err != nil {
			return fmt.Errorf("failed to log posted run action: %w", err)
		}
	}

	return nil
}

func (s *ICTransactionService) PostTransaction(ctx context.Context, txID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	return s.db.WithTransaction(ctx, func(dbTx *sql.Tx) error {

		tx, err := s.txRepo.GetByIDForUpdate(ctx, dbTx, txID)
		if err != nil {
			return fmt.Errorf("transaction not found: %w", err)
		}

		if !tx.Status.CanPost() {
			return fmt.Errorf("transaction cannot be posted, current status: %s", tx.Status)
		}

		mapping, err := s.mappingRepo.GetByEntityPair(ctx, tx.FromEntityID, tx.ToEntityID, tx.TransactionType)
		if err != nil {
			return fmt.Errorf("no account mapping found: %w", err)
		}

		if err := s.glAPI.ValidatePeriodOpen(ctx, tx.FromEntityID, tx.TransactionDate); err != nil {
			return fmt.Errorf("from entity period not open: %w", err)
		}
		if err := s.glAPI.ValidatePeriodOpen(ctx, tx.ToEntityID, tx.TransactionDate); err != nil {
			return fmt.Errorf("to entity period not open: %w", err)
		}

		fromPeriod, err := s.glAPI.GetFiscalPeriodForDate(ctx, tx.FromEntityID, tx.TransactionDate)
		if err != nil {
			return fmt.Errorf("failed to get from entity fiscal period: %w", err)
		}
		toPeriod, err := s.glAPI.GetFiscalPeriodForDate(ctx, tx.ToEntityID, tx.TransactionDate)
		if err != nil {
			return fmt.Errorf("failed to get to entity fiscal period: %w", err)
		}

		tx.SetFiscalPeriods(fromPeriod.ID, toPeriod.ID)

		fromJE, err := s.createFromEntityJournalEntry(ctx, tx, mapping)
		if err != nil {
			return fmt.Errorf("failed to create from entity journal entry: %w", err)
		}

		if err := s.glAPI.PostJournalEntry(ctx, fromJE.ID); err != nil {
			return fmt.Errorf("failed to post from entity journal entry: %w", err)
		}

		toJE, err := s.createToEntityJournalEntry(ctx, tx, mapping)
		if err != nil {
			return fmt.Errorf("failed to create to entity journal entry: %w", err)
		}

		if err := s.glAPI.PostJournalEntry(ctx, toJE.ID); err != nil {
			return fmt.Errorf("failed to post to entity journal entry: %w", err)
		}

		if err := tx.Post(common.ID(userID), fromJE.ID, toJE.ID); err != nil {
			return err
		}

		if err := s.txRepo.WithTx(dbTx).UpdateStatus(ctx, tx); err != nil {
			return fmt.Errorf("failed to update transaction: %w", err)
		}

		if err := s.updateBalances(ctx, dbTx, tx); err != nil {
			return fmt.Errorf("failed to update balances: %w", err)
		}

		if s.auditLogger != nil {
			err = s.auditLogger.LogAction(ctx, "ic.transaction", tx.ID, "posted", map[string]any{
				"transaction_number":    tx.TransactionNumber,
				"from_journal_entry_id": fromJE.ID,
				"to_journal_entry_id":   toJE.ID,
			})
			if err != nil {
				return fmt.Errorf("failed to log posted run action: %w", err)
			}
		}

		return nil
	})
}

func (s *ICTransactionService) createFromEntityJournalEntry(ctx context.Context, tx *domain.ICTransaction, mapping *domain.AccountMapping) (*gl.JournalEntryResponse, error) {
	var lines []gl.JournalLineRequest

	switch tx.TransactionType {
	case domain.TransactionTypeSale, domain.TransactionTypeService, domain.TransactionTypeRecharge:

		if mapping.FromDueFromAccountID == nil {
			return nil, fmt.Errorf("from entity Due From account not configured")
		}
		if mapping.FromRevenueAccountID == nil {
			return nil, fmt.Errorf("from entity Revenue account not configured")
		}

		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromDueFromAccountID,
			Description: fmt.Sprintf("IC Receivable - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromRevenueAccountID,
			Description: fmt.Sprintf("IC Revenue - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	case domain.TransactionTypeLoan:

		if mapping.FromDueFromAccountID == nil {
			return nil, fmt.Errorf("from entity Due From account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromDueFromAccountID,
			Description: fmt.Sprintf("IC Loan Receivable - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})

	case domain.TransactionTypeAllocation:

		if mapping.FromDueFromAccountID == nil {
			return nil, fmt.Errorf("from entity Due From account not configured")
		}
		if mapping.FromExpenseAccountID == nil {
			return nil, fmt.Errorf("from entity Expense account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromDueFromAccountID,
			Description: fmt.Sprintf("IC Allocation Receivable - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromExpenseAccountID,
			Description: fmt.Sprintf("IC Allocation Recovery - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	case domain.TransactionTypeDividend:

		if mapping.FromDueToAccountID == nil {
			return nil, fmt.Errorf("from entity Due To account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromDueToAccountID,
			Description: fmt.Sprintf("IC Dividend Payable - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	case domain.TransactionTypeCapital:

		if mapping.FromDueFromAccountID == nil {
			return nil, fmt.Errorf("from entity Due From account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromDueFromAccountID,
			Description: fmt.Sprintf("IC Capital Receivable - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})

	case domain.TransactionTypeTransfer:

		if mapping.FromDueFromAccountID == nil {
			return nil, fmt.Errorf("from entity Due From account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.FromDueFromAccountID,
			Description: fmt.Sprintf("IC Transfer Receivable - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})

	default:
		return nil, fmt.Errorf("unsupported transaction type: %s", tx.TransactionType)
	}

	req := gl.CreateJournalEntryRequest{
		EntityID:     tx.FromEntityID,
		EntryDate:    tx.TransactionDate,
		Description:  fmt.Sprintf("IC Transaction %s - %s", tx.TransactionNumber, tx.Description),
		CurrencyCode: tx.Currency.Code,
		Lines:        lines,
	}

	return s.glAPI.CreateJournalEntry(ctx, req)
}

func (s *ICTransactionService) createToEntityJournalEntry(ctx context.Context, tx *domain.ICTransaction, mapping *domain.AccountMapping) (*gl.JournalEntryResponse, error) {
	var lines []gl.JournalLineRequest

	switch tx.TransactionType {
	case domain.TransactionTypeSale, domain.TransactionTypeService, domain.TransactionTypeRecharge:

		if mapping.ToExpenseAccountID == nil {
			return nil, fmt.Errorf("to entity Expense account not configured")
		}
		if mapping.ToDueToAccountID == nil {
			return nil, fmt.Errorf("to entity Due To account not configured")
		}

		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToExpenseAccountID,
			Description: fmt.Sprintf("IC Expense - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToDueToAccountID,
			Description: fmt.Sprintf("IC Payable - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	case domain.TransactionTypeLoan:

		if mapping.ToDueToAccountID == nil {
			return nil, fmt.Errorf("to entity Due To account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToDueToAccountID,
			Description: fmt.Sprintf("IC Loan Payable - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	case domain.TransactionTypeAllocation:

		if mapping.ToExpenseAccountID == nil {
			return nil, fmt.Errorf("to entity Expense account not configured")
		}
		if mapping.ToDueToAccountID == nil {
			return nil, fmt.Errorf("to entity Due To account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToExpenseAccountID,
			Description: fmt.Sprintf("IC Allocation Expense - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToDueToAccountID,
			Description: fmt.Sprintf("IC Allocation Payable - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	case domain.TransactionTypeDividend:

		if mapping.ToDueFromAccountID == nil {
			return nil, fmt.Errorf("to entity Due From account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToDueFromAccountID,
			Description: fmt.Sprintf("IC Dividend Receivable - %s", tx.Description),
			Debit:       tx.Amount,
			Credit:      money.Zero(tx.Currency),
		})

	case domain.TransactionTypeCapital:

		if mapping.ToDueToAccountID == nil {
			return nil, fmt.Errorf("to entity Due To account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToDueToAccountID,
			Description: fmt.Sprintf("IC Capital Payable - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	case domain.TransactionTypeTransfer:

		if mapping.ToDueToAccountID == nil {
			return nil, fmt.Errorf("to entity Due To account not configured")
		}
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *mapping.ToDueToAccountID,
			Description: fmt.Sprintf("IC Transfer Payable - %s", tx.Description),
			Debit:       money.Zero(tx.Currency),
			Credit:      tx.Amount,
		})

	default:
		return nil, fmt.Errorf("unsupported transaction type: %s", tx.TransactionType)
	}

	req := gl.CreateJournalEntryRequest{
		EntityID:     tx.ToEntityID,
		EntryDate:    tx.TransactionDate,
		Description:  fmt.Sprintf("IC Transaction %s - %s", tx.TransactionNumber, tx.Description),
		CurrencyCode: tx.Currency.Code,
		Lines:        lines,
	}

	return s.glAPI.CreateJournalEntry(ctx, req)
}

func (s *ICTransactionService) updateBalances(ctx context.Context, dbTx *sql.Tx, tx *domain.ICTransaction) error {
	if tx.FromFiscalPeriodID == nil {
		return fmt.Errorf("from fiscal period not set")
	}

	balance, err := s.balanceRepo.WithTx(dbTx).GetOrCreate(
		ctx,
		tx.FromEntityID,
		tx.ToEntityID,
		*tx.FromFiscalPeriodID,
		tx.Currency.Code,
	)
	if err != nil {
		return fmt.Errorf("failed to get/create balance: %w", err)
	}

	balance.AddDebit(tx.Amount)

	if err := s.balanceRepo.WithTx(dbTx).Update(ctx, balance); err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	return nil
}

func (s *ICTransactionService) ReconcileTransaction(ctx context.Context, txID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	tx, err := s.txRepo.GetByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if err := tx.Reconcile(common.ID(userID)); err != nil {
		return err
	}

	if err := s.txRepo.UpdateStatus(ctx, tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.transaction", tx.ID, "reconciled", map[string]any{
			"transaction_number": tx.TransactionNumber,
		})
		if err != nil {
			return fmt.Errorf("failed to log posted run action: %w", err)
		}
	}

	return nil
}

func (s *ICTransactionService) DisputeTransaction(ctx context.Context, txID common.ID, reason string) error {
	tx, err := s.txRepo.GetByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if err := tx.Dispute(); err != nil {
		return err
	}

	if err := s.txRepo.UpdateStatus(ctx, tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.transaction", tx.ID, "disputed", map[string]any{
			"transaction_number": tx.TransactionNumber,
			"reason":             reason,
		})
		if err != nil {
			return fmt.Errorf("failed to log posted run action: %w", err)
		}
	}

	return nil
}

func (s *ICTransactionService) ResolveDispute(ctx context.Context, txID common.ID) error {
	tx, err := s.txRepo.GetByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if err := tx.ResolveDispute(); err != nil {
		return err
	}

	if err := s.txRepo.UpdateStatus(ctx, tx); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.transaction", tx.ID, "dispute_resolved", map[string]any{
			"transaction_number": tx.TransactionNumber,
		})
		if err != nil {
			return fmt.Errorf("failed to log posted run action: %w", err)
		}
	}

	return nil
}

func (s *ICTransactionService) GetTransaction(ctx context.Context, txID common.ID) (*domain.ICTransaction, error) {
	tx, err := s.txRepo.GetByID(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}

	lines, err := s.txRepo.GetLinesByTransaction(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("failed to load transaction lines: %w", err)
	}
	tx.Lines = lines

	return tx, nil
}

func (s *ICTransactionService) ListTransactions(ctx context.Context, filter domain.ICTransactionFilter) ([]domain.ICTransaction, int64, error) {
	txs, err := s.txRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list transactions: %w", err)
	}

	count, err := s.txRepo.Count(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	return txs, count, nil
}

func (s *ICTransactionService) DeleteTransaction(ctx context.Context, txID common.ID) error {
	tx, err := s.txRepo.GetByID(ctx, txID)
	if err != nil {
		return fmt.Errorf("transaction not found: %w", err)
	}

	if tx.Status != domain.TransactionStatusDraft {
		return fmt.Errorf("can only delete draft transactions")
	}

	if err := s.txRepo.DeleteLines(ctx, txID); err != nil {
		return fmt.Errorf("failed to delete transaction lines: %w", err)
	}

	if err := s.txRepo.Delete(ctx, txID); err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "ic.transaction", tx.ID, "deleted", map[string]any{
			"transaction_number": tx.TransactionNumber,
		})
		if err != nil {
			return fmt.Errorf("failed to log posted run action: %w", err)
		}
	}

	return nil
}
