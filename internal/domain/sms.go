package domain

import "time"

// SMSMessageStatus tracks delivery of an SMS instruction used as a fallback
// channel when the primary network is interrupted.
type SMSMessageStatus string

const (
	SMSStatusPending   SMSMessageStatus = "pending"
	SMSStatusDelivered SMSMessageStatus = "delivered"
)

// SMSMessage represents a black-start instruction that must be delivered even
// if the primary network fails. The scheduler retries pending messages until
// they are acknowledged.
type SMSMessage struct {
	ID            string           `json:"id"`
	BlackStartID  string           `json:"black_start_id"`
	Instruction   string           `json:"instruction"`
	Status        SMSMessageStatus `json:"status"`
	Attempts      int              `json:"attempts"`
	CreatedAt     time.Time        `json:"created_at"`
	DeliveredAt   time.Time        `json:"delivered_at"`
	LastAttemptAt time.Time        `json:"last_attempt_at"`
}
