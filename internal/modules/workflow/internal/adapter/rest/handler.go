package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/workflow/internal/domain"
	"converge-finance.com/m/internal/modules/workflow/internal/repository"
	"converge-finance.com/m/internal/modules/workflow/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// WorkflowHandler handles workflow definition HTTP requests
type WorkflowHandler struct {
	service *service.WorkflowService
	logger  *zap.Logger
}

// NewWorkflowHandler creates a new WorkflowHandler
func NewWorkflowHandler(svc *service.WorkflowService, logger *zap.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		service: svc,
		logger:  logger,
	}
}

// CreateWorkflowRequest is the request body for creating a workflow
type CreateWorkflowRequest struct {
	WorkflowCode string `json:"workflow_code"`
	WorkflowName string `json:"workflow_name"`
	Description  string `json:"description"`
	DocumentType string `json:"document_type"`
}

// CreateWorkflow handles POST /workflows
func (h *WorkflowHandler) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	workflow, err := h.service.CreateWorkflow(r.Context(), service.CreateWorkflowRequest{
		EntityID:     entityID,
		WorkflowCode: req.WorkflowCode,
		WorkflowName: req.WorkflowName,
		Description:  req.Description,
		DocumentType: req.DocumentType,
		CreatedBy:    userID,
	})
	if err != nil {
		h.logger.Error("failed to create workflow", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, workflow)
}

