package close

import (
	"converge-finance.com/m/internal/modules/close/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/close/internal/repository"
	"converge-finance.com/m/internal/modules/close/internal/service"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api                API
	periodCloseService *service.PeriodCloseService
	reportService      *service.ReportService
	cashFlowService    *service.CashFlowService
	eodService         *service.EODService
	router             *rest.CloseRouter
	logger             *zap.Logger
}

type Config struct {
	DB                   *database.PostgresDB
	AuditLogger          *audit.Logger
	Logger               *zap.Logger
	GLAPI                gl.API
	PeriodCloseRepo      repository.PeriodCloseRepository
	CloseRuleRepo        repository.CloseRuleRepository
	CloseRunRepo         repository.CloseRunRepository
	CloseEntryRepo       repository.CloseRunEntryRepository
	ReportTemplateRepo   repository.ReportTemplateRepository
	ReportRunRepo        repository.ReportRunRepository
	ReportDataRepo       repository.ReportDataRepository
	ScheduledRepo        repository.ScheduledReportRepository
	YearEndCheckRepo     repository.YearEndChecklistRepository
	CashFlowConfigRepo   repository.AccountCashFlowConfigRepository
	CashFlowTemplateRepo repository.CashFlowTemplateRepository
	CashFlowRunRepo      repository.CashFlowRunRepository
	CashFlowLineRepo     repository.CashFlowLineRepository
	BusinessDateRepo     repository.BusinessDateRepository
	EODConfigRepo        repository.EODConfigRepository
	EODRunRepo           repository.EODRunRepository
	EODTaskRepo          repository.EODTaskRepository
	EODTaskRunRepo       repository.EODTaskRunRepository
	HolidayRepo          repository.HolidayRepository
	ReconciliationRepo   repository.DailyReconciliationRepository
}

func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		logger: cfg.Logger,
	}

	m.periodCloseService = service.NewPeriodCloseService(
		cfg.DB,
		cfg.PeriodCloseRepo,
		cfg.CloseRuleRepo,
		cfg.CloseRunRepo,
		cfg.CloseEntryRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.reportService = service.NewReportService(
		cfg.DB,
		cfg.ReportTemplateRepo,
		cfg.ReportRunRepo,
		cfg.ReportDataRepo,
		cfg.ScheduledRepo,
		cfg.YearEndCheckRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.cashFlowService = service.NewCashFlowService(
		cfg.DB,
		cfg.CashFlowConfigRepo,
		cfg.CashFlowTemplateRepo,
		cfg.CashFlowRunRepo,
		cfg.CashFlowLineRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.eodService = service.NewEODService(
		cfg.DB,
		cfg.BusinessDateRepo,
		cfg.EODConfigRepo,
		cfg.EODRunRepo,
		cfg.EODTaskRepo,
		cfg.EODTaskRunRepo,
		cfg.HolidayRepo,
		cfg.ReconciliationRepo,
		cfg.GLAPI,
		cfg.AuditLogger,
	)

	m.router = rest.NewCloseRouter(
		cfg.Logger,
		m.periodCloseService,
		m.reportService,
		m.cashFlowService,
		m.eodService,
	)

	m.api = NewCloseAPI(
		m.periodCloseService,
		m.reportService,
		m.cashFlowService,
	)

	return m, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) PeriodCloseService() *service.PeriodCloseService {
	return m.periodCloseService
}

func (m *Module) ReportService() *service.ReportService {
	return m.reportService
}

func (m *Module) CashFlowService() *service.CashFlowService {
	return m.cashFlowService
}

func (m *Module) EODService() *service.EODService {
	return m.eodService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
