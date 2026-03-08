package repository

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
)

type ReceiptRepository interface {
	Create(ctx context.Context, receipt *domain.Receipt) error

	Update(ctx context.Context, receipt *domain.Receipt) error

	GetByID(ctx context.Context, id common.ID) (*domain.Receipt, error)

	GetByNumber(ctx context.Context, entityID common.ID, receiptNumber string) (*domain.Receipt, error)

	List(ctx context.Context, filter domain.ReceiptFilter) ([]domain.Receipt, error)

	Count(ctx context.Context, filter domain.ReceiptFilter) (int, error)

	Delete(ctx context.Context, id common.ID) error

	GetNextReceiptNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	CreateApplication(ctx context.Context, application *domain.ReceiptApplication) error

	DeleteApplication(ctx context.Context, applicationID common.ID) error

	GetApplicationsByReceiptID(ctx context.Context, receiptID common.ID) ([]domain.ReceiptApplication, error)

	GetApplicationsByInvoiceID(ctx context.Context, invoiceID common.ID) ([]domain.ReceiptApplication, error)

	CreateBatch(ctx context.Context, batch *domain.ReceiptBatch) error

	UpdateBatch(ctx context.Context, batch *domain.ReceiptBatch) error

	GetBatchByID(ctx context.Context, id common.ID) (*domain.ReceiptBatch, error)

	AddReceiptToBatch(ctx context.Context, batchID, receiptID common.ID) error

	RemoveReceiptFromBatch(ctx context.Context, batchID, receiptID common.ID) error

	GetReceiptsForCustomer(ctx context.Context, customerID common.ID, limit, offset int) ([]domain.Receipt, error)

	GetUnappliedReceipts(ctx context.Context, entityID common.ID) ([]domain.Receipt, error)

	GetUndepositedReceipts(ctx context.Context, entityID common.ID) ([]domain.Receipt, error)

	GetReceiptsByDateRange(ctx context.Context, entityID common.ID, startDate, endDate time.Time) ([]domain.Receipt, error)

	GetReceiptsSummary(ctx context.Context, entityID common.ID, startDate, endDate time.Time) (*domain.ReceiptsSummary, error)

	GetReversedReceipts(ctx context.Context, entityID common.ID, startDate, endDate time.Time) ([]domain.Receipt, error)
}
