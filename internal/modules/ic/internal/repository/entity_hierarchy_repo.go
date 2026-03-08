package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/domain"
)

type EntityHierarchyRepository interface {
	GetByID(ctx context.Context, id common.ID) (*domain.EntityHierarchy, error)

	GetByCode(ctx context.Context, code string) (*domain.EntityHierarchy, error)

	List(ctx context.Context, filter domain.EntityHierarchyFilter) ([]domain.EntityHierarchy, error)

	Count(ctx context.Context, filter domain.EntityHierarchyFilter) (int64, error)

	GetHierarchyTree(ctx context.Context, rootID common.ID) (*domain.EntityHierarchy, error)

	GetChildren(ctx context.Context, parentID common.ID) ([]domain.EntityHierarchy, error)

	GetDescendants(ctx context.Context, parentID common.ID) ([]domain.EntityHierarchy, error)

	GetAncestors(ctx context.Context, entityID common.ID) ([]domain.EntityHierarchy, error)

	GetSiblings(ctx context.Context, entityID common.ID) ([]domain.EntityHierarchy, error)

	GetRootEntities(ctx context.Context) ([]domain.EntityHierarchy, error)

	UpdateHierarchy(ctx context.Context, entityID common.ID, parentID *common.ID, entityType domain.EntityType, ownershipPercent float64, consolidationMethod domain.ConsolidationMethod) error

	GetConsolidationGroup(ctx context.Context, parentID common.ID) ([]domain.EntityHierarchy, error)

	GetEliminationEntities(ctx context.Context, parentID common.ID) ([]domain.EntityHierarchy, error)

	WithTx(tx *sql.Tx) EntityHierarchyRepository
}
