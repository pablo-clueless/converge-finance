package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/workflow/internal/domain"
	"converge-finance.com/m/internal/platform/database"
	"github.com/shopspring/decimal"
)

type RequestRepository interface {
	Create(ctx context.Context, request *domain.ApprovalRequest) error
	Update(ctx context.Context, request *domain.ApprovalRequest) error
	GetByID(ctx context.Context, id common.ID) (*domain.ApprovalRequest, error)
	GetByRequestNumber(ctx context.Context, entityID common.ID, requestNumber string) (*domain.ApprovalRequest, error)
	GetByDocument(ctx context.Context, entityID common.ID, documentType string, documentID common.ID) (*domain.ApprovalRequest, error)
	List(ctx context.Context, filter RequestFilter) ([]domain.ApprovalRequest, int, error)
	GenerateRequestNumber(ctx context.Context, entityID common.ID, prefix string) (string, error)
}

type ActionRepository interface {
	Create(ctx context.Context, action *domain.ApprovalAction) error
	GetByID(ctx context.Context, id common.ID) (*domain.ApprovalAction, error)
	GetByRequestID(ctx context.Context, requestID common.ID) ([]domain.ApprovalAction, error)
	GetByRequestAndStep(ctx context.Context, requestID common.ID, stepNumber int) ([]domain.ApprovalAction, error)
	CountApprovalsByStep(ctx context.Context, requestID common.ID, stepNumber int) (int, error)
}

type PendingApprovalRepository interface {
	Create(ctx context.Context, pending *domain.PendingApproval) error
	Delete(ctx context.Context, id common.ID) error
	DeleteByRequestAndStep(ctx context.Context, requestID, stepID common.ID) error
	DeleteByRequest(ctx context.Context, requestID common.ID) error
	GetByID(ctx context.Context, id common.ID) (*domain.PendingApproval, error)
	GetByApprover(ctx context.Context, approverID common.ID) ([]domain.PendingApproval, error)
	GetByRequest(ctx context.Context, requestID common.ID) ([]domain.PendingApproval, error)
	GetByRequestAndApprover(ctx context.Context, requestID, approverID common.ID) (*domain.PendingApproval, error)
	GetOverdue(ctx context.Context, entityID common.ID) ([]domain.PendingApproval, error)
	MarkReminderSent(ctx context.Context, id common.ID) error
	MarkEscalated(ctx context.Context, id common.ID) error
}

type RequestFilter struct {
	EntityID     common.ID
	WorkflowID   common.ID
	DocumentType string
	Status       *domain.RequestStatus
	RequestorID  common.ID
	DateFrom     *time.Time
	DateTo       *time.Time
	Limit        int
	Offset       int
}

type PostgresRequestRepo struct {
	db *database.PostgresDB
}

func NewPostgresRequestRepo(db *database.PostgresDB) *PostgresRequestRepo {
	return &PostgresRequestRepo{db: db}
}

