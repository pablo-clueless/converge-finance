package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
)

type PostgresJournalRepository struct {
	db *sql.DB
}

func NewPostgresJournalRepository(db *sql.DB) *PostgresJournalRepository {
	return &PostgresJournalRepository{db: db}
}

func (r *PostgresJournalRepository) Create(ctx context.Context, entry *domain.JournalEntry) error {
	return nil
}

func (r *PostgresJournalRepository) Update(ctx context.Context, entry *domain.JournalEntry) error {
	return nil
}

func (r *PostgresJournalRepository) GetByID(ctx context.Context, id common.ID) (*domain.JournalEntry, error) {
	return nil, nil
}

func (r *PostgresJournalRepository) GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id common.ID) (*domain.JournalEntry, error) {
	return nil, nil
}

func (r *PostgresJournalRepository) GetByNumber(ctx context.Context, entityID common.ID, entryNumber string) (*domain.JournalEntry, error) {
	return nil, nil
}

func (r *PostgresJournalRepository) List(ctx context.Context, filter domain.JournalEntryFilter) ([]domain.JournalEntry, error) {
	return []domain.JournalEntry{}, nil
}

func (r *PostgresJournalRepository) Count(ctx context.Context, filter domain.JournalEntryFilter) (int64, error) {
	return 0, nil
}

func (r *PostgresJournalRepository) GetBySourceReference(ctx context.Context, source, reference string) (*domain.JournalEntry, error) {
	return nil, nil
}

func (r *PostgresJournalRepository) GetPostedByPeriod(ctx context.Context, periodID common.ID) ([]domain.JournalEntry, error) {
	return []domain.JournalEntry{}, nil
}

func (r *PostgresJournalRepository) GetNextEntryNumber(ctx context.Context, entityID common.ID, prefix string) (string, error) {
	return "JE-0001", nil
}

func (r *PostgresJournalRepository) AddLine(ctx context.Context, line *domain.JournalLine) error {
	return nil
}

func (r *PostgresJournalRepository) UpdateLine(ctx context.Context, line *domain.JournalLine) error {
	return nil
}

func (r *PostgresJournalRepository) DeleteLine(ctx context.Context, lineID common.ID) error {
	return nil
}

func (r *PostgresJournalRepository) GetLinesByEntry(ctx context.Context, entryID common.ID) ([]domain.JournalLine, error) {
	return []domain.JournalLine{}, nil
}

func (r *PostgresJournalRepository) GetLinesByAccount(ctx context.Context, accountID common.ID, filter JournalLineFilter) ([]domain.JournalLine, error) {
	return []domain.JournalLine{}, nil
}

func (r *PostgresJournalRepository) GetAccountActivity(ctx context.Context, accountID, periodID common.ID) (debit, credit float64, err error) {
	return 0, 0, nil
}

func (r *PostgresJournalRepository) WithTx(tx *sql.Tx) JournalRepository {
	return r
}
