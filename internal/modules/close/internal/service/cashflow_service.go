package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
	"github.com/shopspring/decimal"
)

type CashFlowService struct {
	db           *database.PostgresDB
	configRepo   repository.AccountCashFlowConfigRepository
	templateRepo repository.CashFlowTemplateRepository
	runRepo      repository.CashFlowRunRepository
	lineRepo     repository.CashFlowLineRepository
	glAPI        gl.API
	auditLogger  *audit.Logger
}

func NewCashFlowService(
	db *database.PostgresDB,
	configRepo repository.AccountCashFlowConfigRepository,
	templateRepo repository.CashFlowTemplateRepository,
	runRepo repository.CashFlowRunRepository,
	lineRepo repository.CashFlowLineRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *CashFlowService {
	return &CashFlowService{
		db:           db,
		configRepo:   configRepo,
		templateRepo: templateRepo,
		runRepo:      runRepo,
		lineRepo:     lineRepo,
		glAPI:        glAPI,
		auditLogger:  auditLogger,
	}
}

// ConfigureAccountCashFlow creates or updates account cash flow configuration
func (s *CashFlowService) ConfigureAccountCashFlow(
	ctx context.Context,
	entityID, accountID common.ID,
	category domain.CashFlowCategory,
	lineItemCode string,
	isCashAccount, isCashEquivalent bool,
	adjustmentType string,
) (*domain.AccountCashFlowConfig, error) {
	// Check if config exists
	existing, err := s.configRepo.GetByAccountID(ctx, entityID, accountID)
	if err == nil && existing != nil {
		// Update existing config
		existing.CashFlowCategory = category
		existing.LineItemCode = lineItemCode
		existing.SetCashAccount(isCashAccount, isCashEquivalent)
		if adjustmentType != "" {
			existing.SetAdjustmentType(adjustmentType)
		}

		if err := s.configRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update account cash flow config: %w", err)
		}

		s.auditLogger.Log(ctx, "account_cashflow_config", existing.ID, "update", map[string]any{
			"account_id":   accountID,
			"category":     category,
			"is_cash":      isCashAccount,
		})

		return existing, nil
	}

	// Create new config
	config := domain.NewAccountCashFlowConfig(entityID, accountID, category, lineItemCode)
	config.SetCashAccount(isCashAccount, isCashEquivalent)
	if adjustmentType != "" {
		config.SetAdjustmentType(adjustmentType)
	}

	if err := s.configRepo.Create(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create account cash flow config: %w", err)
	}

	s.auditLogger.Log(ctx, "account_cashflow_config", config.ID, "create", map[string]any{
		"account_id":   accountID,
		"category":     category,
		"is_cash":      isCashAccount,
	})

	return config, nil
}

// GetAccountCashFlowConfig gets the cash flow config for an account
func (s *CashFlowService) GetAccountCashFlowConfig(ctx context.Context, entityID, accountID common.ID) (*domain.AccountCashFlowConfig, error) {
	return s.configRepo.GetByAccountID(ctx, entityID, accountID)
}

// ListAccountCashFlowConfigs lists all cash flow configs for an entity
func (s *CashFlowService) ListAccountCashFlowConfigs(ctx context.Context, entityID common.ID) ([]domain.AccountCashFlowConfig, error) {
	return s.configRepo.ListByEntity(ctx, entityID)
}

// CreateTemplate creates a new cash flow template
func (s *CashFlowService) CreateTemplate(
	ctx context.Context,
	entityID common.ID,
	templateCode, templateName string,
	method domain.CashFlowMethod,
) (*domain.CashFlowTemplate, error) {
	template := domain.NewCashFlowTemplate(entityID, templateCode, templateName, method)

	if err := s.templateRepo.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to create cash flow template: %w", err)
	}

	s.auditLogger.Log(ctx, "cashflow_template", template.ID, "create", map[string]any{
		"template_code": templateCode,
		"method":        method,
	})

	return template, nil
}

// GetTemplate gets a cash flow template by ID
func (s *CashFlowService) GetTemplate(ctx context.Context, id common.ID) (*domain.CashFlowTemplate, error) {
	return s.templateRepo.GetByID(ctx, id)
}

// ListTemplates lists cash flow templates for an entity
func (s *CashFlowService) ListTemplates(ctx context.Context, entityID common.ID) ([]domain.CashFlowTemplate, error) {
	return s.templateRepo.ListByEntity(ctx, entityID)
}

