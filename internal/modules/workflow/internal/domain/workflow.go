package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

// WorkflowStatus represents the status of a workflow definition
type WorkflowStatus string

const (
	WorkflowStatusDraft    WorkflowStatus = "draft"
	WorkflowStatusActive   WorkflowStatus = "active"
	WorkflowStatusInactive WorkflowStatus = "inactive"
	WorkflowStatusArchived WorkflowStatus = "archived"
)

// StepType represents the type of workflow step
type StepType string

const (
	StepTypeApproval     StepType = "approval"
	StepTypeNotification StepType = "notification"
	StepTypeCondition    StepType = "condition"
	StepTypeParallel     StepType = "parallel"
)

// ApproverType represents who can approve a step
type ApproverType string

const (
	ApproverTypeUser       ApproverType = "user"
	ApproverTypeRole       ApproverType = "role"
	ApproverTypeManager    ApproverType = "manager"
	ApproverTypeDepartment ApproverType = "department"
	ApproverTypeExpression ApproverType = "expression"
)

// Workflow represents a workflow definition
type Workflow struct {
	ID            common.ID
	EntityID      common.ID
	WorkflowCode  string
	WorkflowName  string
	Description   string
	DocumentType  string
	Status        WorkflowStatus
	Version       int
	IsCurrent     bool
	Configuration map[string]any
	CreatedBy     common.ID
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// Related steps
	Steps []WorkflowStep
}

// NewWorkflow creates a new workflow definition
func NewWorkflow(
	entityID common.ID,
	workflowCode, workflowName, description, documentType string,
	createdBy common.ID,
) *Workflow {
	now := time.Now()
	return &Workflow{
		ID:            common.NewID(),
		EntityID:      entityID,
		WorkflowCode:  workflowCode,
		WorkflowName:  workflowName,
		Description:   description,
		DocumentType:  documentType,
		Status:        WorkflowStatusDraft,
		Version:       1,
		IsCurrent:     true,
		Configuration: make(map[string]any),
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
		Steps:         []WorkflowStep{},
	}
}

// Activate activates the workflow
func (w *Workflow) Activate() error {
	if w.Status != WorkflowStatusDraft && w.Status != WorkflowStatusInactive {
		return ErrInvalidWorkflowStatus
	}
	if len(w.Steps) == 0 {
		return ErrNoWorkflowSteps
	}
	w.Status = WorkflowStatusActive
	w.UpdatedAt = time.Now()
	return nil
}

// Deactivate deactivates the workflow
func (w *Workflow) Deactivate() error {
	if w.Status != WorkflowStatusActive {
		return ErrInvalidWorkflowStatus
	}
	w.Status = WorkflowStatusInactive
	w.UpdatedAt = time.Now()
	return nil
}

// Archive archives the workflow
func (w *Workflow) Archive() error {
	if w.Status == WorkflowStatusArchived {
		return ErrInvalidWorkflowStatus
	}
	w.Status = WorkflowStatusArchived
	w.IsCurrent = false
	w.UpdatedAt = time.Now()
	return nil
}

// CanEdit returns true if the workflow can be edited
func (w *Workflow) CanEdit() bool {
	return w.Status == WorkflowStatusDraft || w.Status == WorkflowStatusInactive
}

// IsActive returns true if the workflow is active
func (w *Workflow) IsActive() bool {
	return w.Status == WorkflowStatusActive
}

// AddStep adds a step to the workflow
func (w *Workflow) AddStep(step WorkflowStep) {
	w.Steps = append(w.Steps, step)
	w.UpdatedAt = time.Now()
}

// GetStepByNumber returns the step with the given step number
func (w *Workflow) GetStepByNumber(stepNumber int) *WorkflowStep {
	for i := range w.Steps {
		if w.Steps[i].StepNumber == stepNumber {
			return &w.Steps[i]
		}
	}
	return nil
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	ID                  common.ID
	WorkflowID          common.ID
	StepNumber          int
	StepName            string
	StepType            StepType
	ApproverType        ApproverType
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
	IsActive            bool
	CreatedAt           time.Time
}

// NewWorkflowStep creates a new workflow step
func NewWorkflowStep(
	workflowID common.ID,
	stepNumber int,
	stepName string,
	stepType StepType,
	approverType ApproverType,
) *WorkflowStep {
	return &WorkflowStep{
		ID:                common.NewID(),
		WorkflowID:        workflowID,
		StepNumber:        stepNumber,
		StepName:          stepName,
		StepType:          stepType,
		ApproverType:      approverType,
		RequiredApprovals: 1,
		AllowSelfApproval: false,
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
}

// SetApprover sets the specific approver for the step
func (s *WorkflowStep) SetApprover(approverID common.ID) {
	s.ApproverID = &approverID
}

// SetApproverExpression sets an expression to determine the approver
func (s *WorkflowStep) SetApproverExpression(expression string) {
	s.ApproverExpression = expression
}

// SetThreshold sets the amount threshold for this step
func (s *WorkflowStep) SetThreshold(min, max *decimal.Decimal, currency string) {
	s.ThresholdMin = min
	s.ThresholdMax = max
	s.ThresholdCurrency = currency
}

// SetEscalation sets escalation settings for the step
func (s *WorkflowStep) SetEscalation(hours int, escalateToStep int) {
	s.EscalationHours = &hours
	s.EscalateToStep = &escalateToStep
}

// IsApplicableForAmount checks if the step applies to the given amount
func (s *WorkflowStep) IsApplicableForAmount(amount decimal.Decimal) bool {
	if s.ThresholdMin != nil && amount.LessThan(*s.ThresholdMin) {
		return false
	}
	if s.ThresholdMax != nil && amount.GreaterThan(*s.ThresholdMax) {
		return false
	}
	return true
}

// Deactivate marks the step as inactive
func (s *WorkflowStep) Deactivate() {
	s.IsActive = false
}
