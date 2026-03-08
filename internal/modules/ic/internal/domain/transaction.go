package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type TransactionStatus string

const (
	TransactionStatusDraft      TransactionStatus = "draft"
	TransactionStatusPending    TransactionStatus = "pending"
	TransactionStatusPosted     TransactionStatus = "posted"
	TransactionStatusReconciled TransactionStatus = "reconciled"
	TransactionStatusDisputed   TransactionStatus = "disputed"
)

func (s TransactionStatus) IsValid() bool {
	switch s {
	case TransactionStatusDraft, TransactionStatusPending, TransactionStatusPosted,
		TransactionStatusReconciled, TransactionStatusDisputed:
		return true
	}
	return false
}

func (s TransactionStatus) String() string {
	return string(s)
}

func (s TransactionStatus) CanPost() bool {
	return s == TransactionStatusDraft || s == TransactionStatusPending
}

func (s TransactionStatus) CanReconcile() bool {
	return s == TransactionStatusPosted
}

func (s TransactionStatus) CanDispute() bool {
	return s == TransactionStatusPosted || s == TransactionStatusReconciled
}

type ICTransaction struct {
	ID                common.ID
	TransactionNumber string
	TransactionType   TransactionType

	FromEntityID common.ID
	ToEntityID   common.ID

	TransactionDate time.Time
	DueDate         *time.Time

	Amount       money.Money
	Currency     money.Currency
	ExchangeRate decimal.Decimal
	BaseAmount   money.Money

	Description string
	Reference   string

	Status TransactionStatus

	FromFiscalPeriodID *common.ID
	ToFiscalPeriodID   *common.ID

	FromJournalEntryID *common.ID
	ToJournalEntryID   *common.ID

	CreatedBy    common.ID
	PostedBy     *common.ID
	ReconciledBy *common.ID

	CreatedAt    time.Time
	UpdatedAt    time.Time
	PostedAt     *time.Time
	ReconciledAt *time.Time

	Lines []ICTransactionLine
}

type ICTransactionLine struct {
	ID            common.ID
	TransactionID common.ID
	LineNumber    int

	Description string
	Quantity    decimal.Decimal
	UnitPrice   decimal.Decimal
	Amount      money.Money

	CostCenterCode string
	ProjectCode    string

	CreatedAt time.Time
}

func NewICTransaction(
	fromEntityID common.ID,
	toEntityID common.ID,
	transactionType TransactionType,
	transactionDate time.Time,
	amount money.Money,
	description string,
	createdBy common.ID,
) (*ICTransaction, error) {
	if fromEntityID.IsZero() {
		return nil, fmt.Errorf("from entity ID is required")
	}
	if toEntityID.IsZero() {
		return nil, fmt.Errorf("to entity ID is required")
	}
	if fromEntityID == toEntityID {
		return nil, fmt.Errorf("from and to entities must be different")
	}
	if !transactionType.IsValid() {
		return nil, fmt.Errorf("invalid transaction type: %s", transactionType)
	}
	if !amount.IsPositive() {
		return nil, fmt.Errorf("amount must be positive")
	}
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if createdBy.IsZero() {
		return nil, fmt.Errorf("created by user ID is required")
	}

	now := time.Now()
	return &ICTransaction{
		ID:              common.NewID(),
		TransactionType: transactionType,
		FromEntityID:    fromEntityID,
		ToEntityID:      toEntityID,
		TransactionDate: transactionDate,
		Amount:          amount,
		Currency:        amount.Currency,
		ExchangeRate:    decimal.NewFromInt(1),
		BaseAmount:      amount,
		Description:     description,
		Status:          TransactionStatusDraft,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
		Lines:           make([]ICTransactionLine, 0),
	}, nil
}

func (t *ICTransaction) SetTransactionNumber(number string) {
	t.TransactionNumber = number
	t.UpdatedAt = time.Now()
}

func (t *ICTransaction) SetDueDate(dueDate time.Time) {
	t.DueDate = &dueDate
	t.UpdatedAt = time.Now()
}

func (t *ICTransaction) SetReference(reference string) {
	t.Reference = reference
	t.UpdatedAt = time.Now()
}

func (t *ICTransaction) SetExchangeRate(rate decimal.Decimal, baseCurrency money.Currency) error {
	if rate.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("exchange rate must be positive")
	}
	t.ExchangeRate = rate
	t.BaseAmount = money.NewFromDecimal(t.Amount.Amount.Mul(rate), baseCurrency)
	t.UpdatedAt = time.Now()
	return nil
}

