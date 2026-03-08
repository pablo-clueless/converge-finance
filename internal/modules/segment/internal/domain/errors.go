package domain

import "errors"

var (
	ErrSegmentNotFound          = errors.New("segment not found")
	ErrSegmentCodeAlreadyExists = errors.New("segment code already exists for entity")
	ErrInvalidSegmentType       = errors.New("invalid segment type")
	ErrSegmentHasChildren       = errors.New("segment has child segments")
	ErrCircularParentReference  = errors.New("circular parent reference detected")
	ErrCannotSetSelfAsParent    = errors.New("segment cannot be its own parent")

	ErrHierarchyNotFound          = errors.New("segment hierarchy not found")
	ErrHierarchyCodeAlreadyExists = errors.New("hierarchy code already exists for entity")
	ErrPrimaryHierarchyExists     = errors.New("primary hierarchy already exists for this segment type")

	ErrAssignmentNotFound        = errors.New("assignment not found")
	ErrInvalidAllocationPercent  = errors.New("allocation percent must be between 0 and 100")
	ErrInvalidEffectiveDates     = errors.New("effective_to must be after effective_from")
	ErrOverlappingAssignment     = errors.New("overlapping assignment exists for this period")
	ErrTotalAllocationExceeds100 = errors.New("total allocation percent exceeds 100%")

	ErrBalanceNotFound = errors.New("segment balance not found")

	ErrReportNotFound       = errors.New("segment report not found")
	ErrInvalidReportStatus  = errors.New("invalid report status for this operation")
	ErrReportAlreadyExists  = errors.New("report number already exists")
	ErrNoSegmentsForReport  = errors.New("no segments found for the specified type")

	ErrIntersegmentTransactionNotFound = errors.New("intersegment transaction not found")
	ErrSameSegmentTransaction          = errors.New("from and to segments cannot be the same")
	ErrTransactionAlreadyEliminated    = errors.New("transaction already eliminated")
)
