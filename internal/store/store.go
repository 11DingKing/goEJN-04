package store

import (
	"fmt"
	"sync"

	"ejina-microgrid/internal/domain"
)

// Store is a thread-safe in-memory persistence layer for all microgrid domain
// objects. A single mutex guards every collection so that callers may compose
// read-modify-write sequences atomically. Two usage patterns are supported:
//
//   - Single operations: call the public methods (GetWorkOrder, PutWorkOrder, …),
//     each of which acquires the mutex for the duration of the call.
//   - Compound operations: wrap multiple steps in Mutate and use the lock-free
//     accessors (getWorkOrder, putWorkOrder, …) inside the callback. The public
//     methods must NOT be called from within a Mutate callback because
//     sync.Mutex is not reentrant and doing so self-deadlocks.
type Store struct {
	mu            sync.Mutex
	batteryCabins map[string]domain.BatteryCabin
	powerStations map[string]domain.PowerStation
	dieselGens    map[string]domain.DieselGenerator
	loads         map[string]domain.Load
	inspections   map[string]domain.Inspection
	workOrders    map[string]domain.WorkOrder
	blackStarts   map[string]domain.BlackStart
	gridConns     map[string]domain.GridConnection
	smsMessages   map[string]domain.SMSMessage
}

// New returns an empty store ready for use.
func New() *Store {
	return &Store{
		batteryCabins: make(map[string]domain.BatteryCabin),
		powerStations: make(map[string]domain.PowerStation),
		dieselGens:    make(map[string]domain.DieselGenerator),
		loads:         make(map[string]domain.Load),
		inspections:   make(map[string]domain.Inspection),
		workOrders:    make(map[string]domain.WorkOrder),
		blackStarts:   make(map[string]domain.BlackStart),
		gridConns:     make(map[string]domain.GridConnection),
		smsMessages:   make(map[string]domain.SMSMessage),
	}
}

// Mutate runs fn while holding the exclusive store lock, allowing callers to
// perform atomic multi-step read-modify-write sequences. Inside fn, callers
// must use the lock-free accessors (e.g. getWorkOrder, putWorkOrder) rather than
// the public methods, which would deadlock trying to re-acquire the mutex.
func (s *Store) Mutate(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn()
}

// ---------------------------------------------------------------------------
// Battery cabins
// ---------------------------------------------------------------------------

func (s *Store) SaveBatteryCabin(bc domain.BatteryCabin) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveBatteryCabin(bc)
}

func (s *Store) saveBatteryCabin(bc domain.BatteryCabin) error {
	if _, ok := s.batteryCabins[bc.ID]; ok {
		return fmt.Errorf("%w: battery cabin %s", domain.ErrDuplicateID, bc.ID)
	}
	s.batteryCabins[bc.ID] = bc
	return nil
}

func (s *Store) PutBatteryCabin(bc domain.BatteryCabin) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putBatteryCabin(bc)
}

func (s *Store) putBatteryCabin(bc domain.BatteryCabin) {
	s.batteryCabins[bc.ID] = bc
}

func (s *Store) GetBatteryCabin(id string) (domain.BatteryCabin, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getBatteryCabin(id)
}

func (s *Store) getBatteryCabin(id string) (domain.BatteryCabin, error) {
	bc, ok := s.batteryCabins[id]
	if !ok {
		return domain.BatteryCabin{}, fmt.Errorf("%w: %s", domain.ErrEntityNotFound, id)
	}
	return bc, nil
}

func (s *Store) ListBatteryCabins() []domain.BatteryCabin {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listBatteryCabins()
}

func (s *Store) listBatteryCabins() []domain.BatteryCabin {
	out := make([]domain.BatteryCabin, 0, len(s.batteryCabins))
	for _, bc := range s.batteryCabins {
		out = append(out, bc)
	}
	return out
}

// ---------------------------------------------------------------------------
// Power stations
// ---------------------------------------------------------------------------

func (s *Store) SavePowerStation(ps domain.PowerStation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.savePowerStation(ps)
}

func (s *Store) savePowerStation(ps domain.PowerStation) error {
	if _, ok := s.powerStations[ps.ID]; ok {
		return fmt.Errorf("%w: power station %s", domain.ErrDuplicateID, ps.ID)
	}
	s.powerStations[ps.ID] = ps
	return nil
}

func (s *Store) GetPowerStation(id string) (domain.PowerStation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getPowerStation(id)
}

func (s *Store) getPowerStation(id string) (domain.PowerStation, error) {
	ps, ok := s.powerStations[id]
	if !ok {
		return domain.PowerStation{}, fmt.Errorf("%w: %s", domain.ErrEntityNotFound, id)
	}
	return ps, nil
}

func (s *Store) ListPowerStations() []domain.PowerStation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listPowerStations()
}

func (s *Store) listPowerStations() []domain.PowerStation {
	out := make([]domain.PowerStation, 0, len(s.powerStations))
	for _, ps := range s.powerStations {
		out = append(out, ps)
	}
	return out
}

// ---------------------------------------------------------------------------
// Diesel generators
// ---------------------------------------------------------------------------

