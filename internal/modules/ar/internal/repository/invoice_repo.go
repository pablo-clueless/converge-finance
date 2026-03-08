package repository

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
)

type InvoiceRepository interface {
	Create(ctx context.Context, invoice *domain.Invoice) error

	Update(ctx context.Context, invoice *domain.Invoice) error

	GetByID(ctx context.Context, id common.ID) (*domain.Invoice, error)

	GetByNumber(ctx context.Context, entityID common.ID, invoiceNumber string) (*domain.Invoice, error)

	List(ctx context.Context, filter domain.InvoiceFilter) ([]domain.Invoice, error)

	Count(ctx context.Context, filter domain.InvoiceFilter) (int, error)

	Delete(ctx context.Context, id common.ID) error

	GetNextInvoiceNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	AddLine(ctx context.Context, line *domain.InvoiceLine) error

	UpdateLine(ctx context.Context, line *domain.InvoiceLine) error

	DeleteLine(ctx context.Context, lineID common.ID) error

	GetLinesByInvoiceID(ctx context.Context, invoiceID common.ID) ([]domain.InvoiceLine, error)

	GetOverdueInvoices(ctx context.Context, entityID common.ID, asOfDate time.Time) ([]domain.Invoice, error)

	GetInvoicesDueInRange(ctx context.Context, entityID common.ID, startDate, endDate time.Time) ([]domain.Invoice, error)

	GetUnpaidInvoicesForCustomer(ctx context.Context, customerID common.ID) ([]domain.Invoice, error)

	GetInvoicesWithEarlyPaymentDiscount(ctx context.Context, entityID common.ID, asOfDate time.Time) ([]domain.Invoice, error)

	GetAgingReport(ctx context.Context, entityID common.ID, asOfDate time.Time) (*domain.AgingReport, error)

	GetCustomerAgingReport(ctx context.Context, customerID common.ID, asOfDate time.Time) (*domain.CustomerAging, error)

	GetInvoicesForDunning(ctx context.Context, entityID common.ID, minDaysOverdue int) ([]domain.Invoice, error)

	GetInvoicesByDunningLevel(ctx context.Context, entityID common.ID, level domain.DunningLevel) ([]domain.Invoice, error)

	UpdateDunningLevel(ctx context.Context, invoiceID common.ID, level domain.DunningLevel) error
}
