package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/workflow/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/shopspring/decimal"
)

// WorkflowRepository defines the interface for workflow persistence
type WorkflowRepository interface {
	Create(ctx context.Context, workflow *domain.Workflow) error
	Update(ctx context.Context, workflow *domain.Workflow) error
	GetByID(ctx context.Context, id common.ID) (*domain.Workflow, error)
	GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Workflow, error)
	GetActiveByDocumentType(ctx context.Context, entityID common.ID, documentType string) (*domain.Workflow, error)
	List(ctx context.Context, filter WorkflowFilter) ([]domain.Workflow, int, error)
	Delete(ctx context.Context, id common.ID) error
}

// WorkflowStepRepository defines the interface for workflow step persistence
type WorkflowStepRepository interface {
	Create(ctx context.Context, step *domain.WorkflowStep) error
	Update(ctx context.Context, step *domain.WorkflowStep) error
	GetByID(ctx context.Context, id common.ID) (*domain.WorkflowStep, error)
	GetByWorkflowID(ctx context.Context, workflowID common.ID) ([]domain.WorkflowStep, error)
	GetByWorkflowAndNumber(ctx context.Context, workflowID common.ID, stepNumber int) (*domain.WorkflowStep, error)
	Delete(ctx context.Context, id common.ID) error
	DeleteByWorkflowID(ctx context.Context, workflowID common.ID) error
}

// DelegationRepository defines the interface for delegation persistence
type DelegationRepository interface {
	Create(ctx context.Context, delegation *domain.Delegation) error
	Update(ctx context.Context, delegation *domain.Delegation) error
	GetByID(ctx context.Context, id common.ID) (*domain.Delegation, error)
	GetActiveDelegations(ctx context.Context, entityID, delegatorID common.ID) ([]domain.Delegation, error)
	GetEffectiveDelegateFor(ctx context.Context, entityID, delegatorID common.ID, documentType string, workflowID *common.ID) (*domain.Delegation, error)
	ListByEntity(ctx context.Context, entityID common.ID, activeOnly bool) ([]domain.Delegation, error)
	Delete(ctx context.Context, id common.ID) error
}

// WorkflowFilter contains filter criteria for listing workflows
type WorkflowFilter struct {
	EntityID     common.ID
	DocumentType string
	Status       *domain.WorkflowStatus
	CurrentOnly  bool
	Limit        int
	Offset       int
}

// rowScanner interface for scanning database rows
type rowScanner interface {
	Scan(dest ...any) error
}

// PostgresWorkflowRepo implements WorkflowRepository
type PostgresWorkflowRepo struct {
	db *database.PostgresDB
}

// NewPostgresWorkflowRepo creates a new PostgresWorkflowRepo
func NewPostgresWorkflowRepo(db *database.PostgresDB) *PostgresWorkflowRepo {
	return &PostgresWorkflowRepo{db: db}
}

func (r *PostgresWorkflowRepo) Create(ctx context.Context, workflow *domain.Workflow) error {
	configJSON, err := json.Marshal(workflow.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	query := `
		INSERT INTO workflow.workflows (
			id, entity_id, workflow_code, workflow_name, description,
			document_type, status, version, is_current, configuration,
			created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = r.db.ExecContext(ctx, query,
		workflow.ID,
		workflow.EntityID,
		workflow.WorkflowCode,
		workflow.WorkflowName,
		workflow.Description,
		workflow.DocumentType,
		workflow.Status,
		workflow.Version,
		workflow.IsCurrent,
		configJSON,
		workflow.CreatedBy,
		workflow.CreatedAt,
		workflow.UpdatedAt,
	)

	return err
}

func (r *PostgresWorkflowRepo) Update(ctx context.Context, workflow *domain.Workflow) error {
	configJSON, err := json.Marshal(workflow.Configuration)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	query := `
		UPDATE workflow.workflows SET
			workflow_name = $2,
			description = $3,
			status = $4,
			is_current = $5,
			configuration = $6,
			updated_at = $7
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		workflow.ID,
		workflow.WorkflowName,
		workflow.Description,
		workflow.Status,
		workflow.IsCurrent,
		configJSON,
		workflow.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrWorkflowNotFound
	}

	return nil
}

