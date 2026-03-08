package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type RetentionAction string

const (
	RetentionActionArchive RetentionAction = "archive"
	RetentionActionDelete  RetentionAction = "delete"
	RetentionActionNotify  RetentionAction = "notify"
)

type RetentionPolicy struct {
	ID                     common.ID
	EntityID               *common.ID
	PolicyCode             string
	PolicyName             string
	DocumentType           string
	RetentionDays          int
	ArchiveAfterDays       *int
	DeleteAfterArchiveDays *int
	LegalHoldOverride      bool
	IsDefault              bool
	IsActive               bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func NewRetentionPolicy(
	entityID *common.ID,
	policyCode, policyName string,
	documentType string,
	retentionDays int,
) *RetentionPolicy {
	now := time.Now()
	return &RetentionPolicy{
		ID:                common.NewID(),
		EntityID:          entityID,
		PolicyCode:        policyCode,
		PolicyName:        policyName,
		DocumentType:      documentType,
		RetentionDays:     retentionDays,
		LegalHoldOverride: true,
		IsDefault:         false,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func (p *RetentionPolicy) SetArchivePolicy(archiveAfterDays int, deleteAfterArchiveDays *int) {
	p.ArchiveAfterDays = &archiveAfterDays
	p.DeleteAfterArchiveDays = deleteAfterArchiveDays
	p.UpdatedAt = time.Now()
}

func (p *RetentionPolicy) SetDefault(isDefault bool) {
	p.IsDefault = isDefault
	p.UpdatedAt = time.Now()
}

func (p *RetentionPolicy) Activate() {
	p.IsActive = true
	p.UpdatedAt = time.Now()
}

func (p *RetentionPolicy) Deactivate() {
	p.IsActive = false
	p.UpdatedAt = time.Now()
}

func (p *RetentionPolicy) CalculateExpiryDate(uploadDate time.Time) time.Time {
	return uploadDate.AddDate(0, 0, p.RetentionDays)
}

func (p *RetentionPolicy) CalculateArchiveDate(uploadDate time.Time) *time.Time {
	if p.ArchiveAfterDays == nil {
		return nil
	}
	archiveDate := uploadDate.AddDate(0, 0, *p.ArchiveAfterDays)
	return &archiveDate
}
