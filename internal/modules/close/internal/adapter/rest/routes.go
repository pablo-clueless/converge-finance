package rest

import (
	"converge-finance.com/m/internal/modules/close/internal/service"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type CloseRouter struct {
	logger             *zap.Logger
	periodCloseHandler *PeriodCloseHandler
	reportHandler      *ReportHandler
	cashFlowHandler    *CashFlowHandler
	eodHandler         *EODHandler
}

func NewCloseRouter(
	logger *zap.Logger,
	periodCloseService *service.PeriodCloseService,
	reportService *service.ReportService,
	cashFlowService *service.CashFlowService,
	eodService *service.EODService,
) *CloseRouter {
	return &CloseRouter{
		logger:             logger,
		periodCloseHandler: NewPeriodCloseHandler(logger, periodCloseService),
		reportHandler:      NewReportHandler(logger, reportService),
		cashFlowHandler:    NewCashFlowHandler(logger, cashFlowService),
		eodHandler:         NewEODHandler(logger, eodService),
	}
}

func (cr *CloseRouter) RegisterRoutes(r chi.Router) {
	r.Route("/close", func(r chi.Router) {
		// Period Close Status
		r.Route("/periods", func(r chi.Router) {
			r.Get("/", cr.periodCloseHandler.ListStatuses)
			r.Get("/status", cr.periodCloseHandler.GetStatus)
			r.Post("/soft-close", cr.periodCloseHandler.SoftClose)
			r.Post("/hard-close", cr.periodCloseHandler.HardClose)
			r.Post("/reopen", cr.periodCloseHandler.Reopen)
		})

		// Close Rules
		r.Route("/rules", func(r chi.Router) {
			r.Get("/", cr.periodCloseHandler.ListRules)
			r.Post("/", cr.periodCloseHandler.CreateRule)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.periodCloseHandler.GetRule)
			})
		})

		// Close Runs
		r.Route("/runs", func(r chi.Router) {
			r.Get("/", cr.periodCloseHandler.ListRuns)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.periodCloseHandler.GetRun)
				r.Post("/reverse", cr.periodCloseHandler.ReverseRun)
			})
		})

		// Report Templates
		r.Route("/templates", func(r chi.Router) {
			r.Get("/", cr.reportHandler.ListTemplates)
			r.Post("/", cr.reportHandler.CreateTemplate)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.reportHandler.GetTemplate)
			})
		})

		// Reports
		r.Route("/reports", func(r chi.Router) {
			r.Get("/", cr.reportHandler.ListReportRuns)
			r.Post("/trial-balance", cr.reportHandler.GenerateTrialBalance)
			r.Post("/income-statement", cr.reportHandler.GenerateIncomeStatement)
			r.Post("/balance-sheet", cr.reportHandler.GenerateBalanceSheet)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", cr.reportHandler.GetReportRun)
				r.Delete("/", cr.reportHandler.DeleteReportRun)
			})
		})

		// Year-End Checklist
		r.Route("/year-end-checklist", func(r chi.Router) {
			r.Get("/", cr.reportHandler.GetYearEndChecklist)
			r.Put("/items/{itemId}", cr.reportHandler.UpdateChecklistItem)
		})

		// Cash Flow Statement
		r.Route("/cashflow", func(r chi.Router) {
			// Account Configuration
			r.Route("/config", func(r chi.Router) {
				r.Get("/", cr.cashFlowHandler.ListAccountCashFlowConfigs)
				r.Post("/", cr.cashFlowHandler.ConfigureAccountCashFlow)
				r.Delete("/{id}", cr.cashFlowHandler.DeleteAccountCashFlowConfig)
			})

			// Templates
			r.Route("/templates", func(r chi.Router) {
				r.Get("/", cr.cashFlowHandler.ListTemplates)
				r.Post("/", cr.cashFlowHandler.CreateTemplate)
				r.Get("/{id}", cr.cashFlowHandler.GetTemplate)
			})

			// Cash Flow Runs
			r.Route("/runs", func(r chi.Router) {
				r.Get("/", cr.cashFlowHandler.ListCashFlowRuns)
				r.Post("/generate", cr.cashFlowHandler.GenerateCashFlowStatement)
				r.Get("/{id}", cr.cashFlowHandler.GetCashFlowRun)
			})
		})

		// End of Day (EOD)
		r.Route("/eod", func(r chi.Router) {
			// Business Date
			r.Route("/business-date", func(r chi.Router) {
				r.Get("/", cr.eodHandler.GetBusinessDate)
				r.Post("/initialize", cr.eodHandler.InitializeBusinessDate)
				r.Post("/rollover", cr.eodHandler.RolloverBusinessDate)
			})

			// EOD Configuration
			r.Route("/config", func(r chi.Router) {
				r.Get("/", cr.eodHandler.GetEODConfig)
				r.Put("/", cr.eodHandler.UpdateEODConfig)
			})

			// EOD Runs
			r.Route("/runs", func(r chi.Router) {
				r.Get("/", cr.eodHandler.ListEODRuns)
				r.Post("/", cr.eodHandler.RunEOD)
				r.Get("/latest", cr.eodHandler.GetLatestEODRun)
				r.Get("/{id}", cr.eodHandler.GetEODRun)
			})

			// EOD Tasks
			r.Route("/tasks", func(r chi.Router) {
				r.Get("/", cr.eodHandler.ListEODTasks)
				r.Post("/", cr.eodHandler.CreateEODTask)
				r.Post("/initialize-defaults", cr.eodHandler.InitializeDefaultTasks)
				r.Get("/{id}", cr.eodHandler.GetEODTask)
				r.Put("/{id}", cr.eodHandler.UpdateEODTask)
				r.Delete("/{id}", cr.eodHandler.DeleteEODTask)
			})

			// Holiday Calendar
			r.Route("/holidays", func(r chi.Router) {
				r.Get("/", cr.eodHandler.ListHolidays)
				r.Post("/", cr.eodHandler.AddHoliday)
				r.Get("/check", cr.eodHandler.CheckHoliday)
				r.Delete("/{id}", cr.eodHandler.RemoveHoliday)
			})
		})
	})
}
