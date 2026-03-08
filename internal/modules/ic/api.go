package ic

import (
	"context"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
)

type API interface {
	GetEntityHierarchy(ctx context.Context, rootID common.ID) (*EntityHierarchyResponse, error)
	GetChildEntities(ctx context.Context, parentID common.ID) ([]EntityHierarchyResponse, error)

	RecordIntercompanyActivity(ctx context.Context, req RecordICActivityRequest) (*ICTransactionResponse, error)

	GetTransactionByID(ctx context.Context, txID common.ID) (*ICTransactionResponse, error)
	ListTransactions(ctx context.Context, entityID common.ID, filter TransactionFilterRequest) ([]ICTransactionResponse, error)

	GetReconciliationStatus(ctx context.Context, parentEntityID, fiscalPeriodID common.ID) (*ReconciliationStatusResponse, error)
	GetEntityPairBalance(ctx context.Context, fromEntityID, toEntityID, fiscalPeriodID common.ID) (*EntityPairBalanceResponse, error)

	GenerateEliminations(ctx context.Context, parentEntityID, fiscalPeriodID common.ID, eliminationDate time.Time, currency money.Currency) (*EliminationRunResponse, error)
	PostEliminationRun(ctx context.Context, runID common.ID) error
	GetEliminationRun(ctx context.Context, runID common.ID) (*EliminationRunResponse, error)
}

type RecordICActivityRequest struct {
	FromEntityID    common.ID
	ToEntityID      common.ID
	TransactionType string
	TransactionDate time.Time
	Amount          money.Money
	Description     string
	Reference       string
	AutoPost        bool
}

type TransactionFilterRequest struct {
	FromEntityID    *common.ID
	ToEntityID      *common.ID
	TransactionType *string
	Status          *string
	FiscalPeriodID  *common.ID
	DateFrom        *time.Time
	DateTo          *time.Time
	Limit           int
	Offset          int
}

type EntityHierarchyResponse struct {
	ID                  common.ID
	Code                string
	Name                string
	BaseCurrency        string
	IsActive            bool
	ParentID            *common.ID
	EntityType          string
	OwnershipPercent    float64
	ConsolidationMethod string
	HierarchyLevel      int
	HierarchyPath       string
	Children            []EntityHierarchyResponse
}

type ICTransactionResponse struct {
	ID                 common.ID
	TransactionNumber  string
	TransactionType    string
	FromEntityID       common.ID
	ToEntityID         common.ID
	TransactionDate    time.Time
	DueDate            *time.Time
	Amount             money.Money
	Currency           string
	Description        string
	Reference          string
	Status             string
	FromJournalEntryID *common.ID
	ToJournalEntryID   *common.ID
	CreatedAt          time.Time
	PostedAt           *time.Time
	ReconciledAt       *time.Time
}

type ReconciliationStatusResponse struct {
	ParentEntityID    common.ID
	FiscalPeriodID    common.ID
	TotalEntityPairs  int
	ReconciledPairs   int
	UnreconciledPairs int
	DisputedPairs     int
	TotalDiscrepancy  money.Money
	ReconciliationPct float64
}

type EntityPairBalanceResponse struct {
	FromEntityID   common.ID
	ToEntityID     common.ID
	FiscalPeriodID common.ID
	Currency       string
	OpeningBalance money.Money
	PeriodDebits   money.Money
	PeriodCredits  money.Money
	ClosingBalance money.Money
	IsReconciled   bool
	Discrepancy    money.Money
}

type EliminationRunResponse struct {
	ID               common.ID
	RunNumber        string
	ParentEntityID   common.ID
	FiscalPeriodID   common.ID
	EliminationDate  time.Time
	Currency         string
	EntryCount       int
	TotalElimination money.Money
	Status           string
	JournalEntryID   *common.ID
	CreatedAt        time.Time
	PostedAt         *time.Time
	ReversedAt       *time.Time
}