func (r *PostgresWorkflowRepo) GetByID(ctx context.Context, id common.ID) (*domain.Workflow, error) {
	query := `
		SELECT id, entity_id, workflow_code, workflow_name, description,
			   document_type, status, version, is_current, configuration,
			   created_by, created_at, updated_at
		FROM workflow.workflows
		WHERE id = $1
	`

	return r.scanWorkflow(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresWorkflowRepo) GetByCode(ctx context.Context, entityID common.ID, code string) (*domain.Workflow, error) {
	query := `
		SELECT id, entity_id, workflow_code, workflow_name, description,
			   document_type, status, version, is_current, configuration,
			   created_by, created_at, updated_at
		FROM workflow.workflows
		WHERE entity_id = $1 AND workflow_code = $2 AND is_current = true
	`

	return r.scanWorkflow(r.db.QueryRowContext(ctx, query, entityID, code))
}

func (r *PostgresWorkflowRepo) GetActiveByDocumentType(ctx context.Context, entityID common.ID, documentType string) (*domain.Workflow, error) {
	query := `
		SELECT id, entity_id, workflow_code, workflow_name, description,
			   document_type, status, version, is_current, configuration,
			   created_by, created_at, updated_at
		FROM workflow.workflows
		WHERE entity_id = $1 AND document_type = $2 AND status = 'active' AND is_current = true
		LIMIT 1
	`

	return r.scanWorkflow(r.db.QueryRowContext(ctx, query, entityID, documentType))
}

func (r *PostgresWorkflowRepo) List(ctx context.Context, filter WorkflowFilter) ([]domain.Workflow, int, error) {
	baseQuery := `FROM workflow.workflows WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if filter.DocumentType != "" {
		baseQuery += fmt.Sprintf(` AND document_type = $%d`, argIdx)
		args = append(args, filter.DocumentType)
		argIdx++
	}
	if filter.Status != nil {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.CurrentOnly {
		baseQuery += ` AND is_current = true`
	}

	// Count query
	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query
	dataQuery := `
		SELECT id, entity_id, workflow_code, workflow_name, description,
			   document_type, status, version, is_current, configuration,
			   created_by, created_at, updated_at
		` + baseQuery + ` ORDER BY workflow_code, version DESC`

	if filter.Limit > 0 {
		dataQuery += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		dataQuery += fmt.Sprintf(` OFFSET $%d`, argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var workflows []domain.Workflow
	for rows.Next() {
		workflow, err := r.scanWorkflowRow(rows)
		if err != nil {
			return nil, 0, err
		}
		workflows = append(workflows, *workflow)
	}

	return workflows, total, rows.Err()
}

func (r *PostgresWorkflowRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM workflow.workflows WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrWorkflowNotFound
	}

	return nil
}

func (r *PostgresWorkflowRepo) scanWorkflow(row rowScanner) (*domain.Workflow, error) {
	var workflow domain.Workflow
	var configJSON []byte

	err := row.Scan(
		&workflow.ID,
		&workflow.EntityID,
		&workflow.WorkflowCode,
		&workflow.WorkflowName,
		&workflow.Description,
		&workflow.DocumentType,
		&workflow.Status,
		&workflow.Version,
		&workflow.IsCurrent,
		&configJSON,
		&workflow.CreatedBy,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &workflow.Configuration); err != nil {
		workflow.Configuration = make(map[string]any)
	}

	return &workflow, nil
}

func (r *PostgresWorkflowRepo) scanWorkflowRow(rows *sql.Rows) (*domain.Workflow, error) {
	var workflow domain.Workflow
	var configJSON []byte

	err := rows.Scan(
		&workflow.ID,
		&workflow.EntityID,
		&workflow.WorkflowCode,
		&workflow.WorkflowName,
		&workflow.Description,
		&workflow.DocumentType,
		&workflow.Status,
		&workflow.Version,
		&workflow.IsCurrent,
		&configJSON,
		&workflow.CreatedBy,
		&workflow.CreatedAt,
		&workflow.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(configJSON, &workflow.Configuration); err != nil {
		workflow.Configuration = make(map[string]any)
	}

	return &workflow, nil
}

// PostgresWorkflowStepRepo implements WorkflowStepRepository
type PostgresWorkflowStepRepo struct {
	db *database.PostgresDB
}

// NewPostgresWorkflowStepRepo creates a new PostgresWorkflowStepRepo
func NewPostgresWorkflowStepRepo(db *database.PostgresDB) *PostgresWorkflowStepRepo {
	return &PostgresWorkflowStepRepo{db: db}
}

func (r *PostgresWorkflowStepRepo) Create(ctx context.Context, step *domain.WorkflowStep) error {
	query := `
		INSERT INTO workflow.workflow_steps (
			id, workflow_id, step_number, step_name, step_type,
			approver_type, approver_id, approver_expression,
			threshold_min, threshold_max, threshold_currency,
			required_approvals, allow_self_approval,
			escalation_hours, escalate_to_step, condition_expression,
			is_active, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	_, err := r.db.ExecContext(ctx, query,
		step.ID,
		step.WorkflowID,
		step.StepNumber,
		step.StepName,
		step.StepType,
		step.ApproverType,
		step.ApproverID,
		step.ApproverExpression,
		step.ThresholdMin,
		step.ThresholdMax,
		step.ThresholdCurrency,
		step.RequiredApprovals,
		step.AllowSelfApproval,
		step.EscalationHours,
		step.EscalateToStep,
		step.ConditionExpression,
		step.IsActive,
		step.CreatedAt,
	)

	return err
}

func (r *PostgresWorkflowStepRepo) Update(ctx context.Context, step *domain.WorkflowStep) error {
	query := `
		UPDATE workflow.workflow_steps SET
			step_name = $2,
			step_type = $3,
			approver_type = $4,
			approver_id = $5,
			approver_expression = $6,
			threshold_min = $7,
			threshold_max = $8,
			threshold_currency = $9,
			required_approvals = $10,
			allow_self_approval = $11,
			escalation_hours = $12,
			escalate_to_step = $13,
			condition_expression = $14,
			is_active = $15
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		step.ID,
		step.StepName,
		step.StepType,
		step.ApproverType,
		step.ApproverID,
		step.ApproverExpression,
		step.ThresholdMin,
		step.ThresholdMax,
		step.ThresholdCurrency,
		step.RequiredApprovals,
		step.AllowSelfApproval,
		step.EscalationHours,
		step.EscalateToStep,
		step.ConditionExpression,
		step.IsActive,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrStepNotFound
	}

	return nil
}

func (r *PostgresWorkflowStepRepo) GetByID(ctx context.Context, id common.ID) (*domain.WorkflowStep, error) {
	query := `
		SELECT id, workflow_id, step_number, step_name, step_type,
			   approver_type, approver_id, approver_expression,
			   threshold_min, threshold_max, threshold_currency,
			   required_approvals, allow_self_approval,
			   escalation_hours, escalate_to_step, condition_expression,
			   is_active, created_at
		FROM workflow.workflow_steps
		WHERE id = $1
	`

	return r.scanStep(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresWorkflowStepRepo) GetByWorkflowID(ctx context.Context, workflowID common.ID) ([]domain.WorkflowStep, error) {
	query := `
		SELECT id, workflow_id, step_number, step_name, step_type,
			   approver_type, approver_id, approver_expression,
			   threshold_min, threshold_max, threshold_currency,
			   required_approvals, allow_self_approval,
			   escalation_hours, escalate_to_step, condition_expression,
			   is_active, created_at
		FROM workflow.workflow_steps
		WHERE workflow_id = $1 AND is_active = true
		ORDER BY step_number
	`

	rows, err := r.db.QueryContext(ctx, query, workflowID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var steps []domain.WorkflowStep
	for rows.Next() {
		step, err := r.scanStepRow(rows)
		if err != nil {
			return nil, err
		}
		steps = append(steps, *step)
	}

	return steps, rows.Err()
}

func (r *PostgresWorkflowStepRepo) GetByWorkflowAndNumber(ctx context.Context, workflowID common.ID, stepNumber int) (*domain.WorkflowStep, error) {
	query := `
		SELECT id, workflow_id, step_number, step_name, step_type,
			   approver_type, approver_id, approver_expression,
			   threshold_min, threshold_max, threshold_currency,
			   required_approvals, allow_self_approval,
			   escalation_hours, escalate_to_step, condition_expression,
			   is_active, created_at
		FROM workflow.workflow_steps
		WHERE workflow_id = $1 AND step_number = $2
	`

	return r.scanStep(r.db.QueryRowContext(ctx, query, workflowID, stepNumber))
}

func (r *PostgresWorkflowStepRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM workflow.workflow_steps WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrStepNotFound
	}

	return nil
}

func (r *PostgresWorkflowStepRepo) DeleteByWorkflowID(ctx context.Context, workflowID common.ID) error {
	query := `DELETE FROM workflow.workflow_steps WHERE workflow_id = $1`
	_, err := r.db.ExecContext(ctx, query, workflowID)
	return err
}

func (r *PostgresWorkflowStepRepo) scanStep(row rowScanner) (*domain.WorkflowStep, error) {
	var step domain.WorkflowStep
	var thresholdMin, thresholdMax *decimal.Decimal
	var thresholdCurrency sql.NullString

	err := row.Scan(
		&step.ID,
		&step.WorkflowID,
		&step.StepNumber,
		&step.StepName,
		&step.StepType,
		&step.ApproverType,
		&step.ApproverID,
		&step.ApproverExpression,
		&thresholdMin,
		&thresholdMax,
		&thresholdCurrency,
		&step.RequiredApprovals,
		&step.AllowSelfApproval,
		&step.EscalationHours,
		&step.EscalateToStep,
		&step.ConditionExpression,
		&step.IsActive,
		&step.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrStepNotFound
	}
	if err != nil {
		return nil, err
	}

	step.ThresholdMin = thresholdMin
	step.ThresholdMax = thresholdMax
	if thresholdCurrency.Valid {
		step.ThresholdCurrency = thresholdCurrency.String
	}

	return &step, nil
}

func (r *PostgresWorkflowStepRepo) scanStepRow(rows *sql.Rows) (*domain.WorkflowStep, error) {
	var step domain.WorkflowStep
	var thresholdMin, thresholdMax *decimal.Decimal
	var thresholdCurrency sql.NullString

	err := rows.Scan(
		&step.ID,
		&step.WorkflowID,
		&step.StepNumber,
		&step.StepName,
		&step.StepType,
		&step.ApproverType,
		&step.ApproverID,
		&step.ApproverExpression,
		&thresholdMin,
		&thresholdMax,
		&thresholdCurrency,
		&step.RequiredApprovals,
		&step.AllowSelfApproval,
		&step.EscalationHours,
		&step.EscalateToStep,
		&step.ConditionExpression,
		&step.IsActive,
		&step.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	step.ThresholdMin = thresholdMin
	step.ThresholdMax = thresholdMax
	if thresholdCurrency.Valid {
		step.ThresholdCurrency = thresholdCurrency.String
	}

	return &step, nil
}

// PostgresDelegationRepo implements DelegationRepository
type PostgresDelegationRepo struct {
	db *database.PostgresDB
}

// NewPostgresDelegationRepo creates a new PostgresDelegationRepo
func NewPostgresDelegationRepo(db *database.PostgresDB) *PostgresDelegationRepo {
	return &PostgresDelegationRepo{db: db}
}

func (r *PostgresDelegationRepo) Create(ctx context.Context, delegation *domain.Delegation) error {
	query := `
		INSERT INTO workflow.delegations (
			id, entity_id, delegator_id, delegate_id, workflow_id,
			document_types, start_date, end_date, reason,
			is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.db.ExecContext(ctx, query,
		delegation.ID,
		delegation.EntityID,
		delegation.DelegatorID,
		delegation.DelegateID,
		delegation.WorkflowID,
		delegation.DocumentTypes,
		delegation.StartDate,
		delegation.EndDate,
		delegation.Reason,
		delegation.IsActive,
		delegation.CreatedAt,
		delegation.UpdatedAt,
	)

	return err
}

func (r *PostgresDelegationRepo) Update(ctx context.Context, delegation *domain.Delegation) error {
	query := `
		UPDATE workflow.delegations SET
			workflow_id = $2,
			document_types = $3,
			start_date = $4,
			end_date = $5,
			reason = $6,
			is_active = $7,
			updated_at = $8
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		delegation.ID,
		delegation.WorkflowID,
		delegation.DocumentTypes,
		delegation.StartDate,
		delegation.EndDate,
		delegation.Reason,
		delegation.IsActive,
		delegation.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrDelegationNotFound
	}

	return nil
}

func (r *PostgresDelegationRepo) GetByID(ctx context.Context, id common.ID) (*domain.Delegation, error) {
	query := `
		SELECT id, entity_id, delegator_id, delegate_id, workflow_id,
			   document_types, start_date, end_date, reason,
			   is_active, created_at, updated_at
		FROM workflow.delegations
		WHERE id = $1
	`

	return r.scanDelegation(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresDelegationRepo) GetActiveDelegations(ctx context.Context, entityID, delegatorID common.ID) ([]domain.Delegation, error) {
	query := `
		SELECT id, entity_id, delegator_id, delegate_id, workflow_id,
			   document_types, start_date, end_date, reason,
			   is_active, created_at, updated_at
		FROM workflow.delegations
		WHERE entity_id = $1 AND delegator_id = $2 AND is_active = true
			AND start_date <= CURRENT_DATE
			AND (end_date IS NULL OR end_date >= CURRENT_DATE)
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, entityID, delegatorID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var delegations []domain.Delegation
	for rows.Next() {
		delegation, err := r.scanDelegationRow(rows)
		if err != nil {
			return nil, err
		}
		delegations = append(delegations, *delegation)
	}

	return delegations, rows.Err()
}

func (r *PostgresDelegationRepo) GetEffectiveDelegateFor(ctx context.Context, entityID, delegatorID common.ID, documentType string, workflowID *common.ID) (*domain.Delegation, error) {
	query := `
		SELECT id, entity_id, delegator_id, delegate_id, workflow_id,
			   document_types, start_date, end_date, reason,
			   is_active, created_at, updated_at
		FROM workflow.delegations
		WHERE entity_id = $1 AND delegator_id = $2 AND is_active = true
			AND start_date <= CURRENT_DATE
			AND (end_date IS NULL OR end_date >= CURRENT_DATE)
			AND ($3 = '' OR document_types = '{}' OR $3 = ANY(document_types))
			AND ($4::char(26) IS NULL OR workflow_id IS NULL OR workflow_id = $4)
		ORDER BY created_at DESC
		LIMIT 1
	`

	return r.scanDelegation(r.db.QueryRowContext(ctx, query, entityID, delegatorID, documentType, workflowID))
}

func (r *PostgresDelegationRepo) ListByEntity(ctx context.Context, entityID common.ID, activeOnly bool) ([]domain.Delegation, error) {
	query := `
		SELECT id, entity_id, delegator_id, delegate_id, workflow_id,
			   document_types, start_date, end_date, reason,
			   is_active, created_at, updated_at
		FROM workflow.delegations
		WHERE entity_id = $1
	`

	if activeOnly {
		query += ` AND is_active = true AND start_date <= CURRENT_DATE AND (end_date IS NULL OR end_date >= CURRENT_DATE)`
	}

	query += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var delegations []domain.Delegation
	for rows.Next() {
		delegation, err := r.scanDelegationRow(rows)
		if err != nil {
			return nil, err
		}
		delegations = append(delegations, *delegation)
	}

	return delegations, rows.Err()
}

func (r *PostgresDelegationRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM workflow.delegations WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrDelegationNotFound
	}

	return nil
}

func (r *PostgresDelegationRepo) scanDelegation(row rowScanner) (*domain.Delegation, error) {
	var delegation domain.Delegation

	err := row.Scan(
		&delegation.ID,
		&delegation.EntityID,
		&delegation.DelegatorID,
		&delegation.DelegateID,
		&delegation.WorkflowID,
		&delegation.DocumentTypes,
		&delegation.StartDate,
		&delegation.EndDate,
		&delegation.Reason,
		&delegation.IsActive,
		&delegation.CreatedAt,
		&delegation.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrDelegationNotFound
	}
	if err != nil {
		return nil, err
	}

	return &delegation, nil
}

func (r *PostgresDelegationRepo) scanDelegationRow(rows *sql.Rows) (*domain.Delegation, error) {
	var delegation domain.Delegation

	err := rows.Scan(
		&delegation.ID,
		&delegation.EntityID,
		&delegation.DelegatorID,
		&delegation.DelegateID,
		&delegation.WorkflowID,
		&delegation.DocumentTypes,
		&delegation.StartDate,
		&delegation.EndDate,
		&delegation.Reason,
		&delegation.IsActive,
		&delegation.CreatedAt,
		&delegation.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &delegation, nil
}
