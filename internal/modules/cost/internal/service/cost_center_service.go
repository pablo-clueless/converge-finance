package service

import (
	"context"
	"fmt"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/modules/cost/internal/domain"
	"converge-finance.com/m/internal/modules/cost/internal/repository"
	"converge-finance.com/m/internal/platform/audit"
)

type CostCenterService struct {
	costCenterRepo repository.CostCenterRepository
	auditLogger    *audit.Logger
}

func NewCostCenterService(
	costCenterRepo repository.CostCenterRepository,
	auditLogger *audit.Logger,
) *CostCenterService {
	return &CostCenterService{
		costCenterRepo: costCenterRepo,
		auditLogger:    auditLogger,
	}
}

func (s *CostCenterService) CreateCostCenter(
	ctx context.Context,
	entityID common.ID,
	code string,
	name string,
	centerType domain.CenterType,
	parentID *common.ID,
) (*domain.CostCenter, error) {
	center, err := domain.NewCostCenter(entityID, code, name, centerType)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost center: %w", err)
	}

	if parentID != nil {
		parent, err := s.costCenterRepo.GetByID(ctx, *parentID)
		if err != nil {
			return nil, fmt.Errorf("parent cost center not found: %w", err)
		}
		if parent.EntityID != entityID {
			return nil, fmt.Errorf("parent cost center belongs to different entity")
		}
		center.SetParent(*parentID)
	}

	if err := s.costCenterRepo.Create(ctx, center); err != nil {
		return nil, fmt.Errorf("failed to save cost center: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "cost.cost_center", center.ID, "created", map[string]any{
			"code":        center.Code,
			"name":        center.Name,
			"center_type": center.CenterType,
		})
	}

	return center, nil
}

func (s *CostCenterService) UpdateCostCenter(
	ctx context.Context,
	id common.ID,
	name *string,
	description *string,
	managerID *common.ID,
	managerName *string,
	headcount *int,
	squareFootage *float64,
) (*domain.CostCenter, error) {
	center, err := s.costCenterRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("cost center not found: %w", err)
	}

	if name != nil {
		center.Name = *name
	}
	if description != nil {
		center.Description = *description
	}
	if managerID != nil && managerName != nil {
		center.SetManager(*managerID, *managerName)
	}
	if headcount != nil || squareFootage != nil {
		h := center.Headcount
		sf := center.SquareFootage
		if headcount != nil {
			h = *headcount
		}
		if squareFootage != nil {
			sf = *squareFootage
		}
		center.UpdateStatistics(h, sf)
	}

	if err := s.costCenterRepo.Update(ctx, center); err != nil {
		return nil, fmt.Errorf("failed to update cost center: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "cost.cost_center", center.ID, "updated", nil)
	}

	return center, nil
}

func (s *CostCenterService) GetCostCenter(ctx context.Context, id common.ID) (*domain.CostCenter, error) {
	return s.costCenterRepo.GetByID(ctx, id)
}

func (s *CostCenterService) GetCostCenterByCode(ctx context.Context, entityID common.ID, code string) (*domain.CostCenter, error) {
	return s.costCenterRepo.GetByCode(ctx, entityID, code)
}

func (s *CostCenterService) ListCostCenters(ctx context.Context, filter domain.CostCenterFilter) ([]domain.CostCenter, error) {
	return s.costCenterRepo.List(ctx, filter)
}

func (s *CostCenterService) GetCostCenterHierarchy(ctx context.Context, rootID common.ID) (*domain.CostCenter, error) {
	return s.costCenterRepo.GetHierarchy(ctx, rootID)
}

func (s *CostCenterService) GetChildren(ctx context.Context, parentID common.ID) ([]domain.CostCenter, error) {
	return s.costCenterRepo.GetChildren(ctx, parentID)
}

func (s *CostCenterService) DeactivateCostCenter(ctx context.Context, id common.ID) error {
	center, err := s.costCenterRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("cost center not found: %w", err)
	}

	children, err := s.costCenterRepo.GetChildren(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check children: %w", err)
	}
	if len(children) > 0 {
		return fmt.Errorf("cannot deactivate cost center with children")
	}

	center.Deactivate()

	if err := s.costCenterRepo.Update(ctx, center); err != nil {
		return fmt.Errorf("failed to deactivate cost center: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "cost.cost_center", id, "deactivated", nil)
	}

	return nil
}

func (s *CostCenterService) SetDefaultExpenseAccount(ctx context.Context, centerID, accountID common.ID) error {
	center, err := s.costCenterRepo.GetByID(ctx, centerID)
	if err != nil {
		return fmt.Errorf("cost center not found: %w", err)
	}

	center.SetDefaultExpenseAccount(accountID)

	if err := s.costCenterRepo.Update(ctx, center); err != nil {
		return fmt.Errorf("failed to set default account: %w", err)
	}

	return nil
}