func (r *PostgresRequestRepo) Create(ctx context.Context, request *domain.ApprovalRequest) error {
	metadataJSON, err := json.Marshal(request.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO workflow.requests (
			id, entity_id, request_number, workflow_id, document_type,
			document_id, document_number, amount, currency_code,
			current_step, status, requestor_id, requestor_notes,
			metadata, started_at, completed_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	_, err = r.db.ExecContext(ctx, query,
		request.ID,
		request.EntityID,
		request.RequestNumber,
		request.WorkflowID,
		request.DocumentType,
		request.DocumentID,
		request.DocumentNumber,
		request.Amount,
		request.CurrencyCode,
		request.CurrentStep,
		request.Status,
		request.RequestorID,
		request.RequestorNotes,
		metadataJSON,
		request.StartedAt,
		request.CompletedAt,
		request.CreatedAt,
		request.UpdatedAt,
	)

	return err
}

func (r *PostgresRequestRepo) Update(ctx context.Context, request *domain.ApprovalRequest) error {
	metadataJSON, err := json.Marshal(request.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE workflow.requests SET
			current_step = $2,
			status = $3,
			requestor_notes = $4,
			metadata = $5,
			completed_at = $6,
			updated_at = $7
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		request.ID,
		request.CurrentStep,
		request.Status,
		request.RequestorNotes,
		metadataJSON,
		request.CompletedAt,
		request.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrRequestNotFound
	}

	return nil
}

func (r *PostgresRequestRepo) GetByID(ctx context.Context, id common.ID) (*domain.ApprovalRequest, error) {
	query := `
		SELECT id, entity_id, request_number, workflow_id, document_type,
			   document_id, document_number, amount, currency_code,
			   current_step, status, requestor_id, requestor_notes,
			   metadata, started_at, completed_at, created_at, updated_at
		FROM workflow.requests
		WHERE id = $1
	`

	return r.scanRequest(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresRequestRepo) GetByRequestNumber(ctx context.Context, entityID common.ID, requestNumber string) (*domain.ApprovalRequest, error) {
	query := `
		SELECT id, entity_id, request_number, workflow_id, document_type,
			   document_id, document_number, amount, currency_code,
			   current_step, status, requestor_id, requestor_notes,
			   metadata, started_at, completed_at, created_at, updated_at
		FROM workflow.requests
		WHERE entity_id = $1 AND request_number = $2
	`

	return r.scanRequest(r.db.QueryRowContext(ctx, query, entityID, requestNumber))
}

func (r *PostgresRequestRepo) GetByDocument(ctx context.Context, entityID common.ID, documentType string, documentID common.ID) (*domain.ApprovalRequest, error) {
	query := `
		SELECT id, entity_id, request_number, workflow_id, document_type,
			   document_id, document_number, amount, currency_code,
			   current_step, status, requestor_id, requestor_notes,
			   metadata, started_at, completed_at, created_at, updated_at
		FROM workflow.requests
		WHERE entity_id = $1 AND document_type = $2 AND document_id = $3
		ORDER BY created_at DESC
		LIMIT 1
	`

	return r.scanRequest(r.db.QueryRowContext(ctx, query, entityID, documentType, documentID))
}

func (r *PostgresRequestRepo) List(ctx context.Context, filter RequestFilter) ([]domain.ApprovalRequest, int, error) {
	baseQuery := `FROM workflow.requests WHERE entity_id = $1`
	args := []any{filter.EntityID}
	argIdx := 2

	if !filter.WorkflowID.IsZero() {
		baseQuery += fmt.Sprintf(` AND workflow_id = $%d`, argIdx)
		args = append(args, filter.WorkflowID)
		argIdx++
	}
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
	if !filter.RequestorID.IsZero() {
		baseQuery += fmt.Sprintf(` AND requestor_id = $%d`, argIdx)
		args = append(args, filter.RequestorID)
		argIdx++
	}
	if filter.DateFrom != nil {
		baseQuery += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, *filter.DateFrom)
		argIdx++
	}
	if filter.DateTo != nil {
		baseQuery += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		args = append(args, *filter.DateTo)
		argIdx++
	}

	countQuery := `SELECT COUNT(*) ` + baseQuery
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	dataQuery := `
		SELECT id, entity_id, request_number, workflow_id, document_type,
			   document_id, document_number, amount, currency_code,
			   current_step, status, requestor_id, requestor_notes,
			   metadata, started_at, completed_at, created_at, updated_at
		` + baseQuery + ` ORDER BY created_at DESC`

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

	var requests []domain.ApprovalRequest
	for rows.Next() {
		request, err := r.scanRequestRow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan request row: %w", err)
		}
		requests = append(requests, *request)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate request rows: %w", err)
	}

	return requests, total, nil
}

func (r *PostgresRequestRepo) GenerateRequestNumber(ctx context.Context, entityID common.ID, prefix string) (string, error) {
	query := `SELECT workflow.generate_request_number($1, $2)`
	var requestNumber string
	err := r.db.QueryRowContext(ctx, query, entityID, prefix).Scan(&requestNumber)
	if err != nil {
		return "", fmt.Errorf("failed to generate request number: %w", err)
	}
	return requestNumber, nil
}

func (r *PostgresRequestRepo) scanRequest(row rowScanner) (*domain.ApprovalRequest, error) {
	var request domain.ApprovalRequest
	var metadataJSON []byte
	var amount *decimal.Decimal
	var currencyCode sql.NullString

	err := row.Scan(
		&request.ID,
		&request.EntityID,
		&request.RequestNumber,
		&request.WorkflowID,
		&request.DocumentType,
		&request.DocumentID,
		&request.DocumentNumber,
		&amount,
		&currencyCode,
		&request.CurrentStep,
		&request.Status,
		&request.RequestorID,
		&request.RequestorNotes,
		&metadataJSON,
		&request.StartedAt,
		&request.CompletedAt,
		&request.CreatedAt,
		&request.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrRequestNotFound
	}
	if err != nil {
		return nil, err
	}

	request.Amount = amount
	if currencyCode.Valid {
		request.CurrencyCode = currencyCode.String
	}

	if err := json.Unmarshal(metadataJSON, &request.Metadata); err != nil {
		request.Metadata = make(map[string]any)
	}

	return &request, nil
}

func (r *PostgresRequestRepo) scanRequestRow(rows *sql.Rows) (*domain.ApprovalRequest, error) {
	var request domain.ApprovalRequest
	var metadataJSON []byte
	var amount *decimal.Decimal
	var currencyCode sql.NullString

	err := rows.Scan(
		&request.ID,
		&request.EntityID,
		&request.RequestNumber,
		&request.WorkflowID,
		&request.DocumentType,
		&request.DocumentID,
		&request.DocumentNumber,
		&amount,
		&currencyCode,
		&request.CurrentStep,
		&request.Status,
		&request.RequestorID,
		&request.RequestorNotes,
		&metadataJSON,
		&request.StartedAt,
		&request.CompletedAt,
		&request.CreatedAt,
		&request.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	request.Amount = amount
	if currencyCode.Valid {
		request.CurrencyCode = currencyCode.String
	}

	if err := json.Unmarshal(metadataJSON, &request.Metadata); err != nil {
		request.Metadata = make(map[string]any)
	}

	return &request, nil
}

type PostgresActionRepo struct {
	db *database.PostgresDB
}

func NewPostgresActionRepo(db *database.PostgresDB) *PostgresActionRepo {
	return &PostgresActionRepo{db: db}
}

func (r *PostgresActionRepo) Create(ctx context.Context, action *domain.ApprovalAction) error {
	query := `
		INSERT INTO workflow.actions (
			id, request_id, step_id, step_number, action_type,
			actor_id, delegated_by, comments, acted_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		action.ID,
		action.RequestID,
		action.StepID,
		action.StepNumber,
		action.ActionType,
		action.ActorID,
		action.DelegatedBy,
		action.Comments,
		action.ActedAt,
	)

	return err
}

func (r *PostgresActionRepo) GetByID(ctx context.Context, id common.ID) (*domain.ApprovalAction, error) {
	query := `
		SELECT id, request_id, step_id, step_number, action_type,
			   actor_id, delegated_by, comments, acted_at
		FROM workflow.actions
		WHERE id = $1
	`

	return r.scanAction(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresActionRepo) GetByRequestID(ctx context.Context, requestID common.ID) ([]domain.ApprovalAction, error) {
	query := `
		SELECT id, request_id, step_id, step_number, action_type,
			   actor_id, delegated_by, comments, acted_at
		FROM workflow.actions
		WHERE request_id = $1
		ORDER BY acted_at
	`

	rows, err := r.db.QueryContext(ctx, query, requestID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var actions []domain.ApprovalAction
	for rows.Next() {
		action, err := r.scanActionRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan action row: %w", err)
		}
		actions = append(actions, *action)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate action rows: %w", err)
	}

	return actions, nil
}

func (r *PostgresActionRepo) GetByRequestAndStep(ctx context.Context, requestID common.ID, stepNumber int) ([]domain.ApprovalAction, error) {
	query := `
		SELECT id, request_id, step_id, step_number, action_type,
			   actor_id, delegated_by, comments, acted_at
		FROM workflow.actions
		WHERE request_id = $1 AND step_number = $2
		ORDER BY acted_at
	`

	rows, err := r.db.QueryContext(ctx, query, requestID, stepNumber)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var actions []domain.ApprovalAction
	for rows.Next() {
		action, err := r.scanActionRow(rows)
		if err != nil {
			return nil, err
		}
		actions = append(actions, *action)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate action rows: %w", err)
	}

	return actions, nil
}

func (r *PostgresActionRepo) CountApprovalsByStep(ctx context.Context, requestID common.ID, stepNumber int) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM workflow.actions
		WHERE request_id = $1 AND step_number = $2 AND action_type = 'approve'
	`

	var count int
	err := r.db.QueryRowContext(ctx, query, requestID, stepNumber).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count approvals by step: %w", err)
	}
	return count, nil
}

func (r *PostgresActionRepo) scanAction(row rowScanner) (*domain.ApprovalAction, error) {
	var action domain.ApprovalAction

	err := row.Scan(
		&action.ID,
		&action.RequestID,
		&action.StepID,
		&action.StepNumber,
		&action.ActionType,
		&action.ActorID,
		&action.DelegatedBy,
		&action.Comments,
		&action.ActedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrActionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan action row: %w", err)
	}

	return &action, nil
}

func (r *PostgresActionRepo) scanActionRow(rows *sql.Rows) (*domain.ApprovalAction, error) {
	var action domain.ApprovalAction

	err := rows.Scan(
		&action.ID,
		&action.RequestID,
		&action.StepID,
		&action.StepNumber,
		&action.ActionType,
		&action.ActorID,
		&action.DelegatedBy,
		&action.Comments,
		&action.ActedAt,
	)
	if err != nil {
		return nil, err
	}

	return &action, nil
}

type PostgresPendingApprovalRepo struct {
	db *database.PostgresDB
}

func NewPostgresPendingApprovalRepo(db *database.PostgresDB) *PostgresPendingApprovalRepo {
	return &PostgresPendingApprovalRepo{db: db}
}

func (r *PostgresPendingApprovalRepo) Create(ctx context.Context, pending *domain.PendingApproval) error {
	query := `
		INSERT INTO workflow.pending_approvals (
			id, request_id, step_id, approver_id,
			assigned_at, due_at, reminder_sent, escalated
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		pending.ID,
		pending.RequestID,
		pending.StepID,
		pending.ApproverID,
		pending.AssignedAt,
		pending.DueAt,
		pending.ReminderSent,
		pending.Escalated,
	)

	return err
}

func (r *PostgresPendingApprovalRepo) Delete(ctx context.Context, id common.ID) error {
	query := `DELETE FROM workflow.pending_approvals WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrPendingApprovalNotFound
	}

	return nil
}

