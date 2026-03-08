package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type EliminationType string

const (
	EliminationTypeICReceivablePayable EliminationType = "ic_receivable_payable"
	EliminationTypeICRevenueExpense    EliminationType = "ic_revenue_expense"
	EliminationTypeICDividend          EliminationType = "ic_dividend"
	EliminationTypeICInvestment        EliminationType = "ic_investment"
	EliminationTypeICEquity            EliminationType = "ic_equity"
	EliminationTypeUnrealizedProfit    EliminationType = "unrealized_profit"
)

func (t EliminationType) IsValid() bool {
	switch t {
	case EliminationTypeICReceivablePayable, EliminationTypeICRevenueExpense,
		EliminationTypeICDividend, EliminationTypeICInvestment,
		EliminationTypeICEquity, EliminationTypeUnrealizedProfit:
		return true
	}
	return false
}

func (t EliminationType) String() string {
	return string(t)
}

func (t EliminationType) DisplayName() string {
	switch t {
	case EliminationTypeICReceivablePayable:
		return "IC Receivable/Payable"
	case EliminationTypeICRevenueExpense:
		return "IC Revenue/Expense"
	case EliminationTypeICDividend:
		return "IC Dividend"
	case EliminationTypeICInvestment:
		return "Investment in Subsidiary"
	case EliminationTypeICEquity:
		return "Equity Elimination"
	case EliminationTypeUnrealizedProfit:
		return "Unrealized Profit"
	default:
		return string(t)
	}
}

type EliminationStatus string

const (
	EliminationStatusDraft    EliminationStatus = "draft"
	EliminationStatusPosted   EliminationStatus = "posted"
	EliminationStatusReversed EliminationStatus = "reversed"
)

func (s EliminationStatus) IsValid() bool {
	switch s {
	case EliminationStatusDraft, EliminationStatusPosted, EliminationStatusReversed:
		return true
	}
	return false
}

func (s EliminationStatus) String() string {
	return string(s)
}

func (s EliminationStatus) CanPost() bool {
	return s == EliminationStatusDraft
}

func (s EliminationStatus) CanReverse() bool {
	return s == EliminationStatusPosted
}

type EliminationRuleConfig struct {
	AccountPatterns []string `json:"account_patterns,omitempty"`

	AccountIDs []string `json:"account_ids,omitempty"`

	EntityIDs []string `json:"entity_ids,omitempty"`

	EliminationPercent float64 `json:"elimination_percent,omitempty"`

	AutoBalance bool `json:"auto_balance,omitempty"`

	OffsetAccountID string `json:"offset_account_id,omitempty"`

	Custom map[string]interface{} `json:"custom,omitempty"`
}

