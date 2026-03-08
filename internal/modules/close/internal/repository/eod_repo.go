package repository

import (
	"context"
	"database/sql"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
)

type BusinessDateRepository interface {
	WithTx(tx *sql.Tx) BusinessDateRepository

	Create(ctx context.Context, bd *domain.BusinessDate) error
	Update(ctx context.Context, bd *domain.BusinessDate) error
	GetByEntityID(ctx context.Context, entityID common.ID) (*domain.BusinessDate, error)
}

type EODConfigRepository interface {
	WithTx(tx *sql.Tx) EODConfigRepository

	Create(ctx context.Context, cfg *domain.EODConfig) error
	Update(ctx context.Context, cfg *domain.EODConfig) error
	GetByEntityID(ctx context.Context, entityID common.ID) (*domain.EODConfig, error)
}

type EODRunRepository interface {
	WithTx(tx *sql.Tx) EODRunRepository

	Create(ctx context.Context, run *domain.EODRun) error
	Update(ctx context.Context, run *domain.EODRun) error
	GetByID(ctx context.Context, id common.ID) (*domain.EODRun, error)
	GetByBusinessDate(ctx context.Context, entityID common.ID, date time.Time) (*domain.EODRun, error)
	GetLatest(ctx context.Context, entityID common.ID) (*domain.EODRun, error)
	List(ctx context.Context, filter domain.EODRunFilter) ([]domain.EODRun, error)
}

type EODTaskRepository interface {
	WithTx(tx *sql.Tx) EODTaskRepository

	Create(ctx context.Context, task *domain.EODTask) error
	Update(ctx context.Context, task *domain.EODTask) error
	Delete(ctx context.Context, id common.ID) error
	GetByID(ctx context.Context, id common.ID) (*domain.EODTask, error)
	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.EODTask, error)
	List(ctx context.Context, filter domain.EODTaskFilter) ([]domain.EODTask, error)
	GetActiveTasks(ctx context.Context, entityID common.ID) ([]domain.EODTask, error)
}

type EODTaskRunRepository interface {
	WithTx(tx *sql.Tx) EODTaskRunRepository

	Create(ctx context.Context, tr *domain.EODTaskRun) error
	CreateBatch(ctx context.Context, runs []domain.EODTaskRun) error
	Update(ctx context.Context, tr *domain.EODTaskRun) error
	GetByID(ctx context.Context, id common.ID) (*domain.EODTaskRun, error)
	GetByEODRunID(ctx context.Context, eodRunID common.ID) ([]domain.EODTaskRun, error)
}

type HolidayRepository interface {
	WithTx(tx *sql.Tx) HolidayRepository

	Create(ctx context.Context, holiday *domain.Holiday) error
	Delete(ctx context.Context, id common.ID) error
	GetByID(ctx context.Context, id common.ID) (*domain.Holiday, error)
	GetByDate(ctx context.Context, entityID common.ID, date time.Time) (*domain.Holiday, error)
	IsHoliday(ctx context.Context, entityID common.ID, date time.Time) (bool, error)
	List(ctx context.Context, filter domain.HolidayFilter) ([]domain.Holiday, error)
	GetHolidaysInRange(ctx context.Context, entityID common.ID, start, end time.Time) ([]domain.Holiday, error)
}

type DailyReconciliationRepository interface {
	WithTx(tx *sql.Tx) DailyReconciliationRepository

	Create(ctx context.Context, rec *domain.DailyReconciliation) error
	CreateBatch(ctx context.Context, recs []domain.DailyReconciliation) error
	GetByEODRunID(ctx context.Context, eodRunID common.ID) ([]domain.DailyReconciliation, error)
	GetUnreconciled(ctx context.Context, eodRunID common.ID) ([]domain.DailyReconciliation, error)
}
