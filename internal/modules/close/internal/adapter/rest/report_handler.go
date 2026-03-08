package rest

import (
	"net/http"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/service"
	"go.uber.org/zap"
)

type ReportHandler struct {
	*Handler
	reportService *service.ReportService
}

func NewReportHandler(logger *zap.Logger, svc *service.ReportService) *ReportHandler {
	return &ReportHandler{Handler: NewHandler(logger), reportService: svc}
}

type GenerateReportRequest struct {
	ReportType     string `json:"report_type"`
	FiscalPeriodID string `json:"fiscal_period_id"`
	FiscalYearID   string `json:"fiscal_year_id,omitempty"`
	AsOfDate       string `json:"as_of_date"`
}

type CreateTemplateRequest struct {
	TemplateCode  string                 `json:"template_code"`
	TemplateName  string                 `json:"template_name"`
	ReportType    string                 `json:"report_type"`
	ReportFormat  string                 `json:"report_format"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

type UpdateChecklistItemRequest struct {
	IsCompleted bool   `json:"is_completed"`
	Notes       string `json:"notes,omitempty"`
}

type ReportTemplateResponse struct {
	ID               common.ID `json:"id"`
	EntityID         *common.ID `json:"entity_id,omitempty"`
	TemplateCode     string    `json:"template_code"`
	TemplateName     string    `json:"template_name"`
	ReportType       string    `json:"report_type"`
	ReportFormat     string    `json:"report_format"`
	IsSystemTemplate bool      `json:"is_system_template"`
	IsActive         bool      `json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
}

type ReportRunResponse struct {
	ID             common.ID   `json:"id"`
	EntityID       common.ID   `json:"entity_id"`
	ReportNumber   string      `json:"report_number"`
	ReportType     string      `json:"report_type"`
	ReportFormat   string      `json:"report_format"`
	ReportName     string      `json:"report_name"`
	FiscalPeriodID *common.ID  `json:"fiscal_period_id,omitempty"`
	AsOfDate       string      `json:"as_of_date"`
	Status         string      `json:"status"`
	GeneratedAt    *time.Time  `json:"generated_at,omitempty"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	DataRows       []ReportDataRowResponse `json:"data_rows,omitempty"`
}

type ReportDataRowResponse struct {
	RowNumber    int        `json:"row_number"`
	RowType      string     `json:"row_type"`
	IndentLevel  int        `json:"indent_level"`
	AccountCode  string     `json:"account_code,omitempty"`
	AccountName  string     `json:"account_name,omitempty"`
	Description  string     `json:"description,omitempty"`
	Amount1      *float64   `json:"amount_1,omitempty"`
	Amount2      *float64   `json:"amount_2,omitempty"`
	Amount3      *float64   `json:"amount_3,omitempty"`
	CurrencyCode string     `json:"currency_code,omitempty"`
	IsBold       bool       `json:"is_bold"`
	IsUnderlined bool       `json:"is_underlined"`
}

type YearEndChecklistResponse struct {
	ID            common.ID                     `json:"id"`
	EntityID      common.ID                     `json:"entity_id"`
	FiscalYearID  common.ID                     `json:"fiscal_year_id"`
	ChecklistName string                        `json:"checklist_name"`
	Status        string                        `json:"status"`
	StartedAt     time.Time                     `json:"started_at"`
	CompletedAt   *time.Time                    `json:"completed_at,omitempty"`
	Items         []YearEndChecklistItemResponse `json:"items,omitempty"`
}

type YearEndChecklistItemResponse struct {
	ID             common.ID  `json:"id"`
	SequenceNumber int        `json:"sequence_number"`
	ItemCode       string     `json:"item_code"`
	ItemName       string     `json:"item_name"`
	Description    string     `json:"description,omitempty"`
	IsRequired     bool       `json:"is_required"`
	IsCompleted    bool       `json:"is_completed"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Notes          string     `json:"notes,omitempty"`
}

func (h *ReportHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	reportTypeStr := getStringQuery(r, "report_type")
	isSystemTemplate := getBoolQuery(r, "is_system_template")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.ReportTemplateFilter{
		EntityID:         &entityID,
		IsSystemTemplate: isSystemTemplate,
		Limit:            limit,
		Offset:           offset,
	}

	if reportTypeStr != nil {
		reportType := domain.ReportType(*reportTypeStr)
		filter.ReportType = &reportType
	}

	templates, err := h.reportService.ListReportTemplates(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list report templates", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ReportTemplateResponse, len(templates))
	for i, t := range templates {
		response[i] = toReportTemplateResponse(&t)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ReportHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid template ID")
		return
	}

	template, err := h.reportService.GetReportTemplate(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "report template not found")
		return
	}

	respondJSON(w, http.StatusOK, toReportTemplateResponse(template))
}

