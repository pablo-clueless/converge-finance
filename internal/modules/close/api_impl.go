package close

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/service"
)

type apiImpl struct {
	periodCloseService *service.PeriodCloseService
	reportService      *service.ReportService
	cashFlowService    *service.CashFlowService
}

func NewCloseAPI(
	periodCloseService *service.PeriodCloseService,
	reportService *service.ReportService,
	cashFlowService *service.CashFlowService,
) API {
	return &apiImpl{
		periodCloseService: periodCloseService,
		reportService:      reportService,
		cashFlowService:    cashFlowService,
	}
}

func (a *apiImpl) GetPeriodCloseStatus(ctx context.Context, entityID, fiscalPeriodID common.ID) (*PeriodCloseStatusResponse, error) {
	pc, err := a.periodCloseService.GetPeriodCloseStatus(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return nil, err
	}
	return toPeriodCloseStatusResponse(pc), nil
}

func (a *apiImpl) IsPeriodOpen(ctx context.Context, entityID, fiscalPeriodID common.ID) (bool, error) {
	pc, err := a.periodCloseService.GetPeriodCloseStatus(ctx, entityID, fiscalPeriodID)
	if err != nil {
		// If no status exists, period is open by default
		return true, nil
	}
	return pc.IsOpen(), nil
}

func (a *apiImpl) IsPeriodClosed(ctx context.Context, entityID, fiscalPeriodID common.ID) (bool, error) {
	pc, err := a.periodCloseService.GetPeriodCloseStatus(ctx, entityID, fiscalPeriodID)
	if err != nil {
		return false, nil
	}
	return pc.IsClosed(), nil
}

func (a *apiImpl) SoftClosePeriod(ctx context.Context, entityID, fiscalPeriodID common.ID, userID common.ID) (*PeriodCloseStatusResponse, error) {
	pc, err := a.periodCloseService.SoftClosePeriod(ctx, entityID, fiscalPeriodID, userID)
	if err != nil {
		return nil, err
	}
	return toPeriodCloseStatusResponse(pc), nil
}

func (a *apiImpl) HardClosePeriod(ctx context.Context, entityID, fiscalPeriodID, fiscalYearID common.ID, closeDate time.Time, currency money.Currency, userID common.ID) (*CloseRunResponse, error) {
	run, err := a.periodCloseService.HardClosePeriod(ctx, entityID, fiscalPeriodID, fiscalYearID, closeDate, currency, userID)
	if err != nil {
		return nil, err
	}
	return toCloseRunResponse(run), nil
}

func (a *apiImpl) GenerateTrialBalance(ctx context.Context, entityID, fiscalPeriodID common.ID, asOfDate time.Time, userID common.ID) (*ReportRunResponse, error) {
	run, err := a.reportService.GenerateTrialBalance(ctx, entityID, fiscalPeriodID, asOfDate, userID)
	if err != nil {
		return nil, err
	}
	return toReportRunResponse(run), nil
}

func (a *apiImpl) GenerateIncomeStatement(ctx context.Context, entityID, fiscalPeriodID, fiscalYearID common.ID, asOfDate time.Time, userID common.ID) (*ReportRunResponse, error) {
	run, err := a.reportService.GenerateIncomeStatement(ctx, entityID, fiscalPeriodID, fiscalYearID, asOfDate, userID)
	if err != nil {
		return nil, err
	}
	return toReportRunResponse(run), nil
}

func (a *apiImpl) GenerateBalanceSheet(ctx context.Context, entityID, fiscalPeriodID, fiscalYearID common.ID, asOfDate time.Time, userID common.ID) (*ReportRunResponse, error) {
	run, err := a.reportService.GenerateBalanceSheet(ctx, entityID, fiscalPeriodID, fiscalYearID, asOfDate, userID)
	if err != nil {
		return nil, err
	}
	return toReportRunResponse(run), nil
}

func (a *apiImpl) GetReportRun(ctx context.Context, id common.ID) (*ReportRunResponse, error) {
	run, err := a.reportService.GetReportRun(ctx, id)
	if err != nil {
		return nil, err
	}
	return toReportRunResponse(run), nil
}

