package domain

import "time"

// GridConnStatus enumerates the states of a grid-connection switch request
// (并网切换). Both the dispatcher and the operations team must review and sign
// before synchronization proceeds.
type GridConnStatus string

const (
	GridConnStatusRequested      GridConnStatus = "requested"
	GridConnStatusDispatchSigned GridConnStatus = "dispatch_signed"
	GridConnStatusOMSigned       GridConnStatus = "om_signed"
	GridConnStatusDualSigned     GridConnStatus = "dual_signed"
	GridConnStatusSynchronized   GridConnStatus = "synchronized"
	GridConnStatusCompleted      GridConnStatus = "completed"
	GridConnStatusRejected       GridConnStatus = "rejected"
)

var gridConnTransitions = map[GridConnStatus][]GridConnStatus{
	GridConnStatusRequested:      {GridConnStatusDispatchSigned, GridConnStatusOMSigned, GridConnStatusRejected},
	GridConnStatusDispatchSigned: {GridConnStatusDualSigned, GridConnStatusRejected},
	GridConnStatusOMSigned:       {GridConnStatusDualSigned, GridConnStatusRejected},
	GridConnStatusDualSigned:     {GridConnStatusSynchronized, GridConnStatusRejected},
	GridConnStatusSynchronized:   {GridConnStatusCompleted},
	GridConnStatusCompleted:      {},
	GridConnStatusRejected:       {},
}

// CanTransitionTo reports whether the transition is permitted.
func (s GridConnStatus) CanTransitionTo(target GridConnStatus) bool {
	for _, t := range gridConnTransitions[s] {
		if t == target {
			return true
		}
	}
	return false
}

// Terminal reports whether the status is final.
func (s GridConnStatus) Terminal() bool {
	return s == GridConnStatusCompleted || s == GridConnStatusRejected
}

// SignParty identifies which role is signing off on a grid connection.
type SignParty string

const (
	SignPartyDispatch SignParty = "dispatch"
	SignPartyOM       SignParty = "operations"
)

// GridConnection tracks the dual sign-off process for a grid switch. The
// OffGrid flag indicates that an off-grid operation is concurrent, which
// triggers priority-based load shedding.
type GridConnection struct {
	ID               string         `json:"id"`
	BlackStartID     string         `json:"black_start_id"`
	Status           GridConnStatus `json:"status"`
	DispatchSigned   bool           `json:"dispatch_signed"`
	DispatchSignedBy string         `json:"dispatch_signed_by"`
	DispatchSignedAt time.Time      `json:"dispatch_signed_at"`
	OMSigned         bool           `json:"om_signed"`
	OMSignedBy       string         `json:"om_signed_by"`
	OMSignedAt       time.Time      `json:"om_signed_at"`
	OffGrid          bool           `json:"off_grid"`
	SynchronizedAt   time.Time      `json:"synchronized_at"`
	CompletedAt      time.Time      `json:"completed_at"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// BothSigned reports whether both parties have signed off.
func (gc GridConnection) BothSigned() bool {
	return gc.DispatchSigned && gc.OMSigned
}
