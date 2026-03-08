package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
	"converge-finance.com/m/internal/modules/cost/internal/service"
	"go.uber.org/zap"
)

type BudgetHandler struct {
	*Handler
	budgetService *service.BudgetService
}

func NewBudgetHandler(logger *zap.Logger, svc *service.BudgetService) *BudgetHandler {
	return &BudgetHandler{Handler: NewHandler(logger), budgetService: svc}
}

type CreateBudgetRequest struct {
	BudgetCode   string `json:"budget_code"`
	BudgetName   string `json:"budget_name"`
	BudgetType   string `json:"budget_type"`
	FiscalYearID string `json:"fiscal_year_id"`
	Currency     string `json:"currency"`
}

type AddBudgetLineRequest struct {
	AccountID      string     `json:"account_id"`
	FiscalPeriodID string     `json:"fiscal_period_id"`
	CostCenterID   *string    `json:"cost_center_id,omitempty"`
	BudgetAmount   float64    `json:"budget_amount"`
	Quantity       *float64   `json:"quantity,omitempty"`
	UnitCost       *float64   `json:"unit_cost,omitempty"`
	Notes          string     `json:"notes,omitempty"`
}

type BudgetResponse struct {
	ID               common.ID   `json:"id"`
	EntityID         common.ID   `json:"entity_id"`
	BudgetCode       string      `json:"budget_code"`
	BudgetName       string      `json:"budget_name"`
	BudgetType       string      `json:"budget_type"`
	FiscalYearID     common.ID   `json:"fiscal_year_id"`
	VersionNumber    int         `json:"version_number"`
	IsCurrentVersion bool        `json:"is_current_version"`
	Currency         string      `json:"currency"`
	TotalRevenue     money.Money `json:"total_revenue"`
	TotalExpenses    money.Money `json:"total_expenses"`
	NetBudget        money.Money `json:"net_budget"`
	Status           string      `json:"status"`
	CreatedAt        time.Time   `json:"created_at"`
}

type BudgetLineResponse struct {
	ID               common.ID   `json:"id"`
	AccountID        common.ID   `json:"account_id"`
	AccountCode      string      `json:"account_code"`
	AccountName      string      `json:"account_name"`
	CostCenterID     *common.ID  `json:"cost_center_id,omitempty"`
	CostCenterCode   string      `json:"cost_center_code,omitempty"`
	FiscalPeriodID   common.ID   `json:"fiscal_period_id"`
	FiscalPeriodName string      `json:"fiscal_period_name,omitempty"`
	BudgetAmount     money.Money `json:"budget_amount"`
	Quantity         *float64    `json:"quantity,omitempty"`
	UnitCost         *float64    `json:"unit_cost,omitempty"`
	Notes            string      `json:"notes,omitempty"`
}

type VarianceResponse struct {
	AccountID        common.ID   `json:"account_id"`
	AccountCode      string      `json:"account_code"`
	AccountName      string      `json:"account_name"`
	CostCenterID     *common.ID  `json:"cost_center_id,omitempty"`
	CostCenterCode   string      `json:"cost_center_code,omitempty"`
	BudgetAmount     money.Money `json:"budget_amount"`
	ActualAmount     money.Money `json:"actual_amount"`
	VarianceAmount   money.Money `json:"variance_amount"`
	VariancePercent  float64     `json:"variance_percent"`
	IsFavorable      bool        `json:"is_favorable"`
}

func (h *BudgetHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	budgetType := domain.BudgetType(req.BudgetType)
	if !budgetType.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid budget type")
		return
	}

	fiscalYearID, err := common.Parse(req.FiscalYearID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal year ID")
		return
	}

	currency := money.MustGetCurrency(req.Currency)

	budget, err := h.budgetService.CreateBudget(r.Context(), entityID, req.BudgetCode, req.BudgetName, budgetType, fiscalYearID, currency)
	if err != nil {
		h.logger.Error("failed to create budget", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toBudgetResponse(budget))
}

func (h *BudgetHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	budget, err := h.budgetService.GetBudget(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "budget not found")
		return
	}

	respondJSON(w, http.StatusOK, toBudgetResponse(budget))
}

