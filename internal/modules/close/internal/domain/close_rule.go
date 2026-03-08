package domain

import (
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type CloseRuleType string

const (
	CloseRuleIncomeSummary       CloseRuleType = "income_summary"
	CloseRuleRetainedEarnings    CloseRuleType = "retained_earnings"
	CloseRuleReclassification    CloseRuleType = "reclassification"
	CloseRuleAccrualReversal     CloseRuleType = "accrual_reversal"
	CloseRuleCurrencyRevaluation CloseRuleType = "currency_revaluation"
	CloseRuleDailyAccrual        CloseRuleType = "daily_accrual"
	CloseRuleDailyReconciliation CloseRuleType = "daily_reconciliation"
	CloseRuleDailyValuation      CloseRuleType = "daily_valuation"
)

func (t CloseRuleType) IsValid() bool {
	switch t {
	case CloseRuleIncomeSummary, CloseRuleRetainedEarnings, CloseRuleReclassification,
		CloseRuleAccrualReversal, CloseRuleCurrencyRevaluation,
		CloseRuleDailyAccrual, CloseRuleDailyReconciliation, CloseRuleDailyValuation:
		return true
	}
	return false
}

func (t CloseRuleType) String() string {
	return string(t)
}

type CloseRule struct {
	ID                common.ID
	EntityID          common.ID
	RuleCode          string
	RuleName          string
	RuleType          CloseRuleType
	CloseType         CloseType
	SequenceNumber    int
	SourceAccountType string
	SourceAccountID   *common.ID
	TargetAccountID   common.ID
	Description       string
	IsActive          bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func NewCloseRule(
	entityID common.ID,
	ruleCode, ruleName string,
	ruleType CloseRuleType,
	closeType CloseType,
	targetAccountID common.ID,
) *CloseRule {
	now := time.Now()
	return &CloseRule{
		ID:              common.NewID(),
		EntityID:        entityID,
		RuleCode:        ruleCode,
		RuleName:        ruleName,
		RuleType:        ruleType,
		CloseType:       closeType,
		SequenceNumber:  1,
		TargetAccountID: targetAccountID,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (r *CloseRule) SetSourceAccountType(accountType string) {
	r.SourceAccountType = accountType
	r.UpdatedAt = time.Now()
}

func (r *CloseRule) SetSourceAccount(accountID common.ID) {
	r.SourceAccountID = &accountID
	r.UpdatedAt = time.Now()
}

func (r *CloseRule) SetSequence(seq int) {
	r.SequenceNumber = seq
	r.UpdatedAt = time.Now()
}

func (r *CloseRule) Activate() {
	r.IsActive = true
	r.UpdatedAt = time.Now()
}

func (r *CloseRule) Deactivate() {
	r.IsActive = false
	r.UpdatedAt = time.Now()
}

type CloseRuleFilter struct {
	EntityID  *common.ID
	RuleType  *CloseRuleType
	CloseType *CloseType
	IsActive  *bool
	Limit     int
	Offset    int
}
