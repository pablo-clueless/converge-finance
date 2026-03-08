package repository

import (
	"context"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
)

type CustomerRepository interface {
	Create(ctx context.Context, customer *domain.Customer) error

	Update(ctx context.Context, customer *domain.Customer) error

	GetByID(ctx context.Context, id common.ID) (*domain.Customer, error)

	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Customer, error)

	List(ctx context.Context, filter domain.CustomerFilter) ([]domain.Customer, error)

	Count(ctx context.Context, filter domain.CustomerFilter) (int, error)

	Delete(ctx context.Context, id common.ID) error

	UpdateBalance(ctx context.Context, customerID common.ID, balance domain.CustomerBalance) error

	GetBalance(ctx context.Context, customerID common.ID) (*domain.CustomerBalance, error)

	Search(ctx context.Context, entityID common.ID, query string, limit int) ([]domain.Customer, error)

	GetCustomersOnCreditHold(ctx context.Context, entityID common.ID) ([]domain.Customer, error)

	GetCustomersOverCreditLimit(ctx context.Context, entityID common.ID) ([]domain.Customer, error)
}
