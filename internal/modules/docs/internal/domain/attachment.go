package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type Attachment struct {
	ID            common.ID
	DocumentID    common.ID
	EntityID      common.ID
	ReferenceType string
	ReferenceID   common.ID
	IsPrimary     bool
	AttachedBy    common.ID
	AttachedAt    time.Time
}

func NewAttachment(
	documentID common.ID,
	entityID common.ID,
	referenceType string,
	referenceID common.ID,
	attachedBy common.ID,
) *Attachment {
	return &Attachment{
		ID:            common.NewID(),
		DocumentID:    documentID,
		EntityID:      entityID,
		ReferenceType: referenceType,
		ReferenceID:   referenceID,
		IsPrimary:     false,
		AttachedBy:    attachedBy,
		AttachedAt:    time.Now(),
	}
}

func (a *Attachment) SetPrimary(isPrimary bool) {
	a.IsPrimary = isPrimary
}

type AttachmentWithDocument struct {
	Attachment
	Document *Document
}
