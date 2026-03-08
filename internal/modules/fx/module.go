package fx

import (
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fx/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/fx/internal/repository"
	"converge-finance.com/m/internal/modules/fx/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api                   API
	triangulationService  *service.TriangulationService
	revaluationService    *service.RevaluationService
	configRepo            repository.TriangulationConfigRepository
	pairConfigRepo        repository.CurrencyPairConfigRepository
	logRepo               repository.TriangulationLogRepository
	accountFXConfigRepo   repository.AccountFXConfigRepository
	revaluationRunRepo    repository.RevaluationRunRepository
	revaluationDetailRepo repository.RevaluationDetailRepository
	router                *rest.FXRouter
	logger                *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	JWTService  *auth.JWTService
	Logger      *zap.Logger
	RateService money.ExchangeRateService
}

func NewModule(cfg Config) (*Module, error) {

	configRepo := repository.NewPostgresTriangulationConfigRepo(cfg.DB)
	pairConfigRepo := repository.NewPostgresCurrencyPairConfigRepo(cfg.DB)
	logRepo := repository.NewPostgresTriangulationLogRepo(cfg.DB)

	accountFXConfigRepo := repository.NewPostgresAccountFXConfigRepo(cfg.DB)
	revaluationRunRepo := repository.NewPostgresRevaluationRunRepo(cfg.DB)
	revaluationDetailRepo := repository.NewPostgresRevaluationDetailRepo(cfg.DB)

	triangulationService := service.NewTriangulationService(
		configRepo,
		pairConfigRepo,
		logRepo,
		cfg.RateService,
		cfg.AuditLogger,
		cfg.Logger,
	)

	revaluationService := service.NewRevaluationService(
		accountFXConfigRepo,
		revaluationRunRepo,
		revaluationDetailRepo,
		cfg.RateService,
		cfg.AuditLogger,
		cfg.Logger,
	)

	triangulationHandler := rest.NewTriangulationHandler(triangulationService, cfg.Logger)
	revaluationHandler := rest.NewRevaluationHandler(revaluationService, cfg.Logger)

	router := rest.NewFXRouter(triangulationHandler, revaluationHandler, cfg.JWTService, cfg.Logger)

	api := NewFXAPI(triangulationService, revaluationService)

	return &Module{
		api:                   api,
		triangulationService:  triangulationService,
		revaluationService:    revaluationService,
		configRepo:            configRepo,
		pairConfigRepo:        pairConfigRepo,
		logRepo:               logRepo,
		accountFXConfigRepo:   accountFXConfigRepo,
		revaluationRunRepo:    revaluationRunRepo,
		revaluationDetailRepo: revaluationDetailRepo,
		router:                router,
		logger:                cfg.Logger,
	}, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) TriangulationService() *service.TriangulationService {
	return m.triangulationService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
