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
	InvoiceStatusSent        InvoiceStatus = "sent"
	InvoiceStatusPartialPaid InvoiceStatus = "partial"
	InvoiceStatusPaid        InvoiceStatus = "paid"
	InvoiceStatusOverdue     InvoiceStatus = "overdue"
	InvoiceStatusVoid        InvoiceStatus = "void"
	InvoiceStatusWriteOff    InvoiceStatus = "writeoff"
	InvoiceStatusDisputed    InvoiceStatus = "disputed"
)

type InvoiceType string

const (
	InvoiceTypeStandard  InvoiceType = "standard"
	InvoiceTypeCredit    InvoiceType = "credit"
	InvoiceTypeDebit     InvoiceType = "debit"
	InvoiceTypeProforma  InvoiceType = "proforma"
	InvoiceTypeRecurring InvoiceType = "recurring"
)

type Invoice struct {
	ID         common.ID
	EntityID   common.ID
	CustomerID common.ID
	Customer   *Customer

	InvoiceNumber string
	PONumber      string
	SalesOrderID  *common.ID

	InvoiceType InvoiceType

	InvoiceDate time.Time
	DueDate     time.Time
	ShipDate    *time.Time
	SentDate    *time.Time
	PostingDate *time.Time

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

	BillToAddress CustomerAddress
	ShipToAddress *CustomerAddress

	ApprovedBy *common.ID
	ApprovedAt *time.Time

	DunningLevel     int
	LastDunningDate  *time.Time
	CollectionStatus string

	WriteOffDate   *time.Time
	WriteOffAmount money.Money
	WriteOffReason string

	Description string
	Notes       string
	TermsText   string
	FooterText  string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy common.ID
}

type InvoiceLine struct {
	ID         common.ID
	InvoiceID  common.ID
	LineNumber int

	RevenueAccountID common.ID

	ItemCode    string
	Description string
	Quantity    float64
	UnitPrice   money.Money
	Amount      money.Money
	DiscountPct float64
	DiscountAmt money.Money

	TaxCode   string
	TaxRate   float64
	TaxAmount money.Money

	ProjectID    *common.ID
	CostCenterID *common.ID

	CreatedAt time.Time
}

func NewInvoice(
	entityID common.ID,
	customerID common.ID,
	invoiceNumber string,
	invoiceType InvoiceType,
	invoiceDate time.Time,
	dueDate time.Time,
	currency money.Currency,
	createdBy common.ID,
) (*Invoice, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if customerID.IsZero() {
		return nil, fmt.Errorf("customer ID is required")
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
		CustomerID:     customerID,
		InvoiceNumber:  invoiceNumber,
		InvoiceType:    invoiceType,
		InvoiceDate:    invoiceDate,
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
		WriteOffAmount: money.Zero(currency),
		Lines:          make([]InvoiceLine, 0),
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      createdBy,
	}, nil
}

func (inv *Invoice) AddLine(
	revenueAccountID common.ID,
	description string,
	quantity float64,
	unitPrice money.Money,
) error {
	if revenueAccountID.IsZero() {
		return fmt.Errorf("revenue account ID is required")
	}
	if quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}

	amount := unitPrice.MultiplyFloat(quantity)

	line := InvoiceLine{
		ID:               common.NewID(),
		InvoiceID:        inv.ID,
		LineNumber:       len(inv.Lines) + 1,
		RevenueAccountID: revenueAccountID,
		Description:      description,
		Quantity:         quantity,
		UnitPrice:        unitPrice,
		Amount:           amount,
		DiscountAmt:      money.Zero(inv.Currency),
		TaxAmount:        money.Zero(inv.Currency),
		CreatedAt:        time.Now(),
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
	discountTotal := money.Zero(inv.Currency)

	for _, line := range inv.Lines {
		subtotal = subtotal.MustAdd(line.Amount)
		taxTotal = taxTotal.MustAdd(line.TaxAmount)
		discountTotal = discountTotal.MustAdd(line.DiscountAmt)
	}

	inv.Subtotal = subtotal
	inv.TaxAmount = taxTotal
	inv.DiscountAmount = discountTotal.MustAdd(inv.DiscountAmount)
	inv.TotalAmount = subtotal.MustAdd(taxTotal).MustAdd(inv.ShippingAmount).MustSubtract(discountTotal)
	inv.BalanceDue = inv.TotalAmount.MustSubtract(inv.PaidAmount)
	inv.UpdatedAt = time.Now()
}

func (inv *Invoice) SetLineTax(lineNumber int, taxCode string, taxRate float64) error {
	for i := range inv.Lines {
		if inv.Lines[i].LineNumber == lineNumber {
			inv.Lines[i].TaxCode = taxCode
			inv.Lines[i].TaxRate = taxRate
			inv.Lines[i].TaxAmount = inv.Lines[i].Amount.MultiplyFloat(taxRate / 100)
			inv.recalculateTotals()
			return nil
		}
	}
	return fmt.Errorf("line not found")
}

