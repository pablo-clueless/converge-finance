package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/workflow/internal/domain"
	"converge-finance.com/m/internal/modules/workflow/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// RequestService handles approval request operations
type RequestService struct {
	requestRepo    repository.RequestRepository
	actionRepo     repository.ActionRepository
	pendingRepo    repository.PendingApprovalRepository
	workflowRepo   repository.WorkflowRepository
	stepRepo       repository.WorkflowStepRepository
	delegationRepo repository.DelegationRepository
	auditLogger    *audit.Logger
	logger         *zap.Logger
}

// NewRequestService creates a new RequestService
func NewRequestService(
	requestRepo repository.RequestRepository,
	actionRepo repository.ActionRepository,
	pendingRepo repository.PendingApprovalRepository,
	workflowRepo repository.WorkflowRepository,
	stepRepo repository.WorkflowStepRepository,
	delegationRepo repository.DelegationRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *RequestService {
	return &RequestService{
		requestRepo:    requestRepo,
		actionRepo:     actionRepo,
		pendingRepo:    pendingRepo,
		workflowRepo:   workflowRepo,
		stepRepo:       stepRepo,
		delegationRepo: delegationRepo,
		auditLogger:    auditLogger,
		logger:         logger,
	}
}

// SubmitForApprovalRequest contains data for submitting a document for approval
type SubmitForApprovalRequest struct {
	EntityID       common.ID
	DocumentType   string
	DocumentID     common.ID
	DocumentNumber string
	Amount         *decimal.Decimal
	CurrencyCode   string
	RequestorID    common.ID
	RequestorNotes string
	Metadata       map[string]any
}

// SubmitForApproval submits a document for approval
func (s *RequestService) SubmitForApproval(ctx context.Context, req SubmitForApprovalRequest) (*domain.ApprovalRequest, error) {
	// Check if there's already a pending request for this document
	existing, err := s.requestRepo.GetByDocument(ctx, req.EntityID, req.DocumentType, req.DocumentID)
	if err != nil && err != domain.ErrRequestNotFound {
		return nil, fmt.Errorf("failed to check existing request: %w", err)
	}
	if existing != nil && existing.IsPending() {
		return nil, domain.ErrRequestAlreadyExists
	}

	// Find applicable workflow
	workflow, err := s.workflowRepo.GetActiveByDocumentType(ctx, req.EntityID, req.DocumentType)
	if err != nil {
		if err == domain.ErrWorkflowNotFound {
			return nil, domain.ErrNoApplicableWorkflow
		}
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}

	// Load workflow steps
	steps, err := s.stepRepo.GetByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow steps: %w", err)
	}
	if len(steps) == 0 {
		return nil, domain.ErrNoWorkflowSteps
	}

	// Generate request number
	prefix := "APR"
	requestNumber, err := s.requestRepo.GenerateRequestNumber(ctx, req.EntityID, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to generate request number: %w", err)
	}

	// Create approval request
	request := domain.NewApprovalRequest(
		req.EntityID,
		requestNumber,
		workflow.ID,
		req.DocumentType,
		req.DocumentID,
		req.DocumentNumber,
		req.RequestorID,
	)

	if req.Amount != nil {
		request.SetAmount(*req.Amount, req.CurrencyCode)
	}
	if req.RequestorNotes != "" {
		request.SetRequestorNotes(req.RequestorNotes)
	}
	if req.Metadata != nil {
		request.SetMetadata(req.Metadata)
	}

	if err := s.requestRepo.Create(ctx, request); err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Create pending approvals for the first step
	firstStep := s.findApplicableStep(steps, 1, req.Amount)
	if firstStep == nil {
		return nil, domain.ErrNoWorkflowSteps
	}

	if err := s.createPendingApprovalsForStep(ctx, request, firstStep); err != nil {
		return nil, fmt.Errorf("failed to create pending approvals: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.submitted", map[string]any{
		"entity_id":      req.EntityID,
		"document_type":  req.DocumentType,
		"document_id":    req.DocumentID,
		"workflow_id":    workflow.ID,
		"request_number": requestNumber,
	}); err != nil {
		return nil, fmt.Errorf("failed to log audit event: %w", err)
	}

	return request, nil
}

// GetRequest retrieves an approval request by ID
func (s *RequestService) GetRequest(ctx context.Context, id common.ID) (*domain.ApprovalRequest, error) {
	request, err := s.requestRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	actions, err := s.actionRepo.GetByRequestID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load actions: %w", err)
	}
	request.Actions = actions

	workflow, err := s.workflowRepo.GetByID(ctx, request.WorkflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to load workflow: %w", err)
	}
	request.Workflow = workflow

	return request, nil
}

