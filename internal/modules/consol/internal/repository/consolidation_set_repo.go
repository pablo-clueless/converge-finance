package repository

import (
	"context"
	"database/sql"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/consol/internal/domain"
)

type ConsolidationSetRepository interface {
	WithTx(tx *sql.Tx) ConsolidationSetRepository

	Create(ctx context.Context, set *domain.ConsolidationSet) error
	Update(ctx context.Context, set *domain.ConsolidationSet) error
	Delete(ctx context.Context, id common.ID) error

	GetByID(ctx context.Context, id common.ID) (*domain.ConsolidationSet, error)
	GetByCode(ctx context.Context, parentEntityID common.ID, code string) (*domain.ConsolidationSet, error)
	List(ctx context.Context, filter domain.ConsolidationSetFilter) ([]domain.ConsolidationSet, error)

	AddMember(ctx context.Context, member *domain.ConsolidationSetMember) error
	UpdateMember(ctx context.Context, member *domain.ConsolidationSetMember) error
	RemoveMember(ctx context.Context, setID, entityID common.ID) error
	GetMembers(ctx context.Context, setID common.ID) ([]domain.ConsolidationSetMember, error)
	GetMember(ctx context.Context, setID, entityID common.ID) (*domain.ConsolidationSetMember, error)

	CreateAccountMapping(ctx context.Context, mapping *domain.AccountMapping) error
	UpdateAccountMapping(ctx context.Context, mapping *domain.AccountMapping) error
	DeleteAccountMapping(ctx context.Context, id common.ID) error
	GetAccountMappings(ctx context.Context, setID, entityID common.ID) ([]domain.AccountMapping, error)
	GetAccountMapping(ctx context.Context, setID, entityID, sourceAccountID common.ID) (*domain.AccountMapping, error)
}
