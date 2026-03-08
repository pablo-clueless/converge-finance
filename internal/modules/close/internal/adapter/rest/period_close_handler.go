package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/service"
	"go.uber.org/zap"
)

type PeriodCloseHandler struct {
	*Handler
	periodCloseService *service.PeriodCloseService
}

func NewPeriodCloseHandler(logger *zap.Logger, svc *service.PeriodCloseService) *PeriodCloseHandler {
	return &PeriodCloseHandler{Handler: NewHandler(logger), periodCloseService: svc}
}

type SoftCloseRequest struct {
	FiscalPeriodID string `json:"fiscal_period_id"`
}

type HardCloseRequest struct {
	FiscalPeriodID string `json:"fiscal_period_id"`
	FiscalYearID   string `json:"fiscal_year_id"`
	CloseDate      string `json:"close_date"`
	Currency       string `json:"currency"`
}

type ReopenRequest struct {
	FiscalPeriodID string `json:"fiscal_period_id"`
	Reason         string `json:"reason"`
}

type CreateCloseRuleRequest struct {
	RuleCode        string `json:"rule_code"`
	RuleName        string `json:"rule_name"`
	RuleType        string `json:"rule_type"`
	CloseType       string `json:"close_type"`
	TargetAccountID string `json:"target_account_id"`
}

type PeriodCloseResponse struct {
	ID                    common.ID  `json:"id"`
	EntityID              common.ID  `json:"entity_id"`
	FiscalPeriodID        common.ID  `json:"fiscal_period_id"`
	FiscalYearID          common.ID  `json:"fiscal_year_id"`
	Status                string     `json:"status"`
	SoftClosedAt          *time.Time `json:"soft_closed_at,omitempty"`
	HardClosedAt          *time.Time `json:"hard_closed_at,omitempty"`
	ClosingJournalEntryID *common.ID `json:"closing_journal_entry_id,omitempty"`
	Notes                 string     `json:"notes,omitempty"`
}

type CloseRuleResponse struct {
	ID                common.ID  `json:"id"`
	EntityID          common.ID  `json:"entity_id"`
	RuleCode          string     `json:"rule_code"`
	RuleName          string     `json:"rule_name"`
	RuleType          string     `json:"rule_type"`
	CloseType         string     `json:"close_type"`
	SequenceNumber    int        `json:"sequence_number"`
	SourceAccountType string     `json:"source_account_type,omitempty"`
	SourceAccountID   *common.ID `json:"source_account_id,omitempty"`
	TargetAccountID   common.ID  `json:"target_account_id"`
	IsActive          bool       `json:"is_active"`
	CreatedAt         time.Time  `json:"created_at"`
}

type CloseRunResponse struct {
	ID                    common.ID    `json:"id"`
	EntityID              common.ID    `json:"entity_id"`
	RunNumber             string       `json:"run_number"`
	CloseType             string       `json:"close_type"`
	FiscalPeriodID        common.ID    `json:"fiscal_period_id"`
	CloseDate             string       `json:"close_date"`
	Status                string       `json:"status"`
	RulesExecuted         int          `json:"rules_executed"`
	EntriesCreated        int          `json:"entries_created"`
	TotalDebits           money.Money  `json:"total_debits"`
	TotalCredits          money.Money  `json:"total_credits"`
	ClosingJournalEntryID *common.ID   `json:"closing_journal_entry_id,omitempty"`
	CompletedAt           *time.Time   `json:"completed_at,omitempty"`
	CreatedAt             time.Time    `json:"created_at"`
}

func (h *PeriodCloseHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	fiscalPeriodIDStr := r.URL.Query().Get("fiscal_period_id")

	if fiscalPeriodIDStr == "" {
		respondError(w, http.StatusBadRequest, "fiscal_period_id is required")
		return
	}

	fiscalPeriodID, err := common.Parse(fiscalPeriodIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	pc, err := h.periodCloseService.GetPeriodCloseStatus(r.Context(), entityID, fiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusNotFound, "period close status not found")
		return
	}

	respondJSON(w, http.StatusOK, toPeriodCloseResponse(pc))
}

