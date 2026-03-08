package service

import (
	"context"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
	"converge-finance.com/m/internal/modules/ar/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"go.uber.org/zap"
)

type CustomerService struct {
	customerRepo repository.CustomerRepository
	auditLogger  *audit.Logger
	logger       *zap.Logger
}

func NewCustomerService(
	customerRepo repository.CustomerRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *CustomerService {
	return &CustomerService{
		customerRepo: customerRepo,
		auditLogger:  auditLogger,
		logger:       logger,
	}
}

type CreateCustomerRequest struct {
	EntityID       common.ID
	CustomerCode   string
	Name           string
	LegalName      string
	CustomerType   domain.CustomerType
	TaxID          string
	Email          string
	Phone          string
	Website        string
	Currency       string
	PaymentTerms   domain.PaymentTerms
	CreditLimit    float64
	DunningEnabled bool
	Notes          string
	CreatedBy      common.ID
}

type UpdateCustomerRequest struct {
	ID             common.ID
	Name           *string
	LegalName      *string
	TaxID          *string
	Email          *string
	Phone          *string
	Website        *string
	PaymentTerms   *domain.PaymentTerms
	CreditLimit    *float64
	DunningEnabled *bool
	Notes          *string
}

func (s *CustomerService) CreateCustomer(ctx context.Context, req CreateCustomerRequest) (*domain.Customer, error) {

	currency, err := money.GetCurrency(req.Currency)
	if err != nil {
		return nil, fmt.Errorf("invalid currency: %w", err)
	}

	existing, err := s.customerRepo.GetByCode(ctx, req.EntityID, req.CustomerCode)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("customer code '%s' already exists", req.CustomerCode)
	}

	customer, err := domain.NewCustomer(req.EntityID, req.CustomerCode, req.Name, req.CustomerType, currency, req.CreatedBy)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	customer.LegalName = req.LegalName
	customer.TaxID = req.TaxID
	customer.Email = req.Email
	customer.Phone = req.Phone
	customer.Website = req.Website
	customer.DunningEnabled = req.DunningEnabled
	customer.Notes = req.Notes

	if req.PaymentTerms != "" {
		customer.PaymentTerms = req.PaymentTerms
	}
	if req.CreditLimit > 0 {
		customer.CreditLimit = money.New(req.CreditLimit, currency.Code)
		customer.AvailableCredit = customer.CreditLimit
	}

	if err := customer.Validate(); err != nil {
		return nil, err
	}

	if err := s.customerRepo.Create(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to save customer: %w", err)
	}

	s.auditLogger.Log(ctx, "customer", customer.ID, "customer.created", map[string]any{
		"customer_code": customer.CustomerCode,
		"name":          customer.Name,
	})

	s.logger.Info("Customer created",
		zap.String("customer_id", customer.ID.String()),
		zap.String("customer_code", customer.CustomerCode),
	)

	return customer, nil
}

func (s *CustomerService) UpdateCustomer(ctx context.Context, req UpdateCustomerRequest) (*domain.Customer, error) {
	customer, err := s.customerRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("customer not found: %w", err)
	}

	if req.Name != nil {
		customer.Name = *req.Name
	}
	if req.LegalName != nil {
		customer.LegalName = *req.LegalName
	}
	if req.TaxID != nil {
		customer.TaxID = *req.TaxID
	}
	if req.Email != nil {
		customer.Email = *req.Email
	}
	if req.Phone != nil {
		customer.Phone = *req.Phone
	}
	if req.Website != nil {
		customer.Website = *req.Website
	}
	if req.PaymentTerms != nil {
		customer.PaymentTerms = *req.PaymentTerms
	}
	if req.CreditLimit != nil {
		customer.CreditLimit = money.New(*req.CreditLimit, customer.Currency.Code)
		customer.UpdateBalance(customer.CurrentBalance)
	}
	if req.DunningEnabled != nil {
		customer.DunningEnabled = *req.DunningEnabled
	}
	if req.Notes != nil {
		customer.Notes = *req.Notes
	}

	if err := customer.Validate(); err != nil {
		return nil, err
	}

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	s.logger.Info("Customer updated",
		zap.String("customer_id", customer.ID.String()),
	)

	return customer, nil
}

func (s *CustomerService) GetCustomer(ctx context.Context, id common.ID) (*domain.Customer, error) {
	return s.customerRepo.GetByID(ctx, id)
}

func (s *CustomerService) GetCustomerByCode(ctx context.Context, entityID common.ID, code string) (*domain.Customer, error) {
	return s.customerRepo.GetByCode(ctx, entityID, code)
}

