package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type PaymentStatus string

const (
	PaymentStatusDraft      PaymentStatus = "draft"
	PaymentStatusPending    PaymentStatus = "pending"
	PaymentStatusApproved   PaymentStatus = "approved"
	PaymentStatusScheduled  PaymentStatus = "scheduled"
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusCompleted  PaymentStatus = "completed"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusVoid       PaymentStatus = "void"
)

type PaymentType string

const (
	PaymentTypeRegular PaymentType = "regular"
	PaymentTypeAdvance PaymentType = "advance"
	PaymentTypeRefund  PaymentType = "refund"
)

type Payment struct {
	ID       common.ID
	EntityID common.ID
	VendorID common.ID
	Vendor   *Vendor

	PaymentNumber string
	CheckNumber   string

	PaymentDate   time.Time
	ScheduledDate *time.Time
	ClearedDate   *time.Time

	Status        PaymentStatus
	PaymentType   PaymentType
	PaymentMethod PaymentMethod

	Currency      money.Currency
	ExchangeRate  money.ExchangeRate
	Amount        money.Money
	DiscountTaken money.Money

	BankAccountID *common.ID
	BankReference string

	JournalEntryID *common.ID

	Allocations []PaymentAllocation

	ApprovedBy *common.ID
	ApprovedAt *time.Time

	FailureReason string

	Memo  string
	Notes string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy common.ID
}

type PaymentAllocation struct {
	ID        common.ID
	PaymentID common.ID
	InvoiceID common.ID
	Invoice   *Invoice

	Amount        money.Money
	DiscountTaken money.Money

	CreatedAt time.Time
}

func NewPayment(
	entityID common.ID,
	vendorID common.ID,
	paymentNumber string,
	paymentDate time.Time,
	paymentMethod PaymentMethod,
	currency money.Currency,
	createdBy common.ID,
) (*Payment, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if vendorID.IsZero() {
		return nil, fmt.Errorf("vendor ID is required")
	}
	if paymentNumber == "" {
		return nil, fmt.Errorf("payment number is required")
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}

	now := time.Now()
	return &Payment{
		ID:            common.NewID(),
		EntityID:      entityID,
		VendorID:      vendorID,
		PaymentNumber: paymentNumber,
		PaymentDate:   paymentDate,
		Status:        PaymentStatusDraft,
		PaymentType:   PaymentTypeRegular,
		PaymentMethod: paymentMethod,
		Currency:      currency,
		ExchangeRate:  money.ExchangeRate{Rate: decimal.NewFromInt(1)},
		Amount:        money.Zero(currency),
		DiscountTaken: money.Zero(currency),
		Allocations:   make([]PaymentAllocation, 0),
		CreatedAt:     now,
		UpdatedAt:     now,
		CreatedBy:     createdBy,
	}, nil
}

func (p *Payment) AllocateToInvoice(invoiceID common.ID, amount money.Money, discount money.Money) error {
	if p.Status != PaymentStatusDraft {
		return fmt.Errorf("can only allocate to draft payments")
	}
	if invoiceID.IsZero() {
		return fmt.Errorf("invoice ID is required")
	}
	if amount.IsNegative() || amount.IsZero() {
		return fmt.Errorf("allocation amount must be positive")
	}

	allocation := PaymentAllocation{
		ID:            common.NewID(),
		PaymentID:     p.ID,
		InvoiceID:     invoiceID,
		Amount:        amount,
		DiscountTaken: discount,
		CreatedAt:     time.Now(),
	}

	p.Allocations = append(p.Allocations, allocation)
	p.recalculateTotals()
	return nil
}

func (p *Payment) RemoveAllocation(invoiceID common.ID) error {
	if p.Status != PaymentStatusDraft {
		return fmt.Errorf("can only remove allocations from draft payments")
	}

	for i, alloc := range p.Allocations {
		if alloc.InvoiceID == invoiceID {
			p.Allocations = append(p.Allocations[:i], p.Allocations[i+1:]...)
			p.recalculateTotals()
			return nil
		}
	}
	return fmt.Errorf("allocation not found")
}