func (h *ReportHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req CreateTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)

	reportType := domain.ReportType(req.ReportType)
	if !reportType.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid report_type")
		return
	}

	reportFormat := domain.ReportFormat(req.ReportFormat)
	if !reportFormat.IsValid() {
		respondError(w, http.StatusBadRequest, "invalid report_format")
		return
	}

	template, err := h.reportService.CreateReportTemplate(r.Context(), entityID, req.TemplateCode, req.TemplateName, reportType, reportFormat, req.Configuration)
	if err != nil {
		h.logger.Error("failed to create report template", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toReportTemplateResponse(template))
}

func (h *ReportHandler) GenerateTrialBalance(w http.ResponseWriter, r *http.Request) {
	var req GenerateReportRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	userID := getUserID(r)

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	asOfDate, err := time.Parse("2006-01-02", req.AsOfDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid as_of_date")
		return
	}

	run, err := h.reportService.GenerateTrialBalance(r.Context(), entityID, fiscalPeriodID, asOfDate, userID)
	if err != nil {
		h.logger.Error("failed to generate trial balance", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toReportRunResponse(run, true))
}

func (h *ReportHandler) GenerateIncomeStatement(w http.ResponseWriter, r *http.Request) {
	var req GenerateReportRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	userID := getUserID(r)

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	fiscalYearID, err := common.Parse(req.FiscalYearID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_year_id")
		return
	}

	asOfDate, err := time.Parse("2006-01-02", req.AsOfDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid as_of_date")
		return
	}

	run, err := h.reportService.GenerateIncomeStatement(r.Context(), entityID, fiscalPeriodID, fiscalYearID, asOfDate, userID)
	if err != nil {
		h.logger.Error("failed to generate income statement", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toReportRunResponse(run, true))
}

func (h *ReportHandler) GenerateBalanceSheet(w http.ResponseWriter, r *http.Request) {
	var req GenerateReportRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entityID := getEntityID(r)
	userID := getUserID(r)

	fiscalPeriodID, err := common.Parse(req.FiscalPeriodID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_period_id")
		return
	}

	fiscalYearID, err := common.Parse(req.FiscalYearID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_year_id")
		return
	}

	asOfDate, err := time.Parse("2006-01-02", req.AsOfDate)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid as_of_date")
		return
	}

	run, err := h.reportService.GenerateBalanceSheet(r.Context(), entityID, fiscalPeriodID, fiscalYearID, asOfDate, userID)
	if err != nil {
		h.logger.Error("failed to generate balance sheet", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, toReportRunResponse(run, true))
}

func (h *ReportHandler) GetReportRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid report run ID")
		return
	}

	run, err := h.reportService.GetReportRun(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, "report run not found")
		return
	}

	respondJSON(w, http.StatusOK, toReportRunResponse(run, true))
}

