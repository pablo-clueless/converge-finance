package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
	"converge-finance.com/m/internal/modules/ap/internal/service"
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
	VendorID      string                     `json:"vendor_id"`
	InvoiceNumber string                     `json:"invoice_number"`
	PONumber      string                     `json:"po_number,omitempty"`
	InvoiceDate   string                     `json:"invoice_date"`
	DueDate       string                     `json:"due_date,omitempty"`
	Currency      string                     `json:"currency"`
	Description   string                     `json:"description,omitempty"`
	Notes         string                     `json:"notes,omitempty"`
	Lines         []CreateInvoiceLineRequest `json:"lines"`
}

type CreateInvoiceLineRequest struct {
	AccountID    string  `json:"account_id"`
	Description  string  `json:"description"`
	Quantity     float64 `json:"quantity"`
	UnitPrice    float64 `json:"unit_price"`
	TaxCode      string  `json:"tax_code,omitempty"`
	TaxAmount    float64 `json:"tax_amount,omitempty"`
	ItemCode     string  `json:"item_code,omitempty"`
	ProjectID    string  `json:"project_id,omitempty"`
	CostCenterID string  `json:"cost_center_id,omitempty"`
}

type InvoiceResponse struct {
	ID             string                `json:"id"`
	VendorID       string                `json:"vendor_id"`
	VendorName     string                `json:"vendor_name,omitempty"`
	InvoiceNumber  string                `json:"invoice_number"`
	InternalNumber string                `json:"internal_number"`
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
	ID          string  `json:"id"`
	LineNumber  int     `json:"line_number"`
	AccountID   string  `json:"account_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	TaxCode     string  `json:"tax_code,omitempty"`
	TaxAmount   float64 `json:"tax_amount"`
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
			AccountID:    common.ID(line.AccountID),
			Description:  line.Description,
			Quantity:     line.Quantity,
			UnitPrice:    line.UnitPrice,
			TaxCode:      line.TaxCode,
			TaxAmount:    line.TaxAmount,
			ItemCode:     line.ItemCode,
			ProjectID:    projectID,
			CostCenterID: costCenterID,
		}
	}

	svcReq := service.CreateInvoiceRequest{
		EntityID:      common.ID(claims.EntityID),
		VendorID:      common.ID(req.VendorID),
		InvoiceNumber: req.InvoiceNumber,
		PONumber:      req.PONumber,
		InvoiceDate:   invoiceDate,
		DueDate:       dueDate,
		Currency:      req.Currency,
		Description:   req.Description,
		Notes:         req.Notes,
		Lines:         lines,
		CreatedBy:     common.ID(claims.UserID),
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

	if vendorID := r.URL.Query().Get("vendor_id"); vendorID != "" {
		vid := common.ID(vendorID)
		filter.VendorID = &vid
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

	var req struct {
		Notes string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.invoiceService.ApproveInvoice(r.Context(), common.ID(id), common.ID(claims.UserID), req.Notes); err != nil {
		h.logger.Error("Failed to approve invoice", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.invoiceService.RejectInvoice(r.Context(), common.ID(id), req.Notes); err != nil {
		h.logger.Error("Failed to reject invoice", zap.Error(err))
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
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.invoiceService.VoidInvoice(r.Context(), common.ID(id), req.Reason); err != nil {
		h.logger.Error("Failed to void invoice", zap.Error(err))
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

func (h *InvoiceHandler) GetForPayment(w http.ResponseWriter, r *http.Request) {
	vendorID := r.URL.Query().Get("vendor_id")
	if vendorID == "" {
		respondError(w, http.StatusBadRequest, "vendor_id is required")
		return
	}

	invoices, err := h.invoiceService.GetInvoicesForPayment(r.Context(), common.ID(vendorID))
	if err != nil {
		h.logger.Error("Failed to get invoices for payment", zap.Error(err))
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
		VendorID:       inv.VendorID.String(),
		InvoiceNumber:  inv.InvoiceNumber,
		InternalNumber: inv.InternalNumber,
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

	if inv.Vendor != nil {
		resp.VendorName = inv.Vendor.Name
	}

	if len(inv.Lines) > 0 {
		resp.Lines = make([]InvoiceLineResponse, len(inv.Lines))
		for i, line := range inv.Lines {
			resp.Lines[i] = InvoiceLineResponse{
				ID:          line.ID.String(),
				LineNumber:  line.LineNumber,
				AccountID:   line.AccountID.String(),
				Description: line.Description,
				Quantity:    line.Quantity,
				UnitPrice:   line.UnitPrice.Amount.InexactFloat64(),
				Amount:      line.Amount.Amount.InexactFloat64(),
				TaxCode:     line.TaxCode,
				TaxAmount:   line.TaxAmount.Amount.InexactFloat64(),
			}
		}
	}

	return resp
}
