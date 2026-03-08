package workflow

import (
	"context"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/workflow/internal/domain"
	"converge-finance.com/m/internal/modules/workflow/internal/repository"
	"converge-finance.com/m/internal/modules/workflow/internal/service"
)

type workflowAPI struct {
	workflowService *service.WorkflowService
	requestService  *service.RequestService
}

// NewWorkflowAPI creates a new workflow API implementation
func NewWorkflowAPI(workflowService *service.WorkflowService, requestService *service.RequestService) API {
	return &workflowAPI{
		workflowService: workflowService,
		requestService:  requestService,
	}
}

func (a *workflowAPI) CreateWorkflow(ctx context.Context, req CreateWorkflowRequest) (*WorkflowResponse, error) {
	workflow, err := a.workflowService.CreateWorkflow(ctx, service.CreateWorkflowRequest{
		EntityID:     req.EntityID,
		WorkflowCode: req.WorkflowCode,
		WorkflowName: req.WorkflowName,
		Description:  req.Description,
		DocumentType: req.DocumentType,
		CreatedBy:    req.CreatedBy,
	})
	if err != nil {
		return nil, err
	}

	return a.mapWorkflowToResponse(workflow), nil
}

func (a *workflowAPI) GetWorkflow(ctx context.Context, id common.ID) (*WorkflowResponse, error) {
	workflow, err := a.workflowService.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapWorkflowToResponse(workflow), nil
}

func (a *workflowAPI) GetActiveWorkflowForDocument(ctx context.Context, entityID common.ID, documentType string) (*WorkflowResponse, error) {
	workflow, err := a.workflowService.GetActiveWorkflowForDocument(ctx, entityID, documentType)
	if err != nil {
		return nil, err
	}

	return a.mapWorkflowToResponse(workflow), nil
}

func (a *workflowAPI) ListWorkflows(ctx context.Context, req ListWorkflowsRequest) (*ListWorkflowsResponse, error) {
	filter := repository.WorkflowFilter{
		EntityID:     req.EntityID,
		DocumentType: req.DocumentType,
		CurrentOnly:  req.CurrentOnly,
		Limit:        req.PageSize,
		Offset:       (req.Page - 1) * req.PageSize,
	}

	if req.Status != "" {
		status := domain.WorkflowStatus(req.Status)
		filter.Status = &status
	}

	workflows, total, err := a.workflowService.ListWorkflows(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]WorkflowResponse, len(workflows))
	for i, wf := range workflows {
		responses[i] = *a.mapWorkflowToResponse(&wf)
	}

	return &ListWorkflowsResponse{
		Workflows: responses,
		Total:     total,
		Page:      req.Page,
	}, nil
}

func (a *workflowAPI) ActivateWorkflow(ctx context.Context, id common.ID) (*WorkflowResponse, error) {
	workflow, err := a.workflowService.ActivateWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapWorkflowToResponse(workflow), nil
}

func (a *workflowAPI) DeactivateWorkflow(ctx context.Context, id common.ID) (*WorkflowResponse, error) {
	workflow, err := a.workflowService.DeactivateWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapWorkflowToResponse(workflow), nil
}

func (a *workflowAPI) AddStep(ctx context.Context, workflowID common.ID, req AddStepRequest) (*WorkflowStepResponse, error) {
	step, err := a.workflowService.AddStep(ctx, service.AddStepRequest{
		WorkflowID:          workflowID,
		StepNumber:          req.StepNumber,
		StepName:            req.StepName,
		StepType:            domain.StepType(req.StepType),
		ApproverType:        domain.ApproverType(req.ApproverType),
		ApproverID:          req.ApproverID,
		ApproverExpression:  req.ApproverExpression,
		ThresholdMin:        req.ThresholdMin,
		ThresholdMax:        req.ThresholdMax,
		ThresholdCurrency:   req.ThresholdCurrency,
		RequiredApprovals:   req.RequiredApprovals,
		AllowSelfApproval:   req.AllowSelfApproval,
		EscalationHours:     req.EscalationHours,
		EscalateToStep:      req.EscalateToStep,
		ConditionExpression: req.ConditionExpression,
	})
	if err != nil {
		return nil, err
	}

	return a.mapStepToResponse(step), nil
}

func (a *workflowAPI) RemoveStep(ctx context.Context, workflowID, stepID common.ID) error {
	return a.workflowService.RemoveStep(ctx, workflowID, stepID)
}

