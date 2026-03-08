package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/gl/internal/domain"
)

type AccountRepository interface {
	Create(ctx context.Context, account *domain.Account) error

	Update(ctx context.Context, account *domain.Account) error

	GetByID(ctx context.Context, id common.ID) (*domain.Account, error)

	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Account, error)

	List(ctx context.Context, filter domain.AccountFilter) ([]domain.Account, error)

	Count(ctx context.Context, filter domain.AccountFilter) (int64, error)

	GetTree(ctx context.Context, entityID common.ID) ([]domain.Account, error)

	GetChildren(ctx context.Context, parentID common.ID) ([]domain.Account, error)

	Delete(ctx context.Context, id common.ID) error

	ExistsByCode(ctx context.Context, entityID common.ID, code string) (bool, error)

	GetPostingAccounts(ctx context.Context, entityID common.ID) ([]domain.Account, error)

	WithTx(tx *sql.Tx) AccountRepository
}

type AccountBalanceRepository interface {
	GetByAccountAndPeriod(ctx context.Context, accountID, periodID common.ID) (*AccountBalance, error)

	GetByPeriod(ctx context.Context, periodID common.ID) ([]AccountBalance, error)

	UpsertBalance(ctx context.Context, balance *AccountBalance) error

	RecalculateBalance(ctx context.Context, accountID, periodID common.ID) error

	RollForward(ctx context.Context, fromPeriodID, toPeriodID common.ID) error

	WithTx(tx *sql.Tx) AccountBalanceRepository
}

type AccountBalance struct {
	ID             common.ID
	EntityID       common.ID
	AccountID      common.ID
	FiscalPeriodID common.ID
	OpeningDebit   float64
	OpeningCredit  float64
	PeriodDebit    float64
	PeriodCredit   float64
	ClosingDebit   float64
	ClosingCredit  float64
}

func (ab AccountBalance) NetBalance() float64 {
	return (ab.ClosingDebit - ab.ClosingCredit)
}

func (ab AccountBalance) NetPeriodActivity() float64 {
	return (ab.PeriodDebit - ab.PeriodCredit)
}
