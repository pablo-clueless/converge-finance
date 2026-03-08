package rest

import (
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type DocsRouter struct {
	documentHandler *DocumentHandler
	jwtService      *auth.JWTService
	logger          *zap.Logger
}

func NewDocsRouter(
	documentHandler *DocumentHandler,
	jwtService *auth.JWTService,
	logger *zap.Logger,
) *DocsRouter {
	return &DocsRouter{
		documentHandler: documentHandler,
		jwtService:      jwtService,
		logger:          logger,
	}
}

func (r *DocsRouter) RegisterRoutes(router chi.Router) {
	router.Route("/docs", func(docs chi.Router) {

		docs.Use(auth.AuthMiddleware(r.jwtService))

		docs.Group(func(upload chi.Router) {
			upload.Use(auth.RequirePermission("docs:upload"))
			upload.Post("/upload", r.documentHandler.Upload)
		})

		docs.Group(func(read chi.Router) {
			read.Use(auth.RequirePermission("docs:read"))
			read.Get("/", r.documentHandler.ListDocuments)
			read.Get("/{id}", r.documentHandler.GetDocument)
			read.Get("/{id}/download", r.documentHandler.Download)
			read.Get("/{id}/versions", r.documentHandler.GetVersions)
			read.Get("/{id}/versions/{versionId}/download", r.documentHandler.DownloadVersion)
		})

		docs.Group(func(write chi.Router) {
			write.Use(auth.RequirePermission("docs:write"))
			write.Put("/{id}", r.documentHandler.UpdateDocument)
			write.Post("/{id}/versions", r.documentHandler.CreateVersion)
		})

		docs.Group(func(attach chi.Router) {
			attach.Use(auth.RequirePermission("docs:attach"))
			attach.Post("/{id}/attach", r.documentHandler.Attach)
			attach.Delete("/{id}/attach", r.documentHandler.Detach)
		})

		docs.Group(func(archive chi.Router) {
			archive.Use(auth.RequirePermission("docs:archive"))
			archive.Post("/{id}/archive", r.documentHandler.ArchiveDocument)
		})

		docs.Group(func(del chi.Router) {
			del.Use(auth.RequirePermission("docs:delete"))
			del.Delete("/{id}", r.documentHandler.DeleteDocument)
		})

		docs.Group(func(legalHold chi.Router) {
			legalHold.Use(auth.RequirePermission("docs:legal_hold"))
			legalHold.Post("/{id}/legal-hold", r.documentHandler.SetLegalHold)
		})

		docs.Route("/attachments", func(attachments chi.Router) {
			attachments.Use(auth.RequirePermission("docs:read"))
			attachments.Get("/{refType}/{refID}", r.documentHandler.GetAttachments)
		})

		docs.Route("/config", func(config chi.Router) {
			config.Use(auth.RequirePermission("docs:config:read"))
			config.Get("/retention-policies", r.documentHandler.GetRetentionPolicies)
			config.Get("/storage", r.documentHandler.GetStorageConfigs)
		})
	})
}