func (s *Store) SaveDieselGen(dg domain.DieselGenerator) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveDieselGen(dg)
}

func (s *Store) saveDieselGen(dg domain.DieselGenerator) error {
	if _, ok := s.dieselGens[dg.ID]; ok {
		return fmt.Errorf("%w: diesel generator %s", domain.ErrDuplicateID, dg.ID)
	}
	s.dieselGens[dg.ID] = dg
	return nil
}

func (s *Store) GetDieselGen(id string) (domain.DieselGenerator, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getDieselGen(id)
}

func (s *Store) getDieselGen(id string) (domain.DieselGenerator, error) {
	dg, ok := s.dieselGens[id]
	if !ok {
		return domain.DieselGenerator{}, fmt.Errorf("%w: %s", domain.ErrEntityNotFound, id)
	}
	return dg, nil
}

func (s *Store) ListDieselGens() []domain.DieselGenerator {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listDieselGens()
}

func (s *Store) listDieselGens() []domain.DieselGenerator {
	out := make([]domain.DieselGenerator, 0, len(s.dieselGens))
	for _, dg := range s.dieselGens {
		out = append(out, dg)
	}
	return out
}

// ---------------------------------------------------------------------------
// Loads
// ---------------------------------------------------------------------------

func (s *Store) SaveLoad(l domain.Load) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLoad(l)
}

func (s *Store) saveLoad(l domain.Load) error {
	if _, ok := s.loads[l.ID]; ok {
		return fmt.Errorf("%w: load %s", domain.ErrDuplicateID, l.ID)
	}
	s.loads[l.ID] = l
	return nil
}

func (s *Store) PutLoad(l domain.Load) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putLoad(l)
}

func (s *Store) putLoad(l domain.Load) {
	s.loads[l.ID] = l
}

func (s *Store) GetLoad(id string) (domain.Load, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLoad(id)
}

func (s *Store) getLoad(id string) (domain.Load, error) {
	l, ok := s.loads[id]
	if !ok {
		return domain.Load{}, fmt.Errorf("%w: %s", domain.ErrLoadNotFound, id)
	}
	return l, nil
}

func (s *Store) ListLoads() []domain.Load {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listLoads()
}

func (s *Store) listLoads() []domain.Load {
	out := make([]domain.Load, 0, len(s.loads))
	for _, l := range s.loads {
		out = append(out, l)
	}
	return out
}

// ---------------------------------------------------------------------------
// Inspections
// ---------------------------------------------------------------------------

func (s *Store) SaveInspection(ins domain.Inspection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveInspection(ins)
}

func (s *Store) saveInspection(ins domain.Inspection) error {
	if _, ok := s.inspections[ins.ID]; ok {
		return fmt.Errorf("%w: inspection %s", domain.ErrDuplicateID, ins.ID)
	}
	s.inspections[ins.ID] = ins
	return nil
}

func (s *Store) PutInspection(ins domain.Inspection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putInspection(ins)
}

func (s *Store) putInspection(ins domain.Inspection) {
	s.inspections[ins.ID] = ins
}

func (s *Store) GetInspection(id string) (domain.Inspection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getInspection(id)
}

func (s *Store) getInspection(id string) (domain.Inspection, error) {
	ins, ok := s.inspections[id]
	if !ok {
		return domain.Inspection{}, fmt.Errorf("%w: %s", domain.ErrInspectionNotFound, id)
	}
	return ins, nil
}

func (s *Store) ListInspections() []domain.Inspection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listInspections()
}

func (s *Store) listInspections() []domain.Inspection {
	out := make([]domain.Inspection, 0, len(s.inspections))
	for _, ins := range s.inspections {
		out = append(out, ins)
	}
	return out
}

// ---------------------------------------------------------------------------
// Work orders
// ---------------------------------------------------------------------------

func (s *Store) SaveWorkOrder(wo domain.WorkOrder) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveWorkOrder(wo)
}

func (s *Store) saveWorkOrder(wo domain.WorkOrder) error {
	if _, ok := s.workOrders[wo.ID]; ok {
		return fmt.Errorf("%w: work order %s", domain.ErrDuplicateID, wo.ID)
	}
	s.workOrders[wo.ID] = wo
	return nil
}

func (s *Store) PutWorkOrder(wo domain.WorkOrder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putWorkOrder(wo)
}

func (s *Store) putWorkOrder(wo domain.WorkOrder) {
	s.workOrders[wo.ID] = wo
}

func (s *Store) GetWorkOrder(id string) (domain.WorkOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getWorkOrder(id)
}

func (s *Store) getWorkOrder(id string) (domain.WorkOrder, error) {
	wo, ok := s.workOrders[id]
	if !ok {
		return domain.WorkOrder{}, fmt.Errorf("%w: %s", domain.ErrWorkOrderNotFound, id)
	}
	return wo, nil
}

func (s *Store) ListWorkOrders() []domain.WorkOrder {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listWorkOrders()
}

