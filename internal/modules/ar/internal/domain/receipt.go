package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type ReceiptStatus string

const (
	ReceiptStatusDraft     ReceiptStatus = "draft"
	ReceiptStatusPending   ReceiptStatus = "pending"
	ReceiptStatusConfirmed ReceiptStatus = "confirmed"
	ReceiptStatusApplied   ReceiptStatus = "applied"
	ReceiptStatusReversed  ReceiptStatus = "reversed"
	ReceiptStatusVoid      ReceiptStatus = "void"
)

type ReceiptMethod string

const (
	ReceiptMethodCash    ReceiptMethod = "cash"
	ReceiptMethodCheck   ReceiptMethod = "check"
	ReceiptMethodACH     ReceiptMethod = "ach"
	ReceiptMethodWire    ReceiptMethod = "wire"
	ReceiptMethodCard    ReceiptMethod = "card"
	ReceiptMethodOnline  ReceiptMethod = "online"
	ReceiptMethodLockbox ReceiptMethod = "lockbox"
)

type Receipt struct {
	ID         common.ID
	EntityID   common.ID
	CustomerID common.ID
	Customer   *Customer

	ReceiptNumber   string
	CheckNumber     string
	ReferenceNumber string

	ReceiptDate time.Time
	DepositDate *time.Time
	ClearedDate *time.Time

	Status        ReceiptStatus
	ReceiptMethod ReceiptMethod

	Currency        money.Currency
	ExchangeRate    money.ExchangeRate
	Amount          money.Money
	AppliedAmount   money.Money
	UnappliedAmount money.Money

	BankAccountID *common.ID
	BankReference string

	JournalEntryID *common.ID

	Applications []ReceiptApplication

	ReversedDate    *time.Time
	ReversedBy      *common.ID
	ReversalReason  string
	ReversalEntryID *common.ID

	Memo  string
	Notes string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy common.ID
}

type ReceiptApplication struct {
	ID        common.ID
	ReceiptID common.ID
	InvoiceID common.ID
	Invoice   *Invoice

	Amount        money.Money
	DiscountTaken money.Money

	AppliedAt time.Time
}

func NewReceipt(
	entityID common.ID,
	customerID common.ID,
	receiptNumber string,
	receiptDate time.Time,
	receiptMethod ReceiptMethod,
	currency money.Currency,
	amount money.Money,
	createdBy common.ID,
) (*Receipt, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if customerID.IsZero() {
		return nil, fmt.Errorf("customer ID is required")
	}
	if receiptNumber == "" {
		return nil, fmt.Errorf("receipt number is required")
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}
	if amount.IsZero() || amount.IsNegative() {
		return nil, fmt.Errorf("receipt amount must be positive")
	}

	now := time.Now()
	return &Receipt{
		ID:              common.NewID(),
		EntityID:        entityID,
		CustomerID:      customerID,
		ReceiptNumber:   receiptNumber,
		ReceiptDate:     receiptDate,
		Status:          ReceiptStatusDraft,
		ReceiptMethod:   receiptMethod,
		Currency:        currency,
		ExchangeRate:    money.ExchangeRate{Rate: decimal.NewFromInt(1)},
		Amount:          amount,
		AppliedAmount:   money.Zero(currency),
		UnappliedAmount: amount,
		Applications:    make([]ReceiptApplication, 0),
		CreatedAt:       now,
		UpdatedAt:       now,
		CreatedBy:       createdBy,
	}, nil
}

func (r *Receipt) ApplyToInvoice(invoiceID common.ID, amount money.Money, discount money.Money) error {
	if r.Status != ReceiptStatusDraft && r.Status != ReceiptStatusConfirmed {
		return fmt.Errorf("can only apply confirmed or draft receipts")
	}
	if invoiceID.IsZero() {
		return fmt.Errorf("invoice ID is required")
	}
	if amount.IsNegative() || amount.IsZero() {
		return fmt.Errorf("application amount must be positive")
	}
	if amount.GreaterThan(r.UnappliedAmount) {
		return fmt.Errorf("application amount exceeds unapplied balance")
	}

	application := ReceiptApplication{
		ID:            common.NewID(),
		ReceiptID:     r.ID,
		InvoiceID:     invoiceID,
		Amount:        amount,
		DiscountTaken: discount,
		AppliedAt:     time.Now(),
	}

	r.Applications = append(r.Applications, application)
	r.recalculateAmounts()
	return nil
}

