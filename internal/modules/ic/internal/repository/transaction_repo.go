package repository

import (
	"context"
	"database/sql"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
)

type TransactionRepository interface {
	Create(ctx context.Context, tx *domain.ICTransaction) error

	Update(ctx context.Context, tx *domain.ICTransaction) error

	GetByID(ctx context.Context, id common.ID) (*domain.ICTransaction, error)

	GetByIDForUpdate(ctx context.Context, dbTx *sql.Tx, id common.ID) (*domain.ICTransaction, error)

	GetByNumber(ctx context.Context, entityID common.ID, transactionNumber string) (*domain.ICTransaction, error)

	List(ctx context.Context, filter domain.ICTransactionFilter) ([]domain.ICTransaction, error)

	Count(ctx context.Context, filter domain.ICTransactionFilter) (int64, error)

	Delete(ctx context.Context, id common.ID) error

	GetByEntityPair(ctx context.Context, fromEntityID, toEntityID common.ID, fiscalPeriodID *common.ID) ([]domain.ICTransaction, error)

	GetUnreconciled(ctx context.Context, fromEntityID, toEntityID common.ID) ([]domain.ICTransaction, error)

	GetUnposted(ctx context.Context, entityID common.ID) ([]domain.ICTransaction, error)

	GetDisputed(ctx context.Context, entityID common.ID) ([]domain.ICTransaction, error)

	GetByJournalEntry(ctx context.Context, journalEntryID common.ID) (*domain.ICTransaction, error)

	GetTransactionsByDateRange(ctx context.Context, entityID common.ID, startDate, endDate time.Time) ([]domain.ICTransaction, error)

	GetNextTransactionNumber(ctx context.Context, entityID common.ID) (string, error)

	CreateLine(ctx context.Context, line *domain.ICTransactionLine) error

	GetLinesByTransaction(ctx context.Context, transactionID common.ID) ([]domain.ICTransactionLine, error)

	DeleteLines(ctx context.Context, transactionID common.ID) error

	UpdateStatus(ctx context.Context, tx *domain.ICTransaction) error

	WithTx(tx *sql.Tx) TransactionRepository
}

type TransactionSummary struct {
	TotalTransactions  int
	DraftCount         int
	PendingCount       int
	PostedCount        int
	ReconciledCount    int
	DisputedCount      int
	TotalAmount        float64
	UnreconciledAmount float64
}

type EntityPairSummary struct {
	FromEntityID        common.ID
	FromEntityCode      string
	FromEntityName      string
	ToEntityID          common.ID
	ToEntityCode        string
	ToEntityName        string
	TransactionCount    int
	TotalAmount         float64
	UnreconciledCount   int
	UnreconciledAmount  float64
	LastTransactionDate *time.Time
}
