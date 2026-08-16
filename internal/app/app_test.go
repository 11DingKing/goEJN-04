package app

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ejina-microgrid/internal/config"
	"ejina-microgrid/internal/domain"
	"ejina-microgrid/internal/store"
)

func testConfig() *config.Config {
	return &config.Config{
		ServerPort:             57579,
		ReadTimeout:            5 * time.Second,
		WriteTimeout:           5 * time.Second,
		BatteryTempThresholdC:  45.0,
		MinSOCPercent:          15.0,
		WorkOrderAcceptTimeout: 15 * time.Minute,
		BlackStartDeadline:     20 * time.Minute,
		SMSResendInterval:      30 * time.Second,
		SchedulerInterval:      100 * time.Millisecond,
	}
}

func testService(t *testing.T) (*Service, *FakeClock) {
	t.Helper()
	st := store.New()
	clock := NewFakeClock(time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC))
	cfg := testConfig()
	svc := NewService(st, clock, cfg, NewSequentialIDGenerator())
	svc.SeedDefaultLoads()
	return svc, clock
}

func TestInspectionAnomalyCreatesWorkOrder(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterBatteryCabin(domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1"})

	ins, err := svc.CreateInspection(domain.EntityTypeBatteryCabin, "bc-1", "inspector-zhang")
	if err != nil {
		t.Fatalf("create inspection: %v", err)
	}
	if ins.Status != domain.InspectionStatusPending {
		t.Errorf("status = %s, want pending", ins.Status)
	}

	ins2, wo, err := svc.RecordInspectionResult(ins.ID, "cell temperature elevated", true)
	if err != nil {
		t.Fatalf("record result: %v", err)
	}
	if !ins2.Anomaly {
		t.Error("anomaly should be true")
	}
	if wo == nil {
		t.Fatal("work order should be created for anomaly")
	}
	if wo.Status != domain.WorkOrderStatusDispatched {
		t.Errorf("wo status = %s, want dispatched", wo.Status)
	}
	if ins2.WorkOrderID != wo.ID {
		t.Error("inspection should reference the work order")
	}
}

func TestInspectionNoAnomalyNoWorkOrder(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterPowerStation(domain.PowerStation{ID: "ps-1", Name: "WindFarm-1", Source: "wind"})

	ins, _ := svc.CreateInspection(domain.EntityTypePowerStation, "ps-1", "inspector-li")
	ins2, wo, err := svc.RecordInspectionResult(ins.ID, "all normal", false)
	if err != nil {
		t.Fatalf("record result: %v", err)
	}
	if ins2.Anomaly {
		t.Error("anomaly should be false")
	}
	if wo != nil {
		t.Error("no work order should be created for non-anomaly")
	}
}

func TestWorkOrderAcceptTimeoutEscalation(t *testing.T) {
	svc, clock := testService(t)
	svc.RegisterDieselGen(domain.DieselGenerator{ID: "dg-1", Name: "Gen-1"})

	wo, err := svc.ReportAnomaly(domain.EntityTypeDieselGen, "dg-1", "fuel leak")
	if err != nil {
		t.Fatalf("report anomaly: %v", err)
	}
	if wo.Status != domain.WorkOrderStatusDispatched {
		t.Fatalf("status = %s, want dispatched", wo.Status)
	}

	// Advance past the 15-minute acceptance timeout.
	clock.Advance(16 * time.Minute)

	escalated, err := svc.EscalateTimedOutWorkOrders()
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if len(escalated) != 1 {
		t.Fatalf("expected 1 escalated, got %d", len(escalated))
	}
	if escalated[0] != wo.ID {
		t.Errorf("escalated id = %s, want %s", escalated[0], wo.ID)
	}
	updated, _ := svc.GetWorkOrder(wo.ID)
	if updated.Status != domain.WorkOrderStatusEscalated {
		t.Errorf("status = %s, want escalated", updated.Status)
	}
	if updated.EscalatedTo != "team_leader" {
		t.Errorf("escalated_to = %s, want team_leader", updated.EscalatedTo)
	}

	// Escalated order can still be accepted.
	accepted, err := svc.AcceptWorkOrder(wo.ID, "team-leader-wang")
	if err != nil {
		t.Fatalf("accept after escalation: %v", err)
	}
	if accepted.Status != domain.WorkOrderStatusAccepted {
		t.Errorf("status = %s, want accepted", accepted.Status)
	}
}

func TestWorkOrderWorsenCreatesChildAndNotifiesBackup(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterBatteryCabin(domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1"})

	wo, _ := svc.ReportAnomaly(domain.EntityTypeBatteryCabin, "bc-1", "minor thermal issue")
	svc.AcceptWorkOrder(wo.ID, "repair-zhao")
	svc.StartWorkOrder(wo.ID)

	var backupNotified int32
	svc.SetBackupNotifier(func(childID string) {
		atomic.StoreInt32(&backupNotified, 1)
	})

	parent, child, err := svc.WorsenWorkOrder(wo.ID, "temperature rising rapidly")
	if err != nil {
		t.Fatalf("worsen: %v", err)
	}
	if parent.Status != domain.WorkOrderStatusWorsened {
		t.Errorf("parent status = %s, want worsened", parent.Status)
	}
	if child.ID == "" {
		t.Fatal("child work order should be created")
	}
	if child.ParentOrderID != parent.ID {
		t.Error("child should reference parent")
	}
	if child.Status != domain.WorkOrderStatusDispatched {
		t.Errorf("child status = %s, want dispatched", child.Status)
	}
	if atomic.LoadInt32(&backupNotified) != 1 {
		t.Error("backup team should have been notified")
	}
}

func TestWorkOrderCompleteFlow(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterDieselGen(domain.DieselGenerator{ID: "dg-1", Name: "Gen-1"})

	wo, _ := svc.ReportAnomaly(domain.EntityTypeDieselGen, "dg-1", "starter fault")
	svc.AcceptWorkOrder(wo.ID, "repair-li")
	svc.StartWorkOrder(wo.ID)
	completed, err := svc.CompleteWorkOrder(wo.ID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if completed.Status != domain.WorkOrderStatusCompleted {
		t.Errorf("status = %s, want completed", completed.Status)
	}
	if completed.CompletedAt.IsZero() {
		t.Error("completed_at should be set")
	}
}

func TestWorkOrderInvalidTransition(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterDieselGen(domain.DieselGenerator{ID: "dg-1", Name: "Gen-1"})

	wo, _ := svc.ReportAnomaly(domain.EntityTypeDieselGen, "dg-1", "fault")
	// Cannot complete a dispatched (not yet accepted) order.
	_, err := svc.CompleteWorkOrder(wo.ID)
	if !errors.Is(err, domain.ErrInvalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestConcurrentAcceptWorkOrder(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterBatteryCabin(domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1"})

	wo, _ := svc.ReportAnomaly(domain.EntityTypeBatteryCabin, "bc-1", "fault")

	var wg sync.WaitGroup
	var successCount int32
	var failCount int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := svc.AcceptWorkOrder(wo.ID, "repair-"+strconv.Itoa(n))
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}(i)
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("exactly 1 goroutine should succeed, got %d", successCount)
	}
	if failCount != 19 {
		t.Errorf("19 goroutines should fail, got %d", failCount)
	}
	final, _ := svc.GetWorkOrder(wo.ID)
	if final.Status != domain.WorkOrderStatusAccepted {
		t.Errorf("final status = %s, want accepted", final.Status)
	}
}

func TestBatteryTemperatureAlarmFreezesCharging(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterBatteryCabin(domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1", Charging: true, SOC: 80})

	bc, alarmed, err := svc.UpdateBatteryCabinTelemetry("bc-1", 48, 78)
	if err != nil {
		t.Fatalf("update telemetry: %v", err)
	}
	if !alarmed {
		t.Error("should be alarmed at 48°C")
	}
	if !bc.Alarmed {
		t.Error("battery cabin should be alarmed")
	}
	if !bc.ChargingFrozen {
		t.Error("charging should be frozen")
	}
	if bc.Charging {
		t.Error("charging should be stopped")
	}
}

func TestBlackStartFullFlow(t *testing.T) {
	svc, _ := testService(t)

	bs, err := svc.InitiateBlackStart("dispatcher-chen")
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if bs.Status != domain.BlackStartStatusInitiated {
		t.Errorf("status = %s, want initiated", bs.Status)
	}

	bs, _ = svc.StartBlackStart(bs.ID)
	if bs.Status != domain.BlackStartStatusInProgress {
		t.Errorf("status = %s, want in_progress", bs.Status)
	}

	bs, _ = svc.RestoreBus(bs.ID)
	if bs.Status != domain.BlackStartStatusBusRestored {
		t.Errorf("status = %s, want bus_restored", bs.Status)
	}

	gc, loads, err := svc.RequestGridConnection(bs.ID)
	if err != nil {
		t.Fatalf("request grid conn: %v", err)
	}
	if gc.OffGrid != true {
		t.Error("off-grid should be true during black start")
	}
	// Load shedding should have shed non-priority-1 loads.
	shedCount := 0
	for _, l := range loads {
		if l.Shed {
			shedCount++
		}
	}
	if shedCount == 0 {
		t.Error("non-priority-1 loads should be shed")
	}
	// Priority 1 loads should remain energized.
	for _, l := range svc.ListLoads() {
		if l.Priority == 1 && !l.Energized {
			t.Errorf("priority-1 load %s should remain energized", l.Name)
		}
	}

	// Dual sign-off.
	gc, _ = svc.SignGridConnection(gc.ID, domain.SignPartyDispatch, "dispatcher-chen")
	if gc.Status != domain.GridConnStatusDispatchSigned {
		t.Errorf("status = %s, want dispatch_signed", gc.Status)
	}
	gc, _ = svc.SignGridConnection(gc.ID, domain.SignPartyOM, "om-liu")
	if gc.Status != domain.GridConnStatusDualSigned {
		t.Errorf("status = %s, want dual_signed", gc.Status)
	}

	gc, _ = svc.SynchronizeGrid(gc.ID)
	if gc.Status != domain.GridConnStatusSynchronized {
		t.Errorf("status = %s, want synchronized", gc.Status)
	}

	bs, _ = svc.CompleteBlackStart(bs.ID)
	if bs.Status != domain.BlackStartStatusCompleted {
		t.Errorf("status = %s, want completed", bs.Status)
	}
}

func TestGridConnRequiresDualSignOff(t *testing.T) {
	svc, _ := testService(t)
	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)
	svc.RestoreBus(bs.ID)
	gc, _, _ := svc.RequestGridConnection(bs.ID)

	// Only dispatch signs.
	svc.SignGridConnection(gc.ID, domain.SignPartyDispatch, "dispatcher")
	_, err := svc.SynchronizeGrid(gc.ID)
	if !errors.Is(err, domain.ErrDualSignRequired) {
		t.Errorf("expected ErrDualSignRequired, got %v", err)
	}
}

func TestBlackStartDeadlineExceeded(t *testing.T) {
	svc, clock := testService(t)
	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)

	// Advance past the 20-minute deadline.
	clock.Advance(21 * time.Minute)

	failed, err := svc.FailExpiredBlackStarts()
	if err != nil {
		t.Fatalf("fail expired: %v", err)
	}
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed, got %d", len(failed))
	}
	result, _ := svc.GetBlackStart(bs.ID)
	if result.Status != domain.BlackStartStatusFailed {
		t.Errorf("status = %s, want failed", result.Status)
	}
}

func TestBlackStartRestoreBusAfterDeadlineFails(t *testing.T) {
	svc, clock := testService(t)
	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)

	clock.Advance(21 * time.Minute)

	_, err := svc.RestoreBus(bs.ID)
	if !errors.Is(err, domain.ErrDeadlineExceeded) {
		t.Errorf("expected ErrDeadlineExceeded, got %v", err)
	}
	result, _ := svc.GetBlackStart(bs.ID)
	if result.Status != domain.BlackStartStatusFailed {
		t.Errorf("status = %s, want failed", result.Status)
	}
}

func TestSMSResendOnNetworkInterrupt(t *testing.T) {
	svc, _ := testService(t)

	// Simulate network down: SMS delivery always fails.
	networkUp := false
	svc.SetSMSDeliverer(func(msg domain.SMSMessage) error {
		if !networkUp {
			return errors.New("network unreachable")
		}
		return nil
	})

	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)

	// SMS should be pending because network is down.
	pending := svc.ListPendingSMS()
	if len(pending) == 0 {
		t.Fatal("expected pending SMS messages while network is down")
	}
	for _, msg := range pending {
		if msg.Status != domain.SMSStatusPending {
			t.Errorf("msg %s status = %s, want pending", msg.ID, msg.Status)
		}
	}

	// Retry while network is still down — should remain pending.
	delivered, _ := svc.ResendPendingSMS()
	if len(delivered) != 0 {
		t.Error("no SMS should be delivered while network is down")
	}
	pending = svc.ListPendingSMS()
	if len(pending) == 0 {
		t.Fatal("SMS should still be pending")
	}
	firstAttempts := pending[0].Attempts
	if firstAttempts < 2 {
		t.Errorf("attempts = %d, should be >= 2 after retry", firstAttempts)
	}

	// Network restored.
	networkUp = true
	delivered, _ = svc.ResendPendingSMS()
	if len(delivered) == 0 {
		t.Fatal("SMS should be delivered after network restoration")
	}
	if len(svc.ListPendingSMS()) != 0 {
		t.Error("no pending SMS should remain after delivery")
	}
}

