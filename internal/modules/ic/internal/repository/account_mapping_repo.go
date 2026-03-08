package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
)

type AccountMappingRepository interface {
	Create(ctx context.Context, mapping *domain.AccountMapping) error

	Update(ctx context.Context, mapping *domain.AccountMapping) error

	GetByID(ctx context.Context, id common.ID) (*domain.AccountMapping, error)

	GetByEntityPair(ctx context.Context, fromEntityID, toEntityID common.ID, transactionType domain.TransactionType) (*domain.AccountMapping, error)

	GetByEntityPairAny(ctx context.Context, entityID1, entityID2 common.ID, transactionType domain.TransactionType) (*domain.AccountMapping, error)

	List(ctx context.Context, filter domain.AccountMappingFilter) ([]domain.AccountMapping, error)

	Count(ctx context.Context, filter domain.AccountMappingFilter) (int64, error)

	Delete(ctx context.Context, id common.ID) error

	GetAllForEntity(ctx context.Context, entityID common.ID) ([]domain.AccountMapping, error)

	GetActiveForEntity(ctx context.Context, entityID common.ID) ([]domain.AccountMapping, error)

	ExistsByEntityPair(ctx context.Context, fromEntityID, toEntityID common.ID, transactionType domain.TransactionType) (bool, error)

	WithTx(tx *sql.Tx) AccountMappingRepository
}