// ListRequests lists approval requests with filters
func (s *RequestService) ListRequests(ctx context.Context, filter repository.RequestFilter) ([]domain.ApprovalRequest, int, error) {
	return s.requestRepo.List(ctx, filter)
}

// Approve approves a request at the current step
func (s *RequestService) Approve(ctx context.Context, requestID, actorID common.ID, comments string) error {
	request, err := s.requestRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	if !request.IsPending() {
		return domain.ErrInvalidRequestStatus
	}

	// Check if actor is authorized to approve
	pending, err := s.pendingRepo.GetByRequestAndApprover(ctx, requestID, actorID)
	if err != nil {
		// Check for delegation
		effectiveApprover, delegatedFrom, err := s.findEffectiveApprover(ctx, request, actorID)
		if err != nil {
			return domain.ErrNotAuthorizedToApprove
		}

		pending, err = s.pendingRepo.GetByRequestAndApprover(ctx, requestID, effectiveApprover)
		if err != nil {
			return domain.ErrNotAuthorizedToApprove
		}

		// Create action with delegation info
		return s.processApprovalWithDelegation(ctx, request, pending, actorID, delegatedFrom, comments)
	}

	return s.processApproval(ctx, request, pending, actorID, comments)
}

// Reject rejects an approval request
func (s *RequestService) Reject(ctx context.Context, requestID, actorID common.ID, comments string) error {
	request, err := s.requestRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	if !request.IsPending() {
		return domain.ErrInvalidRequestStatus
	}

	// Check if actor is authorized to approve
	pending, err := s.pendingRepo.GetByRequestAndApprover(ctx, requestID, actorID)
	if err != nil {
		// Check for delegation
		effectiveApprover, delegatedFrom, err := s.findEffectiveApprover(ctx, request, actorID)
		if err != nil {
			return domain.ErrNotAuthorizedToApprove
		}

		pending, err = s.pendingRepo.GetByRequestAndApprover(ctx, requestID, effectiveApprover)
		if err != nil {
			return domain.ErrNotAuthorizedToApprove
		}

		return s.processRejectionWithDelegation(ctx, request, pending, actorID, delegatedFrom, comments)
	}

	return s.processRejection(ctx, request, pending, actorID, comments)
}

// GetPendingApprovals returns pending approvals for an approver
func (s *RequestService) GetPendingApprovals(ctx context.Context, approverID common.ID) ([]domain.PendingApproval, error) {
	pending, err := s.pendingRepo.GetByApprover(ctx, approverID)
	if err != nil {
		return nil, err
	}

	// Enrich with request and step details
	for i := range pending {
		request, err := s.requestRepo.GetByID(ctx, pending[i].RequestID)
		if err != nil {
			continue
		}
		pending[i].Request = request

		step, err := s.stepRepo.GetByID(ctx, pending[i].StepID)
		if err != nil {
			continue
		}
		pending[i].Step = step
	}

	return pending, nil
}

// CancelRequest cancels an approval request
func (s *RequestService) CancelRequest(ctx context.Context, requestID, actorID common.ID) error {
	request, err := s.requestRepo.GetByID(ctx, requestID)
	if err != nil {
		return err
	}

	// Only the requestor can cancel
	if request.RequestorID != actorID {
		return domain.ErrNotAuthorizedToApprove
	}

	if err := request.Cancel(); err != nil {
		return err
	}

	// Remove all pending approvals
	if err := s.pendingRepo.DeleteByRequest(ctx, requestID); err != nil {
		return fmt.Errorf("failed to delete pending approvals: %w", err)
	}

	if err := s.requestRepo.Update(ctx, request); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.cancelled", map[string]any{
		"entity_id":      request.EntityID,
		"cancelled_by":   actorID,
		"request_number": request.RequestNumber,
	}); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

// Helper methods

