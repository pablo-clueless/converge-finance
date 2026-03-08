package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
	"converge-finance.com/m/internal/domain/money"
	"converge-finance.com/m/internal/modules/fa/internal/domain"
	"converge-finance.com/m/internal/modules/fa/internal/repository"
	"converge-finance.com/m/internal/modules/gl"
	"converge-finance.com/m/internal/platform/audit"
	"converge-finance.com/m/internal/platform/auth"
	"converge-finance.com/m/internal/platform/database"
)

type AssetService struct {
	db           *database.PostgresDB
	assetRepo    repository.AssetRepository
	categoryRepo repository.CategoryRepository
	transferRepo repository.TransferRepository
	glAPI        gl.API
	auditLogger  *audit.Logger
}

func NewAssetService(
	db *database.PostgresDB,
	assetRepo repository.AssetRepository,
	categoryRepo repository.CategoryRepository,
	transferRepo repository.TransferRepository,
	glAPI gl.API,
	auditLogger *audit.Logger,
) *AssetService {
	return &AssetService{
		db:           db,
		assetRepo:    assetRepo,
		categoryRepo: categoryRepo,
		transferRepo: transferRepo,
		glAPI:        glAPI,
		auditLogger:  auditLogger,
	}
}

type CreateAssetRequest struct {
	EntityID           common.ID
	CategoryID         common.ID
	AssetCode          string
	AssetName          string
	Description        string
	SerialNumber       string
	AcquisitionDate    time.Time
	AcquisitionCost    money.Money
	DepreciationMethod domain.DepreciationMethod
	UsefulLifeYears    int
	UsefulLifeUnits    *int
	SalvageValue       money.Money
	VendorID           *common.ID
	APInvoiceID        *common.ID
	PONumber           string
	LocationCode       string
	LocationName       string
	DepartmentCode     string
	DepartmentName     string
	CustodianID        *common.ID
	CustodianName      string
	CostCenterID       *common.ID
}

func (s *AssetService) CreateAsset(ctx context.Context, req CreateAssetRequest) (*domain.Asset, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	category, err := s.categoryRepo.GetByID(ctx, req.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("invalid category: %w", err)
	}
	if !category.IsActive {
		return nil, fmt.Errorf("category is not active")
	}

	exists, err := s.assetRepo.ExistsByCode(ctx, req.EntityID, req.AssetCode)
	if err != nil {
		return nil, fmt.Errorf("failed to check asset code: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("asset code already exists: %s", req.AssetCode)
	}

	asset, err := domain.NewAsset(
		req.EntityID,
		req.CategoryID,
		req.AssetCode,
		req.AssetName,
		req.AcquisitionDate,
		req.AcquisitionCost,
		req.DepreciationMethod,
		req.UsefulLifeYears,
		req.SalvageValue,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create asset: %w", err)
	}

	asset.Description = req.Description
	asset.SerialNumber = req.SerialNumber
	asset.VendorID = req.VendorID
	asset.APInvoiceID = req.APInvoiceID
	asset.PONumber = req.PONumber
	asset.UsefulLifeUnits = req.UsefulLifeUnits
	asset.LocationCode = req.LocationCode
	asset.LocationName = req.LocationName
	asset.DepartmentCode = req.DepartmentCode
	asset.DepartmentName = req.DepartmentName
	asset.CustodianID = req.CustodianID
	asset.CustodianName = req.CustodianName
	asset.CostCenterID = req.CostCenterID
	asset.Category = category

	if err := asset.Validate(); err != nil {
		return nil, err
	}

	if err := s.assetRepo.Create(ctx, asset); err != nil {
		return nil, fmt.Errorf("failed to save asset: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "fa.asset", asset.ID, "created", map[string]any{
			"asset_code":       asset.AssetCode,
			"asset_name":       asset.AssetName,
			"acquisition_cost": asset.AcquisitionCost.String(),
			"category_id":      asset.CategoryID,
		})
	}

	return asset, nil
}

