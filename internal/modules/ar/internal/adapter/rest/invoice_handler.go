package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ar/internal/domain"
	"converge-finance.com/m/internal/modules/ar/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type InvoiceHandler struct {
	invoiceService *service.InvoiceService
	logger         *zap.Logger
}

func NewInvoiceHandler(invoiceService *service.InvoiceService, logger *zap.Logger) *InvoiceHandler {
	return &InvoiceHandler{
		invoiceService: invoiceService,
		logger:         logger,
	}
}

type CreateInvoiceRequest struct {
	CustomerID  string                     `json:"customer_id"`
	InvoiceType string                     `json:"invoice_type,omitempty"`
	PONumber    string                     `json:"po_number,omitempty"`
	InvoiceDate string                     `json:"invoice_date"`
	DueDate     string                     `json:"due_date,omitempty"`
	Currency    string                     `json:"currency"`
	Description string                     `json:"description,omitempty"`
	Notes       string                     `json:"notes,omitempty"`
	Lines       []CreateInvoiceLineRequest `json:"lines"`
}

type CreateInvoiceLineRequest struct {
	RevenueAccountID string  `json:"revenue_account_id"`
	ItemCode         string  `json:"item_code,omitempty"`
	Description      string  `json:"description"`
	Quantity         float64 `json:"quantity"`
	UnitPrice        float64 `json:"unit_price"`
	DiscountPct      float64 `json:"discount_pct,omitempty"`
	TaxCode          string  `json:"tax_code,omitempty"`
	TaxRate          float64 `json:"tax_rate,omitempty"`
	ProjectID        string  `json:"project_id,omitempty"`
	CostCenterID     string  `json:"cost_center_id,omitempty"`
}

type InvoiceResponse struct {
	ID             string                `json:"id"`
	CustomerID     string                `json:"customer_id"`
	CustomerName   string                `json:"customer_name,omitempty"`
	InvoiceNumber  string                `json:"invoice_number"`
	InvoiceType    string                `json:"invoice_type"`
	PONumber       string                `json:"po_number,omitempty"`
	InvoiceDate    string                `json:"invoice_date"`
	DueDate        string                `json:"due_date"`
	Status         string                `json:"status"`
	Currency       string                `json:"currency"`
	Subtotal       float64               `json:"subtotal"`
	TaxAmount      float64               `json:"tax_amount"`
	ShippingAmount float64               `json:"shipping_amount"`
	DiscountAmount float64               `json:"discount_amount"`
	TotalAmount    float64               `json:"total_amount"`
	PaidAmount     float64               `json:"paid_amount"`
	BalanceDue     float64               `json:"balance_due"`
	Description    string                `json:"description,omitempty"`
	Notes          string                `json:"notes,omitempty"`
	Lines          []InvoiceLineResponse `json:"lines,omitempty"`
	IsOverdue      bool                  `json:"is_overdue"`
	DaysOverdue    int                   `json:"days_overdue"`
	CreatedAt      string                `json:"created_at"`
	UpdatedAt      string                `json:"updated_at"`
}

type InvoiceLineResponse struct {
	ID               string  `json:"id"`
	LineNumber       int     `json:"line_number"`
	RevenueAccountID string  `json:"revenue_account_id"`
	ItemCode         string  `json:"item_code,omitempty"`
	Description      string  `json:"description"`
	Quantity         float64 `json:"quantity"`
	UnitPrice        float64 `json:"unit_price"`
	Amount           float64 `json:"amount"`
	DiscountPct      float64 `json:"discount_pct,omitempty"`
	DiscountAmt      float64 `json:"discount_amt,omitempty"`
	TaxCode          string  `json:"tax_code,omitempty"`
	TaxRate          float64 `json:"tax_rate,omitempty"`
	TaxAmount        float64 `json:"tax_amount"`
}

func (h *InvoiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	invoiceDate, err := time.Parse("2006-01-02", req.InvoiceDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid invoice date format")
		return
	}

	var dueDate *time.Time
	if req.DueDate != "" {
		parsed, err := time.Parse("2006-01-02", req.DueDate)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid due date format")
			return
		}
		dueDate = &parsed
	}

	invoiceType := domain.InvoiceTypeStandard
	if req.InvoiceType != "" {
		invoiceType = domain.InvoiceType(req.InvoiceType)
	}

	lines := make([]service.CreateInvoiceLineRequest, len(req.Lines))
	for i, line := range req.Lines {
		var projectID, costCenterID *common.ID
		if line.ProjectID != "" {
			pid := common.ID(line.ProjectID)
			projectID = &pid
		}
		if line.CostCenterID != "" {
			ccid := common.ID(line.CostCenterID)
			costCenterID = &ccid
		}

		lines[i] = service.CreateInvoiceLineRequest{
			RevenueAccountID: common.ID(line.RevenueAccountID),
			ItemCode:         line.ItemCode,
			Description:      line.Description,
			Quantity:         line.Quantity,
			UnitPrice:        line.UnitPrice,
			DiscountPct:      line.DiscountPct,
			TaxCode:          line.TaxCode,
			TaxRate:          line.TaxRate,
			ProjectID:        projectID,
			CostCenterID:     costCenterID,
		}
	}

	svcReq := service.CreateInvoiceRequest{
		EntityID:    common.ID(claims.EntityID),
		CustomerID:  common.ID(req.CustomerID),
		InvoiceType: invoiceType,
		PONumber:    req.PONumber,
		InvoiceDate: invoiceDate,
		DueDate:     dueDate,
		Currency:    req.Currency,
		Description: req.Description,
		Notes:       req.Notes,
		Lines:       lines,
		CreatedBy:   common.ID(claims.UserID),
	}

	invoice, err := h.invoiceService.CreateInvoice(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("Failed to create invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toInvoiceResponse(invoice))
}

func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	invoice, err := h.invoiceService.GetInvoice(r.Context(), common.ID(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "Invoice not found")
		return
	}

	respondJSON(w, http.StatusOK, toInvoiceResponse(invoice))
}