func (r *PostgresPendingApprovalRepo) DeleteByRequestAndStep(ctx context.Context, requestID, stepID common.ID) error {
	query := `DELETE FROM workflow.pending_approvals WHERE request_id = $1 AND step_id = $2`
	_, err := r.db.ExecContext(ctx, query, requestID, stepID)
	return err
}

func (r *PostgresPendingApprovalRepo) DeleteByRequest(ctx context.Context, requestID common.ID) error {
	query := `DELETE FROM workflow.pending_approvals WHERE request_id = $1`
	_, err := r.db.ExecContext(ctx, query, requestID)
	return err
}

func (r *PostgresPendingApprovalRepo) GetByID(ctx context.Context, id common.ID) (*domain.PendingApproval, error) {
	query := `
		SELECT id, request_id, step_id, approver_id,
			   assigned_at, due_at, reminder_sent, escalated
		FROM workflow.pending_approvals
		WHERE id = $1
	`

	return r.scanPending(r.db.QueryRowContext(ctx, query, id))
}

func (r *PostgresPendingApprovalRepo) GetByApprover(ctx context.Context, approverID common.ID) ([]domain.PendingApproval, error) {
	query := `
		SELECT pa.id, pa.request_id, pa.step_id, pa.approver_id,
			   pa.assigned_at, pa.due_at, pa.reminder_sent, pa.escalated
		FROM workflow.pending_approvals pa
		JOIN workflow.requests req ON req.id = pa.request_id
		WHERE pa.approver_id = $1 AND req.status IN ('pending', 'in_progress', 'escalated')
		ORDER BY pa.due_at NULLS LAST, pa.assigned_at
	`

	rows, err := r.db.QueryContext(ctx, query, approverID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var pending []domain.PendingApproval
	for rows.Next() {
		p, err := r.scanPendingRow(rows)
		if err != nil {
			return nil, err
		}
		pending = append(pending, *p)
	}

	return pending, rows.Err()
}

func (r *PostgresPendingApprovalRepo) GetByRequest(ctx context.Context, requestID common.ID) ([]domain.PendingApproval, error) {
	query := `
		SELECT id, request_id, step_id, approver_id,
			   assigned_at, due_at, reminder_sent, escalated
		FROM workflow.pending_approvals
		WHERE request_id = $1
		ORDER BY assigned_at
	`

	rows, err := r.db.QueryContext(ctx, query, requestID)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = rows.Close()
		if err != nil {
			log.Printf("failed to close rows: %v", err)
		}
	}()

	var pending []domain.PendingApproval
	for rows.Next() {
		p, err := r.scanPendingRow(rows)
		if err != nil {
			return nil, err
		}
		pending = append(pending, *p)
	}

	return pending, rows.Err()
}

