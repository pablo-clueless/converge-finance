package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

// TransactionType represents the type of intercompany transaction
type TransactionType string

const (
	TransactionTypeSale       TransactionType = "sale"
	TransactionTypeService    TransactionType = "service"
	TransactionTypeLoan       TransactionType = "loan"
	TransactionTypeAllocation TransactionType = "allocation"
	TransactionTypeDividend   TransactionType = "dividend"
	TransactionTypeCapital    TransactionType = "capital"
	TransactionTypeRecharge   TransactionType = "recharge"
	TransactionTypeTransfer   TransactionType = "transfer"
)

func (t TransactionType) IsValid() bool {
	switch t {
	case TransactionTypeSale, TransactionTypeService, TransactionTypeLoan,
		TransactionTypeAllocation, TransactionTypeDividend, TransactionTypeCapital,
		TransactionTypeRecharge, TransactionTypeTransfer:
		return true
	}
	return false
}

func (t TransactionType) String() string {
	return string(t)
}

// DisplayName returns a human-readable name for the transaction type
func (t TransactionType) DisplayName() string {
	switch t {
	case TransactionTypeSale:
		return "Intercompany Sale"
	case TransactionTypeService:
		return "Intercompany Service"
	case TransactionTypeLoan:
		return "Intercompany Loan"
	case TransactionTypeAllocation:
		return "Cost Allocation"
	case TransactionTypeDividend:
		return "Dividend Distribution"
	case TransactionTypeCapital:
		return "Capital Contribution"
	case TransactionTypeRecharge:
		return "Expense Recharge"
	case TransactionTypeTransfer:
		return "Asset/Inventory Transfer"
	default:
		return string(t)
	}
}

