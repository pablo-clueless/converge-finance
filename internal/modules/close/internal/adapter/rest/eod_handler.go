package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/service"
	"converge-finance.com/m/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type EODHandler struct {
	logger     *zap.Logger
	eodService *service.EODService
}

func NewEODHandler(logger *zap.Logger, eodService *service.EODService) *EODHandler {
	return &EODHandler{
		logger:     logger,
		eodService: eodService,
	}
}

func (h *EODHandler) GetBusinessDate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	bd, err := h.eodService.GetBusinessDate(ctx, entityID)
	if err != nil {
		http.Error(w, "Business date not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, bd)
}

func (h *EODHandler) InitializeBusinessDate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	var req struct {
		InitialDate string `json:"initial_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	initialDate, err := time.Parse("2006-01-02", req.InitialDate)
	if err != nil {
		http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	bd, err := h.eodService.InitializeBusinessDate(ctx, entityID, initialDate)
	if err != nil {
		h.logger.Error("Failed to initialize business date", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, bd)
}

func (h *EODHandler) RolloverBusinessDate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))
	userID := common.ID(auth.GetUserIDFromContext(ctx))

	bd, err := h.eodService.RolloverBusinessDate(ctx, entityID, userID)
	if err != nil {
		h.logger.Error("Failed to rollover business date", zap.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, bd)
}

func (h *EODHandler) GetEODConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	config, err := h.eodService.GetEODConfig(ctx, entityID)
	if err != nil {
		config = domain.NewEODConfig(entityID)
	}

	respondJSON(w, http.StatusOK, config)
}

func (h *EODHandler) UpdateEODConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	var req struct {
		EODCutoffTime        string   `json:"eod_cutoff_time"`
		Timezone             string   `json:"timezone"`
		AutoRollover         bool     `json:"auto_rollover"`
		RequireZeroSuspense  bool     `json:"require_zero_suspense"`
		RequireBalancedBooks bool     `json:"require_balanced_books"`
		SkipWeekends         bool     `json:"skip_weekends"`
		SkipHolidays         bool     `json:"skip_holidays"`
		NotifyOnCompletion   bool     `json:"notify_on_completion"`
		NotifyOnFailure      bool     `json:"notify_on_failure"`
		NotificationEmails   []string `json:"notification_emails"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cutoffTime, err := time.Parse("15:04:05", req.EODCutoffTime)
	if err != nil {
		cutoffTime, _ = time.Parse("15:04", req.EODCutoffTime)
	}

	config := &domain.EODConfig{
		EntityID:             entityID,
		EODCutoffTime:        cutoffTime,
		Timezone:             req.Timezone,
		AutoRollover:         req.AutoRollover,
		RequireZeroSuspense:  req.RequireZeroSuspense,
		RequireBalancedBooks: req.RequireBalancedBooks,
		SkipWeekends:         req.SkipWeekends,
		SkipHolidays:         req.SkipHolidays,
		NotifyOnCompletion:   req.NotifyOnCompletion,
		NotifyOnFailure:      req.NotifyOnFailure,
		NotificationEmails:   req.NotificationEmails,
	}

	config, err = h.eodService.CreateOrUpdateEODConfig(ctx, config)
	if err != nil {
		h.logger.Error("Failed to update EOD config", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, config)
}

func (h *EODHandler) RunEOD(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))
	userID := common.ID(auth.GetUserIDFromContext(ctx))

	var req struct {
		BusinessDate string `json:"business_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var businessDate time.Time
	var err error

	if req.BusinessDate != "" {
		businessDate, err = time.Parse("2006-01-02", req.BusinessDate)
		if err != nil {
			http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
			return
		}
	} else {
		bd, err := h.eodService.GetBusinessDate(ctx, entityID)
		if err != nil {
			http.Error(w, "Business date not initialized", http.StatusBadRequest)
			return
		}
		businessDate = bd.CurrentBusinessDate
	}

	run, err := h.eodService.RunEOD(ctx, entityID, businessDate, userID)
	if err != nil {
		h.logger.Error("EOD run failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (h *EODHandler) GetEODRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := common.ID(chi.URLParam(r, "id"))

	run, err := h.eodService.GetEODRun(ctx, id)
	if err != nil {
		http.Error(w, "EOD run not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (h *EODHandler) ListEODRuns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	filter := domain.EODRunFilter{
		EntityID: &entityID,
		Limit:    50,
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.EODStatus(status)
		filter.Status = &s
	}

	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		if t, err := time.Parse("2006-01-02", fromDate); err == nil {
			filter.FromDate = &t
		}
	}

	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		if t, err := time.Parse("2006-01-02", toDate); err == nil {
			filter.ToDate = &t
		}
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	runs, err := h.eodService.ListEODRuns(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list EOD runs", zap.Error(err))
		http.Error(w, "Failed to list EOD runs", http.StatusInternalServerError)
		return
	}

	if runs == nil {
		runs = []domain.EODRun{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"runs":  runs,
		"total": len(runs),
	})
}

func (h *EODHandler) GetLatestEODRun(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	run, err := h.eodService.GetLatestEODRun(ctx, entityID)
	if err != nil {
		http.Error(w, "No EOD runs found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, run)
}

func (h *EODHandler) ListEODTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	filter := domain.EODTaskFilter{
		EntityID: &entityID,
		Limit:    100,
	}

	if active := r.URL.Query().Get("active"); active != "" {
		isActive := active == "true"
		filter.IsActive = &isActive
	}

	tasks, err := h.eodService.ListEODTasks(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list EOD tasks", zap.Error(err))
		http.Error(w, "Failed to list EOD tasks", http.StatusInternalServerError)
		return
	}

	if tasks == nil {
		tasks = []domain.EODTask{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (h *EODHandler) CreateEODTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	var req struct {
		TaskCode       string          `json:"task_code"`
		TaskName       string          `json:"task_name"`
		TaskType       string          `json:"task_type"`
		SequenceNumber int             `json:"sequence_number"`
		IsRequired     bool            `json:"is_required"`
		Configuration  json.RawMessage `json:"configuration"`
		Description    string          `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	taskType := domain.EODTaskType(req.TaskType)
	if !taskType.IsValid() {
		http.Error(w, "Invalid task type", http.StatusBadRequest)
		return
	}

	task := domain.NewEODTask(entityID, req.TaskCode, req.TaskName, taskType, req.SequenceNumber)
	task.IsRequired = req.IsRequired
	task.Description = req.Description
	if req.Configuration != nil {
		task.Configuration = req.Configuration
	}

	task, err := h.eodService.CreateEODTask(ctx, task)
	if err != nil {
		h.logger.Error("Failed to create EOD task", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

func (h *EODHandler) GetEODTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := common.ID(chi.URLParam(r, "id"))

	task, err := h.eodService.GetEODTask(ctx, id)
	if err != nil {
		http.Error(w, "EOD task not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, task)
}

func (h *EODHandler) UpdateEODTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := common.ID(chi.URLParam(r, "id"))

	task, err := h.eodService.GetEODTask(ctx, id)
	if err != nil {
		http.Error(w, "EOD task not found", http.StatusNotFound)
		return
	}

	var req struct {
		TaskName       string          `json:"task_name"`
		SequenceNumber int             `json:"sequence_number"`
		IsRequired     bool            `json:"is_required"`
		IsActive       bool            `json:"is_active"`
		Configuration  json.RawMessage `json:"configuration"`
		Description    string          `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	task.TaskName = req.TaskName
	task.SequenceNumber = req.SequenceNumber
	task.IsRequired = req.IsRequired
	task.IsActive = req.IsActive
	task.Description = req.Description
	if req.Configuration != nil {
		task.Configuration = req.Configuration
	}
	task.UpdatedAt = time.Now()

	task, err = h.eodService.UpdateEODTask(ctx, task)
	if err != nil {
		h.logger.Error("Failed to update EOD task", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, task)
}

func (h *EODHandler) DeleteEODTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := common.ID(chi.URLParam(r, "id"))

	if err := h.eodService.DeleteEODTask(ctx, id); err != nil {
		h.logger.Error("Failed to delete EOD task", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *EODHandler) InitializeDefaultTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	if err := h.eodService.InitializeDefaultTasks(ctx, entityID); err != nil {
		h.logger.Error("Failed to initialize default tasks", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{
		"status":  "success",
		"message": "Default EOD tasks created",
	})
}

func (h *EODHandler) ListHolidays(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	filter := domain.HolidayFilter{
		EntityID: &entityID,
		Limit:    100,
	}

	if year := r.URL.Query().Get("year"); year != "" {
		if y, err := strconv.Atoi(year); err == nil {
			filter.Year = &y
		}
	}

	holidays, err := h.eodService.ListHolidays(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list holidays", zap.Error(err))
		http.Error(w, "Failed to list holidays", http.StatusInternalServerError)
		return
	}

	if holidays == nil {
		holidays = []domain.Holiday{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"holidays": holidays,
		"total":    len(holidays),
	})
}

func (h *EODHandler) AddHoliday(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	var req struct {
		HolidayDate    string `json:"holiday_date"`
		HolidayName    string `json:"holiday_name"`
		HolidayType    string `json:"holiday_type"`
		IsRecurring    bool   `json:"is_recurring"`
		RecurringMonth int    `json:"recurring_month"`
		RecurringDay   int    `json:"recurring_day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	holidayDate, err := time.Parse("2006-01-02", req.HolidayDate)
	if err != nil {
		http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	holidayType := req.HolidayType
	if holidayType == "" {
		holidayType = "bank"
	}

	holiday := domain.NewHoliday(entityID, holidayDate, req.HolidayName, holidayType)
	if req.IsRecurring {
		holiday.SetRecurring(req.RecurringMonth, req.RecurringDay)
	}

	holiday, err = h.eodService.AddHoliday(ctx, holiday)
	if err != nil {
		h.logger.Error("Failed to add holiday", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, holiday)
}

func (h *EODHandler) RemoveHoliday(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := common.ID(chi.URLParam(r, "id"))

	if err := h.eodService.RemoveHoliday(ctx, id); err != nil {
		h.logger.Error("Failed to remove holiday", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *EODHandler) CheckHoliday(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	entityID := common.ID(auth.GetEntityIDFromContext(ctx))

	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		http.Error(w, "date query parameter required", http.StatusBadRequest)
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "Invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	isHoliday, err := h.eodService.IsHoliday(ctx, entityID, date)
	if err != nil {
		h.logger.Error("Failed to check holiday", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"date":       dateStr,
		"is_holiday": isHoliday,
	})
}
