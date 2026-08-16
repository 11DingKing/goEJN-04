package app

import (
	"ejina-microgrid/internal/domain"
)

// enqueueSMS creates an SMS outbox entry for a black-start instruction and
// attempts immediate delivery. If delivery fails the message stays pending and
// is retried by the scheduler. The caller must hold s.mu.
func (s *Service) enqueueSMS(blackStartID, instruction string) (domain.SMSMessage, error) {
	now := s.now()
	msg := domain.SMSMessage{
		ID:           s.ids.Next("sms"),
		BlackStartID: blackStartID,
		Instruction:  instruction,
		Status:       domain.SMSStatusPending,
		CreatedAt:    now,
	}
	if err := s.store.SaveSMS(msg); err != nil {
		return domain.SMSMessage{}, err
	}

	// Attempt immediate delivery.
	if err := s.deliverSMS(msg); err != nil {
		msg.Attempts = 1
		msg.LastAttemptAt = now
		s.store.PutSMS(msg) // remains pending
		return msg, nil
	}

	msg.Status = domain.SMSStatusDelivered
	msg.Attempts = 1
	msg.LastAttemptAt = now
	msg.DeliveredAt = now
	s.store.PutSMS(msg)

	// Link the SMS to the black start.
	bs, bsErr := s.store.GetBlackStart(blackStartID)
	if bsErr == nil {
		bs.SMSMessageIDs = append(bs.SMSMessageIDs, msg.ID)
		s.store.PutBlackStart(bs)
	}
	return msg, nil
}

// ResendPendingSMS retries delivery of every pending SMS message. Messages are
// marked delivered when the delivery function succeeds. Called by the scheduler
// on a fixed interval. Returns the IDs of messages newly delivered.
func (s *Service) ResendPendingSMS() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var delivered []string
	now := s.now()
	for _, msg := range s.store.ListPendingSMS() {
		msg.Attempts++
		msg.LastAttemptAt = now
		if err := s.deliverSMS(msg); err != nil {
			s.store.PutSMS(msg) // still pending
			continue
		}
		msg.Status = domain.SMSStatusDelivered
		msg.DeliveredAt = now
		s.store.PutSMS(msg)
		delivered = append(delivered, msg.ID)
	}
	return delivered, nil
}

// ListPendingSMS returns all undelivered SMS instructions (mainly for
// diagnostics and testing).
func (s *Service) ListPendingSMS() []domain.SMSMessage {
	return s.store.ListPendingSMS()
}

// ListAllSMS returns every SMS outbox entry.
func (s *Service) ListAllSMS() []domain.SMSMessage {
	return s.store.ListAllSMS()
}
