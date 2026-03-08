package rest

import (
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ExportRouter struct {
	handler    *ExportHandler
	jwtService *auth.JWTService
	logger     *zap.Logger
}

func NewExportRouter(
	handler *ExportHandler,
	jwtService *auth.JWTService,
	logger *zap.Logger,
) *ExportRouter {
	return &ExportRouter{
		handler:    handler,
		jwtService: jwtService,
		logger:     logger,
	}
}

func (r *ExportRouter) RegisterRoutes(router chi.Router) {
	router.Route("/export", func(export chi.Router) {

		export.Use(auth.AuthMiddleware(r.jwtService))

		// Job endpoints
		export.Group(func(jobs chi.Router) {
			jobs.Use(auth.RequirePermission("export:job:create"))
			jobs.Post("/jobs", r.handler.RequestExport)
		})

		export.Group(func(jobs chi.Router) {
			jobs.Use(auth.RequirePermission("export:job:read"))
			jobs.Get("/jobs", r.handler.ListJobs)
			jobs.Get("/jobs/{id}", r.handler.GetJob)
			jobs.Get("/jobs/{id}/download", r.handler.DownloadJob)
		})

		// Template endpoints
		export.Group(func(templates chi.Router) {
			templates.Use(auth.RequirePermission("export:template:read"))
			templates.Get("/templates", r.handler.ListTemplates)
			templates.Get("/templates/{id}", r.handler.GetTemplate)
		})

		export.Group(func(templates chi.Router) {
			templates.Use(auth.RequirePermission("export:template:manage"))
			templates.Post("/templates", r.handler.CreateTemplate)
			templates.Put("/templates/{id}", r.handler.UpdateTemplate)
			templates.Delete("/templates/{id}", r.handler.DeleteTemplate)
		})

		// Schedule endpoints
		export.Group(func(schedules chi.Router) {
			schedules.Use(auth.RequirePermission("export:schedule:read"))
			schedules.Get("/schedules", r.handler.ListSchedules)
			schedules.Get("/schedules/{id}", r.handler.GetSchedule)
		})

		export.Group(func(schedules chi.Router) {
			schedules.Use(auth.RequirePermission("export:schedule:manage"))
			schedules.Post("/schedules", r.handler.CreateSchedule)
			schedules.Put("/schedules/{id}", r.handler.UpdateSchedule)
			schedules.Delete("/schedules/{id}", r.handler.DeleteSchedule)
		})
	})
}
