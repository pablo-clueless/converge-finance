package rest

import (
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type FXRouter struct {
	triangulationHandler *TriangulationHandler
	revaluationHandler   *RevaluationHandler
	jwtService           *auth.JWTService
	logger               *zap.Logger
}

func NewFXRouter(
	triangulationHandler *TriangulationHandler,
	revaluationHandler *RevaluationHandler,
	jwtService *auth.JWTService,
	logger *zap.Logger,
) *FXRouter {
	return &FXRouter{
		triangulationHandler: triangulationHandler,
		revaluationHandler:   revaluationHandler,
		jwtService:           jwtService,
		logger:               logger,
	}
}

func (r *FXRouter) RegisterRoutes(router chi.Router) {
	router.Route("/fx", func(fx chi.Router) {

		fx.Use(auth.AuthMiddleware(r.jwtService))

		fx.Group(func(convert chi.Router) {
			convert.Use(auth.RequirePermission("fx:convert"))
			convert.Post("/convert", r.triangulationHandler.Convert)
			convert.Get("/path", r.triangulationHandler.GetPath)
		})

		fx.Group(func(config chi.Router) {
			config.Use(auth.RequirePermission("fx:config:read"))
			config.Get("/config", r.triangulationHandler.GetConfig)
		})

		fx.Group(func(config chi.Router) {
			config.Use(auth.RequirePermission("fx:config:manage"))
			config.Put("/config", r.triangulationHandler.UpdateConfig)
		})

		fx.Group(func(pairs chi.Router) {
			pairs.Use(auth.RequirePermission("fx:config:read"))
			pairs.Get("/pairs", r.triangulationHandler.ListPairConfigs)
		})

		fx.Group(func(pairs chi.Router) {
			pairs.Use(auth.RequirePermission("fx:config:manage"))
			pairs.Post("/pairs", r.triangulationHandler.CreatePairConfig)
		})

		fx.Group(func(log chi.Router) {
			log.Use(auth.RequirePermission("fx:log:read"))
			log.Get("/log", r.triangulationHandler.ListConversionLogs)
			log.Get("/log/{id}", r.triangulationHandler.GetConversionLog)
		})

		fx.Route("/revaluation", func(reval chi.Router) {

			reval.Group(func(accounts chi.Router) {
				accounts.Use(auth.RequirePermission("fx:revaluation:config:read"))
				accounts.Get("/accounts", r.revaluationHandler.ListAccountFXConfigs)
			})
			reval.Group(func(accounts chi.Router) {
				accounts.Use(auth.RequirePermission("fx:revaluation:config:manage"))
				accounts.Post("/accounts", r.revaluationHandler.ConfigureAccountFX)
			})

			reval.Group(func(runs chi.Router) {
				runs.Use(auth.RequirePermission("fx:revaluation:view"))
				runs.Get("/runs", r.revaluationHandler.ListRevaluationRuns)
				runs.Get("/runs/{id}", r.revaluationHandler.GetRevaluationRun)
			})

			reval.Group(func(runs chi.Router) {
				runs.Use(auth.RequirePermission("fx:revaluation:create"))
				runs.Post("/runs", r.revaluationHandler.CreateRevaluationRun)
				runs.Post("/runs/{id}/submit", r.revaluationHandler.SubmitForApproval)
			})

			reval.Group(func(runs chi.Router) {
				runs.Use(auth.RequirePermission("fx:revaluation:approve"))
				runs.Post("/runs/{id}/approve", r.revaluationHandler.ApproveRevaluation)
			})

			reval.Group(func(runs chi.Router) {
				runs.Use(auth.RequirePermission("fx:revaluation:post"))
				runs.Post("/runs/{id}/post", r.revaluationHandler.PostRevaluation)
				runs.Post("/runs/{id}/reverse", r.revaluationHandler.ReverseRevaluation)
			})
		})
	})
}
