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

type WorkflowService struct {
	workflowRepo   repository.WorkflowRepository
	stepRepo       repository.WorkflowStepRepository
	delegationRepo repository.DelegationRepository
	auditLogger    *audit.Logger
	logger         *zap.Logger
}

func NewWorkflowService(
	workflowRepo repository.WorkflowRepository,
	stepRepo repository.WorkflowStepRepository,
	delegationRepo repository.DelegationRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *WorkflowService {
	return &WorkflowService{
		workflowRepo:   workflowRepo,
		stepRepo:       stepRepo,
		delegationRepo: delegationRepo,
		auditLogger:    auditLogger,
		logger:         logger,
	}
}

type CreateWorkflowRequest struct {
	EntityID     common.ID
	WorkflowCode string
	WorkflowName string
	Description  string
	DocumentType string
	CreatedBy    common.ID
}

func (s *WorkflowService) CreateWorkflow(ctx context.Context, req CreateWorkflowRequest) (*domain.Workflow, error) {
	existing, err := s.workflowRepo.GetByCode(ctx, req.EntityID, req.WorkflowCode)
	if err != nil && err != domain.ErrWorkflowNotFound {
		return nil, fmt.Errorf("failed to check existing workflow: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrWorkflowAlreadyExists
	}

	workflow := domain.NewWorkflow(
		req.EntityID,
		req.WorkflowCode,
		req.WorkflowName,
		req.Description,
		req.DocumentType,
		req.CreatedBy,
	)

	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.created", map[string]any{
		"entity_id":     req.EntityID,
		"workflow_code": req.WorkflowCode,
		"document_type": req.DocumentType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return workflow, nil
}

func (s *WorkflowService) GetWorkflow(ctx context.Context, id common.ID) (*domain.Workflow, error) {
	workflow, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	steps, err := s.stepRepo.GetByWorkflowID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load steps: %w", err)
	}
	workflow.Steps = steps

	return workflow, nil
}

func (s *WorkflowService) GetWorkflowByCode(ctx context.Context, entityID common.ID, code string) (*domain.Workflow, error) {
	workflow, err := s.workflowRepo.GetByCode(ctx, entityID, code)
	if err != nil {
		return nil, err
	}

	steps, err := s.stepRepo.GetByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load steps: %w", err)
	}
	workflow.Steps = steps

	return workflow, nil
}

func (s *WorkflowService) GetActiveWorkflowForDocument(ctx context.Context, entityID common.ID, documentType string) (*domain.Workflow, error) {
	workflow, err := s.workflowRepo.GetActiveByDocumentType(ctx, entityID, documentType)
	if err != nil {
		return nil, err
	}

	steps, err := s.stepRepo.GetByWorkflowID(ctx, workflow.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load steps: %w", err)
	}
	workflow.Steps = steps

	return workflow, nil
}

func (s *WorkflowService) ListWorkflows(ctx context.Context, filter repository.WorkflowFilter) ([]domain.Workflow, int, error) {
	return s.workflowRepo.List(ctx, filter)
}

type UpdateWorkflowRequest struct {
	ID           common.ID
	WorkflowName string
	Description  string
}

func (s *WorkflowService) UpdateWorkflow(ctx context.Context, req UpdateWorkflowRequest) (*domain.Workflow, error) {
	workflow, err := s.workflowRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	if !workflow.CanEdit() {
		return nil, domain.ErrInvalidWorkflowStatus
	}

	workflow.WorkflowName = req.WorkflowName
	workflow.Description = req.Description

	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.updated", map[string]any{
		"workflow_name": req.WorkflowName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return s.GetWorkflow(ctx, workflow.ID)
}

func (s *WorkflowService) ActivateWorkflow(ctx context.Context, id common.ID) (*domain.Workflow, error) {
	workflow, err := s.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := workflow.Activate(); err != nil {
		return nil, err
	}

	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to activate workflow: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.activated", map[string]any{
		"entity_id":     workflow.EntityID,
		"workflow_code": workflow.WorkflowCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return workflow, nil
}

