package service

import (
	"context"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
	"converge-finance.com/m/internal/modules/cost/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
)

type BudgetService struct {
	db           *database.PostgresDB
	budgetRepo   repository.BudgetRepository
	lineRepo     repository.BudgetLineRepository
	transferRepo repository.BudgetTransferRepository
	actualRepo   repository.BudgetActualRepository
	glAPI        gl.API
	auditLogger  *audit.Logger
}

func NewBudgetService(
	db *database.PostgresDB,
	budgetRepo repository.BudgetRepository,
	lineRepo repository.BudgetLineRepository,
	transferRepo repository.BudgetTransferRepository,
	actualRepo repository.BudgetActualRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *BudgetService {
	return &BudgetService{
		db:           db,
		budgetRepo:   budgetRepo,
		lineRepo:     lineRepo,
		transferRepo: transferRepo,
		actualRepo:   actualRepo,
		glAPI:        glAPI,
		auditLogger:  auditLogger,
	}
}

func (s *BudgetService) CreateBudget(
	ctx context.Context,
	entityID common.ID,
	budgetCode string,
	budgetName string,
	budgetType domain.BudgetType,
	fiscalYearID common.ID,
	currency money.Currency,
) (*domain.Budget, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	budget, err := domain.NewBudget(entityID, budgetCode, budgetName, budgetType, fiscalYearID, currency, common.ID(userID))
	if err != nil {
		return nil, fmt.Errorf("invalid budget: %w", err)
	}

	if err := s.budgetRepo.Create(ctx, budget); err != nil {
		return nil, fmt.Errorf("failed to save budget: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget", budget.ID, "created", map[string]any{
			"budget_code": budget.BudgetCode,
			"budget_type": budget.BudgetType,
		})
	}

	return budget, nil
}

func (s *BudgetService) AddBudgetLine(
	ctx context.Context,
	budgetID common.ID,
	accountID common.ID,
	fiscalPeriodID common.ID,
	costCenterID *common.ID,
	budgetAmount money.Money,
	quantity *float64,
	unitCost *float64,
	notes string,
) (*domain.BudgetLine, error) {
	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return nil, fmt.Errorf("budget not found: %w", err)
	}

	if !budget.CanModify() {
		return nil, fmt.Errorf("budget cannot be modified in current status")
	}

	line := domain.NewBudgetLine(accountID, fiscalPeriodID, budgetAmount)
	line.BudgetID = budgetID
	line.Notes = notes

	if costCenterID != nil {
		line.SetCostCenter(*costCenterID)
	}

	if quantity != nil && unitCost != nil {
		line.SetQuantityAndUnitCost(*quantity, *unitCost)
	}

	if err := s.lineRepo.Create(ctx, line); err != nil {
		return nil, fmt.Errorf("failed to save budget line: %w", err)
	}

	return line, nil
}

func (s *BudgetService) UpdateBudgetLine(
	ctx context.Context,
	lineID common.ID,
	budgetAmount *money.Money,
	quantity *float64,
	unitCost *float64,
	notes *string,
) (*domain.BudgetLine, error) {
	line, err := s.lineRepo.GetByID(ctx, lineID)
	if err != nil {
		return nil, fmt.Errorf("budget line not found: %w", err)
	}

	budget, err := s.budgetRepo.GetByID(ctx, line.BudgetID)
	if err != nil {
		return nil, fmt.Errorf("budget not found: %w", err)
	}

	if !budget.CanModify() {
		return nil, fmt.Errorf("budget cannot be modified in current status")
	}

	if budgetAmount != nil {
		line.BudgetAmount = *budgetAmount
	}
	if quantity != nil && unitCost != nil {
		line.SetQuantityAndUnitCost(*quantity, *unitCost)
	}
	if notes != nil {
		line.Notes = *notes
	}

	if err := s.lineRepo.Update(ctx, line); err != nil {
		return nil, fmt.Errorf("failed to update budget line: %w", err)
	}

	return line, nil
}

func (s *BudgetService) SubmitBudget(ctx context.Context, budgetID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	if err := budget.Submit(common.ID(userID)); err != nil {
		return err
	}

	if err := s.budgetRepo.Update(ctx, budget); err != nil {
		return fmt.Errorf("failed to update budget: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget", budgetID, "submitted", nil)
	}

	return nil
}

