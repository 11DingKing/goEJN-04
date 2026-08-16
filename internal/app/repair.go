package app

import (
	"fmt"

	"ejina-microgrid/internal/domain"
)

// dispatchWorkOrder transitions a draft work order to dispatched and records
// the dispatch timestamp so the scheduler can enforce the 15-minute acceptance
// timeout. The caller must hold s.mu.
func (s *Service) dispatchWorkOrder(id string) (domain.WorkOrder, error) {
	wo, err := s.store.GetWorkOrder(id)
	if err != nil {
		return domain.WorkOrder{}, err
	}
	if wo.Status != domain.WorkOrderStatusDraft {
		return wo, nil // idempotent: already dispatched or beyond
	}
	now := s.now()
	wo.Status = domain.WorkOrderStatusDispatched
	wo.DispatchedAt = now
	wo.UpdatedAt = now
	s.store.PutWorkOrder(wo)
	return wo, nil
}

// DispatchWorkOrder is the public entry point for dispatching a draft work
// order. It is idempotent: calling it on an already-dispatched order returns
// the existing record without error.
func (s *Service) DispatchWorkOrder(id string) (domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dispatchWorkOrder(id)
}

// AcceptWorkOrder records acceptance by a repair team member. If the order has
// already been accepted the call is idempotent and returns the existing record.
// Only dispatched or escalated orders may be accepted.
func (s *Service) AcceptWorkOrder(id, assignee string) (domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wo, err := s.store.GetWorkOrder(id)
	if err != nil {
		return domain.WorkOrder{}, err
	}

	switch wo.Status {
	case domain.WorkOrderStatusAccepted, domain.WorkOrderStatusInProgress:
		if wo.AssignedTo == assignee {
			return wo, nil // idempotent: same assignee re-sending
		}
		return domain.WorkOrder{}, fmt.Errorf("%w: work order %s already accepted by %s", domain.ErrAlreadyAccepted, id, wo.AssignedTo)
	case domain.WorkOrderStatusDispatched, domain.WorkOrderStatusEscalated:
		// allowed
	default:
		return domain.WorkOrder{}, fmt.Errorf("%w: cannot accept from %s", domain.ErrInvalidTransition, wo.Status)
	}

	now := s.now()
	wo.Status = domain.WorkOrderStatusAccepted
	wo.AssignedTo = assignee
	wo.AcceptedAt = now
	wo.UpdatedAt = now
	s.store.PutWorkOrder(wo)
	return wo, nil
}

// StartWorkOrder moves an accepted order into in_progress.
func (s *Service) StartWorkOrder(id string) (domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wo, err := s.store.GetWorkOrder(id)
	if err != nil {
		return domain.WorkOrder{}, err
	}
	if wo.Status == domain.WorkOrderStatusInProgress {
		return wo, nil // idempotent
	}
	if !wo.Status.CanTransitionTo(domain.WorkOrderStatusInProgress) {
		return domain.WorkOrder{}, fmt.Errorf("%w: %s → in_progress", domain.ErrInvalidTransition, wo.Status)
	}
	wo.Status = domain.WorkOrderStatusInProgress
	wo.UpdatedAt = s.now()
	s.store.PutWorkOrder(wo)
	return wo, nil
}

// WorsenWorkOrder reports that equipment condition has deteriorated during
// repair. It transitions the order to worsened, automatically creates an
// associated child work order for the worsened condition, and notifies the
// backup repair team.
func (s *Service) WorsenWorkOrder(id, description string) (domain.WorkOrder, domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wo, err := s.store.GetWorkOrder(id)
	if err != nil {
		return domain.WorkOrder{}, domain.WorkOrder{}, err
	}
	if !wo.Status.CanTransitionTo(domain.WorkOrderStatusWorsened) {
		return domain.WorkOrder{}, domain.WorkOrder{}, fmt.Errorf("%w: %s → worsened", domain.ErrInvalidTransition, wo.Status)
	}

	now := s.now()
	wo.Status = domain.WorkOrderStatusWorsened
	wo.BackupNotified = true
	wo.UpdatedAt = now
	s.store.PutWorkOrder(wo)

	// Auto-create associated work order for the worsened condition.
	child := domain.WorkOrder{
		ID:            s.ids.Next("wo"),
		Title:         fmt.Sprintf("异常恶化关联工单- %s", wo.EntityID),
		EntityType:    wo.EntityType,
		EntityID:      wo.EntityID,
		Status:        domain.WorkOrderStatusDraft,
		Description:   description,
		ParentOrderID: wo.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.store.SaveWorkOrder(child); err != nil {
		return wo, domain.WorkOrder{}, err
	}
	child, _ = s.dispatchWorkOrder(child.ID)

	// Notify backup repair team (fire-and-forget hook).
	s.notifyBackup(child.ID)

	return wo, child, nil
}

// CompleteWorkOrder closes a repair order.
func (s *Service) CompleteWorkOrder(id string) (domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wo, err := s.store.GetWorkOrder(id)
	if err != nil {
		return domain.WorkOrder{}, err
	}
	if wo.Status == domain.WorkOrderStatusCompleted {
		return wo, nil // idempotent
	}
	if !wo.Status.CanTransitionTo(domain.WorkOrderStatusCompleted) {
		return domain.WorkOrder{}, fmt.Errorf("%w: %s → completed", domain.ErrInvalidTransition, wo.Status)
	}
	now := s.now()
	wo.Status = domain.WorkOrderStatusCompleted
	wo.CompletedAt = now
	wo.UpdatedAt = now
	s.store.PutWorkOrder(wo)
	return wo, nil
}

// CancelWorkOrder cancels a non-terminal order.
func (s *Service) CancelWorkOrder(id string) (domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wo, err := s.store.GetWorkOrder(id)
	if err != nil {
		return domain.WorkOrder{}, err
	}
	if !wo.Status.CanTransitionTo(domain.WorkOrderStatusCancelled) {
		return domain.WorkOrder{}, fmt.Errorf("%w: %s → cancelled", domain.ErrInvalidTransition, wo.Status)
	}
	wo.Status = domain.WorkOrderStatusCancelled
	wo.UpdatedAt = s.now()
	s.store.PutWorkOrder(wo)
	return wo, nil
}

// EscalateTimedOutWorkOrders scans for dispatched work orders whose acceptance
// timeout has expired and escalates them to the team leader. Returns the IDs of
// escalated orders. Called by the scheduler.
func (s *Service) EscalateTimedOutWorkOrders() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	var escalated []string
	for _, wo := range s.store.ListWorkOrders() {
		if wo.Status != domain.WorkOrderStatusDispatched {
			continue
		}
		if now.Sub(wo.DispatchedAt) > s.cfg.WorkOrderAcceptTimeout {
			wo.Status = domain.WorkOrderStatusEscalated
			wo.EscalatedTo = "team_leader"
			wo.UpdatedAt = now
			s.store.PutWorkOrder(wo)
			escalated = append(escalated, wo.ID)
		}
	}
	return escalated, nil
}
