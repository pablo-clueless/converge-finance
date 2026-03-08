package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/modules/segment/internal/repository"
	"converge-finance.com/m/internal/modules/segment/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type SegmentHandler struct {
	service *service.SegmentService
	logger  *zap.Logger
}

func NewSegmentHandler(svc *service.SegmentService, logger *zap.Logger) *SegmentHandler {
	return &SegmentHandler{
		service: svc,
		logger:  logger,
	}
}

type CreateSegmentRequest struct {
	SegmentCode  string  `json:"segment_code"`
	SegmentName  string  `json:"segment_name"`
	SegmentType  string  `json:"segment_type"`
	ParentID     *string `json:"parent_id,omitempty"`
	Description  string  `json:"description,omitempty"`
	ManagerID    *string `json:"manager_id,omitempty"`
	IsReportable bool    `json:"is_reportable"`
}

func (h *SegmentHandler) CreateSegment(w http.ResponseWriter, r *http.Request) {
	var req CreateSegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	segmentType := domain.SegmentType(req.SegmentType)
	if !segmentType.IsValid() {
		h.writeError(w, http.StatusBadRequest, "invalid segment_type")
		return
	}

	var parentID *common.ID
	if req.ParentID != nil && *req.ParentID != "" {
		id := common.ID(*req.ParentID)
		parentID = &id
	}

	var managerID *common.ID
	if req.ManagerID != nil && *req.ManagerID != "" {
		id := common.ID(*req.ManagerID)
		managerID = &id
	}

	segment, err := h.service.CreateSegment(r.Context(), service.CreateSegmentRequest{
		EntityID:     entityID,
		SegmentCode:  req.SegmentCode,
		SegmentName:  req.SegmentName,
		SegmentType:  segmentType,
		ParentID:     parentID,
		Description:  req.Description,
		ManagerID:    managerID,
		IsReportable: req.IsReportable,
	})
	if err != nil {
		h.logger.Error("failed to create segment", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, segment)
}

func (h *SegmentHandler) GetSegment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	segment, err := h.service.GetSegment(r.Context(), id)
	if err == domain.ErrSegmentNotFound {
		h.writeError(w, http.StatusNotFound, "segment not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, segment)
}

type UpdateSegmentRequest struct {
	SegmentName  string  `json:"segment_name,omitempty"`
	Description  string  `json:"description,omitempty"`
	ParentID     *string `json:"parent_id,omitempty"`
	ManagerID    *string `json:"manager_id,omitempty"`
	IsReportable bool    `json:"is_reportable"`
}

func (h *SegmentHandler) UpdateSegment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	var req UpdateSegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var parentID *common.ID
	if req.ParentID != nil && *req.ParentID != "" {
		pid := common.ID(*req.ParentID)
		parentID = &pid
	}

	var managerID *common.ID
	if req.ManagerID != nil && *req.ManagerID != "" {
		mid := common.ID(*req.ManagerID)
		managerID = &mid
	}

	segment, err := h.service.UpdateSegment(r.Context(), service.UpdateSegmentRequest{
		ID:           id,
		SegmentName:  req.SegmentName,
		Description:  req.Description,
		ParentID:     parentID,
		ManagerID:    managerID,
		IsReportable: req.IsReportable,
	})
	if err != nil {
		h.logger.Error("failed to update segment", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, segment)
}

func (h *SegmentHandler) DeleteSegment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	if err := h.service.DeleteSegment(r.Context(), id); err != nil {
		h.logger.Error("failed to delete segment", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SegmentHandler) ListSegments(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.SegmentFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if segmentType := r.URL.Query().Get("segment_type"); segmentType != "" {
		t := domain.SegmentType(segmentType)
		filter.SegmentType = &t
	}

	if isActive := r.URL.Query().Get("is_active"); isActive != "" {
		active := isActive == "true"
		filter.IsActive = &active
	}

	if isReportable := r.URL.Query().Get("is_reportable"); isReportable != "" {
		reportable := isReportable == "true"
		filter.IsReportable = &reportable
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	segments, total, err := h.service.ListSegments(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   segments,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *SegmentHandler) GetSegmentTree(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	segmentTypeStr := r.URL.Query().Get("segment_type")
	if segmentTypeStr == "" {
		h.writeError(w, http.StatusBadRequest, "segment_type is required")
		return
	}

	segmentType := domain.SegmentType(segmentTypeStr)
	if !segmentType.IsValid() {
		h.writeError(w, http.StatusBadRequest, "invalid segment_type")
		return
	}

	tree, err := h.service.GetSegmentTree(r.Context(), entityID, segmentType)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, tree)
}

func (h *SegmentHandler) ActivateSegment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	segment, err := h.service.ActivateSegment(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to activate segment", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, segment)
}

func (h *SegmentHandler) DeactivateSegment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	segment, err := h.service.DeactivateSegment(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to deactivate segment", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, segment)
}

type CreateHierarchyRequest struct {
	HierarchyCode string `json:"hierarchy_code"`
	HierarchyName string `json:"hierarchy_name"`
	SegmentType   string `json:"segment_type"`
	Description   string `json:"description,omitempty"`
	IsPrimary     bool   `json:"is_primary"`
}

func (h *SegmentHandler) CreateHierarchy(w http.ResponseWriter, r *http.Request) {
	var req CreateHierarchyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	segmentType := domain.SegmentType(req.SegmentType)
	if !segmentType.IsValid() {
		h.writeError(w, http.StatusBadRequest, "invalid segment_type")
		return
	}

	hierarchy, err := h.service.CreateHierarchy(r.Context(), service.CreateHierarchyRequest{
		EntityID:      entityID,
		HierarchyCode: req.HierarchyCode,
		HierarchyName: req.HierarchyName,
		SegmentType:   segmentType,
		Description:   req.Description,
		IsPrimary:     req.IsPrimary,
	})
	if err != nil {
		h.logger.Error("failed to create hierarchy", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, hierarchy)
}

func (h *SegmentHandler) GetHierarchy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	hierarchy, err := h.service.GetHierarchy(r.Context(), id)
	if err == domain.ErrHierarchyNotFound {
		h.writeError(w, http.StatusNotFound, "hierarchy not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, hierarchy)
}

func (h *SegmentHandler) ListHierarchies(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	var segmentType *domain.SegmentType
	if segmentTypeStr := r.URL.Query().Get("segment_type"); segmentTypeStr != "" {
		t := domain.SegmentType(segmentTypeStr)
		segmentType = &t
	}

	hierarchies, err := h.service.ListHierarchies(r.Context(), entityID, segmentType)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, hierarchies)
}

type AssignToSegmentRequest struct {
	SegmentID         string  `json:"segment_id"`
	AssignmentType    string  `json:"assignment_type"`
	AssignmentID      string  `json:"assignment_id"`
	AllocationPercent float64 `json:"allocation_percent"`
	EffectiveFrom     string  `json:"effective_from"`
	EffectiveTo       *string `json:"effective_to,omitempty"`
}

func (h *SegmentHandler) AssignToSegment(w http.ResponseWriter, r *http.Request) {
	var req AssignToSegmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	effectiveFrom, err := time.Parse("2006-01-02", req.EffectiveFrom)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid effective_from date (use YYYY-MM-DD)")
		return
	}

	var effectiveTo *time.Time
	if req.EffectiveTo != nil && *req.EffectiveTo != "" {
		t, err := time.Parse("2006-01-02", *req.EffectiveTo)
		if err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid effective_to date (use YYYY-MM-DD)")
			return
		}
		effectiveTo = &t
	}

	assignment, err := h.service.AssignToSegment(r.Context(), service.AssignToSegmentRequest{
		EntityID:          entityID,
		SegmentID:         common.ID(req.SegmentID),
		AssignmentType:    req.AssignmentType,
		AssignmentID:      common.ID(req.AssignmentID),
		AllocationPercent: decimal.NewFromFloat(req.AllocationPercent),
		EffectiveFrom:     effectiveFrom,
		EffectiveTo:       effectiveTo,
	})
	if err != nil {
		h.logger.Error("failed to create assignment", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, assignment)
}

func (h *SegmentHandler) GetAssignment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	assignment, err := h.service.GetAssignment(r.Context(), id)
	if err == domain.ErrAssignmentNotFound {
		h.writeError(w, http.StatusNotFound, "assignment not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, assignment)
}

func (h *SegmentHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.AssignmentFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if segmentID := r.URL.Query().Get("segment_id"); segmentID != "" {
		id := common.ID(segmentID)
		filter.SegmentID = &id
	}

	if assignmentType := r.URL.Query().Get("assignment_type"); assignmentType != "" {
		filter.AssignmentType = assignmentType
	}

	if assignmentID := r.URL.Query().Get("assignment_id"); assignmentID != "" {
		id := common.ID(assignmentID)
		filter.AssignmentID = &id
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	assignments, total, err := h.service.ListAssignments(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   assignments,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *SegmentHandler) DeleteAssignment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	if err := h.service.DeleteAssignment(r.Context(), id); err != nil {
		h.logger.Error("failed to delete assignment", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *SegmentHandler) GetBalanceSummary(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	periodID := r.URL.Query().Get("period_id")
	if periodID == "" {
		h.writeError(w, http.StatusBadRequest, "period_id is required")
		return
	}

	summary, err := h.service.GetBalanceSummary(r.Context(), entityID, common.ID(periodID))
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, summary)
}

type CreateIntersegmentTransactionRequest struct {
	FiscalPeriodID  string `json:"fiscal_period_id"`
	FromSegmentID   string `json:"from_segment_id"`
	ToSegmentID     string `json:"to_segment_id"`
	TransactionDate string `json:"transaction_date"`
	Description     string `json:"description,omitempty"`
	Amount          string `json:"amount"`
	CurrencyCode    string `json:"currency_code"`
}

func (h *SegmentHandler) CreateIntersegmentTransaction(w http.ResponseWriter, r *http.Request) {
	var req CreateIntersegmentTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	transactionDate, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid transaction_date (use YYYY-MM-DD)")
		return
	}

	txn, err := h.service.CreateIntersegmentTransaction(r.Context(), service.CreateIntersegmentTransactionRequest{
		EntityID:        entityID,
		FiscalPeriodID:  common.ID(req.FiscalPeriodID),
		FromSegmentID:   common.ID(req.FromSegmentID),
		ToSegmentID:     common.ID(req.ToSegmentID),
		TransactionDate: transactionDate,
		Description:     req.Description,
		Amount:          req.Amount,
		CurrencyCode:    req.CurrencyCode,
	})
	if err != nil {
		h.logger.Error("failed to create intersegment transaction", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, txn)
}

func (h *SegmentHandler) ListIntersegmentTransactions(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.IntersegmentFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if periodID := r.URL.Query().Get("period_id"); periodID != "" {
		filter.FiscalPeriodID = common.ID(periodID)
	}

	if fromSegmentID := r.URL.Query().Get("from_segment_id"); fromSegmentID != "" {
		id := common.ID(fromSegmentID)
		filter.FromSegmentID = &id
	}

	if toSegmentID := r.URL.Query().Get("to_segment_id"); toSegmentID != "" {
		id := common.ID(toSegmentID)
		filter.ToSegmentID = &id
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	transactions, total, err := h.service.ListIntersegmentTransactions(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   transactions,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *SegmentHandler) EliminateIntersegmentTransaction(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	if err := h.service.EliminateIntersegmentTransaction(r.Context(), id); err != nil {
		h.logger.Error("failed to eliminate intersegment transaction", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"status": "eliminated"})
}

func (h *SegmentHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *SegmentHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}

type ReportHandler struct {
	service *service.ReportService
	logger  *zap.Logger
}

func NewReportHandler(svc *service.ReportService, logger *zap.Logger) *ReportHandler {
	return &ReportHandler{
		service: svc,
		logger:  logger,
	}
}

type GenerateReportRequest struct {
	ReportName          string  `json:"report_name"`
	FiscalPeriodID      string  `json:"fiscal_period_id"`
	FiscalYearID        string  `json:"fiscal_year_id"`
	AsOfDate            string  `json:"as_of_date"`
	SegmentType         string  `json:"segment_type"`
	HierarchyID         *string `json:"hierarchy_id,omitempty"`
	IncludeIntersegment bool    `json:"include_intersegment"`
	CurrencyCode        string  `json:"currency_code"`
}

func (h *ReportHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	var req GenerateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))
	userID := common.ID(auth.GetUserIDFromContext(r.Context()))

	asOfDate, err := time.Parse("2006-01-02", req.AsOfDate)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid as_of_date (use YYYY-MM-DD)")
		return
	}

	segmentType := domain.SegmentType(req.SegmentType)
	if !segmentType.IsValid() {
		h.writeError(w, http.StatusBadRequest, "invalid segment_type")
		return
	}

	var hierarchyID *common.ID
	if req.HierarchyID != nil && *req.HierarchyID != "" {
		id := common.ID(*req.HierarchyID)
		hierarchyID = &id
	}

	report, err := h.service.GenerateSegmentReport(r.Context(), service.GenerateSegmentReportRequest{
		EntityID:            entityID,
		ReportName:          req.ReportName,
		FiscalPeriodID:      common.ID(req.FiscalPeriodID),
		FiscalYearID:        common.ID(req.FiscalYearID),
		AsOfDate:            asOfDate,
		SegmentType:         segmentType,
		HierarchyID:         hierarchyID,
		IncludeIntersegment: req.IncludeIntersegment,
		CurrencyCode:        req.CurrencyCode,
		GeneratedBy:         userID,
	})
	if err != nil {
		h.logger.Error("failed to generate report", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, report)
}

func (h *ReportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	report, err := h.service.GetReport(r.Context(), id)
	if err == domain.ErrReportNotFound {
		h.writeError(w, http.StatusNotFound, "report not found")
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	entityID := common.ID(auth.GetEntityIDFromContext(r.Context()))

	filter := repository.ReportFilter{
		EntityID: entityID,
		Limit:    50,
		Offset:   0,
	}

	if periodID := r.URL.Query().Get("period_id"); periodID != "" {
		id := common.ID(periodID)
		filter.FiscalPeriodID = &id
	}

	if yearID := r.URL.Query().Get("year_id"); yearID != "" {
		id := common.ID(yearID)
		filter.FiscalYearID = &id
	}

	if segmentType := r.URL.Query().Get("segment_type"); segmentType != "" {
		t := domain.SegmentType(segmentType)
		filter.SegmentType = &t
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.ReportStatus(status)
		filter.Status = &s
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	reports, total, err := h.service.ListReports(r.Context(), filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"data":   reports,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *ReportHandler) FinalizeReport(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	report, err := h.service.FinalizeReport(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to finalize report", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) ApproveReport(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	report, err := h.service.ApproveReport(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to approve report", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) PublishReport(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	report, err := h.service.PublishReport(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to publish report", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) DeleteReport(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	if err := h.service.DeleteReport(r.Context(), id); err != nil {
		h.logger.Error("failed to delete report", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ReportHandler) RegenerateReportData(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	report, err := h.service.RegenerateReportData(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to regenerate report data", zap.Error(err))
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, report)
}

func (h *ReportHandler) GetReportSummary(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id := common.ID(idStr)

	summary, err := h.service.GetReportSummary(r.Context(), id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, summary)
}

func (h *ReportHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *ReportHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
