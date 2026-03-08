package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
)

type PeriodCloseRepository interface {
	WithTx(tx *sql.Tx) PeriodCloseRepository

	Create(ctx context.Context, pc *domain.PeriodClose) error

	Update(ctx context.Context, pc *domain.PeriodClose) error

	GetByID(ctx context.Context, id common.ID) (*domain.PeriodClose, error)

	GetByPeriod(ctx context.Context, entityID, fiscalPeriodID common.ID) (*domain.PeriodClose, error)

	List(ctx context.Context, filter domain.PeriodCloseFilter) ([]domain.PeriodClose, error)

	GetOpenPeriods(ctx context.Context, entityID common.ID) ([]domain.PeriodClose, error)

	GetClosedPeriods(ctx context.Context, entityID common.ID) ([]domain.PeriodClose, error)
}

type CloseRuleRepository interface {
	WithTx(tx *sql.Tx) CloseRuleRepository

	Create(ctx context.Context, rule *domain.CloseRule) error

	Update(ctx context.Context, rule *domain.CloseRule) error

	GetByID(ctx context.Context, id common.ID) (*domain.CloseRule, error)

	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.CloseRule, error)

	List(ctx context.Context, filter domain.CloseRuleFilter) ([]domain.CloseRule, error)

	GetActiveRulesForCloseType(ctx context.Context, entityID common.ID, closeType domain.CloseType) ([]domain.CloseRule, error)

	Delete(ctx context.Context, id common.ID) error
}
