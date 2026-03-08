package rest

import (
	"converge-finance.com/m/internal/modules/consol/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ConsolRouter struct {
	logger               *zap.Logger
	setHandler           *ConsolidationSetHandler
	runHandler           *ConsolidationRunHandler
	translationHandler   *TranslationHandler
}

func NewConsolRouter(
	logger *zap.Logger,
	consolidationService *service.ConsolidationService,
	translationService *service.TranslationService,
	auditLogger *audit.Logger,
) *ConsolRouter {
	return &ConsolRouter{
		logger:             logger,
		setHandler:         NewConsolidationSetHandler(logger, consolidationService),
		runHandler:         NewConsolidationRunHandler(logger, consolidationService),
		translationHandler: NewTranslationHandler(logger, translationService),
	}
}

func (cr *ConsolRouter) RegisterRoutes(r chi.Router) {
	r.Route("/consol", func(r chi.Router) {
		// Consolidation Sets
		r.Route("/sets", func(r chi.Router) {
			r.Get("/", cr.setHandler.ListSets)
			r.Post("/", cr.setHandler.CreateSet)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.setHandler.GetSet)
				r.Post("/members", cr.setHandler.AddMember)
			})
		})

		// Consolidation Runs
		r.Route("/runs", func(r chi.Router) {
			r.Get("/", cr.runHandler.ListRuns)
			r.Post("/", cr.runHandler.InitiateRun)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.runHandler.GetRun)
				r.Post("/execute", cr.runHandler.ExecuteRun)
				r.Post("/post", cr.runHandler.PostRun)
				r.Post("/reverse", cr.runHandler.ReverseRun)
				r.Get("/trial-balance", cr.runHandler.GetTrialBalance)
			})
		})

		// Exchange Rates
		r.Route("/rates", func(r chi.Router) {
			r.Get("/", cr.translationHandler.ListRates)
			r.Post("/", cr.translationHandler.CreateRate)
			r.Get("/lookup", cr.translationHandler.GetRate)
			r.Post("/translate", cr.translationHandler.TranslateAmount)

			r.Route("/{id}", func(r chi.Router) {
				r.Put("/", cr.translationHandler.UpdateRate)
				r.Delete("/", cr.translationHandler.DeleteRate)
			})
		})
	})
}