func (h *PeriodCloseHandler) ListStatuses(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	fiscalYearIDStr := getStringQuery(r, "fiscal_year_id")
	statusStr := getStringQuery(r, "status")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.PeriodCloseFilter{
		EntityID: &entityID,
		Limit:    limit,
		Offset:   offset,
	}

	if fiscalYearIDStr != nil {
		fiscalYearID, _ := common.Parse(*fiscalYearIDStr)
		filter.FiscalYearID = &fiscalYearID
	}

	if statusStr != nil {
		status := domain.PeriodCloseStatus(*statusStr)
		filter.Status = &status
	}

	statuses, err := h.periodCloseService.ListPeriodCloseStatuses(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list period close statuses", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]PeriodCloseResponse, len(statuses))
	for i, pc := range statuses {
		response[i] = *toPeriodCloseResponse(&pc)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *PeriodCloseHandler) SoftClose(w http.ResponseWriter, r *http.Request) {
	var req SoftCloseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	userID := getUserID(r)

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	pc, err := h.periodCloseService.SoftClosePeriod(r.Context(), entityID, fiscalPeriodID, userID)
	if err != nil {
		h.logger.Error("failed to soft close period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toPeriodCloseResponse(pc))
}

func (h *PeriodCloseHandler) HardClose(w http.ResponseWriter, r *http.Request) {
	var req HardCloseRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	userID := getUserID(r)

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	fiscalYearID, err := common.Parse(req.FiscalYearID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_year_id")
		return
	}

	closeDate, err := time.Parse("2006-01-02", req.CloseDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid close_date")
		return
	}

	currency := money.MustGetCurrency(req.Currency)

	run, err := h.periodCloseService.HardClosePeriod(r.Context(), entityID, fiscalPeriodID, fiscalYearID, closeDate, currency, userID)
	if err != nil {
		h.logger.Error("failed to hard close period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toCloseRunResponse(run))
}

func (h *PeriodCloseHandler) Reopen(w http.ResponseWriter, r *http.Request) {
	var req ReopenRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	userID := getUserID(r)

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	pc, err := h.periodCloseService.ReopenPeriod(r.Context(), entityID, fiscalPeriodID, userID, req.Reason)
	if err != nil {
		h.logger.Error("failed to reopen period", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toPeriodCloseResponse(pc))
}

func (h *PeriodCloseHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req CreateCloseRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)

	ruleType := domain.CloseRuleType(req.RuleType)
	if !ruleType.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid rule_type")
		return
	}

	closeType := domain.CloseType(req.CloseType)
	if !closeType.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid close_type")
		return
	}

	targetAccountID, err := common.Parse(req.TargetAccountID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid target_account_id")
		return
	}

	rule, err := h.periodCloseService.CreateCloseRule(r.Context(), entityID, req.RuleCode, req.RuleName, ruleType, closeType, targetAccountID)
	if err != nil {
		h.logger.Error("failed to create close rule", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toCloseRuleResponse(rule))
}

func (h *PeriodCloseHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}

	rule, err := h.periodCloseService.GetCloseRule(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "close rule not found")
		return
	}

	respondJSON(w, http.StatusOK, toCloseRuleResponse(rule))
}

func (h *PeriodCloseHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	isActive := getBoolQuery(r, "is_active")
	closeTypeStr := getStringQuery(r, "close_type")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.CloseRuleFilter{
		EntityID: &entityID,
		IsActive: isActive,
		Limit:    limit,
		Offset:   offset,
	}

	if closeTypeStr != nil {
		closeType := domain.CloseType(*closeTypeStr)
		filter.CloseType = &closeType
	}

	rules, err := h.periodCloseService.ListCloseRules(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list close rules", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]CloseRuleResponse, len(rules))
	for i, rule := range rules {
		response[i] = *toCloseRuleResponse(&rule)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *PeriodCloseHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	run, err := h.periodCloseService.GetCloseRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "close run not found")
		return
	}

	respondJSON(w, http.StatusOK, toCloseRunResponse(run))
}

func (h *PeriodCloseHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	statusStr := getStringQuery(r, "status")
	closeTypeStr := getStringQuery(r, "close_type")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.CloseRunFilter{
		EntityID: &entityID,
		Limit:    limit,
		Offset:   offset,
	}

	if statusStr != nil {
		status := domain.CloseRunStatus(*statusStr)
		filter.Status = &status
	}

	if closeTypeStr != nil {
		closeType := domain.CloseType(*closeTypeStr)
		filter.CloseType = &closeType
	}

	runs, err := h.periodCloseService.ListCloseRuns(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list close runs", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]CloseRunResponse, len(runs))
	for i, run := range runs {
		response[i] = *toCloseRunResponse(&run)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *PeriodCloseHandler) ReverseRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid run ID")
		return
	}

	userID := getUserID(r)

	run, err := h.periodCloseService.ReverseCloseRun(r.Context(), id, userID)
	if err != nil {
		h.logger.Error("failed to reverse close run", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toCloseRunResponse(run))
}

func toPeriodCloseResponse(pc *domain.PeriodClose) *PeriodCloseResponse {
	return &PeriodCloseResponse{
		ID:                    pc.ID,
		EntityID:              pc.EntityID,
		FiscalPeriodID:        pc.FiscalPeriodID,
		FiscalYearID:          pc.FiscalYearID,
		Status:                string(pc.Status),
		SoftClosedAt:          pc.SoftClosedAt,
		HardClosedAt:          pc.HardClosedAt,
		ClosingJournalEntryID: pc.ClosingJournalEntryID,
		Notes:                 pc.Notes,
	}
}

func toCloseRuleResponse(rule *domain.CloseRule) *CloseRuleResponse {
	return &CloseRuleResponse{
		ID:                rule.ID,
		EntityID:          rule.EntityID,
		RuleCode:          rule.RuleCode,
		RuleName:          rule.RuleName,
		RuleType:          string(rule.RuleType),
		CloseType:         string(rule.CloseType),
		SequenceNumber:    rule.SequenceNumber,
		SourceAccountType: rule.SourceAccountType,
		SourceAccountID:   rule.SourceAccountID,
		TargetAccountID:   rule.TargetAccountID,
		IsActive:          rule.IsActive,
		CreatedAt:         rule.CreatedAt,
	}
}

func toCloseRunResponse(run *domain.CloseRun) *CloseRunResponse {
	return &CloseRunResponse{
		ID:                    run.ID,
		EntityID:              run.EntityID,
		RunNumber:             run.RunNumber,
		CloseType:             string(run.CloseType),
		FiscalPeriodID:        run.FiscalPeriodID,
		CloseDate:             run.CloseDate.Format("2006-01-02"),
		Status:                string(run.Status),
		RulesExecuted:         run.RulesExecuted,
		EntriesCreated:        run.EntriesCreated,
		TotalDebits:           run.TotalDebits,
		TotalCredits:          run.TotalCredits,
		ClosingJournalEntryID: run.ClosingJournalEntryID,
		CompletedAt:           run.CompletedAt,
		CreatedAt:             run.CreatedAt,
	}
}
