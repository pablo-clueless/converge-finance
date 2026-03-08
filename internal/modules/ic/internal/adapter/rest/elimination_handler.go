package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
	"converge-finance.com/m/internal/modules/ic/internal/service"
)

type EliminationHandler struct {
	*Handler
	elimService *service.EliminationService
}

func NewEliminationHandler(h *Handler, elimService *service.EliminationService) *EliminationHandler {
	return &EliminationHandler{
		Handler:     h,
		elimService: elimService,
	}
}

type EliminationRuleResponse struct {
	ID              string    `json:"id"`
	ParentEntityID  string    `json:"parent_entity_id"`
	RuleCode        string    `json:"rule_code"`
	RuleName        string    `json:"rule_name"`
	EliminationType string    `json:"elimination_type"`
	Description     string    `json:"description,omitempty"`
	SequenceNumber  int       `json:"sequence_number"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func toRuleResponse(r *domain.EliminationRule) EliminationRuleResponse {
	return EliminationRuleResponse{
		ID:              r.ID.String(),
		ParentEntityID:  r.ParentEntityID.String(),
		RuleCode:        r.RuleCode,
		RuleName:        r.RuleName,
		EliminationType: string(r.EliminationType),
		Description:     r.Description,
		SequenceNumber:  r.SequenceNumber,
		IsActive:        r.IsActive,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

type EliminationRunResponse struct {
	ID               string     `json:"id"`
	RunNumber        string     `json:"run_number"`
	ParentEntityID   string     `json:"parent_entity_id"`
	FiscalPeriodID   string     `json:"fiscal_period_id"`
	EliminationDate  string     `json:"elimination_date"`
	Currency         string     `json:"currency"`
	EntryCount       int        `json:"entry_count"`
	TotalElimination string     `json:"total_elimination"`
	Status           string     `json:"status"`
	JournalEntryID   string     `json:"journal_entry_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	PostedAt         *time.Time `json:"posted_at,omitempty"`
	ReversedAt       *time.Time `json:"reversed_at,omitempty"`
}

func toRunResponse(r *domain.EliminationRun) EliminationRunResponse {
	resp := EliminationRunResponse{
		ID:               r.ID.String(),
		RunNumber:        r.RunNumber,
		ParentEntityID:   r.ParentEntityID.String(),
		FiscalPeriodID:   r.FiscalPeriodID.String(),
		EliminationDate:  r.EliminationDate.Format("2006-01-02"),
		Currency:         r.Currency.Code,
		EntryCount:       r.EntryCount,
		TotalElimination: r.TotalEliminations.Amount.String(),
		Status:           string(r.Status),
		CreatedAt:        r.CreatedAt,
		PostedAt:         r.PostedAt,
		ReversedAt:       r.ReversedAt,
	}
	if r.JournalEntryID != nil {
		resp.JournalEntryID = r.JournalEntryID.String()
	}
	return resp
}

// Rules handlers

func (h *EliminationHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	parentEntityID := getEntityID(r)
	if parentEntityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	rules, err := h.elimService.ListRules(r.Context(), parentEntityID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []EliminationRuleResponse
	for _, rule := range rules {
		responses = append(responses, toRuleResponse(&rule))
	}

	respondJSON(w, http.StatusOK, responses)
}

type CreateRuleRequest struct {
	RuleCode        string `json:"rule_code"`
	RuleName        string `json:"rule_name"`
	EliminationType string `json:"elimination_type"`
	Description     string `json:"description,omitempty"`
	SequenceNumber  int    `json:"sequence_number"`
}

func (h *EliminationHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	parentEntityID := getEntityID(r)
	if parentEntityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var req CreateRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	createReq := service.CreateRuleRequest{
		ParentEntityID:  parentEntityID,
		RuleCode:        req.RuleCode,
		RuleName:        req.RuleName,
		EliminationType: domain.EliminationType(req.EliminationType),
		Description:     req.Description,
		SequenceNumber:  req.SequenceNumber,
	}

	rule, err := h.elimService.CreateRule(r.Context(), createReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toRuleResponse(rule))
}

func (h *EliminationHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	rule, err := h.elimService.GetRule(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "rule not found")
		return
	}

	respondJSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *EliminationHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	var req CreateRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updateReq := service.CreateRuleRequest{
		RuleName:       req.RuleName,
		Description:    req.Description,
		SequenceNumber: req.SequenceNumber,
	}

	rule, err := h.elimService.UpdateRule(r.Context(), id, updateReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toRuleResponse(rule))
}

func (h *EliminationHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.elimService.DeleteRule(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "rule deleted"})
}

// Run handlers

func (h *EliminationHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	parentEntityID := getEntityID(r)
	if parentEntityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var fiscalPeriodID *common.ID
	if fpid := r.URL.Query().Get("fiscal_period_id"); fpid != "" {
		id, _ := common.Parse(fpid)
		fiscalPeriodID = &id
	}

	var status *domain.EliminationStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domain.EliminationStatus(s)
		status = &st
	}

	filter := domain.EliminationRunFilter{
		ParentEntityID: &parentEntityID,
		FiscalPeriodID: fiscalPeriodID,
		Status:         status,
		Limit:          getIntQuery(r, "limit", 50),
		Offset:         getIntQuery(r, "offset", 0),
	}

	runs, total, err := h.elimService.ListEliminationRuns(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var responses []EliminationRunResponse
	for _, run := range runs {
		responses = append(responses, toRunResponse(&run))
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:    responses,
		Total:   total,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
		HasMore: int64(filter.Offset+len(responses)) < total,
	})
}

type GenerateEliminationsRequest struct {
	FiscalPeriodID  string `json:"fiscal_period_id"`
	EliminationDate string `json:"elimination_date"`
	Currency        string `json:"currency"`
}

func (h *EliminationHandler) Generate(w http.ResponseWriter, r *http.Request) {
	parentEntityID := getEntityID(r)
	if parentEntityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	var req GenerateEliminationsRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	elimDate, err := time.Parse("2006-01-02", req.EliminationDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid elimination_date")
		return
	}

	genReq := service.GenerateEliminationsRequest{
		ParentEntityID:  parentEntityID,
		FiscalPeriodID:  fiscalPeriodID,
		EliminationDate: elimDate,
		CurrencyCode:    req.Currency,
	}

	run, err := h.elimService.GenerateEliminations(r.Context(), genReq)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toRunResponse(run))
}

func (h *EliminationHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	run, err := h.elimService.GetEliminationRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "run not found")
		return
	}

	respondJSON(w, http.StatusOK, toRunResponse(run))
}

func (h *EliminationHandler) PostRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.elimService.PostEliminationRun(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	run, _ := h.elimService.GetEliminationRun(r.Context(), id)
	respondJSON(w, http.StatusOK, toRunResponse(run))
}

func (h *EliminationHandler) ReverseRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	run, err := h.elimService.ReverseEliminationRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toRunResponse(run))
}

func (h *EliminationHandler) DeleteRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid ID")
		return
	}

	if err := h.elimService.DeleteEliminationRun(r.Context(), id); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "run deleted"})
}