func (s *Store) listWorkOrders() []domain.WorkOrder {
	out := make([]domain.WorkOrder, 0, len(s.workOrders))
	for _, wo := range s.workOrders {
		out = append(out, wo)
	}
	return out
}

// ---------------------------------------------------------------------------
// Black starts
// ---------------------------------------------------------------------------

func (s *Store) SaveBlackStart(bs domain.BlackStart) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveBlackStart(bs)
}

func (s *Store) saveBlackStart(bs domain.BlackStart) error {
	if _, ok := s.blackStarts[bs.ID]; ok {
		return fmt.Errorf("%w: black start %s", domain.ErrDuplicateID, bs.ID)
	}
	s.blackStarts[bs.ID] = bs
	return nil
}

func (s *Store) PutBlackStart(bs domain.BlackStart) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putBlackStart(bs)
}

func (s *Store) putBlackStart(bs domain.BlackStart) {
	s.blackStarts[bs.ID] = bs
}

func (s *Store) GetBlackStart(id string) (domain.BlackStart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getBlackStart(id)
}

func (s *Store) getBlackStart(id string) (domain.BlackStart, error) {
	bs, ok := s.blackStarts[id]
	if !ok {
		return domain.BlackStart{}, fmt.Errorf("%w: %s", domain.ErrBlackStartNotFound, id)
	}
	return bs, nil
}

func (s *Store) ListBlackStarts() []domain.BlackStart {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listBlackStarts()
}

func (s *Store) listBlackStarts() []domain.BlackStart {
	out := make([]domain.BlackStart, 0, len(s.blackStarts))
	for _, bs := range s.blackStarts {
		out = append(out, bs)
	}
	return out
}

// ---------------------------------------------------------------------------
// Grid connections
// ---------------------------------------------------------------------------

func (s *Store) SaveGridConn(gc domain.GridConnection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveGridConn(gc)
}

func (s *Store) saveGridConn(gc domain.GridConnection) error {
	if _, ok := s.gridConns[gc.ID]; ok {
		return fmt.Errorf("%w: grid connection %s", domain.ErrDuplicateID, gc.ID)
	}
	s.gridConns[gc.ID] = gc
	return nil
}

func (s *Store) PutGridConn(gc domain.GridConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putGridConn(gc)
}

func (s *Store) putGridConn(gc domain.GridConnection) {
	s.gridConns[gc.ID] = gc
}

func (s *Store) GetGridConn(id string) (domain.GridConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getGridConn(id)
}

func (s *Store) getGridConn(id string) (domain.GridConnection, error) {
	gc, ok := s.gridConns[id]
	if !ok {
		return domain.GridConnection{}, fmt.Errorf("%w: %s", domain.ErrGridConnNotFound, id)
	}
	return gc, nil
}

func (s *Store) ListGridConns() []domain.GridConnection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listGridConns()
}

func (s *Store) listGridConns() []domain.GridConnection {
	out := make([]domain.GridConnection, 0, len(s.gridConns))
	for _, gc := range s.gridConns {
		out = append(out, gc)
	}
	return out
}

// ---------------------------------------------------------------------------
// SMS messages
// ---------------------------------------------------------------------------

func (s *Store) SaveSMS(msg domain.SMSMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveSMS(msg)
}

func (s *Store) saveSMS(msg domain.SMSMessage) error {
	if _, ok := s.smsMessages[msg.ID]; ok {
		return fmt.Errorf("%w: sms %s", domain.ErrDuplicateID, msg.ID)
	}
	s.smsMessages[msg.ID] = msg
	return nil
}

func (s *Store) PutSMS(msg domain.SMSMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putSMS(msg)
}

func (s *Store) putSMS(msg domain.SMSMessage) {
	s.smsMessages[msg.ID] = msg
}

func (s *Store) GetSMS(id string) (domain.SMSMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getSMS(id)
}

func (s *Store) getSMS(id string) (domain.SMSMessage, error) {
	msg, ok := s.smsMessages[id]
	if !ok {
		return domain.SMSMessage{}, fmt.Errorf("%w: %s", domain.ErrSMSNotFound, id)
	}
	return msg, nil
}

func (s *Store) ListPendingSMS() []domain.SMSMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listPendingSMS()
}

func (s *Store) listPendingSMS() []domain.SMSMessage {
	out := make([]domain.SMSMessage, 0)
	for _, msg := range s.smsMessages {
		if msg.Status == domain.SMSStatusPending {
			out = append(out, msg)
		}
	}
	return out
}

func (s *Store) ListAllSMS() []domain.SMSMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listAllSMS()
}

func (s *Store) listAllSMS() []domain.SMSMessage {
	out := make([]domain.SMSMessage, 0, len(s.smsMessages))
	for _, msg := range s.smsMessages {
		out = append(out, msg)
	}
	return out
}

// SeedLoads populates the store with an initial set of loads if none exist.
func (s *Store) SeedLoads(loads []domain.Load) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seedLoads(loads)
}

func (s *Store) seedLoads(loads []domain.Load) {
	if len(s.loads) > 0 {
		return
	}
	for _, l := range loads {
		s.loads[l.ID] = l
	}
}
