package rest

import (
	"converge-finance.com/m/internal/modules/gl/internal/repository"
	"converge-finance.com/m/internal/modules/gl/internal/service"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type GLRouter struct {
	accountHandler *AccountHandler
	journalHandler *JournalHandler
	periodHandler  *PeriodHandler
	reportHandler  *ReportHandler
}

func NewGLRouter(
	logger *zap.Logger,
	accountRepo repository.AccountRepository,
	journalRepo repository.JournalRepository,
	periodRepo repository.PeriodRepository,
	postingEngine *service.PostingEngine,
	auditLogger *audit.Logger,
) *GLRouter {
	return &GLRouter{
		accountHandler: NewAccountHandler(logger, accountRepo, auditLogger),
		journalHandler: NewJournalHandler(logger, journalRepo, periodRepo, postingEngine, auditLogger),
		periodHandler:  NewPeriodHandler(logger, periodRepo, auditLogger),
		reportHandler:  NewReportHandler(logger, accountRepo, journalRepo, periodRepo),
	}
}

func (gr *GLRouter) RegisterRoutes(r chi.Router) {
	r.Route("/gl", func(r chi.Router) {

		r.Route("/accounts", func(r chi.Router) {

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLAccountRead))
				r.Get("/", gr.accountHandler.List)
				r.Get("/tree", gr.accountHandler.GetTree)
				r.Get("/{id}", gr.accountHandler.Get)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLAccountCreate))
				r.Post("/", gr.accountHandler.Create)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLAccountUpdate))
				r.Put("/{id}", gr.accountHandler.Update)
				r.Post("/{id}/activate", gr.accountHandler.Activate)
				r.Post("/{id}/deactivate", gr.accountHandler.Deactivate)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLAccountDelete))
				r.Delete("/{id}", gr.accountHandler.Delete)
			})
		})

		r.Route("/journals", func(r chi.Router) {

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLJournalRead))
				r.Get("/", gr.journalHandler.List)
				r.Get("/{id}", gr.journalHandler.Get)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLJournalCreate))
				r.Post("/", gr.journalHandler.Create)
				r.Put("/{id}", gr.journalHandler.Update)
				r.Delete("/{id}", gr.journalHandler.Delete)
				r.Post("/{id}/submit", gr.journalHandler.Submit)
				r.Post("/{id}/lines", gr.journalHandler.AddLine)
				r.Delete("/{id}/lines/{lineNumber}", gr.journalHandler.RemoveLine)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLJournalPost))
				r.Post("/{id}/post", gr.journalHandler.Post)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLJournalReverse))
				r.Post("/{id}/reverse", gr.journalHandler.Reverse)
			})
		})

		r.Route("/fiscal-years", func(r chi.Router) {

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLJournalRead))
				r.Get("/", gr.periodHandler.ListFiscalYears)
				r.Get("/current", gr.periodHandler.GetCurrentFiscalYear)
				r.Get("/{id}", gr.periodHandler.GetFiscalYear)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLPeriodManage))
				r.Post("/", gr.periodHandler.CreateFiscalYear)
				r.Post("/{id}/close", gr.periodHandler.CloseFiscalYear)
			})
		})

		r.Route("/periods", func(r chi.Router) {

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLJournalRead))
				r.Get("/", gr.periodHandler.ListPeriods)
				r.Get("/open", gr.periodHandler.ListOpenPeriods)
				r.Get("/for-date", gr.periodHandler.GetPeriodForDate)
				r.Get("/{id}", gr.periodHandler.GetPeriod)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequirePermission(auth.PermGLPeriodManage))
				r.Post("/{id}/open", gr.periodHandler.OpenPeriod)
				r.Post("/{id}/close", gr.periodHandler.ClosePeriod)
				r.Post("/{id}/reopen", gr.periodHandler.ReopenPeriod)
			})
		})

		r.Route("/reports", func(r chi.Router) {
			r.Use(auth.RequirePermission(auth.PermGLReportView))
			r.Get("/trial-balance", gr.reportHandler.TrialBalance)
			r.Get("/account-activity", gr.reportHandler.AccountActivity)
			r.Get("/balance-sheet", gr.reportHandler.BalanceSheet)
			r.Get("/income-statement", gr.reportHandler.IncomeStatement)
		})
	})
}
