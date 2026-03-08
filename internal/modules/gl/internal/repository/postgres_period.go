package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
)

type PostgresPeriodRepository struct {
	db *sql.DB
}

func NewPostgresPeriodRepository(db *sql.DB) *PostgresPeriodRepository {
	return &PostgresPeriodRepository{db: db}
}

func (r *PostgresPeriodRepository) CreateYear(ctx context.Context, year *domain.FiscalYear) error {
	return nil
}

func (r *PostgresPeriodRepository) UpdateYear(ctx context.Context, year *domain.FiscalYear) error {
	return nil
}

func (r *PostgresPeriodRepository) GetYearByID(ctx context.Context, id common.ID) (*domain.FiscalYear, error) {
	return nil, nil
}

func (r *PostgresPeriodRepository) GetYearByCode(ctx context.Context, entityID common.ID, code string) (*domain.FiscalYear, error) {
	return nil, nil
}

func (r *PostgresPeriodRepository) ListYears(ctx context.Context, entityID common.ID) ([]domain.FiscalYear, error) {
	return []domain.FiscalYear{}, nil
}

func (r *PostgresPeriodRepository) GetCurrentYear(ctx context.Context, entityID common.ID) (*domain.FiscalYear, error) {
	return nil, nil
}

func (r *PostgresPeriodRepository) CreatePeriod(ctx context.Context, period *domain.FiscalPeriod) error {
	return nil
}

func (r *PostgresPeriodRepository) UpdatePeriod(ctx context.Context, period *domain.FiscalPeriod) error {
	return nil
}

func (r *PostgresPeriodRepository) GetPeriodByID(ctx context.Context, id common.ID) (*domain.FiscalPeriod, error) {
	return nil, nil
}

func (r *PostgresPeriodRepository) GetPeriodForDate(ctx context.Context, entityID common.ID, date string) (*domain.FiscalPeriod, error) {
	return nil, nil
}

func (r *PostgresPeriodRepository) GetPeriodsForYear(ctx context.Context, yearID common.ID) ([]domain.FiscalPeriod, error) {
	return []domain.FiscalPeriod{}, nil
}

func (r *PostgresPeriodRepository) GetOpenPeriods(ctx context.Context, entityID common.ID) ([]domain.FiscalPeriod, error) {
	return []domain.FiscalPeriod{}, nil
}

func (r *PostgresPeriodRepository) WithTx(tx *sql.Tx) PeriodRepository {
	return r
}
