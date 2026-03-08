package rest

import (
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type SegmentRouter struct {
	segmentHandler *SegmentHandler
	reportHandler  *ReportHandler
	jwtService     *auth.JWTService
	logger         *zap.Logger
}

func NewSegmentRouter(
	segmentHandler *SegmentHandler,
	reportHandler *ReportHandler,
	jwtService *auth.JWTService,
	logger *zap.Logger,
) *SegmentRouter {
	return &SegmentRouter{
		segmentHandler: segmentHandler,
		reportHandler:  reportHandler,
		jwtService:     jwtService,
		logger:         logger,
	}
}

func (r *SegmentRouter) RegisterRoutes(router chi.Router) {
	router.Route("/segments", func(seg chi.Router) {

		seg.Use(auth.AuthMiddleware(r.jwtService))

		seg.Group(func(view chi.Router) {
			view.Use(auth.RequirePermission("segment:view"))
			view.Get("/", r.segmentHandler.ListSegments)
			view.Get("/tree", r.segmentHandler.GetSegmentTree)
			view.Get("/{id}", r.segmentHandler.GetSegment)
		})

		seg.Group(func(manage chi.Router) {
			manage.Use(auth.RequirePermission("segment:manage"))
			manage.Post("/", r.segmentHandler.CreateSegment)
			manage.Put("/{id}", r.segmentHandler.UpdateSegment)
			manage.Delete("/{id}", r.segmentHandler.DeleteSegment)
			manage.Post("/{id}/activate", r.segmentHandler.ActivateSegment)
			manage.Post("/{id}/deactivate", r.segmentHandler.DeactivateSegment)
		})

		seg.Route("/hierarchies", func(hier chi.Router) {
			hier.Group(func(view chi.Router) {
				view.Use(auth.RequirePermission("segment:view"))
				view.Get("/", r.segmentHandler.ListHierarchies)
				view.Get("/{id}", r.segmentHandler.GetHierarchy)
			})

			hier.Group(func(manage chi.Router) {
				manage.Use(auth.RequirePermission("segment:manage"))
				manage.Post("/", r.segmentHandler.CreateHierarchy)
			})
		})

		seg.Route("/assignments", func(assign chi.Router) {
			assign.Group(func(view chi.Router) {
				view.Use(auth.RequirePermission("segment:assignment:view"))
				view.Get("/", r.segmentHandler.ListAssignments)
				view.Get("/{id}", r.segmentHandler.GetAssignment)
			})

			assign.Group(func(manage chi.Router) {
				manage.Use(auth.RequirePermission("segment:assignment:manage"))
				manage.Post("/", r.segmentHandler.AssignToSegment)
				manage.Delete("/{id}", r.segmentHandler.DeleteAssignment)
			})
		})

		seg.Route("/balances", func(bal chi.Router) {
			bal.Group(func(view chi.Router) {
				view.Use(auth.RequirePermission("segment:balance:view"))
				view.Get("/summary", r.segmentHandler.GetBalanceSummary)
			})
		})

		seg.Route("/intersegment", func(inter chi.Router) {
			inter.Group(func(view chi.Router) {
				view.Use(auth.RequirePermission("segment:intersegment:view"))
				inter.Get("/", r.segmentHandler.ListIntersegmentTransactions)
			})

			inter.Group(func(manage chi.Router) {
				manage.Use(auth.RequirePermission("segment:intersegment:manage"))
				inter.Post("/", r.segmentHandler.CreateIntersegmentTransaction)
				inter.Post("/{id}/eliminate", r.segmentHandler.EliminateIntersegmentTransaction)
			})
		})

		seg.Route("/reports", func(rpt chi.Router) {
			rpt.Group(func(view chi.Router) {
				view.Use(auth.RequirePermission("segment:report:view"))
				view.Get("/", r.reportHandler.ListReports)
				view.Get("/{id}", r.reportHandler.GetReport)
				view.Get("/{id}/summary", r.reportHandler.GetReportSummary)
			})

			rpt.Group(func(create chi.Router) {
				create.Use(auth.RequirePermission("segment:report:create"))
				create.Post("/", r.reportHandler.GenerateReport)
				create.Post("/{id}/regenerate", r.reportHandler.RegenerateReportData)
			})

			rpt.Group(func(workflow chi.Router) {
				workflow.Use(auth.RequirePermission("segment:report:finalize"))
				workflow.Post("/{id}/finalize", r.reportHandler.FinalizeReport)
			})

			rpt.Group(func(workflow chi.Router) {
				workflow.Use(auth.RequirePermission("segment:report:approve"))
				workflow.Post("/{id}/approve", r.reportHandler.ApproveReport)
			})

			rpt.Group(func(workflow chi.Router) {
				workflow.Use(auth.RequirePermission("segment:report:publish"))
				workflow.Post("/{id}/publish", r.reportHandler.PublishReport)
			})

			rpt.Group(func(manage chi.Router) {
				manage.Use(auth.RequirePermission("segment:report:delete"))
				manage.Delete("/{id}", r.reportHandler.DeleteReport)
			})
		})
	})
}
