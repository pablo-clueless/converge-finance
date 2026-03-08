package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/service"
	"go.uber.org/zap"
)

type CashFlowHandler struct {
	*Handler
	service *service.CashFlowService
}

func NewCashFlowHandler(logger *zap.Logger, svc *service.CashFlowService) *CashFlowHandler {
	return &CashFlowHandler{
		Handler: NewHandler(logger),
		service: svc,
	}
}

type configureAccountCashFlowRequest struct {
	AccountID        string `json:"account_id"`
	Category         string `json:"category"`
	LineItemCode     string `json:"line_item_code"`
	IsCashAccount    bool   `json:"is_cash_account"`
	IsCashEquivalent bool   `json:"is_cash_equivalent"`
	AdjustmentType   string `json:"adjustment_type,omitempty"`
}

func (h *CashFlowHandler) ConfigureAccountCashFlow(w http.ResponseWriter, r *http.Request) {
	var req configureAccountCashFlowRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	accountID, err := common.Parse(req.AccountID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid account ID")
		return
	}

	config, err := h.service.ConfigureAccountCashFlow(
		r.Context(),
		entityID,
		accountID,
		domain.CashFlowCategory(req.Category),
		req.LineItemCode,
		req.IsCashAccount,
		req.IsCashEquivalent,
		req.AdjustmentType,
	)
	if err != nil {
		h.logger.Error("Failed to configure account cash flow", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, config)
}

func (h *CashFlowHandler) ListAccountCashFlowConfigs(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	configs, err := h.service.ListAccountCashFlowConfigs(r.Context(), entityID)
	if err != nil {
		h.logger.Error("Failed to list account cash flow configs", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, configs)
}

func (h *CashFlowHandler) DeleteAccountCashFlowConfig(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	if err := h.service.DeleteCashFlowConfig(r.Context(), id); err != nil {
		h.logger.Error("Failed to delete account cash flow config", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type createCashFlowTemplateRequest struct {
	TemplateCode string `json:"template_code"`
	TemplateName string `json:"template_name"`
	Method       string `json:"method"`
}

func (h *CashFlowHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req createCashFlowTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	template, err := h.service.CreateTemplate(
		r.Context(),
		entityID,
		req.TemplateCode,
		req.TemplateName,
		domain.CashFlowMethod(req.Method),
	)
	if err != nil {
		h.logger.Error("Failed to create cash flow template", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, template)
}

func (h *CashFlowHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid template ID")
		return
	}

	template, err := h.service.GetTemplate(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get cash flow template", zap.Error(err))
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, template)
}

func (h *CashFlowHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	templates, err := h.service.ListTemplates(r.Context(), entityID)
	if err != nil {
		h.logger.Error("Failed to list cash flow templates", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, templates)
}

type generateCashFlowRequest struct {
	FiscalPeriodID string `json:"fiscal_period_id"`
	FiscalYearID   string `json:"fiscal_year_id"`
	Method         string `json:"method"`
	PeriodStart    string `json:"period_start"`
	PeriodEnd      string `json:"period_end"`
	CurrencyCode   string `json:"currency_code"`
}

func (h *CashFlowHandler) GenerateCashFlowStatement(w http.ResponseWriter, r *http.Request) {
	var req generateCashFlowRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	userID := getUserID(r)
	if userID.IsZero() {
		respondError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid fiscal period ID")
		return
	}

	fiscalYearID, err := common.Parse(req.FiscalYearID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid fiscal year ID")
		return
	}

	periodStart, err := time.Parse(time.RFC3339, req.PeriodStart)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid period start date")
		return
	}

	periodEnd, err := time.Parse(time.RFC3339, req.PeriodEnd)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid period end date")
		return
	}

	run, err := h.service.GenerateCashFlowStatement(
		r.Context(),
		entityID,
		fiscalPeriodID,
		fiscalYearID,
		domain.CashFlowMethod(req.Method),
		periodStart,
		periodEnd,
		req.CurrencyCode,
		userID,
	)
	if err != nil {
		h.logger.Error("Failed to generate cash flow statement", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, run)
}

func (h *CashFlowHandler) GetCashFlowRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid run ID")
		return
	}

	run, err := h.service.GetCashFlowRun(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get cash flow run", zap.Error(err))
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (h *CashFlowHandler) ListCashFlowRuns(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	if entityID.IsZero() {
		respondError(w, http.StatusBadRequest, "Entity ID is required")
		return
	}

	var fiscalPeriodID, fiscalYearID common.ID
	if fpID := r.URL.Query().Get("fiscal_period_id"); fpID != "" {
		fiscalPeriodID, _ = common.Parse(fpID)
	}
	if fyID := r.URL.Query().Get("fiscal_year_id"); fyID != "" {
		fiscalYearID, _ = common.Parse(fyID)
	}

	var status *domain.CashFlowRunStatus
	if s := getStringQuery(r, "status"); s != nil {
		st := domain.CashFlowRunStatus(*s)
		status = &st
	}

	limit := getIntQuery(r, "limit", 20)
	offset := getIntQuery(r, "offset", 0)

	runs, total, err := h.service.ListCashFlowRuns(
		r.Context(),
		entityID,
		fiscalPeriodID,
		fiscalYearID,
		status,
		limit,
		offset,
	)
	if err != nil {
		h.logger.Error("Failed to list cash flow runs", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]any{
		"runs":  runs,
		"total": total,
	}

	respondJSON(w, http.StatusOK, response)
}
