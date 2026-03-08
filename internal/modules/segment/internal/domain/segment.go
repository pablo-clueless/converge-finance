package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type SegmentType string

const (
	SegmentTypeOperating   SegmentType = "operating"
	SegmentTypeGeographic  SegmentType = "geographic"
	SegmentTypeProduct     SegmentType = "product"
	SegmentTypeCustomer    SegmentType = "customer"
	SegmentTypeCustom      SegmentType = "custom"
)

func (t SegmentType) IsValid() bool {
	switch t {
	case SegmentTypeOperating, SegmentTypeGeographic, SegmentTypeProduct, SegmentTypeCustomer, SegmentTypeCustom:
		return true
	default:
		return false
	}
}

type Segment struct {
	ID           common.ID
	EntityID     common.ID
	SegmentCode  string
	SegmentName  string
	SegmentType  SegmentType
	ParentID     *common.ID
	Description  string
	ManagerID    *common.ID
	IsReportable bool
	IsActive     bool
	Metadata     map[string]any
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Children []Segment
}

func NewSegment(
	entityID common.ID,
	segmentCode, segmentName string,
	segmentType SegmentType,
) *Segment {
	now := time.Now()
	return &Segment{
		ID:           common.NewID(),
		EntityID:     entityID,
		SegmentCode:  segmentCode,
		SegmentName:  segmentName,
		SegmentType:  segmentType,
		IsReportable: true,
		IsActive:     true,
		Metadata:     make(map[string]any),
		CreatedAt:    now,
		UpdatedAt:    now,
		Children:     []Segment{},
	}
}

func (s *Segment) SetParent(parentID common.ID) {
	s.ParentID = &parentID
	s.UpdatedAt = time.Now()
}

func (s *Segment) SetManager(managerID common.ID) {
	s.ManagerID = &managerID
	s.UpdatedAt = time.Now()
}

func (s *Segment) SetDescription(description string) {
	s.Description = description
	s.UpdatedAt = time.Now()
}

func (s *Segment) SetReportable(isReportable bool) {
	s.IsReportable = isReportable
	s.UpdatedAt = time.Now()
}

func (s *Segment) Activate() {
	s.IsActive = true
	s.UpdatedAt = time.Now()
}

func (s *Segment) Deactivate() {
	s.IsActive = false
	s.UpdatedAt = time.Now()
}

func (s *Segment) Update(name, description string, isReportable bool) {
	if name != "" {
		s.SegmentName = name
	}
	s.Description = description
	s.IsReportable = isReportable
	s.UpdatedAt = time.Now()
}

type SegmentHierarchy struct {
	ID            common.ID
	EntityID      common.ID
	HierarchyCode string
	HierarchyName string
	SegmentType   SegmentType
	Description   string
	IsPrimary     bool
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewSegmentHierarchy(
	entityID common.ID,
	hierarchyCode, hierarchyName string,
	segmentType SegmentType,
) *SegmentHierarchy {
	now := time.Now()
	return &SegmentHierarchy{
		ID:            common.NewID(),
		EntityID:      entityID,
		HierarchyCode: hierarchyCode,
		HierarchyName: hierarchyName,
		SegmentType:   segmentType,
		IsPrimary:     false,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (h *SegmentHierarchy) SetPrimary(isPrimary bool) {
	h.IsPrimary = isPrimary
	h.UpdatedAt = time.Now()
}

func (h *SegmentHierarchy) SetDescription(description string) {
	h.Description = description
	h.UpdatedAt = time.Now()
}

func (h *SegmentHierarchy) Activate() {
	h.IsActive = true
	h.UpdatedAt = time.Now()
}

func (h *SegmentHierarchy) Deactivate() {
	h.IsActive = false
	h.UpdatedAt = time.Now()
}

type SegmentTree struct {
	Segments    []Segment
	SegmentType SegmentType
	TotalCount  int
}

type IntersegmentTransaction struct {
	ID             common.ID
	EntityID       common.ID
	FiscalPeriodID common.ID
	FromSegmentID  common.ID
	ToSegmentID    common.ID
	JournalEntryID *common.ID
	TransactionDate time.Time
	Description    string
	Amount         string
	CurrencyCode   string
	IsEliminated   bool
	CreatedAt      time.Time
}

func NewIntersegmentTransaction(
	entityID, fiscalPeriodID, fromSegmentID, toSegmentID common.ID,
	transactionDate time.Time,
	amount, currencyCode string,
) *IntersegmentTransaction {
	return &IntersegmentTransaction{
		ID:              common.NewID(),
		EntityID:        entityID,
		FiscalPeriodID:  fiscalPeriodID,
		FromSegmentID:   fromSegmentID,
		ToSegmentID:     toSegmentID,
		TransactionDate: transactionDate,
		Amount:          amount,
		CurrencyCode:    currencyCode,
		IsEliminated:    false,
		CreatedAt:       time.Now(),
	}
}

func (t *IntersegmentTransaction) SetJournalEntry(journalEntryID common.ID) {
	t.JournalEntryID = &journalEntryID
}

func (t *IntersegmentTransaction) Eliminate() {
	t.IsEliminated = true
}
