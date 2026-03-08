package ap

import (
	"converge-finance.com/m/internal/modules/ap/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/ap/internal/repository"
	"converge-finance.com/m/internal/modules/ap/internal/service"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Module represents the Accounts Payable module
type Module struct {
	vendorService  *service.VendorService
	invoiceService *service.InvoiceService
	paymentService *service.PaymentService
	agingService   *service.AgingService
	vendorRepo     repository.VendorRepository
	invoiceRepo    repository.InvoiceRepository
	paymentRepo    repository.PaymentRepository
	router         *rest.APRouter
	logger         *zap.Logger
}

// Config holds the configuration for the AP module
type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	JWTService  *auth.JWTService
	Logger      *zap.Logger
	GLAPI       gl.API

	// Repositories (injected for testing)
	VendorRepo  repository.VendorRepository
	InvoiceRepo repository.InvoiceRepository
	PaymentRepo repository.PaymentRepository
}

// NewModule creates a new AP module
func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		vendorRepo:  cfg.VendorRepo,
		invoiceRepo: cfg.InvoiceRepo,
		paymentRepo: cfg.PaymentRepo,
		logger:      cfg.Logger,
	}

	// Create services
	m.vendorService = service.NewVendorService(
		m.vendorRepo,
		cfg.AuditLogger,
		cfg.Logger,
	)

	m.invoiceService = service.NewInvoiceService(
		m.invoiceRepo,
		m.vendorRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
		cfg.Logger,
	)

	m.paymentService = service.NewPaymentService(
		m.paymentRepo,
		m.invoiceRepo,
		m.vendorRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
		cfg.Logger,
	)

	m.agingService = service.NewAgingService(
		m.invoiceRepo,
		m.vendorRepo,
		m.paymentRepo,
		cfg.Logger,
	)

	// Create router
	m.router = rest.NewAPRouter(
		cfg.Logger,
		m.vendorService,
		m.invoiceService,
		m.paymentService,
		m.agingService,
		cfg.JWTService,
	)

	return m, nil
}

// RegisterRoutes registers the AP module routes with the router
func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}

// VendorService returns the vendor service
func (m *Module) VendorService() *service.VendorService {
	return m.vendorService
}

// InvoiceService returns the invoice service
func (m *Module) InvoiceService() *service.InvoiceService {
	return m.invoiceService
}

// PaymentService returns the payment service
func (m *Module) PaymentService() *service.PaymentService {
	return m.paymentService
}

// AgingService returns the aging service
func (m *Module) AgingService() *service.AgingService {
	return m.agingService
}
