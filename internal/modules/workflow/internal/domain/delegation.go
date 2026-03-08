package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

// Delegation represents a delegation of approval authority
type Delegation struct {
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

// NewDelegation creates a new delegation
func NewDelegation(
	entityID common.ID,
	delegatorID common.ID,
	delegateID common.ID,
	startDate time.Time,
	reason string,
) (*Delegation, error) {
	if delegatorID == delegateID {
		return nil, ErrSelfDelegation
	}

	now := time.Now()
	return &Delegation{
		ID:            common.NewID(),
		EntityID:      entityID,
		DelegatorID:   delegatorID,
		DelegateID:    delegateID,
		DocumentTypes: []string{},
		StartDate:     startDate,
		Reason:        reason,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// SetEndDate sets the end date for the delegation
func (d *Delegation) SetEndDate(endDate time.Time) error {
	if endDate.Before(d.StartDate) {
		return ErrInvalidDelegationDates
	}
	d.EndDate = &endDate
	d.UpdatedAt = time.Now()
	return nil
}

// SetWorkflow limits delegation to a specific workflow
func (d *Delegation) SetWorkflow(workflowID common.ID) {
	d.WorkflowID = &workflowID
	d.UpdatedAt = time.Now()
}

// SetDocumentTypes limits delegation to specific document types
func (d *Delegation) SetDocumentTypes(documentTypes []string) {
	d.DocumentTypes = documentTypes
	d.UpdatedAt = time.Now()
}

// Deactivate deactivates the delegation
func (d *Delegation) Deactivate() {
	d.IsActive = false
	d.UpdatedAt = time.Now()
}

// Activate activates the delegation
func (d *Delegation) Activate() {
	d.IsActive = true
	d.UpdatedAt = time.Now()
}

// IsEffective returns true if the delegation is currently effective
func (d *Delegation) IsEffective() bool {
	if !d.IsActive {
		return false
	}

	now := time.Now()

	// Check if start date has passed
	if now.Before(d.StartDate) {
		return false
	}

	// Check if end date exists and has passed
	if d.EndDate != nil && now.After(*d.EndDate) {
		return false
	}

	return true
}

// AppliesToDocumentType checks if the delegation applies to a document type
func (d *Delegation) AppliesToDocumentType(documentType string) bool {
	// If no document types specified, applies to all
	if len(d.DocumentTypes) == 0 {
		return true
	}

	for _, dt := range d.DocumentTypes {
		if dt == documentType {
			return true
		}
	}

	return false
}

// AppliesToWorkflow checks if the delegation applies to a workflow
func (d *Delegation) AppliesToWorkflow(workflowID common.ID) bool {
	// If no workflow specified, applies to all
	if d.WorkflowID == nil {
		return true
	}

	return *d.WorkflowID == workflowID
}
