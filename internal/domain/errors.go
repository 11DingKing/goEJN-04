package domain

import "errors"

// Sentinel errors for domain-level failure conditions. Callers may use
// errors.Is to distinguish between business-rule violations and infrastructural
// problems.
var (
	ErrInvalidTransition    = errors.New("invalid state transition")
	ErrEntityNotFound       = errors.New("entity not found")
	ErrWorkOrderNotFound    = errors.New("work order not found")
	ErrBlackStartNotFound   = errors.New("black start not found")
	ErrGridConnNotFound     = errors.New("grid connection not found")
	ErrInspectionNotFound   = errors.New("inspection not found")
	ErrSMSNotFound          = errors.New("sms message not found")
	ErrLoadNotFound         = errors.New("load not found")
	ErrTempExceedsThreshold = errors.New("battery cell temperature exceeds threshold")
	ErrSOCBelowMinimum      = errors.New("state of charge below minimum for off-grid operation")
	ErrAlreadySigned        = errors.New("sign-off already recorded for this party")
	ErrDualSignRequired     = errors.New("dual sign-off required before synchronization")
	ErrDeadlineExceeded     = errors.New("deadline exceeded")
	ErrDuplicateID          = errors.New("duplicate id")
	ErrOffGridProhibited    = errors.New("off-grid operation prohibited")
	ErrAlreadyAccepted      = errors.New("work order already accepted")
	ErrNotDispatched        = errors.New("work order has not been dispatched")
)
