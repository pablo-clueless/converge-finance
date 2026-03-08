package ar

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type API interface {
	GetCustomerByID(ctx context.Context, customerID common.ID) (*CustomerResponse, error)
	GetCustomerByCode(ctx context.Context, entityID common.ID, code string) (*CustomerResponse, error)
	GetCustomerBalance(ctx context.Context, customerID common.ID) (*CustomerBalanceResponse, error)
	IsCustomerInGoodStanding(ctx context.Context, customerID common.ID) (bool, error)

	GetInvoiceByID(ctx context.Context, invoiceID common.ID) (*InvoiceResponse, error)
	GetInvoiceByNumber(ctx context.Context, entityID common.ID, invoiceNumber string) (*InvoiceResponse, error)
	GetOpenInvoicesForCustomer(ctx context.Context, customerID common.ID) ([]InvoiceResponse, error)
	GetTotalOutstandingByCustomer(ctx context.Context, customerID common.ID) (money.Money, error)

	GetReceiptByID(ctx context.Context, receiptID common.ID) (*ReceiptResponse, error)
	GetUnappliedReceiptsForCustomer(ctx context.Context, customerID common.ID) ([]ReceiptResponse, error)

	GetCustomerAgingReport(ctx context.Context, customerID common.ID, asOfDate time.Time) (*CustomerAgingResponse, error)
	GetOverdueInvoicesForCustomer(ctx context.Context, customerID common.ID) ([]InvoiceResponse, error)
}

type CustomerResponse struct {
	ID              common.ID
	EntityID        common.ID
	CustomerCode    string
	Name            string
	LegalName       string
	CustomerType    string
	TaxID           string
	Email           string
	Phone           string
	Website         string
	Status          string
	CurrencyCode    string
	PaymentTerms    string
	CreditLimit     money.Money
	CurrentBalance  money.Money
	AvailableCredit money.Money
	OnCreditHold    bool
	DunningEnabled  bool
	ARAccountID     *common.ID
}

type CustomerBalanceResponse struct {
	CustomerID      common.ID
	TotalInvoiced   money.Money
	TotalReceived   money.Money
	CurrentBalance  money.Money
	OverdueBalance  money.Money
	AvailableCredit money.Money
}

type InvoiceResponse struct {
	ID             common.ID
	EntityID       common.ID
	CustomerID     common.ID
	InvoiceNumber  string
	InvoiceType    string
	PONumber       string
	InvoiceDate    time.Time
	DueDate        time.Time
	Status         string
	CurrencyCode   string
	Subtotal       money.Money
	TaxAmount      money.Money
	DiscountAmount money.Money
	TotalAmount    money.Money
	PaidAmount     money.Money
	BalanceDue     money.Money
	IsOverdue      bool
	DaysOverdue    int
	DunningLevel   int
}

type ReceiptResponse struct {
	ID              common.ID
	EntityID        common.ID
	CustomerID      common.ID
	ReceiptNumber   string
	ReceiptDate     time.Time
	ReceiptMethod   string
	Status          string
	CurrencyCode    string
	Amount          money.Money
	AppliedAmount   money.Money
	UnappliedAmount money.Money
}

type CustomerAgingResponse struct {
	CustomerID    common.ID
	CustomerCode  string
	CustomerName  string
	CurrencyCode  string
	Current       money.Money
	Days1To30     money.Money
	Days31To60    money.Money
	Days61To90    money.Money
	Over90Days    money.Money
	TotalBalance  money.Money
	InvoiceCount  int
	OldestDueDate *time.Time
}