func (h *InvoiceHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	filter := domain.InvoiceFilter{
		EntityID: common.ID(claims.EntityID),
		Limit:    50,
		Offset:   0,
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	if offset := r.URL.Query().Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.InvoiceStatus(status)
		filter.Status = &s
	}

	if customerID := r.URL.Query().Get("customer_id"); customerID != "" {
		cid := common.ID(customerID)
		filter.CustomerID = &cid
	}

	if r.URL.Query().Get("overdue_only") == "true" {
		filter.OverdueOnly = true
	}

	if r.URL.Query().Get("unpaid_only") == "true" {
		filter.UnpaidOnly = true
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = search
	}

	invoices, total, err := h.invoiceService.ListInvoices(r.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list invoices", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]InvoiceResponse, len(invoices))
	for i, inv := range invoices {
		response[i] = toInvoiceResponse(&inv)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"invoices": response,
		"total":    total,
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	})
}

func (h *InvoiceHandler) Submit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.invoiceService.SubmitInvoice(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to submit invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := h.invoiceService.ApproveInvoice(r.Context(), common.ID(id), common.ID(claims.UserID)); err != nil {
		h.logger.Error("Failed to approve invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.invoiceService.RejectInvoice(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to reject invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) Send(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.invoiceService.SendInvoice(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to send invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) Void(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if err := h.invoiceService.VoidInvoice(r.Context(), common.ID(id), req.Reason); err != nil {
		h.logger.Error("Failed to void invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) WriteOff(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.invoiceService.WriteOffInvoice(r.Context(), common.ID(id), req.Reason); err != nil {
		h.logger.Error("Failed to write off invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) GetOverdue(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	invoices, err := h.invoiceService.GetOverdueInvoices(r.Context(), common.ID(claims.EntityID))
	if err != nil {
		h.logger.Error("Failed to get overdue invoices", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]InvoiceResponse, len(invoices))
	for i, inv := range invoices {
		response[i] = toInvoiceResponse(&inv)
	}

	respondJSON(w, http.StatusOK, response)
}

func toInvoiceResponse(inv *domain.Invoice) InvoiceResponse {
	resp := InvoiceResponse{
		ID:             inv.ID.String(),
		CustomerID:     inv.CustomerID.String(),
		InvoiceNumber:  inv.InvoiceNumber,
		InvoiceType:    string(inv.InvoiceType),
		PONumber:       inv.PONumber,
		InvoiceDate:    inv.InvoiceDate.Format("2006-01-02"),
		DueDate:        inv.DueDate.Format("2006-01-02"),
		Status:         string(inv.Status),
		Currency:       inv.Currency.Code,
		Subtotal:       inv.Subtotal.Amount.InexactFloat64(),
		TaxAmount:      inv.TaxAmount.Amount.InexactFloat64(),
		ShippingAmount: inv.ShippingAmount.Amount.InexactFloat64(),
		DiscountAmount: inv.DiscountAmount.Amount.InexactFloat64(),
		TotalAmount:    inv.TotalAmount.Amount.InexactFloat64(),
		PaidAmount:     inv.PaidAmount.Amount.InexactFloat64(),
		BalanceDue:     inv.BalanceDue.Amount.InexactFloat64(),
		Description:    inv.Description,
		Notes:          inv.Notes,
		IsOverdue:      inv.IsOverdue(),
		DaysOverdue:    inv.DaysOverdue(),
		CreatedAt:      inv.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      inv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if inv.Customer != nil {
		resp.CustomerName = inv.Customer.Name
	}

	if len(inv.Lines) > 0 {
		resp.Lines = make([]InvoiceLineResponse, len(inv.Lines))
		for i, line := range inv.Lines {
			resp.Lines[i] = InvoiceLineResponse{
				ID:               line.ID.String(),
				LineNumber:       line.LineNumber,
				RevenueAccountID: line.RevenueAccountID.String(),
				ItemCode:         line.ItemCode,
				Description:      line.Description,
				Quantity:         line.Quantity,
				UnitPrice:        line.UnitPrice.Amount.InexactFloat64(),
				Amount:           line.Amount.Amount.InexactFloat64(),
				DiscountPct:      line.DiscountPct,
				DiscountAmt:      line.DiscountAmt.Amount.InexactFloat64(),
				TaxCode:          line.TaxCode,
				TaxRate:          line.TaxRate,
				TaxAmount:        line.TaxAmount.Amount.InexactFloat64(),
			}
		}
	}

	return resp
}
