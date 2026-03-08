package rest

import (
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/modules/fa/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type FARouter struct {
	categoryHandler     *CategoryHandler
	assetHandler        *AssetHandler
	depreciationHandler *DepreciationHandler
	transferHandler     *TransferHandler
}

func NewFARouter(
	logger *zap.Logger,
	categoryRepo repository.CategoryRepository,
	assetRepo repository.AssetRepository,
	depRepo repository.DepreciationRepository,
	transferRepo repository.TransferRepository,
	assetService *service.AssetService,
	depEngine *service.DepreciationEngine,
	auditLogger *audit.Logger,
) *FARouter {
	baseHandler := NewHandler(logger)

	return &FARouter{
		categoryHandler:     NewCategoryHandler(baseHandler, categoryRepo, auditLogger),
		assetHandler:        NewAssetHandler(baseHandler, assetRepo, categoryRepo, assetService, auditLogger),
		depreciationHandler: NewDepreciationHandler(baseHandler, depRepo, depEngine, auditLogger),
		transferHandler:     NewTransferHandler(baseHandler, transferRepo, assetService, auditLogger),
	}
}

func (r *FARouter) RegisterRoutes(router chi.Router) {
	router.Route("/fa", func(rt chi.Router) {
		rt.Route("/categories", func(rt chi.Router) {
			rt.Get("/", r.categoryHandler.List)
			rt.Post("/", r.categoryHandler.Create)
			rt.Get("/{id}", r.categoryHandler.Get)
			rt.Put("/{id}", r.categoryHandler.Update)
			rt.Delete("/{id}", r.categoryHandler.Delete)
			rt.Post("/{id}/activate", r.categoryHandler.Activate)
			rt.Post("/{id}/deactivate", r.categoryHandler.Deactivate)
		})

		rt.Route("/assets", func(rt chi.Router) {
			rt.Get("/", r.assetHandler.List)
			rt.Post("/", r.assetHandler.Create)
			rt.Get("/{id}", r.assetHandler.Get)
			rt.Put("/{id}", r.assetHandler.Update)
			rt.Delete("/{id}", r.assetHandler.Delete)
			rt.Post("/{id}/activate", r.assetHandler.Activate)
			rt.Post("/{id}/suspend", r.assetHandler.Suspend)
			rt.Post("/{id}/reactivate", r.assetHandler.Reactivate)
			rt.Post("/{id}/dispose", r.assetHandler.Dispose)
			rt.Post("/{id}/write-off", r.assetHandler.WriteOff)
			rt.Post("/{id}/units", r.assetHandler.RecordUnits)
			rt.Get("/{id}/depreciation-schedule", r.assetHandler.GetDepreciationSchedule)
		})

		rt.Route("/depreciation", func(rt chi.Router) {
			rt.Post("/preview", r.depreciationHandler.Preview)
			rt.Post("/run", r.depreciationHandler.Run)
			rt.Get("/runs", r.depreciationHandler.ListRuns)
			rt.Get("/runs/{id}", r.depreciationHandler.GetRun)
			rt.Post("/runs/{id}/post", r.depreciationHandler.PostRun)
			rt.Post("/runs/{id}/reverse", r.depreciationHandler.ReverseRun)
		})

		rt.Route("/transfers", func(rt chi.Router) {
			rt.Get("/", r.transferHandler.List)
			rt.Post("/", r.transferHandler.Create)
			rt.Get("/{id}", r.transferHandler.Get)
			rt.Post("/{id}/approve", r.transferHandler.Approve)
			rt.Post("/{id}/complete", r.transferHandler.Complete)
			rt.Post("/{id}/cancel", r.transferHandler.Cancel)
		})

		rt.Route("/reports", func(rt chi.Router) {
			rt.Get("/register", r.assetHandler.AssetRegister)
			rt.Get("/book-value", r.assetHandler.BookValueReport)
		})
	})
}
