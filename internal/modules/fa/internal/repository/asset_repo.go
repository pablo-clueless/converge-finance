package repository

import (
	"context"
	"database/sql"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
)

type AssetRepository interface {
	Create(ctx context.Context, asset *domain.Asset) error

	Update(ctx context.Context, asset *domain.Asset) error

	GetByID(ctx context.Context, id common.ID) (*domain.Asset, error)

	GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id common.ID) (*domain.Asset, error)

	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Asset, error)

	List(ctx context.Context, filter domain.AssetFilter) ([]domain.Asset, error)

	Count(ctx context.Context, filter domain.AssetFilter) (int64, error)

	Delete(ctx context.Context, id common.ID) error

	ExistsByCode(ctx context.Context, entityID common.ID, code string) (bool, error)

	GetDepreciableAssets(ctx context.Context, entityID common.ID, asOfDate time.Time) ([]domain.Asset, error)

	GetAssetsByCategory(ctx context.Context, categoryID common.ID) ([]domain.Asset, error)

	GetAssetsByLocation(ctx context.Context, entityID common.ID, locationCode string) ([]domain.Asset, error)

	GetAssetsByDepartment(ctx context.Context, entityID common.ID, departmentCode string) ([]domain.Asset, error)

	GetAssetsByCustodian(ctx context.Context, custodianID common.ID) ([]domain.Asset, error)

	GetFullyDepreciatedAssets(ctx context.Context, entityID common.ID) ([]domain.Asset, error)

	GetAssetsDueForDepreciation(ctx context.Context, entityID common.ID, throughDate time.Time) ([]domain.Asset, error)

	Search(ctx context.Context, entityID common.ID, query string, limit int) ([]domain.Asset, error)

	UpdateDepreciation(ctx context.Context, asset *domain.Asset) error

	GetNextAssetNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	WithTx(tx *sql.Tx) AssetRepository
}

type AssetSummary struct {
	TotalAssets           int
	TotalAcquisitionCost  float64
	TotalBookValue        float64
	TotalAccumDeprec      float64
	ActiveAssets          int
	DisposedAssets        int
	FullyDepreciatedCount int
}

type AssetsByCategory struct {
	CategoryID     common.ID
	CategoryCode   string
	CategoryName   string
	AssetCount     int
	TotalCost      float64
	TotalBookValue float64
}

type AssetsByLocation struct {
	LocationCode   string
	LocationName   string
	AssetCount     int
	TotalCost      float64
	TotalBookValue float64
}
