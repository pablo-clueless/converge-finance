package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
	"converge-finance.com/m/internal/modules/ic/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
)

type ReconciliationService struct {
	txRepo        repository.TransactionRepository
	balanceRepo   repository.BalanceRepository
	hierarchyRepo repository.EntityHierarchyRepository
	auditLogger   *audit.Logger
}

func NewReconciliationService(
	txRepo repository.TransactionRepository,
	balanceRepo repository.BalanceRepository,
	hierarchyRepo repository.EntityHierarchyRepository,
	auditLogger *audit.Logger,
) *ReconciliationService {
	return &ReconciliationService{
		txRepo:        txRepo,
		balanceRepo:   balanceRepo,
		hierarchyRepo: hierarchyRepo,
		auditLogger:   auditLogger,
	}
}

func (s *ReconciliationService) GetReconciliationStatus(
	ctx context.Context,
	parentEntityID common.ID,
	fiscalPeriodID common.ID,
) (*domain.ReconciliationSummary, error) {
	summary, err := s.balanceRepo.GetReconciliationSummary(ctx, parentEntityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation summary: %w", err)
	}

	return summary, nil
}

func (s *ReconciliationService) GetEntityPairReconciliation(
	ctx context.Context,
	fromEntityID common.ID,
	toEntityID common.ID,
	fiscalPeriodID common.ID,
) (*domain.EntityPairReconciliation, error) {

	fromEntity, err := s.hierarchyRepo.GetByID(ctx, fromEntityID)
	if err != nil {
		return nil, fmt.Errorf("from entity not found: %w", err)
	}

	toEntity, err := s.hierarchyRepo.GetByID(ctx, toEntityID)
	if err != nil {
		return nil, fmt.Errorf("to entity not found: %w", err)
	}

	fromBalance, err := s.balanceRepo.GetByEntityPair(ctx, fromEntityID, toEntityID, fiscalPeriodID)
	if err != nil {

		fromBalance = domain.NewEntityPairBalance(fromEntityID, toEntityID, fiscalPeriodID, money.USD)
	}

	toBalance, err := s.balanceRepo.GetByEntityPair(ctx, toEntityID, fromEntityID, fiscalPeriodID)
	if err != nil {

		toBalance = domain.NewEntityPairBalance(toEntityID, fromEntityID, fiscalPeriodID, money.USD)
	}

	fromTxs, err := s.txRepo.GetByEntityPair(ctx, fromEntityID, toEntityID, &fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get from entity transactions: %w", err)
	}

	toTxs, err := s.txRepo.GetByEntityPair(ctx, toEntityID, fromEntityID, &fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get to entity transactions: %w", err)
	}

	discrepancy := fromBalance.ClosingBalance.MustAdd(toBalance.ClosingBalance)

	reconciliation := &domain.EntityPairReconciliation{
		FromEntityID:       fromEntityID,
		FromEntityCode:     fromEntity.Code,
		FromEntityName:     fromEntity.Name,
		ToEntityID:         toEntityID,
		ToEntityCode:       toEntity.Code,
		ToEntityName:       toEntity.Name,
		FiscalPeriodID:     fiscalPeriodID,
		FromOpeningBalance: fromBalance.OpeningBalance,
		ToOpeningBalance:   toBalance.OpeningBalance,
		FromClosingBalance: fromBalance.ClosingBalance,
		ToClosingBalance:   toBalance.ClosingBalance,
		DiscrepancyAmount:  discrepancy,
		IsReconciled:       discrepancy.IsZero(),
		FromTransactions:   fromTxs,
		ToTransactions:     toTxs,
	}

	reconciliation.MatchedTransactions, reconciliation.UnmatchedFromTransactions, reconciliation.UnmatchedToTransactions = s.matchTransactions(fromTxs, toTxs)

	return reconciliation, nil
}

