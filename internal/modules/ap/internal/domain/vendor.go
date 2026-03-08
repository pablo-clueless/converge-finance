package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type VendorStatus string

const (
	VendorStatusActive   VendorStatus = "active"
	VendorStatusInactive VendorStatus = "inactive"
	VendorStatusBlocked  VendorStatus = "blocked"
)

type PaymentTerms string

const (
	PaymentTermsNet30        PaymentTerms = "net_30"
	PaymentTermsNet45        PaymentTerms = "net_45"
	PaymentTermsNet60        PaymentTerms = "net_60"
	PaymentTermsNet90        PaymentTerms = "net_90"
	PaymentTermsDueOnReceipt PaymentTerms = "due_on_receipt"
	PaymentTermsCustom       PaymentTerms = "custom"
)

func (pt PaymentTerms) DaysToPay() int {
	switch pt {
	case PaymentTermsNet30:
		return 30
	case PaymentTermsNet45:
		return 45
	case PaymentTermsNet60:
		return 60
	case PaymentTermsNet90:
		return 90
	case PaymentTermsDueOnReceipt:
		return 0
	default:
		return 30
	}
}

type PaymentMethod string

const (
	PaymentMethodCheck PaymentMethod = "check"
	PaymentMethodACH   PaymentMethod = "ach"
	PaymentMethodWire  PaymentMethod = "wire"
	PaymentMethodCard  PaymentMethod = "card"
)

type VendorAddress struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type VendorBankInfo struct {
	BankName      string `json:"bank_name"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	RoutingNumber string `json:"routing_number"`
	SwiftCode     string `json:"swift_code,omitempty"`
	IBAN          string `json:"iban,omitempty"`
}

type Vendor struct {
	ID         common.ID
	EntityID   common.ID
	VendorCode string
	Name       string
	LegalName  string
	TaxID      string
	Status     VendorStatus

	Email   string
	Phone   string
	Website string

	BillingAddress VendorAddress
	RemitToAddress *VendorAddress

	PaymentTerms     PaymentTerms
	PaymentTermsDays int
	PaymentMethod    PaymentMethod
	Currency         money.Currency
	BankInfo         *VendorBankInfo

	CreditLimit    money.Money
	CurrentBalance money.Money

	DefaultExpenseAccountID *common.ID
	APAccountID             *common.ID

	Is1099Vendor   bool
	W9OnFile       bool
	W9ReceivedDate *time.Time

	Notes string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy common.ID
}

func NewVendor(
	entityID common.ID,
	vendorCode string,
	name string,
	currency money.Currency,
	createdBy common.ID,
) (*Vendor, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if vendorCode == "" {
		return nil, fmt.Errorf("vendor code is required")
	}
	if name == "" {
		return nil, fmt.Errorf("vendor name is required")
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}

	now := time.Now()
	return &Vendor{
		ID:             common.NewID(),
		EntityID:       entityID,
		VendorCode:     vendorCode,
		Name:           name,
		Status:         VendorStatusActive,
		PaymentTerms:   PaymentTermsNet30,
		PaymentMethod:  PaymentMethodCheck,
		Currency:       currency,
		CreditLimit:    money.Zero(currency),
		CurrentBalance: money.Zero(currency),
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      createdBy,
	}, nil
}

func (v *Vendor) Validate() error {
	ve := common.NewValidationError()

	if v.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if v.VendorCode == "" {
		ve.Add("vendor_code", "required", "Vendor code is required")
	}
	if v.Name == "" {
		ve.Add("name", "required", "Vendor name is required")
	}
	if v.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (v *Vendor) Activate() {
	v.Status = VendorStatusActive
	v.UpdatedAt = time.Now()
}

func (v *Vendor) Deactivate() {
	v.Status = VendorStatusInactive
	v.UpdatedAt = time.Now()
}

func (v *Vendor) Block() {
	v.Status = VendorStatusBlocked
	v.UpdatedAt = time.Now()
}

func (v *Vendor) CanReceivePayments() bool {
	return v.Status == VendorStatusActive
}

func (v *Vendor) IsWithinCreditLimit(amount money.Money) bool {
	if v.CreditLimit.IsZero() {
		return true
	}
	newBalance := v.CurrentBalance.MustAdd(amount)
	return newBalance.LessThanOrEqual(v.CreditLimit)
}

func (v *Vendor) UpdateBalance(amount money.Money) {
	v.CurrentBalance = amount
	v.UpdatedAt = time.Now()
}

func (v *Vendor) GetDueDate(invoiceDate time.Time) time.Time {
	if v.PaymentTerms == PaymentTermsCustom {
		return invoiceDate.AddDate(0, 0, v.PaymentTermsDays)
	}
	return invoiceDate.AddDate(0, 0, v.PaymentTerms.DaysToPay())
}

type VendorFilter struct {
	EntityID common.ID
	Status   *VendorStatus
	Search   string
	Is1099   *bool
	Limit    int
	Offset   int
}
