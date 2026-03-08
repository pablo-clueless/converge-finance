package rest

import (
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ap/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type AgingHandler struct {
	agingService *service.AgingService
	logger       *zap.Logger
}

func NewAgingHandler(agingService *service.AgingService, logger *zap.Logger) *AgingHandler {
	return &AgingHandler{
		agingService: agingService,
		logger:       logger,
	}
}

func (h *AgingHandler) GetAgingReport(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	asOfDate := time.Now()
	if asOf := r.URL.Query().Get("as_of_date"); asOf != "" {
		if parsed, err := time.Parse("2006-01-02", asOf); err == nil {
			asOfDate = parsed
		}
	}

	report, err := h.agingService.GetAgingReport(r.Context(), common.ID(claims.EntityID), asOfDate)
	if err != nil {
		h.logger.Error("Failed to get aging report", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	vendorAgings := make([]map[string]interface{}, len(report.VendorAgings))
	for i, va := range report.VendorAgings {
		vendorAgings[i] = map[string]interface{}{
			"vendor_id":     va.VendorID.String(),
			"vendor_code":   va.VendorCode,
			"vendor_name":   va.VendorName,
			"current":       va.Current.Amount.InexactFloat64(),
			"days_1_30":     va.Days1To30.Amount.InexactFloat64(),
			"days_31_60":    va.Days31To60.Amount.InexactFloat64(),
			"days_61_90":    va.Days61To90.Amount.InexactFloat64(),
			"over_90_days":  va.Over90Days.Amount.InexactFloat64(),
			"total_balance": va.TotalBalance.Amount.InexactFloat64(),
			"invoice_count": va.InvoiceCount,
		}
		if va.OldestDue != nil {
			vendorAgings[i]["oldest_due"] = va.OldestDue.Format("2006-01-02")
		}
	}

	buckets := report.GetAgingBuckets()
	bucketResponse := make([]map[string]interface{}, len(buckets))
	for i, b := range buckets {
		bucketResponse[i] = map[string]interface{}{
			"label":      b.Label,
			"amount":     b.Amount.Amount.InexactFloat64(),
			"percentage": b.Percentage,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"as_of_date":           asOfDate.Format("2006-01-02"),
		"currency":             report.Currency.Code,
		"total_current":        report.TotalCurrent.Amount.InexactFloat64(),
		"total_days_1_30":      report.TotalDays1To30.Amount.InexactFloat64(),
		"total_days_31_60":     report.TotalDays31To60.Amount.InexactFloat64(),
		"total_days_61_90":     report.TotalDays61To90.Amount.InexactFloat64(),
		"total_over_90_days":   report.TotalOver90Days.Amount.InexactFloat64(),
		"grand_total":          report.GrandTotal.Amount.InexactFloat64(),
		"total_vendors":        report.TotalVendors,
		"vendors_with_balance": report.VendorsWithBalance,
		"vendors_overdue":      report.VendorsOverdue,
		"buckets":              bucketResponse,
		"vendor_agings":        vendorAgings,
		"generated_at":         report.GeneratedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *AgingHandler) GetVendorAging(w http.ResponseWriter, r *http.Request) {
	vendorID := chi.URLParam(r, "id")

	asOfDate := time.Now()
	if asOf := r.URL.Query().Get("as_of_date"); asOf != "" {
		if parsed, err := time.Parse("2006-01-02", asOf); err == nil {
			asOfDate = parsed
		}
	}

	aging, err := h.agingService.GetVendorAgingReport(r.Context(), common.ID(vendorID), asOfDate)
	if err != nil {
		h.logger.Error("Failed to get vendor aging", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := map[string]interface{}{
		"vendor_id":     aging.VendorID.String(),
		"vendor_code":   aging.VendorCode,
		"vendor_name":   aging.VendorName,
		"as_of_date":    asOfDate.Format("2006-01-02"),
		"current":       aging.Current.Amount.InexactFloat64(),
		"days_1_30":     aging.Days1To30.Amount.InexactFloat64(),
		"days_31_60":    aging.Days31To60.Amount.InexactFloat64(),
		"days_61_90":    aging.Days61To90.Amount.InexactFloat64(),
		"over_90_days":  aging.Over90Days.Amount.InexactFloat64(),
		"total_balance": aging.TotalBalance.Amount.InexactFloat64(),
		"invoice_count": aging.InvoiceCount,
	}

	if aging.OldestDue != nil {
		response["oldest_due"] = aging.OldestDue.Format("2006-01-02")
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *AgingHandler) GetCashRequirements(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	asOfDate := time.Now()
	if asOf := r.URL.Query().Get("as_of_date"); asOf != "" {
		if parsed, err := time.Parse("2006-01-02", asOf); err == nil {
			asOfDate = parsed
		}
	}

	weeksAhead := 4
	if weeks := r.URL.Query().Get("weeks_ahead"); weeks != "" {
		if w, err := strconv.Atoi(weeks); err == nil && w > 0 && w <= 12 {
			weeksAhead = w
		}
	}

	req, err := h.agingService.GetCashRequirements(r.Context(), common.ID(claims.EntityID), asOfDate, weeksAhead)
	if err != nil {
		h.logger.Error("Failed to get cash requirements", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	projections := make([]map[string]interface{}, len(req.Projections))
	for i, p := range req.Projections {
		projections[i] = map[string]interface{}{
			"period_start": p.PeriodStart.Format("2006-01-02"),
			"period_end":   p.PeriodEnd.Format("2006-01-02"),
			"label":        p.Label,
			"due_amount":   p.DueAmount.Amount.InexactFloat64(),
			"due_count":    p.DueCount,
			"cumulative":   p.Cumulative.Amount.InexactFloat64(),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"as_of_date":           asOfDate.Format("2006-01-02"),
		"currency":             req.Currency.Code,
		"overdue_amount":       req.OverdueAmount.Amount.InexactFloat64(),
		"overdue_count":        req.OverdueCount,
		"due_this_week":        req.DueThisWeek.Amount.InexactFloat64(),
		"due_this_week_count":  req.DueThisWeekCount,
		"due_next_week":        req.DueNextWeek.Amount.InexactFloat64(),
		"due_next_week_count":  req.DueNextWeekCount,
		"due_this_month":       req.DueThisMonth.Amount.InexactFloat64(),
		"due_this_month_count": req.DueThisMonthCount,
		"scheduled_amount":     req.ScheduledAmount.Amount.InexactFloat64(),
		"scheduled_count":      req.ScheduledCount,
		"potential_discounts":  req.PotentialDiscounts.Amount.InexactFloat64(),
		"projections":          projections,
		"generated_at":         req.GeneratedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *AgingHandler) GetOverdueAlerts(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	alerts, err := h.agingService.GetOverdueAlerts(r.Context(), common.ID(claims.EntityID))
	if err != nil {
		h.logger.Error("Failed to get overdue alerts", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]map[string]interface{}, len(alerts))
	for i, alert := range alerts {
		response[i] = map[string]interface{}{
			"vendor_id":      alert.VendorID.String(),
			"vendor_code":    alert.VendorCode,
			"vendor_name":    alert.VendorName,
			"overdue_amount": alert.OverdueAmount.Amount.InexactFloat64(),
			"days_overdue":   alert.DaysOverdue,
			"priority":       alert.Priority,
		}
		if alert.OldestInvoice != nil {
			response[i]["oldest_invoice"] = alert.OldestInvoice.Format("2006-01-02")
		}
	}

	respondJSON(w, http.StatusOK, response)
}