func (s *ReconciliationService) matchTransactions(
	fromTxs []domain.ICTransaction,
	toTxs []domain.ICTransaction,
) ([]domain.TransactionMatch, []domain.ICTransaction, []domain.ICTransaction) {
	var matched []domain.TransactionMatch
	unmatchedFrom := make([]domain.ICTransaction, 0)
	toMatched := make(map[common.ID]bool)

	for _, fromTx := range fromTxs {
		found := false

		for _, toTx := range toTxs {
			if toMatched[toTx.ID] {
				continue
			}

			if fromTx.Amount.Equals(toTx.Amount) && fromTx.TransactionDate.Equal(toTx.TransactionDate) {
				matched = append(matched, domain.TransactionMatch{
					FromTransaction: fromTx,
					ToTransaction:   toTx,
					MatchType:       domain.MatchTypeExact,
					Difference:      money.Zero(fromTx.Currency),
				})
				toMatched[toTx.ID] = true
				found = true
				break
			}

			if fromTx.Amount.Equals(toTx.Amount) {
				matched = append(matched, domain.TransactionMatch{
					FromTransaction: fromTx,
					ToTransaction:   toTx,
					MatchType:       domain.MatchTypeAmount,
					Difference:      money.Zero(fromTx.Currency),
				})
				toMatched[toTx.ID] = true
				found = true
				break
			}

			if fromTx.Reference != "" && fromTx.Reference == toTx.Reference {
				diff := fromTx.Amount.MustSubtract(toTx.Amount)
				matched = append(matched, domain.TransactionMatch{
					FromTransaction: fromTx,
					ToTransaction:   toTx,
					MatchType:       domain.MatchTypeReference,
					Difference:      diff,
				})
				toMatched[toTx.ID] = true
				found = true
				break
			}
		}

		if !found {
			unmatchedFrom = append(unmatchedFrom, fromTx)
		}
	}

	unmatchedTo := make([]domain.ICTransaction, 0)
	for _, toTx := range toTxs {
		if !toMatched[toTx.ID] {
			unmatchedTo = append(unmatchedTo, toTx)
		}
	}

	return matched, unmatchedFrom, unmatchedTo
}

func (s *ReconciliationService) AutoReconcile(
	ctx context.Context,
	fromEntityID common.ID,
	toEntityID common.ID,
	fiscalPeriodID common.ID,
) (int, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return 0, fmt.Errorf("user not authenticated")
	}

	fromTxs, err := s.txRepo.GetUnreconciled(ctx, fromEntityID, toEntityID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unreconciled transactions: %w", err)
	}

	toTxs, err := s.txRepo.GetUnreconciled(ctx, toEntityID, fromEntityID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unreconciled transactions: %w", err)
	}

	matches, _, _ := s.matchTransactions(fromTxs, toTxs)

	reconciled := 0
	for _, match := range matches {

		if match.MatchType != domain.MatchTypeExact {
			continue
		}

		if match.FromTransaction.Status == domain.TransactionStatusPosted {
			fromTx := match.FromTransaction
			if err := fromTx.Reconcile(common.ID(userID)); err == nil {
				if err := s.txRepo.UpdateStatus(ctx, &fromTx); err == nil {
					reconciled++
				}
			}
		}

		if match.ToTransaction.Status == domain.TransactionStatusPosted {
			toTx := match.ToTransaction
			if err := toTx.Reconcile(common.ID(userID)); err == nil {
				if err := s.txRepo.UpdateStatus(ctx, &toTx); err == nil {
					reconciled++
				}
			}
		}
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "ic.reconciliation", common.NewID(), "auto_reconcile", map[string]any{
			"from_entity_id":   fromEntityID,
			"to_entity_id":     toEntityID,
			"fiscal_period_id": fiscalPeriodID,
			"reconciled_count": reconciled,
		})
	}

	return reconciled, nil
}

func (s *ReconciliationService) GetDiscrepancies(
	ctx context.Context,
	parentEntityID common.ID,
	fiscalPeriodID common.ID,
) ([]domain.ReconciliationDiscrepancy, error) {
	discrepancies, err := s.balanceRepo.GetDiscrepancies(ctx, parentEntityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get discrepancies: %w", err)
	}

	return discrepancies, nil
}

func (s *ReconciliationService) RecalculateBalances(
	ctx context.Context,
	fiscalPeriodID common.ID,
) error {
	if err := s.balanceRepo.RecalculateAllBalances(ctx, fiscalPeriodID); err != nil {
		return fmt.Errorf("failed to recalculate balances: %w", err)
	}

	return nil
}

func (s *ReconciliationService) GetUnreconciledTransactions(
	ctx context.Context,
	entityID common.ID,
) ([]domain.ICTransaction, error) {

	filter := domain.ICTransactionFilter{
		EntityID: &entityID,
		Status:   func() *domain.TransactionStatus { s := domain.TransactionStatusPosted; return &s }(),
	}

	txs, err := s.txRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	return txs, nil
}

func (s *ReconciliationService) GetBalanceHistory(
	ctx context.Context,
	fromEntityID common.ID,
	toEntityID common.ID,
	limit int,
) ([]domain.EntityPairBalance, error) {
	filter := domain.EntityPairBalanceFilter{
		FromEntityID: &fromEntityID,
		ToEntityID:   &toEntityID,
		Limit:        limit,
	}

	balances, err := s.balanceRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance history: %w", err)
	}

	return balances, nil
}

func (s *ReconciliationService) MarkBalanceReconciled(
	ctx context.Context,
	balanceID common.ID,
) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	if err := s.balanceRepo.MarkReconciled(ctx, balanceID); err != nil {
		return fmt.Errorf("failed to mark balance as reconciled: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "ic.balance", balanceID, "reconciled", map[string]any{
			"reconciled_at": time.Now(),
		})
	}

	return nil
}
