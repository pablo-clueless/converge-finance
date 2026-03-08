package domain

import "errors"

var (
	ErrConfigNotFound             = errors.New("triangulation config not found")
	ErrConfigAlreadyExists        = errors.New("triangulation config already exists for entity")
	ErrInvalidMaxLegs             = errors.New("max legs must be between 2 and 5")
	ErrSameCurrency               = errors.New("from and to currencies cannot be the same")
	ErrInvalidViaCurrency         = errors.New("via currency cannot be same as from or to currency")
	ErrInvalidSpreadMarkup        = errors.New("spread markup cannot be negative")
	ErrPairConfigNotFound         = errors.New("currency pair config not found")
	ErrNoConversionPath           = errors.New("no conversion path found")
	ErrRateNotFound               = errors.New("exchange rate not found")
	ErrMaxLegsExceeded            = errors.New("conversion requires more legs than allowed")
	ErrInvalidAmount              = errors.New("conversion amount must be positive")
	ErrCurrencyMismatch           = errors.New("currency mismatch in conversion")
	ErrInvalidRateType            = errors.New("invalid rate type")
	ErrConversionFailed           = errors.New("currency conversion failed")
	ErrLogNotFound                = errors.New("triangulation log not found")
	ErrAccountFXConfigNotFound    = errors.New("account FX config not found")
	ErrRevaluationRunNotFound     = errors.New("revaluation run not found")
	ErrInvalidRevaluationStatus   = errors.New("invalid revaluation status for this operation")
	ErrNoRevaluationDetails       = errors.New("no revaluation details to process")
	ErrRevaluationAlreadyPosted   = errors.New("revaluation run already posted")
	ErrRevaluationAlreadyReversed = errors.New("revaluation run already reversed")
	ErrNoMonetaryAccounts         = errors.New("no monetary accounts found for revaluation")
)
