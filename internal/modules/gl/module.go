package gl

import (
	"converge-finance.com/m/internal/modules/gl/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"converge-finance.com/m/internal/modules/gl/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api           API
	postingEngine *service.PostingEngine
	accountRepo   repository.AccountRepository
	journalRepo   repository.JournalRepository
	periodRepo    repository.PeriodRepository
	balanceRepo   repository.AccountBalanceRepository
	router        *rest.GLRouter
	logger        *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	Logger      *zap.Logger

	AccountRepo repository.AccountRepository
	JournalRepo repository.JournalRepository
	PeriodRepo  repository.PeriodRepository
	BalanceRepo repository.AccountBalanceRepository
}

func NewModule(cfg Config) (*Module, error) {
	m := &Module{
		logger: cfg.Logger,
	}

	// Auto-create repositories if not provided
	if cfg.AccountRepo != nil {
		m.accountRepo = cfg.AccountRepo
	} else if cfg.DB != nil {
		m.accountRepo = repository.NewPostgresAccountRepository(cfg.DB.DB)
	}

	if cfg.JournalRepo != nil {
		m.journalRepo = cfg.JournalRepo
	} else if cfg.DB != nil {
		m.journalRepo = repository.NewPostgresJournalRepository(cfg.DB.DB)
	}

	if cfg.PeriodRepo != nil {
		m.periodRepo = cfg.PeriodRepo
	} else if cfg.DB != nil {
		m.periodRepo = repository.NewPostgresPeriodRepository(cfg.DB.DB)
	}

	if cfg.BalanceRepo != nil {
		m.balanceRepo = cfg.BalanceRepo
	} else if cfg.DB != nil {
		m.balanceRepo = repository.NewPostgresBalanceRepository(cfg.DB.DB)
	}

	m.postingEngine = service.NewPostingEngine(
		cfg.DB,
		m.journalRepo,
		m.accountRepo,
		m.periodRepo,
		m.balanceRepo,
		cfg.AuditLogger,
	)

	m.router = rest.NewGLRouter(
		cfg.Logger,
		m.accountRepo,
		m.journalRepo,
		m.periodRepo,
		m.postingEngine,
		cfg.AuditLogger,
	)

	m.api = NewGLAPI(
		m.postingEngine,
		m.journalRepo,
		m.periodRepo,
		m.accountRepo,
		m.balanceRepo,
	)

	return m, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) PostingEngine() *service.PostingEngine {
	return m.postingEngine
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