func TestSMSNetworkInterruptAndRestore(t *testing.T) {
	svc, _ := testService(t)

	networkUp := false
	svc.SetSMSDeliverer(func(msg domain.SMSMessage) error {
		if !networkUp {
			return errors.New("network unreachable")
		}
		return nil
	})

	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)
	svc.ReportNetworkInterrupt(bs.ID)

	// While network is down, pending SMS exist.
	if len(svc.ListPendingSMS()) == 0 {
		t.Fatal("expected pending SMS")
	}

	// Restore network and explicitly call RestoreNetwork.
	networkUp = true
	updated, delivered, err := svc.RestoreNetwork(bs.ID)
	if err != nil {
		t.Fatalf("restore network: %v", err)
	}
	if updated.NetworkInterrupted {
		t.Error("network_interrupted should be cleared")
	}
	if len(delivered) == 0 {
		t.Error("SMS should be delivered on network restore")
	}
	if len(svc.ListPendingSMS()) != 0 {
		t.Error("no pending SMS should remain")
	}
}

func TestIdempotentDispatchAndAccept(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterBatteryCabin(domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1"})

	// Create a draft work order directly via inspection.
	ins, _ := svc.CreateInspection(domain.EntityTypeBatteryCabin, "bc-1", "inspector")
	_, wo, _ := svc.RecordInspectionResult(ins.ID, "issue", true)

	// Dispatch is idempotent (already dispatched via RecordInspectionResult).
	wo2, err := svc.DispatchWorkOrder(wo.ID)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if wo2.Status != domain.WorkOrderStatusDispatched {
		t.Errorf("status = %s, want dispatched", wo2.Status)
	}

	// Accept is idempotent.
	svc.AcceptWorkOrder(wo.ID, "repair-1")
	wo3, err := svc.AcceptWorkOrder(wo.ID, "repair-1")
	if err != nil {
		t.Fatalf("second accept: %v", err)
	}
	if wo3.AssignedTo != "repair-1" {
		t.Errorf("assignee = %s, want repair-1 (first accept)", wo3.AssignedTo)
	}
}

func TestIdempotentSignOff(t *testing.T) {
	svc, _ := testService(t)
	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)
	svc.RestoreBus(bs.ID)
	gc, _, _ := svc.RequestGridConnection(bs.ID)

	svc.SignGridConnection(gc.ID, domain.SignPartyDispatch, "dispatcher-1")
	gc2, err := svc.SignGridConnection(gc.ID, domain.SignPartyDispatch, "dispatcher-2")
	if err != nil {
		t.Fatalf("second dispatch sign: %v", err)
	}
	if gc2.DispatchSignedBy != "dispatcher-1" {
		t.Errorf("signed_by = %s, want dispatcher-1", gc2.DispatchSignedBy)
	}
}

func TestReportAnomalyEvaluatesBatteryRules(t *testing.T) {
	svc, _ := testService(t)
	svc.RegisterBatteryCabin(domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1", CellTempC: 50, SOC: 80})

	wo, err := svc.ReportAnomaly(domain.EntityTypeBatteryCabin, "bc-1", "overheating reported")
	if err != nil {
		t.Fatalf("report anomaly: %v", err)
	}
	if wo.Status != domain.WorkOrderStatusDispatched {
		t.Errorf("wo status = %s, want dispatched", wo.Status)
	}
	bc, _ := svc.GetBatteryCabin("bc-1")
	if !bc.Alarmed || !bc.ChargingFrozen {
		t.Error("battery cabin should be alarmed and frozen after anomaly report with high temp")
	}
}

func TestInitiateBlackStartIdempotent(t *testing.T) {
	svc, _ := testService(t)
	bs1, _ := svc.InitiateBlackStart("dispatcher")
	bs2, _ := svc.InitiateBlackStart("dispatcher")
	if bs1.ID != bs2.ID {
		t.Errorf("second initiate should return existing active black start, got %s vs %s", bs1.ID, bs2.ID)
	}
}
