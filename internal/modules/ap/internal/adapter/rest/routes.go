package rest

import (
	"converge-finance.com/m/internal/modules/ap/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type APRouter struct {
	vendorHandler  *VendorHandler
	invoiceHandler *InvoiceHandler
	paymentHandler *PaymentHandler
	agingHandler   *AgingHandler
	jwtService     *auth.JWTService
	logger         *zap.Logger
}

func NewAPRouter(
	logger *zap.Logger,
	vendorService *service.VendorService,
	invoiceService *service.InvoiceService,
	paymentService *service.PaymentService,
	agingService *service.AgingService,
	jwtService *auth.JWTService,
) *APRouter {
	return &APRouter{
		vendorHandler:  NewVendorHandler(vendorService, logger),
		invoiceHandler: NewInvoiceHandler(invoiceService, logger),
		paymentHandler: NewPaymentHandler(paymentService, logger),
		agingHandler:   NewAgingHandler(agingService, logger),
		jwtService:     jwtService,
		logger:         logger,
	}
}

func (r *APRouter) RegisterRoutes(router chi.Router) {
	router.Route("/ap", func(ap chi.Router) {

		ap.Use(auth.AuthMiddleware(r.jwtService))

		ap.Route("/vendors", func(vendors chi.Router) {

			vendors.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("ap:vendor:read"))
				read.Get("/", r.vendorHandler.List)
				read.Get("/search", r.vendorHandler.Search)
				read.Get("/{id}", r.vendorHandler.Get)
				read.Get("/{id}/balance", r.vendorHandler.GetBalance)
			})

			vendors.Group(func(write chi.Router) {
				write.Use(auth.RequirePermission("ap:vendor:write"))
				write.Post("/", r.vendorHandler.Create)
				write.Put("/{id}", r.vendorHandler.Update)
				write.Delete("/{id}", r.vendorHandler.Delete)
			})

			vendors.Group(func(admin chi.Router) {
				admin.Use(auth.RequirePermission("ap:vendor:admin"))
				admin.Post("/{id}/activate", r.vendorHandler.Activate)
				admin.Post("/{id}/deactivate", r.vendorHandler.Deactivate)
				admin.Post("/{id}/block", r.vendorHandler.Block)
			})
		})

		ap.Route("/invoices", func(invoices chi.Router) {

			invoices.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("ap:invoice:read"))
				read.Get("/", r.invoiceHandler.List)
				read.Get("/overdue", r.invoiceHandler.GetOverdue)
				read.Get("/for-payment", r.invoiceHandler.GetForPayment)
				read.Get("/{id}", r.invoiceHandler.Get)
			})

			invoices.Group(func(write chi.Router) {
				write.Use(auth.RequirePermission("ap:invoice:write"))
				write.Post("/", r.invoiceHandler.Create)
				write.Post("/{id}/submit", r.invoiceHandler.Submit)
			})

			invoices.Group(func(approve chi.Router) {
				approve.Use(auth.RequirePermission("ap:invoice:approve"))
				approve.Post("/{id}/approve", r.invoiceHandler.Approve)
				approve.Post("/{id}/reject", r.invoiceHandler.Reject)
			})

			invoices.Group(func(void chi.Router) {
				void.Use(auth.RequirePermission("ap:invoice:void"))
				void.Post("/{id}/void", r.invoiceHandler.Void)
			})
		})

		ap.Route("/payments", func(payments chi.Router) {

			payments.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("ap:payment:read"))
				read.Get("/", r.paymentHandler.List)
				read.Get("/scheduled", r.paymentHandler.GetScheduled)
				read.Get("/summary", r.paymentHandler.GetSummary)
				read.Get("/{id}", r.paymentHandler.Get)
			})

			payments.Group(func(write chi.Router) {
				write.Use(auth.RequirePermission("ap:payment:write"))
				write.Post("/", r.paymentHandler.Create)
				write.Post("/{id}/submit", r.paymentHandler.Submit)
				write.Post("/{id}/schedule", r.paymentHandler.Schedule)
			})

			payments.Group(func(approve chi.Router) {
				approve.Use(auth.RequirePermission("ap:payment:approve"))
				approve.Post("/{id}/approve", r.paymentHandler.Approve)
				approve.Post("/{id}/reject", r.paymentHandler.Reject)
			})

			payments.Group(func(process chi.Router) {
				process.Use(auth.RequirePermission("ap:payment:process"))
				process.Post("/{id}/process", r.paymentHandler.Process)
				process.Post("/{id}/complete", r.paymentHandler.Complete)
				process.Post("/{id}/fail", r.paymentHandler.Fail)
			})

			payments.Group(func(void chi.Router) {
				void.Use(auth.RequirePermission("ap:payment:void"))
				void.Post("/{id}/void", r.paymentHandler.Void)
			})
		})

		ap.Route("/reports", func(reports chi.Router) {
			reports.Use(auth.RequirePermission("ap:reports:read"))
			reports.Get("/aging", r.agingHandler.GetAgingReport)
			reports.Get("/aging/vendor/{id}", r.agingHandler.GetVendorAging)
			reports.Get("/cash-requirements", r.agingHandler.GetCashRequirements)
			reports.Get("/overdue-alerts", r.agingHandler.GetOverdueAlerts)
		})
	})
}