func (h *ReportHandler) ListReportRuns(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	reportTypeStr := getStringQuery(r, "report_type")
	statusStr := getStringQuery(r, "status")
	limit := getIntQuery(r, "limit", 50)
	offset := getIntQuery(r, "offset", 0)

	filter := domain.ReportRunFilter{
		EntityID: &entityID,
		Limit:    limit,
		Offset:   offset,
	}

	if reportTypeStr != nil {
		reportType := domain.ReportType(*reportTypeStr)
		filter.ReportType = &reportType
	}

	if statusStr != nil {
		status := domain.ReportStatus(*statusStr)
		filter.Status = &status
	}

	runs, err := h.reportService.ListReportRuns(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list report runs", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]ReportRunResponse, len(runs))
	for i, run := range runs {
		response[i] = toReportRunResponse(&run, false)
	}

	respondJSON(w, http.StatusOK, response)
}

func (h *ReportHandler) DeleteReportRun(w http.ResponseWriter, r *http.Request) {
	id, err := getIDParam(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid report run ID")
		return
	}

	if err := h.reportService.DeleteReportRun(r.Context(), id); err != nil {
		h.logger.Error("failed to delete report run", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, SuccessResponse{Success: true, Message: "Report run deleted"})
}

func (h *ReportHandler) GetYearEndChecklist(w http.ResponseWriter, r *http.Request) {
	entityID := getEntityID(r)
	fiscalYearIDStr := r.URL.Query().Get("fiscal_year_id")

	if fiscalYearIDStr == "" {
		respondError(w, http.StatusBadRequest, "fiscal_year_id is required")
		return
	}

	fiscalYearID, err := common.Parse(fiscalYearIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid fiscal_year_id")
		return
	}

	checklist, err := h.reportService.GetYearEndChecklist(r.Context(), entityID, fiscalYearID)
	if err != nil {
		h.logger.Error("failed to get year-end checklist", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toYearEndChecklistResponse(checklist))
}

func (h *ReportHandler) UpdateChecklistItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := getIDParam(r, "itemId")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid item ID")
		return
	}

	var req UpdateChecklistItemRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := getUserID(r)

	item, err := h.reportService.UpdateChecklistItem(r.Context(), itemID, req.IsCompleted, userID, req.Notes)
	if err != nil {
		h.logger.Error("failed to update checklist item", zap.Error(err))
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, toYearEndChecklistItemResponse(item))
}

func toReportTemplateResponse(t *domain.ReportTemplate) ReportTemplateResponse {
	return ReportTemplateResponse{
		ID:               t.ID,
		EntityID:         t.EntityID,
		TemplateCode:     t.TemplateCode,
		TemplateName:     t.TemplateName,
		ReportType:       string(t.ReportType),
		ReportFormat:     string(t.ReportFormat),
		IsSystemTemplate: t.IsSystemTemplate,
		IsActive:         t.IsActive,
		CreatedAt:        t.CreatedAt,
	}
}

func toReportRunResponse(run *domain.ReportRun, includeData bool) ReportRunResponse {
	resp := ReportRunResponse{
		ID:             run.ID,
		EntityID:       run.EntityID,
		ReportNumber:   run.ReportNumber,
		ReportType:     string(run.ReportType),
		ReportFormat:   string(run.ReportFormat),
		ReportName:     run.ReportName,
		FiscalPeriodID: run.FiscalPeriodID,
		AsOfDate:       run.AsOfDate.Format("2006-01-02"),
		Status:         string(run.Status),
		GeneratedAt:    run.GeneratedAt,
		CompletedAt:    run.CompletedAt,
		CreatedAt:      run.CreatedAt,
	}

	if includeData && len(run.DataRows) > 0 {
		resp.DataRows = make([]ReportDataRowResponse, len(run.DataRows))
		for i, row := range run.DataRows {
			resp.DataRows[i] = ReportDataRowResponse{
				RowNumber:    row.RowNumber,
				RowType:      string(row.RowType),
				IndentLevel:  row.IndentLevel,
				AccountCode:  row.AccountCode,
				AccountName:  row.AccountName,
				Description:  row.Description,
				Amount1:      row.Amount1,
				Amount2:      row.Amount2,
				Amount3:      row.Amount3,
				CurrencyCode: row.CurrencyCode,
				IsBold:       row.IsBold,
				IsUnderlined: row.IsUnderlined,
			}
		}
	}

	return resp
}

func toYearEndChecklistResponse(c *domain.YearEndChecklist) YearEndChecklistResponse {
	resp := YearEndChecklistResponse{
		ID:            c.ID,
		EntityID:      c.EntityID,
		FiscalYearID:  c.FiscalYearID,
		ChecklistName: c.ChecklistName,
		Status:        c.Status,
		StartedAt:     c.StartedAt,
		CompletedAt:   c.CompletedAt,
	}

	if len(c.Items) > 0 {
		resp.Items = make([]YearEndChecklistItemResponse, len(c.Items))
		for i, item := range c.Items {
			resp.Items[i] = *toYearEndChecklistItemResponse(&item)
		}
	}

	return resp
}

func toYearEndChecklistItemResponse(item *domain.YearEndChecklistItem) *YearEndChecklistItemResponse {
	return &YearEndChecklistItemResponse{
		ID:             item.ID,
		SequenceNumber: item.SequenceNumber,
		ItemCode:       item.ItemCode,
		ItemName:       item.ItemName,
		Description:    item.Description,
		IsRequired:     item.IsRequired,
		IsCompleted:    item.IsCompleted,
		CompletedAt:    item.CompletedAt,
		Notes:          item.Notes,
	}
}
