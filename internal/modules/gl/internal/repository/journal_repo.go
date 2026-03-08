package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
)

type JournalRepository interface {
	Create(ctx context.Context, entry *domain.JournalEntry) error

	Update(ctx context.Context, entry *domain.JournalEntry) error

	GetByID(ctx context.Context, id common.ID) (*domain.JournalEntry, error)

	GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id common.ID) (*domain.JournalEntry, error)

	GetByNumber(ctx context.Context, entityID common.ID, entryNumber string) (*domain.JournalEntry, error)

	List(ctx context.Context, filter domain.JournalEntryFilter) ([]domain.JournalEntry, error)

	Count(ctx context.Context, filter domain.JournalEntryFilter) (int64, error)

	GetBySourceReference(ctx context.Context, source, reference string) (*domain.JournalEntry, error)

	GetPostedByPeriod(ctx context.Context, periodID common.ID) ([]domain.JournalEntry, error)

	GetNextEntryNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	AddLine(ctx context.Context, line *domain.JournalLine) error

	UpdateLine(ctx context.Context, line *domain.JournalLine) error

	DeleteLine(ctx context.Context, lineID common.ID) error

	GetLinesByEntry(ctx context.Context, entryID common.ID) ([]domain.JournalLine, error)

	GetLinesByAccount(ctx context.Context, accountID common.ID, filter JournalLineFilter) ([]domain.JournalLine, error)

	GetAccountActivity(ctx context.Context, accountID, periodID common.ID) (debit, credit float64, err error)

	WithTx(tx *sql.Tx) JournalRepository
}

type JournalLineFilter struct {
	PeriodID   *common.ID
	DateFrom   *string
	DateTo     *string
	PostedOnly bool
	Limit      int
	Offset     int
}

type PeriodRepository interface {
	CreateYear(ctx context.Context, year *domain.FiscalYear) error

	UpdateYear(ctx context.Context, year *domain.FiscalYear) error

	GetYearByID(ctx context.Context, id common.ID) (*domain.FiscalYear, error)

	GetYearByCode(ctx context.Context, entityID common.ID, code string) (*domain.FiscalYear, error)

	ListYears(ctx context.Context, entityID common.ID) ([]domain.FiscalYear, error)

	GetCurrentYear(ctx context.Context, entityID common.ID) (*domain.FiscalYear, error)

	CreatePeriod(ctx context.Context, period *domain.FiscalPeriod) error

	UpdatePeriod(ctx context.Context, period *domain.FiscalPeriod) error

	GetPeriodByID(ctx context.Context, id common.ID) (*domain.FiscalPeriod, error)

	GetPeriodForDate(ctx context.Context, entityID common.ID, date string) (*domain.FiscalPeriod, error)

	GetPeriodsForYear(ctx context.Context, yearID common.ID) ([]domain.FiscalPeriod, error)

	GetOpenPeriods(ctx context.Context, entityID common.ID) ([]domain.FiscalPeriod, error)

	WithTx(tx *sql.Tx) PeriodRepository
}
