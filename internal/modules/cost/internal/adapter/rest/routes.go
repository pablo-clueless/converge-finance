package rest

import (
	"converge-finance.com/m/internal/modules/cost/internal/service"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type CostRouter struct {
	logger            *zap.Logger
	costCenterHandler *CostCenterHandler
	allocationHandler *AllocationHandler
	budgetHandler     *BudgetHandler
}

func NewCostRouter(
	logger *zap.Logger,
	costCenterService *service.CostCenterService,
	allocationService *service.AllocationService,
	budgetService *service.BudgetService,
) *CostRouter {
	return &CostRouter{
		logger:            logger,
		costCenterHandler: NewCostCenterHandler(logger, costCenterService),
		allocationHandler: NewAllocationHandler(logger, allocationService),
		budgetHandler:     NewBudgetHandler(logger, budgetService),
	}
}

func (cr *CostRouter) RegisterRoutes(r chi.Router) {
	r.Route("/cost", func(r chi.Router) {
		// Cost Centers
		r.Route("/centers", func(r chi.Router) {
			r.Get("/", cr.costCenterHandler.List)
			r.Post("/", cr.costCenterHandler.Create)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.costCenterHandler.Get)
				r.Delete("/", cr.costCenterHandler.Deactivate)
			})
		})

		// Allocation Rules
		r.Route("/allocation-rules", func(r chi.Router) {
			r.Get("/", cr.allocationHandler.ListRules)
			r.Post("/", cr.allocationHandler.CreateRule)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.allocationHandler.GetRule)
				r.Post("/targets", cr.allocationHandler.AddTarget)
			})
		})

		// Allocation Runs
		r.Route("/allocation-runs", func(r chi.Router) {
			r.Get("/", cr.allocationHandler.ListRuns)
			r.Post("/", cr.allocationHandler.InitiateRun)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.allocationHandler.GetRun)
				r.Post("/execute", cr.allocationHandler.ExecuteRun)
				r.Post("/post", cr.allocationHandler.PostRun)
			})
		})

		// Budgets
		r.Route("/budgets", func(r chi.Router) {
			r.Get("/", cr.budgetHandler.List)
			r.Post("/", cr.budgetHandler.Create)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.budgetHandler.Get)
				r.Post("/lines", cr.budgetHandler.AddLine)
				r.Post("/submit", cr.budgetHandler.Submit)
				r.Post("/approve", cr.budgetHandler.Approve)
				r.Post("/reject", cr.budgetHandler.Reject)
				r.Get("/variance", cr.budgetHandler.GetVariance)
			})
		})
	})
}
