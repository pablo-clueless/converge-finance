package ic

import (
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/modules/ic/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/ic/internal/repository"
	"converge-finance.com/m/internal/modules/ic/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api           API
	txService     *service.ICTransactionService
	reconcService *service.ReconciliationService
	elimService   *service.EliminationService
	hierarchyRepo repository.EntityHierarchyRepository
	mappingRepo   repository.AccountMappingRepository
	txRepo        repository.TransactionRepository
	balanceRepo   repository.BalanceRepository
	elimRepo      repository.EliminationRepository
	router        *rest.ICRouter
	logger        *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	Logger      *zap.Logger
	GLAPI       gl.API

	HierarchyRepo repository.EntityHierarchyRepository
	MappingRepo   repository.AccountMappingRepository
	TxRepo        repository.TransactionRepository
	BalanceRepo   repository.BalanceRepository
	ElimRepo      repository.EliminationRepository
}

func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		hierarchyRepo: cfg.HierarchyRepo,
		mappingRepo:   cfg.MappingRepo,
		txRepo:        cfg.TxRepo,
		balanceRepo:   cfg.BalanceRepo,
		elimRepo:      cfg.ElimRepo,
		logger:        cfg.Logger,
	}

	m.txService = service.NewICTransactionService(
		cfg.DB,
		m.txRepo,
		m.mappingRepo,
		m.balanceRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.reconcService = service.NewReconciliationService(
		m.txRepo,
		m.balanceRepo,
		m.hierarchyRepo,
		cfg.AuditLogger,
	)

	m.elimService = service.NewEliminationService(
		cfg.DB,
		m.elimRepo,
		m.balanceRepo,
		m.hierarchyRepo,
		m.mappingRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.router = rest.NewICRouter(
		cfg.Logger,
		m.hierarchyRepo,
		m.mappingRepo,
		m.txService,
		m.reconcService,
		m.elimService,
		cfg.AuditLogger,
	)

	m.api = NewICAPI(
		m.hierarchyRepo,
		m.balanceRepo,
		m.txService,
		m.reconcService,
		m.elimService,
	)

	return m, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) TransactionService() *service.ICTransactionService {
	return m.txService
}

func (m *Module) ReconciliationService() *service.ReconciliationService {
	return m.reconcService
}

func (m *Module) EliminationService() *service.EliminationService {
	return m.elimService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