// GenerateCashFlowStatement generates a cash flow statement
func (s *CashFlowService) GenerateCashFlowStatement(
	ctx context.Context,
	entityID common.ID,
	fiscalPeriodID, fiscalYearID common.ID,
	method domain.CashFlowMethod,
	periodStart, periodEnd time.Time,
	currencyCode string,
	userID common.ID,
) (*domain.CashFlowRun, error) {
	// Generate run number
	runNumber, err := s.runRepo.GenerateRunNumber(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate run number: %w", err)
	}

	// Create the run
	run := domain.NewCashFlowRun(
		entityID,
		runNumber,
		fiscalPeriodID,
		fiscalYearID,
		method,
		periodStart,
		periodEnd,
		currencyCode,
		userID,
	)

	run.StartGeneration()

	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create cash flow run: %w", err)
	}

	// Get cash accounts to calculate opening and closing balances
	cashConfigs, err := s.configRepo.ListCashAccounts(ctx, entityID)
	if err != nil {
		run.Fail()
		_ = s.runRepo.Update(ctx, run)
		return run, fmt.Errorf("failed to get cash accounts: %w", err)
	}

	// Calculate opening and closing cash balances
	openingCash := decimal.Zero
	closingCash := decimal.Zero

	for _, config := range cashConfigs {
		balance, err := s.glAPI.GetAccountBalance(ctx, config.AccountID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		// Opening balance
		openingDebit := balance.OpeningDebit.Amount
		openingCredit := balance.OpeningCredit.Amount
		openingCash = openingCash.Add(openingDebit.Sub(openingCredit))

		// Closing balance
		closingDebit := balance.ClosingDebit.Amount
		closingCredit := balance.ClosingCredit.Amount
		closingCash = closingCash.Add(closingDebit.Sub(closingCredit))
	}

	// Generate lines based on method
	var lines []domain.CashFlowLine
	var operatingNet, investingNet, financingNet decimal.Decimal

	if method == domain.CashFlowMethodIndirect {
		lines, operatingNet, investingNet, financingNet, err = s.generateIndirectMethod(ctx, run, entityID, fiscalPeriodID)
	} else {
		lines, operatingNet, investingNet, financingNet, err = s.generateDirectMethod(ctx, run, entityID, fiscalPeriodID)
	}

	if err != nil {
		run.Fail()
		_ = s.runRepo.Update(ctx, run)
		return run, fmt.Errorf("failed to generate cash flow lines: %w", err)
	}

	// Save lines
	if err := s.lineRepo.CreateBatch(ctx, lines); err != nil {
		run.Fail()
		_ = s.runRepo.Update(ctx, run)
		return run, fmt.Errorf("failed to save cash flow lines: %w", err)
	}

	// Calculate FX effect (difference between net change and actual cash change)
	netChange := operatingNet.Add(investingNet).Add(financingNet)
	actualChange := closingCash.Sub(openingCash)
	fxEffect := actualChange.Sub(netChange)

	// Set totals and complete
	run.SetTotals(operatingNet, investingNet, financingNet)
	run.SetCashBalances(openingCash, closingCash, fxEffect)
	run.Complete()
	run.Lines = lines

	if err := s.runRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update cash flow run: %w", err)
	}

	s.auditLogger.Log(ctx, "cashflow_run", run.ID, "generate", map[string]any{
		"method":        method,
		"period_id":     fiscalPeriodID,
		"operating_net": operatingNet.String(),
		"investing_net": investingNet.String(),
		"financing_net": financingNet.String(),
	})

	return run, nil
}

