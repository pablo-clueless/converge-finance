package service

import (
	"context"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/segment/internal/domain"
	"converge-finance.com/m/internal/modules/segment/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type SegmentService struct {
	segmentRepo     repository.SegmentRepository
	hierarchyRepo   repository.SegmentHierarchyRepository
	assignmentRepo  repository.AssignmentRepository
	balanceRepo     repository.BalanceRepository
	intersegmentRepo repository.IntersegmentTransactionRepository
	auditLogger     *audit.Logger
	logger          *zap.Logger
}

func NewSegmentService(
	segmentRepo repository.SegmentRepository,
	hierarchyRepo repository.SegmentHierarchyRepository,
	assignmentRepo repository.AssignmentRepository,
	balanceRepo repository.BalanceRepository,
	intersegmentRepo repository.IntersegmentTransactionRepository,
	auditLogger *audit.Logger,
	logger *zap.Logger,
) *SegmentService {
	return &SegmentService{
		segmentRepo:     segmentRepo,
		hierarchyRepo:   hierarchyRepo,
		assignmentRepo:  assignmentRepo,
		balanceRepo:     balanceRepo,
		intersegmentRepo: intersegmentRepo,
		auditLogger:     auditLogger,
		logger:          logger,
	}
}

type CreateSegmentRequest struct {
	EntityID     common.ID
	SegmentCode  string
	SegmentName  string
	SegmentType  domain.SegmentType
	ParentID     *common.ID
	Description  string
	ManagerID    *common.ID
	IsReportable bool
}