func (s *BudgetService) ApproveBudget(ctx context.Context, budgetID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	if err := budget.Approve(common.ID(userID)); err != nil {
		return err
	}

	if err := s.budgetRepo.Update(ctx, budget); err != nil {
		return fmt.Errorf("failed to update budget: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget", budgetID, "approved", nil)
	}

	return nil
}

func (s *BudgetService) RejectBudget(ctx context.Context, budgetID common.ID, reason string) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	if err := budget.Reject(common.ID(userID), reason); err != nil {
		return err
	}

	if err := s.budgetRepo.Update(ctx, budget); err != nil {
		return fmt.Errorf("failed to update budget: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget", budgetID, "rejected", map[string]any{
			"reason": reason,
		})
	}

	return nil
}

func (s *BudgetService) ActivateBudget(ctx context.Context, budgetID common.ID) error {
	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	if err := budget.Activate(); err != nil {
		return err
	}

	if err := s.budgetRepo.Update(ctx, budget); err != nil {
		return fmt.Errorf("failed to update budget: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget", budgetID, "activated", nil)
	}

	return nil
}

func (s *BudgetService) CreateBudgetVersion(ctx context.Context, budgetID common.ID) (*domain.Budget, error) {
	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return nil, fmt.Errorf("budget not found: %w", err)
	}

	newVersion := budget.CreateNewVersion()

	if err := s.budgetRepo.Create(ctx, newVersion); err != nil {
		return nil, fmt.Errorf("failed to create new version: %w", err)
	}

	if err := s.budgetRepo.Update(ctx, budget); err != nil {
		return nil, fmt.Errorf("failed to update old version: %w", err)
	}

	if err := s.lineRepo.CopyFromBudget(ctx, budget.ID, newVersion.ID); err != nil {
		return nil, fmt.Errorf("failed to copy budget lines: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget", newVersion.ID, "version_created", map[string]any{
			"parent_version_id": budget.ID,
			"version_number":    newVersion.VersionNumber,
		})
	}

	return newVersion, nil
}

func (s *BudgetService) GetBudget(ctx context.Context, id common.ID) (*domain.Budget, error) {
	budget, err := s.budgetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	lines, err := s.lineRepo.GetByBudget(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get budget lines: %w", err)
	}
	budget.Lines = lines

	return budget, nil
}

func (s *BudgetService) ListBudgets(ctx context.Context, filter domain.BudgetFilter) ([]domain.Budget, error) {
	return s.budgetRepo.List(ctx, filter)
}

func (s *BudgetService) GetBudgetLines(ctx context.Context, budgetID common.ID) ([]domain.BudgetLine, error) {
	return s.lineRepo.GetByBudget(ctx, budgetID)
}

func (s *BudgetService) GetVarianceAnalysis(
	ctx context.Context,
	budgetID common.ID,
	fiscalPeriodID common.ID,
) (*domain.VarianceSummary, []domain.VarianceAnalysis, error) {
	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return nil, nil, fmt.Errorf("budget not found: %w", err)
	}

	budgetLines, err := s.lineRepo.GetByBudgetAndPeriod(ctx, budgetID, fiscalPeriodID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get budget lines: %w", err)
	}

	actuals, err := s.actualRepo.GetByPeriod(ctx, budget.EntityID, fiscalPeriodID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get actuals: %w", err)
	}

	actualMap := make(map[string]domain.BudgetActual)
	for _, actual := range actuals {
		key := string(actual.AccountID)
		if actual.CostCenterID != nil {
			key += "-" + string(*actual.CostCenterID)
		}
		actualMap[key] = actual
	}

	var analyses []domain.VarianceAnalysis
	for _, line := range budgetLines {
		key := string(line.AccountID)
		if line.CostCenterID != nil {
			key += "-" + string(*line.CostCenterID)
		}

		actualAmount := money.Zero(budget.Currency)
		if actual, ok := actualMap[key]; ok {
			actualAmount = actual.ActualAmount
		}

		accountType := "expense"

		analysis := domain.NewVarianceAnalysis(
			line.AccountID,
			line.AccountCode,
			line.AccountName,
			accountType,
			fiscalPeriodID,
			line.BudgetAmount,
			actualAmount,
		)

		if line.CostCenterID != nil {
			analysis.SetCostCenter(*line.CostCenterID, line.CostCenterCode, line.CostCenterName)
		}

		analyses = append(analyses, *analysis)
	}

	summary := domain.NewVarianceSummary(budget.EntityID, fiscalPeriodID, budgetID, budget.Currency)
	summary.BudgetName = budget.BudgetName
	summary.Calculate(analyses)

	return summary, analyses, nil
}