func (a *workflowAPI) SubmitForApproval(ctx context.Context, req SubmitForApprovalRequest) (*ApprovalRequestResponse, error) {
	request, err := a.requestService.SubmitForApproval(ctx, service.SubmitForApprovalRequest{
		EntityID:       req.EntityID,
		DocumentType:   req.DocumentType,
		DocumentID:     req.DocumentID,
		DocumentNumber: req.DocumentNumber,
		Amount:         req.Amount,
		CurrencyCode:   req.CurrencyCode,
		RequestorID:    req.RequestorID,
		RequestorNotes: req.RequestorNotes,
		Metadata:       req.Metadata,
	})
	if err != nil {
		return nil, err
	}

	return a.mapRequestToResponse(request), nil
}

func (a *workflowAPI) GetRequest(ctx context.Context, id common.ID) (*ApprovalRequestResponse, error) {
	request, err := a.requestService.GetRequest(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapRequestToResponse(request), nil
}

func (a *workflowAPI) ListRequests(ctx context.Context, req ListRequestsRequest) (*ListRequestsResponse, error) {
	filter := repository.RequestFilter{
		EntityID:     req.EntityID,
		WorkflowID:   req.WorkflowID,
		DocumentType: req.DocumentType,
		RequestorID:  req.RequestorID,
		DateFrom:     req.DateFrom,
		DateTo:       req.DateTo,
		Limit:        req.PageSize,
		Offset:       (req.Page - 1) * req.PageSize,
	}

	if req.Status != "" {
		status := domain.RequestStatus(req.Status)
		filter.Status = &status
	}

	requests, total, err := a.requestService.ListRequests(ctx, filter)
	if err != nil {
		return nil, err
	}

	responses := make([]ApprovalRequestResponse, len(requests))
	for i, r := range requests {
		responses[i] = *a.mapRequestToResponse(&r)
	}

	return &ListRequestsResponse{
		Requests: responses,
		Total:    total,
		Page:     req.Page,
	}, nil
}

func (a *workflowAPI) Approve(ctx context.Context, requestID, actorID common.ID, comments string) error {
	return a.requestService.Approve(ctx, requestID, actorID, comments)
}

func (a *workflowAPI) Reject(ctx context.Context, requestID, actorID common.ID, comments string) error {
	return a.requestService.Reject(ctx, requestID, actorID, comments)
}

func (a *workflowAPI) CancelRequest(ctx context.Context, requestID, actorID common.ID) error {
	return a.requestService.CancelRequest(ctx, requestID, actorID)
}

func (a *workflowAPI) GetPendingApprovals(ctx context.Context, approverID common.ID) ([]PendingApprovalResponse, error) {
	pending, err := a.requestService.GetPendingApprovals(ctx, approverID)
	if err != nil {
		return nil, err
	}

	responses := make([]PendingApprovalResponse, len(pending))
	for i, p := range pending {
		responses[i] = PendingApprovalResponse{
			ID:           p.ID,
			RequestID:    p.RequestID,
			StepID:       p.StepID,
			ApproverID:   p.ApproverID,
			AssignedAt:   p.AssignedAt,
			DueAt:        p.DueAt,
			ReminderSent: p.ReminderSent,
			Escalated:    p.Escalated,
		}

		if p.Request != nil {
			responses[i].Request = a.mapRequestToResponse(p.Request)
		}

		if p.Step != nil {
			responses[i].Step = a.mapStepToResponse(p.Step)
		}
	}

	return responses, nil
}

func (a *workflowAPI) CreateDelegation(ctx context.Context, req CreateDelegationRequest) (*DelegationResponse, error) {
	delegation, err := a.workflowService.CreateDelegation(ctx, service.CreateDelegationRequest{
		EntityID:      req.EntityID,
		DelegatorID:   req.DelegatorID,
		DelegateID:    req.DelegateID,
		WorkflowID:    req.WorkflowID,
		DocumentTypes: req.DocumentTypes,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Reason:        req.Reason,
	})
	if err != nil {
		return nil, err
	}

	return a.mapDelegationToResponse(delegation), nil
}

func (a *workflowAPI) GetDelegation(ctx context.Context, id common.ID) (*DelegationResponse, error) {
	delegation, err := a.workflowService.GetDelegation(ctx, id)
	if err != nil {
		return nil, err
	}

	return a.mapDelegationToResponse(delegation), nil
}

func (a *workflowAPI) ListDelegations(ctx context.Context, entityID common.ID, activeOnly bool) ([]DelegationResponse, error) {
	delegations, err := a.workflowService.ListDelegations(ctx, entityID, activeOnly)
	if err != nil {
		return nil, err
	}

	responses := make([]DelegationResponse, len(delegations))
	for i, d := range delegations {
		responses[i] = *a.mapDelegationToResponse(&d)
	}

	return responses, nil
}

func (a *workflowAPI) DeactivateDelegation(ctx context.Context, id common.ID) error {
	return a.workflowService.DeactivateDelegation(ctx, id)
}

// Mapping helper functions

func (a *workflowAPI) mapWorkflowToResponse(wf *domain.Workflow) *WorkflowResponse {
	resp := &WorkflowResponse{
		ID:           wf.ID,
		EntityID:     wf.EntityID,
		WorkflowCode: wf.WorkflowCode,
		WorkflowName: wf.WorkflowName,
		Description:  wf.Description,
		DocumentType: wf.DocumentType,
		Status:       string(wf.Status),
		Version:      wf.Version,
		IsCurrent:    wf.IsCurrent,
		Steps:        make([]WorkflowStepResponse, len(wf.Steps)),
		CreatedBy:    wf.CreatedBy,
		CreatedAt:    wf.CreatedAt,
		UpdatedAt:    wf.UpdatedAt,
	}

	for i, step := range wf.Steps {
		resp.Steps[i] = *a.mapStepToResponse(&step)
	}

	return resp
}

func (a *workflowAPI) mapStepToResponse(step *domain.WorkflowStep) *WorkflowStepResponse {
	return &WorkflowStepResponse{
		ID:                  step.ID,
		WorkflowID:          step.WorkflowID,
		StepNumber:          step.StepNumber,
		StepName:            step.StepName,
		StepType:            string(step.StepType),
		ApproverType:        string(step.ApproverType),
		ApproverID:          step.ApproverID,
		ApproverExpression:  step.ApproverExpression,
		ThresholdMin:        step.ThresholdMin,
		ThresholdMax:        step.ThresholdMax,
		ThresholdCurrency:   step.ThresholdCurrency,
		RequiredApprovals:   step.RequiredApprovals,
		AllowSelfApproval:   step.AllowSelfApproval,
		EscalationHours:     step.EscalationHours,
		EscalateToStep:      step.EscalateToStep,
		ConditionExpression: step.ConditionExpression,
		IsActive:            step.IsActive,
		CreatedAt:           step.CreatedAt,
	}
}

func (a *workflowAPI) mapRequestToResponse(req *domain.ApprovalRequest) *ApprovalRequestResponse {
	resp := &ApprovalRequestResponse{
		ID:             req.ID,
		EntityID:       req.EntityID,
		RequestNumber:  req.RequestNumber,
		WorkflowID:     req.WorkflowID,
		DocumentType:   req.DocumentType,
		DocumentID:     req.DocumentID,
		DocumentNumber: req.DocumentNumber,
		Amount:         req.Amount,
		CurrencyCode:   req.CurrencyCode,
		CurrentStep:    req.CurrentStep,
		Status:         string(req.Status),
		RequestorID:    req.RequestorID,
		RequestorNotes: req.RequestorNotes,
		Metadata:       req.Metadata,
		StartedAt:      req.StartedAt,
		CompletedAt:    req.CompletedAt,
		CreatedAt:      req.CreatedAt,
		UpdatedAt:      req.UpdatedAt,
		Actions:        make([]ApprovalActionResponse, len(req.Actions)),
	}

	for i, action := range req.Actions {
		resp.Actions[i] = ApprovalActionResponse{
			ID:          action.ID,
			RequestID:   action.RequestID,
			StepID:      action.StepID,
			StepNumber:  action.StepNumber,
			ActionType:  string(action.ActionType),
			ActorID:     action.ActorID,
			DelegatedBy: action.DelegatedBy,
			Comments:    action.Comments,
			ActedAt:     action.ActedAt,
		}
	}

	return resp
}

func (a *workflowAPI) mapDelegationToResponse(d *domain.Delegation) *DelegationResponse {
	return &DelegationResponse{
		ID:            d.ID,
		EntityID:      d.EntityID,
		DelegatorID:   d.DelegatorID,
		DelegateID:    d.DelegateID,
		WorkflowID:    d.WorkflowID,
		DocumentTypes: d.DocumentTypes,
		StartDate:     d.StartDate,
		EndDate:       d.EndDate,
		Reason:        d.Reason,
		IsActive:      d.IsActive,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}
