package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
)

type TransferRepository interface {
	Create(ctx context.Context, transfer *domain.AssetTransfer) error

	Update(ctx context.Context, transfer *domain.AssetTransfer) error

	GetByID(ctx context.Context, id common.ID) (*domain.AssetTransfer, error)

	GetByIDForUpdate(ctx context.Context, tx *sql.Tx, id common.ID) (*domain.AssetTransfer, error)

	GetByNumber(ctx context.Context, entityID common.ID, transferNumber string) (*domain.AssetTransfer, error)

	List(ctx context.Context, filter domain.AssetTransferFilter) ([]domain.AssetTransfer, error)

	Count(ctx context.Context, filter domain.AssetTransferFilter) (int64, error)

	Delete(ctx context.Context, id common.ID) error

	GetByAsset(ctx context.Context, assetID common.ID) ([]domain.AssetTransfer, error)

	GetPendingTransfers(ctx context.Context, entityID common.ID) ([]domain.AssetTransfer, error)

	GetTransferHistory(ctx context.Context, assetID common.ID) ([]domain.AssetTransfer, error)

	GetNextTransferNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)

	WithTx(tx *sql.Tx) TransferRepository
}