func (r *PostgresPendingApprovalRepo) GetByRequestAndApprover(ctx context.Context, requestID, approverID common.ID) (*domain.PendingApproval, error) {
	query := `
		SELECT id, request_id, step_id, approver_id,
			   assigned_at, due_at, reminder_sent, escalated
		FROM workflow.pending_approvals
		WHERE request_id = $1 AND approver_id = $2
		LIMIT 1
	`

	return r.scanPending(r.db.QueryRowContext(ctx, query, requestID, approverID))
}

func (r *PostgresPendingApprovalRepo) GetOverdue(ctx context.Context, entityID common.ID) ([]domain.PendingApproval, error) {
	query := `
		SELECT pa.id, pa.request_id, pa.step_id, pa.approver_id,
			   pa.assigned_at, pa.due_at, pa.reminder_sent, pa.escalated
		FROM workflow.pending_approvals pa
		JOIN workflow.requests req ON req.id = pa.request_id
		WHERE req.entity_id = $1
			AND pa.due_at IS NOT NULL
			AND pa.due_at < NOW()
			AND pa.escalated = false
			AND req.status IN ('pending', 'in_progress')
		ORDER BY pa.due_at
	`

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

	var pending []domain.PendingApproval
	for rows.Next() {
		p, err := r.scanPendingRow(rows)
		if err != nil {
			return nil, err
		}
		pending = append(pending, *p)
	}

	return pending, rows.Err()
}

