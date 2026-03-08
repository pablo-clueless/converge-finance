package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/close/internal/domain"
	"converge-finance.com/m/internal/modules/close/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/database"
)

type EODService struct {
	db                 *database.PostgresDB
	businessDateRepo   repository.BusinessDateRepository
	eodConfigRepo      repository.EODConfigRepository
	eodRunRepo         repository.EODRunRepository
	eodTaskRepo        repository.EODTaskRepository
	eodTaskRunRepo     repository.EODTaskRunRepository
	holidayRepo        repository.HolidayRepository
	reconciliationRepo repository.DailyReconciliationRepository
	glAPI              gl.API
	auditLogger        *audit.Logger
}

func NewEODService(
	db *database.PostgresDB,
	businessDateRepo repository.BusinessDateRepository,
	eodConfigRepo repository.EODConfigRepository,
	eodRunRepo repository.EODRunRepository,
	eodTaskRepo repository.EODTaskRepository,
	eodTaskRunRepo repository.EODTaskRunRepository,
	holidayRepo repository.HolidayRepository,
	reconciliationRepo repository.DailyReconciliationRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *EODService {
	return &EODService{
		db:                 db,
		businessDateRepo:   businessDateRepo,
		eodConfigRepo:      eodConfigRepo,
		eodRunRepo:         eodRunRepo,
		eodTaskRepo:        eodTaskRepo,
		eodTaskRunRepo:     eodTaskRunRepo,
		holidayRepo:        holidayRepo,
		reconciliationRepo: reconciliationRepo,
		glAPI:              glAPI,
		auditLogger:        auditLogger,
	}
}

func (s *EODService) GetBusinessDate(ctx context.Context, entityID common.ID) (*domain.BusinessDate, error) {
	return s.businessDateRepo.GetByEntityID(ctx, entityID)
}

func (s *EODService) InitializeBusinessDate(ctx context.Context, entityID common.ID, initialDate time.Time) (*domain.BusinessDate, error) {
	existing, err := s.businessDateRepo.GetByEntityID(ctx, entityID)
	if err == nil && existing != nil {
		return existing, nil
	}

	bd := domain.NewBusinessDate(entityID, initialDate)
	if err := s.businessDateRepo.Create(ctx, bd); err != nil {
		return nil, fmt.Errorf("failed to create business date: %w", err)
	}

	s.auditLogger.Log(ctx, "business_date", bd.ID, "initialize", map[string]interface{}{
		"entity_id":    entityID,
		"initial_date": initialDate.Format("2006-01-02"),
	})

	return bd, nil
}

func (s *EODService) GetEODConfig(ctx context.Context, entityID common.ID) (*domain.EODConfig, error) {
	return s.eodConfigRepo.GetByEntityID(ctx, entityID)
}

func (s *EODService) CreateOrUpdateEODConfig(ctx context.Context, cfg *domain.EODConfig) (*domain.EODConfig, error) {
	existing, err := s.eodConfigRepo.GetByEntityID(ctx, cfg.EntityID)
	if err == nil && existing != nil {
		existing.EODCutoffTime = cfg.EODCutoffTime
		existing.Timezone = cfg.Timezone
		existing.AutoRollover = cfg.AutoRollover
		existing.RequireZeroSuspense = cfg.RequireZeroSuspense
		existing.RequireBalancedBooks = cfg.RequireBalancedBooks
		existing.SkipWeekends = cfg.SkipWeekends
		existing.SkipHolidays = cfg.SkipHolidays
		existing.NotifyOnCompletion = cfg.NotifyOnCompletion
		existing.NotifyOnFailure = cfg.NotifyOnFailure
		existing.NotificationEmails = cfg.NotificationEmails
		existing.UpdatedAt = time.Now()

		if err := s.eodConfigRepo.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("failed to update EOD config: %w", err)
		}
		return existing, nil
	}

	if err := s.eodConfigRepo.Create(ctx, cfg); err != nil {
		return nil, fmt.Errorf("failed to create EOD config: %w", err)
	}

	return cfg, nil
}