func toPeriodCloseStatusResponse(pc *domain.PeriodClose) *PeriodCloseStatusResponse {
	return &PeriodCloseStatusResponse{
		ID:                    pc.ID,
		EntityID:              pc.EntityID,
		FiscalPeriodID:        pc.FiscalPeriodID,
		FiscalYearID:          pc.FiscalYearID,
		Status:                string(pc.Status),
		SoftClosedAt:          pc.SoftClosedAt,
		HardClosedAt:          pc.HardClosedAt,
		ClosingJournalEntryID: pc.ClosingJournalEntryID,
	}
}

func toCloseRunResponse(run *domain.CloseRun) *CloseRunResponse {
	return &CloseRunResponse{
		ID:                    run.ID,
		EntityID:              run.EntityID,
		RunNumber:             run.RunNumber,
		CloseType:             string(run.CloseType),
		FiscalPeriodID:        run.FiscalPeriodID,
		CloseDate:             run.CloseDate,
		Status:                string(run.Status),
		RulesExecuted:         run.RulesExecuted,
		EntriesCreated:        run.EntriesCreated,
		TotalDebits:           run.TotalDebits,
		TotalCredits:          run.TotalCredits,
		ClosingJournalEntryID: run.ClosingJournalEntryID,
		CompletedAt:           run.CompletedAt,
	}
}

func toReportRunResponse(run *domain.ReportRun) *ReportRunResponse {
	resp := &ReportRunResponse{
		ID:             run.ID,
		EntityID:       run.EntityID,
		ReportNumber:   run.ReportNumber,
		ReportType:     string(run.ReportType),
		ReportFormat:   string(run.ReportFormat),
		ReportName:     run.ReportName,
		FiscalPeriodID: run.FiscalPeriodID,
		AsOfDate:       run.AsOfDate,
		Status:         string(run.Status),
		GeneratedAt:    run.GeneratedAt,
		CompletedAt:    run.CompletedAt,
	}

	if len(run.DataRows) > 0 {
		resp.DataRows = make([]ReportDataRowResponse, len(run.DataRows))
		for i, row := range run.DataRows {
			resp.DataRows[i] = ReportDataRowResponse{
				RowNumber:    row.RowNumber,
				RowType:      string(row.RowType),
				IndentLevel:  row.IndentLevel,
				AccountCode:  row.AccountCode,
				AccountName:  row.AccountName,
				Description:  row.Description,
				Amount1:      row.Amount1,
				Amount2:      row.Amount2,
				Amount3:      row.Amount3,
				CurrencyCode: row.CurrencyCode,
				IsBold:       row.IsBold,
				IsUnderlined: row.IsUnderlined,
			}
		}
	}

	return resp
}

// Cash Flow methods

func (a *apiImpl) ConfigureAccountCashFlow(ctx context.Context, req ConfigureAccountCashFlowRequest) (*AccountCashFlowConfigResponse, error) {
	config, err := a.cashFlowService.ConfigureAccountCashFlow(
		ctx,
		req.EntityID,
		req.AccountID,
		domain.CashFlowCategory(req.Category),
		req.LineItemCode,
		req.IsCashAccount,
		req.IsCashEquivalent,
		req.AdjustmentType,
	)
	if err != nil {
		return nil, err
	}
	return toAccountCashFlowConfigResponse(config), nil
}

func (a *apiImpl) ListAccountCashFlowConfigs(ctx context.Context, entityID common.ID) ([]AccountCashFlowConfigResponse, error) {
	configs, err := a.cashFlowService.ListAccountCashFlowConfigs(ctx, entityID)
	if err != nil {
		return nil, err
	}

	resp := make([]AccountCashFlowConfigResponse, len(configs))
	for i, c := range configs {
		resp[i] = *toAccountCashFlowConfigResponse(&c)
	}
	return resp, nil
}

