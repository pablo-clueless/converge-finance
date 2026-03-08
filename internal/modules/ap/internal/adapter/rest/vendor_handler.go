package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ap/internal/domain"
	"converge-finance.com/m/internal/modules/ap/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type VendorHandler struct {
	vendorService *service.VendorService
	logger        *zap.Logger
}

func NewVendorHandler(vendorService *service.VendorService, logger *zap.Logger) *VendorHandler {
	return &VendorHandler{
		vendorService: vendorService,
		logger:        logger,
	}
}

type CreateVendorRequest struct {
	VendorCode    string  `json:"vendor_code"`
	Name          string  `json:"name"`
	LegalName     string  `json:"legal_name,omitempty"`
	TaxID         string  `json:"tax_id,omitempty"`
	Email         string  `json:"email,omitempty"`
	Phone         string  `json:"phone,omitempty"`
	Website       string  `json:"website,omitempty"`
	Currency      string  `json:"currency"`
	PaymentTerms  string  `json:"payment_terms,omitempty"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	CreditLimit   float64 `json:"credit_limit,omitempty"`
	Is1099Vendor  bool    `json:"is_1099_vendor"`
	Notes         string  `json:"notes,omitempty"`
}

type VendorResponse struct {
	ID             string  `json:"id"`
	VendorCode     string  `json:"vendor_code"`
	Name           string  `json:"name"`
	LegalName      string  `json:"legal_name,omitempty"`
	TaxID          string  `json:"tax_id,omitempty"`
	Email          string  `json:"email,omitempty"`
	Phone          string  `json:"phone,omitempty"`
	Website        string  `json:"website,omitempty"`
	Status         string  `json:"status"`
	Currency       string  `json:"currency"`
	PaymentTerms   string  `json:"payment_terms"`
	PaymentMethod  string  `json:"payment_method"`
	CreditLimit    float64 `json:"credit_limit"`
	CurrentBalance float64 `json:"current_balance"`
	Is1099Vendor   bool    `json:"is_1099_vendor"`
	Notes          string  `json:"notes,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

func (h *VendorHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateVendorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	paymentTerms := domain.PaymentTermsNet30
	if req.PaymentTerms != "" {
		paymentTerms = domain.PaymentTerms(req.PaymentTerms)
	}

	paymentMethod := domain.PaymentMethodCheck
	if req.PaymentMethod != "" {
		paymentMethod = domain.PaymentMethod(req.PaymentMethod)
	}

	svcReq := service.CreateVendorRequest{
		EntityID:      common.ID(claims.EntityID),
		VendorCode:    req.VendorCode,
		Name:          req.Name,
		LegalName:     req.LegalName,
		TaxID:         req.TaxID,
		Email:         req.Email,
		Phone:         req.Phone,
		Website:       req.Website,
		Currency:      req.Currency,
		PaymentTerms:  paymentTerms,
		PaymentMethod: paymentMethod,
		CreditLimit:   req.CreditLimit,
		Is1099Vendor:  req.Is1099Vendor,
		Notes:         req.Notes,
		CreatedBy:     common.ID(claims.UserID),
	}

	vendor, err := h.vendorService.CreateVendor(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("Failed to create vendor", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toVendorResponse(vendor))
}

func (h *VendorHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	vendor, err := h.vendorService.GetVendor(r.Context(), common.ID(id))
	if err != nil {
		respondError(w, http.StatusNotFound, "Vendor not found")
		return
	}

	respondJSON(w, http.StatusOK, toVendorResponse(vendor))
}

func (h *VendorHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	filter := domain.VendorFilter{
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
		s := domain.VendorStatus(status)
		filter.Status = &s
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = search
	}

	vendors, total, err := h.vendorService.ListVendors(r.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list vendors", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]VendorResponse, len(vendors))
	for i, v := range vendors {
		response[i] = toVendorResponse(&v)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"vendors": response,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}

func (h *VendorHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req CreateVendorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var paymentTerms *domain.PaymentTerms
	if req.PaymentTerms != "" {
		pt := domain.PaymentTerms(req.PaymentTerms)
		paymentTerms = &pt
	}

	var paymentMethod *domain.PaymentMethod
	if req.PaymentMethod != "" {
		pm := domain.PaymentMethod(req.PaymentMethod)
		paymentMethod = &pm
	}

	svcReq := service.UpdateVendorRequest{
		ID:            common.ID(id),
		Name:          &req.Name,
		LegalName:     &req.LegalName,
		TaxID:         &req.TaxID,
		Email:         &req.Email,
		Phone:         &req.Phone,
		Website:       &req.Website,
		PaymentTerms:  paymentTerms,
		PaymentMethod: paymentMethod,
		CreditLimit:   &req.CreditLimit,
		Is1099Vendor:  &req.Is1099Vendor,
		Notes:         &req.Notes,
	}

	vendor, err := h.vendorService.UpdateVendor(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("Failed to update vendor", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toVendorResponse(vendor))
}

func (h *VendorHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.vendorService.DeleteVendor(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to delete vendor", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *VendorHandler) Activate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.vendorService.ActivateVendor(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to activate vendor", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *VendorHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.vendorService.DeactivateVendor(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to deactivate vendor", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *VendorHandler) Block(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.vendorService.BlockVendor(r.Context(), common.ID(id)); err != nil {
		h.logger.Error("Failed to block vendor", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *VendorHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	balance, err := h.vendorService.GetVendorBalance(r.Context(), common.ID(id))
	if err != nil {
		h.logger.Error("Failed to get vendor balance", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"vendor_id":        balance.VendorID.String(),
		"total_invoiced":   balance.TotalInvoiced.Amount.InexactFloat64(),
		"total_paid":       balance.TotalPaid.Amount.InexactFloat64(),
		"current_balance":  balance.CurrentBalance.Amount.InexactFloat64(),
		"overdue_balance":  balance.OverdueBalance.Amount.InexactFloat64(),
		"available_credit": balance.AvailableCredit.Amount.InexactFloat64(),
	})
}

func (h *VendorHandler) Search(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "Search query is required")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	vendors, err := h.vendorService.SearchVendors(r.Context(), common.ID(claims.EntityID), query, limit)
	if err != nil {
		h.logger.Error("Failed to search vendors", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]VendorResponse, len(vendors))
	for i, v := range vendors {
		response[i] = toVendorResponse(&v)
	}

	respondJSON(w, http.StatusOK, response)
}

func toVendorResponse(v *domain.Vendor) VendorResponse {
	return VendorResponse{
		ID:             v.ID.String(),
		VendorCode:     v.VendorCode,
		Name:           v.Name,
		LegalName:      v.LegalName,
		TaxID:          v.TaxID,
		Email:          v.Email,
		Phone:          v.Phone,
		Website:        v.Website,
		Status:         string(v.Status),
		Currency:       v.Currency.Code,
		PaymentTerms:   string(v.PaymentTerms),
		PaymentMethod:  string(v.PaymentMethod),
		CreditLimit:    v.CreditLimit.Amount.InexactFloat64(),
		CurrentBalance: v.CurrentBalance.Amount.InexactFloat64(),
		Is1099Vendor:   v.Is1099Vendor,
		Notes:          v.Notes,
		CreatedAt:      v.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      v.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