func (h *BudgetHandler) List(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	statusStr := getStringQuery(r, "status")
	typeStr := getStringQuery(r, "budget_type")
	currentOnly := getBoolQuery(r, "current_only")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.BudgetFilter{
		EntityID: &entityID,
		Limit:    limit,
		Offset:   offset,
	}

	if statusStr != nil {
		status := domain.BudgetStatus(*statusStr)
		filter.Status = &status
	}
	if typeStr != nil {
		budgetType := domain.BudgetType(*typeStr)
		filter.BudgetType = &budgetType
	}
	if currentOnly != nil && *currentOnly {
		filter.CurrentOnly = true
	}

	budgets, err := h.budgetService.ListBudgets(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list budgets", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]BudgetResponse, len(budgets))
	for i, budget := range budgets {
		response[i] = toBudgetResponse(&budget)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *BudgetHandler) AddLine(w http.ResponseWriter, r *http.Request) {
	budgetID, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	var req AddBudgetLineRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	accountID, _ := common.Parse(req.AccountID)
	fiscalPeriodID, _ := common.Parse(req.FiscalPeriodID)

	budget, err := h.budgetService.GetBudget(r.Context(), budgetID)
	if err != nil {
		respondError(w, http.StatusNotFound, "budget not found")
		return
	}

	budgetAmount := money.New(req.BudgetAmount, budget.Currency.Code)

	var costCenterID *common.ID
	if req.CostCenterID != nil {
		id, _ := common.Parse(*req.CostCenterID)
		costCenterID = &id
	}

	line, err := h.budgetService.AddBudgetLine(r.Context(), budgetID, accountID, fiscalPeriodID, costCenterID, budgetAmount, req.Quantity, req.UnitCost, req.Notes)
	if err != nil {
		h.logger.Error("failed to add budget line", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toBudgetLineResponse(line))
}

func (h *BudgetHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	if err := h.budgetService.SubmitBudget(r.Context(), id); err != nil {
		h.logger.Error("failed to submit budget", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "budget submitted"})
}

func (h *BudgetHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	if err := h.budgetService.ApproveBudget(r.Context(), id); err != nil {
		h.logger.Error("failed to approve budget", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "budget approved"})
}

func (h *BudgetHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.budgetService.RejectBudget(r.Context(), id, req.Reason); err != nil {
		h.logger.Error("failed to reject budget", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "budget rejected"})
}

func (h *BudgetHandler) GetVariance(w http.ResponseWriter, r *http.Request) {
	budgetID, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid budget ID")
		return
	}

	periodIDStr := r.URL.Query().Get("fiscal_period_id")
	if periodIDStr == "" {
		respondError(w, http.StatusBadRequest, "fiscal_period_id is required")
		return
	}

	fiscalPeriodID, err := common.Parse(periodIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal period ID")
		return
	}

	summary, analyses, err := h.budgetService.GetVarianceAnalysis(r.Context(), budgetID, fiscalPeriodID)
	if err != nil {
		h.logger.Error("failed to get variance analysis", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	variances := make([]VarianceResponse, len(analyses))
	for i, a := range analyses {
		variances[i] = VarianceResponse{
			AccountID:       a.AccountID,
			AccountCode:     a.AccountCode,
			AccountName:     a.AccountName,
			CostCenterID:    a.CostCenterID,
			CostCenterCode:  a.CostCenterCode,
			BudgetAmount:    a.BudgetAmount,
			ActualAmount:    a.ActualAmount,
			VarianceAmount:  a.VarianceAmount,
			VariancePercent: a.VariancePercent,
			IsFavorable:     a.IsFavorable,
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"summary":   summary,
		"variances": variances,
	})
}

func toBudgetResponse(b *domain.Budget) BudgetResponse {
	return BudgetResponse{
		ID:               b.ID,
		EntityID:         b.EntityID,
		BudgetCode:       b.BudgetCode,
		BudgetName:       b.BudgetName,
		BudgetType:       string(b.BudgetType),
		FiscalYearID:     b.FiscalYearID,
		VersionNumber:    b.VersionNumber,
		IsCurrentVersion: b.IsCurrentVersion,
		Currency:         b.Currency.Code,
		TotalRevenue:     b.TotalRevenue,
		TotalExpenses:    b.TotalExpenses,
		NetBudget:        b.NetBudget,
		Status:           string(b.Status),
		CreatedAt:        b.CreatedAt,
	}
}

func toBudgetLineResponse(l *domain.BudgetLine) BudgetLineResponse {
	return BudgetLineResponse{
		ID:               l.ID,
		AccountID:        l.AccountID,
		AccountCode:      l.AccountCode,
		AccountName:      l.AccountName,
		CostCenterID:     l.CostCenterID,
		CostCenterCode:   l.CostCenterCode,
		FiscalPeriodID:   l.FiscalPeriodID,
		FiscalPeriodName: l.FiscalPeriodName,
		BudgetAmount:     l.BudgetAmount,
		Quantity:         l.Quantity,
		UnitCost:         l.UnitCost,
		Notes:            l.Notes,
	}
}
