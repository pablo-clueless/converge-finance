package repository

import (
	"context"
	"database/sql"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
)

type DepreciationRepository interface {
	CreateRun(ctx context.Context, run *domain.DepreciationRun) error

	UpdateRun(ctx context.Context, run *domain.DepreciationRun) error

	GetRunByID(ctx context.Context, id common.ID) (*domain.DepreciationRun, error)

	GetRunByIDForUpdate(ctx context.Context, tx *sql.Tx, id common.ID) (*domain.DepreciationRun, error)

	GetRunByNumber(ctx context.Context, entityID common.ID, runNumber string) (*domain.DepreciationRun, error)

	ListRuns(ctx context.Context, filter domain.DepreciationRunFilter) ([]domain.DepreciationRun, error)

	CountRuns(ctx context.Context, filter domain.DepreciationRunFilter) (int64, error)

	DeleteRun(ctx context.Context, id common.ID) error

	GetRunByPeriod(ctx context.Context, entityID common.ID, fiscalPeriodID common.ID) (*domain.DepreciationRun, error)

	GetLatestRun(ctx context.Context, entityID common.ID) (*domain.DepreciationRun, error)

	GetRunsForYear(ctx context.Context, entityID common.ID, fiscalYearID common.ID) ([]domain.DepreciationRun, error)

	CreateEntry(ctx context.Context, entry *domain.DepreciationEntry) error

	CreateEntries(ctx context.Context, entries []domain.DepreciationEntry) error

	GetEntriesByRun(ctx context.Context, runID common.ID) ([]domain.DepreciationEntry, error)

	GetEntriesByAsset(ctx context.Context, assetID common.ID) ([]domain.DepreciationEntry, error)

	GetEntryByRunAndAsset(ctx context.Context, runID, assetID common.ID) (*domain.DepreciationEntry, error)

	DeleteEntriesByRun(ctx context.Context, runID common.ID) error

	GetDepreciationHistory(ctx context.Context, assetID common.ID, fromDate, toDate time.Time) ([]domain.DepreciationEntry, error)

	GetTotalDepreciationByPeriod(ctx context.Context, entityID common.ID, fiscalPeriodID common.ID) (float64, error)

	GetNextRunNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	WithTx(tx *sql.Tx) DepreciationRepository
}

type DepreciationSummary struct {
	FiscalPeriodID      common.ID
	PeriodName          string
	RunCount            int
	TotalAssets         int
	TotalDepreciation   float64
	PostedDepreciation  float64
	PendingDepreciation float64
}

type AssetDepreciationHistory struct {
	AssetID                 common.ID
	AssetCode               string
	AssetName               string
	AcquisitionCost         float64
	TotalDepreciation       float64
	BookValue               float64
	DepreciationMethod      string
	UsefulLifeYears         int
	MonthsDepreciated       int
	RemainingMonths         int
	MonthlyDepreciationRate float64
}
