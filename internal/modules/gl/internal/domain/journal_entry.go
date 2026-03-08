package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type JournalEntryStatus string

const (
	JournalEntryStatusDraft    JournalEntryStatus = "draft"
	JournalEntryStatusPending  JournalEntryStatus = "pending"
	JournalEntryStatusPosted   JournalEntryStatus = "posted"
	JournalEntryStatusReversed JournalEntryStatus = "reversed"
)

func (s JournalEntryStatus) IsEditable() bool {
	return s == JournalEntryStatusDraft || s == JournalEntryStatusPending
}

type JournalEntrySource string

const (
	JournalEntrySourceManual    JournalEntrySource = "manual"
	JournalEntrySourceAP        JournalEntrySource = "ap"
	JournalEntrySourceAR        JournalEntrySource = "ar"
	JournalEntrySourceFA        JournalEntrySource = "fa"
	JournalEntrySourceRecurring JournalEntrySource = "recurring"
	JournalEntrySourceClosing   JournalEntrySource = "closing"
	JournalEntrySourceSystem    JournalEntrySource = "system"
)

type JournalEntry struct {
	ID              common.ID
	EntityID        common.ID
	EntryNumber     string
	FiscalPeriodID  common.ID
	EntryDate       time.Time
	PostingDate     *time.Time
	Description     string
	Source          JournalEntrySource
	SourceReference string
	Status          JournalEntryStatus
	Currency        money.Currency
	ExchangeRate    decimal.Decimal
	Lines           []JournalLine
	IsReversing     bool
	ReversalOfID    *common.ID
	ReversedByID    *common.ID
	CreatedBy       common.ID
	ApprovedBy      *common.ID
	PostedBy        *common.ID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	PostedAt        *time.Time
}

