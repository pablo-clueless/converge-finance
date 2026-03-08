package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
)

type CostCenterRepository interface {
	WithTx(tx *sql.Tx) CostCenterRepository

	Create(ctx context.Context, center *domain.CostCenter) error
	Update(ctx context.Context, center *domain.CostCenter) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.CostCenter, error)
	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.CostCenter, error)
	List(ctx context.Context, filter domain.CostCenterFilter) ([]domain.CostCenter, error)

	GetChildren(ctx context.Context, parentID common.ID) ([]domain.CostCenter, error)
	GetHierarchy(ctx context.Context, rootID common.ID) (*domain.CostCenter, error)

	GetTotalHeadcount(ctx context.Context, entityID common.ID) (int, error)
	GetTotalSquareFootage(ctx context.Context, entityID common.ID) (float64, error)
}
