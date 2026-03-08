package ic

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
	"converge-finance.com/m/internal/modules/ic/internal/repository"
	"converge-finance.com/m/internal/modules/ic/internal/service"
)

type icAPI struct {
	hierarchyRepo repository.EntityHierarchyRepository
	balanceRepo   repository.BalanceRepository
	txService     *service.ICTransactionService
	reconcService *service.ReconciliationService
	elimService   *service.EliminationService
}

func NewICAPI(
	hierarchyRepo repository.EntityHierarchyRepository,
	balanceRepo repository.BalanceRepository,
	txService *service.ICTransactionService,
	reconcService *service.ReconciliationService,
	elimService *service.EliminationService,
) API {
	return &icAPI{
		hierarchyRepo: hierarchyRepo,
		balanceRepo:   balanceRepo,
		txService:     txService,
		reconcService: reconcService,
		elimService:   elimService,
	}
}

func (a *icAPI) GetEntityHierarchy(ctx context.Context, rootID common.ID) (*EntityHierarchyResponse, error) {
	tree, err := a.hierarchyRepo.GetHierarchyTree(ctx, rootID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity hierarchy: %w", err)
	}
	resp := toEntityHierarchyResponse(tree)
	return &resp, nil
}

func (a *icAPI) GetChildEntities(ctx context.Context, parentID common.ID) ([]EntityHierarchyResponse, error) {
	children, err := a.hierarchyRepo.GetChildren(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get child entities: %w", err)
	}

	responses := make([]EntityHierarchyResponse, len(children))
	for i, child := range children {
		responses[i] = toEntityHierarchyResponse(&child)
	}
	return responses, nil
}

func (a *icAPI) RecordIntercompanyActivity(ctx context.Context, req RecordICActivityRequest) (*ICTransactionResponse, error) {
	createReq := service.CreateTransactionRequest{
		FromEntityID:    req.FromEntityID,
		ToEntityID:      req.ToEntityID,
		TransactionType: domain.TransactionType(req.TransactionType),
		TransactionDate: req.TransactionDate,
		Amount:          req.Amount,
		Description:     req.Description,
		Reference:       req.Reference,
	}

	tx, err := a.txService.CreateTransaction(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create IC transaction: %w", err)
	}

	if req.AutoPost {
		if err := a.txService.PostTransaction(ctx, tx.ID); err != nil {
			return nil, fmt.Errorf("failed to post IC transaction: %w", err)
		}
		// Refresh to get updated status
		tx, err = a.txService.GetTransaction(ctx, tx.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get updated transaction: %w", err)
		}
	}

	return toICTransactionResponse(tx), nil
}

func (a *icAPI) GetTransactionByID(ctx context.Context, txID common.ID) (*ICTransactionResponse, error) {
	tx, err := a.txService.GetTransaction(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("transaction not found: %w", err)
	}
	return toICTransactionResponse(tx), nil
}

