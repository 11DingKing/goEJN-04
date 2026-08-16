package domain

import "time"

// BlackStartStatus enumerates the states of a black-start sequence
// (黑启动). The sequence must restore bus voltage within a configured deadline
// (twenty minutes in production) and then proceed to auto-synchronous grid
// connection.
type BlackStartStatus string

const (
	BlackStartStatusInitiated   BlackStartStatus = "initiated"
	BlackStartStatusInProgress  BlackStartStatus = "in_progress"
	BlackStartStatusBusRestored BlackStartStatus = "bus_restored"
	BlackStartStatusGridSyncing BlackStartStatus = "grid_syncing"
	BlackStartStatusCompleted   BlackStartStatus = "completed"
	BlackStartStatusFailed      BlackStartStatus = "failed"
)

var blackStartTransitions = map[BlackStartStatus][]BlackStartStatus{
	BlackStartStatusInitiated:   {BlackStartStatusInProgress, BlackStartStatusFailed},
	BlackStartStatusInProgress:  {BlackStartStatusBusRestored, BlackStartStatusFailed},
	BlackStartStatusBusRestored: {BlackStartStatusGridSyncing, BlackStartStatusFailed},
	BlackStartStatusGridSyncing: {BlackStartStatusCompleted, BlackStartStatusFailed},
	BlackStartStatusCompleted:   {},
	BlackStartStatusFailed:      {},
}

// CanTransitionTo reports whether the transition is permitted.
func (s BlackStartStatus) CanTransitionTo(target BlackStartStatus) bool {
	for _, t := range blackStartTransitions[s] {
		if t == target {
			return true
		}
	}
	return false
}

// Terminal reports whether the status is final.
func (s BlackStartStatus) Terminal() bool {
	return s == BlackStartStatusCompleted || s == BlackStartStatusFailed
}

// BlackStart records a single black-start sequence. Network interruptions do
// not change the status; instead the SMS outbox ensures instructions are
// retried until acknowledged.
type BlackStart struct {
	ID                 string           `json:"id"`
	Status             BlackStartStatus `json:"status"`
	Initiator          string           `json:"initiator"`
	OutageAt           time.Time        `json:"outage_at"`
	InitiatedAt        time.Time        `json:"initiated_at"`
	BusRestoredAt      time.Time        `json:"bus_restored_at"`
	CompletedAt        time.Time        `json:"completed_at"`
	Deadline           time.Time        `json:"deadline"`
	GridConnID         string           `json:"grid_conn_id"`
	NetworkInterrupted bool             `json:"network_interrupted"`
	SMSMessageIDs      []string         `json:"sms_message_ids"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

// IsOverdue reports whether the bus voltage deadline has passed without
// restoration.
func (bs BlackStart) IsOverdue(now time.Time) bool {
	return bs.Status == BlackStartStatusInProgress && now.After(bs.Deadline)
}
