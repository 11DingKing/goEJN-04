package domain

import "time"

// WorkOrderStatus enumerates the states of a repair work order
// (维修工单) through its closed-loop lifecycle.
type WorkOrderStatus string

const (
	WorkOrderStatusDraft      WorkOrderStatus = "draft"
	WorkOrderStatusDispatched WorkOrderStatus = "dispatched"
	WorkOrderStatusAccepted   WorkOrderStatus = "accepted"
	WorkOrderStatusInProgress WorkOrderStatus = "in_progress"
	WorkOrderStatusWorsened   WorkOrderStatus = "worsened"
	WorkOrderStatusCompleted  WorkOrderStatus = "completed"
	WorkOrderStatusEscalated  WorkOrderStatus = "escalated"
	WorkOrderStatusCancelled  WorkOrderStatus = "cancelled"
)

var workOrderTransitions = map[WorkOrderStatus][]WorkOrderStatus{
	WorkOrderStatusDraft:      {WorkOrderStatusDispatched, WorkOrderStatusCancelled},
	WorkOrderStatusDispatched: {WorkOrderStatusAccepted, WorkOrderStatusEscalated, WorkOrderStatusCancelled},
	WorkOrderStatusEscalated:  {WorkOrderStatusAccepted, WorkOrderStatusCancelled},
	WorkOrderStatusAccepted:   {WorkOrderStatusInProgress, WorkOrderStatusCancelled},
	WorkOrderStatusInProgress: {WorkOrderStatusWorsened, WorkOrderStatusCompleted, WorkOrderStatusCancelled},
	WorkOrderStatusWorsened:   {WorkOrderStatusInProgress, WorkOrderStatusCompleted, WorkOrderStatusCancelled},
	WorkOrderStatusCompleted:  {},
	WorkOrderStatusCancelled:  {},
}

// CanTransitionTo reports whether a transition from the current status to the
// target status is permitted by the work-order state machine.
func (s WorkOrderStatus) CanTransitionTo(target WorkOrderStatus) bool {
	for _, t := range workOrderTransitions[s] {
		if t == target {
			return true
		}
	}
	return false
}

// Terminal reports whether the status is a final state with no further
// transitions.
func (s WorkOrderStatus) Terminal() bool {
	return s == WorkOrderStatusCompleted || s == WorkOrderStatusCancelled
}

// WorkOrder is the central record of a repair closed-loop task. When equipment
// deteriorates during repair an associated child order is created and the
// backup repair team is notified.
type WorkOrder struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	EntityType     EntityType      `json:"entity_type"`
	EntityID       string          `json:"entity_id"`
	Status         WorkOrderStatus `json:"status"`
	Description    string          `json:"description"`
	AssignedTo     string          `json:"assigned_to"`
	EscalatedTo    string          `json:"escalated_to"`
	ParentOrderID  string          `json:"parent_order_id"`
	BackupNotified bool            `json:"backup_notified"`
	DispatchedAt   time.Time       `json:"dispatched_at"`
	AcceptedAt     time.Time       `json:"accepted_at"`
	CompletedAt    time.Time       `json:"completed_at"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