func (s *AssetService) ActivateAsset(ctx context.Context, assetID common.ID, depreciationStartDate time.Time) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	asset, err := s.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	if err := asset.Activate(depreciationStartDate, common.ID(userID)); err != nil {
		return err
	}

	if err := s.assetRepo.Update(ctx, asset); err != nil {
		return fmt.Errorf("failed to update asset: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "fa.asset", asset.ID, "activated", map[string]any{
			"asset_code":              asset.AssetCode,
			"depreciation_start_date": depreciationStartDate.Format("2006-01-02"),
		})
	}

	return nil
}

func (s *AssetService) SuspendAsset(ctx context.Context, assetID common.ID, reason string) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	asset, err := s.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	if err := asset.Suspend(reason); err != nil {
		return err
	}

	if err := s.assetRepo.Update(ctx, asset); err != nil {
		return fmt.Errorf("failed to update asset: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "fa.asset", asset.ID, "suspended", map[string]any{
			"asset_code": asset.AssetCode,
			"reason":     reason,
		})
	}

	return nil
}

func (s *AssetService) ReactivateAsset(ctx context.Context, assetID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	asset, err := s.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	if err := asset.Reactivate(); err != nil {
		return err
	}

	if err := s.assetRepo.Update(ctx, asset); err != nil {
		return fmt.Errorf("failed to update asset: %w", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogAction(ctx, "fa.asset", asset.ID, "reactivated", map[string]any{
			"asset_code": asset.AssetCode,
		})
	}

	return nil
}

type DisposalRequest struct {
	AssetID      common.ID
	DisposalType domain.DisposalType
	Proceeds     money.Money
	Cost         money.Money
	Notes        string
}

func (s *AssetService) DisposeAsset(ctx context.Context, req DisposalRequest) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	return s.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		asset, err := s.assetRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, req.AssetID)
		if err != nil {
			return fmt.Errorf("failed to get asset: %w", err)
		}

		category, err := s.categoryRepo.GetByID(ctx, asset.CategoryID)
		if err != nil {
			return fmt.Errorf("failed to get category: %w", err)
		}
		asset.Category = category

		if err := asset.Dispose(req.DisposalType, req.Proceeds, req.Cost, req.Notes); err != nil {
			return err
		}

		journalEntry, err := s.createDisposalGLEntry(ctx, asset, req)
		if err != nil {
			return fmt.Errorf("failed to create disposal GL entry: %w", err)
		}

		if err := s.glAPI.PostJournalEntry(ctx, journalEntry.ID); err != nil {
			return fmt.Errorf("failed to post disposal GL entry: %w", err)
		}

		asset.DisposalJournalID = &journalEntry.ID

		if err := s.assetRepo.WithTx(tx).Update(ctx, asset); err != nil {
			return fmt.Errorf("failed to update asset: %w", err)
		}

		if s.auditLogger != nil {
			_ = s.auditLogger.LogAction(ctx, "fa.asset", asset.ID, "disposed", map[string]any{
				"asset_code":       asset.AssetCode,
				"disposal_type":    req.DisposalType,
				"proceeds":         req.Proceeds.String(),
				"cost":             req.Cost.String(),
				"book_value":       asset.BookValue.String(),
				"gain_loss":        asset.DisposalGainLoss.String(),
				"journal_entry_id": journalEntry.ID,
			})
		}

		return nil
	})
}

