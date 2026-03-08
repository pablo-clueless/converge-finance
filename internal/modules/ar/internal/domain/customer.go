package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type CustomerStatus string

const (
	CustomerStatusActive    CustomerStatus = "active"
	CustomerStatusInactive  CustomerStatus = "inactive"
	CustomerStatusSuspended CustomerStatus = "suspended"
	CustomerStatusBlocked   CustomerStatus = "blocked"
)

type CustomerType string

const (
	CustomerTypeIndividual CustomerType = "individual"
	CustomerTypeBusiness   CustomerType = "business"
	CustomerTypeGovernment CustomerType = "government"
	CustomerTypeNonProfit  CustomerType = "nonprofit"
)

type PaymentTerms string

const (
	PaymentTermsNet30        PaymentTerms = "net_30"
	PaymentTermsNet45        PaymentTerms = "net_45"
	PaymentTermsNet60        PaymentTerms = "net_60"
	PaymentTermsNet90        PaymentTerms = "net_90"
	PaymentTermsDueOnReceipt PaymentTerms = "due_on_receipt"
	PaymentTermsPrepaid      PaymentTerms = "prepaid"
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
	case PaymentTermsDueOnReceipt, PaymentTermsPrepaid:
		return 0
	default:
		return 30
	}
}

type CustomerAddress struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
}

type CustomerContact struct {
	ID        common.ID
	Name      string
	Title     string
	Email     string
	Phone     string
	IsPrimary bool
	CreatedAt time.Time
}

type Customer struct {
	ID           common.ID
	EntityID     common.ID
	CustomerCode string
	Name         string
	LegalName    string
	CustomerType CustomerType
	TaxID        string
	Status       CustomerStatus

	Email   string
	Phone   string
	Website string

	BillingAddress  CustomerAddress
	ShippingAddress *CustomerAddress

	PaymentTerms     PaymentTerms
	PaymentTermsDays int
	Currency         money.Currency

	CreditLimit      money.Money
	CurrentBalance   money.Money
	AvailableCredit  money.Money
	CreditHoldAmount money.Money
	OnCreditHold     bool
	CreditHoldReason string
	CreditHoldDate   *time.Time
	CreditApprovedBy *common.ID
	CreditApprovedAt *time.Time

	DefaultRevenueAccountID *common.ID
	ARAccountID             *common.ID

	DunningEnabled  bool
	LastDunningDate *time.Time
	DunningLevel    int

	Contacts []CustomerContact

	Notes string

	CreatedAt time.Time
	UpdatedAt time.Time
	CreatedBy common.ID
}

func NewCustomer(
	entityID common.ID,
	customerCode string,
	name string,
	customerType CustomerType,
	currency money.Currency,
	createdBy common.ID,
) (*Customer, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if customerCode == "" {
		return nil, fmt.Errorf("customer code is required")
	}
	if name == "" {
		return nil, fmt.Errorf("customer name is required")
	}
	if currency.IsZero() {
		return nil, fmt.Errorf("currency is required")
	}

	now := time.Now()
	return &Customer{
		ID:               common.NewID(),
		EntityID:         entityID,
		CustomerCode:     customerCode,
		Name:             name,
		CustomerType:     customerType,
		Status:           CustomerStatusActive,
		PaymentTerms:     PaymentTermsNet30,
		Currency:         currency,
		CreditLimit:      money.Zero(currency),
		CurrentBalance:   money.Zero(currency),
		AvailableCredit:  money.Zero(currency),
		CreditHoldAmount: money.Zero(currency),
		DunningEnabled:   true,
		Contacts:         make([]CustomerContact, 0),
		CreatedAt:        now,
		UpdatedAt:        now,
		CreatedBy:        createdBy,
	}, nil
}