func (inv *Invoice) SetShipping(shippingAmount money.Money) {
	inv.ShippingAmount = shippingAmount
	inv.recalculateTotals()
}

func (inv *Invoice) SetDiscount(discountAmount money.Money) {

	subtotal := money.Zero(inv.Currency)
	taxTotal := money.Zero(inv.Currency)
	lineDiscounts := money.Zero(inv.Currency)

	for _, line := range inv.Lines {
		subtotal = subtotal.MustAdd(line.Amount)
		taxTotal = taxTotal.MustAdd(line.TaxAmount)
		lineDiscounts = lineDiscounts.MustAdd(line.DiscountAmt)
	}

	inv.Subtotal = subtotal
	inv.TaxAmount = taxTotal
	inv.DiscountAmount = discountAmount.MustAdd(lineDiscounts)
	inv.TotalAmount = subtotal.MustAdd(taxTotal).MustAdd(inv.ShippingAmount).MustSubtract(inv.DiscountAmount)
	inv.BalanceDue = inv.TotalAmount.MustSubtract(inv.PaidAmount)
	inv.UpdatedAt = time.Now()
}

func (inv *Invoice) Validate() error {
	ve := common.NewValidationError()

	if inv.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if inv.CustomerID.IsZero() {
		ve.Add("customer_id", "required", "Customer ID is required")
	}
	if inv.InvoiceNumber == "" {
		ve.Add("invoice_number", "required", "Invoice number is required")
	}
	if inv.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}
	if len(inv.Lines) == 0 && inv.InvoiceType == InvoiceTypeStandard {
		ve.Add("lines", "required", "At least one line item is required")
	}
	if inv.TotalAmount.IsNegative() && inv.InvoiceType != InvoiceTypeCredit {
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

func (inv *Invoice) Approve(approvedBy common.ID) error {
	if inv.Status != InvoiceStatusPending {
		return fmt.Errorf("can only approve pending invoices")
	}

	now := time.Now()
	inv.Status = InvoiceStatusApproved
	inv.ApprovedBy = &approvedBy
	inv.ApprovedAt = &now
	inv.UpdatedAt = now
	return nil
}

func (inv *Invoice) Reject() error {
	if inv.Status != InvoiceStatusPending {
		return fmt.Errorf("can only reject pending invoices")
	}

	inv.Status = InvoiceStatusDraft
	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) Send() error {
	if inv.Status != InvoiceStatusApproved {
		return fmt.Errorf("can only send approved invoices")
	}

	now := time.Now()
	inv.Status = InvoiceStatusSent
	inv.SentDate = &now
	inv.UpdatedAt = now
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
	if inv.Status == InvoiceStatusVoid || inv.Status == InvoiceStatusDraft {
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
		inv.Status = InvoiceStatusSent
	} else {
		inv.Status = InvoiceStatusPartialPaid
	}

	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) WriteOff(reason string) error {
	if inv.BalanceDue.IsZero() {
		return fmt.Errorf("no balance to write off")
	}

	now := time.Now()
	inv.WriteOffAmount = inv.BalanceDue
	inv.WriteOffReason = reason
	inv.WriteOffDate = &now
	inv.BalanceDue = money.Zero(inv.Currency)
	inv.Status = InvoiceStatusWriteOff
	inv.UpdatedAt = now
	return nil
}

func (inv *Invoice) MarkDisputed() error {
	inv.Status = InvoiceStatusDisputed
	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) ResolveDispute() error {
	if inv.Status != InvoiceStatusDisputed {
		return fmt.Errorf("invoice is not disputed")
	}

	if inv.BalanceDue.IsZero() {
		inv.Status = InvoiceStatusPaid
	} else if !inv.PaidAmount.IsZero() {
		inv.Status = InvoiceStatusPartialPaid
	} else {
		inv.Status = InvoiceStatusSent
	}

	inv.UpdatedAt = time.Now()
	return nil
}

func (inv *Invoice) IsOverdue() bool {
	if inv.Status == InvoiceStatusPaid || inv.Status == InvoiceStatusVoid || inv.Status == InvoiceStatusWriteOff {
		return false
	}
	return time.Now().After(inv.DueDate) && !inv.BalanceDue.IsZero()
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
	CustomerID  *common.ID
	Status      *InvoiceStatus
	InvoiceType *InvoiceType
	DueDateFrom *time.Time
	DueDateTo   *time.Time
	OverdueOnly bool
	UnpaidOnly  bool
	Search      string
	Limit       int
	Offset      int
}
