package service

import (
	"context"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
	"converge-finance.com/m/internal/modules/ap/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"go.uber.org/zap"
)

type VendorService struct {
	vendorRepo  repository.VendorRepository
	auditLogger *audit.Logger
	logger      *zap.Logger
}

func NewVendorService(
	vendorRepo repository.VendorRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *VendorService {
	return &VendorService{
		vendorRepo:  vendorRepo,
		auditLogger: auditLogger,
		logger:      logger,
	}
}

type CreateVendorRequest struct {
	EntityID      common.ID
	VendorCode    string
	Name          string
	LegalName     string
	TaxID         string
	Email         string
	Phone         string
	Website       string
	Currency      string
	PaymentTerms  domain.PaymentTerms
	PaymentMethod domain.PaymentMethod
	CreditLimit   float64
	Is1099Vendor  bool
	Notes         string
	CreatedBy     common.ID
}

type UpdateVendorRequest struct {
	ID            common.ID
	Name          *string
	LegalName     *string
	TaxID         *string
	Email         *string
	Phone         *string
	Website       *string
	PaymentTerms  *domain.PaymentTerms
	PaymentMethod *domain.PaymentMethod
	CreditLimit   *float64
	Is1099Vendor  *bool
	Notes         *string
}

func (s *VendorService) CreateVendor(ctx context.Context, req CreateVendorRequest) (*domain.Vendor, error) {

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}

	existing, err := s.vendorRepo.GetByCode(ctx, req.EntityID, req.VendorCode)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("vendor code '%s' already exists", req.VendorCode)
	}

	vendor, err := domain.NewVendor(req.EntityID, req.VendorCode, req.Name, currency, req.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create vendor: %w", err)
	}

	vendor.LegalName = req.LegalName
	vendor.TaxID = req.TaxID
	vendor.Email = req.Email
	vendor.Phone = req.Phone
	vendor.Website = req.Website
	vendor.Is1099Vendor = req.Is1099Vendor
	vendor.Notes = req.Notes

	if req.PaymentTerms != "" {
		vendor.PaymentTerms = req.PaymentTerms
	}
	if req.PaymentMethod != "" {
		vendor.PaymentMethod = req.PaymentMethod
	}
	if req.CreditLimit > 0 {
		vendor.CreditLimit = money.New(req.CreditLimit, currency.Code)
	}

	if err := vendor.Validate(); err != nil {
		return nil, err
	}

	if err := s.vendorRepo.Create(ctx, vendor); err != nil {
		return nil, fmt.Errorf("failed to save vendor: %w", err)
	}

	s.auditLogger.Log(ctx, "vendor", vendor.ID, "vendor.created", map[string]any{
		"vendor_code": vendor.VendorCode,
		"name":        vendor.Name,
	})

	s.logger.Info("Vendor created",
		zap.String("vendor_id", vendor.ID.String()),
		zap.String("vendor_code", vendor.VendorCode),
	)

	return vendor, nil
}

func (s *VendorService) UpdateVendor(ctx context.Context, req UpdateVendorRequest) (*domain.Vendor, error) {
	vendor, err := s.vendorRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("vendor not found: %w", err)
	}

	if req.Name != nil {
		vendor.Name = *req.Name
	}
	if req.LegalName != nil {
		vendor.LegalName = *req.LegalName
	}
	if req.TaxID != nil {
		vendor.TaxID = *req.TaxID
	}
	if req.Email != nil {
		vendor.Email = *req.Email
	}
	if req.Phone != nil {
		vendor.Phone = *req.Phone
	}
	if req.Website != nil {
		vendor.Website = *req.Website
	}
	if req.PaymentTerms != nil {
		vendor.PaymentTerms = *req.PaymentTerms
	}
	if req.PaymentMethod != nil {
		vendor.PaymentMethod = *req.PaymentMethod
	}
	if req.CreditLimit != nil {
		vendor.CreditLimit = money.New(*req.CreditLimit, vendor.Currency.Code)
	}
	if req.Is1099Vendor != nil {
		vendor.Is1099Vendor = *req.Is1099Vendor
	}
	if req.Notes != nil {
		vendor.Notes = *req.Notes
	}

	if err := vendor.Validate(); err != nil {
		return nil, err
	}

	if err := s.vendorRepo.Update(ctx, vendor); err != nil {
		return nil, fmt.Errorf("failed to update vendor: %w", err)
	}

	s.logger.Info("Vendor updated",
		zap.String("vendor_id", vendor.ID.String()),
	)

	return vendor, nil
}