// generateIndirectMethod generates cash flow using indirect method
func (s *CashFlowService) generateIndirectMethod(
	ctx context.Context,
	run *domain.CashFlowRun,
	entityID, fiscalPeriodID common.ID,
) ([]domain.CashFlowLine, decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	var lines []domain.CashFlowLine
	lineNum := 0

	operatingNet := decimal.Zero
	investingNet := decimal.Zero
	financingNet := decimal.Zero

	// Get account configurations
	configs, err := s.configRepo.ListByEntity(ctx, entityID)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, err
	}

	// Get net income from income statement accounts
	netIncome, err := s.calculateNetIncome(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, err
	}

	// === OPERATING ACTIVITIES ===
	lineNum++
	opHeader := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
		domain.CashFlowLineTypeSubtotal, "OP_HEADER", "Cash Flows from Operating Activities", decimal.Zero)
	opHeader.SetFormatting(0, true)
	lines = append(lines, *opHeader)

	// Net Income
	lineNum++
	niLine := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
		domain.CashFlowLineTypeCashReceipt, "NET_INCOME", "Net Income", netIncome)
	niLine.SetFormatting(1, false)
	niLine.SetCalculation("From Income Statement")
	lines = append(lines, *niLine)
	operatingNet = operatingNet.Add(netIncome)

	// Adjustments header
	lineNum++
	adjHeader := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
		domain.CashFlowLineTypeAdjustment, "ADJ_HEADER", "Adjustments to reconcile net income:", decimal.Zero)
	adjHeader.SetFormatting(1, true)
	lines = append(lines, *adjHeader)

	// Process operating adjustments
	for _, config := range configs {
		if config.CashFlowCategory != domain.CashFlowCategoryOperating || config.AdjustmentType == "" {
			continue
		}

		balance, err := s.glAPI.GetAccountBalance(ctx, config.AccountID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		// Calculate the change in the account
		change := balance.ClosingDebit.Amount.Sub(balance.OpeningDebit.Amount).
			Sub(balance.ClosingCredit.Amount.Sub(balance.OpeningCredit.Amount))

		if change.IsZero() {
			continue
		}

		// Adjust sign based on account type for indirect method
		adjustedAmount := change
		switch config.AdjustmentType {
		case "add_back":
			adjustedAmount = change.Abs()
		case "subtract":
			adjustedAmount = change.Abs().Neg()
		}

		lineNum++
		adjLine := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
			domain.CashFlowLineTypeAdjustment, config.LineItemCode, config.AdjustmentType+" adjustment", adjustedAmount)
		adjLine.SetFormatting(2, false)
		adjLine.SetSourceAccounts([]string{config.AccountID.String()})
		lines = append(lines, *adjLine)
		operatingNet = operatingNet.Add(adjustedAmount)
	}

	// Operating subtotal
	lineNum++
	opSubtotal := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
		domain.CashFlowLineTypeTotal, "OP_TOTAL", "Net Cash from Operating Activities", operatingNet)
	opSubtotal.SetFormatting(0, true)
	lines = append(lines, *opSubtotal)

	// === INVESTING ACTIVITIES ===
	lineNum++
	invHeader := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryInvesting,
		domain.CashFlowLineTypeSubtotal, "INV_HEADER", "Cash Flows from Investing Activities", decimal.Zero)
	invHeader.SetFormatting(0, true)
	lines = append(lines, *invHeader)

	// Process investing items
	for _, config := range configs {
		if config.CashFlowCategory != domain.CashFlowCategoryInvesting {
			continue
		}

		balance, err := s.glAPI.GetAccountBalance(ctx, config.AccountID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		change := balance.ClosingDebit.Amount.Sub(balance.OpeningDebit.Amount).
			Sub(balance.ClosingCredit.Amount.Sub(balance.OpeningCredit.Amount))

		if change.IsZero() {
			continue
		}

		// For investing, cash paid is negative, cash received is positive
		cashEffect := change.Neg() // Increase in asset = cash outflow

		lineNum++
		invLine := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryInvesting,
			domain.CashFlowLineTypeCashPayment, config.LineItemCode, "Investing activity", cashEffect)
		invLine.SetFormatting(1, false)
		invLine.SetSourceAccounts([]string{config.AccountID.String()})
		lines = append(lines, *invLine)
		investingNet = investingNet.Add(cashEffect)
	}

	// Investing subtotal
	lineNum++
	invSubtotal := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryInvesting,
		domain.CashFlowLineTypeTotal, "INV_TOTAL", "Net Cash from Investing Activities", investingNet)
	invSubtotal.SetFormatting(0, true)
	lines = append(lines, *invSubtotal)

	// === FINANCING ACTIVITIES ===
	lineNum++
	finHeader := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryFinancing,
		domain.CashFlowLineTypeSubtotal, "FIN_HEADER", "Cash Flows from Financing Activities", decimal.Zero)
	finHeader.SetFormatting(0, true)
	lines = append(lines, *finHeader)

	// Process financing items
	for _, config := range configs {
		if config.CashFlowCategory != domain.CashFlowCategoryFinancing {
			continue
		}

		balance, err := s.glAPI.GetAccountBalance(ctx, config.AccountID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		change := balance.ClosingCredit.Amount.Sub(balance.OpeningCredit.Amount).
			Sub(balance.ClosingDebit.Amount.Sub(balance.OpeningDebit.Amount))

		if change.IsZero() {
			continue
		}

		// For financing, increase in liability = cash inflow
		lineNum++
		finLine := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryFinancing,
			domain.CashFlowLineTypeCashReceipt, config.LineItemCode, "Financing activity", change)
		finLine.SetFormatting(1, false)
		finLine.SetSourceAccounts([]string{config.AccountID.String()})
		lines = append(lines, *finLine)
		financingNet = financingNet.Add(change)
	}

	// Financing subtotal
	lineNum++
	finSubtotal := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryFinancing,
		domain.CashFlowLineTypeTotal, "FIN_TOTAL", "Net Cash from Financing Activities", financingNet)
	finSubtotal.SetFormatting(0, true)
	lines = append(lines, *finSubtotal)

	return lines, operatingNet, investingNet, financingNet, nil
}

