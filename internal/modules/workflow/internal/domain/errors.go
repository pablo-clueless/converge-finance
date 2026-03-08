package domain

import "errors"

var (
	// Workflow errors
	ErrWorkflowNotFound      = errors.New("workflow not found")
	ErrWorkflowAlreadyExists = errors.New("workflow with this code already exists")
	ErrInvalidWorkflowStatus = errors.New("invalid workflow status for this operation")
	ErrNoWorkflowSteps       = errors.New("workflow must have at least one step")
	ErrWorkflowNotActive     = errors.New("workflow is not active")

	// Step errors
	ErrStepNotFound       = errors.New("workflow step not found")
	ErrInvalidStepNumber  = errors.New("invalid step number")
	ErrDuplicateStepNumber = errors.New("step number already exists")
	ErrInvalidStepType    = errors.New("invalid step type")
	ErrInvalidApproverType = errors.New("invalid approver type")

	// Request errors
	ErrRequestNotFound       = errors.New("approval request not found")
	ErrInvalidRequestStatus  = errors.New("invalid request status for this operation")
	ErrRequestAlreadyExists  = errors.New("approval request already exists for this document")
	ErrNoApplicableWorkflow  = errors.New("no applicable workflow found for this document")
	ErrInvalidDocumentType   = errors.New("invalid document type")

	// Action errors
	ErrActionNotFound        = errors.New("approval action not found")
	ErrInvalidActionType     = errors.New("invalid action type")
	ErrNotAuthorizedToApprove = errors.New("not authorized to approve this request")
	ErrSelfApprovalNotAllowed = errors.New("self-approval is not allowed for this step")
	ErrAlreadyActedOnStep    = errors.New("already acted on this step")

	// Pending approval errors
	ErrPendingApprovalNotFound = errors.New("pending approval not found")
	ErrNoPendingApprovals      = errors.New("no pending approvals for this approver")

	// Delegation errors
	ErrDelegationNotFound      = errors.New("delegation not found")
	ErrSelfDelegation          = errors.New("cannot delegate to yourself")
	ErrInvalidDelegationDates  = errors.New("end date cannot be before start date")
	ErrDelegationNotEffective  = errors.New("delegation is not currently effective")
	ErrCircularDelegation      = errors.New("circular delegation detected")
)
