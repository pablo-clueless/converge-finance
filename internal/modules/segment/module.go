package segment

import (
	"converge-finance.com/m/internal/modules/segment/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/segment/internal/repository"
	"converge-finance.com/m/internal/modules/segment/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api              API
	segmentService   *service.SegmentService
	reportService    *service.ReportService
	segmentRepo      repository.SegmentRepository
	hierarchyRepo    repository.SegmentHierarchyRepository
	assignmentRepo   repository.AssignmentRepository
	balanceRepo      repository.BalanceRepository
	intersegmentRepo repository.IntersegmentTransactionRepository
	reportRepo       repository.ReportRepository
	reportDataRepo   repository.ReportDataRepository
	router           *rest.SegmentRouter
	logger           *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	JWTService  *auth.JWTService
	Logger      *zap.Logger
}

func NewModule(cfg Config) (*Module, error) {

	segmentRepo := repository.NewPostgresSegmentRepo(cfg.DB)
	hierarchyRepo := repository.NewPostgresSegmentHierarchyRepo(cfg.DB)
	assignmentRepo := repository.NewPostgresAssignmentRepo(cfg.DB)
	balanceRepo := repository.NewPostgresBalanceRepo(cfg.DB)
	intersegmentRepo := repository.NewPostgresIntersegmentTransactionRepo(cfg.DB)
	reportRepo := repository.NewPostgresReportRepo(cfg.DB)
	reportDataRepo := repository.NewPostgresReportDataRepo(cfg.DB)

	segmentService := service.NewSegmentService(
		segmentRepo,
		hierarchyRepo,
		assignmentRepo,
		balanceRepo,
		intersegmentRepo,
		cfg.AuditLogger,
		cfg.Logger,
	)

	reportService := service.NewReportService(
		reportRepo,
		reportDataRepo,
		segmentRepo,
		balanceRepo,
		cfg.AuditLogger,
		cfg.Logger,
	)

	segmentHandler := rest.NewSegmentHandler(segmentService, cfg.Logger)
	reportHandler := rest.NewReportHandler(reportService, cfg.Logger)

	router := rest.NewSegmentRouter(segmentHandler, reportHandler, cfg.JWTService, cfg.Logger)

	api := NewSegmentAPI(segmentService, reportService)

	return &Module{
		api:              api,
		segmentService:   segmentService,
		reportService:    reportService,
		segmentRepo:      segmentRepo,
		hierarchyRepo:    hierarchyRepo,
		assignmentRepo:   assignmentRepo,
		balanceRepo:      balanceRepo,
		intersegmentRepo: intersegmentRepo,
		reportRepo:       reportRepo,
		reportDataRepo:   reportDataRepo,
		router:           router,
		logger:           cfg.Logger,
	}, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) SegmentService() *service.SegmentService {
	return m.segmentService
}

func (m *Module) ReportService() *service.ReportService {
	return m.reportService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