func (s *RequestService) processApproval(ctx context.Context, request *domain.ApprovalRequest, pending *domain.PendingApproval, actorID common.ID, comments string) error {
	step, err := s.stepRepo.GetByID(ctx, pending.StepID)
	if err != nil {
		return err
	}

	// Check self-approval
	if !step.AllowSelfApproval && request.RequestorID == actorID {
		return domain.ErrSelfApprovalNotAllowed
	}

	// Create approval action
	action := domain.NewApprovalAction(
		request.ID,
		step.ID,
		step.StepNumber,
		domain.ActionTypeApprove,
		actorID,
		comments,
	)

	if err := s.actionRepo.Create(ctx, action); err != nil {
		return fmt.Errorf("failed to create action: %w", err)
	}

	// Remove the pending approval
	if err := s.pendingRepo.Delete(ctx, pending.ID); err != nil {
		return fmt.Errorf("failed to delete pending approval: %w", err)
	}

	// Check if step is complete
	approvalCount, err := s.actionRepo.CountApprovalsByStep(ctx, request.ID, step.StepNumber)
	if err != nil {
		return fmt.Errorf("failed to count approvals: %w", err)
	}

	if approvalCount >= step.RequiredApprovals {
		// Step is complete, advance to next step
		return s.advanceToNextStep(ctx, request, step)
	}

	if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.approved.step", map[string]any{
		"step_number":    step.StepNumber,
		"actor_id":       actorID,
		"approval_count": approvalCount,
		"required":       step.RequiredApprovals,
	}); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

func (s *RequestService) processApprovalWithDelegation(ctx context.Context, request *domain.ApprovalRequest, pending *domain.PendingApproval, actorID, delegatedFrom common.ID, comments string) error {
	step, err := s.stepRepo.GetByID(ctx, pending.StepID)
	if err != nil {
		return err
	}

	// Create approval action with delegation
	action := domain.NewApprovalAction(
		request.ID,
		step.ID,
		step.StepNumber,
		domain.ActionTypeApprove,
		actorID,
		comments,
	)
	action.SetDelegatedBy(delegatedFrom)

	if err := s.actionRepo.Create(ctx, action); err != nil {
		return fmt.Errorf("failed to create action: %w", err)
	}

	// Remove the pending approval
	if err := s.pendingRepo.Delete(ctx, pending.ID); err != nil {
		return fmt.Errorf("failed to delete pending approval: %w", err)
	}

	// Check if step is complete
	approvalCount, err := s.actionRepo.CountApprovalsByStep(ctx, request.ID, step.StepNumber)
	if err != nil {
		return fmt.Errorf("failed to count approvals: %w", err)
	}

	if approvalCount >= step.RequiredApprovals {
		return s.advanceToNextStep(ctx, request, step)
	}

	if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.approved.delegated", map[string]any{
		"step_number":    step.StepNumber,
		"actor_id":       actorID,
		"delegated_from": delegatedFrom,
	}); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

func (s *RequestService) processRejection(ctx context.Context, request *domain.ApprovalRequest, pending *domain.PendingApproval, actorID common.ID, comments string) error {
	step, err := s.stepRepo.GetByID(ctx, pending.StepID)
	if err != nil {
		return err
	}

	// Create rejection action
	action := domain.NewApprovalAction(
		request.ID,
		step.ID,
		step.StepNumber,
		domain.ActionTypeReject,
		actorID,
		comments,
	)

	if err := s.actionRepo.Create(ctx, action); err != nil {
		return fmt.Errorf("failed to create action: %w", err)
	}

	// Reject the request
	if err := request.Reject(); err != nil {
		return err
	}

	// Remove all pending approvals
	if err := s.pendingRepo.DeleteByRequest(ctx, request.ID); err != nil {
		return fmt.Errorf("failed to delete pending approvals: %w", err)
	}

	if err := s.requestRepo.Update(ctx, request); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.rejected", map[string]any{
		"step_number": step.StepNumber,
		"actor_id":    actorID,
		"comments":    comments,
	}); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

