package domain

import (
	"encoding/json"
	"errors"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type EODStatus string

const (
	EODStatusPending    EODStatus = "pending"
	EODStatusInProgress EODStatus = "in_progress"
	EODStatusCompleted  EODStatus = "completed"
	EODStatusFailed     EODStatus = "failed"
	EODStatusSkipped    EODStatus = "skipped"
)

func (s EODStatus) IsValid() bool {
	switch s {
	case EODStatusPending, EODStatusInProgress, EODStatusCompleted, EODStatusFailed, EODStatusSkipped:
		return true
	}
	return false
}

func (s EODStatus) String() string {
	return string(s)
}

type EODTaskType string

const (
	EODTaskValidateTransactions EODTaskType = "validate_transactions"
	EODTaskPostPendingBatches   EODTaskType = "post_pending_batches"
	EODTaskRunReconciliation    EODTaskType = "run_reconciliation"
	EODTaskCalculateAccruals    EODTaskType = "calculate_accruals"
	EODTaskFXRateUpdate         EODTaskType = "fx_rate_update"
	EODTaskGenerateDailyReports EODTaskType = "generate_daily_reports"
	EODTaskValidateBalances     EODTaskType = "validate_balances"
	EODTaskRolloverDate         EODTaskType = "rollover_date"
	EODTaskCustom               EODTaskType = "custom"
)

func (t EODTaskType) IsValid() bool {
	switch t {
	case EODTaskValidateTransactions, EODTaskPostPendingBatches, EODTaskRunReconciliation,
		EODTaskCalculateAccruals, EODTaskFXRateUpdate, EODTaskGenerateDailyReports,
		EODTaskValidateBalances, EODTaskRolloverDate, EODTaskCustom:
		return true
	}
	return false
}

func (t EODTaskType) String() string {
	return string(t)
}

type BusinessDate struct {
	ID                  common.ID
	EntityID            common.ID
	CurrentBusinessDate time.Time
	LastEODDate         *time.Time
	LastEODRunID        *common.ID
	NextBusinessDate    *time.Time
	IsHoliday           bool
	HolidayName         string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func NewBusinessDate(entityID common.ID, currentDate time.Time) *BusinessDate {
	now := time.Now()
	return &BusinessDate{
		ID:                  common.NewID(),
		EntityID:            entityID,
		CurrentBusinessDate: currentDate,
		IsHoliday:           false,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func (b *BusinessDate) Rollover(nextDate time.Time, eodRunID common.ID) {
	now := time.Now()
	currentDate := b.CurrentBusinessDate
	b.LastEODDate = &currentDate
	b.LastEODRunID = &eodRunID
	b.CurrentBusinessDate = nextDate
	b.UpdatedAt = now
}

type EODConfig struct {
	ID                   common.ID
	EntityID             common.ID
	EODCutoffTime        time.Time
	Timezone             string
	AutoRollover         bool
	RequireZeroSuspense  bool
	RequireBalancedBooks bool
	SkipWeekends         bool
	SkipHolidays         bool
	NotifyOnCompletion   bool
	NotifyOnFailure      bool
	NotificationEmails   []string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func NewEODConfig(entityID common.ID) *EODConfig {
	now := time.Now()
	cutoffTime, _ := time.Parse("15:04:05", "17:00:00")
	return &EODConfig{
		ID:                   common.NewID(),
		EntityID:             entityID,
		EODCutoffTime:        cutoffTime,
		Timezone:             "UTC",
		AutoRollover:         false,
		RequireZeroSuspense:  true,
		RequireBalancedBooks: true,
		SkipWeekends:         true,
		SkipHolidays:         true,
		NotifyOnCompletion:   true,
		NotifyOnFailure:      true,
		NotificationEmails:   []string{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

type EODRun struct {
	ID                    common.ID
	EntityID              common.ID
	RunNumber             string
	BusinessDate          time.Time
	Status                EODStatus
	StartedAt             *time.Time
	CompletedAt           *time.Time
	TotalTasks            int
	CompletedTasks        int
	FailedTasks           int
	SkippedTasks          int
	JournalEntriesPosted  int
	TransactionsValidated int
	Warnings              []string
	Errors                []string
	InitiatedBy           common.ID
	CreatedAt             time.Time
	UpdatedAt             time.Time
	TaskRuns              []EODTaskRun
}

func NewEODRun(entityID common.ID, businessDate time.Time, initiatedBy common.ID) *EODRun {
	now := time.Now()
	runNumber := "EOD" + businessDate.Format("20060102")
	return &EODRun{
		ID:           common.NewID(),
		EntityID:     entityID,
		RunNumber:    runNumber,
		BusinessDate: businessDate,
		Status:       EODStatusPending,
		Warnings:     []string{},
		Errors:       []string{},
		InitiatedBy:  initiatedBy,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func (r *EODRun) Start() error {
	if r.Status != EODStatusPending {
		return errors.New("EOD run must be pending to start")
	}
	now := time.Now()
	r.Status = EODStatusInProgress
	r.StartedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *EODRun) Complete() error {
	if r.Status != EODStatusInProgress {
		return errors.New("EOD run must be in progress to complete")
	}
	now := time.Now()
	r.Status = EODStatusCompleted
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *EODRun) Fail(errorMsg string) error {
	now := time.Now()
	r.Status = EODStatusFailed
	r.Errors = append(r.Errors, errorMsg)
	r.CompletedAt = &now
	r.UpdatedAt = now
	return nil
}

func (r *EODRun) AddWarning(warning string) {
	r.Warnings = append(r.Warnings, warning)
	r.UpdatedAt = time.Now()
}

func (r *EODRun) IncrementTaskCount(completed, failed, skipped int) {
	r.CompletedTasks += completed
	r.FailedTasks += failed
	r.SkippedTasks += skipped
	r.UpdatedAt = time.Now()
}

func (r *EODRun) SetTotalTasks(total int) {
	r.TotalTasks = total
	r.UpdatedAt = time.Now()
}

type EODTask struct {
	ID             common.ID
	EntityID       common.ID
	TaskCode       string
	TaskName       string
	TaskType       EODTaskType
	SequenceNumber int
	IsRequired     bool
	IsActive       bool
	Configuration  json.RawMessage
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewEODTask(entityID common.ID, taskCode, taskName string, taskType EODTaskType, sequence int) *EODTask {
	now := time.Now()
	return &EODTask{
		ID:             common.NewID(),
		EntityID:       entityID,
		TaskCode:       taskCode,
		TaskName:       taskName,
		TaskType:       taskType,
		SequenceNumber: sequence,
		IsRequired:     true,
		IsActive:       true,
		Configuration:  json.RawMessage("{}"),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (t *EODTask) SetConfiguration(config json.RawMessage) {
	t.Configuration = config
	t.UpdatedAt = time.Now()
}

func (t *EODTask) Activate() {
	t.IsActive = true
	t.UpdatedAt = time.Now()
}

func (t *EODTask) Deactivate() {
	t.IsActive = false
	t.UpdatedAt = time.Now()
}

type EODTaskRun struct {
	ID               common.ID
	EODRunID         common.ID
	EODTaskID        common.ID
	TaskCode         string
	TaskName         string
	SequenceNumber   int
	Status           EODStatus
	StartedAt        *time.Time
	CompletedAt      *time.Time
	DurationMs       int
	RecordsProcessed int
	RecordsFailed    int
	ResultSummary    json.RawMessage
	ErrorMessage     string
	Warnings         []string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewEODTaskRun(eodRunID common.ID, task *EODTask) *EODTaskRun {
	now := time.Now()
	return &EODTaskRun{
		ID:             common.NewID(),
		EODRunID:       eodRunID,
		EODTaskID:      task.ID,
		TaskCode:       task.TaskCode,
		TaskName:       task.TaskName,
		SequenceNumber: task.SequenceNumber,
		Status:         EODStatusPending,
		ResultSummary:  json.RawMessage("{}"),
		Warnings:       []string{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (tr *EODTaskRun) Start() error {
	if tr.Status != EODStatusPending {
		return errors.New("task run must be pending to start")
	}
	now := time.Now()
	tr.Status = EODStatusInProgress
	tr.StartedAt = &now
	tr.UpdatedAt = now
	return nil
}

func (tr *EODTaskRun) Complete(recordsProcessed, recordsFailed int, summary json.RawMessage) error {
	if tr.Status != EODStatusInProgress {
		return errors.New("task run must be in progress to complete")
	}
	now := time.Now()
	tr.Status = EODStatusCompleted
	tr.CompletedAt = &now
	tr.RecordsProcessed = recordsProcessed
	tr.RecordsFailed = recordsFailed
	tr.ResultSummary = summary
	if tr.StartedAt != nil {
		tr.DurationMs = int(now.Sub(*tr.StartedAt).Milliseconds())
	}
	tr.UpdatedAt = now
	return nil
}

func (tr *EODTaskRun) Fail(errorMsg string) error {
	now := time.Now()
	tr.Status = EODStatusFailed
	tr.ErrorMessage = errorMsg
	tr.CompletedAt = &now
	if tr.StartedAt != nil {
		tr.DurationMs = int(now.Sub(*tr.StartedAt).Milliseconds())
	}
	tr.UpdatedAt = now
	return nil
}

func (tr *EODTaskRun) Skip(reason string) error {
	now := time.Now()
	tr.Status = EODStatusSkipped
	tr.ErrorMessage = reason
	tr.CompletedAt = &now
	tr.UpdatedAt = now
	return nil
}

func (tr *EODTaskRun) AddWarning(warning string) {
	tr.Warnings = append(tr.Warnings, warning)
	tr.UpdatedAt = time.Now()
}

type Holiday struct {
	ID             common.ID
	EntityID       common.ID
	HolidayDate    time.Time
	HolidayName    string
	HolidayType    string
	IsRecurring    bool
	RecurringMonth *int
	RecurringDay   *int
	CreatedAt      time.Time
}

func NewHoliday(entityID common.ID, date time.Time, name, holidayType string) *Holiday {
	return &Holiday{
		ID:          common.NewID(),
		EntityID:    entityID,
		HolidayDate: date,
		HolidayName: name,
		HolidayType: holidayType,
		IsRecurring: false,
		CreatedAt:   time.Now(),
	}
}

func (h *Holiday) SetRecurring(month, day int) {
	h.IsRecurring = true
	h.RecurringMonth = &month
	h.RecurringDay = &day
}

type DailyReconciliation struct {
	ID                  common.ID
	EODRunID            common.ID
	AccountID           common.ID
	AccountCode         string
	AccountName         string
	ExpectedBalance     float64
	ActualBalance       float64
	Difference          float64
	CurrencyCode        string
	IsReconciled        bool
	ReconciliationNotes string
	CreatedAt           time.Time
}

func NewDailyReconciliation(
	eodRunID, accountID common.ID,
	accountCode, accountName string,
	expected, actual float64,
	currency string,
) *DailyReconciliation {
	diff := actual - expected
	return &DailyReconciliation{
		ID:              common.NewID(),
		EODRunID:        eodRunID,
		AccountID:       accountID,
		AccountCode:     accountCode,
		AccountName:     accountName,
		ExpectedBalance: expected,
		ActualBalance:   actual,
		Difference:      diff,
		CurrencyCode:    currency,
		IsReconciled:    diff == 0,
		CreatedAt:       time.Now(),
	}
}

type EODRunFilter struct {
	EntityID     *common.ID
	BusinessDate *time.Time
	Status       *EODStatus
	FromDate     *time.Time
	ToDate       *time.Time
	Limit        int
	Offset       int
}

type EODTaskFilter struct {
	EntityID *common.ID
	TaskType *EODTaskType
	IsActive *bool
	Limit    int
	Offset   int
}

type HolidayFilter struct {
	EntityID *common.ID
	Year     *int
	Limit    int
	Offset   int
}
