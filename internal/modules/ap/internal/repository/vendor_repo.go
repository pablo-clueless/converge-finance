package repository

import (
	"context"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
)

type VendorRepository interface {
	Create(ctx context.Context, vendor *domain.Vendor) error

	Update(ctx context.Context, vendor *domain.Vendor) error

	GetByID(ctx context.Context, id common.ID) (*domain.Vendor, error)

	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Vendor, error)

	List(ctx context.Context, filter domain.VendorFilter) ([]domain.Vendor, error)

	Count(ctx context.Context, filter domain.VendorFilter) (int, error)

	Delete(ctx context.Context, id common.ID) error

	UpdateBalance(ctx context.Context, vendorID common.ID, balance domain.VendorBalance) error

	GetBalance(ctx context.Context, vendorID common.ID) (*domain.VendorBalance, error)

	Search(ctx context.Context, entityID common.ID, query string, limit int) ([]domain.Vendor, error)

	GetVendorsRequiring1099(ctx context.Context, entityID common.ID, year int) ([]domain.Vendor, error)
}
