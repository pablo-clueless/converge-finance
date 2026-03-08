package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusApproved  TransferStatus = "approved"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusCancelled TransferStatus = "cancelled"
)

func (s TransferStatus) IsValid() bool {
	switch s {
	case TransferStatusPending, TransferStatusApproved, TransferStatusCompleted, TransferStatusCancelled:
		return true
	}
	return false
}

func (s TransferStatus) CanApprove() bool {
	return s == TransferStatusPending
}

func (s TransferStatus) CanComplete() bool {
	return s == TransferStatusApproved
}

func (s TransferStatus) CanCancel() bool {
	return s == TransferStatusPending || s == TransferStatusApproved
}

type AssetTransfer struct {
	ID              common.ID
	EntityID        common.ID
	TransferNumber  string
	AssetID         common.ID
	TransferDate    time.Time
	EffectiveDate   time.Time

	FromLocationCode  string
	FromLocationName  string
	ToLocationCode    string
	ToLocationName    string

	FromDepartmentCode string
	FromDepartmentName string
	ToDepartmentCode   string
	ToDepartmentName   string

	FromCustodianID   *common.ID
	FromCustodianName string
	ToCustodianID     *common.ID
	ToCustodianName   string

	FromCostCenterID *common.ID
	ToCostCenterID   *common.ID

	Reason string
	Status TransferStatus

	ApprovedBy  *common.ID
	ApprovedAt  *time.Time
	CompletedBy *common.ID
	CompletedAt *time.Time

	CreatedBy common.ID
	CreatedAt time.Time
	UpdatedAt time.Time

	Asset *Asset
}

func NewAssetTransfer(
	entityID common.ID,
	transferNumber string,
	asset *Asset,
	transferDate time.Time,
	effectiveDate time.Time,
	createdBy common.ID,
) (*AssetTransfer, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if transferNumber == "" {
		return nil, fmt.Errorf("transfer number is required")
	}
	if asset == nil {
		return nil, fmt.Errorf("asset is required")
	}
	if !asset.Status.CanTransfer() {
		return nil, fmt.Errorf("asset cannot be transferred with status: %s", asset.Status)
	}

	now := time.Now()
	return &AssetTransfer{
		ID:                 common.NewID(),
		EntityID:           entityID,
		TransferNumber:     transferNumber,
		AssetID:            asset.ID,
		TransferDate:       transferDate,
		EffectiveDate:      effectiveDate,
		FromLocationCode:   asset.LocationCode,
		FromLocationName:   asset.LocationName,
		FromDepartmentCode: asset.DepartmentCode,
		FromDepartmentName: asset.DepartmentName,
		FromCustodianID:    asset.CustodianID,
		FromCustodianName:  asset.CustodianName,
		FromCostCenterID:   asset.CostCenterID,
		Status:             TransferStatusPending,
		CreatedBy:          createdBy,
		CreatedAt:          now,
		UpdatedAt:          now,
		Asset:              asset,
	}, nil
}

func (t *AssetTransfer) SetDestination(
	locationCode string,
	locationName string,
	departmentCode string,
	departmentName string,
	custodianID *common.ID,
	custodianName string,
	costCenterID *common.ID,
) {
	t.ToLocationCode = locationCode
	t.ToLocationName = locationName
	t.ToDepartmentCode = departmentCode
	t.ToDepartmentName = departmentName
	t.ToCustodianID = custodianID
	t.ToCustodianName = custodianName
	t.ToCostCenterID = costCenterID
	t.UpdatedAt = time.Now()
}

func (t *AssetTransfer) Validate() error {
	ve := common.NewValidationError()

	if t.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if t.TransferNumber == "" {
		ve.Add("transfer_number", "required", "Transfer number is required")
	}
	if t.AssetID.IsZero() {
		ve.Add("asset_id", "required", "Asset ID is required")
	}

	hasLocationChange := t.ToLocationCode != "" || t.ToLocationCode != t.FromLocationCode
	hasDepartmentChange := t.ToDepartmentCode != "" || t.ToDepartmentCode != t.FromDepartmentCode
	hasCustodianChange := t.ToCustodianID != nil || (t.ToCustodianID == nil && t.FromCustodianID != nil)

	if !hasLocationChange && !hasDepartmentChange && !hasCustodianChange {
		ve.Add("destination", "required", "At least one destination field must change")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

func (t *AssetTransfer) Approve(approvedBy common.ID) error {
	if !t.Status.CanApprove() {
		return fmt.Errorf("cannot approve transfer with status: %s", t.Status)
	}

	now := time.Now()
	t.Status = TransferStatusApproved
	t.ApprovedBy = &approvedBy
	t.ApprovedAt = &now
	t.UpdatedAt = now
	return nil
}

func (t *AssetTransfer) Complete(completedBy common.ID) error {
	if !t.Status.CanComplete() {
		return fmt.Errorf("cannot complete transfer with status: %s", t.Status)
	}

	now := time.Now()
	t.Status = TransferStatusCompleted
	t.CompletedBy = &completedBy
	t.CompletedAt = &now
	t.UpdatedAt = now
	return nil
}

func (t *AssetTransfer) Cancel() error {
	if !t.Status.CanCancel() {
		return fmt.Errorf("cannot cancel transfer with status: %s", t.Status)
	}

	t.Status = TransferStatusCancelled
	t.UpdatedAt = time.Now()
	return nil
}

func (t *AssetTransfer) ApplyToAsset(asset *Asset) {
	if t.ToLocationCode != "" {
		asset.LocationCode = t.ToLocationCode
		asset.LocationName = t.ToLocationName
	}
	if t.ToDepartmentCode != "" {
		asset.DepartmentCode = t.ToDepartmentCode
		asset.DepartmentName = t.ToDepartmentName
	}
	if t.ToCustodianID != nil || t.FromCustodianID != nil {
		asset.CustodianID = t.ToCustodianID
		asset.CustodianName = t.ToCustodianName
	}
	if t.ToCostCenterID != nil || t.FromCostCenterID != nil {
		asset.CostCenterID = t.ToCostCenterID
	}
	asset.UpdatedAt = time.Now()
}

type AssetTransferFilter struct {
	EntityID     common.ID
	AssetID      *common.ID
	Status       *TransferStatus
	DateFrom     *time.Time
	DateTo       *time.Time
	LocationCode *string
	Limit        int
	Offset       int
}
