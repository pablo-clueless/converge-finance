package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type AllocationMethod string

const (
	AllocationMethodDirect       AllocationMethod = "direct"
	AllocationMethodHeadcount    AllocationMethod = "headcount"
	AllocationMethodSquareFootage AllocationMethod = "square_footage"
	AllocationMethodRevenue      AllocationMethod = "revenue"
	AllocationMethodUsage        AllocationMethod = "usage"
	AllocationMethodActivity     AllocationMethod = "activity"
	AllocationMethodFixedPercent AllocationMethod = "fixed_percent"
	AllocationMethodStepDown     AllocationMethod = "step_down"
	AllocationMethodReciprocal   AllocationMethod = "reciprocal"
)

func (m AllocationMethod) IsValid() bool {
	switch m {
	case AllocationMethodDirect, AllocationMethodHeadcount, AllocationMethodSquareFootage,
		AllocationMethodRevenue, AllocationMethodUsage, AllocationMethodActivity,
		AllocationMethodFixedPercent, AllocationMethodStepDown, AllocationMethodReciprocal:
		return true
	}
	return false
}

type AllocationStatus string

const (
	AllocationStatusDraft      AllocationStatus = "draft"
	AllocationStatusInProgress AllocationStatus = "in_progress"
	AllocationStatusCompleted  AllocationStatus = "completed"
	AllocationStatusPosted     AllocationStatus = "posted"
	AllocationStatusReversed   AllocationStatus = "reversed"
)

func (s AllocationStatus) IsValid() bool {
	switch s {
	case AllocationStatusDraft, AllocationStatusInProgress, AllocationStatusCompleted,
		AllocationStatusPosted, AllocationStatusReversed:
		return true
	}
	return false
}

type AllocationRule struct {
	ID                 common.ID
	EntityID           common.ID
	RuleCode           string
	RuleName           string
	Description        string
	SourceCostCenterID common.ID
	AllocationMethod   AllocationMethod
	AccountFilter      map[string]interface{}
	SequenceNumber     int
	IsActive           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time

	SourceCostCenterCode string
	SourceCostCenterName string
	Targets              []AllocationTarget
}

func NewAllocationRule(
	entityID common.ID,
	ruleCode string,
	ruleName string,
	sourceCostCenterID common.ID,
	allocationMethod AllocationMethod,
) (*AllocationRule, error) {
	if ruleCode == "" {
		return nil, fmt.Errorf("rule code is required")
	}
	if ruleName == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	if !allocationMethod.IsValid() {
		return nil, fmt.Errorf("invalid allocation method")
	}

	return &AllocationRule{
		ID:                 common.NewID(),
		EntityID:           entityID,
		RuleCode:           ruleCode,
		RuleName:           ruleName,
		SourceCostCenterID: sourceCostCenterID,
		AllocationMethod:   allocationMethod,
		AccountFilter:      make(map[string]interface{}),
		IsActive:           true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}, nil
}

func (r *AllocationRule) AddTarget(target AllocationTarget) {
	target.AllocationRuleID = r.ID
	r.Targets = append(r.Targets, target)
	r.UpdatedAt = time.Now()
}

func (r *AllocationRule) SetSequence(sequence int) {
	r.SequenceNumber = sequence
	r.UpdatedAt = time.Now()
}

func (r *AllocationRule) Deactivate() {
	r.IsActive = false
	r.UpdatedAt = time.Now()
}

type AllocationTarget struct {
	ID                 common.ID
	AllocationRuleID   common.ID
	TargetCostCenterID common.ID
	FixedPercent       *float64
	DriverValue        *float64
	IsActive           bool
	CreatedAt          time.Time

	TargetCostCenterCode string
	TargetCostCenterName string
}

func NewAllocationTarget(
	targetCostCenterID common.ID,
	fixedPercent *float64,
	driverValue *float64,
) (*AllocationTarget, error) {
	if fixedPercent != nil && (*fixedPercent < 0 || *fixedPercent > 100) {
		return nil, fmt.Errorf("fixed percent must be between 0 and 100")
	}

	return &AllocationTarget{
		ID:                 common.NewID(),
		TargetCostCenterID: targetCostCenterID,
		FixedPercent:       fixedPercent,
		DriverValue:        driverValue,
		IsActive:           true,
		CreatedAt:          time.Now(),
	}, nil
}

