package service

import (
	"context"
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

type ReportService struct {
	db               *database.PostgresDB
	templateRepo     repository.ReportTemplateRepository
	reportRunRepo    repository.ReportRunRepository
	reportDataRepo   repository.ReportDataRepository
	scheduledRepo    repository.ScheduledReportRepository
	yearEndCheckRepo repository.YearEndChecklistRepository
	glAPI            gl.API
	auditLogger      *audit.Logger
}

func NewReportService(
	db *database.PostgresDB,
	templateRepo repository.ReportTemplateRepository,
	reportRunRepo repository.ReportRunRepository,
	reportDataRepo repository.ReportDataRepository,
	scheduledRepo repository.ScheduledReportRepository,
	yearEndCheckRepo repository.YearEndChecklistRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *ReportService {
	return &ReportService{
		db:               db,
		templateRepo:     templateRepo,
		reportRunRepo:    reportRunRepo,
		reportDataRepo:   reportDataRepo,
		scheduledRepo:    scheduledRepo,
		yearEndCheckRepo: yearEndCheckRepo,
		glAPI:            glAPI,
		auditLogger:      auditLogger,
	}
}

func (s *ReportService) GetReportTemplate(ctx context.Context, id common.ID) (*domain.ReportTemplate, error) {
	return s.templateRepo.GetByID(ctx, id)
}

func (s *ReportService) ListReportTemplates(ctx context.Context, filter domain.ReportTemplateFilter) ([]domain.ReportTemplate, error) {
	return s.templateRepo.List(ctx, filter)
}

func (s *ReportService) GetSystemTemplates(ctx context.Context) ([]domain.ReportTemplate, error) {
	return s.templateRepo.GetSystemTemplates(ctx)
}

func (s *ReportService) CreateReportTemplate(
	ctx context.Context,
	entityID common.ID,
	templateCode, templateName string,
	reportType domain.ReportType,
	reportFormat domain.ReportFormat,
	config map[string]interface{},
) (*domain.ReportTemplate, error) {
	template := domain.NewReportTemplate(&entityID, templateCode, templateName, reportType, reportFormat)

	if config != nil {
		if err := template.SetConfiguration(config); err != nil {
			return nil, fmt.Errorf("failed to set configuration: %w", err)
		}
	}

	if err := s.templateRepo.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("failed to create report template: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "report_template", template.ID, "create", map[string]interface{}{
		"template_code": templateCode,
		"report_type":   reportType,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return template, nil
}

func (s *ReportService) GenerateTrialBalance(
	ctx context.Context,
	entityID common.ID,
	fiscalPeriodID common.ID,
	asOfDate time.Time,
	userID common.ID,
) (*domain.ReportRun, error) {

	tb, err := s.glAPI.GetTrialBalance(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trial balance: %w", err)
	}

	reportNumber, err := s.reportRunRepo.GetNextReportNumber(ctx, entityID, "TB")
	if err != nil {
		return nil, fmt.Errorf("failed to generate report number: %w", err)
	}

	run := domain.NewReportRun(
		entityID,
		reportNumber,
		nil,
		domain.ReportTypeTrialBalance,
		domain.ReportFormatDetailed,
		fmt.Sprintf("Trial Balance - %s", tb.PeriodName),
		asOfDate,
		userID,
	)
	run.SetPeriod(fiscalPeriodID, tb.EntityID)

	if err := run.StartGenerating(); err != nil {
		return nil, err
	}

	if err := s.reportRunRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create report run: %w", err)
	}

	var rows []domain.ReportDataRow
	rowNum := 0

	rowNum++
	headerRow := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeHeader)
	headerRow.Description = "Trial Balance"
	headerRow.IsBold = true
	rows = append(rows, *headerRow)

	for _, acc := range tb.Accounts {
		rowNum++
		row := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeDetail)
		row.AccountID = &acc.AccountID
		row.AccountCode = acc.AccountCode
		row.AccountName = acc.AccountName

		debitVal := acc.Debit.Amount.InexactFloat64()
		creditVal := acc.Credit.Amount.InexactFloat64()
		row.Amount1 = &debitVal
		row.Amount2 = &creditVal
		row.CurrencyCode = acc.Debit.Currency.Code

		rows = append(rows, *row)
	}

	rowNum++
	totalRow := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeTotal)
	totalRow.Description = "Total"
	totalRow.IsBold = true
	totalRow.IsUnderlined = true
	totalDebit := tb.TotalDebit.Amount.InexactFloat64()
	totalCredit := tb.TotalCredit.Amount.InexactFloat64()
	totalRow.Amount1 = &totalDebit
	totalRow.Amount2 = &totalCredit
	totalRow.CurrencyCode = tb.TotalDebit.Currency.Code
	rows = append(rows, *totalRow)

	if err := s.reportDataRepo.CreateBatch(ctx, rows); err != nil {
		_ = run.Fail(err.Error())
		_ = s.reportRunRepo.Update(ctx, run)
		return run, fmt.Errorf("failed to save report data: %w", err)
	}

	if err := run.Complete(); err != nil {
		return run, err
	}

	if err := s.reportRunRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update report run: %w", err)
	}

	run.DataRows = rows

	if err := s.auditLogger.Log(ctx, "report_run", run.ID, "generate", map[string]interface{}{
		"report_type": domain.ReportTypeTrialBalance,
		"period_id":   fiscalPeriodID,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return run, nil
}

func (s *ReportService) GenerateIncomeStatement(
	ctx context.Context,
	entityID common.ID,
	fiscalPeriodID, fiscalYearID common.ID,
	asOfDate time.Time,
	userID common.ID,
) (*domain.ReportRun, error) {

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
		return nil, fmt.Errorf("failed to get revenue accounts: %w", err)
	}

	expenseAccounts, err := s.glAPI.ListAccounts(ctx, entityID, gl.AccountFilterRequest{
		Type:      &expenseType,
		IsPosting: &isPosting,
		IsActive:  &isActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get expense accounts: %w", err)
	}

	reportNumber, err := s.reportRunRepo.GetNextReportNumber(ctx, entityID, "IS")
	if err != nil {
		return nil, fmt.Errorf("failed to generate report number: %w", err)
	}

	run := domain.NewReportRun(
		entityID,
		reportNumber,
		nil,
		domain.ReportTypeIncomeStatement,
		domain.ReportFormatSummary,
		"Income Statement",
		asOfDate,
		userID,
	)
	run.SetPeriod(fiscalPeriodID, fiscalYearID)

	if err := run.StartGenerating(); err != nil {
		return nil, err
	}

	if err := s.reportRunRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create report run: %w", err)
	}

	var rows []domain.ReportDataRow
	rowNum := 0

	var currency money.Currency
	if len(revenueAccounts) > 0 {
		bal, _ := s.glAPI.GetAccountBalance(ctx, revenueAccounts[0].ID, fiscalPeriodID)
		if bal != nil {
			currency = bal.ClosingCredit.Currency
		}
	}
	if currency.Code == "" {
		currency = money.MustGetCurrency("USD")
	}

	totalRevenue := money.Zero(currency)
	totalExpenses := money.Zero(currency)

	rowNum++
	revenueHeader := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeHeader)
	revenueHeader.Description = "Revenue"
	revenueHeader.IsBold = true
	rows = append(rows, *revenueHeader)

	for _, acc := range revenueAccounts {
		balance, err := s.glAPI.GetAccountBalance(ctx, acc.ID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		amount := balance.ClosingCredit.MustSubtract(balance.ClosingDebit)
		if amount.IsZero() {
			continue
		}

		rowNum++
		row := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeDetail)
		row.AccountID = &acc.ID
		row.AccountCode = acc.Code
		row.AccountName = acc.Name
		row.IndentLevel = 1
		amtVal := amount.Amount.InexactFloat64()
		row.Amount1 = &amtVal
		row.CurrencyCode = amount.Currency.Code
		rows = append(rows, *row)

		totalRevenue = totalRevenue.MustAdd(amount)
	}

	rowNum++
	revSubtotal := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeSubtotal)
	revSubtotal.Description = "Total Revenue"
	revSubtotal.IsBold = true
	revVal := totalRevenue.Amount.InexactFloat64()
	revSubtotal.Amount1 = &revVal
	revSubtotal.CurrencyCode = totalRevenue.Currency.Code
	rows = append(rows, *revSubtotal)

	rowNum++
	blankRow := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeBlank)
	rows = append(rows, *blankRow)

	rowNum++
	expenseHeader := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeHeader)
	expenseHeader.Description = "Expenses"
	expenseHeader.IsBold = true
	rows = append(rows, *expenseHeader)

	for _, acc := range expenseAccounts {
		balance, err := s.glAPI.GetAccountBalance(ctx, acc.ID, fiscalPeriodID)
		if err != nil || balance == nil {
			continue
		}

		amount := balance.ClosingDebit.MustSubtract(balance.ClosingCredit)
		if amount.IsZero() {
			continue
		}

		rowNum++
		row := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeDetail)
		row.AccountID = &acc.ID
		row.AccountCode = acc.Code
		row.AccountName = acc.Name
		row.IndentLevel = 1
		amtVal := amount.Amount.InexactFloat64()
		row.Amount1 = &amtVal
		row.CurrencyCode = amount.Currency.Code
		rows = append(rows, *row)

		totalExpenses = totalExpenses.MustAdd(amount)
	}

	rowNum++
	expSubtotal := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeSubtotal)
	expSubtotal.Description = "Total Expenses"
	expSubtotal.IsBold = true
	expVal := totalExpenses.Amount.InexactFloat64()
	expSubtotal.Amount1 = &expVal
	expSubtotal.CurrencyCode = totalExpenses.Currency.Code
	rows = append(rows, *expSubtotal)

	rowNum++
	blankRow2 := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeBlank)
	rows = append(rows, *blankRow2)

	netIncome := totalRevenue.MustSubtract(totalExpenses)
	rowNum++
	netIncomeRow := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeTotal)
	netIncomeRow.Description = "Net Income"
	netIncomeRow.IsBold = true
	netIncomeRow.IsUnderlined = true
	netVal := netIncome.Amount.InexactFloat64()
	netIncomeRow.Amount1 = &netVal
	netIncomeRow.CurrencyCode = netIncome.Currency.Code
	rows = append(rows, *netIncomeRow)

	if err := s.reportDataRepo.CreateBatch(ctx, rows); err != nil {
		_ = run.Fail(err.Error())
		_ = s.reportRunRepo.Update(ctx, run)
		return run, fmt.Errorf("failed to save report data: %w", err)
	}

	if err := run.Complete(); err != nil {
		return run, err
	}

	if err := s.reportRunRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update report run: %w", err)
	}

	run.DataRows = rows

	if err := s.auditLogger.Log(ctx, "report_run", run.ID, "generate", map[string]interface{}{
		"report_type": domain.ReportTypeIncomeStatement,
		"period_id":   fiscalPeriodID,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return run, nil
}

func (s *ReportService) GenerateBalanceSheet(
	ctx context.Context,
	entityID common.ID,
	fiscalPeriodID, fiscalYearID common.ID,
	asOfDate time.Time,
	userID common.ID,
) (*domain.ReportRun, error) {

	assetType := "asset"
	liabilityType := "liability"
	equityType := "equity"
	isPosting := true
	isActive := true

	assetAccounts, err := s.glAPI.ListAccounts(ctx, entityID, gl.AccountFilterRequest{
		Type:      &assetType,
		IsPosting: &isPosting,
		IsActive:  &isActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get asset accounts: %w", err)
	}

	liabilityAccounts, err := s.glAPI.ListAccounts(ctx, entityID, gl.AccountFilterRequest{
		Type:      &liabilityType,
		IsPosting: &isPosting,
		IsActive:  &isActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get liability accounts: %w", err)
	}

	equityAccounts, err := s.glAPI.ListAccounts(ctx, entityID, gl.AccountFilterRequest{
		Type:      &equityType,
		IsPosting: &isPosting,
		IsActive:  &isActive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get equity accounts: %w", err)
	}

	reportNumber, err := s.reportRunRepo.GetNextReportNumber(ctx, entityID, "BS")
	if err != nil {
		return nil, fmt.Errorf("failed to generate report number: %w", err)
	}

	run := domain.NewReportRun(
		entityID,
		reportNumber,
		nil,
		domain.ReportTypeBalanceSheet,
		domain.ReportFormatSummary,
		"Balance Sheet",
		asOfDate,
		userID,
	)
	run.SetPeriod(fiscalPeriodID, fiscalYearID)

	if err := run.StartGenerating(); err != nil {
		return nil, err
	}

	if err := s.reportRunRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create report run: %w", err)
	}

	var currency money.Currency
	if len(assetAccounts) > 0 {
		bal, _ := s.glAPI.GetAccountBalance(ctx, assetAccounts[0].ID, fiscalPeriodID)
		if bal != nil {
			currency = bal.ClosingDebit.Currency
		}
	}
	if currency.Code == "" {
		currency = money.MustGetCurrency("USD")
	}

	var rows []domain.ReportDataRow
	rowNum := 0

	var totalAssets, totalLiabilities, totalEquity money.Money

	addSection := func(title string, accounts []gl.AccountResponse, isDebitNormal bool) money.Money {
		total := money.Zero(currency)

		rowNum++
		header := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeHeader)
		header.Description = title
		header.IsBold = true
		rows = append(rows, *header)

		for _, acc := range accounts {
			balance, err := s.glAPI.GetAccountBalance(ctx, acc.ID, fiscalPeriodID)
			if err != nil || balance == nil {
				continue
			}

			var amount money.Money
			if isDebitNormal {
				amount = balance.ClosingDebit.MustSubtract(balance.ClosingCredit)
			} else {
				amount = balance.ClosingCredit.MustSubtract(balance.ClosingDebit)
			}

			if amount.IsZero() {
				continue
			}

			rowNum++
			row := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeDetail)
			row.AccountID = &acc.ID
			row.AccountCode = acc.Code
			row.AccountName = acc.Name
			row.IndentLevel = 1
			amtVal := amount.Amount.InexactFloat64()
			row.Amount1 = &amtVal
			row.CurrencyCode = amount.Currency.Code
			rows = append(rows, *row)

			total = total.MustAdd(amount)
		}

		rowNum++
		subtotal := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeSubtotal)
		subtotal.Description = "Total " + title
		subtotal.IsBold = true
		totalVal := total.Amount.InexactFloat64()
		subtotal.Amount1 = &totalVal
		subtotal.CurrencyCode = total.Currency.Code
		rows = append(rows, *subtotal)

		rowNum++
		blank := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeBlank)
		rows = append(rows, *blank)

		return total
	}

	totalAssets = addSection("Assets", assetAccounts, true)

	totalLiabilities = addSection("Liabilities", liabilityAccounts, false)

	totalEquity = addSection("Equity", equityAccounts, false)

	totalLiabEquity := totalLiabilities.MustAdd(totalEquity)
	rowNum++
	totalRow := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeTotal)
	totalRow.Description = "Total Liabilities & Equity"
	totalRow.IsBold = true
	totalRow.IsUnderlined = true
	leVal := totalLiabEquity.Amount.InexactFloat64()
	totalRow.Amount1 = &leVal
	totalRow.CurrencyCode = totalLiabEquity.Currency.Code
	rows = append(rows, *totalRow)

	rowNum++
	checkRow := domain.NewReportDataRow(run.ID, rowNum, domain.RowTypeDetail)
	diff := totalAssets.MustSubtract(totalLiabEquity)
	if diff.IsZero() {
		checkRow.Description = "Balance Check: OK"
	} else {
		diffVal := diff.Amount.InexactFloat64()
		checkRow.Description = fmt.Sprintf("Out of Balance: %.2f", diffVal)
	}
	rows = append(rows, *checkRow)

	if err := s.reportDataRepo.CreateBatch(ctx, rows); err != nil {
		_ = run.Fail(err.Error())
		_ = s.reportRunRepo.Update(ctx, run)
		return run, fmt.Errorf("failed to save report data: %w", err)
	}

	if err := run.Complete(); err != nil {
		return run, err
	}

	if err := s.reportRunRepo.Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update report run: %w", err)
	}

	run.DataRows = rows

	if err := s.auditLogger.Log(ctx, "report_run", run.ID, "generate", map[string]interface{}{
		"report_type": domain.ReportTypeBalanceSheet,
		"period_id":   fiscalPeriodID,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return run, nil
}

func (s *ReportService) GetReportRun(ctx context.Context, id common.ID) (*domain.ReportRun, error) {
	run, err := s.reportRunRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	rows, err := s.reportDataRepo.GetByReportRunID(ctx, id)
	if err == nil {
		run.DataRows = rows
	}

	return run, nil
}

func (s *ReportService) ListReportRuns(ctx context.Context, filter domain.ReportRunFilter) ([]domain.ReportRun, error) {
	return s.reportRunRepo.List(ctx, filter)
}

func (s *ReportService) DeleteReportRun(ctx context.Context, id common.ID) error {
	if err := s.reportRunRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete report run: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "report_run", id, "delete", nil); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}
	return nil
}

