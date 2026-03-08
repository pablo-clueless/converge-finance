package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type TranslationMethod string

const (
	TranslationMethodCurrentRate         TranslationMethod = "current_rate"
	TranslationMethodTemporal            TranslationMethod = "temporal"
	TranslationMethodMonetaryNonMonetary TranslationMethod = "monetary_nonmonetary"
)

func (m TranslationMethod) IsValid() bool {
	switch m {
	case TranslationMethodCurrentRate, TranslationMethodTemporal, TranslationMethodMonetaryNonMonetary:
		return true
	}
	return false
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

type ConsolidationSet struct {
	ID                       common.ID
	SetCode                  string
	SetName                  string
	Description              string
	ParentEntityID           common.ID
	ReportingCurrency        money.Currency
	DefaultTranslationMethod TranslationMethod
	IsActive                 bool
	CreatedAt                time.Time
	UpdatedAt                time.Time

	Members []ConsolidationSetMember
}

func NewConsolidationSet(
	setCode string,
	setName string,
	parentEntityID common.ID,
	reportingCurrency money.Currency,
) (*ConsolidationSet, error) {
	if setCode == "" {
		return nil, fmt.Errorf("set code is required")
	}
	if setName == "" {
		return nil, fmt.Errorf("set name is required")
	}

	return &ConsolidationSet{
		ID:                       common.NewID(),
		SetCode:                  setCode,
		SetName:                  setName,
		ParentEntityID:           parentEntityID,
		ReportingCurrency:        reportingCurrency,
		DefaultTranslationMethod: TranslationMethodCurrentRate,
		IsActive:                 true,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}, nil
}

func (s *ConsolidationSet) AddMember(member ConsolidationSetMember) error {
	for _, m := range s.Members {
		if m.EntityID == member.EntityID {
			return fmt.Errorf("entity already in consolidation set")
		}
	}

	member.ConsolidationSetID = s.ID
	s.Members = append(s.Members, member)
	s.UpdatedAt = time.Now()
	return nil
}

func (s *ConsolidationSet) RemoveMember(entityID common.ID) error {
	for i, m := range s.Members {
		if m.EntityID == entityID {
			s.Members = append(s.Members[:i], s.Members[i+1:]...)
			s.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("entity not found in consolidation set")
}

func (s *ConsolidationSet) Deactivate() {
	s.IsActive = false
	s.UpdatedAt = time.Now()
}

func (s *ConsolidationSet) Activate() {
	s.IsActive = true
	s.UpdatedAt = time.Now()
}

type ConsolidationSetMember struct {
	ID                  common.ID
	ConsolidationSetID  common.ID
	EntityID            common.ID
	OwnershipPercent    float64
	ConsolidationMethod ConsolidationMethod
	TranslationMethod   *TranslationMethod
	FunctionalCurrency  money.Currency
	SequenceNumber      int
	IsActive            bool
	CreatedAt           time.Time
	UpdatedAt           time.Time

	EntityCode string
	EntityName string
}

func NewConsolidationSetMember(
	entityID common.ID,
	ownershipPercent float64,
	consolidationMethod ConsolidationMethod,
	functionalCurrency money.Currency,
) (*ConsolidationSetMember, error) {
	if ownershipPercent <= 0 || ownershipPercent > 100 {
		return nil, fmt.Errorf("ownership percent must be between 0 and 100")
	}
	if !consolidationMethod.IsValid() {
		return nil, fmt.Errorf("invalid consolidation method")
	}

	return &ConsolidationSetMember{
		ID:                  common.NewID(),
		EntityID:            entityID,
		OwnershipPercent:    ownershipPercent,
		ConsolidationMethod: consolidationMethod,
		FunctionalCurrency:  functionalCurrency,
		IsActive:            true,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil
}

func (m *ConsolidationSetMember) MinorityPercent() float64 {
	return 100 - m.OwnershipPercent
}

func (m *ConsolidationSetMember) HasMinorityInterest() bool {
	return m.OwnershipPercent < 100
}

func (m *ConsolidationSetMember) GetTranslationMethod(defaultMethod TranslationMethod) TranslationMethod {
	if m.TranslationMethod != nil {
		return *m.TranslationMethod
	}
	return defaultMethod
}

func (m *ConsolidationSetMember) SetTranslationMethod(method TranslationMethod) error {
	if !method.IsValid() {
		return fmt.Errorf("invalid translation method")
	}
	m.TranslationMethod = &method
	m.UpdatedAt = time.Now()
	return nil
}

type ConsolidationSetFilter struct {
	ParentEntityID *common.ID
	IsActive       *bool
	Search         string
	Limit          int
	Offset         int
}

type AccountMapping struct {
	ID                 common.ID
	ConsolidationSetID common.ID
	EntityID           common.ID
	SourceAccountID    common.ID
	TargetAccountID    common.ID
	RateType           string
	IsActive           bool
	CreatedAt          time.Time
	UpdatedAt          time.Time

	SourceAccountCode string
	SourceAccountName string
	TargetAccountCode string
	TargetAccountName string
}

func NewAccountMapping(
	consolidationSetID common.ID,
	entityID common.ID,
	sourceAccountID common.ID,
	targetAccountID common.ID,
	rateType string,
) (*AccountMapping, error) {
	if rateType == "" {
		rateType = "closing"
	}

	return &AccountMapping{
		ID:                 common.NewID(),
		ConsolidationSetID: consolidationSetID,
		EntityID:           entityID,
		SourceAccountID:    sourceAccountID,
		TargetAccountID:    targetAccountID,
		RateType:           rateType,
		IsActive:           true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}, nil
}