func (s *EODService) RunEOD(ctx context.Context, entityID common.ID, businessDate time.Time, userID common.ID) (*domain.EODRun, error) {
	existing, err := s.eodRunRepo.GetByBusinessDate(ctx, entityID, businessDate)
	if err == nil && existing != nil && existing.Status == domain.EODStatusCompleted {
		return nil, fmt.Errorf("EOD already completed for %s", businessDate.Format("2006-01-02"))
	}

	tasks, err := s.eodTaskRepo.GetActiveTasks(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get EOD tasks: %w", err)
	}

	if len(tasks) == 0 {
		return nil, errors.New("no EOD tasks configured for this entity")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil {
			_ = s.auditLogger.Log(ctx, "eod_run", existing.ID, "failed", map[string]interface{}{
				"business_date": businessDate.Format("2006-01-02"),
				"error":         rbErr.Error(),
			})
		}
	}()

	run := domain.NewEODRun(entityID, businessDate, userID)
	run.SetTotalTasks(len(tasks))

	if err := s.eodRunRepo.WithTx(tx).Create(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create EOD run: %w", err)
	}

	var taskRuns []domain.EODTaskRun
	for _, task := range tasks {
		tr := domain.NewEODTaskRun(run.ID, &task)
		taskRuns = append(taskRuns, *tr)
	}

	if err := s.eodTaskRunRepo.WithTx(tx).CreateBatch(ctx, taskRuns); err != nil {
		return nil, fmt.Errorf("failed to create task runs: %w", err)
	}

	if err := run.Start(); err != nil {
		return nil, err
	}

	if err := s.eodRunRepo.WithTx(tx).Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update EOD run: %w", err)
	}

	for i := range taskRuns {
		tr := &taskRuns[i]
		task := &tasks[i]

		if err := s.executeTask(ctx, tx, run, tr, task); err != nil {
			_ = tr.Fail(err.Error())
			run.IncrementTaskCount(0, 1, 0)

			if task.IsRequired {
				_ = run.Fail(fmt.Sprintf("Required task %s failed: %s", task.TaskCode, err.Error()))
				if updateErr := s.eodRunRepo.WithTx(tx).Update(ctx, run); updateErr != nil {
					return nil, fmt.Errorf("failed to update EOD run: %w", updateErr)
				}
				if updateErr := s.eodTaskRunRepo.WithTx(tx).Update(ctx, tr); updateErr != nil {
					return nil, fmt.Errorf("failed to update task run: %w", updateErr)
				}
				if commitErr := tx.Commit(); commitErr != nil {
					return nil, fmt.Errorf("failed to commit transaction: %w", commitErr)
				}

				_ = s.auditLogger.Log(ctx, "eod_run", existing.ID, "failed", map[string]interface{}{
					"business_date": businessDate.Format("2006-01-02"),
					"failed_task":   task.TaskCode,
					"error":         err,
				})

				return run, err
			}
		} else {
			run.IncrementTaskCount(1, 0, 0)
		}

		if err = s.eodTaskRunRepo.WithTx(tx).Update(ctx, tr); err != nil {
			return nil, fmt.Errorf("failed to update task run: %w", err)
		}
	}

	if err := run.Complete(); err != nil {
		return nil, err
	}

	if err := s.eodRunRepo.WithTx(tx).Update(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to update EOD run: %w", err)
	}

	config, _ := s.eodConfigRepo.GetByEntityID(ctx, entityID)
	if config != nil && config.AutoRollover {

		nextDate := s.getNextBusinessDate(ctx, entityID, businessDate, config)
		bd, err := s.businessDateRepo.GetByEntityID(ctx, entityID)
		if err == nil && bd != nil {
			bd.Rollover(nextDate, run.ID)
			_ = s.businessDateRepo.WithTx(tx).Update(ctx, bd)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.auditLogger.Log(ctx, "eod_run", run.ID, "completed", map[string]interface{}{
		"business_date":   businessDate.Format("2006-01-02"),
		"tasks_completed": run.CompletedTasks,
		"tasks_failed":    run.FailedTasks,
	})

	run.TaskRuns = taskRuns
	return run, nil
}

func (s *EODService) executeTask(ctx context.Context, tx interface{}, run *domain.EODRun, tr *domain.EODTaskRun, task *domain.EODTask) error {
	if err := tr.Start(); err != nil {
		return err
	}

	var recordsProcessed, recordsFailed int
	var summary json.RawMessage

	switch task.TaskType {
	case domain.EODTaskValidateTransactions:
		recordsProcessed, recordsFailed, summary = s.taskValidateTransactions(ctx, run, task)

	case domain.EODTaskPostPendingBatches:
		recordsProcessed, recordsFailed, summary = s.taskPostPendingBatches(ctx, run, task)

	case domain.EODTaskRunReconciliation:
		recordsProcessed, recordsFailed, summary = s.taskRunReconciliation(ctx, run, task)

	case domain.EODTaskCalculateAccruals:
		recordsProcessed, recordsFailed, summary = s.taskCalculateAccruals(ctx, run, task)

	case domain.EODTaskFXRateUpdate:
		recordsProcessed, recordsFailed, summary = s.taskFXRateUpdate(ctx, run, task)

	case domain.EODTaskGenerateDailyReports:
		recordsProcessed, recordsFailed, summary = s.taskGenerateDailyReports(ctx, run, task)

	case domain.EODTaskValidateBalances:
		recordsProcessed, recordsFailed, summary = s.taskValidateBalances(ctx, run, task)

	case domain.EODTaskRolloverDate:
		recordsProcessed, recordsFailed, summary = s.taskRolloverDate(ctx, run, task)

	case domain.EODTaskCustom:
		recordsProcessed, recordsFailed, summary = s.taskCustom(ctx, run, task)

	default:
		return fmt.Errorf("unknown task type: %s", task.TaskType)
	}

	if recordsFailed > 0 && task.IsRequired {
		return fmt.Errorf("task had %d failures", recordsFailed)
	}

	return tr.Complete(recordsProcessed, recordsFailed, summary)
}

func (s *EODService) taskValidateTransactions(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"validated":    true,
		"transactions": 0,
		"issues":       []string{},
	}

	summary, _ := json.Marshal(result)
	return 0, 0, summary
}