func (s *SegmentService) CreateSegment(ctx context.Context, req CreateSegmentRequest) (*domain.Segment, error) {
	if !req.SegmentType.IsValid() {
		return nil, domain.ErrInvalidSegmentType
	}

	existing, err := s.segmentRepo.GetByCode(ctx, req.EntityID, req.SegmentCode)
	if err != nil && err != domain.ErrSegmentNotFound {
		return nil, fmt.Errorf("failed to check existing segment: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrSegmentCodeAlreadyExists
	}

	if req.ParentID != nil {
		parent, err := s.segmentRepo.GetByID(ctx, *req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent segment not found: %w", err)
		}
		if parent.SegmentType != req.SegmentType {
			return nil, fmt.Errorf("parent segment must be of the same type")
		}
	}

	segment := domain.NewSegment(req.EntityID, req.SegmentCode, req.SegmentName, req.SegmentType)
	if req.ParentID != nil {
		segment.SetParent(*req.ParentID)
	}
	if req.ManagerID != nil {
		segment.SetManager(*req.ManagerID)
	}
	segment.SetDescription(req.Description)
	segment.SetReportable(req.IsReportable)

	if err := s.segmentRepo.Create(ctx, segment); err != nil {
		return nil, fmt.Errorf("failed to create segment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment", segment.ID, "segment.created", map[string]any{
		"entity_id":    req.EntityID,
		"segment_code": req.SegmentCode,
		"segment_type": req.SegmentType,
	})

	return segment, nil
}

func (s *SegmentService) GetSegment(ctx context.Context, id common.ID) (*domain.Segment, error) {
	return s.segmentRepo.GetByID(ctx, id)
}

func (s *SegmentService) GetSegmentByCode(ctx context.Context, entityID common.ID, code string) (*domain.Segment, error) {
	return s.segmentRepo.GetByCode(ctx, entityID, code)
}

type UpdateSegmentRequest struct {
	ID           common.ID
	SegmentName  string
	Description  string
	ParentID     *common.ID
	ManagerID    *common.ID
	IsReportable bool
}

func (s *SegmentService) UpdateSegment(ctx context.Context, req UpdateSegmentRequest) (*domain.Segment, error) {
	segment, err := s.segmentRepo.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	if req.ParentID != nil {
		if *req.ParentID == segment.ID {
			return nil, domain.ErrCannotSetSelfAsParent
		}

		if err := s.validateNoCircularReference(ctx, segment.ID, *req.ParentID); err != nil {
			return nil, err
		}

		segment.SetParent(*req.ParentID)
	}

	if req.ManagerID != nil {
		segment.SetManager(*req.ManagerID)
	}

	segment.Update(req.SegmentName, req.Description, req.IsReportable)

	if err := s.segmentRepo.Update(ctx, segment); err != nil {
		return nil, fmt.Errorf("failed to update segment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment", segment.ID, "segment.updated", map[string]any{
		"entity_id":    segment.EntityID,
		"segment_code": segment.SegmentCode,
	})

	return segment, nil
}

func (s *SegmentService) validateNoCircularReference(ctx context.Context, segmentID, newParentID common.ID) error {
	currentID := newParentID
	visited := make(map[common.ID]bool)
	visited[segmentID] = true

	for {
		if visited[currentID] {
			return domain.ErrCircularParentReference
		}
		visited[currentID] = true

		parent, err := s.segmentRepo.GetByID(ctx, currentID)
		if err != nil {
			if err == domain.ErrSegmentNotFound {
				return nil
			}
			return err
		}

		if parent.ParentID == nil {
			return nil
		}

		currentID = *parent.ParentID
	}
}

func (s *SegmentService) DeleteSegment(ctx context.Context, id common.ID) error {
	children, err := s.segmentRepo.GetChildren(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check children: %w", err)
	}
	if len(children) > 0 {
		return domain.ErrSegmentHasChildren
	}

	segment, err := s.segmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.segmentRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete segment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment", id, "segment.deleted", map[string]any{
		"entity_id":    segment.EntityID,
		"segment_code": segment.SegmentCode,
	})

	return nil
}

func (s *SegmentService) ListSegments(ctx context.Context, filter repository.SegmentFilter) ([]domain.Segment, int, error) {
	return s.segmentRepo.List(ctx, filter)
}

func (s *SegmentService) GetSegmentTree(ctx context.Context, entityID common.ID, segmentType domain.SegmentType) (*domain.SegmentTree, error) {
	return s.segmentRepo.GetTree(ctx, entityID, segmentType)
}

type CreateHierarchyRequest struct {
	EntityID      common.ID
	HierarchyCode string
	HierarchyName string
	SegmentType   domain.SegmentType
	Description   string
	IsPrimary     bool
}

func (s *SegmentService) CreateHierarchy(ctx context.Context, req CreateHierarchyRequest) (*domain.SegmentHierarchy, error) {
	if !req.SegmentType.IsValid() {
		return nil, domain.ErrInvalidSegmentType
	}

	existing, err := s.hierarchyRepo.GetByCode(ctx, req.EntityID, req.HierarchyCode)
	if err != nil && err != domain.ErrHierarchyNotFound {
		return nil, fmt.Errorf("failed to check existing hierarchy: %w", err)
	}
	if existing != nil {
		return nil, domain.ErrHierarchyCodeAlreadyExists
	}

	if req.IsPrimary {
		existingPrimary, err := s.hierarchyRepo.GetPrimary(ctx, req.EntityID, req.SegmentType)
		if err != nil && err != domain.ErrHierarchyNotFound {
			return nil, fmt.Errorf("failed to check primary hierarchy: %w", err)
		}
		if existingPrimary != nil {
			return nil, domain.ErrPrimaryHierarchyExists
		}
	}

	hierarchy := domain.NewSegmentHierarchy(req.EntityID, req.HierarchyCode, req.HierarchyName, req.SegmentType)
	hierarchy.SetDescription(req.Description)
	hierarchy.SetPrimary(req.IsPrimary)

	if err := s.hierarchyRepo.Create(ctx, hierarchy); err != nil {
		return nil, fmt.Errorf("failed to create hierarchy: %w", err)
	}

	s.auditLogger.Log(ctx, "segment_hierarchy", hierarchy.ID, "hierarchy.created", map[string]any{
		"entity_id":      req.EntityID,
		"hierarchy_code": req.HierarchyCode,
		"segment_type":   req.SegmentType,
	})

	return hierarchy, nil
}

func (s *SegmentService) GetHierarchy(ctx context.Context, id common.ID) (*domain.SegmentHierarchy, error) {
	return s.hierarchyRepo.GetByID(ctx, id)
}

func (s *SegmentService) ListHierarchies(ctx context.Context, entityID common.ID, segmentType *domain.SegmentType) ([]domain.SegmentHierarchy, error) {
	return s.hierarchyRepo.List(ctx, entityID, segmentType)
}

type AssignToSegmentRequest struct {
	EntityID          common.ID
	SegmentID         common.ID
	AssignmentType    string
	AssignmentID      common.ID
	AllocationPercent decimal.Decimal
	EffectiveFrom     time.Time
	EffectiveTo       *time.Time
}

func (s *SegmentService) AssignToSegment(ctx context.Context, req AssignToSegmentRequest) (*domain.Assignment, error) {
	_, err := s.segmentRepo.GetByID(ctx, req.SegmentID)
	if err != nil {
		return nil, fmt.Errorf("segment not found: %w", err)
	}

	currentTotal, err := s.assignmentRepo.GetTotalAllocation(
		ctx,
		req.AssignmentType,
		req.AssignmentID,
		nil,
		req.EffectiveFrom,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get current allocation: %w", err)
	}

	newTotal := decimal.NewFromFloat(currentTotal).Add(req.AllocationPercent)
	if newTotal.GreaterThan(decimal.NewFromInt(100)) {
		return nil, domain.ErrTotalAllocationExceeds100
	}

	assignment, err := domain.NewAssignment(
		req.EntityID,
		req.SegmentID,
		req.AssignmentType,
		req.AssignmentID,
		req.AllocationPercent,
		req.EffectiveFrom,
	)
	if err != nil {
		return nil, err
	}

	if req.EffectiveTo != nil {
		if err := assignment.SetEffectiveTo(*req.EffectiveTo); err != nil {
			return nil, err
		}
	}

	if err := s.assignmentRepo.Create(ctx, assignment); err != nil {
		return nil, fmt.Errorf("failed to create assignment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment_assignment", assignment.ID, "assignment.created", map[string]any{
		"entity_id":          req.EntityID,
		"segment_id":         req.SegmentID,
		"assignment_type":    req.AssignmentType,
		"allocation_percent": req.AllocationPercent,
	})

	return assignment, nil
}

func (s *SegmentService) GetAssignment(ctx context.Context, id common.ID) (*domain.Assignment, error) {
	return s.assignmentRepo.GetByID(ctx, id)
}

func (s *SegmentService) ListAssignments(ctx context.Context, filter repository.AssignmentFilter) ([]domain.Assignment, int, error) {
	return s.assignmentRepo.List(ctx, filter)
}

func (s *SegmentService) GetEffectiveAssignments(ctx context.Context, entityID common.ID, assignmentType string, assignmentID common.ID, date time.Time) ([]domain.Assignment, error) {
	return s.assignmentRepo.ListEffective(ctx, entityID, assignmentType, assignmentID, date)
}

func (s *SegmentService) UpdateAssignment(ctx context.Context, id common.ID, allocationPercent *decimal.Decimal, effectiveTo *time.Time) (*domain.Assignment, error) {
	assignment, err := s.assignmentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if allocationPercent != nil {
		currentTotal, err := s.assignmentRepo.GetTotalAllocation(
			ctx,
			assignment.AssignmentType,
			assignment.AssignmentID,
			&id,
			assignment.EffectiveFrom,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get current allocation: %w", err)
		}

		newTotal := decimal.NewFromFloat(currentTotal).Add(*allocationPercent)
		if newTotal.GreaterThan(decimal.NewFromInt(100)) {
			return nil, domain.ErrTotalAllocationExceeds100
		}

		if err := assignment.UpdateAllocationPercent(*allocationPercent); err != nil {
			return nil, err
		}
	}

	if effectiveTo != nil {
		if err := assignment.SetEffectiveTo(*effectiveTo); err != nil {
			return nil, err
		}
	}

	if err := s.assignmentRepo.Update(ctx, assignment); err != nil {
		return nil, fmt.Errorf("failed to update assignment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment_assignment", assignment.ID, "assignment.updated", map[string]any{
		"entity_id":  assignment.EntityID,
		"segment_id": assignment.SegmentID,
	})

	return assignment, nil
}

func (s *SegmentService) DeleteAssignment(ctx context.Context, id common.ID) error {
	assignment, err := s.assignmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.assignmentRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete assignment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment_assignment", id, "assignment.deleted", map[string]any{
		"entity_id":  assignment.EntityID,
		"segment_id": assignment.SegmentID,
	})

	return nil
}

type BalanceCalculationProvider interface {
	GetAccountBalances(ctx context.Context, entityID, fiscalPeriodID common.ID) ([]repository.AccountBalanceData, error)
	GetSegmentAllocations(ctx context.Context, entityID, accountID common.ID, effectiveDate time.Time) ([]repository.SegmentAllocationData, error)
}

func (s *SegmentService) CalculateSegmentBalances(ctx context.Context, entityID, periodID common.ID, provider BalanceCalculationProvider, effectiveDate time.Time, currencyCode string) error {
	if err := s.balanceRepo.DeleteByPeriod(ctx, entityID, periodID); err != nil {
		return fmt.Errorf("failed to clear existing balances: %w", err)
	}

	accountBalances, err := provider.GetAccountBalances(ctx, entityID, periodID)
	if err != nil {
		return fmt.Errorf("failed to get account balances: %w", err)
	}

	balanceMap := make(map[string]*domain.SegmentBalance)

	for _, accBal := range accountBalances {
		allocations, err := provider.GetSegmentAllocations(ctx, entityID, accBal.AccountID, effectiveDate)
		if err != nil {
			s.logger.Warn("failed to get segment allocations",
				zap.String("account_id", string(accBal.AccountID)),
				zap.Error(err))
			continue
		}

		for _, alloc := range allocations {
			key := fmt.Sprintf("%s:%s:%s", alloc.SegmentID, periodID, accBal.AccountID)

			if _, exists := balanceMap[key]; !exists {
				balanceMap[key] = domain.NewSegmentBalance(
					entityID,
					alloc.SegmentID,
					periodID,
					accBal.AccountID,
					currencyCode,
				)
			}

			balance := balanceMap[key]
			allocDebit := accBal.DebitAmount.Mul(alloc.AllocationPercent).Div(decimal.NewFromInt(100))
			allocCredit := accBal.CreditAmount.Mul(alloc.AllocationPercent).Div(decimal.NewFromInt(100))

			balance.AddDebit(allocDebit)
			balance.AddCredit(allocCredit)
		}
	}

	for _, balance := range balanceMap {
		if balance.IsZero() {
			continue
		}
		if err := s.balanceRepo.Upsert(ctx, balance); err != nil {
			return fmt.Errorf("failed to upsert balance: %w", err)
		}
	}

	s.auditLogger.Log(ctx, "segment_balance", entityID, "balances.calculated", map[string]any{
		"entity_id":      entityID,
		"period_id":      periodID,
		"balance_count":  len(balanceMap),
	})

	return nil
}

func (s *SegmentService) GetSegmentBalances(ctx context.Context, segmentID, periodID common.ID) ([]domain.SegmentBalance, error) {
	return s.balanceRepo.ListBySegment(ctx, segmentID, periodID)
}

func (s *SegmentService) GetBalanceSummary(ctx context.Context, entityID, periodID common.ID) ([]domain.SegmentBalanceSummary, error) {
	return s.balanceRepo.GetSummaryBySegment(ctx, entityID, periodID)
}

type CreateIntersegmentTransactionRequest struct {
	EntityID        common.ID
	FiscalPeriodID  common.ID
	FromSegmentID   common.ID
	ToSegmentID     common.ID
	TransactionDate time.Time
	Description     string
	Amount          string
	CurrencyCode    string
}

func (s *SegmentService) CreateIntersegmentTransaction(ctx context.Context, req CreateIntersegmentTransactionRequest) (*domain.IntersegmentTransaction, error) {
	if req.FromSegmentID == req.ToSegmentID {
		return nil, domain.ErrSameSegmentTransaction
	}

	_, err := s.segmentRepo.GetByID(ctx, req.FromSegmentID)
	if err != nil {
		return nil, fmt.Errorf("from segment not found: %w", err)
	}

	_, err = s.segmentRepo.GetByID(ctx, req.ToSegmentID)
	if err != nil {
		return nil, fmt.Errorf("to segment not found: %w", err)
	}

	txn := domain.NewIntersegmentTransaction(
		req.EntityID,
		req.FiscalPeriodID,
		req.FromSegmentID,
		req.ToSegmentID,
		req.TransactionDate,
		req.Amount,
		req.CurrencyCode,
	)
	txn.Description = req.Description

	if err := s.intersegmentRepo.Create(ctx, txn); err != nil {
		return nil, fmt.Errorf("failed to create intersegment transaction: %w", err)
	}

	s.auditLogger.Log(ctx, "intersegment_transaction", txn.ID, "intersegment.created", map[string]any{
		"entity_id":       req.EntityID,
		"from_segment_id": req.FromSegmentID,
		"to_segment_id":   req.ToSegmentID,
		"amount":          req.Amount,
	})

	return txn, nil
}

func (s *SegmentService) GetIntersegmentTransaction(ctx context.Context, id common.ID) (*domain.IntersegmentTransaction, error) {
	return s.intersegmentRepo.GetByID(ctx, id)
}

func (s *SegmentService) ListIntersegmentTransactions(ctx context.Context, filter repository.IntersegmentFilter) ([]domain.IntersegmentTransaction, int, error) {
	return s.intersegmentRepo.List(ctx, filter)
}

func (s *SegmentService) EliminateIntersegmentTransaction(ctx context.Context, id common.ID) error {
	txn, err := s.intersegmentRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if txn.IsEliminated {
		return domain.ErrTransactionAlreadyEliminated
	}

	if err := s.intersegmentRepo.Eliminate(ctx, id); err != nil {
		return fmt.Errorf("failed to eliminate transaction: %w", err)
	}

	s.auditLogger.Log(ctx, "intersegment_transaction", id, "intersegment.eliminated", map[string]any{
		"entity_id": txn.EntityID,
	})

	return nil
}

func (s *SegmentService) ActivateSegment(ctx context.Context, id common.ID) (*domain.Segment, error) {
	segment, err := s.segmentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	segment.Activate()

	if err := s.segmentRepo.Update(ctx, segment); err != nil {
		return nil, fmt.Errorf("failed to activate segment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment", id, "segment.activated", map[string]any{
		"entity_id":    segment.EntityID,
		"segment_code": segment.SegmentCode,
	})

	return segment, nil
}

func (s *SegmentService) DeactivateSegment(ctx context.Context, id common.ID) (*domain.Segment, error) {
	segment, err := s.segmentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	segment.Deactivate()

	if err := s.segmentRepo.Update(ctx, segment); err != nil {
		return nil, fmt.Errorf("failed to deactivate segment: %w", err)
	}

	s.auditLogger.Log(ctx, "segment", id, "segment.deactivated", map[string]any{
		"entity_id":    segment.EntityID,
		"segment_code": segment.SegmentCode,
	})

	return segment, nil
}
