package workflow

import (
	"converge-finance.com/m/internal/modules/workflow/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/workflow/internal/repository"
	"converge-finance.com/m/internal/modules/workflow/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// Module represents the workflow module with all its dependencies
type Module struct {
	api             API
	workflowService *service.WorkflowService
	requestService  *service.RequestService
	workflowRepo    repository.WorkflowRepository
	stepRepo        repository.WorkflowStepRepository
	delegationRepo  repository.DelegationRepository
	requestRepo     repository.RequestRepository
	actionRepo      repository.ActionRepository
	pendingRepo     repository.PendingApprovalRepository
	router          *rest.WorkflowRouter
	logger          *zap.Logger
}

// Config contains configuration for creating the workflow module
type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	JWTService  *auth.JWTService
	Logger      *zap.Logger
}

// NewModule creates a new workflow module instance
func NewModule(cfg Config) (*Module, error) {
	// Initialize repositories
	workflowRepo := repository.NewPostgresWorkflowRepo(cfg.DB)
	stepRepo := repository.NewPostgresWorkflowStepRepo(cfg.DB)
	delegationRepo := repository.NewPostgresDelegationRepo(cfg.DB)
	requestRepo := repository.NewPostgresRequestRepo(cfg.DB)
	actionRepo := repository.NewPostgresActionRepo(cfg.DB)
	pendingRepo := repository.NewPostgresPendingApprovalRepo(cfg.DB)

	// Initialize services
	workflowService := service.NewWorkflowService(
		workflowRepo,
		stepRepo,
		delegationRepo,
		cfg.AuditLogger,
		cfg.Logger,
	)

	requestService := service.NewRequestService(
		requestRepo,
		actionRepo,
		pendingRepo,
		workflowRepo,
		stepRepo,
		delegationRepo,
		cfg.AuditLogger,
		cfg.Logger,
	)

	// Initialize handlers
	workflowHandler := rest.NewWorkflowHandler(workflowService, cfg.Logger)
	requestHandler := rest.NewRequestHandler(requestService, cfg.Logger)

	// Initialize router
	router := rest.NewWorkflowRouter(workflowHandler, requestHandler, cfg.JWTService, cfg.Logger)

	// Initialize API
	api := NewWorkflowAPI(workflowService, requestService)

	return &Module{
		api:             api,
		workflowService: workflowService,
		requestService:  requestService,
		workflowRepo:    workflowRepo,
		stepRepo:        stepRepo,
		delegationRepo:  delegationRepo,
		requestRepo:     requestRepo,
		actionRepo:      actionRepo,
		pendingRepo:     pendingRepo,
		router:          router,
		logger:          cfg.Logger,
	}, nil
}

// API returns the public API for the workflow module
func (m *Module) API() API {
	return m.api
}

// WorkflowService returns the workflow service for internal use
func (m *Module) WorkflowService() *service.WorkflowService {
	return m.workflowService
}

// RequestService returns the request service for internal use
func (m *Module) RequestService() *service.RequestService {
	return m.requestService
}

// RegisterRoutes registers the workflow module routes with the given router
func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
