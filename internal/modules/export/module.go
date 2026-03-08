package export

import (
	"converge-finance.com/m/internal/modules/export/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/export/internal/repository"
	"converge-finance.com/m/internal/modules/export/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api           API
	exportService *service.ExportService
	templateRepo  repository.TemplateRepository
	jobRepo       repository.JobRepository
	scheduleRepo  repository.ScheduleRepository
	router        *rest.ExportRouter
	logger        *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	JWTService  *auth.JWTService
	Logger      *zap.Logger
	ExportPath  string
}

func NewModule(cfg Config) (*Module, error) {
	templateRepo := repository.NewPostgresTemplateRepo(cfg.DB)
	jobRepo := repository.NewPostgresJobRepo(cfg.DB)
	scheduleRepo := repository.NewPostgresScheduleRepo(cfg.DB)

	exportPath := cfg.ExportPath
	if exportPath == "" {
		exportPath = "/tmp/exports"
	}

	exportService := service.NewExportService(
		templateRepo,
		jobRepo,
		scheduleRepo,
		cfg.AuditLogger,
		cfg.Logger,
		exportPath,
	)

	handler := rest.NewExportHandler(exportService, cfg.Logger)
	router := rest.NewExportRouter(handler, cfg.JWTService, cfg.Logger)

	api := NewExportAPI(exportService)

	return &Module{
		api:           api,
		exportService: exportService,
		templateRepo:  templateRepo,
		jobRepo:       jobRepo,
		scheduleRepo:  scheduleRepo,
		router:        router,
		logger:        cfg.Logger,
	}, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) ExportService() *service.ExportService {
	return m.exportService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
