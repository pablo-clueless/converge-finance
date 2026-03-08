package rest

import (
	"converge-finance.com/m/internal/modules/ic/internal/repository"
	"converge-finance.com/m/internal/modules/ic/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ICRouter struct {
	hierarchyHandler     *HierarchyHandler
	mappingHandler       *MappingHandler
	transactionHandler   *TransactionHandler
	reconciliationHandler *ReconciliationHandler
	eliminationHandler   *EliminationHandler
}

func NewICRouter(
	logger *zap.Logger,
	hierarchyRepo repository.EntityHierarchyRepository,
	mappingRepo repository.AccountMappingRepository,
	txService *service.ICTransactionService,
	reconcService *service.ReconciliationService,
	elimService *service.EliminationService,
	auditLogger *audit.Logger,
) *ICRouter {
	baseHandler := NewHandler(logger)

	return &ICRouter{
		hierarchyHandler:      NewHierarchyHandler(baseHandler, hierarchyRepo),
		mappingHandler:        NewMappingHandler(baseHandler, mappingRepo, auditLogger),
		transactionHandler:    NewTransactionHandler(baseHandler, txService),
		reconciliationHandler: NewReconciliationHandler(baseHandler, reconcService),
		eliminationHandler:    NewEliminationHandler(baseHandler, elimService),
	}
}

func (r *ICRouter) RegisterRoutes(router chi.Router) {
	router.Route("/ic", func(rt chi.Router) {
		// Entity Hierarchy
		rt.Route("/hierarchy", func(rt chi.Router) {
			rt.Get("/", r.hierarchyHandler.GetTree)
			rt.Get("/roots", r.hierarchyHandler.GetRoots)
			rt.Get("/{id}", r.hierarchyHandler.Get)
			rt.Put("/{id}", r.hierarchyHandler.Update)
			rt.Get("/{id}/children", r.hierarchyHandler.GetChildren)
		})

		// Account Mappings
		rt.Route("/mappings", func(rt chi.Router) {
			rt.Get("/", r.mappingHandler.List)
			rt.Post("/", r.mappingHandler.Create)
			rt.Get("/{id}", r.mappingHandler.Get)
			rt.Put("/{id}", r.mappingHandler.Update)
			rt.Delete("/{id}", r.mappingHandler.Delete)
		})

		// IC Transactions
		rt.Route("/transactions", func(rt chi.Router) {
			rt.Get("/", r.transactionHandler.List)
			rt.Post("/", r.transactionHandler.Create)
			rt.Get("/{id}", r.transactionHandler.Get)
			rt.Delete("/{id}", r.transactionHandler.Delete)
			rt.Post("/{id}/submit", r.transactionHandler.Submit)
			rt.Post("/{id}/post", r.transactionHandler.Post)
			rt.Post("/{id}/reconcile", r.transactionHandler.Reconcile)
			rt.Post("/{id}/dispute", r.transactionHandler.Dispute)
			rt.Post("/{id}/resolve-dispute", r.transactionHandler.ResolveDispute)
		})

		// Reconciliation
		rt.Route("/reconciliation", func(rt chi.Router) {
			rt.Get("/status", r.reconciliationHandler.GetStatus)
			rt.Get("/discrepancies", r.reconciliationHandler.GetDiscrepancies)
			rt.Get("/entity-pair/{from}/{to}", r.reconciliationHandler.GetEntityPairReconciliation)
			rt.Post("/auto-reconcile/{from}/{to}", r.reconciliationHandler.AutoReconcile)
			rt.Post("/recalculate", r.reconciliationHandler.RecalculateBalances)
		})

		// Elimination Rules
		rt.Route("/elimination-rules", func(rt chi.Router) {
			rt.Get("/", r.eliminationHandler.ListRules)
			rt.Post("/", r.eliminationHandler.CreateRule)
			rt.Get("/{id}", r.eliminationHandler.GetRule)
			rt.Put("/{id}", r.eliminationHandler.UpdateRule)
			rt.Delete("/{id}", r.eliminationHandler.DeleteRule)
		})

		// Elimination Runs
		rt.Route("/elimination-runs", func(rt chi.Router) {
			rt.Get("/", r.eliminationHandler.ListRuns)
			rt.Post("/generate", r.eliminationHandler.Generate)
			rt.Get("/{id}", r.eliminationHandler.GetRun)
			rt.Post("/{id}/post", r.eliminationHandler.PostRun)
			rt.Post("/{id}/reverse", r.eliminationHandler.ReverseRun)
			rt.Delete("/{id}", r.eliminationHandler.DeleteRun)
		})
	})
}
