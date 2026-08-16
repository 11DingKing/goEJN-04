package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"ejina-microgrid/internal/domain"
)

func TestStoreBatteryCabinCRUD(t *testing.T) {
	s := New()
	bc := domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1", CellTempC: 30, SOC: 80}
	if err := s.SaveBatteryCabin(bc); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.SaveBatteryCabin(bc); err == nil {
		t.Error("duplicate save should fail")
	}
	got, err := s.GetBatteryCabin("bc-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Cabin-1" {
		t.Errorf("name = %s, want Cabin-1", got.Name)
	}
	got.CellTempC = 50
	s.PutBatteryCabin(got)
	updated, _ := s.GetBatteryCabin("bc-1")
	if updated.CellTempC != 50 {
		t.Errorf("temp = %v, want 50", updated.CellTempC)
	}
	if _, err := s.GetBatteryCabin("nonexistent"); !errors.Is(err, domain.ErrEntityNotFound) {
		t.Errorf("get nonexistent should return ErrEntityNotFound, got %v", err)
	}
}

func TestStoreWorkOrderCRUD(t *testing.T) {
	s := New()
	wo := domain.WorkOrder{ID: "wo-1", Title: "Test", Status: domain.WorkOrderStatusDraft}
	if err := s.SaveWorkOrder(wo); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetWorkOrder("wo-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != domain.WorkOrderStatusDraft {
		t.Errorf("status = %s, want draft", got.Status)
	}
	got.Status = domain.WorkOrderStatusDispatched
	s.PutWorkOrder(got)
	updated, _ := s.GetWorkOrder("wo-1")
	if updated.Status != domain.WorkOrderStatusDispatched {
		t.Errorf("status = %s, want dispatched", updated.Status)
	}
}

func TestStoreSMSPendingFilter(t *testing.T) {
	s := New()
	s.SaveSMS(domain.SMSMessage{ID: "sms-1", Status: domain.SMSStatusPending})
	s.SaveSMS(domain.SMSMessage{ID: "sms-2", Status: domain.SMSStatusDelivered})
	s.SaveSMS(domain.SMSMessage{ID: "sms-3", Status: domain.SMSStatusPending})
	pending := s.ListPendingSMS()
	if len(pending) != 2 {
		t.Errorf("pending count = %d, want 2", len(pending))
	}
	all := s.ListAllSMS()
	if len(all) != 3 {
		t.Errorf("total count = %d, want 3", len(all))
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			bc := domain.BatteryCabin{ID: fmt.Sprintf("bc-%d", n)}
			_ = s.SaveBatteryCabin(bc)
			_, _ = s.GetBatteryCabin(fmt.Sprintf("bc-%d", n))
			_ = s.ListBatteryCabins()
		}(i)
	}
	wg.Wait()
	if len(s.ListBatteryCabins()) != 100 {
		t.Errorf("expected 100 cabins, got %d", len(s.ListBatteryCabins()))
	}
}

// TestStoreMutate verifies that a compound read-modify-write sequence performed
// inside Mutate works when using the lock-free accessors. The callback must use
// getWorkOrder/putWorkOrder rather than the public methods because sync.Mutex
// is not reentrant: the public methods would self-deadlock trying to re-acquire
// the mutex already held by Mutate.
func TestStoreMutate(t *testing.T) {
	s := New()
	s.SaveWorkOrder(domain.WorkOrder{ID: "wo-1", Status: domain.WorkOrderStatusDraft})
	s.SaveWorkOrder(domain.WorkOrder{ID: "wo-2", Status: domain.WorkOrderStatusDraft})

	s.Mutate(func() {
		wo, _ := s.getWorkOrder("wo-1")
		wo.Status = domain.WorkOrderStatusDispatched
		s.putWorkOrder(wo)
	})
	got, _ := s.GetWorkOrder("wo-1")
	if got.Status != domain.WorkOrderStatusDispatched {
		t.Errorf("status = %s, want dispatched", got.Status)
	}
}

// TestStoreMutateAtomicity verifies that Mutate serialises concurrent
// read-modify-write sequences so that no updates are lost. N goroutines each
// read a battery cabin's temperature, increment it by one, and write it back,
// all inside Mutate. Because Mutate holds the store lock across each
// read-modify-write, the final value must equal N exactly; any lost update
// would produce a smaller value.
func TestStoreMutateAtomicity(t *testing.T) {
	s := New()
	s.SaveBatteryCabin(domain.BatteryCabin{ID: "bc-1", CellTempC: 0})

	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Mutate(func() {
				bc, _ := s.getBatteryCabin("bc-1")
				bc.CellTempC += 1
				s.putBatteryCabin(bc)
			})
		}()
	}
	wg.Wait()

	got, err := s.GetBatteryCabin("bc-1")
	if err != nil {
		t.Fatalf("get after concurrent mutate: %v", err)
	}
	if got.CellTempC != float64(goroutines) {
		t.Errorf("CellTempC = %v, want %d (lost updates detected)", got.CellTempC, goroutines)
	}
}

func TestStoreSeedLoads(t *testing.T) {
	s := New()
	loads := []domain.Load{
		{ID: "l1", Priority: 1, Energized: true},
		{ID: "l2", Priority: 2, Energized: true},
	}
	s.SeedLoads(loads)
	if len(s.ListLoads()) != 2 {
		t.Errorf("expected 2 loads, got %d", len(s.ListLoads()))
	}
	// Second seed should be a no-op.
	s.SeedLoads([]domain.Load{{ID: "l3", Priority: 3}})
	if len(s.ListLoads()) != 2 {
		t.Errorf("second seed should not add loads, got %d", len(s.ListLoads()))
	}
}