func (s *WorkflowService) DeactivateWorkflow(ctx context.Context, id common.ID) (*domain.Workflow, error) {
	workflow, err := s.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := workflow.Deactivate(); err != nil {
		return nil, err
	}

	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to deactivate workflow: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.deactivated", map[string]any{
		"entity_id":     workflow.EntityID,
		"workflow_code": workflow.WorkflowCode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return workflow, nil
}

func (s *WorkflowService) ArchiveWorkflow(ctx context.Context, id common.ID) error {
	workflow, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := workflow.Archive(); err != nil {
		return err
	}

	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		return fmt.Errorf("failed to archive workflow: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.archived", map[string]any{
		"entity_id":     workflow.EntityID,
		"workflow_code": workflow.WorkflowCode,
	})
	if err != nil {
		return fmt.Errorf("failed to log posted run action: %w", err)
	}

	return nil
}

type AddStepRequest struct {
	WorkflowID          common.ID
	StepNumber          int
	StepName            string
	StepType            domain.StepType
	ApproverType        domain.ApproverType
	ApproverID          *common.ID
	ApproverExpression  string
	ThresholdMin        *decimal.Decimal
	ThresholdMax        *decimal.Decimal
	ThresholdCurrency   string
	RequiredApprovals   int
	AllowSelfApproval   bool
	EscalationHours     *int
	EscalateToStep      *int
	ConditionExpression string
}