func (c *Customer) Validate() error {
	ve := common.NewValidationError()

	if c.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if c.CustomerCode == "" {
		ve.Add("customer_code", "required", "Customer code is required")
	}
	if c.Name == "" {
		ve.Add("name", "required", "Customer name is required")
	}
	if c.Currency.IsZero() {
		ve.Add("currency", "required", "Currency is required")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (c *Customer) Activate() {
	c.Status = CustomerStatusActive
	c.OnCreditHold = false
	c.CreditHoldReason = ""
	c.CreditHoldDate = nil
	c.UpdatedAt = time.Now()
}

func (c *Customer) Deactivate() {
	c.Status = CustomerStatusInactive
	c.UpdatedAt = time.Now()
}

func (c *Customer) Suspend(reason string) {
	c.Status = CustomerStatusSuspended
	c.OnCreditHold = true
	c.CreditHoldReason = reason
	now := time.Now()
	c.CreditHoldDate = &now
	c.UpdatedAt = now
}

func (c *Customer) Block(reason string) {
	c.Status = CustomerStatusBlocked
	c.OnCreditHold = true
	c.CreditHoldReason = reason
	now := time.Now()
	c.CreditHoldDate = &now
	c.UpdatedAt = now
}

func (c *Customer) ReleaseCreditHold(approvedBy common.ID) {
	c.OnCreditHold = false
	c.CreditHoldReason = ""
	c.CreditHoldDate = nil
	c.Status = CustomerStatusActive
	now := time.Now()
	c.CreditApprovedBy = &approvedBy
	c.CreditApprovedAt = &now
	c.UpdatedAt = now
}

func (c *Customer) CanPlaceOrder() bool {
	return c.Status == CustomerStatusActive && !c.OnCreditHold
}

func (c *Customer) CanInvoice() bool {
	return c.Status != CustomerStatusBlocked
}

func (c *Customer) IsWithinCreditLimit(amount money.Money) bool {
	if c.CreditLimit.IsZero() {
		return true
	}
	newBalance := c.CurrentBalance.MustAdd(amount)
	return newBalance.LessThanOrEqual(c.CreditLimit)
}

func (c *Customer) ShouldTriggerCreditHold() bool {
	if c.CreditHoldAmount.IsZero() {
		return false
	}
	return c.CurrentBalance.GreaterThan(c.CreditHoldAmount)
}

func (c *Customer) UpdateBalance(balance money.Money) {
	c.CurrentBalance = balance
	if !c.CreditLimit.IsZero() {
		c.AvailableCredit = c.CreditLimit.MustSubtract(balance)
		if c.AvailableCredit.IsNegative() {
			c.AvailableCredit = money.Zero(c.Currency)
		}
	}
	c.UpdatedAt = time.Now()
}

func (c *Customer) SetCreditLimit(limit money.Money, approvedBy common.ID) {
	c.CreditLimit = limit
	now := time.Now()
	c.CreditApprovedBy = &approvedBy
	c.CreditApprovedAt = &now
	c.UpdateBalance(c.CurrentBalance)
}

func (c *Customer) GetDueDate(invoiceDate time.Time) time.Time {
	if c.PaymentTerms == PaymentTermsCustom {
		return invoiceDate.AddDate(0, 0, c.PaymentTermsDays)
	}
	return invoiceDate.AddDate(0, 0, c.PaymentTerms.DaysToPay())
}

func (c *Customer) AddContact(name, title, email, phone string, isPrimary bool) {
	contact := CustomerContact{
		ID:        common.NewID(),
		Name:      name,
		Title:     title,
		Email:     email,
		Phone:     phone,
		IsPrimary: isPrimary,
		CreatedAt: time.Now(),
	}

	if isPrimary {
		for i := range c.Contacts {
			c.Contacts[i].IsPrimary = false
		}
	}

	c.Contacts = append(c.Contacts, contact)
	c.UpdatedAt = time.Now()
}

func (c *Customer) GetPrimaryContact() *CustomerContact {
	for i := range c.Contacts {
		if c.Contacts[i].IsPrimary {
			return &c.Contacts[i]
		}
	}
	if len(c.Contacts) > 0 {
		return &c.Contacts[0]
	}
	return nil
}

type CustomerFilter struct {
	EntityID     common.ID
	Status       *CustomerStatus
	CustomerType *CustomerType
	OnCreditHold *bool
	Search       string
	Limit        int
	Offset       int
}

type CustomerBalance struct {
	CustomerID      common.ID
	TotalInvoiced   money.Money
	TotalReceived   money.Money
	CurrentBalance  money.Money
	OverdueBalance  money.Money
	AvailableCredit money.Money
	LastInvoiceDate *time.Time
	LastPaymentDate *time.Time
	UpdatedAt       time.Time
}
