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

type AllocationHandler struct {
	*Handler
	allocationService *service.AllocationService
}

func NewAllocationHandler(logger *zap.Logger, svc *service.AllocationService) *AllocationHandler {
	return &AllocationHandler{Handler: NewHandler(logger), allocationService: svc}
}

type CreateAllocationRuleRequest struct {
	RuleCode           string `json:"rule_code"`
	RuleName           string `json:"rule_name"`
	SourceCostCenterID string `json:"source_cost_center_id"`
	AllocationMethod   string `json:"allocation_method"`
}

type AddAllocationTargetRequest struct {
	TargetCostCenterID string   `json:"target_cost_center_id"`
	FixedPercent       *float64 `json:"fixed_percent,omitempty"`
	DriverValue        *float64 `json:"driver_value,omitempty"`
}

type InitiateAllocationRunRequest struct {
	FiscalPeriodID string `json:"fiscal_period_id"`
	AllocationDate string `json:"allocation_date"`
	Currency       string `json:"currency"`
}

type AllocationRuleResponse struct {
	ID                   common.ID                  `json:"id"`
	EntityID             common.ID                  `json:"entity_id"`
	RuleCode             string                     `json:"rule_code"`
	RuleName             string                     `json:"rule_name"`
	SourceCostCenterID   common.ID                  `json:"source_cost_center_id"`
	SourceCostCenterCode string                     `json:"source_cost_center_code,omitempty"`
	AllocationMethod     string                     `json:"allocation_method"`
	SequenceNumber       int                        `json:"sequence_number"`
	IsActive             bool                       `json:"is_active"`
	Targets              []AllocationTargetResponse `json:"targets,omitempty"`
	CreatedAt            time.Time                  `json:"created_at"`
}

type AllocationTargetResponse struct {
	ID                   common.ID `json:"id"`
	TargetCostCenterID   common.ID `json:"target_cost_center_id"`
	TargetCostCenterCode string    `json:"target_cost_center_code,omitempty"`
	FixedPercent         *float64  `json:"fixed_percent,omitempty"`
	DriverValue          *float64  `json:"driver_value,omitempty"`
	IsActive             bool      `json:"is_active"`
}

