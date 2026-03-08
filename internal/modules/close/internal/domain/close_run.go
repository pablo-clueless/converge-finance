package domain

import (
	"errors"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type CloseRunStatus string

const (
	CloseRunStatusPending    CloseRunStatus = "pending"
	CloseRunStatusProcessing CloseRunStatus = "processing"
	CloseRunStatusCompleted  CloseRunStatus = "completed"
	CloseRunStatusFailed     CloseRunStatus = "failed"
	CloseRunStatusReversed   CloseRunStatus = "reversed"
)

func (s CloseRunStatus) IsValid() bool {
	switch s {
	case CloseRunStatusPending, CloseRunStatusProcessing, CloseRunStatusCompleted,
		CloseRunStatusFailed, CloseRunStatusReversed:
		return true
	}
	return false
}

func (s CloseRunStatus) String() string {
	return string(s)
}

type CloseRun struct {
	ID                     common.ID
	EntityID               common.ID
	RunNumber              string
	CloseType              CloseType
	FiscalPeriodID         common.ID
	FiscalYearID           common.ID
	CloseDate              time.Time
	Status                 CloseRunStatus
	RulesExecuted          int
	EntriesCreated         int
	TotalDebits            money.Money
	TotalCredits           money.Money
	Currency               money.Currency
	ClosingJournalEntryID  *common.ID
	ReversalJournalEntryID *common.ID
	InitiatedBy            common.ID
	CompletedAt            *time.Time
	ReversedAt             *time.Time
	ReversedBy             *common.ID
	ErrorMessage           string
	CreatedAt              time.Time
	UpdatedAt              time.Time

	Entries []CloseRunEntry
}

func NewCloseRun(
	entityID common.ID,
	runNumber string,
	closeType CloseType,
	fiscalPeriodID, fiscalYearID common.ID,
	closeDate time.Time,
	currency money.Currency,
	initiatedBy common.ID,
) *CloseRun {
	now := time.Now()
	return &CloseRun{
		ID:             common.NewID(),
		EntityID:       entityID,
		RunNumber:      runNumber,
		CloseType:      closeType,
		FiscalPeriodID: fiscalPeriodID,
		FiscalYearID:   fiscalYearID,
		CloseDate:      closeDate,
		Status:         CloseRunStatusPending,
		TotalDebits:    money.Zero(currency),
		TotalCredits:   money.Zero(currency),
		Currency:       currency,
		InitiatedBy:    initiatedBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (r *CloseRun) StartProcessing() error {
	if r.Status != CloseRunStatusPending {
		return errors.New("close run must be pending to start processing")
	}

	r.Status = CloseRunStatusProcessing
	r.UpdatedAt = time.Now()
	return nil
}

func (r *CloseRun) Complete(journalEntryID common.ID, rulesExecuted, entriesCreated int, totalDebits, totalCredits money.Money) error {
	if r.Status != CloseRunStatusProcessing {
		return errors.New("close run must be processing to complete")
	}

	now := time.Now()
	r.Status = CloseRunStatusCompleted
	r.ClosingJournalEntryID = &journalEntryID
	r.RulesExecuted = rulesExecuted
	r.EntriesCreated = entriesCreated
	r.TotalDebits = totalDebits
	r.TotalCredits = totalCredits
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *CloseRun) Fail(errorMessage string) error {
	if r.Status != CloseRunStatusProcessing {
		return errors.New("close run must be processing to fail")
	}

	r.Status = CloseRunStatusFailed
	r.ErrorMessage = errorMessage
	r.UpdatedAt = time.Now()
	return nil
}

func (r *CloseRun) Reverse(userID common.ID, reversalJournalEntryID common.ID) error {
	if r.Status != CloseRunStatusCompleted {
		return errors.New("can only reverse completed close runs")
	}

	now := time.Now()
	r.Status = CloseRunStatusReversed
	r.ReversalJournalEntryID = &reversalJournalEntryID
	r.ReversedAt = &now
	r.ReversedBy = &userID
	r.UpdatedAt = now
	return nil
}

func (r *CloseRun) AddEntry(entry CloseRunEntry) {
	r.Entries = append(r.Entries, entry)
}

type CloseRunEntry struct {
	ID                common.ID
	CloseRunID        common.ID
	CloseRuleID       common.ID
	SequenceNumber    int
	SourceAccountID   common.ID
	SourceAccountCode string
	SourceAccountName string
	TargetAccountID   common.ID
	TargetAccountCode string
	TargetAccountName string
	Amount            money.Money
	Currency          money.Currency
	Description       string
	CreatedAt         time.Time
}

func NewCloseRunEntry(
	closeRunID, closeRuleID common.ID,
	sequenceNumber int,
	sourceAccountID common.ID,
	sourceAccountCode, sourceAccountName string,
	targetAccountID common.ID,
	targetAccountCode, targetAccountName string,
	amount money.Money,
	description string,
) *CloseRunEntry {
	return &CloseRunEntry{
		ID:                common.NewID(),
		CloseRunID:        closeRunID,
		CloseRuleID:       closeRuleID,
		SequenceNumber:    sequenceNumber,
		SourceAccountID:   sourceAccountID,
		SourceAccountCode: sourceAccountCode,
		SourceAccountName: sourceAccountName,
		TargetAccountID:   targetAccountID,
		TargetAccountCode: targetAccountCode,
		TargetAccountName: targetAccountName,
		Amount:            amount,
		Currency:          amount.Currency,
		Description:       description,
		CreatedAt:         time.Now(),
	}
}

type CloseRunFilter struct {
	EntityID       *common.ID
	CloseType      *CloseType
	FiscalPeriodID *common.ID
	FiscalYearID   *common.ID
	Status         *CloseRunStatus
	Limit          int
	Offset         int
}
