package domain

import (
	"fmt"
	"strings"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"github.com/shopspring/decimal"
)

type EntityType string

const (
	EntityTypeOperating   EntityType = "operating"
	EntityTypeHolding     EntityType = "holding"
	EntityTypeElimination EntityType = "elimination"
)

func (t EntityType) IsValid() bool {
	switch t {
	case EntityTypeOperating, EntityTypeHolding, EntityTypeElimination:
		return true
	}
	return false
}

func (t EntityType) String() string {
	return string(t)
}

type ConsolidationMethod string

const (
	ConsolidationMethodFull         ConsolidationMethod = "full"
	ConsolidationMethodProportional ConsolidationMethod = "proportional"
	ConsolidationMethodEquity       ConsolidationMethod = "equity"
	ConsolidationMethodNone         ConsolidationMethod = "none"
)

func (m ConsolidationMethod) IsValid() bool {
	switch m {
	case ConsolidationMethodFull, ConsolidationMethodProportional, ConsolidationMethodEquity, ConsolidationMethodNone:
		return true
	}
	return false
}

func (m ConsolidationMethod) String() string {
	return string(m)
}

type EntityHierarchy struct {
	ID           common.ID
	Code         string
	Name         string
	BaseCurrency string
	IsActive     bool

	ParentID            *common.ID
	EntityType          EntityType
	OwnershipPercent    decimal.Decimal
	ConsolidationMethod ConsolidationMethod
	HierarchyLevel      int
	HierarchyPath       string

	CreatedAt time.Time
	UpdatedAt time.Time

	Parent   *EntityHierarchy
	Children []*EntityHierarchy
}

func NewEntityHierarchy(
	id common.ID,
	code string,
	name string,
	baseCurrency string,
) *EntityHierarchy {
	now := time.Now()
	return &EntityHierarchy{
		ID:                  id,
		Code:                code,
		Name:                name,
		BaseCurrency:        baseCurrency,
		IsActive:            true,
		EntityType:          EntityTypeOperating,
		OwnershipPercent:    decimal.NewFromInt(100),
		ConsolidationMethod: ConsolidationMethodFull,
		HierarchyLevel:      0,
		HierarchyPath:       id.String(),
		CreatedAt:           now,
		UpdatedAt:           now,
		Children:            make([]*EntityHierarchy, 0),
	}
}

func (e *EntityHierarchy) SetParent(parent *EntityHierarchy) error {
	if parent != nil {

		if e.ID == parent.ID {
			return fmt.Errorf("entity cannot be its own parent")
		}
		if e.IsAncestorOf(parent) {
			return fmt.Errorf("circular reference detected in entity hierarchy")
		}

		e.ParentID = &parent.ID
		e.HierarchyLevel = parent.HierarchyLevel + 1
		e.HierarchyPath = parent.HierarchyPath + "/" + e.ID.String()
		e.Parent = parent
	} else {
		e.ParentID = nil
		e.HierarchyLevel = 0
		e.HierarchyPath = e.ID.String()
		e.Parent = nil
	}
	e.UpdatedAt = time.Now()
	return nil
}

func (e *EntityHierarchy) SetOwnership(percent decimal.Decimal, method ConsolidationMethod) error {
	if percent.LessThan(decimal.Zero) || percent.GreaterThan(decimal.NewFromInt(100)) {
		return fmt.Errorf("ownership percent must be between 0 and 100")
	}
	if !method.IsValid() {
		return fmt.Errorf("invalid consolidation method: %s", method)
	}

	e.OwnershipPercent = percent
	e.ConsolidationMethod = method
	e.UpdatedAt = time.Now()
	return nil
}

func (e *EntityHierarchy) SetEntityType(entityType EntityType) error {
	if !entityType.IsValid() {
		return fmt.Errorf("invalid entity type: %s", entityType)
	}
	e.EntityType = entityType
	e.UpdatedAt = time.Now()
	return nil
}

func (e *EntityHierarchy) IsAncestorOf(other *EntityHierarchy) bool {
	if other == nil {
		return false
	}
	return strings.Contains(other.HierarchyPath, e.ID.String())
}

func (e *EntityHierarchy) IsDescendantOf(other *EntityHierarchy) bool {
	if other == nil {
		return false
	}
	return strings.Contains(e.HierarchyPath, other.ID.String())
}

func (e *EntityHierarchy) IsRoot() bool {
	return e.ParentID == nil
}

func (e *EntityHierarchy) HasChildren() bool {
	return len(e.Children) > 0
}

func (e *EntityHierarchy) GetRootID() common.ID {
	parts := strings.Split(e.HierarchyPath, "/")
	if len(parts) > 0 {
		id, _ := common.Parse(parts[0])
		return id
	}
	return e.ID
}

func (e *EntityHierarchy) GetAllDescendantIDs() []common.ID {
	var ids []common.ID
	for _, child := range e.Children {
		ids = append(ids, child.ID)
		ids = append(ids, child.GetAllDescendantIDs()...)
	}
	return ids
}

func (e *EntityHierarchy) RequiresElimination() bool {
	return e.ConsolidationMethod == ConsolidationMethodFull ||
		e.ConsolidationMethod == ConsolidationMethodProportional
}

func (e *EntityHierarchy) Validate() error {
	ve := common.NewValidationError()

	if e.ID.IsZero() {
		ve.Add("id", "required", "Entity ID is required")
	}
	if e.Code == "" {
		ve.Add("code", "required", "Entity code is required")
	}
	if e.Name == "" {
		ve.Add("name", "required", "Entity name is required")
	}
	if !e.EntityType.IsValid() {
		ve.Add("entity_type", "invalid", "Invalid entity type")
	}
	if e.OwnershipPercent.LessThan(decimal.Zero) || e.OwnershipPercent.GreaterThan(decimal.NewFromInt(100)) {
		ve.Add("ownership_percent", "range", "Ownership percent must be between 0 and 100")
	}
	if !e.ConsolidationMethod.IsValid() {
		ve.Add("consolidation_method", "invalid", "Invalid consolidation method")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

type EntityHierarchyFilter struct {
	ParentID   *common.ID
	EntityType *EntityType
	IsActive   *bool
	RootOnly   bool
	Limit      int
	Offset     int
}