func (r *Receipt) RemoveApplication(invoiceID common.ID) error {
	if r.Status == ReceiptStatusApplied {
		return fmt.Errorf("cannot remove applications from fully applied receipts")
	}

	for i, app := range r.Applications {
		if app.InvoiceID == invoiceID {
			r.Applications = append(r.Applications[:i], r.Applications[i+1:]...)
			r.recalculateAmounts()
			return nil
		}
	}
	return fmt.Errorf("application not found")
}

func (r *Receipt) recalculateAmounts() {
	applied := money.Zero(r.Currency)
	for _, app := range r.Applications {
		applied = applied.MustAdd(app.Amount)
	}

	r.AppliedAmount = applied
	r.UnappliedAmount = r.Amount.MustSubtract(applied)
	r.UpdatedAt = time.Now()
}

func (r *Receipt) Validate() error {
	ve := common.NewValidationError()

	if r.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if r.CustomerID.IsZero() {
		ve.Add("customer_id", "required", "Customer ID is required")
	}
	if r.ReceiptNumber == "" {
		ve.Add("receipt_number", "required", "Receipt number is required")
	}
	if r.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}
	if r.Amount.IsNegative() || r.Amount.IsZero() {
		ve.Add("amount", "invalid", "Amount must be positive")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (r *Receipt) Confirm() error {
	if r.Status != ReceiptStatusDraft && r.Status != ReceiptStatusPending {
		return fmt.Errorf("can only confirm draft or pending receipts")
	}
	if err := r.Validate(); err != nil {
		return err
	}

	r.Status = ReceiptStatusConfirmed
	r.UpdatedAt = time.Now()
	return nil
}

func (r *Receipt) MarkApplied() error {
	if r.Status != ReceiptStatusConfirmed {
		return fmt.Errorf("receipt must be confirmed before marking as applied")
	}
	if !r.UnappliedAmount.IsZero() {
		return fmt.Errorf("receipt has unapplied balance")
	}

	r.Status = ReceiptStatusApplied
	r.UpdatedAt = time.Now()
	return nil
}

func (r *Receipt) Reverse(reason string, reversedBy common.ID) error {
	if r.Status == ReceiptStatusReversed || r.Status == ReceiptStatusVoid {
		return fmt.Errorf("receipt is already reversed or voided")
	}

	now := time.Now()
	r.Status = ReceiptStatusReversed
	r.ReversedDate = &now
	r.ReversedBy = &reversedBy
	r.ReversalReason = reason
	r.UpdatedAt = now
	return nil
}

func (r *Receipt) Void() error {
	if r.Status == ReceiptStatusApplied {
		return fmt.Errorf("cannot void an applied receipt")
	}
	if r.Status == ReceiptStatusReversed {
		return fmt.Errorf("cannot void a reversed receipt")
	}

	r.Status = ReceiptStatusVoid
	r.UpdatedAt = time.Now()
	return nil
}

func (r *Receipt) TotalDiscountsTaken() money.Money {
	total := money.Zero(r.Currency)
	for _, app := range r.Applications {
		total = total.MustAdd(app.DiscountTaken)
	}
	return total
}

func (r *Receipt) IsFullyApplied() bool {
	return r.UnappliedAmount.IsZero()
}

type ReceiptFilter struct {
	EntityID      common.ID
	CustomerID    *common.ID
	Status        *ReceiptStatus
	ReceiptMethod *ReceiptMethod
	DateFrom      *time.Time
	DateTo        *time.Time
	Undeposited   bool
	Unapplied     bool
	Search        string
	Limit         int
	Offset        int
}

type ReceiptBatch struct {
	ID            common.ID
	EntityID      common.ID
	BatchNumber   string
	ReceiptMethod ReceiptMethod
	DepositDate   time.Time
	Status        ReceiptStatus

	Receipts     []common.ID
	TotalAmount  money.Money
	ReceiptCount int

	BankAccountID *common.ID

	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   common.ID
	DepositedAt *time.Time
	DepositedBy *common.ID
}