func NewJournalEntry(
	entityID common.ID,
	entryNumber string,
	fiscalPeriodID common.ID,
	entryDate time.Time,
	description string,
	currency money.Currency,
	createdBy common.ID,
) (*JournalEntry, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if entryNumber == "" {
		return nil, fmt.Errorf("entry number is required")
	}
	if fiscalPeriodID.IsZero() {
		return nil, fmt.Errorf("fiscal period ID is required")
	}
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}
	if createdBy.IsZero() {
		return nil, fmt.Errorf("created by user ID is required")
	}

	now := time.Now()
	return &JournalEntry{
		ID:             common.NewID(),
		EntityID:       entityID,
		EntryNumber:    entryNumber,
		FiscalPeriodID: fiscalPeriodID,
		EntryDate:      entryDate,
		Description:    description,
		Source:         JournalEntrySourceManual,
		Status:         JournalEntryStatusDraft,
		Currency:       currency,
		ExchangeRate:   decimal.NewFromInt(1),
		Lines:          make([]JournalLine, 0),
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

func (je *JournalEntry) AddLine(accountID common.ID, description string, debit, credit money.Money) error {
	if !je.Status.IsEditable() {
		return fmt.Errorf("cannot modify a %s journal entry", je.Status)
	}

	if debit.IsZero() && credit.IsZero() {
		return fmt.Errorf("either debit or credit amount must be non-zero")
	}
	if !debit.IsZero() && !credit.IsZero() {
		return fmt.Errorf("line cannot have both debit and credit amounts")
	}

	if !debit.IsZero() && !debit.Currency.Equals(je.Currency) {
		return fmt.Errorf("debit currency mismatch: expected %s, got %s", je.Currency.Code, debit.Currency.Code)
	}
	if !credit.IsZero() && !credit.Currency.Equals(je.Currency) {
		return fmt.Errorf("credit currency mismatch: expected %s, got %s", je.Currency.Code, credit.Currency.Code)
	}

	lineNumber := len(je.Lines) + 1
	line := JournalLine{
		ID:             common.NewID(),
		JournalEntryID: je.ID,
		LineNumber:     lineNumber,
		AccountID:      accountID,
		Description:    description,
		DebitAmount:    debit,
		CreditAmount:   credit,
		CreatedAt:      time.Now(),
	}

	if !debit.IsZero() {
		line.BaseDebit = debit.Convert(je.Currency, je.ExchangeRate)
	}
	if !credit.IsZero() {
		line.BaseCredit = credit.Convert(je.Currency, je.ExchangeRate)
	}

	je.Lines = append(je.Lines, line)
	je.UpdatedAt = time.Now()

	return nil
}

func (je *JournalEntry) RemoveLine(lineNumber int) error {
	if !je.Status.IsEditable() {
		return fmt.Errorf("cannot modify a %s journal entry", je.Status)
	}

	if lineNumber < 1 || lineNumber > len(je.Lines) {
		return fmt.Errorf("invalid line number: %d", lineNumber)
	}

	je.Lines = append(je.Lines[:lineNumber-1], je.Lines[lineNumber:]...)

	for i := range je.Lines {
		je.Lines[i].LineNumber = i + 1
	}

	je.UpdatedAt = time.Now()
	return nil
}

func (je *JournalEntry) Validate() error {
	ve := common.NewValidationError()

	if je.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if je.EntryNumber == "" {
		ve.Add("entry_number", "required", "Entry number is required")
	}
	if je.FiscalPeriodID.IsZero() {
		ve.Add("fiscal_period_id", "required", "Fiscal period is required")
	}
	if je.Description == "" {
		ve.Add("description", "required", "Description is required")
	}
	if je.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}
	if je.ExchangeRate.IsZero() || je.ExchangeRate.IsNegative() {
		ve.Add("exchange_rate", "invalid", "Exchange rate must be positive")
	}

	if len(je.Lines) < 2 {
		ve.Add("lines", "min_lines", "Journal entry must have at least 2 lines")
	}

	if !je.IsBalanced() {
		ve.Add("lines", "unbalanced", fmt.Sprintf("Journal entry is not balanced: debits=%s, credits=%s",
			je.TotalDebits().String(), je.TotalCredits().String()))
	}

	for i, line := range je.Lines {
		if line.AccountID.IsZero() {
			ve.Add(fmt.Sprintf("lines[%d].account_id", i), "required", "Account ID is required")
		}
		if line.DebitAmount.IsZero() && line.CreditAmount.IsZero() {
			ve.Add(fmt.Sprintf("lines[%d]", i), "zero_amount", "Line must have either debit or credit amount")
		}
		if !line.DebitAmount.IsZero() && !line.CreditAmount.IsZero() {
			ve.Add(fmt.Sprintf("lines[%d]", i), "both_amounts", "Line cannot have both debit and credit amounts")
		}
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (je *JournalEntry) IsBalanced() bool {
	totalDebits := je.TotalDebits()
	totalCredits := je.TotalCredits()
	return totalDebits.Amount.Equal(totalCredits.Amount)
}

func (je *JournalEntry) TotalDebits() money.Money {
	total := money.Zero(je.Currency)
	for _, line := range je.Lines {
		if !line.DebitAmount.IsZero() {
			total = total.MustAdd(line.DebitAmount)
		}
	}
	return total
}

func (je *JournalEntry) TotalCredits() money.Money {
	total := money.Zero(je.Currency)
	for _, line := range je.Lines {
		if !line.CreditAmount.IsZero() {
			total = total.MustAdd(line.CreditAmount)
		}
	}
	return total
}

func (je *JournalEntry) Submit() error {
	if je.Status != JournalEntryStatusDraft {
		return fmt.Errorf("can only submit draft entries, current status: %s", je.Status)
	}

	if err := je.Validate(); err != nil {
		return err
	}

	je.Status = JournalEntryStatusPending
	je.UpdatedAt = time.Now()
	return nil
}

func (je *JournalEntry) Approve(approvedBy common.ID) error {
	if je.Status != JournalEntryStatusPending {
		return fmt.Errorf("can only approve pending entries, current status: %s", je.Status)
	}

	je.ApprovedBy = &approvedBy
	je.UpdatedAt = time.Now()
	return nil
}

func (je *JournalEntry) Post(postedBy common.ID) error {
	if je.Status == JournalEntryStatusPosted {
		return fmt.Errorf("entry is already posted")
	}
	if je.Status == JournalEntryStatusReversed {
		return fmt.Errorf("cannot post a reversed entry")
	}

	if err := je.Validate(); err != nil {
		return err
	}

	now := time.Now()
	postingDate := je.EntryDate
	je.PostingDate = &postingDate
	je.Status = JournalEntryStatusPosted
	je.PostedBy = &postedBy
	je.PostedAt = &now
	je.UpdatedAt = now

	return nil
}

func (je *JournalEntry) Reverse(reversalDate time.Time, reversalNumber string, reversedBy common.ID) (*JournalEntry, error) {
	if je.Status != JournalEntryStatusPosted {
		return nil, fmt.Errorf("can only reverse posted entries, current status: %s", je.Status)
	}
	if je.ReversedByID != nil {
		return nil, fmt.Errorf("entry has already been reversed")
	}

	now := time.Now()

	reversal := &JournalEntry{
		ID:              common.NewID(),
		EntityID:        je.EntityID,
		EntryNumber:     reversalNumber,
		FiscalPeriodID:  je.FiscalPeriodID,
		EntryDate:       reversalDate,
		Description:     fmt.Sprintf("Reversal of %s: %s", je.EntryNumber, je.Description),
		Source:          je.Source,
		SourceReference: je.SourceReference,
		Status:          JournalEntryStatusDraft,
		Currency:        je.Currency,
		ExchangeRate:    je.ExchangeRate,
		Lines:           make([]JournalLine, 0, len(je.Lines)),
		IsReversing:     true,
		ReversalOfID:    &je.ID,
		CreatedBy:       reversedBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	for _, line := range je.Lines {
		reversedLine := JournalLine{
			ID:             common.NewID(),
			JournalEntryID: reversal.ID,
			LineNumber:     line.LineNumber,
			AccountID:      line.AccountID,
			Description:    line.Description,
			DebitAmount:    line.CreditAmount,
			CreditAmount:   line.DebitAmount,
			BaseDebit:      line.BaseCredit,
			BaseCredit:     line.BaseDebit,
			CreatedAt:      now,
		}
		reversal.Lines = append(reversal.Lines, reversedLine)
	}

	je.ReversedByID = &reversal.ID
	je.Status = JournalEntryStatusReversed
	je.UpdatedAt = now

	return reversal, nil
}

func (je *JournalEntry) CanModify() bool {
	return je.Status.IsEditable()
}

func (je *JournalEntry) CanPost() bool {
	return je.Status == JournalEntryStatusDraft || je.Status == JournalEntryStatusPending
}

func (je *JournalEntry) CanReverse() bool {
	return je.Status == JournalEntryStatusPosted && je.ReversedByID == nil
}

type JournalEntryFilter struct {
	EntityID       common.ID
	FiscalPeriodID *common.ID
	Status         *JournalEntryStatus
	Source         *JournalEntrySource
	AccountID      *common.ID
	DateFrom       *time.Time
	DateTo         *time.Time
	SearchQuery    string
	Limit          int
	Offset         int
}
