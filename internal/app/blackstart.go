package app

import (
	"fmt"

	"ejina-microgrid/internal/domain"
)

// InitiateBlackStart creates a new black-start sequence after a power outage.
// The bus-voltage restoration deadline is set to now + BlackStartDeadline
// (twenty minutes in production). If an active (non-terminal) black start
// already exists the call is idempotent and returns the existing record.
func (s *Service) InitiateBlackStart(initiator string) (domain.BlackStart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Idempotency: return existing active black start if one exists.
	for _, bs := range s.store.ListBlackStarts() {
		if !bs.Status.Terminal() {
			return bs, nil
		}
	}

	now := s.now()
	outageAt := now
	bs := domain.BlackStart{
		ID:          s.ids.Next("bs"),
		Status:      domain.BlackStartStatusInitiated,
		Initiator:   initiator,
		OutageAt:    outageAt,
		InitiatedAt: now,
		Deadline:    now.Add(s.cfg.BlackStartDeadline),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.SaveBlackStart(bs); err != nil {
		return domain.BlackStart{}, err
	}
	return bs, nil
}

// StartBlackStart transitions a black start from initiated to in_progress and
// enqueues the start instruction via the SMS outbox.
func (s *Service) StartBlackStart(id string) (domain.BlackStart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs, err := s.store.GetBlackStart(id)
	if err != nil {
		return domain.BlackStart{}, err
	}
	if bs.Status == domain.BlackStartStatusInProgress {
		return bs, nil // idempotent
	}
	if !bs.Status.CanTransitionTo(domain.BlackStartStatusInProgress) {
		return domain.BlackStart{}, fmt.Errorf("%w: %s → in_progress", domain.ErrInvalidTransition, bs.Status)
	}
	bs.Status = domain.BlackStartStatusInProgress
	bs.UpdatedAt = s.now()
	s.store.PutBlackStart(bs)

	if _, err := s.enqueueSMS(id, "BLACK_START_BEGIN:启动柴油机组,建立独立电源"); err != nil {
		return bs, err
	}
	return bs, nil
}

// RestoreBus records that bus voltage has been restored. This must happen
// before the deadline; otherwise the black start is failed by the scheduler.
func (s *Service) RestoreBus(id string) (domain.BlackStart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs, err := s.store.GetBlackStart(id)
	if err != nil {
		return domain.BlackStart{}, err
	}
	if bs.Status == domain.BlackStartStatusBusRestored {
		return bs, nil // idempotent
	}
	now := s.now()
	if bs.IsOverdue(now) {
		_ = s.failBlackStart(bs, "bus restoration deadline exceeded")
		return bs, fmt.Errorf("%w: black start %s", domain.ErrDeadlineExceeded, id)
	}
	if !bs.Status.CanTransitionTo(domain.BlackStartStatusBusRestored) {
		return domain.BlackStart{}, fmt.Errorf("%w: %s → bus_restored", domain.ErrInvalidTransition, bs.Status)
	}
	bs.Status = domain.BlackStartStatusBusRestored
	bs.BusRestoredAt = now
	bs.UpdatedAt = now
	s.store.PutBlackStart(bs)

	if _, err := s.enqueueSMS(id, "BUS_RESTORED:母线电压恢复正常,准备并网"); err != nil {
		return bs, err
	}
	return bs, nil
}

// RequestGridConnection creates a grid-connection request associated with a
// black start and begins the auto-synchronous grid connection workflow. When
// off-grid operation is concurrent (the black start has not yet completed),
// priority-based load shedding is applied: level-1 loads (hospital, water
// supply) are preserved and all others are shed in sequence.
func (s *Service) RequestGridConnection(blackStartID string) (domain.GridConnection, []domain.Load, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs, err := s.store.GetBlackStart(blackStartID)
	if err != nil {
		return domain.GridConnection{}, nil, err
	}
	if bs.Status != domain.BlackStartStatusBusRestored && bs.Status != domain.BlackStartStatusGridSyncing {
		return domain.GridConnection{}, nil, fmt.Errorf("%w: black start must be bus_restored to request grid connection (current: %s)", domain.ErrInvalidTransition, bs.Status)
	}

	now := s.now()
	offGrid := !bs.Status.Terminal()

	gc := domain.GridConnection{
		ID:           s.ids.Next("gc"),
		BlackStartID: blackStartID,
		Status:       domain.GridConnStatusRequested,
		OffGrid:      offGrid,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.store.SaveGridConn(gc); err != nil {
		return domain.GridConnection{}, nil, err
	}

	// Transition black start to grid_syncing.
	if bs.Status.CanTransitionTo(domain.BlackStartStatusGridSyncing) {
		bs.Status = domain.BlackStartStatusGridSyncing
		bs.GridConnID = gc.ID
		bs.UpdatedAt = now
		s.store.PutBlackStart(bs)
	} else {
		bs.GridConnID = gc.ID
		bs.UpdatedAt = now
		s.store.PutBlackStart(bs)
	}

	// When off-grid and grid connection are concurrent, shed non-critical
	// loads to protect level-1 consumers (hospitals, water supply).
	var shedLoads []domain.Load
	if offGrid {
		loads := s.store.ListLoads()
		shed, _ := domain.ShedLoadsByPriority(loads, 1, now)
		for _, l := range shed {
			s.store.PutLoad(l)
		}
		shedLoads = shed
	}

	if _, err := s.enqueueSMS(blackStartID, "GRID_SYNC_REQUEST:并网申请已发起,等待双方复核签字"); err != nil {
		return gc, shedLoads, err
	}
	return gc, shedLoads, nil
}

// SignGridConnection records a sign-off from either the dispatch or operations
// party. Both parties must sign before synchronization can proceed. Signing is
// idempotent: signing twice for the same party is a no-op.
func (s *Service) SignGridConnection(gridConnID string, party domain.SignParty, signer string) (domain.GridConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gc, err := s.store.GetGridConn(gridConnID)
	if err != nil {
		return domain.GridConnection{}, err
	}
	if gc.Status.Terminal() {
		return gc, nil // idempotent for terminal states
	}

	now := s.now()
	switch party {
	case domain.SignPartyDispatch:
		if gc.DispatchSigned {
			return gc, nil // idempotent
		}
		gc.DispatchSigned = true
		gc.DispatchSignedBy = signer
		gc.DispatchSignedAt = now
	case domain.SignPartyOM:
		if gc.OMSigned {
			return gc, nil // idempotent
		}
		gc.OMSigned = true
		gc.OMSignedBy = signer
		gc.OMSignedAt = now
	default:
		return domain.GridConnection{}, fmt.Errorf("unknown sign party: %s", party)
	}

	// Update status based on sign-off progress.
	if gc.DispatchSigned && gc.OMSigned {
		gc.Status = domain.GridConnStatusDualSigned
	} else if gc.DispatchSigned {
		gc.Status = domain.GridConnStatusDispatchSigned
	} else {
		gc.Status = domain.GridConnStatusOMSigned
	}
	gc.UpdatedAt = now
	s.store.PutGridConn(gc)

	return gc, nil
}

// SynchronizeGrid performs the auto-synchronous grid connection. Both parties
// must have signed off first.
func (s *Service) SynchronizeGrid(gridConnID string) (domain.GridConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gc, err := s.store.GetGridConn(gridConnID)
	if err != nil {
		return domain.GridConnection{}, err
	}
	if gc.Status == domain.GridConnStatusSynchronized || gc.Status == domain.GridConnStatusCompleted {
		return gc, nil // idempotent
	}
	if !gc.BothSigned() {
		return domain.GridConnection{}, fmt.Errorf("%w: both dispatch and operations must sign", domain.ErrDualSignRequired)
	}
	if !gc.Status.CanTransitionTo(domain.GridConnStatusSynchronized) {
		return domain.GridConnection{}, fmt.Errorf("%w: %s → synchronized", domain.ErrInvalidTransition, gc.Status)
	}
	now := s.now()
	gc.Status = domain.GridConnStatusSynchronized
	gc.SynchronizedAt = now
	gc.UpdatedAt = now
	s.store.PutGridConn(gc)

	if _, err := s.enqueueSMS(gc.BlackStartID, "GRID_SYNCHRONIZED:自动同期并网完成"); err != nil {
		return gc, err
	}
	return gc, nil
}

// CompleteBlackStart finalises the black-start sequence after grid
// synchronization is complete.
func (s *Service) CompleteBlackStart(blackStartID string) (domain.BlackStart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs, err := s.store.GetBlackStart(blackStartID)
	if err != nil {
		return domain.BlackStart{}, err
	}
	if bs.Status == domain.BlackStartStatusCompleted {
		return bs, nil // idempotent
	}
	if !bs.Status.CanTransitionTo(domain.BlackStartStatusCompleted) {
		return domain.BlackStart{}, fmt.Errorf("%w: %s → completed", domain.ErrInvalidTransition, bs.Status)
	}

	// Verify the associated grid connection is synchronized.
	if bs.GridConnID != "" {
		gc, gcErr := s.store.GetGridConn(bs.GridConnID)
		if gcErr == nil && gc.Status != domain.GridConnStatusSynchronized && gc.Status != domain.GridConnStatusCompleted {
			return domain.BlackStart{}, fmt.Errorf("grid connection %s not synchronized (status: %s)", bs.GridConnID, gc.Status)
		}
	}

	now := s.now()
	bs.Status = domain.BlackStartStatusCompleted
	bs.CompletedAt = now
	bs.UpdatedAt = now
	s.store.PutBlackStart(bs)

	// Complete the grid connection too.
	if bs.GridConnID != "" {
		gc, gcErr := s.store.GetGridConn(bs.GridConnID)
		if gcErr == nil && gc.Status.CanTransitionTo(domain.GridConnStatusCompleted) {
			gc.Status = domain.GridConnStatusCompleted
			gc.CompletedAt = now
			gc.UpdatedAt = now
			s.store.PutGridConn(gc)
		}
	}
	return bs, nil
}

// ReportNetworkInterrupt flags the black start as network-interrupted. The SMS
// outbox retains all pending instructions; the scheduler will retry them.
func (s *Service) ReportNetworkInterrupt(blackStartID string) (domain.BlackStart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs, err := s.store.GetBlackStart(blackStartID)
	if err != nil {
		return domain.BlackStart{}, err
	}
	bs.NetworkInterrupted = true
	bs.UpdatedAt = s.now()
	s.store.PutBlackStart(bs)
	return bs, nil
}

// RestoreNetwork clears the network-interrupted flag and immediately retries
// pending SMS messages for the affected black start.
func (s *Service) RestoreNetwork(blackStartID string) (domain.BlackStart, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bs, err := s.store.GetBlackStart(blackStartID)
	if err != nil {
		return domain.BlackStart{}, nil, err
	}
	bs.NetworkInterrupted = false
	bs.UpdatedAt = s.now()
	s.store.PutBlackStart(bs)

	// Retry all pending SMS for this black start.
	var delivered []string
	now := s.now()
	for _, msg := range s.store.ListPendingSMS() {
		if msg.BlackStartID != blackStartID {
			continue
		}
		msg.Attempts++
		msg.LastAttemptAt = now
		if err := s.deliverSMS(msg); err != nil {
			s.store.PutSMS(msg)
			continue
		}
		msg.Status = domain.SMSStatusDelivered
		msg.DeliveredAt = now
		s.store.PutSMS(msg)
		delivered = append(delivered, msg.ID)
	}
	return bs, delivered, nil
}

// FailExpiredBlackStarts scans for in-progress black starts whose bus
// restoration deadline has passed and marks them as failed. Called by the
// scheduler.
func (s *Service) FailExpiredBlackStarts() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	var failed []string
	for _, bs := range s.store.ListBlackStarts() {
		if bs.IsOverdue(now) {
			s.failBlackStart(bs, "bus restoration deadline exceeded")
			failed = append(failed, bs.ID)
		}
	}
	return failed, nil
}

// failBlackStart transitions a black start to failed. The caller must hold s.mu.
func (s *Service) failBlackStart(bs domain.BlackStart, reason string) error {
	if !bs.Status.CanTransitionTo(domain.BlackStartStatusFailed) {
		return nil
	}
	bs.Status = domain.BlackStartStatusFailed
	bs.UpdatedAt = s.now()
	s.store.PutBlackStart(bs)
	_, _ = s.enqueueSMS(bs.ID, "BLACK_START_FAILED: "+reason)
	return nil
}
