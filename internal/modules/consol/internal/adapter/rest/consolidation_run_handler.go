package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
	"converge-finance.com/m/internal/modules/consol/internal/service"
	"go.uber.org/zap"
)

type ConsolidationRunHandler struct {
	*Handler
	consolidationService *service.ConsolidationService
}

func NewConsolidationRunHandler(
	logger *zap.Logger,
	consolidationService *service.ConsolidationService,
) *ConsolidationRunHandler {
	return &ConsolidationRunHandler{
		Handler:              NewHandler(logger),
		consolidationService: consolidationService,
	}
}

type InitiateRunRequest struct {
	ConsolidationSetID common.ID `json:"consolidation_set_id"`
	FiscalPeriodID     common.ID `json:"fiscal_period_id"`
	ConsolidationDate  string    `json:"consolidation_date"`
	ClosingRateDate    string    `json:"closing_rate_date"`
	AverageRateDate    *string   `json:"average_rate_date,omitempty"`
}

type ConsolidationRunResponse struct {
	ID                    common.ID  `json:"id"`
	RunNumber             string     `json:"run_number"`
	ConsolidationSetID    common.ID  `json:"consolidation_set_id"`
	ConsolidationSetCode  string     `json:"consolidation_set_code,omitempty"`
	ConsolidationSetName  string     `json:"consolidation_set_name,omitempty"`
	FiscalPeriodID        common.ID  `json:"fiscal_period_id"`
	FiscalPeriodName      string     `json:"fiscal_period_name,omitempty"`
	ReportingCurrency     string     `json:"reporting_currency"`
	ConsolidationDate     string     `json:"consolidation_date"`
	ClosingRateDate       string     `json:"closing_rate_date"`
	AverageRateDate       *string    `json:"average_rate_date,omitempty"`
	EntityCount           int        `json:"entity_count"`
	TotalAssets           money.Money `json:"total_assets"`
	TotalLiabilities      money.Money `json:"total_liabilities"`
	TotalEquity           money.Money `json:"total_equity"`
	TotalRevenue          money.Money `json:"total_revenue"`
	TotalExpenses         money.Money `json:"total_expenses"`
	NetIncome             money.Money `json:"net_income"`
	TotalCTA              money.Money `json:"total_cta"`
	TotalMinorityInterest money.Money `json:"total_minority_interest"`
	Status                string      `json:"status"`
	JournalEntryID        *common.ID  `json:"journal_entry_id,omitempty"`
	CreatedAt             time.Time   `json:"created_at"`
	CompletedAt           *time.Time  `json:"completed_at,omitempty"`
	PostedAt              *time.Time  `json:"posted_at,omitempty"`
}

type ConsolidatedBalanceResponse struct {
	AccountID             common.ID   `json:"account_id"`
	AccountCode           string      `json:"account_code"`
	AccountName           string      `json:"account_name"`
	AccountType           string      `json:"account_type"`
	OpeningBalance        money.Money `json:"opening_balance"`
	PeriodDebit           money.Money `json:"period_debit"`
	PeriodCredit          money.Money `json:"period_credit"`
	EliminationDebit      money.Money `json:"elimination_debit"`
	EliminationCredit     money.Money `json:"elimination_credit"`
	TranslationAdjustment money.Money `json:"translation_adjustment"`
	MinorityInterest      money.Money `json:"minority_interest"`
	ClosingBalance        money.Money `json:"closing_balance"`
}