func (a *icAPI) ListTransactions(ctx context.Context, entityID common.ID, filter TransactionFilterRequest) ([]ICTransactionResponse, error) {
	domainFilter := domain.ICTransactionFilter{
		EntityID:       &entityID,
		FromEntityID:   filter.FromEntityID,
		ToEntityID:     filter.ToEntityID,
		FiscalPeriodID: filter.FiscalPeriodID,
		DateFrom:       filter.DateFrom,
		DateTo:         filter.DateTo,
		Limit:          filter.Limit,
		Offset:         filter.Offset,
	}

	if filter.TransactionType != nil {
		tt := domain.TransactionType(*filter.TransactionType)
		domainFilter.TransactionType = &tt
	}

	if filter.Status != nil {
		st := domain.TransactionStatus(*filter.Status)
		domainFilter.Status = &st
	}

	txs, _, err := a.txService.ListTransactions(ctx, domainFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	responses := make([]ICTransactionResponse, len(txs))
	for i, tx := range txs {
		responses[i] = *toICTransactionResponse(&tx)
	}
	return responses, nil
}

func (a *icAPI) GetReconciliationStatus(ctx context.Context, parentEntityID, fiscalPeriodID common.ID) (*ReconciliationStatusResponse, error) {
	summary, err := a.reconcService.GetReconciliationStatus(ctx, parentEntityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reconciliation status: %w", err)
	}

	return &ReconciliationStatusResponse{
		ParentEntityID:    summary.ParentEntityID,
		FiscalPeriodID:    summary.FiscalPeriodID,
		TotalEntityPairs:  summary.TotalEntityPairs,
		ReconciledPairs:   summary.ReconciledPairs,
		UnreconciledPairs: summary.UnreconciledPairs,
		DisputedPairs:     summary.DisputedPairs,
		TotalDiscrepancy:  summary.TotalDiscrepancy,
		ReconciliationPct: summary.GetReconciliationPercentage().InexactFloat64(),
	}, nil
}

func (a *icAPI) GetEntityPairBalance(ctx context.Context, fromEntityID, toEntityID, fiscalPeriodID common.ID) (*EntityPairBalanceResponse, error) {
	balance, err := a.balanceRepo.GetByEntityPair(ctx, fromEntityID, toEntityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("balance not found: %w", err)
	}

	return &EntityPairBalanceResponse{
		FromEntityID:   balance.FromEntityID,
		ToEntityID:     balance.ToEntityID,
		FiscalPeriodID: balance.FiscalPeriodID,
		Currency:       balance.Currency.Code,
		OpeningBalance: balance.OpeningBalance,
		PeriodDebits:   balance.PeriodDebits,
		PeriodCredits:  balance.PeriodCredits,
		ClosingBalance: balance.ClosingBalance,
		IsReconciled:   balance.IsReconciled,
		Discrepancy:    balance.DiscrepancyAmount,
	}, nil
}

func (a *icAPI) GenerateEliminations(ctx context.Context, parentEntityID, fiscalPeriodID common.ID, eliminationDate time.Time, currency money.Currency) (*EliminationRunResponse, error) {
	req := service.GenerateEliminationsRequest{
		ParentEntityID:  parentEntityID,
		FiscalPeriodID:  fiscalPeriodID,
		EliminationDate: eliminationDate,
		CurrencyCode:    currency.Code,
	}

	run, err := a.elimService.GenerateEliminations(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate eliminations: %w", err)
	}

	return toEliminationRunResponse(run), nil
}

func (a *icAPI) PostEliminationRun(ctx context.Context, runID common.ID) error {
	if err := a.elimService.PostEliminationRun(ctx, runID); err != nil {
		return fmt.Errorf("failed to post elimination run: %w", err)
	}
	return nil
}

func (a *icAPI) GetEliminationRun(ctx context.Context, runID common.ID) (*EliminationRunResponse, error) {
	run, err := a.elimService.GetEliminationRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("elimination run not found: %w", err)
	}
	return toEliminationRunResponse(run), nil
}

// Helper functions

func toEntityHierarchyResponse(e *domain.EntityHierarchy) EntityHierarchyResponse {
	resp := EntityHierarchyResponse{
		ID:                  e.ID,
		Code:                e.Code,
		Name:                e.Name,
		BaseCurrency:        e.BaseCurrency,
		IsActive:            e.IsActive,
		ParentID:            e.ParentID,
		EntityType:          string(e.EntityType),
		OwnershipPercent:    e.OwnershipPercent.InexactFloat64(),
		ConsolidationMethod: string(e.ConsolidationMethod),
		HierarchyLevel:      e.HierarchyLevel,
		HierarchyPath:       e.HierarchyPath,
		Children:            make([]EntityHierarchyResponse, 0),
	}
	for _, child := range e.Children {
		resp.Children = append(resp.Children, toEntityHierarchyResponse(child))
	}
	return resp
}

func toICTransactionResponse(tx *domain.ICTransaction) *ICTransactionResponse {
	return &ICTransactionResponse{
		ID:                 tx.ID,
		TransactionNumber:  tx.TransactionNumber,
		TransactionType:    string(tx.TransactionType),
		FromEntityID:       tx.FromEntityID,
		ToEntityID:         tx.ToEntityID,
		TransactionDate:    tx.TransactionDate,
		DueDate:            tx.DueDate,
		Amount:             tx.Amount,
		Currency:           tx.Currency.Code,
		Description:        tx.Description,
		Reference:          tx.Reference,
		Status:             string(tx.Status),
		FromJournalEntryID: tx.FromJournalEntryID,
		ToJournalEntryID:   tx.ToJournalEntryID,
		CreatedAt:          tx.CreatedAt,
		PostedAt:           tx.PostedAt,
		ReconciledAt:       tx.ReconciledAt,
	}
}

func toEliminationRunResponse(run *domain.EliminationRun) *EliminationRunResponse {
	return &EliminationRunResponse{
		ID:               run.ID,
		RunNumber:        run.RunNumber,
		ParentEntityID:   run.ParentEntityID,
		FiscalPeriodID:   run.FiscalPeriodID,
		EliminationDate:  run.EliminationDate,
		Currency:         run.Currency.Code,
		EntryCount:       run.EntryCount,
		TotalElimination: run.TotalEliminations,
		Status:           string(run.Status),
		JournalEntryID:   run.JournalEntryID,
		CreatedAt:        run.CreatedAt,
		PostedAt:         run.PostedAt,
		ReversedAt:       run.ReversedAt,
	}
}