func (s *AssetService) createDisposalGLEntry(ctx context.Context, asset *domain.Asset, req DisposalRequest) (*gl.JournalEntryResponse, error) {
	assetAccountID := asset.GetEffectiveAssetAccountID(asset.Category)
	accumAccountID := asset.GetEffectiveAccumDepAccountID(asset.Category)

	if assetAccountID == nil {
		return nil, fmt.Errorf("no asset account configured")
	}
	if accumAccountID == nil {
		return nil, fmt.Errorf("no accumulated depreciation account configured")
	}

	var gainLossAccountID *common.ID
	if asset.Category != nil {
		gainLossAccountID = asset.Category.GainLossAccountID
	}
	if gainLossAccountID == nil {
		return nil, fmt.Errorf("no gain/loss account configured")
	}

	var lines []gl.JournalLineRequest

	netProceeds := req.Proceeds.MustSubtract(req.Cost)
	if !netProceeds.IsZero() {
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *assetAccountID,
			Description: fmt.Sprintf("Disposal proceeds - %s", asset.AssetCode),
			Debit:       netProceeds,
			Credit:      money.Zero(asset.Currency),
		})
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *accumAccountID,
		Description: fmt.Sprintf("Clear accumulated depreciation - %s", asset.AssetCode),
		Debit:       asset.AccumulatedDepreciation,
		Credit:      money.Zero(asset.Currency),
	})

	if asset.DisposalGainLoss.IsPositive() {
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *gainLossAccountID,
			Description: fmt.Sprintf("Gain on disposal - %s", asset.AssetCode),
			Debit:       money.Zero(asset.Currency),
			Credit:      asset.DisposalGainLoss,
		})
	} else if asset.DisposalGainLoss.IsNegative() {
		lines = append(lines, gl.JournalLineRequest{
			AccountID:   *gainLossAccountID,
			Description: fmt.Sprintf("Loss on disposal - %s", asset.AssetCode),
			Debit:       asset.DisposalGainLoss.Abs(),
			Credit:      money.Zero(asset.Currency),
		})
	}

	lines = append(lines, gl.JournalLineRequest{
		AccountID:   *assetAccountID,
		Description: fmt.Sprintf("Remove asset cost - %s", asset.AssetCode),
		Debit:       money.Zero(asset.Currency),
		Credit:      asset.AcquisitionCost,
	})

	req2 := gl.CreateJournalEntryRequest{
		EntityID:     asset.EntityID,
		EntryDate:    time.Now(),
		Description:  fmt.Sprintf("Disposal of Fixed Asset - %s (%s)", asset.AssetName, asset.AssetCode),
		CurrencyCode: asset.Currency.Code,
		Lines:        lines,
	}

	return s.glAPI.CreateJournalEntry(ctx, req2)
}

func (s *AssetService) WriteOffAsset(ctx context.Context, assetID common.ID, notes string) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	return s.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		asset, err := s.assetRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, assetID)
		if err != nil {
			return fmt.Errorf("failed to get asset: %w", err)
		}

		if err := asset.WriteOff(notes); err != nil {
			return err
		}

		if err := s.assetRepo.WithTx(tx).Update(ctx, asset); err != nil {
			return fmt.Errorf("failed to update asset: %w", err)
		}

		if s.auditLogger != nil {
			_ = s.auditLogger.LogAction(ctx, "fa.asset", asset.ID, "written_off", map[string]any{
				"asset_code": asset.AssetCode,
				"book_value": asset.BookValue.String(),
				"notes":      notes,
			})
		}

		return nil
	})
}

type TransferRequest struct {
	AssetID          common.ID
	TransferDate     time.Time
	EffectiveDate    time.Time
	ToLocationCode   string
	ToLocationName   string
	ToDepartmentCode string
	ToDepartmentName string
	ToCustodianID    *common.ID
	ToCustodianName  string
	ToCostCenterID   *common.ID
	Reason           string
}