type EliminationRule struct {
	ID              common.ID
	ParentEntityID  common.ID
	RuleCode        string
	RuleName        string
	EliminationType EliminationType
	Description     string
	RuleConfig      EliminationRuleConfig
	SequenceNumber  int
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewEliminationRule(
	parentEntityID common.ID,
	ruleCode string,
	ruleName string,
	eliminationType EliminationType,
) (*EliminationRule, error) {
	if parentEntityID.IsZero() {
		return nil, fmt.Errorf("parent entity ID is required")
	}
	if ruleCode == "" {
		return nil, fmt.Errorf("rule code is required")
	}
	if ruleName == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	if !eliminationType.IsValid() {
		return nil, fmt.Errorf("invalid elimination type: %s", eliminationType)
	}

	now := time.Now()
	return &EliminationRule{
		ID:              common.NewID(),
		ParentEntityID:  parentEntityID,
		RuleCode:        ruleCode,
		RuleName:        ruleName,
		EliminationType: eliminationType,
		RuleConfig:      EliminationRuleConfig{},
		SequenceNumber:  0,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func (r *EliminationRule) SetConfig(config EliminationRuleConfig) {
	r.RuleConfig = config
	r.UpdatedAt = time.Now()
}

func (r *EliminationRule) SetSequence(seq int) {
	r.SequenceNumber = seq
	r.UpdatedAt = time.Now()
}

func (r *EliminationRule) Activate() {
	r.IsActive = true
	r.UpdatedAt = time.Now()
}

func (r *EliminationRule) Deactivate() {
	r.IsActive = false
	r.UpdatedAt = time.Now()
}

func (r *EliminationRule) GetConfigJSON() ([]byte, error) {
	return json.Marshal(r.RuleConfig)
}

func (r *EliminationRule) Validate() error {
	ve := common.NewValidationError()

	if r.ParentEntityID.IsZero() {
		ve.Add("parent_entity_id", "required", "Parent entity ID is required")
	}
	if r.RuleCode == "" {
		ve.Add("rule_code", "required", "Rule code is required")
	}
	if len(r.RuleCode) > 50 {
		ve.Add("rule_code", "max_length", "Rule code cannot exceed 50 characters")
	}
	if r.RuleName == "" {
		ve.Add("rule_name", "required", "Rule name is required")
	}
	if len(r.RuleName) > 255 {
		ve.Add("rule_name", "max_length", "Rule name cannot exceed 255 characters")
	}
	if !r.EliminationType.IsValid() {
		ve.Add("elimination_type", "invalid", "Invalid elimination type")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

type EliminationRun struct {
	ID              common.ID
	RunNumber       string
	ParentEntityID  common.ID
	FiscalPeriodID  common.ID
	EliminationDate time.Time
	Currency        money.Currency

	EntryCount        int
	TotalEliminations money.Money

	Status EliminationStatus

	JournalEntryID *common.ID

	CreatedBy  common.ID
	PostedBy   *common.ID
	ReversedBy *common.ID

	CreatedAt  time.Time
	UpdatedAt  time.Time
	PostedAt   *time.Time
	ReversedAt *time.Time

	Entries []EliminationEntry
}

func NewEliminationRun(
	parentEntityID common.ID,
	fiscalPeriodID common.ID,
	eliminationDate time.Time,
	currency money.Currency,
	createdBy common.ID,
) (*EliminationRun, error) {
	if parentEntityID.IsZero() {
		return nil, fmt.Errorf("parent entity ID is required")
	}
	if fiscalPeriodID.IsZero() {
		return nil, fmt.Errorf("fiscal period ID is required")
	}
	if eliminationDate.IsZero() {
		return nil, fmt.Errorf("elimination date is required")
	}
	if createdBy.IsZero() {
		return nil, fmt.Errorf("created by user ID is required")
	}

	now := time.Now()
	return &EliminationRun{
		ID:                common.NewID(),
		ParentEntityID:    parentEntityID,
		FiscalPeriodID:    fiscalPeriodID,
		EliminationDate:   eliminationDate,
		Currency:          currency,
		EntryCount:        0,
		TotalEliminations: money.Zero(currency),
		Status:            EliminationStatusDraft,
		CreatedBy:         createdBy,
		CreatedAt:         now,
		UpdatedAt:         now,
		Entries:           make([]EliminationEntry, 0),
	}, nil
}

func (r *EliminationRun) SetRunNumber(number string) {
	r.RunNumber = number
	r.UpdatedAt = time.Now()
}

func (r *EliminationRun) AddEntry(entry EliminationEntry) error {
	if r.Status != EliminationStatusDraft {
		return fmt.Errorf("can only add entries to draft elimination runs")
	}

	entry.EliminationRunID = r.ID
	entry.LineNumber = len(r.Entries) + 1
	r.Entries = append(r.Entries, entry)
	r.EntryCount = len(r.Entries)

	if entry.DebitAmount.IsPositive() {
		r.TotalEliminations = r.TotalEliminations.MustAdd(entry.DebitAmount)
	}

	r.UpdatedAt = time.Now()
	return nil
}

func (r *EliminationRun) Post(postedBy common.ID, journalEntryID common.ID) error {
	if !r.Status.CanPost() {
		return fmt.Errorf("cannot post elimination run with status: %s", r.Status)
	}
	if len(r.Entries) == 0 {
		return fmt.Errorf("cannot post elimination run with no entries")
	}

	now := time.Now()
	r.Status = EliminationStatusPosted
	r.PostedBy = &postedBy
	r.PostedAt = &now
	r.JournalEntryID = &journalEntryID
	r.UpdatedAt = now
	return nil
}

func (r *EliminationRun) Reverse(reversedBy common.ID) error {
	if !r.Status.CanReverse() {
		return fmt.Errorf("cannot reverse elimination run with status: %s", r.Status)
	}

	now := time.Now()
	r.Status = EliminationStatusReversed
	r.ReversedBy = &reversedBy
	r.ReversedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *EliminationRun) IsBalanced() bool {
	totalDebits := money.Zero(r.Currency)
	totalCredits := money.Zero(r.Currency)

	for _, entry := range r.Entries {
		totalDebits = totalDebits.MustAdd(entry.DebitAmount)
		totalCredits = totalCredits.MustAdd(entry.CreditAmount)
	}

	return totalDebits.Equals(totalCredits)
}

func (r *EliminationRun) GetTotalDebits() money.Money {
	total := money.Zero(r.Currency)
	for _, entry := range r.Entries {
		total = total.MustAdd(entry.DebitAmount)
	}
	return total
}

func (r *EliminationRun) GetTotalCredits() money.Money {
	total := money.Zero(r.Currency)
	for _, entry := range r.Entries {
		total = total.MustAdd(entry.CreditAmount)
	}
	return total
}

func (r *EliminationRun) Validate() error {
	ve := common.NewValidationError()

	if r.ParentEntityID.IsZero() {
		ve.Add("parent_entity_id", "required", "Parent entity ID is required")
	}
	if r.FiscalPeriodID.IsZero() {
		ve.Add("fiscal_period_id", "required", "Fiscal period ID is required")
	}
	if r.EliminationDate.IsZero() {
		ve.Add("elimination_date", "required", "Elimination date is required")
	}

	if r.Status == EliminationStatusPosted && !r.IsBalanced() {
		ve.Add("entries", "unbalanced", "Elimination entries are not balanced")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

type EliminationEntry struct {
	ID                common.ID
	EliminationRunID  common.ID
	EliminationRuleID *common.ID
	LineNumber        int
	EliminationType   EliminationType

	FromEntityID *common.ID
	ToEntityID   *common.ID

	AccountID    common.ID
	Description  string
	DebitAmount  money.Money
	CreditAmount money.Money

	CreatedAt time.Time
}

func NewEliminationEntry(
	eliminationType EliminationType,
	accountID common.ID,
	description string,
	debitAmount money.Money,
	creditAmount money.Money,
) (*EliminationEntry, error) {
	if !eliminationType.IsValid() {
		return nil, fmt.Errorf("invalid elimination type: %s", eliminationType)
	}
	if accountID.IsZero() {
		return nil, fmt.Errorf("account ID is required")
	}

	hasDebit := debitAmount.IsPositive()
	hasCredit := creditAmount.IsPositive()
	if hasDebit == hasCredit {
		return nil, fmt.Errorf("entry must have either debit or credit, not both or neither")
	}

	return &EliminationEntry{
		ID:              common.NewID(),
		EliminationType: eliminationType,
		AccountID:       accountID,
		Description:     description,
		DebitAmount:     debitAmount,
		CreditAmount:    creditAmount,
		CreatedAt:       time.Now(),
	}, nil
}

func (e *EliminationEntry) SetSourceEntities(fromEntityID, toEntityID *common.ID) {
	e.FromEntityID = fromEntityID
	e.ToEntityID = toEntityID
}

func (e *EliminationEntry) SetRuleID(ruleID common.ID) {
	e.EliminationRuleID = &ruleID
}

func (e *EliminationEntry) IsDebit() bool {
	return e.DebitAmount.IsPositive()
}

func (e *EliminationEntry) GetAmount() money.Money {
	if e.DebitAmount.IsPositive() {
		return e.DebitAmount
	}
	return e.CreditAmount
}

type EliminationRuleFilter struct {
	ParentEntityID  *common.ID
	EliminationType *EliminationType
	IsActive        *bool
	Limit           int
	Offset          int
}

type EliminationRunFilter struct {
	ParentEntityID *common.ID
	FiscalPeriodID *common.ID
	Status         *EliminationStatus
	DateFrom       *time.Time
	DateTo         *time.Time
	Limit          int
	Offset         int
}
