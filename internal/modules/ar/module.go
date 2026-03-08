package ar

import (
	"converge-finance.com/m/internal/modules/ar/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/ar/internal/repository"
	"converge-finance.com/m/internal/modules/ar/internal/service"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	customerService *service.CustomerService
	invoiceService  *service.InvoiceService
	receiptService  *service.ReceiptService
	dunningService  *service.DunningService
	customerRepo    repository.CustomerRepository
	invoiceRepo     repository.InvoiceRepository
	receiptRepo     repository.ReceiptRepository
	router          *rest.ARRouter
	logger          *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	JWTService  *auth.JWTService
	Logger      *zap.Logger
	GLAPI       gl.API

	CustomerRepo repository.CustomerRepository
	InvoiceRepo  repository.InvoiceRepository
	ReceiptRepo  repository.ReceiptRepository
}

func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		customerRepo: cfg.CustomerRepo,
		invoiceRepo:  cfg.InvoiceRepo,
		receiptRepo:  cfg.ReceiptRepo,
		logger:       cfg.Logger,
	}

	m.customerService = service.NewCustomerService(
		m.customerRepo,
		cfg.AuditLogger,
		cfg.Logger,
	)

	m.invoiceService = service.NewInvoiceService(
		m.invoiceRepo,
		m.customerRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
		cfg.Logger,
	)

	m.receiptService = service.NewReceiptService(
		m.receiptRepo,
		m.invoiceRepo,
		m.customerRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
		cfg.Logger,
	)

	m.dunningService = service.NewDunningService(
		m.invoiceRepo,
		m.customerRepo,
		cfg.Logger,
	)

	m.router = rest.NewARRouter(
		cfg.Logger,
		m.customerService,
		m.invoiceService,
		m.receiptService,
		m.dunningService,
		cfg.JWTService,
	)

	return m, nil
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}

func (m *Module) CustomerService() *service.CustomerService {
	return m.customerService
}

func (m *Module) InvoiceService() *service.InvoiceService {
	return m.invoiceService
}

func (m *Module) ReceiptService() *service.ReceiptService {
	return m.receiptService
}

func (m *Module) DunningService() *service.DunningService {
	return m.dunningService
}