// GetWorkflow handles GET /workflows/{id}
func (h *WorkflowHandler) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	workflow, err := h.service.GetWorkflow(r.Context(), id)
	if err == domain.ErrWorkflowNotFound {
		h.writeError(w, http.StatusNotFound, "workflow not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, workflow)
}

// ListWorkflows handles GET /workflows
func (h *WorkflowHandler) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.WorkflowFilter{
		EntityID:    entityID,
		CurrentOnly: true,
		Limit:       50,
		Offset:      0,
	}

	if documentType := r.URL.Query().Get("document_type"); documentType != "" {
		filter.DocumentType = documentType
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := domain.WorkflowStatus(statusStr)
		filter.Status = &status
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	workflows, total, err := h.service.ListWorkflows(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   workflows,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// UpdateWorkflowRequest is the request body for updating a workflow
type UpdateWorkflowRequest struct {
	WorkflowName string `json:"workflow_name"`
	Description  string `json:"description"`
}

// UpdateWorkflow handles PUT /workflows/{id}
func (h *WorkflowHandler) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	var req UpdateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	workflow, err := h.service.UpdateWorkflow(r.Context(), service.UpdateWorkflowRequest{
		ID:           id,
		WorkflowName: req.WorkflowName,
		Description:  req.Description,
	})
	if err != nil {
		h.logger.Error("failed to update workflow", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, workflow)
}

// ActivateWorkflow handles POST /workflows/{id}/activate
func (h *WorkflowHandler) ActivateWorkflow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	workflow, err := h.service.ActivateWorkflow(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to activate workflow", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, workflow)
}

// DeactivateWorkflow handles POST /workflows/{id}/deactivate
func (h *WorkflowHandler) DeactivateWorkflow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	workflow, err := h.service.DeactivateWorkflow(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to deactivate workflow", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, workflow)
}

// ArchiveWorkflow handles POST /workflows/{id}/archive
func (h *WorkflowHandler) ArchiveWorkflow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	if err := h.service.ArchiveWorkflow(r.Context(), id); err != nil {
		h.logger.Error("failed to archive workflow", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

// AddStepRequest is the request body for adding a workflow step
type AddStepRequest struct {
	StepNumber          int      `json:"step_number"`
	StepName            string   `json:"step_name"`
	StepType            string   `json:"step_type"`
	ApproverType        string   `json:"approver_type"`
	ApproverID          string   `json:"approver_id,omitempty"`
	ApproverExpression  string   `json:"approver_expression,omitempty"`
	ThresholdMin        *float64 `json:"threshold_min,omitempty"`
	ThresholdMax        *float64 `json:"threshold_max,omitempty"`
	ThresholdCurrency   string   `json:"threshold_currency,omitempty"`
	RequiredApprovals   int      `json:"required_approvals,omitempty"`
	AllowSelfApproval   bool     `json:"allow_self_approval"`
	EscalationHours     *int     `json:"escalation_hours,omitempty"`
	EscalateToStep      *int     `json:"escalate_to_step,omitempty"`
	ConditionExpression string   `json:"condition_expression,omitempty"`
}

// AddStep handles POST /workflows/{id}/steps
func (h *WorkflowHandler) AddStep(w http.ResponseWriter, r *http.Request) {
	workflowIDStr := chi.URLParam(r, "id")
	workflowID := common.ID(workflowIDStr)

	var req AddStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	stepReq := service.AddStepRequest{
		WorkflowID:          workflowID,
		StepNumber:          req.StepNumber,
		StepName:            req.StepName,
		StepType:            domain.StepType(req.StepType),
		ApproverType:        domain.ApproverType(req.ApproverType),
		ApproverExpression:  req.ApproverExpression,
		RequiredApprovals:   req.RequiredApprovals,
		AllowSelfApproval:   req.AllowSelfApproval,
		EscalationHours:     req.EscalationHours,
		EscalateToStep:      req.EscalateToStep,
		ConditionExpression: req.ConditionExpression,
	}

	if req.ApproverID != "" {
		approverID := common.ID(req.ApproverID)
		stepReq.ApproverID = &approverID
	}

	if req.ThresholdMin != nil {
		min := decimal.NewFromFloat(*req.ThresholdMin)
		stepReq.ThresholdMin = &min
	}
	if req.ThresholdMax != nil {
		max := decimal.NewFromFloat(*req.ThresholdMax)
		stepReq.ThresholdMax = &max
	}
	stepReq.ThresholdCurrency = req.ThresholdCurrency

	step, err := h.service.AddStep(r.Context(), stepReq)
	if err != nil {
		h.logger.Error("failed to add step", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, step)
}

// RemoveStep handles DELETE /workflows/{id}/steps/{stepId}
func (h *WorkflowHandler) RemoveStep(w http.ResponseWriter, r *http.Request) {
	workflowIDStr := chi.URLParam(r, "id")
	workflowID := common.ID(workflowIDStr)
	stepIDStr := chi.URLParam(r, "stepId")
	stepID := common.ID(stepIDStr)

	if err := h.service.RemoveStep(r.Context(), workflowID, stepID); err != nil {
		h.logger.Error("failed to remove step", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// CreateDelegationRequest is the request body for creating a delegation
type CreateDelegationRequest struct {
	DelegateID    string   `json:"delegate_id"`
	WorkflowID    string   `json:"workflow_id,omitempty"`
	DocumentTypes []string `json:"document_types,omitempty"`
	StartDate     string   `json:"start_date"`
	EndDate       string   `json:"end_date,omitempty"`
	Reason        string   `json:"reason"`
}

// CreateDelegation handles POST /delegations
func (h *WorkflowHandler) CreateDelegation(w http.ResponseWriter, r *http.Request) {
	var req CreateDelegationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	delegatorID := common.ID(auth.GetUserIDFromContext(r.Context()))

	delegReq := service.CreateDelegationRequest{
		EntityID:      entityID,
		DelegatorID:   delegatorID,
		DelegateID:    common.ID(req.DelegateID),
		DocumentTypes: req.DocumentTypes,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Reason:        req.Reason,
	}

	if req.WorkflowID != "" {
		workflowID := common.ID(req.WorkflowID)
		delegReq.WorkflowID = &workflowID
	}

	delegation, err := h.service.CreateDelegation(r.Context(), delegReq)
	if err != nil {
		h.logger.Error("failed to create delegation", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, delegation)
}

// GetDelegation handles GET /delegations/{id}
func (h *WorkflowHandler) GetDelegation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	delegation, err := h.service.GetDelegation(r.Context(), id)
	if err == domain.ErrDelegationNotFound {
		h.writeError(w, http.StatusNotFound, "delegation not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, delegation)
}

// ListDelegations handles GET /delegations
func (h *WorkflowHandler) ListDelegations(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	activeOnly := r.URL.Query().Get("active_only") == "true"

	delegations, err := h.service.ListDelegations(r.Context(), entityID, activeOnly)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, delegations)
}

// DeactivateDelegation handles POST /delegations/{id}/deactivate
func (h *WorkflowHandler) DeactivateDelegation(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	if err := h.service.DeactivateDelegation(r.Context(), id); err != nil {
		h.logger.Error("failed to deactivate delegation", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}

func (h *WorkflowHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *WorkflowHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

// RequestHandler handles approval request HTTP requests
type RequestHandler struct {
	service *service.RequestService
	logger  *zap.Logger
}

// NewRequestHandler creates a new RequestHandler
func NewRequestHandler(svc *service.RequestService, logger *zap.Logger) *RequestHandler {
	return &RequestHandler{
		service: svc,
		logger:  logger,
	}
}

// SubmitForApprovalRequest is the request body for submitting for approval
type SubmitForApprovalRequest struct {
	DocumentType   string         `json:"document_type"`
	DocumentID     string         `json:"document_id"`
	DocumentNumber string         `json:"document_number"`
	Amount         *float64       `json:"amount,omitempty"`
	CurrencyCode   string         `json:"currency_code,omitempty"`
	Notes          string         `json:"notes,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// SubmitForApproval handles POST /requests
func (h *RequestHandler) SubmitForApproval(w http.ResponseWriter, r *http.Request) {
	var req SubmitForApprovalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	requestorID := common.ID(auth.GetUserIDFromContext(r.Context()))

	submitReq := service.SubmitForApprovalRequest{
		EntityID:       entityID,
		DocumentType:   req.DocumentType,
		DocumentID:     common.ID(req.DocumentID),
		DocumentNumber: req.DocumentNumber,
		RequestorID:    requestorID,
		RequestorNotes: req.Notes,
		Metadata:       req.Metadata,
	}

	if req.Amount != nil {
		amount := decimal.NewFromFloat(*req.Amount)
		submitReq.Amount = &amount
		submitReq.CurrencyCode = req.CurrencyCode
	}

	request, err := h.service.SubmitForApproval(r.Context(), submitReq)
	if err != nil {
		h.logger.Error("failed to submit for approval", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, request)
}

// GetRequest handles GET /requests/{id}
func (h *RequestHandler) GetRequest(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	request, err := h.service.GetRequest(r.Context(), id)
	if err == domain.ErrRequestNotFound {
		h.writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, request)
}

// ListRequests handles GET /requests
func (h *RequestHandler) ListRequests(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.RequestFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if documentType := r.URL.Query().Get("document_type"); documentType != "" {
		filter.DocumentType = documentType
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := domain.RequestStatus(statusStr)
		filter.Status = &status
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	requests, total, err := h.service.ListRequests(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   requests,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// ApproveRequest is the request body for approving a request
type ApproveRequest struct {
	Comments string `json:"comments,omitempty"`
}

// Approve handles POST /requests/{id}/approve
func (h *RequestHandler) Approve(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)
	actorID := common.ID(auth.GetUserIDFromContext(r.Context()))

	var req ApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body
		req = ApproveRequest{}
	}

	if err := h.service.Approve(r.Context(), id, actorID, req.Comments); err != nil {
		h.logger.Error("failed to approve request", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch updated request
	request, err := h.service.GetRequest(r.Context(), id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, request)
}

// RejectRequest is the request body for rejecting a request
type RejectRequest struct {
	Comments string `json:"comments"`
}

// Reject handles POST /requests/{id}/reject
func (h *RequestHandler) Reject(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)
	actorID := common.ID(auth.GetUserIDFromContext(r.Context()))

	var req RejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.Reject(r.Context(), id, actorID, req.Comments); err != nil {
		h.logger.Error("failed to reject request", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch updated request
	request, err := h.service.GetRequest(r.Context(), id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, request)
}

// Cancel handles POST /requests/{id}/cancel
func (h *RequestHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)
	actorID := common.ID(auth.GetUserIDFromContext(r.Context()))

	if err := h.service.CancelRequest(r.Context(), id, actorID); err != nil {
		h.logger.Error("failed to cancel request", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GetPendingApprovals handles GET /pending-approvals
func (h *RequestHandler) GetPendingApprovals(w http.ResponseWriter, r *http.Request) {
	approverID := common.ID(auth.GetUserIDFromContext(r.Context()))

	pending, err := h.service.GetPendingApprovals(r.Context(), approverID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, pending)
}

func (h *RequestHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *RequestHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