func (s *AssetService) CreateTransfer(ctx context.Context, req TransferRequest) (*domain.AssetTransfer, error) {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return nil, fmt.Errorf("user not authenticated")
	}

	asset, err := s.assetRepo.GetByID(ctx, req.AssetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	transferNumber, err := s.transferRepo.GetNextTransferNumber(ctx, asset.EntityID, "TRF")
	if err != nil {
		return nil, fmt.Errorf("failed to generate transfer number: %w", err)
	}

	transfer, err := domain.NewAssetTransfer(
		asset.EntityID,
		transferNumber,
		asset,
		req.TransferDate,
		req.EffectiveDate,
		common.ID(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer: %w", err)
	}

	transfer.SetDestination(
		req.ToLocationCode,
		req.ToLocationName,
		req.ToDepartmentCode,
		req.ToDepartmentName,
		req.ToCustodianID,
		req.ToCustodianName,
		req.ToCostCenterID,
	)
	transfer.Reason = req.Reason

	if err := transfer.Validate(); err != nil {
		return nil, err
	}

	if err := s.transferRepo.Create(ctx, transfer); err != nil {
		return nil, fmt.Errorf("failed to save transfer: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "fa.transfer", transfer.ID, "created", map[string]any{
			"transfer_number": transfer.TransferNumber,
			"asset_id":        asset.ID,
			"asset_code":      asset.AssetCode,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to log audit action: %w", err)
		}
	}

	return transfer, nil
}

func (s *AssetService) ApproveTransfer(ctx context.Context, transferID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	transfer, err := s.transferRepo.GetByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("failed to get transfer: %w", err)
	}

	if err := transfer.Approve(common.ID(userID)); err != nil {
		return err
	}

	if err := s.transferRepo.Update(ctx, transfer); err != nil {
		return fmt.Errorf("failed to update transfer: %w", err)
	}

	if s.auditLogger != nil {
		err = s.auditLogger.LogAction(ctx, "fa.transfer", transfer.ID, "approved", map[string]any{
			"transfer_number": transfer.TransferNumber,
		})
		if err != nil {
			return fmt.Errorf("failed to log audit action: %w", err)
		}
	}

	return nil
}

func (s *AssetService) CompleteTransfer(ctx context.Context, transferID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	return s.db.WithTransaction(ctx, func(tx *sql.Tx) error {
		transfer, err := s.transferRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, transferID)
		if err != nil {
			return fmt.Errorf("failed to get transfer: %w", err)
		}

		asset, err := s.assetRepo.WithTx(tx).GetByIDForUpdate(ctx, tx, transfer.AssetID)
		if err != nil {
			return fmt.Errorf("failed to get asset: %w", err)
		}

		if err := transfer.Complete(common.ID(userID)); err != nil {
			return err
		}

		transfer.ApplyToAsset(asset)

		if err := s.transferRepo.WithTx(tx).Update(ctx, transfer); err != nil {
			return fmt.Errorf("failed to update transfer: %w", err)
		}

		if err := s.assetRepo.WithTx(tx).Update(ctx, asset); err != nil {
			return fmt.Errorf("failed to update asset: %w", err)
		}

		if s.auditLogger != nil {
			err = s.auditLogger.LogAction(ctx, "fa.transfer", transfer.ID, "completed", map[string]any{
				"transfer_number": transfer.TransferNumber,
				"asset_id":        asset.ID,
				"asset_code":      asset.AssetCode,
				"to_location":     transfer.ToLocationCode,
				"to_department":   transfer.ToDepartmentCode,
			})
			if err != nil {
				return fmt.Errorf("failed to log audit action: %w", err)
			}
		}

		return nil
	})
}

func (s *AssetService) CancelTransfer(ctx context.Context, transferID common.ID) error {
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		return fmt.Errorf("user not authenticated")
	}

	transfer, err := s.transferRepo.GetByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("failed to get transfer: %w", err)
	}

	if err := transfer.Cancel(); err != nil {
		return err
	}

	if err := s.transferRepo.Update(ctx, transfer); err != nil {
		return fmt.Errorf("failed to update transfer: %w", err)
	}

	if s.auditLogger != nil {
		s.auditLogger.LogAction(ctx, "fa.transfer", transfer.ID, "cancelled", map[string]any{
			"transfer_number": transfer.TransferNumber,
		})
	}

	return nil
}

func (s *AssetService) RecordUnits(ctx context.Context, assetID common.ID, units int) error {
	asset, err := s.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	if err := asset.RecordUnits(units); err != nil {
		return err
	}

	if err := s.assetRepo.Update(ctx, asset); err != nil {
		return fmt.Errorf("failed to update asset: %w", err)
	}

	return nil
}

func (s *AssetService) LinkAPInvoice(ctx context.Context, assetID, invoiceID common.ID) error {
	asset, err := s.assetRepo.GetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("failed to get asset: %w", err)
	}

	asset.APInvoiceID = &invoiceID
	asset.UpdatedAt = time.Now()

	if err := s.assetRepo.Update(ctx, asset); err != nil {
		return fmt.Errorf("failed to update asset: %w", err)
	}

	return nil
}