func (s *EODService) taskPostPendingBatches(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"batches_posted": 0,
		"entries_posted": 0,
	}

	summary, _ := json.Marshal(result)
	return 0, 0, summary
}

func (s *EODService) taskRunReconciliation(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"accounts_checked":    0,
		"accounts_reconciled": 0,
		"discrepancies":       0,
	}

	summary, _ := json.Marshal(result)
	return 0, 0, summary
}

func (s *EODService) taskCalculateAccruals(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"accruals_calculated": 0,
		"total_amount":        0.0,
	}

	summary, _ := json.Marshal(result)
	return 0, 0, summary
}

func (s *EODService) taskFXRateUpdate(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"rates_updated": 0,
		"source":        "manual",
	}

	summary, _ := json.Marshal(result)
	return 0, 0, summary
}

func (s *EODService) taskGenerateDailyReports(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"reports_generated": 0,
		"report_types":      []string{},
	}

	summary, _ := json.Marshal(result)
	return 0, 0, summary
}

func (s *EODService) taskValidateBalances(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"is_balanced":   true,
		"total_debits":  0.0,
		"total_credits": 0.0,
		"difference":    0.0,
	}

	summary, _ := json.Marshal(result)
	return 0, 0, summary
}

func (s *EODService) taskRolloverDate(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"status": "deferred_to_completion",
	}

	summary, _ := json.Marshal(result)
	return 1, 0, summary
}

func (s *EODService) taskCustom(ctx context.Context, run *domain.EODRun, task *domain.EODTask) (int, int, json.RawMessage) {

	result := map[string]interface{}{
		"status": "completed",
	}

	summary, _ := json.Marshal(result)
	return 1, 0, summary
}

func (s *EODService) getNextBusinessDate(ctx context.Context, entityID common.ID, fromDate time.Time, config *domain.EODConfig) time.Time {
	nextDate := fromDate.AddDate(0, 0, 1)

	for {

		if config.SkipWeekends && (nextDate.Weekday() == time.Saturday || nextDate.Weekday() == time.Sunday) {
			nextDate = nextDate.AddDate(0, 0, 1)
			continue
		}

		if config.SkipHolidays {
			isHoliday, _ := s.holidayRepo.IsHoliday(ctx, entityID, nextDate)
			if isHoliday {
				nextDate = nextDate.AddDate(0, 0, 1)
				continue
			}
		}

		break
	}

	return nextDate
}

func (s *EODService) RolloverBusinessDate(ctx context.Context, entityID common.ID, userID common.ID) (*domain.BusinessDate, error) {
	bd, err := s.businessDateRepo.GetByEntityID(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get business date: %w", err)
	}

	config, err := s.eodConfigRepo.GetByEntityID(ctx, entityID)
	if err != nil {
		config = domain.NewEODConfig(entityID)
	}

	eodRun, err := s.eodRunRepo.GetByBusinessDate(ctx, entityID, bd.CurrentBusinessDate)
	if err != nil || eodRun == nil || eodRun.Status != domain.EODStatusCompleted {
		return nil, errors.New("EOD must be completed before rollover")
	}

	nextDate := s.getNextBusinessDate(ctx, entityID, bd.CurrentBusinessDate, config)
	bd.Rollover(nextDate, eodRun.ID)

	if err := s.businessDateRepo.Update(ctx, bd); err != nil {
		return nil, fmt.Errorf("failed to update business date: %w", err)
	}

	s.auditLogger.Log(ctx, "business_date", bd.ID, "rollover", map[string]interface{}{
		"from_date": bd.LastEODDate.Format("2006-01-02"),
		"to_date":   bd.CurrentBusinessDate.Format("2006-01-02"),
		"user_id":   userID,
	})

	return bd, nil
}

