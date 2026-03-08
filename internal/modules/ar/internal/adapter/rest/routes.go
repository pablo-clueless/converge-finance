package rest

import (
	"converge-finance.com/m/internal/modules/ar/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ARRouter struct {
	customerHandler *CustomerHandler
	invoiceHandler  *InvoiceHandler
	receiptHandler  *ReceiptHandler
	reportsHandler  *ReportsHandler
	jwtService      *auth.JWTService
	logger          *zap.Logger
}

func NewARRouter(
	logger *zap.Logger,
	customerService *service.CustomerService,
	invoiceService *service.InvoiceService,
	receiptService *service.ReceiptService,
	dunningService *service.DunningService,
	jwtService *auth.JWTService,
) *ARRouter {
	return &ARRouter{
		customerHandler: NewCustomerHandler(customerService, logger),
		invoiceHandler:  NewInvoiceHandler(invoiceService, logger),
		receiptHandler:  NewReceiptHandler(receiptService, logger),
		reportsHandler:  NewReportsHandler(dunningService, logger),
		jwtService:      jwtService,
		logger:          logger,
	}
}

func (r *ARRouter) RegisterRoutes(router chi.Router) {
	router.Route("/ar", func(ar chi.Router) {

		ar.Use(auth.AuthMiddleware(r.jwtService))

		ar.Route("/customers", func(customers chi.Router) {

			customers.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("ar:customer:read"))
				read.Get("/", r.customerHandler.List)
				read.Get("/search", r.customerHandler.Search)
				read.Get("/{id}", r.customerHandler.Get)
				read.Get("/{id}/balance", r.customerHandler.GetBalance)
			})

			customers.Group(func(write chi.Router) {
				write.Use(auth.RequirePermission("ar:customer:write"))
				write.Post("/", r.customerHandler.Create)
				write.Put("/{id}", r.customerHandler.Update)
				write.Delete("/{id}", r.customerHandler.Delete)
			})

			customers.Group(func(admin chi.Router) {
				admin.Use(auth.RequirePermission("ar:customer:admin"))
				admin.Post("/{id}/activate", r.customerHandler.Activate)
				admin.Post("/{id}/deactivate", r.customerHandler.Deactivate)
				admin.Post("/{id}/suspend", r.customerHandler.Suspend)
				admin.Post("/{id}/release-hold", r.customerHandler.ReleaseCreditHold)
			})
		})

		ar.Route("/invoices", func(invoices chi.Router) {

			invoices.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("ar:invoice:read"))
				read.Get("/", r.invoiceHandler.List)
				read.Get("/overdue", r.invoiceHandler.GetOverdue)
				read.Get("/{id}", r.invoiceHandler.Get)
			})

			invoices.Group(func(write chi.Router) {
				write.Use(auth.RequirePermission("ar:invoice:write"))
				write.Post("/", r.invoiceHandler.Create)
				write.Post("/{id}/submit", r.invoiceHandler.Submit)
			})

			invoices.Group(func(approve chi.Router) {
				approve.Use(auth.RequirePermission("ar:invoice:approve"))
				approve.Post("/{id}/approve", r.invoiceHandler.Approve)
				approve.Post("/{id}/reject", r.invoiceHandler.Reject)
			})

			invoices.Group(func(send chi.Router) {
				send.Use(auth.RequirePermission("ar:invoice:send"))
				send.Post("/{id}/send", r.invoiceHandler.Send)
			})

			invoices.Group(func(void chi.Router) {
				void.Use(auth.RequirePermission("ar:invoice:void"))
				void.Post("/{id}/void", r.invoiceHandler.Void)
				void.Post("/{id}/write-off", r.invoiceHandler.WriteOff)
			})

			invoices.Group(func(dunning chi.Router) {
				dunning.Use(auth.RequirePermission("ar:dunning:write"))
				dunning.Post("/{id}/escalate-dunning", r.reportsHandler.EscalateDunning)
			})
		})

		ar.Route("/receipts", func(receipts chi.Router) {

			receipts.Group(func(read chi.Router) {
				read.Use(auth.RequirePermission("ar:receipt:read"))
				read.Get("/", r.receiptHandler.List)
				read.Get("/unapplied", r.receiptHandler.GetUnapplied)
				read.Get("/summary", r.receiptHandler.GetSummary)
				read.Get("/{id}", r.receiptHandler.Get)
			})

			receipts.Group(func(write chi.Router) {
				write.Use(auth.RequirePermission("ar:receipt:write"))
				write.Post("/", r.receiptHandler.Create)
			})

			receipts.Group(func(confirm chi.Router) {
				confirm.Use(auth.RequirePermission("ar:receipt:confirm"))
				confirm.Post("/{id}/confirm", r.receiptHandler.Confirm)
			})

			receipts.Group(func(reverse chi.Router) {
				reverse.Use(auth.RequirePermission("ar:receipt:reverse"))
				reverse.Post("/{id}/reverse", r.receiptHandler.Reverse)
			})

			receipts.Group(func(void chi.Router) {
				void.Use(auth.RequirePermission("ar:receipt:void"))
				void.Post("/{id}/void", r.receiptHandler.Void)
			})
		})

		ar.Route("/reports", func(reports chi.Router) {
			reports.Use(auth.RequirePermission("ar:reports:read"))
			reports.Get("/aging", r.reportsHandler.GetAgingReport)
			reports.Get("/aging/customer/{id}", r.reportsHandler.GetCustomerAging)
			reports.Get("/cash-forecast", r.reportsHandler.GetCashForecast)
			reports.Get("/overdue-alerts", r.reportsHandler.GetOverdueAlerts)
			reports.Get("/customer-statistics/{id}", r.reportsHandler.GetCustomerStatistics)
			reports.Get("/credit-review", r.reportsHandler.GetCustomersForCreditReview)
		})
	})
}