func (s *RequestService) processRejectionWithDelegation(ctx context.Context, request *domain.ApprovalRequest, pending *domain.PendingApproval, actorID, delegatedFrom common.ID, comments string) error {
	step, err := s.stepRepo.GetByID(ctx, pending.StepID)
	if err != nil {
		return err
	}

	// Create rejection action with delegation
	action := domain.NewApprovalAction(
		request.ID,
		step.ID,
		step.StepNumber,
		domain.ActionTypeReject,
		actorID,
		comments,
	)
	action.SetDelegatedBy(delegatedFrom)

	if err := s.actionRepo.Create(ctx, action); err != nil {
		return fmt.Errorf("failed to create action: %w", err)
	}

	// Reject the request
	if err := request.Reject(); err != nil {
		return err
	}

	// Remove all pending approvals
	if err := s.pendingRepo.DeleteByRequest(ctx, request.ID); err != nil {
		return fmt.Errorf("failed to delete pending approvals: %w", err)
	}

	if err := s.requestRepo.Update(ctx, request); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.rejected.delegated", map[string]any{
		"step_number":    step.StepNumber,
		"actor_id":       actorID,
		"delegated_from": delegatedFrom,
	}); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

func (s *RequestService) advanceToNextStep(ctx context.Context, request *domain.ApprovalRequest, currentStep *domain.WorkflowStep) error {
	// Load all steps for the workflow
	steps, err := s.stepRepo.GetByWorkflowID(ctx, request.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to load workflow steps: %w", err)
	}

	// Find next applicable step
	nextStepNumber := currentStep.StepNumber + 1
	nextStep := s.findApplicableStep(steps, nextStepNumber, request.Amount)

	if nextStep == nil {
		// No more steps, approve the request
		if err := request.Approve(); err != nil {
			return err
		}

		if err := s.requestRepo.Update(ctx, request); err != nil {
			return fmt.Errorf("failed to update request: %w", err)
		}

		if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.approved", map[string]any{
			"entity_id":      request.EntityID,
			"request_number": request.RequestNumber,
		}); err != nil {
			return fmt.Errorf("failed to log audit event: %w", err)
		}

		return nil
	}

	// Advance to next step
	request.AdvanceToStep(nextStep.StepNumber)

	if err := s.requestRepo.Update(ctx, request); err != nil {
		return fmt.Errorf("failed to update request: %w", err)
	}

	// Create pending approvals for next step
	if err := s.createPendingApprovalsForStep(ctx, request, nextStep); err != nil {
		return fmt.Errorf("failed to create pending approvals: %w", err)
	}

	if err := s.auditLogger.Log(ctx, "workflow_request", request.ID, "request.advanced", map[string]any{
		"from_step": currentStep.StepNumber,
		"to_step":   nextStep.StepNumber,
	}); err != nil {
		return fmt.Errorf("failed to log audit event: %w", err)
	}

	return nil
}

func (s *RequestService) findApplicableStep(steps []domain.WorkflowStep, fromStepNumber int, amount *decimal.Decimal) *domain.WorkflowStep {
	for i := range steps {
		step := &steps[i]
		if step.StepNumber < fromStepNumber {
			continue
		}
		if !step.IsActive {
			continue
		}

		// Check threshold applicability
		if amount != nil && !step.IsApplicableForAmount(*amount) {
			continue
		}

		return step
	}
	return nil
}

func (s *RequestService) createPendingApprovalsForStep(ctx context.Context, request *domain.ApprovalRequest, step *domain.WorkflowStep) error {
	// For simplicity, if there's a specific approver ID, use it
	// In a full implementation, this would resolve roles, managers, expressions, etc.
	if step.ApproverID != nil {
		pending := domain.NewPendingApproval(request.ID, step.ID, *step.ApproverID)

		// Set due date if escalation is configured
		if step.EscalationHours != nil {
			dueAt := time.Now().Add(time.Duration(*step.EscalationHours) * time.Hour)
			pending.SetDueDate(dueAt)
		}

		if err := s.pendingRepo.Create(ctx, pending); err != nil {
			return err
		}
	}

	return nil
}

func (s *RequestService) findEffectiveApprover(ctx context.Context, request *domain.ApprovalRequest, actorID common.ID) (common.ID, common.ID, error) {
	// Check if anyone has delegated to this actor
	pending, err := s.pendingRepo.GetByRequest(ctx, request.ID)
	if err != nil {
		return common.ID(""), common.ID(""), err
	}

	for _, p := range pending {
		delegation, err := s.delegationRepo.GetEffectiveDelegateFor(
			ctx,
			request.EntityID,
			p.ApproverID,
			request.DocumentType,
			&request.WorkflowID,
		)
		if err != nil {
			continue
		}

		if delegation != nil && delegation.DelegateID == actorID {
			return p.ApproverID, delegation.DelegatorID, nil
		}
	}

	return common.ID(""), common.ID(""), domain.ErrNotAuthorizedToApprove
}
