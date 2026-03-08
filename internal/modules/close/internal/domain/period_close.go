package domain

import (
	"errors"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type PeriodCloseStatus string

const (
	PeriodStatusOpen       PeriodCloseStatus = "open"
	PeriodStatusSoftClosed PeriodCloseStatus = "soft_closed"
	PeriodStatusHardClosed PeriodCloseStatus = "hard_closed"
)

func (s PeriodCloseStatus) IsValid() bool {
	switch s {
	case PeriodStatusOpen, PeriodStatusSoftClosed, PeriodStatusHardClosed:
		return true
	}
	return false
}

func (s PeriodCloseStatus) String() string {
	return string(s)
}

type CloseType string

const (
	CloseTypeDay     CloseType = "day"
	CloseTypePeriod  CloseType = "period"
	CloseTypeQuarter CloseType = "quarter"
	CloseTypeYear    CloseType = "year"
)

func (t CloseType) IsValid() bool {
	switch t {
	case CloseTypeDay, CloseTypePeriod, CloseTypeQuarter, CloseTypeYear:
		return true
	}
	return false
}

func (t CloseType) String() string {
	return string(t)
}

type PeriodClose struct {
	ID                    common.ID
	EntityID              common.ID
	FiscalPeriodID        common.ID
	FiscalYearID          common.ID
	Status                PeriodCloseStatus
	SoftClosedAt          *time.Time
	SoftClosedBy          *common.ID
	HardClosedAt          *time.Time
	HardClosedBy          *common.ID
	ReopenedAt            *time.Time
	ReopenedBy            *common.ID
	ReopenReason          string
	ClosingJournalEntryID *common.ID
	Notes                 string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func NewPeriodClose(entityID, fiscalPeriodID, fiscalYearID common.ID) *PeriodClose {
	now := time.Now()
	return &PeriodClose{
		ID:             common.NewID(),
		EntityID:       entityID,
		FiscalPeriodID: fiscalPeriodID,
		FiscalYearID:   fiscalYearID,
		Status:         PeriodStatusOpen,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (p *PeriodClose) SoftClose(userID common.ID) error {
	if p.Status != PeriodStatusOpen {
		return errors.New("period must be open to soft close")
	}

	now := time.Now()
	p.Status = PeriodStatusSoftClosed
	p.SoftClosedAt = &now
	p.SoftClosedBy = &userID
	p.UpdatedAt = now
	return nil
}

func (p *PeriodClose) HardClose(userID common.ID, journalEntryID *common.ID) error {
	if p.Status != PeriodStatusSoftClosed {
		return errors.New("period must be soft closed before hard close")
	}

	now := time.Now()
	p.Status = PeriodStatusHardClosed
	p.HardClosedAt = &now
	p.HardClosedBy = &userID
	p.ClosingJournalEntryID = journalEntryID
	p.UpdatedAt = now
	return nil
}

func (p *PeriodClose) Reopen(userID common.ID, reason string) error {
	if p.Status == PeriodStatusOpen {
		return errors.New("period is already open")
	}

	now := time.Now()
	p.Status = PeriodStatusOpen
	p.ReopenedAt = &now
	p.ReopenedBy = &userID
	p.ReopenReason = reason
	p.SoftClosedAt = nil
	p.SoftClosedBy = nil
	p.HardClosedAt = nil
	p.HardClosedBy = nil
	p.ClosingJournalEntryID = nil
	p.UpdatedAt = now
	return nil
}

func (p *PeriodClose) IsOpen() bool {
	return p.Status == PeriodStatusOpen
}

func (p *PeriodClose) IsClosed() bool {
	return p.Status == PeriodStatusSoftClosed || p.Status == PeriodStatusHardClosed
}

func (p *PeriodClose) AllowsPosting() bool {
	return p.Status == PeriodStatusOpen
}

func (p *PeriodClose) AllowsAdjustments() bool {
	return p.Status == PeriodStatusOpen || p.Status == PeriodStatusSoftClosed
}

type PeriodCloseFilter struct {
	EntityID       *common.ID
	FiscalPeriodID *common.ID
	FiscalYearID   *common.ID
	Status         *PeriodCloseStatus
	Limit          int
	Offset         int
}
