package fa

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type API interface {
	GetAssetByID(ctx context.Context, assetID common.ID) (*AssetResponse, error)
	GetAssetByCode(ctx context.Context, entityID common.ID, code string) (*AssetResponse, error)
	ListAssets(ctx context.Context, entityID common.ID, filter AssetFilterRequest) ([]AssetResponse, error)

	RunDepreciation(ctx context.Context, entityID common.ID, fiscalPeriodID common.ID, periodDate time.Time, currency money.Currency) (*DepreciationRunResponse, error)
	GetDepreciationRun(ctx context.Context, runID common.ID) (*DepreciationRunResponse, error)

	LinkAPInvoice(ctx context.Context, assetID, invoiceID common.ID) error

	GetCategoryByID(ctx context.Context, categoryID common.ID) (*CategoryResponse, error)
	ListCategories(ctx context.Context, entityID common.ID) ([]CategoryResponse, error)
}

type AssetFilterRequest struct {
	CategoryID *common.ID
	Status     *string
	Search     string
	Limit      int
	Offset     int
}

type AssetResponse struct {
	ID                      common.ID
	EntityID                common.ID
	CategoryID              common.ID
	AssetCode               string
	AssetName               string
	Description             string
	SerialNumber            string
	AcquisitionDate         time.Time
	AcquisitionCost         money.Money
	CurrencyCode            string
	DepreciationMethod      string
	UsefulLifeYears         int
	UsefulLifeUnits         *int
	SalvageValue            money.Money
	DepreciationStartDate   *time.Time
	AccumulatedDepreciation money.Money
	BookValue               money.Money
	UnitsUsed               int
	LastDepreciationDate    *time.Time
	LocationCode            string
	DepartmentCode          string
	Status                  string
	ActivatedAt             *time.Time
	DisposedAt              *time.Time
	DisposalType            *string
	DisposalProceeds        *money.Money
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type CategoryResponse struct {
	ID                           common.ID
	EntityID                     common.ID
	Code                         string
	Name                         string
	Description                  string
	DepreciationMethod           string
	DefaultUsefulLifeYears       int
	DefaultSalvagePercent        float64
	AssetAccountID               *common.ID
	AccumDepreciationAccountID   *common.ID
	DepreciationExpenseAccountID *common.ID
	GainLossAccountID            *common.ID
	IsActive                     bool
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

type DepreciationRunResponse struct {
	ID                common.ID
	EntityID          common.ID
	RunNumber         string
	FiscalPeriodID    common.ID
	DepreciationDate  time.Time
	AssetCount        int
	TotalDepreciation money.Money
	CurrencyCode      string
	Status            string
	JournalEntryID    *common.ID
	PostedAt          *time.Time
	CreatedAt         time.Time
}