// generateDirectMethod generates cash flow using direct method
func (s *CashFlowService) generateDirectMethod(
	ctx context.Context,
	run *domain.CashFlowRun,
	entityID, fiscalPeriodID common.ID,
) ([]domain.CashFlowLine, decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	var lines []domain.CashFlowLine
	lineNum := 0

	operatingNet := decimal.Zero
	investingNet := decimal.Zero
	financingNet := decimal.Zero

	// Get account configurations
	configs, err := s.configRepo.ListByEntity(ctx, entityID)
	if err != nil {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, err
	}

	// === OPERATING ACTIVITIES ===
	lineNum++
	opHeader := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
		domain.CashFlowLineTypeSubtotal, "OP_HEADER", "Cash Flows from Operating Activities", decimal.Zero)
	opHeader.SetFormatting(0, true)
	lines = append(lines, *opHeader)

	// Process operating receipts and payments
	for _, config := range configs {
		if config.CashFlowCategory != domain.CashFlowCategoryOperating {
			continue
		}

		balance, err := s.glAPI.GetAccountBalance(ctx, config.AccountID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		// For direct method, we look at actual cash movements
		debitChange := balance.ClosingDebit.Amount.Sub(balance.OpeningDebit.Amount)
		creditChange := balance.ClosingCredit.Amount.Sub(balance.OpeningCredit.Amount)

		if !debitChange.IsZero() {
			lineNum++
			lineType := domain.CashFlowLineTypeCashReceipt
			if debitChange.IsNegative() {
				lineType = domain.CashFlowLineTypeCashPayment
			}
			line := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
				lineType, config.LineItemCode+"_DR", "Operating cash movement", debitChange)
			line.SetFormatting(1, false)
			line.SetSourceAccounts([]string{config.AccountID.String()})
			lines = append(lines, *line)
			operatingNet = operatingNet.Add(debitChange)
		}

		if !creditChange.IsZero() {
			lineNum++
			lineType := domain.CashFlowLineTypeCashPayment
			if creditChange.IsPositive() {
				lineType = domain.CashFlowLineTypeCashReceipt
			}
			line := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
				lineType, config.LineItemCode+"_CR", "Operating cash movement", creditChange.Neg())
			line.SetFormatting(1, false)
			line.SetSourceAccounts([]string{config.AccountID.String()})
			lines = append(lines, *line)
			operatingNet = operatingNet.Sub(creditChange)
		}
	}

	// Operating subtotal
	lineNum++
	opSubtotal := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryOperating,
		domain.CashFlowLineTypeTotal, "OP_TOTAL", "Net Cash from Operating Activities", operatingNet)
	opSubtotal.SetFormatting(0, true)
	lines = append(lines, *opSubtotal)

	// === INVESTING ACTIVITIES ===
	lineNum++
	invHeader := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryInvesting,
		domain.CashFlowLineTypeSubtotal, "INV_HEADER", "Cash Flows from Investing Activities", decimal.Zero)
	invHeader.SetFormatting(0, true)
	lines = append(lines, *invHeader)

	for _, config := range configs {
		if config.CashFlowCategory != domain.CashFlowCategoryInvesting {
			continue
		}

		balance, err := s.glAPI.GetAccountBalance(ctx, config.AccountID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		change := balance.ClosingDebit.Amount.Sub(balance.OpeningDebit.Amount).
			Sub(balance.ClosingCredit.Amount.Sub(balance.OpeningCredit.Amount))

		if change.IsZero() {
			continue
		}

		cashEffect := change.Neg()
		lineNum++
		invLine := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryInvesting,
			domain.CashFlowLineTypeCashPayment, config.LineItemCode, "Investing activity", cashEffect)
		invLine.SetFormatting(1, false)
		invLine.SetSourceAccounts([]string{config.AccountID.String()})
		lines = append(lines, *invLine)
		investingNet = investingNet.Add(cashEffect)
	}

	// Investing subtotal
	lineNum++
	invSubtotal := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryInvesting,
		domain.CashFlowLineTypeTotal, "INV_TOTAL", "Net Cash from Investing Activities", investingNet)
	invSubtotal.SetFormatting(0, true)
	lines = append(lines, *invSubtotal)

	// === FINANCING ACTIVITIES ===
	lineNum++
	finHeader := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryFinancing,
		domain.CashFlowLineTypeSubtotal, "FIN_HEADER", "Cash Flows from Financing Activities", decimal.Zero)
	finHeader.SetFormatting(0, true)
	lines = append(lines, *finHeader)

	for _, config := range configs {
		if config.CashFlowCategory != domain.CashFlowCategoryFinancing {
			continue
		}

		balance, err := s.glAPI.GetAccountBalance(ctx, config.AccountID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		change := balance.ClosingCredit.Amount.Sub(balance.OpeningCredit.Amount).
			Sub(balance.ClosingDebit.Amount.Sub(balance.OpeningDebit.Amount))

		if change.IsZero() {
			continue
		}

		lineNum++
		finLine := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryFinancing,
			domain.CashFlowLineTypeCashReceipt, config.LineItemCode, "Financing activity", change)
		finLine.SetFormatting(1, false)
		finLine.SetSourceAccounts([]string{config.AccountID.String()})
		lines = append(lines, *finLine)
		financingNet = financingNet.Add(change)
	}

	// Financing subtotal
	lineNum++
	finSubtotal := domain.NewCashFlowLine(run.ID, lineNum, domain.CashFlowCategoryFinancing,
		domain.CashFlowLineTypeTotal, "FIN_TOTAL", "Net Cash from Financing Activities", financingNet)
	finSubtotal.SetFormatting(0, true)
	lines = append(lines, *finSubtotal)

	return lines, operatingNet, investingNet, financingNet, nil
}

