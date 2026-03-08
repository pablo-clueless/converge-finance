package repository

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *domain.Payment) error

	Update(ctx context.Context, payment *domain.Payment) error

	GetByID(ctx context.Context, id common.ID) (*domain.Payment, error)

	GetByNumber(ctx context.Context, entityID common.ID, paymentNumber string) (*domain.Payment, error)

	List(ctx context.Context, filter domain.PaymentFilter) ([]domain.Payment, error)

	Count(ctx context.Context, filter domain.PaymentFilter) (int, error)

	Delete(ctx context.Context, id common.ID) error

	GetNextPaymentNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	CreateAllocation(ctx context.Context, allocation *domain.PaymentAllocation) error

	DeleteAllocation(ctx context.Context, allocationID common.ID) error

	GetAllocationsByPaymentID(ctx context.Context, paymentID common.ID) ([]domain.PaymentAllocation, error)

	GetAllocationsByInvoiceID(ctx context.Context, invoiceID common.ID) ([]domain.PaymentAllocation, error)

	CreateBatch(ctx context.Context, batch *domain.PaymentBatch) error

	UpdateBatch(ctx context.Context, batch *domain.PaymentBatch) error

	GetBatchByID(ctx context.Context, id common.ID) (*domain.PaymentBatch, error)

	GetBatchesByStatus(ctx context.Context, entityID common.ID, status domain.PaymentStatus) ([]domain.PaymentBatch, error)

	AddPaymentToBatch(ctx context.Context, batchID, paymentID common.ID) error

	RemovePaymentFromBatch(ctx context.Context, batchID, paymentID common.ID) error

	GetPaymentsForVendor(ctx context.Context, vendorID common.ID, limit, offset int) ([]domain.Payment, error)

	GetScheduledPayments(ctx context.Context, entityID common.ID, beforeDate time.Time) ([]domain.Payment, error)

	GetPaymentsByDateRange(ctx context.Context, entityID common.ID, startDate, endDate time.Time) ([]domain.Payment, error)

	GetPaymentsSummary(ctx context.Context, entityID common.ID, startDate, endDate time.Time) (*domain.PaymentsSummary, error)

	GetNextCheckNumber(ctx context.Context, bankAccountID common.ID) (string, error)

	GetPaymentsByCheckRange(ctx context.Context, bankAccountID common.ID, startCheck, endCheck string) ([]domain.Payment, error)
}
