package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type CenterType string

const (
	CenterTypeProduction     CenterType = "production"
	CenterTypeService        CenterType = "service"
	CenterTypeAdministrative CenterType = "administrative"
	CenterTypeSelling        CenterType = "selling"
	CenterTypeResearch       CenterType = "research"
	CenterTypeSupport        CenterType = "support"
)

func (t CenterType) IsValid() bool {
	switch t {
	case CenterTypeProduction, CenterTypeService, CenterTypeAdministrative,
		CenterTypeSelling, CenterTypeResearch, CenterTypeSupport:
		return true
	}
	return false
}

type CostCenter struct {
	ID                      common.ID
	EntityID                common.ID
	Code                    string
	Name                    string
	Description             string
	CenterType              CenterType
	ParentID                *common.ID
	HierarchyLevel          int
	HierarchyPath           string
	ManagerID               *common.ID
	ManagerName             string
	DefaultExpenseAccountID *common.ID
	Headcount               int
	SquareFootage           float64
	IsActive                bool
	CreatedAt               time.Time
	UpdatedAt               time.Time

	Children []CostCenter
}

func NewCostCenter(
	entityID common.ID,
	code string,
	name string,
	centerType CenterType,
) (*CostCenter, error) {
	if code == "" {
		return nil, fmt.Errorf("cost center code is required")
	}
	if name == "" {
		return nil, fmt.Errorf("cost center name is required")
	}
	if !centerType.IsValid() {
		return nil, fmt.Errorf("invalid center type")
	}

	return &CostCenter{
		ID:         common.NewID(),
		EntityID:   entityID,
		Code:       code,
		Name:       name,
		CenterType: centerType,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (c *CostCenter) SetParent(parentID common.ID) {
	c.ParentID = &parentID
	c.UpdatedAt = time.Now()
}

func (c *CostCenter) SetManager(managerID common.ID, managerName string) {
	c.ManagerID = &managerID
	c.ManagerName = managerName
	c.UpdatedAt = time.Now()
}

func (c *CostCenter) SetDefaultExpenseAccount(accountID common.ID) {
	c.DefaultExpenseAccountID = &accountID
	c.UpdatedAt = time.Now()
}

func (c *CostCenter) UpdateStatistics(headcount int, squareFootage float64) {
	c.Headcount = headcount
	c.SquareFootage = squareFootage
	c.UpdatedAt = time.Now()
}

func (c *CostCenter) Deactivate() {
	c.IsActive = false
	c.UpdatedAt = time.Now()
}

func (c *CostCenter) Activate() {
	c.IsActive = true
	c.UpdatedAt = time.Now()
}

type CostCenterFilter struct {
	EntityID   *common.ID
	ParentID   *common.ID
	CenterType *CenterType
	IsActive   *bool
	Search     string
	Limit      int
	Offset     int
}