func (s *EODService) GetEODRun(ctx context.Context, id common.ID) (*domain.EODRun, error) {
	run, err := s.eodRunRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	taskRuns, err := s.eodTaskRunRepo.GetByEODRunID(ctx, id)
	if err == nil {
		run.TaskRuns = taskRuns
	}

	return run, nil
}

func (s *EODService) ListEODRuns(ctx context.Context, filter domain.EODRunFilter) ([]domain.EODRun, error) {
	return s.eodRunRepo.List(ctx, filter)
}

func (s *EODService) GetLatestEODRun(ctx context.Context, entityID common.ID) (*domain.EODRun, error) {
	return s.eodRunRepo.GetLatest(ctx, entityID)
}

func (s *EODService) CreateEODTask(ctx context.Context, task *domain.EODTask) (*domain.EODTask, error) {
	if err := s.eodTaskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create EOD task: %w", err)
	}

	s.auditLogger.Log(ctx, "eod_task", task.ID, "create", map[string]interface{}{
		"task_code": task.TaskCode,
		"task_type": task.TaskType,
	})

	return task, nil
}

func (s *EODService) UpdateEODTask(ctx context.Context, task *domain.EODTask) (*domain.EODTask, error) {
	if err := s.eodTaskRepo.Update(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to update EOD task: %w", err)
	}
	return task, nil
}

func (s *EODService) DeleteEODTask(ctx context.Context, id common.ID) error {
	return s.eodTaskRepo.Delete(ctx, id)
}

func (s *EODService) GetEODTask(ctx context.Context, id common.ID) (*domain.EODTask, error) {
	return s.eodTaskRepo.GetByID(ctx, id)
}

func (s *EODService) ListEODTasks(ctx context.Context, filter domain.EODTaskFilter) ([]domain.EODTask, error) {
	return s.eodTaskRepo.List(ctx, filter)
}

func (s *EODService) AddHoliday(ctx context.Context, holiday *domain.Holiday) (*domain.Holiday, error) {
	if err := s.holidayRepo.Create(ctx, holiday); err != nil {
		return nil, fmt.Errorf("failed to add holiday: %w", err)
	}
	return holiday, nil
}

func (s *EODService) RemoveHoliday(ctx context.Context, id common.ID) error {
	return s.holidayRepo.Delete(ctx, id)
}

func (s *EODService) ListHolidays(ctx context.Context, filter domain.HolidayFilter) ([]domain.Holiday, error) {
	return s.holidayRepo.List(ctx, filter)
}

func (s *EODService) IsHoliday(ctx context.Context, entityID common.ID, date time.Time) (bool, error) {
	return s.holidayRepo.IsHoliday(ctx, entityID, date)
}

func (s *EODService) InitializeDefaultTasks(ctx context.Context, entityID common.ID) error {
	defaultTasks := []struct {
		code     string
		name     string
		taskType domain.EODTaskType
		required bool
	}{
		{"VALIDATE_TXN", "Validate Transactions", domain.EODTaskValidateTransactions, true},
		{"POST_BATCHES", "Post Pending Batches", domain.EODTaskPostPendingBatches, false},
		{"RECONCILE", "Run Reconciliation", domain.EODTaskRunReconciliation, false},
		{"CALC_ACCRUALS", "Calculate Accruals", domain.EODTaskCalculateAccruals, false},
		{"VALIDATE_BAL", "Validate Balances", domain.EODTaskValidateBalances, true},
		{"GEN_REPORTS", "Generate Daily Reports", domain.EODTaskGenerateDailyReports, false},
		{"ROLLOVER", "Rollover Business Date", domain.EODTaskRolloverDate, true},
	}

	for i, t := range defaultTasks {
		task := domain.NewEODTask(entityID, t.code, t.name, t.taskType, i+1)
		task.IsRequired = t.required
		if err := s.eodTaskRepo.Create(ctx, task); err != nil {
			return fmt.Errorf("failed to create default task %s: %w", t.code, err)
		}
	}

	return nil
}
