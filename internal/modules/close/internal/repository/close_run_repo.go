package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
)

type CloseRunRepository interface {
	WithTx(tx *sql.Tx) CloseRunRepository

	Create(ctx context.Context, run *domain.CloseRun) error

	Update(ctx context.Context, run *domain.CloseRun) error

	GetByID(ctx context.Context, id common.ID) (*domain.CloseRun, error)

	GetByRunNumber(ctx context.Context, entityID common.ID, runNumber string) (*domain.CloseRun, error)

	List(ctx context.Context, filter domain.CloseRunFilter) ([]domain.CloseRun, error)

	GetNextRunNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	GetLatestForPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID) (*domain.CloseRun, error)
}

type CloseRunEntryRepository interface {
	WithTx(tx *sql.Tx) CloseRunEntryRepository

	Create(ctx context.Context, entry *domain.CloseRunEntry) error

	CreateBatch(ctx context.Context, entries []domain.CloseRunEntry) error

	GetByRunID(ctx context.Context, runID common.ID) ([]domain.CloseRunEntry, error)

	DeleteByRunID(ctx context.Context, runID common.ID) error
}
