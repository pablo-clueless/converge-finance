package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type AccountFXTreatment string

const (
	AccountFXTreatmentMonetary    AccountFXTreatment = "monetary"
	AccountFXTreatmentNonMonetary AccountFXTreatment = "nonmonetary"
	AccountFXTreatmentExcluded    AccountFXTreatment = "excluded"
)

type RevaluationStatus string

const (
	RevaluationStatusDraft           RevaluationStatus = "draft"
	RevaluationStatusPendingApproval RevaluationStatus = "pending_approval"
	RevaluationStatusApproved        RevaluationStatus = "approved"
	RevaluationStatusPosted          RevaluationStatus = "posted"
	RevaluationStatusReversed        RevaluationStatus = "reversed"
)

type RevaluationType string

const (
	RevaluationTypeUnrealized RevaluationType = "unrealized"
	RevaluationTypeRealized   RevaluationType = "realized"
)

type AccountFXConfig struct {
	ID                       common.ID
	EntityID                 common.ID
	AccountID                common.ID
	FXTreatment              AccountFXTreatment
	RevaluationGainAccountID *common.ID
	RevaluationLossAccountID *common.ID
	IsActive                 bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func NewAccountFXConfig(
	entityID, accountID common.ID,
	treatment AccountFXTreatment,
) *AccountFXConfig {
	now := time.Now()
	return &AccountFXConfig{
		ID:          common.NewID(),
		EntityID:    entityID,
		AccountID:   accountID,
		FXTreatment: treatment,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (c *AccountFXConfig) SetGainLossAccounts(gainAccountID, lossAccountID common.ID) {
	c.RevaluationGainAccountID = &gainAccountID
	c.RevaluationLossAccountID = &lossAccountID
	c.UpdatedAt = time.Now()
}

func (c *AccountFXConfig) SetTreatment(treatment AccountFXTreatment) {
	c.FXTreatment = treatment
	c.UpdatedAt = time.Now()
}

type RevaluationRun struct {
	ID                     common.ID
	EntityID               common.ID
	RunNumber              string
	FiscalPeriodID         common.ID
	RevaluationDate        time.Time
	RateDate               time.Time
	FunctionalCurrency     money.Currency
	Status                 RevaluationStatus
	TotalUnrealizedGain    decimal.Decimal
	TotalUnrealizedLoss    decimal.Decimal
	NetRevaluation         decimal.Decimal
	AccountsProcessed      int
	JournalEntryID         *common.ID
	ReversalJournalEntryID *common.ID
	CreatedBy              common.ID
	ApprovedBy             *common.ID
	PostedBy               *common.ID
	ReversedBy             *common.ID
	CreatedAt              time.Time
	UpdatedAt              time.Time
	ApprovedAt             *time.Time
	PostedAt               *time.Time
	ReversedAt             *time.Time

	Details []RevaluationDetail
}

func NewRevaluationRun(
	entityID common.ID,
	runNumber string,
	fiscalPeriodID common.ID,
	revaluationDate, rateDate time.Time,
	functionalCurrency money.Currency,
	createdBy common.ID,
) *RevaluationRun {
	now := time.Now()
	return &RevaluationRun{
		ID:                  common.NewID(),
		EntityID:            entityID,
		RunNumber:           runNumber,
		FiscalPeriodID:      fiscalPeriodID,
		RevaluationDate:     revaluationDate,
		RateDate:            rateDate,
		FunctionalCurrency:  functionalCurrency,
		Status:              RevaluationStatusDraft,
		TotalUnrealizedGain: decimal.Zero,
		TotalUnrealizedLoss: decimal.Zero,
		NetRevaluation:      decimal.Zero,
		AccountsProcessed:   0,
		CreatedBy:           createdBy,
		CreatedAt:           now,
		UpdatedAt:           now,
		Details:             []RevaluationDetail{},
	}
}

func (r *RevaluationRun) SubmitForApproval() error {
	if r.Status != RevaluationStatusDraft {
		return ErrInvalidRevaluationStatus
	}
	if len(r.Details) == 0 {
		return ErrNoRevaluationDetails
	}
	r.Status = RevaluationStatusPendingApproval
	r.UpdatedAt = time.Now()
	return nil
}

func (r *RevaluationRun) Approve(approverID common.ID) error {
	if r.Status != RevaluationStatusPendingApproval {
		return ErrInvalidRevaluationStatus
	}
	now := time.Now()
	r.Status = RevaluationStatusApproved
	r.ApprovedBy = &approverID
	r.ApprovedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *RevaluationRun) Post(posterID common.ID, journalEntryID common.ID) error {
	if r.Status != RevaluationStatusApproved {
		return ErrInvalidRevaluationStatus
	}
	now := time.Now()
	r.Status = RevaluationStatusPosted
	r.PostedBy = &posterID
	r.JournalEntryID = &journalEntryID
	r.PostedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *RevaluationRun) Reverse(reversedBy common.ID, reversalJournalID common.ID) error {
	if r.Status != RevaluationStatusPosted {
		return ErrInvalidRevaluationStatus
	}
	now := time.Now()
	r.Status = RevaluationStatusReversed
	r.ReversedBy = &reversedBy
	r.ReversalJournalEntryID = &reversalJournalID
	r.ReversedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *RevaluationRun) AddDetail(detail RevaluationDetail) {
	r.Details = append(r.Details, detail)
	r.AccountsProcessed = len(r.Details)

	if detail.RevaluationAmount.GreaterThan(decimal.Zero) {
		r.TotalUnrealizedGain = r.TotalUnrealizedGain.Add(detail.RevaluationAmount)
	} else {
		r.TotalUnrealizedLoss = r.TotalUnrealizedLoss.Add(detail.RevaluationAmount.Abs())
	}
	r.NetRevaluation = r.TotalUnrealizedGain.Sub(r.TotalUnrealizedLoss)
	r.UpdatedAt = time.Now()
}

func (r *RevaluationRun) CanEdit() bool {
	return r.Status == RevaluationStatusDraft
}

func (r *RevaluationRun) CanApprove() bool {
	return r.Status == RevaluationStatusPendingApproval
}

func (r *RevaluationRun) CanPost() bool {
	return r.Status == RevaluationStatusApproved
}

func (r *RevaluationRun) CanReverse() bool {
	return r.Status == RevaluationStatusPosted
}

type RevaluationDetail struct {
	ID                       common.ID
	RevaluationRunID         common.ID
	AccountID                common.ID
	AccountCode              string
	AccountName              string
	OriginalCurrency         money.Currency
	OriginalBalance          decimal.Decimal
	OriginalRate             decimal.Decimal
	OriginalFunctionalAmount decimal.Decimal
	NewRate                  decimal.Decimal
	NewFunctionalAmount      decimal.Decimal
	RevaluationAmount        decimal.Decimal
	RevaluationType          RevaluationType
	GainLossAccountID        common.ID
	CreatedAt                time.Time
}

func NewRevaluationDetail(
	revaluationRunID common.ID,
	accountID common.ID,
	accountCode, accountName string,
	originalCurrency money.Currency,
	originalBalance, originalRate, originalFunctionalAmount decimal.Decimal,
	newRate, newFunctionalAmount decimal.Decimal,
	gainLossAccountID common.ID,
) *RevaluationDetail {
	revaluationAmount := newFunctionalAmount.Sub(originalFunctionalAmount)

	return &RevaluationDetail{
		ID:                       common.NewID(),
		RevaluationRunID:         revaluationRunID,
		AccountID:                accountID,
		AccountCode:              accountCode,
		AccountName:              accountName,
		OriginalCurrency:         originalCurrency,
		OriginalBalance:          originalBalance,
		OriginalRate:             originalRate,
		OriginalFunctionalAmount: originalFunctionalAmount,
		NewRate:                  newRate,
		NewFunctionalAmount:      newFunctionalAmount,
		RevaluationAmount:        revaluationAmount,
		RevaluationType:          RevaluationTypeUnrealized,
		GainLossAccountID:        gainLossAccountID,
		CreatedAt:                time.Now(),
	}
}

func (d *RevaluationDetail) IsGain() bool {
	return d.RevaluationAmount.GreaterThan(decimal.Zero)
}

func (d *RevaluationDetail) IsLoss() bool {
	return d.RevaluationAmount.LessThan(decimal.Zero)
}