func (p *Payment) recalculateTotals() {
	total := money.Zero(p.Currency)
	discountTotal := money.Zero(p.Currency)

	for _, alloc := range p.Allocations {
		total = total.MustAdd(alloc.Amount)
		discountTotal = discountTotal.MustAdd(alloc.DiscountTaken)
	}

	p.Amount = total
	p.DiscountTaken = discountTotal
	p.UpdatedAt = time.Now()
}

func (p *Payment) SetAmount(amount money.Money) {
	p.Amount = amount
	p.UpdatedAt = time.Now()
}

func (p *Payment) Validate() error {
	ve := common.NewValidationError()

	if p.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if p.VendorID.IsZero() {
		ve.Add("vendor_id", "required", "Vendor ID is required")
	}
	if p.PaymentNumber == "" {
		ve.Add("payment_number", "required", "Payment number is required")
	}
	if p.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}
	if p.Amount.IsNegative() {
		ve.Add("amount", "invalid", "Amount cannot be negative")
	}
	if p.Amount.IsZero() && len(p.Allocations) == 0 {
		ve.Add("amount", "required", "Payment amount or allocations required")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (p *Payment) Submit() error {
	if p.Status != PaymentStatusDraft {
		return fmt.Errorf("can only submit draft payments")
	}
	if err := p.Validate(); err != nil {
		return err
	}

	p.Status = PaymentStatusPending
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Payment) Approve(approvedBy common.ID) error {
	if p.Status != PaymentStatusPending {
		return fmt.Errorf("can only approve pending payments")
	}

	now := time.Now()
	p.Status = PaymentStatusApproved
	p.ApprovedBy = &approvedBy
	p.ApprovedAt = &now
	p.UpdatedAt = now
	return nil
}

func (p *Payment) Reject() error {
	if p.Status != PaymentStatusPending {
		return fmt.Errorf("can only reject pending payments")
	}

	p.Status = PaymentStatusDraft
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Payment) Schedule(scheduledDate time.Time) error {
	if p.Status != PaymentStatusApproved {
		return fmt.Errorf("can only schedule approved payments")
	}
	if scheduledDate.Before(time.Now()) {
		return fmt.Errorf("scheduled date must be in the future")
	}

	p.Status = PaymentStatusScheduled
	p.ScheduledDate = &scheduledDate
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Payment) Process() error {
	if p.Status != PaymentStatusApproved && p.Status != PaymentStatusScheduled {
		return fmt.Errorf("payment is not ready for processing")
	}

	p.Status = PaymentStatusProcessing
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Payment) Complete(bankReference string) error {
	if p.Status != PaymentStatusProcessing {
		return fmt.Errorf("can only complete processing payments")
	}

	now := time.Now()
	p.Status = PaymentStatusCompleted
	p.BankReference = bankReference
	p.ClearedDate = &now
	p.UpdatedAt = now
	return nil
}

func (p *Payment) Fail(reason string) error {
	if p.Status != PaymentStatusProcessing {
		return fmt.Errorf("can only fail processing payments")
	}

	p.Status = PaymentStatusFailed
	p.FailureReason = reason
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Payment) Void() error {
	if p.Status == PaymentStatusCompleted {
		return fmt.Errorf("cannot void a completed payment")
	}

	p.Status = PaymentStatusVoid
	p.UpdatedAt = time.Now()
	return nil
}

func (p *Payment) TotalAllocated() money.Money {
	total := money.Zero(p.Currency)
	for _, alloc := range p.Allocations {
		total = total.MustAdd(alloc.Amount)
	}
	return total
}

func (p *Payment) UnallocatedAmount() money.Money {
	return p.Amount.MustSubtract(p.TotalAllocated())
}

type PaymentFilter struct {
	EntityID      common.ID
	VendorID      *common.ID
	Status        *PaymentStatus
	PaymentMethod *PaymentMethod
	DateFrom      *time.Time
	DateTo        *time.Time
	Search        string
	Limit         int
	Offset        int
}

type PaymentBatch struct {
	ID            common.ID
	EntityID      common.ID
	BatchNumber   string
	PaymentMethod PaymentMethod
	PaymentDate   time.Time
	Status        PaymentStatus

	Payments     []common.ID
	TotalAmount  money.Money
	PaymentCount int

	BankAccountID *common.ID

	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   common.ID
	ProcessedAt *time.Time
	ProcessedBy *common.ID
}
