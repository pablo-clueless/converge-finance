package docs

import (
	"converge-finance.com/m/internal/modules/docs/internal/adapter/rest"
	"converge-finance.com/m/internal/modules/docs/internal/repository"
	"converge-finance.com/m/internal/modules/docs/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Module struct {
	api             API
	documentService *service.DocumentService
	documentRepo    repository.DocumentRepository
	versionRepo     repository.DocumentVersionRepository
	attachmentRepo  repository.AttachmentRepository
	storageRepo     repository.StorageConfigRepository
	retentionRepo   repository.RetentionPolicyRepository
	router          *rest.DocsRouter
	logger          *zap.Logger
}

type Config struct {
	DB          *database.PostgresDB
	AuditLogger *audit.Logger
	JWTService  *auth.JWTService
	Logger      *zap.Logger
}

func NewModule(cfg Config) (*Module, error) {

	documentRepo := repository.NewPostgresDocumentRepo(cfg.DB)
	versionRepo := repository.NewPostgresDocumentVersionRepo(cfg.DB)
	attachmentRepo := repository.NewPostgresAttachmentRepo(cfg.DB)
	storageRepo := repository.NewPostgresStorageConfigRepo(cfg.DB)
	retentionRepo := repository.NewPostgresRetentionPolicyRepo(cfg.DB)

	documentService := service.NewDocumentService(
		documentRepo,
		versionRepo,
		attachmentRepo,
		storageRepo,
		retentionRepo,
		cfg.AuditLogger,
		cfg.Logger,
	)

	documentHandler := rest.NewDocumentHandler(documentService, cfg.Logger)

	router := rest.NewDocsRouter(documentHandler, cfg.JWTService, cfg.Logger)

	api := NewDocsAPI(documentService)

	return &Module{
		api:             api,
		documentService: documentService,
		documentRepo:    documentRepo,
		versionRepo:     versionRepo,
		attachmentRepo:  attachmentRepo,
		storageRepo:     storageRepo,
		retentionRepo:   retentionRepo,
		router:          router,
		logger:          cfg.Logger,
	}, nil
}

func (m *Module) API() API {
	return m.api
}

func (m *Module) DocumentService() *service.DocumentService {
	return m.documentService
}

func (m *Module) RegisterRoutes(r chi.Router) {
	m.router.RegisterRoutes(r)
}
