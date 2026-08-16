package app

import (
	"sync"
	"time"

	"ejina-microgrid/internal/config"
	"ejina-microgrid/internal/domain"
	"ejina-microgrid/internal/store"
)

// Service is the application layer that orchestrates all four microgrid
// workflows: inspection, anomaly reporting, repair closed-loop, and black-start
// with auto-synchronous grid connection. It owns a mutex that serialises
// multi-step state transitions so that concurrent callers cannot interleave
// read-modify-write sequences.
type Service struct {
	store *store.Store
	clock Clock
	cfg   *config.Config
	ids   IDGenerator
	mu    sync.Mutex

	// backupNotifier is invoked when a work order worsens and the backup
	// repair team must be alerted. Defaults to a no-op.
	backupNotifier func(workOrderID string)

	// smsDeliverer attempts to deliver a single SMS instruction. Returning a
	// nil error marks the message as delivered; a non-nil error leaves it
	// pending for retry. Defaults to always-succeed.
	smsDeliverer func(msg domain.SMSMessage) error

	// smsMu protects the smsDeliverer / backupNotifier hooks so they can be
	// swapped at runtime (mainly in tests).
	hookMu sync.RWMutex
}

// NewService creates a Service backed by the given store, clock and config.
func NewService(s *store.Store, clock Clock, cfg *config.Config, ids IDGenerator) *Service {
	svc := &Service{
		store: s,
		clock: clock,
		cfg:   cfg,
		ids:   ids,
	}
	svc.backupNotifier = func(string) {}
	svc.smsDeliverer = func(domain.SMSMessage) error { return nil }
	return svc
}

// SetBackupNotifier replaces the backup-team notification hook.
func (s *Service) SetBackupNotifier(fn func(workOrderID string)) {
	s.hookMu.Lock()
	defer s.hookMu.Unlock()
	s.backupNotifier = fn
}

// SetSMSDeliverer replaces the SMS delivery function.
func (s *Service) SetSMSDeliverer(fn func(domain.SMSMessage) error) {
	s.hookMu.Lock()
	defer s.hookMu.Unlock()
	s.smsDeliverer = fn
}

func (s *Service) notifyBackup(workOrderID string) {
	s.hookMu.RLock()
	fn := s.backupNotifier
	s.hookMu.RUnlock()
	if fn != nil {
		fn(workOrderID)
	}
}

func (s *Service) deliverSMS(msg domain.SMSMessage) error {
	s.hookMu.RLock()
	fn := s.smsDeliverer
	s.hookMu.RUnlock()
	if fn != nil {
		return fn(msg)
	}
	return nil
}

// now returns the current time from the injected clock.
func (s *Service) now() time.Time { return s.clock.Now() }

// ---------------------------------------------------------------------------
// Entity management
// ---------------------------------------------------------------------------

// RegisterBatteryCabin adds a new battery cabin to the fleet.
func (s *Service) RegisterBatteryCabin(bc domain.BatteryCabin) error {
	bc.UpdatedAt = s.now()
	return s.store.SaveBatteryCabin(bc)
}

// UpdateBatteryCabinTelemetry updates temperature and SOC, then evaluates the
// thermal alarm rule. Returns true when an alarm was raised or is already
// active.
func (s *Service) UpdateBatteryCabinTelemetry(id string, tempC, soc float64) (domain.BatteryCabin, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bc, err := s.store.GetBatteryCabin(id)
	if err != nil {
		return domain.BatteryCabin{}, false, err
	}
	bc.CellTempC = tempC
	bc.SOC = soc
	alarmed := bc.EvaluateTemperature(s.cfg.BatteryTempThresholdC, s.now())
	s.store.PutBatteryCabin(bc)
	return bc, alarmed, nil
}

// RegisterPowerStation adds a new wind/solar station.
func (s *Service) RegisterPowerStation(ps domain.PowerStation) error {
	ps.UpdatedAt = s.now()
	return s.store.SavePowerStation(ps)
}

// RegisterDieselGen adds a new diesel generator.
func (s *Service) RegisterDieselGen(dg domain.DieselGenerator) error {
	dg.UpdatedAt = s.now()
	return s.store.SaveDieselGen(dg)
}

// RegisterLoad adds a new consumer load.
func (s *Service) RegisterLoad(l domain.Load) error {
	l.UpdatedAt = s.now()
	return s.store.SaveLoad(l)
}

// SeedDefaultLoads populates the standard priority loads if the store is empty.
func (s *Service) SeedDefaultLoads() {
	now := s.now()
	loads := []domain.Load{
		{ID: "load-hospital", Name: "额济纳旗人民医院", Priority: 1, Energized: true, UpdatedAt: now},
		{ID: "load-water", Name: "自来水厂", Priority: 1, Energized: true, UpdatedAt: now},
		{ID: "load-residential", Name: "居民生活区", Priority: 2, Energized: true, UpdatedAt: now},
		{ID: "load-commercial", Name: "商业区", Priority: 2, Energized: true, UpdatedAt: now},
		{ID: "load-industrial", Name: "工业园区", Priority: 3, Energized: true, UpdatedAt: now},
		{ID: "load-agricultural", Name: "农业灌溉", Priority: 3, Energized: true, UpdatedAt: now},
	}
	s.store.SeedLoads(loads)
}

// GetBatteryCabin delegates to the store.
func (s *Service) GetBatteryCabin(id string) (domain.BatteryCabin, error) {
	return s.store.GetBatteryCabin(id)
}

// ListBatteryCabins delegates to the store.
func (s *Service) ListBatteryCabins() []domain.BatteryCabin {
	return s.store.ListBatteryCabins()
}

// ListLoads delegates to the store.
func (s *Service) ListLoads() []domain.Load {
	return s.store.ListLoads()
}

// ListWorkOrders delegates to the store.
func (s *Service) ListWorkOrders() []domain.WorkOrder {
	return s.store.ListWorkOrders()
}

// GetWorkOrder delegates to the store.
func (s *Service) GetWorkOrder(id string) (domain.WorkOrder, error) {
	return s.store.GetWorkOrder(id)
}

// ListBlackStarts delegates to the store.
func (s *Service) ListBlackStarts() []domain.BlackStart {
	return s.store.ListBlackStarts()
}

// GetBlackStart delegates to the store.
func (s *Service) GetBlackStart(id string) (domain.BlackStart, error) {
	return s.store.GetBlackStart(id)
}

// ListGridConns delegates to the store.
func (s *Service) ListGridConns() []domain.GridConnection {
	return s.store.ListGridConns()
}

// GetGridConn delegates to the store.
func (s *Service) GetGridConn(id string) (domain.GridConnection, error) {
	return s.store.GetGridConn(id)
}

// ListInspections delegates to the store.
func (s *Service) ListInspections() []domain.Inspection {
	return s.store.ListInspections()
}

// Config exposes the active configuration (read-only by convention).
func (s *Service) Config() *config.Config { return s.cfg }
