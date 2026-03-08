package rest

import (
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// WorkflowRouter handles workflow module routing
type WorkflowRouter struct {
	workflowHandler *WorkflowHandler
	requestHandler  *RequestHandler
	jwtService      *auth.JWTService
	logger          *zap.Logger
}

// NewWorkflowRouter creates a new WorkflowRouter
func NewWorkflowRouter(
	workflowHandler *WorkflowHandler,
	requestHandler *RequestHandler,
	jwtService *auth.JWTService,
	logger *zap.Logger,
) *WorkflowRouter {
	return &WorkflowRouter{
		workflowHandler: workflowHandler,
		requestHandler:  requestHandler,
		jwtService:      jwtService,
		logger:          logger,
	}
}

// RegisterRoutes registers all workflow routes
func (r *WorkflowRouter) RegisterRoutes(router chi.Router) {
	router.Route("/workflow", func(wf chi.Router) {

		// All workflow routes require authentication
		wf.Use(auth.AuthMiddleware(r.jwtService))

		// Workflow definition routes
		wf.Route("/workflows", func(workflows chi.Router) {
			// Read workflows
			workflows.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("workflow:read"))
				read.Get("/", r.workflowHandler.ListWorkflows)
				read.Get("/{id}", r.workflowHandler.GetWorkflow)
			})

			// Create workflows
			workflows.Group(func(create chi.Router) {
				create.Use(auth.RequirePermission("workflow:create"))
				create.Post("/", r.workflowHandler.CreateWorkflow)
			})

			// Manage workflows
			workflows.Group(func(manage chi.Router) {
				manage.Use(auth.RequirePermission("workflow:manage"))
				manage.Put("/{id}", r.workflowHandler.UpdateWorkflow)
				manage.Post("/{id}/activate", r.workflowHandler.ActivateWorkflow)
				manage.Post("/{id}/deactivate", r.workflowHandler.DeactivateWorkflow)
				manage.Post("/{id}/archive", r.workflowHandler.ArchiveWorkflow)
			})

			// Manage workflow steps
			workflows.Group(func(steps chi.Router) {
				steps.Use(auth.RequirePermission("workflow:manage"))
				steps.Post("/{id}/steps", r.workflowHandler.AddStep)
				steps.Delete("/{id}/steps/{stepId}", r.workflowHandler.RemoveStep)
			})
		})

		// Delegation routes
		wf.Route("/delegations", func(delegations chi.Router) {
			// Read delegations
			delegations.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("workflow:delegation:read"))
				read.Get("/", r.workflowHandler.ListDelegations)
				read.Get("/{id}", r.workflowHandler.GetDelegation)
			})

			// Create delegations (users can create their own delegations)
			delegations.Group(func(create chi.Router) {
				create.Use(auth.RequirePermission("workflow:delegation:create"))
				create.Post("/", r.workflowHandler.CreateDelegation)
			})

			// Manage delegations
			delegations.Group(func(manage chi.Router) {
				manage.Use(auth.RequirePermission("workflow:delegation:manage"))
				manage.Post("/{id}/deactivate", r.workflowHandler.DeactivateDelegation)
			})
		})

		// Approval request routes
		wf.Route("/requests", func(requests chi.Router) {
			// Submit for approval
			requests.Group(func(submit chi.Router) {
				submit.Use(auth.RequirePermission("workflow:request:submit"))
				submit.Post("/", r.requestHandler.SubmitForApproval)
			})

			// View requests
			requests.Group(func(view chi.Router) {
				view.Use(auth.RequirePermission("workflow:request:read"))
				view.Get("/", r.requestHandler.ListRequests)
				view.Get("/{id}", r.requestHandler.GetRequest)
			})

			// Approve/reject requests
			requests.Group(func(action chi.Router) {
				action.Use(auth.RequirePermission("workflow:request:approve"))
				action.Post("/{id}/approve", r.requestHandler.Approve)
				action.Post("/{id}/reject", r.requestHandler.Reject)
			})

			// Cancel requests (requestor only, checked in handler)
			requests.Group(func(cancel chi.Router) {
				cancel.Use(auth.RequirePermission("workflow:request:read"))
				cancel.Post("/{id}/cancel", r.requestHandler.Cancel)
			})
		})

		// Pending approvals for current user
		wf.Group(func(pending chi.Router) {
			pending.Use(auth.RequirePermission("workflow:request:approve"))
			pending.Get("/pending-approvals", r.requestHandler.GetPendingApprovals)
		})
	})
}
