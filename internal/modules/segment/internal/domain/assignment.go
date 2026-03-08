package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

type AllocationBasis string

const (
	AllocationBasisDirect       AllocationBasis = "direct"
	AllocationBasisRevenue      AllocationBasis = "revenue"
	AllocationBasisHeadcount    AllocationBasis = "headcount"
	AllocationBasisSquareFootage AllocationBasis = "square_footage"
	AllocationBasisCustom       AllocationBasis = "custom"
)

func (b AllocationBasis) IsValid() bool {
	switch b {
	case AllocationBasisDirect, AllocationBasisRevenue, AllocationBasisHeadcount, AllocationBasisSquareFootage, AllocationBasisCustom:
		return true
	default:
		return false
	}
}

type Assignment struct {
	ID               common.ID
	EntityID         common.ID
	SegmentID        common.ID
	AssignmentType   string
	AssignmentID     common.ID
	AllocationPercent decimal.Decimal
	EffectiveFrom    time.Time
	EffectiveTo      *time.Time
	IsActive         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewAssignment(
	entityID, segmentID common.ID,
	assignmentType string,
	assignmentID common.ID,
	allocationPercent decimal.Decimal,
	effectiveFrom time.Time,
) (*Assignment, error) {
	if allocationPercent.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidAllocationPercent
	}
	if allocationPercent.GreaterThan(decimal.NewFromInt(100)) {
		return nil, ErrInvalidAllocationPercent
	}

	now := time.Now()
	return &Assignment{
		ID:               common.NewID(),
		EntityID:         entityID,
		SegmentID:        segmentID,
		AssignmentType:   assignmentType,
		AssignmentID:     assignmentID,
		AllocationPercent: allocationPercent,
		EffectiveFrom:    effectiveFrom,
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (a *Assignment) SetEffectiveTo(effectiveTo time.Time) error {
	if effectiveTo.Before(a.EffectiveFrom) {
		return ErrInvalidEffectiveDates
	}
	a.EffectiveTo = &effectiveTo
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Assignment) UpdateAllocationPercent(percent decimal.Decimal) error {
	if percent.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidAllocationPercent
	}
	if percent.GreaterThan(decimal.NewFromInt(100)) {
		return ErrInvalidAllocationPercent
	}
	a.AllocationPercent = percent
	a.UpdatedAt = time.Now()
	return nil
}

func (a *Assignment) Deactivate() {
	a.IsActive = false
	a.UpdatedAt = time.Now()
}

func (a *Assignment) Activate() {
	a.IsActive = true
	a.UpdatedAt = time.Now()
}

func (a *Assignment) IsEffective(date time.Time) bool {
	if !a.IsActive {
		return false
	}
	if date.Before(a.EffectiveFrom) {
		return false
	}
	if a.EffectiveTo != nil && date.After(*a.EffectiveTo) {
		return false
	}
	return true
}
