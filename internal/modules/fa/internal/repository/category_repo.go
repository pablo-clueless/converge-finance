package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
)

type CategoryRepository interface {
	Create(ctx context.Context, category *domain.AssetCategory) error

	Update(ctx context.Context, category *domain.AssetCategory) error

	GetByID(ctx context.Context, id common.ID) (*domain.AssetCategory, error)

	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.AssetCategory, error)

	List(ctx context.Context, filter domain.AssetCategoryFilter) ([]domain.AssetCategory, error)

	Count(ctx context.Context, filter domain.AssetCategoryFilter) (int64, error)

	Delete(ctx context.Context, id common.ID) error

	ExistsByCode(ctx context.Context, entityID common.ID, code string) (bool, error)

	GetActiveCategories(ctx context.Context, entityID common.ID) ([]domain.AssetCategory, error)

	WithTx(tx *sql.Tx) CategoryRepository
}