func (s *CustomerService) ListCustomers(ctx context.Context, filter domain.CustomerFilter) ([]domain.Customer, int, error) {
	customers, err := s.customerRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.customerRepo.Count(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	return customers, count, nil
}

func (s *CustomerService) SearchCustomers(ctx context.Context, entityID common.ID, query string, limit int) ([]domain.Customer, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.customerRepo.Search(ctx, entityID, query, limit)
}

func (s *CustomerService) ActivateCustomer(ctx context.Context, id common.ID) error {
	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	customer.Activate()

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to activate customer: %w", err)
	}

	s.logger.Info("Customer activated", zap.String("customer_id", id.String()))
	return nil
}

func (s *CustomerService) DeactivateCustomer(ctx context.Context, id common.ID) error {
	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	customer.Deactivate()

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to deactivate customer: %w", err)
	}

	s.logger.Info("Customer deactivated", zap.String("customer_id", id.String()))
	return nil
}

func (s *CustomerService) SuspendCustomer(ctx context.Context, id common.ID, reason string) error {
	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	customer.Suspend(reason)

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to suspend customer: %w", err)
	}

	s.auditLogger.Log(ctx, "customer", customer.ID, "customer.suspended", map[string]any{
		"reason": reason,
	})

	s.logger.Info("Customer suspended", zap.String("customer_id", id.String()))
	return nil
}

func (s *CustomerService) ReleaseCreditHold(ctx context.Context, id common.ID, approvedBy common.ID) error {
	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	customer.ReleaseCreditHold(approvedBy)

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to release credit hold: %w", err)
	}

	s.auditLogger.Log(ctx, "customer", customer.ID, "customer.credit_hold_released", map[string]any{
		"approved_by": approvedBy.String(),
	})

	s.logger.Info("Customer credit hold released", zap.String("customer_id", id.String()))
	return nil
}

func (s *CustomerService) SetCreditLimit(ctx context.Context, id common.ID, limit float64, approvedBy common.ID) error {
	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	limitMoney := money.New(limit, customer.Currency.Code)
	customer.SetCreditLimit(limitMoney, approvedBy)

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to set credit limit: %w", err)
	}

	s.auditLogger.Log(ctx, "customer", customer.ID, "customer.credit_limit_changed", map[string]any{
		"new_limit":   limit,
		"approved_by": approvedBy.String(),
	})

	s.logger.Info("Customer credit limit updated", zap.String("customer_id", id.String()))
	return nil
}

func (s *CustomerService) GetCustomerBalance(ctx context.Context, customerID common.ID) (*domain.CustomerBalance, error) {
	return s.customerRepo.GetBalance(ctx, customerID)
}

func (s *CustomerService) GetCustomersOnCreditHold(ctx context.Context, entityID common.ID) ([]domain.Customer, error) {
	return s.customerRepo.GetCustomersOnCreditHold(ctx, entityID)
}

func (s *CustomerService) GetCustomersOverCreditLimit(ctx context.Context, entityID common.ID) ([]domain.Customer, error) {
	return s.customerRepo.GetCustomersOverCreditLimit(ctx, entityID)
}

func (s *CustomerService) AddContact(ctx context.Context, customerID common.ID, name, title, email, phone string, isPrimary bool) error {
	customer, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	customer.AddContact(name, title, email, phone, isPrimary)

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to add contact: %w", err)
	}

	s.logger.Info("Contact added to customer", zap.String("customer_id", customerID.String()))
	return nil
}

func (s *CustomerService) SetBillingAddress(ctx context.Context, customerID common.ID, address domain.CustomerAddress) error {
	customer, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	customer.BillingAddress = address

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to update billing address: %w", err)
	}

	s.logger.Info("Customer billing address updated", zap.String("customer_id", customerID.String()))
	return nil
}

func (s *CustomerService) SetShippingAddress(ctx context.Context, customerID common.ID, address *domain.CustomerAddress) error {
	customer, err := s.customerRepo.GetByID(ctx, customerID)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	customer.ShippingAddress = address

	if err := s.customerRepo.Update(ctx, customer); err != nil {
		return fmt.Errorf("failed to update shipping address: %w", err)
	}

	s.logger.Info("Customer shipping address updated", zap.String("customer_id", customerID.String()))
	return nil
}

func (s *CustomerService) DeleteCustomer(ctx context.Context, id common.ID) error {
	customer, err := s.customerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("customer not found: %w", err)
	}

	if !customer.CurrentBalance.IsZero() {
		return fmt.Errorf("cannot delete customer with outstanding balance")
	}

	if err := s.customerRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete customer: %w", err)
	}

	s.logger.Info("Customer deleted", zap.String("customer_id", id.String()))
	return nil
}