// AccountMapping defines the GL accounts used for intercompany transactions
// between a pair of entities for a specific transaction type
type AccountMapping struct {
	ID              common.ID
	FromEntityID    common.ID
	ToEntityID      common.ID
	TransactionType TransactionType

	// FROM entity accounts (the initiating entity)
	FromDueToAccountID   *common.ID // Liability: Due to TO_entity
	FromDueFromAccountID *common.ID // Asset: Due from TO_entity
	FromRevenueAccountID *common.ID // IC Revenue account
	FromExpenseAccountID *common.ID // IC Expense account

	// TO entity accounts (the counterparty)
	ToDueToAccountID   *common.ID // Liability: Due to FROM_entity
	ToDueFromAccountID *common.ID // Asset: Due from FROM_entity
	ToRevenueAccountID *common.ID // IC Revenue account
	ToExpenseAccountID *common.ID // IC Expense account

	Description string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewAccountMapping creates a new intercompany account mapping
func NewAccountMapping(
	fromEntityID common.ID,
	toEntityID common.ID,
	transactionType TransactionType,
) (*AccountMapping, error) {
	if fromEntityID.IsZero() {
		return nil, fmt.Errorf("from entity ID is required")
	}
	if toEntityID.IsZero() {
		return nil, fmt.Errorf("to entity ID is required")
	}
	if fromEntityID == toEntityID {
		return nil, fmt.Errorf("from and to entities must be different")
	}
	if !transactionType.IsValid() {
		return nil, fmt.Errorf("invalid transaction type: %s", transactionType)
	}

	now := time.Now()
	return &AccountMapping{
		ID:              common.NewID(),
		FromEntityID:    fromEntityID,
		ToEntityID:      toEntityID,
		TransactionType: transactionType,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// SetFromAccounts sets the accounts for the initiating entity
func (m *AccountMapping) SetFromAccounts(dueToID, dueFromID, revenueID, expenseID *common.ID) {
	m.FromDueToAccountID = dueToID
	m.FromDueFromAccountID = dueFromID
	m.FromRevenueAccountID = revenueID
	m.FromExpenseAccountID = expenseID
	m.UpdatedAt = time.Now()
}

// SetToAccounts sets the accounts for the counterparty entity
func (m *AccountMapping) SetToAccounts(dueToID, dueFromID, revenueID, expenseID *common.ID) {
	m.ToDueToAccountID = dueToID
	m.ToDueFromAccountID = dueFromID
	m.ToRevenueAccountID = revenueID
	m.ToExpenseAccountID = expenseID
	m.UpdatedAt = time.Now()
}

// Activate activates the account mapping
func (m *AccountMapping) Activate() {
	m.IsActive = true
	m.UpdatedAt = time.Now()
}

// Deactivate deactivates the account mapping
func (m *AccountMapping) Deactivate() {
	m.IsActive = false
	m.UpdatedAt = time.Now()
}

// Validate validates the account mapping
func (m *AccountMapping) Validate() error {
	ve := common.NewValidationError()

	if m.FromEntityID.IsZero() {
		ve.Add("from_entity_id", "required", "From entity ID is required")
	}
	if m.ToEntityID.IsZero() {
		ve.Add("to_entity_id", "required", "To entity ID is required")
	}
	if m.FromEntityID == m.ToEntityID {
		ve.Add("to_entity_id", "invalid", "From and to entities must be different")
	}
	if !m.TransactionType.IsValid() {
		ve.Add("transaction_type", "invalid", "Invalid transaction type")
	}

	// Validate that required accounts are set based on transaction type
	switch m.TransactionType {
	case TransactionTypeSale, TransactionTypeService, TransactionTypeRecharge:
		// For revenue/expense transactions, need Due To/From and Revenue/Expense accounts
		if m.FromDueFromAccountID == nil {
			ve.Add("from_due_from_account_id", "required", "From Due From account is required for this transaction type")
		}
		if m.FromRevenueAccountID == nil {
			ve.Add("from_revenue_account_id", "required", "From Revenue account is required for this transaction type")
		}
		if m.ToDueToAccountID == nil {
			ve.Add("to_due_to_account_id", "required", "To Due To account is required for this transaction type")
		}
		if m.ToExpenseAccountID == nil {
			ve.Add("to_expense_account_id", "required", "To Expense account is required for this transaction type")
		}

	case TransactionTypeLoan, TransactionTypeTransfer:
		// For balance sheet transactions, just need Due To/From accounts
		if m.FromDueFromAccountID == nil && m.FromDueToAccountID == nil {
			ve.Add("from_accounts", "required", "From Due To or Due From account is required")
		}
		if m.ToDueFromAccountID == nil && m.ToDueToAccountID == nil {
			ve.Add("to_accounts", "required", "To Due To or Due From account is required")
		}
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

// HasRequiredAccounts checks if all required accounts are configured
func (m *AccountMapping) HasRequiredAccounts() bool {
	err := m.Validate()
	return err == nil
}

// GetReversedMapping returns a new mapping with from/to entities swapped
func (m *AccountMapping) GetReversedMapping() *AccountMapping {
	return &AccountMapping{
		ID:                   m.ID, // Same ID - this is a view
		FromEntityID:         m.ToEntityID,
		ToEntityID:           m.FromEntityID,
		TransactionType:      m.TransactionType,
		FromDueToAccountID:   m.ToDueToAccountID,
		FromDueFromAccountID: m.ToDueFromAccountID,
		FromRevenueAccountID: m.ToRevenueAccountID,
		FromExpenseAccountID: m.ToExpenseAccountID,
		ToDueToAccountID:     m.FromDueToAccountID,
		ToDueFromAccountID:   m.FromDueFromAccountID,
		ToRevenueAccountID:   m.FromRevenueAccountID,
		ToExpenseAccountID:   m.FromExpenseAccountID,
		Description:          m.Description,
		IsActive:             m.IsActive,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}
}

// AccountMappingFilter for querying account mappings
type AccountMappingFilter struct {
	FromEntityID    *common.ID
	ToEntityID      *common.ID
	TransactionType *TransactionType
	IsActive        *bool
	Limit           int
	Offset          int
}
