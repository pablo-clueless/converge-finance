package rest

import (
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ar/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ReportsHandler struct {
	dunningService *service.DunningService
	logger         *zap.Logger
}

func NewReportsHandler(dunningService *service.DunningService, logger *zap.Logger) *ReportsHandler {
	return &ReportsHandler{
		dunningService: dunningService,
		logger:         logger,
	}
}

func (h *ReportsHandler) GetAgingReport(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	asOfDate := time.Now()
	if dateStr := r.URL.Query().Get("as_of_date"); dateStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			asOfDate = parsed
		}
	}

	report, err := h.dunningService.GetAgingReport(r.Context(), common.ID(claims.EntityID), asOfDate)
	if err != nil {
		h.logger.Error("Failed to get aging report", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	customerAgings := make([]map[string]interface{}, len(report.CustomerAgings))
	for i, ca := range report.CustomerAgings {
		customerAgings[i] = map[string]interface{}{
			"customer_id":   ca.CustomerID.String(),
			"customer_code": ca.CustomerCode,
			"customer_name": ca.CustomerName,
			"current":       ca.Current.Amount.InexactFloat64(),
			"days_1_30":     ca.Days1To30.Amount.InexactFloat64(),
			"days_31_60":    ca.Days31To60.Amount.InexactFloat64(),
			"days_61_90":    ca.Days61To90.Amount.InexactFloat64(),
			"over_90_days":  ca.Over90Days.Amount.InexactFloat64(),
			"total":         ca.TotalBalance.Amount.InexactFloat64(),
			"invoice_count": ca.InvoiceCount,
		}
		if ca.OldestDue != nil {
			customerAgings[i]["oldest_due"] = ca.OldestDue.Format("2006-01-02")
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"as_of_date":      report.AsOfDate.Format("2006-01-02"),
		"currency":        report.Currency.Code,
		"total_current":   report.TotalCurrent.Amount.InexactFloat64(),
		"total_1_30":      report.TotalDays1To30.Amount.InexactFloat64(),
		"total_31_60":     report.TotalDays31To60.Amount.InexactFloat64(),
		"total_61_90":     report.TotalDays61To90.Amount.InexactFloat64(),
		"total_over_90":   report.TotalOver90Days.Amount.InexactFloat64(),
		"grand_total":     report.GrandTotal.Amount.InexactFloat64(),
		"customer_agings": customerAgings,
	})
}

func (h *ReportsHandler) GetCustomerAging(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		respondError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	asOfDate := time.Now()
	if dateStr := r.URL.Query().Get("as_of_date"); dateStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			asOfDate = parsed
		}
	}

	aging, err := h.dunningService.GetCustomerAgingReport(r.Context(), common.ID(customerID), asOfDate)
	if err != nil {
		h.logger.Error("Failed to get customer aging", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"customer_id":   aging.CustomerID.String(),
		"customer_code": aging.CustomerCode,
		"customer_name": aging.CustomerName,
		"current":       aging.Current.Amount.InexactFloat64(),
		"days_1_30":     aging.Days1To30.Amount.InexactFloat64(),
		"days_31_60":    aging.Days31To60.Amount.InexactFloat64(),
		"days_61_90":    aging.Days61To90.Amount.InexactFloat64(),
		"over_90_days":  aging.Over90Days.Amount.InexactFloat64(),
		"total":         aging.TotalBalance.Amount.InexactFloat64(),
		"invoice_count": aging.InvoiceCount,
	}
	if aging.OldestDue != nil {
		response["oldest_due"] = aging.OldestDue.Format("2006-01-02")
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ReportsHandler) GetCashForecast(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	asOfDate := time.Now()
	if dateStr := r.URL.Query().Get("as_of_date"); dateStr != "" {
		if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
			asOfDate = parsed
		}
	}

	weeksAhead := 8
	if weeks := r.URL.Query().Get("weeks"); weeks != "" {
		if w, err := strconv.Atoi(weeks); err == nil && w > 0 && w <= 52 {
			weeksAhead = w
		}
	}

	forecast, err := h.dunningService.GetCashForecast(r.Context(), common.ID(claims.EntityID), asOfDate, weeksAhead)
	if err != nil {
		h.logger.Error("Failed to get cash forecast", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	projections := make([]map[string]interface{}, len(forecast.Projections))
	for i, p := range forecast.Projections {
		projections[i] = map[string]interface{}{
			"period_start":    p.PeriodStart.Format("2006-01-02"),
			"period_end":      p.PeriodEnd.Format("2006-01-02"),
			"label":           p.Label,
			"expected_amount": p.ExpectedAmount.Amount.InexactFloat64(),
			"invoice_count":   p.InvoiceCount,
			"cumulative":      p.Cumulative.Amount.InexactFloat64(),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"as_of_date":          forecast.AsOfDate.Format("2006-01-02"),
		"generated_at":        forecast.GeneratedAt.Format(time.RFC3339),
		"currency":            forecast.Currency.Code,
		"overdue_amount":      forecast.OverdueAmount.Amount.InexactFloat64(),
		"overdue_count":       forecast.OverdueCount,
		"expected_this_week":  forecast.ExpectedThisWeek.Amount.InexactFloat64(),
		"expected_next_week":  forecast.ExpectedNextWeek.Amount.InexactFloat64(),
		"expected_this_month": forecast.ExpectedThisMonth.Amount.InexactFloat64(),
		"expected_next_month": forecast.ExpectedNextMonth.Amount.InexactFloat64(),
		"projections":         projections,
	})
}

func (h *ReportsHandler) GetOverdueAlerts(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	alerts, err := h.dunningService.GetOverdueAlerts(r.Context(), common.ID(claims.EntityID))
	if err != nil {
		h.logger.Error("Failed to get overdue alerts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]map[string]interface{}, len(alerts))
	for i, alert := range alerts {
		response[i] = map[string]interface{}{
			"customer_id":    alert.CustomerID.String(),
			"customer_code":  alert.CustomerCode,
			"customer_name":  alert.CustomerName,
			"overdue_amount": alert.OverdueAmount.Amount.InexactFloat64(),
			"days_overdue":   alert.DaysOverdue,
			"priority":       alert.Priority,
			"on_credit_hold": alert.OnCreditHold,
		}
		if alert.OldestInvoice != nil {
			response[i]["oldest_invoice"] = alert.OldestInvoice.Format("2006-01-02")
		}
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ReportsHandler) GetCustomerStatistics(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	if customerID == "" {
		respondError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}

	stats, err := h.dunningService.GetCustomerStatistics(r.Context(), common.ID(customerID))
	if err != nil {
		h.logger.Error("Failed to get customer statistics", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"customer_id":          stats.CustomerID.String(),
		"total_invoices":       stats.TotalInvoices,
		"total_outstanding":    stats.TotalOutstanding.Amount.InexactFloat64(),
		"total_overdue":        stats.TotalOverdue.Amount.InexactFloat64(),
		"average_payment_days": stats.AveragePaymentDays,
		"invoices_this_year":   stats.InvoicesThisYear,
		"receipts_this_year":   stats.ReceiptsThisYear,
	})
}

func (h *ReportsHandler) GetCustomersForCreditReview(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	customers, err := h.dunningService.GetCustomersForCreditReview(r.Context(), common.ID(claims.EntityID))
	if err != nil {
		h.logger.Error("Failed to get customers for credit review", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]CustomerResponse, len(customers))
	for i, c := range customers {
		response[i] = toCustomerResponse(&c)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ReportsHandler) EscalateDunning(w http.ResponseWriter, r *http.Request) {
	invoiceID := chi.URLParam(r, "id")
	if invoiceID == "" {
		respondError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	if err := h.dunningService.EscalateDunning(r.Context(), common.ID(invoiceID)); err != nil {
		h.logger.Error("Failed to escalate dunning", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
