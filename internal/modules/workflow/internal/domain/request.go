package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

// RequestStatus represents the status of an approval request
type RequestStatus string

const (
	RequestStatusPending    RequestStatus = "pending"
	RequestStatusInProgress RequestStatus = "in_progress"
	RequestStatusApproved   RequestStatus = "approved"
	RequestStatusRejected   RequestStatus = "rejected"
	RequestStatusCancelled  RequestStatus = "cancelled"
	RequestStatusEscalated  RequestStatus = "escalated"
)

// ApprovalRequest represents an approval request for a document
type ApprovalRequest struct {
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
	Status          RequestStatus
	RequestorID     common.ID
	RequestorNotes  string
	Metadata        map[string]any
	StartedAt       time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Related objects
	Workflow *Workflow
	Actions  []ApprovalAction
}

// NewApprovalRequest creates a new approval request
func NewApprovalRequest(
	entityID common.ID,
	requestNumber string,
	workflowID common.ID,
	documentType string,
	documentID common.ID,
	documentNumber string,
	requestorID common.ID,
) *ApprovalRequest {
	now := time.Now()
	return &ApprovalRequest{
		ID:             common.NewID(),
		EntityID:       entityID,
		RequestNumber:  requestNumber,
		WorkflowID:     workflowID,
		DocumentType:   documentType,
		DocumentID:     documentID,
		DocumentNumber: documentNumber,
		CurrentStep:    1,
		Status:         RequestStatusPending,
		RequestorID:    requestorID,
		Metadata:       make(map[string]any),
		StartedAt:      now,
		CreatedAt:      now,
		UpdatedAt:      now,
		Actions:        []ApprovalAction{},
	}
}

// SetAmount sets the amount for threshold-based routing
func (r *ApprovalRequest) SetAmount(amount decimal.Decimal, currencyCode string) {
	r.Amount = &amount
	r.CurrencyCode = currencyCode
	r.UpdatedAt = time.Now()
}

// SetRequestorNotes sets notes from the requestor
func (r *ApprovalRequest) SetRequestorNotes(notes string) {
	r.RequestorNotes = notes
	r.UpdatedAt = time.Now()
}

// SetMetadata sets metadata for the request
func (r *ApprovalRequest) SetMetadata(metadata map[string]any) {
	r.Metadata = metadata
	r.UpdatedAt = time.Now()
}

// AdvanceToStep moves the request to the specified step
func (r *ApprovalRequest) AdvanceToStep(stepNumber int) {
	r.CurrentStep = stepNumber
	r.Status = RequestStatusInProgress
	r.UpdatedAt = time.Now()
}

// Approve marks the request as fully approved
func (r *ApprovalRequest) Approve() error {
	if r.Status == RequestStatusApproved || r.Status == RequestStatusRejected || r.Status == RequestStatusCancelled {
		return ErrInvalidRequestStatus
	}
	now := time.Now()
	r.Status = RequestStatusApproved
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

// Reject marks the request as rejected
func (r *ApprovalRequest) Reject() error {
	if r.Status == RequestStatusApproved || r.Status == RequestStatusRejected || r.Status == RequestStatusCancelled {
		return ErrInvalidRequestStatus
	}
	now := time.Now()
	r.Status = RequestStatusRejected
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

// Cancel cancels the request
func (r *ApprovalRequest) Cancel() error {
	if r.Status == RequestStatusApproved || r.Status == RequestStatusRejected || r.Status == RequestStatusCancelled {
		return ErrInvalidRequestStatus
	}
	now := time.Now()
	r.Status = RequestStatusCancelled
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

// Escalate marks the request as escalated
func (r *ApprovalRequest) Escalate() {
	r.Status = RequestStatusEscalated
	r.UpdatedAt = time.Now()
}

// IsPending returns true if the request is still pending approval
func (r *ApprovalRequest) IsPending() bool {
	return r.Status == RequestStatusPending || r.Status == RequestStatusInProgress || r.Status == RequestStatusEscalated
}

// IsCompleted returns true if the request has been completed
func (r *ApprovalRequest) IsCompleted() bool {
	return r.Status == RequestStatusApproved || r.Status == RequestStatusRejected || r.Status == RequestStatusCancelled
}

// AddAction adds an action to the request
func (r *ApprovalRequest) AddAction(action ApprovalAction) {
	r.Actions = append(r.Actions, action)
	r.UpdatedAt = time.Now()
}

// PendingApproval represents a pending approval assignment
type PendingApproval struct {
	ID           common.ID
	RequestID    common.ID
	StepID       common.ID
	ApproverID   common.ID
	AssignedAt   time.Time
	DueAt        *time.Time
	ReminderSent bool
	Escalated    bool

	// Related objects for response
	Request *ApprovalRequest
	Step    *WorkflowStep
}

// NewPendingApproval creates a new pending approval assignment
func NewPendingApproval(
	requestID common.ID,
	stepID common.ID,
	approverID common.ID,
) *PendingApproval {
	return &PendingApproval{
		ID:           common.NewID(),
		RequestID:    requestID,
		StepID:       stepID,
		ApproverID:   approverID,
		AssignedAt:   time.Now(),
		ReminderSent: false,
		Escalated:    false,
	}
}

// SetDueDate sets the due date for this approval
func (p *PendingApproval) SetDueDate(dueAt time.Time) {
	p.DueAt = &dueAt
}

// MarkReminderSent marks that a reminder has been sent
func (p *PendingApproval) MarkReminderSent() {
	p.ReminderSent = true
}

// MarkEscalated marks that the approval has been escalated
func (p *PendingApproval) MarkEscalated() {
	p.Escalated = true
}

// IsOverdue returns true if the approval is past its due date
func (p *PendingApproval) IsOverdue() bool {
	if p.DueAt == nil {
		return false
	}
	return time.Now().After(*p.DueAt)
}