// calculateNetIncome calculates net income for the period
func (s *CashFlowService) calculateNetIncome(ctx context.Context, entityID, fiscalPeriodID common.ID) (decimal.Decimal, error) {
	revenueType := "revenue"
	expenseType := "expense"
	isPosting := true
	isActive := true

	revenueAccounts, err := s.glAPI.ListAccounts(ctx, entityID, gl.AccountFilterRequest{
		Type:      &revenueType,
		IsPosting: &isPosting,
		IsActive:  &isActive,
	})
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get revenue accounts: %w", err)
	}

	expenseAccounts, err := s.glAPI.ListAccounts(ctx, entityID, gl.AccountFilterRequest{
		Type:      &expenseType,
		IsPosting: &isPosting,
		IsActive:  &isActive,
	})
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get expense accounts: %w", err)
	}

	totalRevenue := decimal.Zero
	for _, acc := range revenueAccounts {
		balance, err := s.glAPI.GetAccountBalance(ctx, acc.ID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}
		// Revenue is credit normal
		amount := balance.ClosingCredit.Amount.Sub(balance.ClosingDebit.Amount)
		totalRevenue = totalRevenue.Add(amount)
	}

	totalExpenses := decimal.Zero
	for _, acc := range expenseAccounts {
		balance, err := s.glAPI.GetAccountBalance(ctx, acc.ID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}
		// Expense is debit normal
		amount := balance.ClosingDebit.Amount.Sub(balance.ClosingCredit.Amount)
		totalExpenses = totalExpenses.Add(amount)
	}

	return totalRevenue.Sub(totalExpenses), nil
}

// GetCashFlowRun retrieves a cash flow run by ID
func (s *CashFlowService) GetCashFlowRun(ctx context.Context, id common.ID) (*domain.CashFlowRun, error) {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Load lines
	lines, err := s.lineRepo.GetByRunID(ctx, id)
	if err == nil {
		run.Lines = lines
	}

	return run, nil
}

// ListCashFlowRuns lists cash flow runs with filtering
func (s *CashFlowService) ListCashFlowRuns(
	ctx context.Context,
	entityID common.ID,
	fiscalPeriodID, fiscalYearID common.ID,
	status *domain.CashFlowRunStatus,
	limit, offset int,
) ([]domain.CashFlowRun, int, error) {
	filter := repository.CashFlowRunFilter{
		EntityID:       entityID,
		FiscalPeriodID: fiscalPeriodID,
		FiscalYearID:   fiscalYearID,
		Status:         status,
		Limit:          limit,
		Offset:         offset,
	}

	return s.runRepo.List(ctx, filter)
}

// DeleteCashFlowConfig deletes an account cash flow configuration
func (s *CashFlowService) DeleteCashFlowConfig(ctx context.Context, id common.ID) error {
	if err := s.configRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete account cash flow config: %w", err)
	}

	s.auditLogger.Log(ctx, "account_cashflow_config", id, "delete", nil)
	return nil
}