func (h *ConsolidationRunHandler) InitiateRun(w http.ResponseWriter, r *http.Request) {
	var req InitiateRunRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	consolidationDate, err := time.Parse("2006-01-02", req.ConsolidationDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid consolidation date format")
		return
	}

	closingRateDate, err := time.Parse("2006-01-02", req.ClosingRateDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid closing rate date format")
		return
	}

	run, err := h.consolidationService.InitiateConsolidationRun(
		r.Context(),
		req.ConsolidationSetID,
		req.FiscalPeriodID,
		consolidationDate,
		closingRateDate,
	)
	if err != nil {
		h.logger.Error("failed to initiate consolidation run", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.AverageRateDate != nil {
		avgDate, err := time.Parse("2006-01-02", *req.AverageRateDate)
		if err == nil {
			run.SetAverageRateDate(avgDate)
		}
	}

	respondJSON(w, http.StatusCreated, toRunResponse(run))
}

func (h *ConsolidationRunHandler) ExecuteRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := h.consolidationService.ExecuteConsolidation(r.Context(), id); err != nil {
		h.logger.Error("failed to execute consolidation", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	run, _ := h.consolidationService.GetConsolidationRun(r.Context(), id)
	respondJSON(w, http.StatusOK, toRunResponse(run))
}

func (h *ConsolidationRunHandler) PostRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := h.consolidationService.PostConsolidation(r.Context(), id); err != nil {
		h.logger.Error("failed to post consolidation", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	run, _ := h.consolidationService.GetConsolidationRun(r.Context(), id)
	respondJSON(w, http.StatusOK, toRunResponse(run))
}

func (h *ConsolidationRunHandler) ReverseRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := h.consolidationService.ReverseConsolidation(r.Context(), id); err != nil {
		h.logger.Error("failed to reverse consolidation", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	run, _ := h.consolidationService.GetConsolidationRun(r.Context(), id)
	respondJSON(w, http.StatusOK, toRunResponse(run))
}

func (h *ConsolidationRunHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	run, err := h.consolidationService.GetConsolidationRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "consolidation run not found")
		return
	}

	respondJSON(w, http.StatusOK, toRunResponse(run))
}

func (h *ConsolidationRunHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	setIDStr := r.URL.Query().Get("consolidation_set_id")
	periodIDStr := r.URL.Query().Get("fiscal_period_id")
	statusStr := getStringQuery(r, "status")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.ConsolidationRunFilter{
		Limit:  limit,
		Offset: offset,
	}

	if setIDStr != "" {
		setID, err := common.Parse(setIDStr)
		if err == nil {
			filter.ConsolidationSetID = &setID
		}
	}

	if periodIDStr != "" {
		periodID, err := common.Parse(periodIDStr)
		if err == nil {
			filter.FiscalPeriodID = &periodID
		}
	}

	if statusStr != nil {
		status := domain.RunStatus(*statusStr)
		filter.Status = &status
	}

	runs, err := h.consolidationService.ListConsolidationRuns(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list consolidation runs", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ConsolidationRunResponse, len(runs))
	for i, run := range runs {
		response[i] = toRunResponse(&run)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ConsolidationRunHandler) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	balances, err := h.consolidationService.GetConsolidatedTrialBalance(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get trial balance", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ConsolidatedBalanceResponse, len(balances))
	for i, balance := range balances {
		response[i] = ConsolidatedBalanceResponse{
			AccountID:             balance.AccountID,
			AccountCode:           balance.AccountCode,
			AccountName:           balance.AccountName,
			AccountType:           balance.AccountType,
			OpeningBalance:        balance.OpeningBalance,
			PeriodDebit:           balance.PeriodDebit,
			PeriodCredit:          balance.PeriodCredit,
			EliminationDebit:      balance.EliminationDebit,
			EliminationCredit:     balance.EliminationCredit,
			TranslationAdjustment: balance.TranslationAdjustment,
			MinorityInterest:      balance.MinorityInterest,
			ClosingBalance:        balance.ClosingBalance,
		}
	}

	respondJSON(w, http.StatusOK, response)
}

func toRunResponse(run *domain.ConsolidationRun) ConsolidationRunResponse {
	resp := ConsolidationRunResponse{
		ID:                    run.ID,
		RunNumber:             run.RunNumber,
		ConsolidationSetID:    run.ConsolidationSetID,
		ConsolidationSetCode:  run.ConsolidationSetCode,
		ConsolidationSetName:  run.ConsolidationSetName,
		FiscalPeriodID:        run.FiscalPeriodID,
		FiscalPeriodName:      run.FiscalPeriodName,
		ReportingCurrency:     run.ReportingCurrency.Code,
		ConsolidationDate:     run.ConsolidationDate.Format("2006-01-02"),
		ClosingRateDate:       run.ClosingRateDate.Format("2006-01-02"),
		EntityCount:           run.EntityCount,
		TotalAssets:           run.TotalAssets,
		TotalLiabilities:      run.TotalLiabilities,
		TotalEquity:           run.TotalEquity,
		TotalRevenue:          run.TotalRevenue,
		TotalExpenses:         run.TotalExpenses,
		NetIncome:             run.NetIncome,
		TotalCTA:              run.TotalCTA,
		TotalMinorityInterest: run.TotalMinorityInterest,
		Status:                string(run.Status),
		JournalEntryID:        run.JournalEntryID,
		CreatedAt:             run.CreatedAt,
		CompletedAt:           run.CompletedAt,
		PostedAt:              run.PostedAt,
	}

	if run.AverageRateDate != nil {
		avgDate := run.AverageRateDate.Format("2006-01-02")
		resp.AverageRateDate = &avgDate
	}

	return resp
}