type AllocationRun struct {
	ID             common.ID
	EntityID       common.ID
	RunNumber      string
	FiscalPeriodID common.ID
	AllocationDate time.Time
	RulesExecuted  int
	TotalAllocated money.Money
	Status         AllocationStatus
	JournalEntryID *common.ID
	CreatedBy      common.ID
	CompletedBy    *common.ID
	PostedBy       *common.ID
	ReversedBy     *common.ID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
	PostedAt       *time.Time
	ReversedAt     *time.Time

	FiscalPeriodName string
	Entries          []AllocationEntry
}

func NewAllocationRun(
	entityID common.ID,
	runNumber string,
	fiscalPeriodID common.ID,
	allocationDate time.Time,
	createdBy common.ID,
	currency money.Currency,
) (*AllocationRun, error) {
	if runNumber == "" {
		return nil, fmt.Errorf("run number is required")
	}

	return &AllocationRun{
		ID:             common.NewID(),
		EntityID:       entityID,
		RunNumber:      runNumber,
		FiscalPeriodID: fiscalPeriodID,
		AllocationDate: allocationDate,
		TotalAllocated: money.Zero(currency),
		Status:         AllocationStatusDraft,
		CreatedBy:      createdBy,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, nil
}

func (r *AllocationRun) StartProcessing() error {
	if r.Status != AllocationStatusDraft {
		return fmt.Errorf("can only start from draft status")
	}
	r.Status = AllocationStatusInProgress
	r.UpdatedAt = time.Now()
	return nil
}

func (r *AllocationRun) Complete(completedBy common.ID, rulesExecuted int, totalAllocated money.Money) error {
	if r.Status != AllocationStatusInProgress {
		return fmt.Errorf("can only complete from in_progress status")
	}
	r.Status = AllocationStatusCompleted
	r.CompletedBy = &completedBy
	r.RulesExecuted = rulesExecuted
	r.TotalAllocated = totalAllocated
	now := time.Now()
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *AllocationRun) Post(postedBy common.ID, journalEntryID common.ID) error {
	if r.Status != AllocationStatusCompleted {
		return fmt.Errorf("can only post from completed status")
	}
	r.Status = AllocationStatusPosted
	r.PostedBy = &postedBy
	r.JournalEntryID = &journalEntryID
	now := time.Now()
	r.PostedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *AllocationRun) Reverse(reversedBy common.ID) error {
	if r.Status != AllocationStatusPosted {
		return fmt.Errorf("can only reverse from posted status")
	}
	r.Status = AllocationStatusReversed
	r.ReversedBy = &reversedBy
	now := time.Now()
	r.ReversedAt = &now
	r.UpdatedAt = now
	return nil
}

type AllocationEntry struct {
	ID                 common.ID
	AllocationRunID    common.ID
	AllocationRuleID   *common.ID
	LineNumber         int
	SourceCostCenterID common.ID
	SourceAccountID    common.ID
	TargetCostCenterID common.ID
	TargetAccountID    common.ID
	AllocationPercent  float64
	AllocatedAmount    money.Money
	Description        string
	CreatedAt          time.Time

	SourceCostCenterCode string
	SourceAccountCode    string
	TargetCostCenterCode string
	TargetAccountCode    string
}

func NewAllocationEntry(
	allocationRunID common.ID,
	lineNumber int,
	sourceCostCenterID common.ID,
	sourceAccountID common.ID,
	targetCostCenterID common.ID,
	targetAccountID common.ID,
	allocationPercent float64,
	allocatedAmount money.Money,
) *AllocationEntry {
	return &AllocationEntry{
		ID:                 common.NewID(),
		AllocationRunID:    allocationRunID,
		LineNumber:         lineNumber,
		SourceCostCenterID: sourceCostCenterID,
		SourceAccountID:    sourceAccountID,
		TargetCostCenterID: targetCostCenterID,
		TargetAccountID:    targetAccountID,
		AllocationPercent:  allocationPercent,
		AllocatedAmount:    allocatedAmount,
		CreatedAt:          time.Now(),
	}
}

type AllocationRuleFilter struct {
	EntityID           *common.ID
	SourceCostCenterID *common.ID
	AllocationMethod   *AllocationMethod
	IsActive           *bool
	Limit              int
	Offset             int
}

type AllocationRunFilter struct {
	EntityID       *common.ID
	FiscalPeriodID *common.ID
	Status         *AllocationStatus
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}