func (s *VendorService) GetVendor(ctx context.Context, id common.ID) (*domain.Vendor, error) {
	return s.vendorRepo.GetByID(ctx, id)
}

func (s *VendorService) GetVendorByCode(ctx context.Context, entityID common.ID, code string) (*domain.Vendor, error) {
	return s.vendorRepo.GetByCode(ctx, entityID, code)
}

func (s *VendorService) ListVendors(ctx context.Context, filter domain.VendorFilter) ([]domain.Vendor, int, error) {
	vendors, err := s.vendorRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.vendorRepo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return vendors, count, nil
}

func (s *VendorService) SearchVendors(ctx context.Context, entityID common.ID, query string, limit int) ([]domain.Vendor, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.vendorRepo.Search(ctx, entityID, query, limit)
}

func (s *VendorService) ActivateVendor(ctx context.Context, id common.ID) error {
	vendor, err := s.vendorRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	vendor.Activate()

	if err := s.vendorRepo.Update(ctx, vendor); err != nil {
		return fmt.Errorf("failed to activate vendor: %w", err)
	}

	s.logger.Info("Vendor activated", zap.String("vendor_id", id.String()))
	return nil
}

func (s *VendorService) DeactivateVendor(ctx context.Context, id common.ID) error {
	vendor, err := s.vendorRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	vendor.Deactivate()

	if err := s.vendorRepo.Update(ctx, vendor); err != nil {
		return fmt.Errorf("failed to deactivate vendor: %w", err)
	}

	s.logger.Info("Vendor deactivated", zap.String("vendor_id", id.String()))
	return nil
}

func (s *VendorService) BlockVendor(ctx context.Context, id common.ID) error {
	vendor, err := s.vendorRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	vendor.Block()

	if err := s.vendorRepo.Update(ctx, vendor); err != nil {
		return fmt.Errorf("failed to block vendor: %w", err)
	}

	s.logger.Info("Vendor blocked", zap.String("vendor_id", id.String()))
	return nil
}

func (s *VendorService) GetVendorBalance(ctx context.Context, vendorID common.ID) (*domain.VendorBalance, error) {
	return s.vendorRepo.GetBalance(ctx, vendorID)
}

func (s *VendorService) GetVendorsRequiring1099(ctx context.Context, entityID common.ID, year int) ([]domain.Vendor, error) {
	return s.vendorRepo.GetVendorsRequiring1099(ctx, entityID, year)
}

func (s *VendorService) SetVendorBankInfo(ctx context.Context, vendorID common.ID, bankInfo domain.VendorBankInfo) error {
	vendor, err := s.vendorRepo.GetByID(ctx, vendorID)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	vendor.BankInfo = &bankInfo

	if err := s.vendorRepo.Update(ctx, vendor); err != nil {
		return fmt.Errorf("failed to update vendor bank info: %w", err)
	}

	s.logger.Info("Vendor bank info updated", zap.String("vendor_id", vendorID.String()))
	return nil
}

func (s *VendorService) SetVendorAddress(ctx context.Context, vendorID common.ID, billing domain.VendorAddress, remitTo *domain.VendorAddress) error {
	vendor, err := s.vendorRepo.GetByID(ctx, vendorID)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	vendor.BillingAddress = billing
	vendor.RemitToAddress = remitTo

	if err := s.vendorRepo.Update(ctx, vendor); err != nil {
		return fmt.Errorf("failed to update vendor address: %w", err)
	}

	s.logger.Info("Vendor address updated", zap.String("vendor_id", vendorID.String()))
	return nil
}

func (s *VendorService) DeleteVendor(ctx context.Context, id common.ID) error {

	vendor, err := s.vendorRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("vendor not found: %w", err)
	}

	if !vendor.CurrentBalance.IsZero() {
		return fmt.Errorf("cannot delete vendor with outstanding balance")
	}

	if err := s.vendorRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete vendor: %w", err)
	}

	s.logger.Info("Vendor deleted", zap.String("vendor_id", id.String()))
	return nil
}
