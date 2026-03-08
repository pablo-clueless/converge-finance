package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
)

type ReportTemplateRepository interface {
	WithTx(tx *sql.Tx) ReportTemplateRepository

	Create(ctx context.Context, template *domain.ReportTemplate) error

	Update(ctx context.Context, template *domain.ReportTemplate) error

	GetByID(ctx context.Context, id common.ID) (*domain.ReportTemplate, error)

	GetByCode(ctx context.Context, entityID *common.ID, code string) (*domain.ReportTemplate, error)

	List(ctx context.Context, filter domain.ReportTemplateFilter) ([]domain.ReportTemplate, error)

	GetSystemTemplates(ctx context.Context) ([]domain.ReportTemplate, error)

	Delete(ctx context.Context, id common.ID) error
}

type ReportRunRepository interface {
	WithTx(tx *sql.Tx) ReportRunRepository

	Create(ctx context.Context, run *domain.ReportRun) error

	Update(ctx context.Context, run *domain.ReportRun) error

	GetByID(ctx context.Context, id common.ID) (*domain.ReportRun, error)

	GetByReportNumber(ctx context.Context, entityID common.ID, reportNumber string) (*domain.ReportRun, error)

	List(ctx context.Context, filter domain.ReportRunFilter) ([]domain.ReportRun, error)

	GetNextReportNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	Delete(ctx context.Context, id common.ID) error
}

type ReportDataRepository interface {
	WithTx(tx *sql.Tx) ReportDataRepository

	Create(ctx context.Context, row *domain.ReportDataRow) error

	CreateBatch(ctx context.Context, rows []domain.ReportDataRow) error

	GetByReportRunID(ctx context.Context, reportRunID common.ID) ([]domain.ReportDataRow, error)

	DeleteByReportRunID(ctx context.Context, reportRunID common.ID) error
}

type ScheduledReportRepository interface {
	WithTx(tx *sql.Tx) ScheduledReportRepository

	Create(ctx context.Context, sr *domain.ScheduledReport) error

	Update(ctx context.Context, sr *domain.ScheduledReport) error

	GetByID(ctx context.Context, id common.ID) (*domain.ScheduledReport, error)

	List(ctx context.Context, entityID common.ID) ([]domain.ScheduledReport, error)

	GetDueReports(ctx context.Context) ([]domain.ScheduledReport, error)

	Delete(ctx context.Context, id common.ID) error
}

type YearEndChecklistRepository interface {
	WithTx(tx *sql.Tx) YearEndChecklistRepository

	Create(ctx context.Context, checklist *domain.YearEndChecklist) error

	Update(ctx context.Context, checklist *domain.YearEndChecklist) error

	GetByID(ctx context.Context, id common.ID) (*domain.YearEndChecklist, error)

	GetByFiscalYear(ctx context.Context, entityID, fiscalYearID common.ID) (*domain.YearEndChecklist, error)

	CreateItem(ctx context.Context, item *domain.YearEndChecklistItem) error

	UpdateItem(ctx context.Context, item *domain.YearEndChecklistItem) error

	GetItems(ctx context.Context, checklistID common.ID) ([]domain.YearEndChecklistItem, error)
}
