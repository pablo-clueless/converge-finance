package rest

import (
	"net/http"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/ic/internal/service"
)

type ReconciliationHandler struct {
	*Handler
	reconcService *service.ReconciliationService
}

func NewReconciliationHandler(h *Handler, reconcService *service.ReconciliationService) *ReconciliationHandler {
	return &ReconciliationHandler{
		Handler:       h,
		reconcService: reconcService,
	}
}

func (h *ReconciliationHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	parentEntityID := getEntityID(r)
	if parentEntityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	fiscalPeriodID, err := common.Parse(r.URL.Query().Get("fiscal_period_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	summary, err := h.reconcService.GetReconciliationStatus(r.Context(), parentEntityID, fiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

func (h *ReconciliationHandler) GetEntityPairReconciliation(w http.ResponseWriter, r *http.Request) {
	fromEntityID, err := getIDParam(r, "from")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid from entity ID")
		return
	}

	toEntityID, err := getIDParam(r, "to")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid to entity ID")
		return
	}

	fiscalPeriodID, err := common.Parse(r.URL.Query().Get("fiscal_period_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	reconciliation, err := h.reconcService.GetEntityPairReconciliation(r.Context(), fromEntityID, toEntityID, fiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, reconciliation)
}

func (h *ReconciliationHandler) GetDiscrepancies(w http.ResponseWriter, r *http.Request) {
	parentEntityID := getEntityID(r)
	if parentEntityID.IsZero() {
		respondError(w, http.StatusBadRequest, "entity ID is required")
		return
	}

	fiscalPeriodID, err := common.Parse(r.URL.Query().Get("fiscal_period_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	discrepancies, err := h.reconcService.GetDiscrepancies(r.Context(), parentEntityID, fiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, discrepancies)
}

func (h *ReconciliationHandler) AutoReconcile(w http.ResponseWriter, r *http.Request) {
	fromEntityID, err := getIDParam(r, "from")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid from entity ID")
		return
	}

	toEntityID, err := getIDParam(r, "to")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid to entity ID")
		return
	}

	fiscalPeriodID, err := common.Parse(r.URL.Query().Get("fiscal_period_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	count, err := h.reconcService.AutoReconcile(r.Context(), fromEntityID, toEntityID, fiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"success":          true,
		"reconciled_count": count,
	})
}

func (h *ReconciliationHandler) RecalculateBalances(w http.ResponseWriter, r *http.Request) {
	fiscalPeriodID, err := common.Parse(r.URL.Query().Get("fiscal_period_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	if err := h.reconcService.RecalculateBalances(r.Context(), fiscalPeriodID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "balances recalculated"})
}