type AllocationRunResponse struct {
	ID              common.ID   `json:"id"`
	EntityID        common.ID   `json:"entity_id"`
	RunNumber       string      `json:"run_number"`
	FiscalPeriodID  common.ID   `json:"fiscal_period_id"`
	AllocationDate  string      `json:"allocation_date"`
	RulesExecuted   int         `json:"rules_executed"`
	TotalAllocated  money.Money `json:"total_allocated"`
	Status          string      `json:"status"`
	JournalEntryID  *common.ID  `json:"journal_entry_id,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	CompletedAt     *time.Time  `json:"completed_at,omitempty"`
	PostedAt        *time.Time  `json:"posted_at,omitempty"`
}

func (h *AllocationHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req CreateAllocationRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	method := domain.AllocationMethod(req.AllocationMethod)
	if !method.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid allocation method")
		return
	}

	sourceCostCenterID, err := common.Parse(req.SourceCostCenterID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid source cost center ID")
		return
	}

	rule, err := h.allocationService.CreateAllocationRule(r.Context(), entityID, req.RuleCode, req.RuleName, sourceCostCenterID, method)
	if err != nil {
		h.logger.Error("failed to create allocation rule", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toAllocationRuleResponse(rule))
}

func (h *AllocationHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}

	rule, err := h.allocationService.GetAllocationRule(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "allocation rule not found")
		return
	}

	respondJSON(w, http.StatusOK, toAllocationRuleResponse(rule))
}

func (h *AllocationHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	isActive := getBoolQuery(r, "is_active")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.AllocationRuleFilter{
		EntityID: &entityID,
		IsActive: isActive,
		Limit:    limit,
		Offset:   offset,
	}

	rules, err := h.allocationService.ListAllocationRules(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list allocation rules", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]AllocationRuleResponse, len(rules))
	for i, rule := range rules {
		response[i] = toAllocationRuleResponse(&rule)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *AllocationHandler) AddTarget(w http.ResponseWriter, r *http.Request) {
	ruleID, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}

	var req AddAllocationTargetRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	targetCostCenterID, err := common.Parse(req.TargetCostCenterID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid target cost center ID")
		return
	}

	target, err := h.allocationService.AddAllocationTarget(r.Context(), ruleID, targetCostCenterID, req.FixedPercent, req.DriverValue)
	if err != nil {
		h.logger.Error("failed to add allocation target", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toAllocationTargetResponse(target))
}

func (h *AllocationHandler) InitiateRun(w http.ResponseWriter, r *http.Request) {
	var req InitiateAllocationRunRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal period ID")
		return
	}

	allocationDate, err := time.Parse("2006-01-02", req.AllocationDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid allocation date")
		return
	}

	currency := money.MustGetCurrency(req.Currency)

	run, err := h.allocationService.InitiateAllocationRun(r.Context(), entityID, fiscalPeriodID, allocationDate, currency)
	if err != nil {
		h.logger.Error("failed to initiate allocation run", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toAllocationRunResponse(run))
}

func (h *AllocationHandler) ExecuteRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := h.allocationService.ExecuteAllocation(r.Context(), id); err != nil {
		h.logger.Error("failed to execute allocation", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	run, _ := h.allocationService.GetAllocationRun(r.Context(), id)
	respondJSON(w, http.StatusOK, toAllocationRunResponse(run))
}

func (h *AllocationHandler) PostRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	if err := h.allocationService.PostAllocation(r.Context(), id); err != nil {
		h.logger.Error("failed to post allocation", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	run, _ := h.allocationService.GetAllocationRun(r.Context(), id)
	respondJSON(w, http.StatusOK, toAllocationRunResponse(run))
}

func (h *AllocationHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	run, err := h.allocationService.GetAllocationRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "allocation run not found")
		return
	}

	respondJSON(w, http.StatusOK, toAllocationRunResponse(run))
}

func (h *AllocationHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	statusStr := getStringQuery(r, "status")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.AllocationRunFilter{
		EntityID: &entityID,
		Limit:    limit,
		Offset:   offset,
	}

	if statusStr != nil {
		status := domain.AllocationStatus(*statusStr)
		filter.Status = &status
	}

	runs, err := h.allocationService.ListAllocationRuns(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list allocation runs", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]AllocationRunResponse, len(runs))
	for i, run := range runs {
		response[i] = toAllocationRunResponse(&run)
	}

	respondJSON(w, http.StatusOK, response)
}

func toAllocationRuleResponse(r *domain.AllocationRule) AllocationRuleResponse {
	resp := AllocationRuleResponse{
		ID:                   r.ID,
		EntityID:             r.EntityID,
		RuleCode:             r.RuleCode,
		RuleName:             r.RuleName,
		SourceCostCenterID:   r.SourceCostCenterID,
		SourceCostCenterCode: r.SourceCostCenterCode,
		AllocationMethod:     string(r.AllocationMethod),
		SequenceNumber:       r.SequenceNumber,
		IsActive:             r.IsActive,
		CreatedAt:            r.CreatedAt,
	}

	if len(r.Targets) > 0 {
		resp.Targets = make([]AllocationTargetResponse, len(r.Targets))
		for i, t := range r.Targets {
			resp.Targets[i] = toAllocationTargetResponse(&t)
		}
	}

	return resp
}

func toAllocationTargetResponse(t *domain.AllocationTarget) AllocationTargetResponse {
	return AllocationTargetResponse{
		ID:                   t.ID,
		TargetCostCenterID:   t.TargetCostCenterID,
		TargetCostCenterCode: t.TargetCostCenterCode,
		FixedPercent:         t.FixedPercent,
		DriverValue:          t.DriverValue,
		IsActive:             t.IsActive,
	}
}

func toAllocationRunResponse(r *domain.AllocationRun) AllocationRunResponse {
	return AllocationRunResponse{
		ID:             r.ID,
		EntityID:       r.EntityID,
		RunNumber:      r.RunNumber,
		FiscalPeriodID: r.FiscalPeriodID,
		AllocationDate: r.AllocationDate.Format("2006-01-02"),
		RulesExecuted:  r.RulesExecuted,
		TotalAllocated: r.TotalAllocated,
		Status:         string(r.Status),
		JournalEntryID: r.JournalEntryID,
		CreatedAt:      r.CreatedAt,
		CompletedAt:    r.CompletedAt,
		PostedAt:       r.PostedAt,
	}
}