func (s *BudgetService) RefreshActuals(ctx context.Context, entityID, fiscalPeriodID common.ID) error {
	if err := s.actualRepo.RefreshFromGL(ctx, entityID, fiscalPeriodID); err != nil {
		return fmt.Errorf("failed to refresh actuals: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.actuals", common.NewID(), "refreshed", map[string]any{
			"entity_id":        entityID,
			"fiscal_period_id": fiscalPeriodID,
		})
	}

	return nil
}

func (s *BudgetService) RequestBudgetTransfer(
	ctx context.Context,
	budgetID common.ID,
	fromAccountID common.ID,
	fromCostCenterID *common.ID,
	fromPeriodID common.ID,
	toAccountID common.ID,
	toCostCenterID *common.ID,
	toPeriodID common.ID,
	transferAmount money.Money,
	reason string,
) (*domain.BudgetTransfer, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	budget, err := s.budgetRepo.GetByID(ctx, budgetID)
	if err != nil {
		return nil, fmt.Errorf("budget not found: %w", err)
	}

	if budget.Status != domain.BudgetStatusActive {
		return nil, fmt.Errorf("budget transfers only allowed on active budgets")
	}

	transferNumber, err := s.transferRepo.GetNextTransferNumber(ctx, budgetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transfer number: %w", err)
	}

	transfer, err := domain.NewBudgetTransfer(
		budgetID,
		transferNumber,
		budget.CreatedAt,
		fromAccountID,
		fromPeriodID,
		toAccountID,
		toPeriodID,
		transferAmount,
		reason,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid transfer: %w", err)
	}

	transfer.FromCostCenterID = fromCostCenterID
	transfer.ToCostCenterID = toCostCenterID

	if err := s.transferRepo.Create(ctx, transfer); err != nil {
		return nil, fmt.Errorf("failed to save transfer: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget_transfer", transfer.ID, "requested", map[string]any{
			"budget_id":       budgetID,
			"transfer_amount": transferAmount,
		})
	}

	return transfer, nil
}

func (s *BudgetService) ApproveBudgetTransfer(ctx context.Context, transferID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	transfer, err := s.transferRepo.GetByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("transfer not found: %w", err)
	}

	if transfer.IsApproved {
		return fmt.Errorf("transfer already approved")
	}

	fromLine, err := s.lineRepo.GetByBudgetAccountAndPeriod(
		ctx, transfer.BudgetID, transfer.FromAccountID, transfer.FromPeriodID, transfer.FromCostCenterID,
	)
	if err != nil {
		return fmt.Errorf("source budget line not found: %w", err)
	}

	if fromLine.BudgetAmount.LessThan(transfer.TransferAmount) {
		return fmt.Errorf("insufficient budget in source line")
	}

	toLine, err := s.lineRepo.GetByBudgetAccountAndPeriod(
		ctx, transfer.BudgetID, transfer.ToAccountID, transfer.ToPeriodID, transfer.ToCostCenterID,
	)
	if err != nil {
		toLine = domain.NewBudgetLine(transfer.ToAccountID, transfer.ToPeriodID, money.Zero(transfer.TransferAmount.Currency))
		toLine.BudgetID = transfer.BudgetID
		toLine.CostCenterID = transfer.ToCostCenterID
		if err := s.lineRepo.Create(ctx, toLine); err != nil {
			return fmt.Errorf("failed to create target line: %w", err)
		}
	}

	fromLine.BudgetAmount = fromLine.BudgetAmount.MustSubtract(transfer.TransferAmount)
	toLine.BudgetAmount = toLine.BudgetAmount.MustAdd(transfer.TransferAmount)

	if err := s.lineRepo.Update(ctx, fromLine); err != nil {
		return fmt.Errorf("failed to update source line: %w", err)
	}
	if err := s.lineRepo.Update(ctx, toLine); err != nil {
		return fmt.Errorf("failed to update target line: %w", err)
	}

	transfer.Approve(common.ID(userID))

	if err := s.transferRepo.Update(ctx, transfer); err != nil {
		return fmt.Errorf("failed to update transfer: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "cost.budget_transfer", transferID, "approved", nil)
	}

	return nil
}
