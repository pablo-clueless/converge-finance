package cost

import (
	"converge-finance.com/m/internal/modules/cost/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/cost/internal/repository"
	"converge-finance.com/m/internal/modules/cost/internal/service"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api               API
	costCenterService *service.CostCenterService
	allocationService *service.AllocationService
	budgetService     *service.BudgetService
	router            *rest.CostRouter
	logger            *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	Logger      *zap.Logger
	GLAPI       gl.API

	CostCenterRepo repository.CostCenterRepository
	RuleRepo       repository.AllocationRuleRepository
	RunRepo        repository.AllocationRunRepository
	BudgetRepo     repository.BudgetRepository
	LineRepo       repository.BudgetLineRepository
	TransferRepo   repository.BudgetTransferRepository
	ActualRepo     repository.BudgetActualRepository
}

func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		logger: cfg.Logger,
	}

	m.costCenterService = service.NewCostCenterService(
		cfg.CostCenterRepo,
		cfg.AuditLogger,
	)

	m.allocationService = service.NewAllocationService(
		cfg.DB,
		cfg.RuleRepo,
		cfg.RunRepo,
		cfg.CostCenterRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.budgetService = service.NewBudgetService(
		cfg.DB,
		cfg.BudgetRepo,
		cfg.LineRepo,
		cfg.TransferRepo,
		cfg.ActualRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.router = rest.NewCostRouter(
		cfg.Logger,
		m.costCenterService,
		m.allocationService,
		m.budgetService,
	)

	m.api = NewCostAPI(
		m.costCenterService,
		m.allocationService,
		m.budgetService,
	)

	return m, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) CostCenterService() *service.CostCenterService {
	return m.costCenterService
}

func (m *Module) AllocationService() *service.AllocationService {
	return m.allocationService
}

func (m *Module) BudgetService() *service.BudgetService {
	return m.budgetService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
