package fa

import (
	"converge-finance.com/m/internal/modules/fa/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/modules/fa/internal/service"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api          API
	assetService *service.AssetService
	depEngine    *service.DepreciationEngine
	categoryRepo repository.CategoryRepository
	assetRepo    repository.AssetRepository
	depRepo      repository.DepreciationRepository
	transferRepo repository.TransferRepository
	router       *rest.FARouter
	logger       *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	Logger      *zap.Logger
	GLAPI       gl.API

	CategoryRepo repository.CategoryRepository
	AssetRepo    repository.AssetRepository
	DepRepo      repository.DepreciationRepository
	TransferRepo repository.TransferRepository
}

func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		categoryRepo: cfg.CategoryRepo,
		assetRepo:    cfg.AssetRepo,
		depRepo:      cfg.DepRepo,
		transferRepo: cfg.TransferRepo,
		logger:       cfg.Logger,
	}

	m.depEngine = service.NewDepreciationEngine(
		cfg.DB,
		m.assetRepo,
		m.categoryRepo,
		m.depRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.assetService = service.NewAssetService(
		cfg.DB,
		m.assetRepo,
		m.categoryRepo,
		m.transferRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.router = rest.NewFARouter(
		cfg.Logger,
		m.categoryRepo,
		m.assetRepo,
		m.depRepo,
		m.transferRepo,
		m.assetService,
		m.depEngine,
		cfg.AuditLogger,
	)

	m.api = NewFAAPI(
		m.assetRepo,
		m.categoryRepo,
		m.depRepo,
		m.depEngine,
		m.assetService,
	)

	return m, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) AssetService() *service.AssetService {
	return m.assetService
}

func (m *Module) DepreciationEngine() *service.DepreciationEngine {
	return m.depEngine
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