func (t *ICTransaction) SetFiscalPeriods(fromPeriodID, toPeriodID common.ID) {
	t.FromFiscalPeriodID = &fromPeriodID
	t.ToFiscalPeriodID = &toPeriodID
	t.UpdatedAt = time.Now()
}

func (t *ICTransaction) AddLine(
	description string,
	quantity decimal.Decimal,
	unitPrice decimal.Decimal,
	amount money.Money,
) error {
	if t.Status != TransactionStatusDraft {
		return fmt.Errorf("can only add lines to draft transactions")
	}

	lineNumber := len(t.Lines) + 1
	line := ICTransactionLine{
		ID:            common.NewID(),
		TransactionID: t.ID,
		LineNumber:    lineNumber,
		Description:   description,
		Quantity:      quantity,
		UnitPrice:     unitPrice,
		Amount:        amount,
		CreatedAt:     time.Now(),
	}
	t.Lines = append(t.Lines, line)
	t.UpdatedAt = time.Now()
	return nil
}

func (t *ICTransaction) Submit() error {
	if t.Status != TransactionStatusDraft {
		return fmt.Errorf("can only submit draft transactions, current status: %s", t.Status)
	}
	t.Status = TransactionStatusPending
	t.UpdatedAt = time.Now()
	return nil
}

func (t *ICTransaction) Post(
	postedBy common.ID,
	fromJournalEntryID common.ID,
	toJournalEntryID common.ID,
) error {
	if !t.Status.CanPost() {
		return fmt.Errorf("cannot post transaction with status: %s", t.Status)
	}

	now := time.Now()
	t.Status = TransactionStatusPosted
	t.PostedBy = &postedBy
	t.PostedAt = &now
	t.FromJournalEntryID = &fromJournalEntryID
	t.ToJournalEntryID = &toJournalEntryID
	t.UpdatedAt = now
	return nil
}

func (t *ICTransaction) Reconcile(reconciledBy common.ID) error {
	if !t.Status.CanReconcile() {
		return fmt.Errorf("cannot reconcile transaction with status: %s", t.Status)
	}

	now := time.Now()
	t.Status = TransactionStatusReconciled
	t.ReconciledBy = &reconciledBy
	t.ReconciledAt = &now
	t.UpdatedAt = now
	return nil
}

func (t *ICTransaction) Dispute() error {
	if !t.Status.CanDispute() {
		return fmt.Errorf("cannot dispute transaction with status: %s", t.Status)
	}
	t.Status = TransactionStatusDisputed
	t.UpdatedAt = time.Now()
	return nil
}

func (t *ICTransaction) ResolveDispute() error {
	if t.Status != TransactionStatusDisputed {
		return fmt.Errorf("can only resolve disputed transactions")
	}
	t.Status = TransactionStatusPosted
	t.UpdatedAt = time.Now()
	return nil
}

func (t *ICTransaction) GetLinesTotalAmount() money.Money {
	total := money.Zero(t.Currency)
	for _, line := range t.Lines {
		total = total.MustAdd(line.Amount)
	}
	return total
}

func (t *ICTransaction) Validate() error {
	ve := common.NewValidationError()

	if t.FromEntityID.IsZero() {
		ve.Add("from_entity_id", "required", "From entity ID is required")
	}
	if t.ToEntityID.IsZero() {
		ve.Add("to_entity_id", "required", "To entity ID is required")
	}
	if t.FromEntityID == t.ToEntityID {
		ve.Add("to_entity_id", "invalid", "From and to entities must be different")
	}
	if !t.TransactionType.IsValid() {
		ve.Add("transaction_type", "invalid", "Invalid transaction type")
	}
	if !t.Amount.IsPositive() {
		ve.Add("amount", "min", "Amount must be positive")
	}
	if t.Description == "" {
		ve.Add("description", "required", "Description is required")
	}
	if t.TransactionDate.IsZero() {
		ve.Add("transaction_date", "required", "Transaction date is required")
	}

	if len(t.Lines) > 0 {
		linesTotalAmount := t.GetLinesTotalAmount()
		if !linesTotalAmount.Equals(t.Amount) {
			ve.Add("lines", "mismatch", fmt.Sprintf("Lines total (%s) does not match header amount (%s)",
				linesTotalAmount.Amount.String(), t.Amount.Amount.String()))
		}
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

type ICTransactionFilter struct {
	FromEntityID    *common.ID
	ToEntityID      *common.ID
	EntityID        *common.ID
	TransactionType *TransactionType
	Status          *TransactionStatus
	FiscalPeriodID  *common.ID
	DateFrom        *time.Time
	DateTo          *time.Time
	Search          string
	Limit           int
	Offset          int
}