func (a *apiImpl) GenerateCashFlowStatement(ctx context.Context, req GenerateCashFlowRequest) (*CashFlowRunResponse, error) {
	run, err := a.cashFlowService.GenerateCashFlowStatement(
		ctx,
		req.EntityID,
		req.FiscalPeriodID,
		req.FiscalYearID,
		domain.CashFlowMethod(req.Method),
		req.PeriodStart,
		req.PeriodEnd,
		req.CurrencyCode,
		req.UserID,
	)
	if err != nil {
		return nil, err
	}
	return toCashFlowRunResponse(run), nil
}

func (a *apiImpl) GetCashFlowRun(ctx context.Context, id common.ID) (*CashFlowRunResponse, error) {
	run, err := a.cashFlowService.GetCashFlowRun(ctx, id)
	if err != nil {
		return nil, err
	}
	return toCashFlowRunResponse(run), nil
}

func (a *apiImpl) ListCashFlowRuns(ctx context.Context, req ListCashFlowRunsRequest) (*ListCashFlowRunsResponse, error) {
	var status *domain.CashFlowRunStatus
	if req.Status != nil {
		s := domain.CashFlowRunStatus(*req.Status)
		status = &s
	}

	runs, total, err := a.cashFlowService.ListCashFlowRuns(
		ctx,
		req.EntityID,
		req.FiscalPeriodID,
		req.FiscalYearID,
		status,
		req.Limit,
		req.Offset,
	)
	if err != nil {
		return nil, err
	}

	resp := &ListCashFlowRunsResponse{
		Runs:  make([]CashFlowRunResponse, len(runs)),
		Total: total,
	}
	for i, run := range runs {
		resp.Runs[i] = *toCashFlowRunResponse(&run)
	}
	return resp, nil
}

func toAccountCashFlowConfigResponse(c *domain.AccountCashFlowConfig) *AccountCashFlowConfigResponse {
	return &AccountCashFlowConfigResponse{
		ID:               c.ID,
		EntityID:         c.EntityID,
		AccountID:        c.AccountID,
		Category:         string(c.CashFlowCategory),
		LineItemCode:     c.LineItemCode,
		IsCashAccount:    c.IsCashAccount,
		IsCashEquivalent: c.IsCashEquivalent,
		AdjustmentType:   c.AdjustmentType,
		CreatedAt:        c.CreatedAt,
		UpdatedAt:        c.UpdatedAt,
	}
}

func toCashFlowRunResponse(run *domain.CashFlowRun) *CashFlowRunResponse {
	resp := &CashFlowRunResponse{
		ID:             run.ID,
		EntityID:       run.EntityID,
		RunNumber:      run.RunNumber,
		Method:         string(run.Method),
		FiscalPeriodID: run.FiscalPeriodID,
		FiscalYearID:   run.FiscalYearID,
		PeriodStart:    run.PeriodStart,
		PeriodEnd:      run.PeriodEnd,
		CurrencyCode:   run.CurrencyCode,
		OperatingNet:   run.OperatingNet.InexactFloat64(),
		InvestingNet:   run.InvestingNet.InexactFloat64(),
		FinancingNet:   run.FinancingNet.InexactFloat64(),
		NetChange:      run.NetChange.InexactFloat64(),
		OpeningCash:    run.OpeningCash.InexactFloat64(),
		ClosingCash:    run.ClosingCash.InexactFloat64(),
		FXEffect:       run.FXEffect.InexactFloat64(),
		Status:         string(run.Status),
		GeneratedBy:    run.GeneratedBy,
		GeneratedAt:    run.GeneratedAt,
	}

	if len(run.Lines) > 0 {
		resp.Lines = make([]CashFlowLineResponse, len(run.Lines))
		for i, line := range run.Lines {
			resp.Lines[i] = CashFlowLineResponse{
				ID:             line.ID,
				LineNumber:     line.LineNumber,
				Category:       string(line.Category),
				LineType:       string(line.LineType),
				LineCode:       line.LineCode,
				Description:    line.Description,
				Amount:         line.Amount.InexactFloat64(),
				IndentLevel:    line.IndentLevel,
				IsBold:         line.IsBold,
				SourceAccounts: line.SourceAccounts,
				Calculation:    line.Calculation,
			}
		}
	}

	return resp
}
