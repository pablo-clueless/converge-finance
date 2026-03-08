package workflow

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

// API defines the public interface for the workflow module
type API interface {
	// Workflow definition methods
	CreateWorkflow(ctx context.Context, req CreateWorkflowRequest) (*WorkflowResponse, error)
	GetWorkflow(ctx context.Context, id common.ID) (*WorkflowResponse, error)
	GetActiveWorkflowForDocument(ctx context.Context, entityID common.ID, documentType string) (*WorkflowResponse, error)
	ListWorkflows(ctx context.Context, req ListWorkflowsRequest) (*ListWorkflowsResponse, error)
	ActivateWorkflow(ctx context.Context, id common.ID) (*WorkflowResponse, error)
	DeactivateWorkflow(ctx context.Context, id common.ID) (*WorkflowResponse, error)

	// Workflow step methods
	AddStep(ctx context.Context, workflowID common.ID, req AddStepRequest) (*WorkflowStepResponse, error)
	RemoveStep(ctx context.Context, workflowID, stepID common.ID) error

	// Approval request methods
	SubmitForApproval(ctx context.Context, req SubmitForApprovalRequest) (*ApprovalRequestResponse, error)
	GetRequest(ctx context.Context, id common.ID) (*ApprovalRequestResponse, error)
	ListRequests(ctx context.Context, req ListRequestsRequest) (*ListRequestsResponse, error)
	Approve(ctx context.Context, requestID, actorID common.ID, comments string) error
	Reject(ctx context.Context, requestID, actorID common.ID, comments string) error
	CancelRequest(ctx context.Context, requestID, actorID common.ID) error
	GetPendingApprovals(ctx context.Context, approverID common.ID) ([]PendingApprovalResponse, error)

	// Delegation methods
	CreateDelegation(ctx context.Context, req CreateDelegationRequest) (*DelegationResponse, error)
	GetDelegation(ctx context.Context, id common.ID) (*DelegationResponse, error)
	ListDelegations(ctx context.Context, entityID common.ID, activeOnly bool) ([]DelegationResponse, error)
	DeactivateDelegation(ctx context.Context, id common.ID) error
}

// CreateWorkflowRequest contains data for creating a workflow
type CreateWorkflowRequest struct {
	EntityID     common.ID
	WorkflowCode string
	WorkflowName string
	Description  string
	DocumentType string
	CreatedBy    common.ID
}

// WorkflowResponse represents a workflow definition
type WorkflowResponse struct {
	ID            common.ID
	EntityID      common.ID
	WorkflowCode  string
	WorkflowName  string
	Description   string
	DocumentType  string
	Status        string
	Version       int
	IsCurrent     bool
	Steps         []WorkflowStepResponse
	CreatedBy     common.ID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// WorkflowStepResponse represents a workflow step
type WorkflowStepResponse struct {
	ID                  common.ID
	WorkflowID          common.ID
	StepNumber          int
	StepName            string
	StepType            string
	ApproverType        string
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

// ListWorkflowsRequest contains filter criteria for listing workflows
type ListWorkflowsRequest struct {
	EntityID     common.ID
	DocumentType string
	Status       string
	CurrentOnly  bool
	Page         int
	PageSize     int
}

// ListWorkflowsResponse contains paginated workflow results
type ListWorkflowsResponse struct {
	Workflows []WorkflowResponse
	Total     int
	Page      int
}

// AddStepRequest contains data for adding a workflow step
type AddStepRequest struct {
	StepNumber          int
	StepName            string
	StepType            string
	ApproverType        string
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

// ApprovalRequestResponse represents an approval request
type ApprovalRequestResponse struct {
	ID              common.ID
	EntityID        common.ID
	RequestNumber   string
	WorkflowID      common.ID
	DocumentType    string
	DocumentID      common.ID
	DocumentNumber  string
	Amount          *decimal.Decimal
	CurrencyCode    string
	CurrentStep     int
	Status          string
	RequestorID     common.ID
	RequestorNotes  string
	Metadata        map[string]any
	StartedAt       time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Actions         []ApprovalActionResponse
}

// ApprovalActionResponse represents an approval action
type ApprovalActionResponse struct {
	ID          common.ID
	RequestID   common.ID
	StepID      common.ID
	StepNumber  int
	ActionType  string
	ActorID     common.ID
	DelegatedBy *common.ID
	Comments    string
	ActedAt     time.Time
}

// ListRequestsRequest contains filter criteria for listing requests
type ListRequestsRequest struct {
	EntityID     common.ID
	WorkflowID   common.ID
	DocumentType string
	Status       string
	RequestorID  common.ID
	DateFrom     *time.Time
	DateTo       *time.Time
	Page         int
	PageSize     int
}

// ListRequestsResponse contains paginated request results
type ListRequestsResponse struct {
	Requests []ApprovalRequestResponse
	Total    int
	Page     int
}

// PendingApprovalResponse represents a pending approval
type PendingApprovalResponse struct {
	ID           common.ID
	RequestID    common.ID
	StepID       common.ID
	ApproverID   common.ID
	AssignedAt   time.Time
	DueAt        *time.Time
	ReminderSent bool
	Escalated    bool
	Request      *ApprovalRequestResponse
	Step         *WorkflowStepResponse
}

// CreateDelegationRequest contains data for creating a delegation
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

// DelegationResponse represents a delegation
type DelegationResponse struct {
	ID            common.ID
	EntityID      common.ID
	DelegatorID   common.ID
	DelegateID    common.ID
	WorkflowID    *common.ID
	DocumentTypes []string
	StartDate     time.Time
	EndDate       *time.Time
	Reason        string
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