func (s *WorkflowService) AddStep(ctx context.Context, req AddStepRequest) (*domain.WorkflowStep, error) {
	workflow, err := s.workflowRepo.GetByID(ctx, req.WorkflowID)
	if err != nil {
		return nil, err
	}

	if !workflow.CanEdit() {
		return nil, domain.ErrInvalidWorkflowStatus
	}

	existing, err := s.stepRepo.GetByWorkflowAndNumber(ctx, req.WorkflowID, req.StepNumber)
	if err != nil && err != domain.ErrStepNotFound {
		return nil, fmt.Errorf("failed to check existing step: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrDuplicateStepNumber
	}

	step := domain.NewWorkflowStep(
		req.WorkflowID,
		req.StepNumber,
		req.StepName,
		req.StepType,
		req.ApproverType,
	)

	if req.ApproverID != nil {
		step.SetApprover(*req.ApproverID)
	}
	if req.ApproverExpression != "" {
		step.SetApproverExpression(req.ApproverExpression)
	}
	if req.ThresholdMin != nil || req.ThresholdMax != nil {
		step.SetThreshold(req.ThresholdMin, req.ThresholdMax, req.ThresholdCurrency)
	}
	if req.RequiredApprovals > 0 {
		step.RequiredApprovals = req.RequiredApprovals
	}
	step.AllowSelfApproval = req.AllowSelfApproval
	if req.EscalationHours != nil && req.EscalateToStep != nil {
		step.SetEscalation(*req.EscalationHours, *req.EscalateToStep)
	}
	step.ConditionExpression = req.ConditionExpression

	if err := s.stepRepo.Create(ctx, step); err != nil {
		return nil, fmt.Errorf("failed to create step: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.step.added", map[string]any{
		"step_id":     step.ID,
		"step_number": req.StepNumber,
		"step_name":   req.StepName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return step, nil
}

func (s *WorkflowService) UpdateStep(ctx context.Context, step *domain.WorkflowStep) error {
	workflow, err := s.workflowRepo.GetByID(ctx, step.WorkflowID)
	if err != nil {
		return err
	}

	if !workflow.CanEdit() {
		return domain.ErrInvalidWorkflowStatus
	}

	if err := s.stepRepo.Update(ctx, step); err != nil {
		return fmt.Errorf("failed to update step: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.step.updated", map[string]any{
		"step_id":     step.ID,
		"step_number": step.StepNumber,
	})
	if err != nil {
		return fmt.Errorf("failed to log posted run action: %w", err)
	}

	return nil
}

func (s *WorkflowService) RemoveStep(ctx context.Context, workflowID, stepID common.ID) error {
	workflow, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		return err
	}

	if !workflow.CanEdit() {
		return domain.ErrInvalidWorkflowStatus
	}

	step, err := s.stepRepo.GetByID(ctx, stepID)
	if err != nil {
		return err
	}

	if step.WorkflowID != workflowID {
		return domain.ErrStepNotFound
	}

	if err := s.stepRepo.Delete(ctx, stepID); err != nil {
		return fmt.Errorf("failed to delete step: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow", workflow.ID, "workflow.step.removed", map[string]any{
		"step_id":     stepID,
		"step_number": step.StepNumber,
	})
	if err != nil {
		return fmt.Errorf("failed to log posted run action: %w", err)
	}

	return nil
}

type CreateDelegationRequest struct {
	EntityID      common.ID
	DelegatorID   common.ID
	DelegateID    common.ID
	WorkflowID    *common.ID
	DocumentTypes []string
	StartDate     string
	EndDate       string
	Reason        string
}

func (s *WorkflowService) CreateDelegation(ctx context.Context, req CreateDelegationRequest) (*domain.Delegation, error) {
	startDate, err := parseDate(req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}

	delegation, err := domain.NewDelegation(
		req.EntityID,
		req.DelegatorID,
		req.DelegateID,
		startDate,
		req.Reason,
	)
	if err != nil {
		return nil, err
	}

	if req.EndDate != "" {
		endDate, err := parseDate(req.EndDate)
		if err != nil {
			return nil, fmt.Errorf("invalid end date: %w", err)
		}
		if err := delegation.SetEndDate(endDate); err != nil {
			return nil, err
		}
	}

	if req.WorkflowID != nil {
		delegation.SetWorkflow(*req.WorkflowID)
	}

	if len(req.DocumentTypes) > 0 {
		delegation.SetDocumentTypes(req.DocumentTypes)
	}

	if err := s.delegationRepo.Create(ctx, delegation); err != nil {
		return nil, fmt.Errorf("failed to create delegation: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow_delegation", delegation.ID, "delegation.created", map[string]any{
		"entity_id":    req.EntityID,
		"delegator_id": req.DelegatorID,
		"delegate_id":  req.DelegateID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to log posted run action: %w", err)
	}

	return delegation, nil
}

func (s *WorkflowService) GetDelegation(ctx context.Context, id common.ID) (*domain.Delegation, error) {
	return s.delegationRepo.GetByID(ctx, id)
}

func (s *WorkflowService) ListDelegations(ctx context.Context, entityID common.ID, activeOnly bool) ([]domain.Delegation, error) {
	return s.delegationRepo.ListByEntity(ctx, entityID, activeOnly)
}

func (s *WorkflowService) GetEffectiveDelegate(ctx context.Context, entityID, approverID common.ID, documentType string, workflowID *common.ID) (*domain.Delegation, error) {
	return s.delegationRepo.GetEffectiveDelegateFor(ctx, entityID, approverID, documentType, workflowID)
}

func (s *WorkflowService) DeactivateDelegation(ctx context.Context, id common.ID) error {
	delegation, err := s.delegationRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	delegation.Deactivate()

	if err := s.delegationRepo.Update(ctx, delegation); err != nil {
		return fmt.Errorf("failed to deactivate delegation: %w", err)
	}

	err = s.auditLogger.Log(ctx, "workflow_delegation", delegation.ID, "delegation.deactivated", map[string]any{
		"entity_id":    delegation.EntityID,
		"delegator_id": delegation.DelegatorID,
	})
	if err != nil {
		return fmt.Errorf("failed to log posted run action: %w", err)
	}

	return nil
}

func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}
