package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
)

type EliminationRepository interface {
	CreateRule(ctx context.Context, rule *domain.EliminationRule) error

	UpdateRule(ctx context.Context, rule *domain.EliminationRule) error

	GetRuleByID(ctx context.Context, id common.ID) (*domain.EliminationRule, error)

	GetRuleByCode(ctx context.Context, parentEntityID common.ID, ruleCode string) (*domain.EliminationRule, error)

	ListRules(ctx context.Context, filter domain.EliminationRuleFilter) ([]domain.EliminationRule, error)

	CountRules(ctx context.Context, filter domain.EliminationRuleFilter) (int64, error)

	DeleteRule(ctx context.Context, id common.ID) error

	GetActiveRules(ctx context.Context, parentEntityID common.ID) ([]domain.EliminationRule, error)

	GetRulesByType(ctx context.Context, parentEntityID common.ID, eliminationType domain.EliminationType) ([]domain.EliminationRule, error)

	CreateRun(ctx context.Context, run *domain.EliminationRun) error

	UpdateRun(ctx context.Context, run *domain.EliminationRun) error

	GetRunByID(ctx context.Context, id common.ID) (*domain.EliminationRun, error)

	GetRunByIDForUpdate(ctx context.Context, dbTx *sql.Tx, id common.ID) (*domain.EliminationRun, error)

	GetRunByNumber(ctx context.Context, parentEntityID common.ID, runNumber string) (*domain.EliminationRun, error)

	ListRuns(ctx context.Context, filter domain.EliminationRunFilter) ([]domain.EliminationRun, error)

	CountRuns(ctx context.Context, filter domain.EliminationRunFilter) (int64, error)

	DeleteRun(ctx context.Context, id common.ID) error

	GetRunsByPeriod(ctx context.Context, parentEntityID common.ID, fiscalPeriodID common.ID) ([]domain.EliminationRun, error)

	GetPostedRunByPeriod(ctx context.Context, parentEntityID common.ID, fiscalPeriodID common.ID) (*domain.EliminationRun, error)

	GetLatestRun(ctx context.Context, parentEntityID common.ID) (*domain.EliminationRun, error)

	GetNextRunNumber(ctx context.Context, parentEntityID common.ID) (string, error)

	CreateEntry(ctx context.Context, entry *domain.EliminationEntry) error

	CreateEntries(ctx context.Context, entries []domain.EliminationEntry) error

	GetEntriesByRun(ctx context.Context, runID common.ID) ([]domain.EliminationEntry, error)

	GetEntriesByRunAndType(ctx context.Context, runID common.ID, eliminationType domain.EliminationType) ([]domain.EliminationEntry, error)

	DeleteEntriesByRun(ctx context.Context, runID common.ID) error

	GetEntryCountByRun(ctx context.Context, runID common.ID) (int, error)

	WithTx(tx *sql.Tx) EliminationRepository
}

type EliminationSummary struct {
	TotalRuns       int
	DraftRuns       int
	PostedRuns      int
	ReversedRuns    int
	TotalEntries    int
	TotalEliminated float64
}
