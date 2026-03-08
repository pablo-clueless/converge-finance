package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

// ActionType represents the type of action taken on a request
type ActionType string

const (
	ActionTypeApprove     ActionType = "approve"
	ActionTypeReject      ActionType = "reject"
	ActionTypeRequestInfo ActionType = "request_info"
	ActionTypeDelegate    ActionType = "delegate"
	ActionTypeEscalate    ActionType = "escalate"
)

// ApprovalAction represents an action taken on an approval request
type ApprovalAction struct {
	ID          common.ID
	RequestID   common.ID
	StepID      common.ID
	StepNumber  int
	ActionType  ActionType
	ActorID     common.ID
	DelegatedBy *common.ID
	Comments    string
	ActedAt     time.Time
}

// NewApprovalAction creates a new approval action
func NewApprovalAction(
	requestID common.ID,
	stepID common.ID,
	stepNumber int,
	actionType ActionType,
	actorID common.ID,
	comments string,
) *ApprovalAction {
	return &ApprovalAction{
		ID:         common.NewID(),
		RequestID:  requestID,
		StepID:     stepID,
		StepNumber: stepNumber,
		ActionType: actionType,
		ActorID:    actorID,
		Comments:   comments,
		ActedAt:    time.Now(),
	}
}

// SetDelegatedBy sets who delegated this action
func (a *ApprovalAction) SetDelegatedBy(delegatorID common.ID) {
	a.DelegatedBy = &delegatorID
}

// IsApproval returns true if the action is an approval
func (a *ApprovalAction) IsApproval() bool {
	return a.ActionType == ActionTypeApprove
}

// IsRejection returns true if the action is a rejection
func (a *ApprovalAction) IsRejection() bool {
	return a.ActionType == ActionTypeReject
}

// IsDelegation returns true if the action is a delegation
func (a *ApprovalAction) IsDelegation() bool {
	return a.ActionType == ActionTypeDelegate
}

// IsEscalation returns true if the action is an escalation
func (a *ApprovalAction) IsEscalation() bool {
	return a.ActionType == ActionTypeEscalate
}