func (s *ReportService) GetYearEndChecklist(ctx context.Context, entityID, fiscalYearID common.ID) (*domain.YearEndChecklist, error) {
	checklist, err := s.yearEndCheckRepo.GetByFiscalYear(ctx, entityID, fiscalYearID)
	if err == nil {

		items, _ := s.yearEndCheckRepo.GetItems(ctx, checklist.ID)
		checklist.Items = items
		return checklist, nil
	}

	checklist = &domain.YearEndChecklist{
		ID:            common.NewID(),
		EntityID:      entityID,
		FiscalYearID:  fiscalYearID,
		ChecklistName: "Year-End Close",
		Status:        "in_progress",
		StartedAt:     time.Now(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.yearEndCheckRepo.Create(ctx, checklist); err != nil {
		return nil, fmt.Errorf("failed to create year-end checklist: %w", err)
	}

	defaultItems := []struct {
		code, name, desc string
	}{
		{"REC_AP", "Reconcile Accounts Payable", "Verify all AP balances match vendor statements"},
		{"REC_AR", "Reconcile Accounts Receivable", "Verify all AR balances and aging"},
		{"REC_BANK", "Reconcile Bank Accounts", "Complete all bank reconciliations"},
		{"REV_FA", "Review Fixed Assets", "Review depreciation and asset disposals"},
		{"REV_INV", "Review Inventory", "Verify inventory counts and valuations"},
		{"ADJ_ACCR", "Post Accrual Adjustments", "Record all accrued expenses and revenue"},
		{"ADJ_PREPD", "Adjust Prepaid Expenses", "Amortize prepaid expenses"},
		{"REV_IC", "Review Intercompany Balances", "Verify and eliminate intercompany transactions"},
		{"GEN_TB", "Generate Trial Balance", "Run final trial balance"},
		{"CLOSE_INC", "Close Income Accounts", "Close revenue accounts to income summary"},
		{"CLOSE_EXP", "Close Expense Accounts", "Close expense accounts to income summary"},
		{"CLOSE_SUM", "Close Income Summary", "Close income summary to retained earnings"},
	}

	for i, item := range defaultItems {
		checkItem := &domain.YearEndChecklistItem{
			ID:             common.NewID(),
			ChecklistID:    checklist.ID,
			SequenceNumber: i + 1,
			ItemCode:       item.code,
			ItemName:       item.name,
			Description:    item.desc,
			IsRequired:     true,
			IsCompleted:    false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		_ = s.yearEndCheckRepo.CreateItem(ctx, checkItem)
		checklist.Items = append(checklist.Items, *checkItem)
	}

	if err := s.auditLogger.Log(ctx, "year_end_checklist", checklist.ID, "create", map[string]interface{}{
		"fiscal_year_id": fiscalYearID,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return checklist, nil
}

func (s *ReportService) UpdateChecklistItem(ctx context.Context, itemID common.ID, completed bool, userID common.ID, notes string) (*domain.YearEndChecklistItem, error) {

	item := &domain.YearEndChecklistItem{
		ID:          itemID,
		IsCompleted: completed,
		Notes:       notes,
		UpdatedAt:   time.Now(),
	}

	if completed {
		now := time.Now()
		item.CompletedAt = &now
		item.CompletedBy = &userID
	}

	if err := s.yearEndCheckRepo.UpdateItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to update checklist item: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "checklist_item", itemID, "update", map[string]interface{}{
		"completed": completed,
		"user_id":   userID,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return item, nil
}
