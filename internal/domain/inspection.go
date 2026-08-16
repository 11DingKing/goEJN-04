package domain

import "time"

// InspectionStatus tracks the lifecycle of a patrol inspection task.
type InspectionStatus string

const (
	InspectionStatusPending   InspectionStatus = "pending"
	InspectionStatusCompleted InspectionStatus = "completed"
)

// Inspection represents a scheduled or ad-hoc inspection of a physical asset.
// When the inspector records an anomaly the workflow automatically creates an
// anomaly report and, if warranted, a repair work order.
type Inspection struct {
	ID          string           `json:"id"`
	EntityType  EntityType       `json:"entity_type"`
	EntityID    string           `json:"entity_id"`
	Inspector   string           `json:"inspector"`
	Status      InspectionStatus `json:"status"`
	Result      string           `json:"result"`
	Anomaly     bool             `json:"anomaly"`
	WorkOrderID string           `json:"work_order_id"`
	CreatedAt   time.Time        `json:"created_at"`
	CompletedAt time.Time        `json:"completed_at"`
}
