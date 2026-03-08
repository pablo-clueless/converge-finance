package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"github.com/shopspring/decimal"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft       InvoiceStatus = "draft"
	InvoiceStatusPending     InvoiceStatus = "pending"
	InvoiceStatusApproved    InvoiceStatus = "approved"
	InvoiceStatusPartialPaid InvoiceStatus = "partial"
	InvoiceStatusPaid        InvoiceStatus = "paid"
	InvoiceStatusVoid        InvoiceStatus = "void"
	InvoiceStatusDisputed    InvoiceStatus = "disputed"
)

type Invoice struct {
	ID       common.ID
	EntityID common.ID
	VendorID common.ID
	Vendor   *Vendor

	InvoiceNumber  string
	InternalNumber string
	PONumber       string

	InvoiceDate  time.Time
	ReceivedDate time.Time
	DueDate      time.Time
	PostingDate  *time.Time

	Status InvoiceStatus

	Currency       money.Currency
	ExchangeRate   money.ExchangeRate
	Subtotal       money.Money
	TaxAmount      money.Money
	ShippingAmount money.Money
	DiscountAmount money.Money
	TotalAmount    money.Money
	PaidAmount     money.Money
	BalanceDue     money.Money

	DiscountTerms   string
	DiscountPercent float64
	DiscountDueDate *time.Time

	Lines []InvoiceLine

	JournalEntryID *common.ID

	ApprovedBy    *common.ID
	ApprovedAt    *time.Time
	ApprovalNotes string

	Description string
	Notes       string
	Attachments []string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy common.ID
}

type InvoiceLine struct {
	ID         common.ID
	InvoiceID  common.ID
	LineNumber int

	AccountID common.ID

	Description string
	Quantity    float64
	UnitPrice   money.Money
	Amount      money.Money

	TaxCode   string
	TaxAmount money.Money

	ItemCode string

	ProjectID    *common.ID
	CostCenterID *common.ID

	CreatedAt time.Time
}

func NewInvoice(
	entityID common.ID,
	vendorID common.ID,
	invoiceNumber string,
	invoiceDate time.Time,
	dueDate time.Time,
	currency money.Currency,
	createdBy common.ID,
) (*Invoice, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if vendorID.IsZero() {
		return nil, fmt.Errorf("vendor ID is required")
	}
	if invoiceNumber == "" {
		return nil, fmt.Errorf("invoice number is required")
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}

	now := time.Now()
	return &Invoice{
		ID:             common.NewID(),
		EntityID:       entityID,
		VendorID:       vendorID,
		InvoiceNumber:  invoiceNumber,
		InvoiceDate:    invoiceDate,
		ReceivedDate:   now,
		DueDate:        dueDate,
		Status:         InvoiceStatusDraft,
		Currency:       currency,
		ExchangeRate:   money.ExchangeRate{Rate: decimal.NewFromInt(1)},
		Subtotal:       money.Zero(currency),
		TaxAmount:      money.Zero(currency),
		ShippingAmount: money.Zero(currency),
		DiscountAmount: money.Zero(currency),
		TotalAmount:    money.Zero(currency),
		PaidAmount:     money.Zero(currency),
		BalanceDue:     money.Zero(currency),
		Lines:          make([]InvoiceLine, 0),
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      createdBy,
	}, nil
}

func (inv *Invoice) AddLine(
	accountID common.ID,
	description string,
	quantity float64,
	unitPrice money.Money,
) error {
	if accountID.IsZero() {
		return fmt.Errorf("account ID is required")
	}
	if quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}

	amount := unitPrice.MultiplyFloat(quantity)

	line := InvoiceLine{
		ID:          common.NewID(),
		InvoiceID:   inv.ID,
		LineNumber:  len(inv.Lines) + 1,
		AccountID:   accountID,
		Description: description,
		Quantity:    quantity,
		UnitPrice:   unitPrice,
		Amount:      amount,
		TaxAmount:   money.Zero(inv.Currency),
		CreatedAt:   time.Now(),
	}

	inv.Lines = append(inv.Lines, line)
	inv.recalculateTotals()
	return nil
}

func (inv *Invoice) RemoveLine(lineNumber int) error {
	if inv.Status != InvoiceStatusDraft {
		return fmt.Errorf("can only remove lines from draft invoices")
	}

	for i, line := range inv.Lines {
		if line.LineNumber == lineNumber {
			inv.Lines = append(inv.Lines[:i], inv.Lines[i+1:]...)
			inv.renumberLines()
			inv.recalculateTotals()
			return nil
		}
	}
	return fmt.Errorf("line not found")
}

func (inv *Invoice) renumberLines() {
	for i := range inv.Lines {
		inv.Lines[i].LineNumber = i + 1
	}
}

func (inv *Invoice) recalculateTotals() {
	subtotal := money.Zero(inv.Currency)
	taxTotal := money.Zero(inv.Currency)

	for _, line := range inv.Lines {
		subtotal = subtotal.MustAdd(line.Amount)
		taxTotal = taxTotal.MustAdd(line.TaxAmount)
	}

	inv.Subtotal = subtotal
	inv.TaxAmount = taxTotal
	inv.TotalAmount = subtotal.MustAdd(taxTotal).MustAdd(inv.ShippingAmount).MustSubtract(inv.DiscountAmount)
	inv.BalanceDue = inv.TotalAmount.MustSubtract(inv.PaidAmount)
	inv.UpdatedAt = time.Now()
}

