package domain

import (
	"fmt"
	"time"

	"converge-finance.com/m/internal/domain/common"
)

type FiscalYearStatus string

const (
	FiscalYearStatusOpen    FiscalYearStatus = "open"
	FiscalYearStatusClosing FiscalYearStatus = "closing"
	FiscalYearStatusClosed  FiscalYearStatus = "closed"
)

type FiscalYear struct {
	ID        common.ID
	EntityID  common.ID
	YearCode  string
	StartDate time.Time
	EndDate   time.Time
	Status    FiscalYearStatus
	Periods   []FiscalPeriod
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewFiscalYear(entityID common.ID, yearCode string, startDate, endDate time.Time) (*FiscalYear, error) {
	if entityID.IsZero() {
		return nil, fmt.Errorf("entity ID is required")
	}
	if yearCode == "" {
		return nil, fmt.Errorf("year code is required")
	}
	if !endDate.After(startDate) {
		return nil, fmt.Errorf("end date must be after start date")
	}

	now := time.Now()
	return &FiscalYear{
		ID:        common.NewID(),
		EntityID:  entityID,
		YearCode:  yearCode,
		StartDate: startDate,
		EndDate:   endDate,
		Status:    FiscalYearStatusOpen,
		Periods:   make([]FiscalPeriod, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (fy *FiscalYear) GenerateMonthlyPeriods() error {
	if len(fy.Periods) > 0 {
		return fmt.Errorf("periods already exist")
	}

	currentDate := fy.StartDate
	periodNum := 1

	for currentDate.Before(fy.EndDate) {

		nextMonth := currentDate.AddDate(0, 1, 0)
		periodEnd := nextMonth.AddDate(0, 0, -1)

		if periodEnd.After(fy.EndDate) {
			periodEnd = fy.EndDate
		}

		period := FiscalPeriod{
			ID:           common.NewID(),
			EntityID:     fy.EntityID,
			FiscalYearID: fy.ID,
			PeriodNumber: periodNum,
			PeriodName:   currentDate.Format("January 2006"),
			StartDate:    currentDate,
			EndDate:      periodEnd,
			Status:       FiscalPeriodStatusFuture,
			IsAdjustment: false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		fy.Periods = append(fy.Periods, period)
		periodNum++
		currentDate = nextMonth
	}

	return nil
}

func (fy *FiscalYear) AddAdjustmentPeriod() error {

	for _, p := range fy.Periods {
		if p.IsAdjustment {
			return fmt.Errorf("adjustment period already exists")
		}
	}

	period := FiscalPeriod{
		ID:           common.NewID(),
		EntityID:     fy.EntityID,
		FiscalYearID: fy.ID,
		PeriodNumber: 13,
		PeriodName:   "Year-End Adjustments",
		StartDate:    fy.EndDate,
		EndDate:      fy.EndDate,
		Status:       FiscalPeriodStatusFuture,
		IsAdjustment: true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	fy.Periods = append(fy.Periods, period)
	return nil
}

func (fy *FiscalYear) GetPeriodForDate(date time.Time) *FiscalPeriod {
	for i := range fy.Periods {
		if !fy.Periods[i].IsAdjustment &&
			!date.Before(fy.Periods[i].StartDate) &&
			!date.After(fy.Periods[i].EndDate) {
			return &fy.Periods[i]
		}
	}
	return nil
}

func (fy *FiscalYear) GetAdjustmentPeriod() *FiscalPeriod {
	for i := range fy.Periods {
		if fy.Periods[i].IsAdjustment {
			return &fy.Periods[i]
		}
	}
	return nil
}

func (fy *FiscalYear) Close() error {
	if fy.Status == FiscalYearStatusClosed {
		return fmt.Errorf("fiscal year is already closed")
	}

	for _, p := range fy.Periods {
		if p.Status != FiscalPeriodStatusClosed {
			return fmt.Errorf("cannot close fiscal year: period %d is not closed", p.PeriodNumber)
		}
	}

	fy.Status = FiscalYearStatusClosed
	fy.UpdatedAt = time.Now()
	return nil
}

func (fy *FiscalYear) ContainsDate(date time.Time) bool {
	return !date.Before(fy.StartDate) && !date.After(fy.EndDate)
}

type FiscalPeriodStatus string

const (
	FiscalPeriodStatusFuture  FiscalPeriodStatus = "future"
	FiscalPeriodStatusOpen    FiscalPeriodStatus = "open"
	FiscalPeriodStatusClosing FiscalPeriodStatus = "closing"
	FiscalPeriodStatusClosed  FiscalPeriodStatus = "closed"
)

type FiscalPeriod struct {
	ID           common.ID
	EntityID     common.ID
	FiscalYearID common.ID
	PeriodNumber int
	PeriodName   string
	StartDate    time.Time
	EndDate      time.Time
	Status       FiscalPeriodStatus
	IsAdjustment bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (fp FiscalPeriod) CanPost() bool {
	return fp.Status == FiscalPeriodStatusOpen
}

func (fp *FiscalPeriod) Open() error {
	if fp.Status == FiscalPeriodStatusOpen {
		return fmt.Errorf("period is already open")
	}
	if fp.Status == FiscalPeriodStatusClosed {
		return fmt.Errorf("cannot reopen a closed period")
	}

	fp.Status = FiscalPeriodStatusOpen
	fp.UpdatedAt = time.Now()
	return nil
}

func (fp *FiscalPeriod) StartClosing() error {
	if fp.Status != FiscalPeriodStatusOpen {
		return fmt.Errorf("can only start closing an open period")
	}

	fp.Status = FiscalPeriodStatusClosing
	fp.UpdatedAt = time.Now()
	return nil
}

func (fp *FiscalPeriod) Close() error {
	if fp.Status == FiscalPeriodStatusClosed {
		return fmt.Errorf("period is already closed")
	}
	if fp.Status == FiscalPeriodStatusFuture {
		return fmt.Errorf("cannot close a future period")
	}

	fp.Status = FiscalPeriodStatusClosed
	fp.UpdatedAt = time.Now()
	return nil
}

func (fp *FiscalPeriod) Reopen() error {
	if fp.Status != FiscalPeriodStatusClosing {
		return fmt.Errorf("can only reopen a closing period")
	}

	fp.Status = FiscalPeriodStatusOpen
	fp.UpdatedAt = time.Now()
	return nil
}

func (fp FiscalPeriod) ContainsDate(date time.Time) bool {
	return !date.Before(fp.StartDate) && !date.After(fp.EndDate)
}

func (fp FiscalPeriod) Validate() error {
	ve := common.NewValidationError()

	if fp.EntityID.IsZero() {
		ve.Add("entity_id", "required", "Entity ID is required")
	}
	if fp.FiscalYearID.IsZero() {
		ve.Add("fiscal_year_id", "required", "Fiscal year ID is required")
	}
	if fp.PeriodNumber < 1 || fp.PeriodNumber > 13 {
		ve.Add("period_number", "invalid", "Period number must be between 1 and 13")
	}
	if fp.PeriodName == "" {
		ve.Add("period_name", "required", "Period name is required")
	}
	if fp.EndDate.Before(fp.StartDate) {
		ve.Add("end_date", "invalid", "End date cannot be before start date")
	}

	if ve.HasErrors() {
		return ve
	}
	return nil
}

type PeriodFilter struct {
	EntityID     common.ID
	FiscalYearID *common.ID
	Status       *FiscalPeriodStatus
	DateFrom     *time.Time
	DateTo       *time.Time
}
