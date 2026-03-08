package consol

import (
	"converge-finance.com/m/internal/modules/consol/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/consol/internal/repository"
	"converge-finance.com/m/internal/modules/consol/internal/service"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/modules/ic"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api                  API
	consolidationService *service.ConsolidationService
	translationService   *service.TranslationService
	router               *rest.ConsolRouter
	logger               *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	Logger      *zap.Logger
	GLAPI       gl.API
	ICAPI       ic.API

	SetRepo             repository.ConsolidationSetRepository
	RunRepo             repository.ConsolidationRunRepository
	EntityBalanceRepo   repository.EntityBalanceRepository
	ConsolidatedRepo    repository.ConsolidatedBalanceRepository
	MinorityRepo        repository.MinorityInterestRepository
	RateRepo            repository.ExchangeRateRepository
	TranslationAdjRepo  repository.TranslationAdjustmentRepository
}

func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		logger: cfg.Logger,
	}

	m.consolidationService = service.NewConsolidationService(
		cfg.DB,
		cfg.SetRepo,
		cfg.RunRepo,
		cfg.EntityBalanceRepo,
		cfg.ConsolidatedRepo,
		cfg.MinorityRepo,
		cfg.RateRepo,
		cfg.TranslationAdjRepo,
		cfg.GLAPI,
		cfg.ICAPI,
		cfg.AuditLogger,
	)

	m.translationService = service.NewTranslationService(
		cfg.RateRepo,
		cfg.AuditLogger,
	)

	m.router = rest.NewConsolRouter(
		cfg.Logger,
		m.consolidationService,
		m.translationService,
		cfg.AuditLogger,
	)

	m.api = NewConsolAPI(
		m.consolidationService,
		m.translationService,
	)

	return m, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) ConsolidationService() *service.ConsolidationService {
	return m.consolidationService
}

func (m *Module) TranslationService() *service.TranslationService {
	return m.translationService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