func (inv *Invoice) SetTax(taxAmount money.Money) {
	inv.TaxAmount = taxAmount
	inv.recalculateTotals()
}

func (inv *Invoice) SetShipping(shippingAmount money.Money) {
	inv.ShippingAmount = shippingAmount
	inv.recalculateTotals()
}

func (inv *Invoice) SetDiscount(discountAmount money.Money) {
	inv.DiscountAmount = discountAmount
	inv.recalculateTotals()
}

func (inv *Invoice) Validate() error {
	ve := common.NewValidationError()

	if inv.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if inv.VendorID.IsZero() {
		ve.Add("vendor_id", "required", "Vendor ID is required")
	}
	if inv.InvoiceNumber == "" {
		ve.Add("invoice_number", "required", "Invoice number is required")
	}
	if inv.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}
	if len(inv.Lines) == 0 {
		ve.Add("lines", "required", "At least one line item is required")
	}
	if inv.TotalAmount.IsNegative() {
		ve.Add("total_amount", "invalid", "Total amount cannot be negative")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (inv *Invoice) Submit() error {
	if inv.Status != InvoiceStatusDraft {
		return fmt.Errorf("can only submit draft invoices")
	}
	if err := inv.Validate(); err != nil {
		return err
	}

	inv.Status = InvoiceStatusPending
	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) Approve(approvedBy common.ID, notes string) error {
	if inv.Status != InvoiceStatusPending {
		return fmt.Errorf("can only approve pending invoices")
	}

	now := time.Now()
	inv.Status = InvoiceStatusApproved
	inv.ApprovedBy = &approvedBy
	inv.ApprovedAt = &now
	inv.ApprovalNotes = notes
	inv.UpdatedAt = now
	return nil
}

func (inv *Invoice) Reject(notes string) error {
	if inv.Status != InvoiceStatusPending {
		return fmt.Errorf("can only reject pending invoices")
	}

	inv.Status = InvoiceStatusDraft
	inv.ApprovalNotes = notes
	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) Void() error {
	if inv.Status == InvoiceStatusPaid {
		return fmt.Errorf("cannot void a paid invoice")
	}
	if !inv.PaidAmount.IsZero() {
		return fmt.Errorf("cannot void an invoice with payments")
	}

	inv.Status = InvoiceStatusVoid
	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) ApplyPayment(amount money.Money) error {
	if inv.Status != InvoiceStatusApproved && inv.Status != InvoiceStatusPartialPaid {
		return fmt.Errorf("invoice is not ready for payment")
	}
	if amount.GreaterThan(inv.BalanceDue) {
		return fmt.Errorf("payment amount exceeds balance due")
	}

	inv.PaidAmount = inv.PaidAmount.MustAdd(amount)
	inv.BalanceDue = inv.TotalAmount.MustSubtract(inv.PaidAmount)

	if inv.BalanceDue.IsZero() {
		inv.Status = InvoiceStatusPaid
	} else {
		inv.Status = InvoiceStatusPartialPaid
	}

	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) UnapplyPayment(amount money.Money) error {
	if amount.GreaterThan(inv.PaidAmount) {
		return fmt.Errorf("unapply amount exceeds paid amount")
	}

	inv.PaidAmount = inv.PaidAmount.MustSubtract(amount)
	inv.BalanceDue = inv.TotalAmount.MustSubtract(inv.PaidAmount)

	if inv.PaidAmount.IsZero() {
		inv.Status = InvoiceStatusApproved
	} else {
		inv.Status = InvoiceStatusPartialPaid
	}

	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) IsOverdue() bool {
	if inv.Status == InvoiceStatusPaid || inv.Status == InvoiceStatusVoid {
		return false
	}
	return time.Now().After(inv.DueDate)
}

func (inv *Invoice) DaysOverdue() int {
	if !inv.IsOverdue() {
		return 0
	}
	return int(time.Since(inv.DueDate).Hours() / 24)
}

func (inv *Invoice) CanTakeEarlyDiscount() bool {
	if inv.DiscountDueDate == nil || inv.DiscountPercent == 0 {
		return false
	}
	return time.Now().Before(*inv.DiscountDueDate)
}

func (inv *Invoice) EarlyPaymentAmount() money.Money {
	if !inv.CanTakeEarlyDiscount() {
		return inv.BalanceDue
	}
	discount := inv.BalanceDue.MultiplyFloat(inv.DiscountPercent / 100)
	return inv.BalanceDue.MustSubtract(discount)
}

type InvoiceFilter struct {
	EntityID    common.ID
	VendorID    *common.ID
	Status      *InvoiceStatus
	DueDateFrom *time.Time
	DueDateTo   *time.Time
	OverdueOnly bool
	UnpaidOnly  bool
	Search      string
	Limit       int
	Offset      int
}