func (r *PostgresPendingApprovalRepo) MarkReminderSent(ctx context.Context, id common.ID) error {
	query := `UPDATE workflow.pending_approvals SET reminder_sent = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *PostgresPendingApprovalRepo) MarkEscalated(ctx context.Context, id common.ID) error {
	query := `UPDATE workflow.pending_approvals SET escalated = true WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *PostgresPendingApprovalRepo) scanPending(row rowScanner) (*domain.PendingApproval, error) {
	var pending domain.PendingApproval

	err := row.Scan(
		&pending.ID,
		&pending.RequestID,
		&pending.StepID,
		&pending.ApproverID,
		&pending.AssignedAt,
		&pending.DueAt,
		&pending.ReminderSent,
		&pending.Escalated,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrPendingApprovalNotFound
	}
	if err != nil {
		return nil, err
	}

	return &pending, nil
}

func (r *PostgresPendingApprovalRepo) scanPendingRow(rows *sql.Rows) (*domain.PendingApproval, error) {
	var pending domain.PendingApproval

	err := rows.Scan(
		&pending.ID,
		&pending.RequestID,
		&pending.StepID,
		&pending.ApproverID,
		&pending.AssignedAt,
		&pending.DueAt,
		&pending.ReminderSent,
		&pending.Escalated,
	)
	if err != nil {
		return nil, err
	}

	return &pending, nil
}
