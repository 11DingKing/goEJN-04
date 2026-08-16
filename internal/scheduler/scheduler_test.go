package scheduler

import (
	"context"
	"testing"
	"time"

	"ejina-microgrid/internal/app"
	"ejina-microgrid/internal/config"
	"ejina-microgrid/internal/domain"
	"ejina-microgrid/internal/store"
)

func testService(t *testing.T) (*app.Service, *app.FakeClock, *config.Config) {
	t.Helper()
	st := store.New()
	clock := app.NewFakeClock(time.Date(2026, 8, 16, 10, 0, 0, 0, time.UTC))
	cfg := &config.Config{
		ServerPort:             57579,
		BatteryTempThresholdC:  45.0,
		MinSOCPercent:          15.0,
		WorkOrderAcceptTimeout: 15 * time.Minute,
		BlackStartDeadline:     20 * time.Minute,
		SMSResendInterval:      30 * time.Second,
		SchedulerInterval:      50 * time.Millisecond,
	}
	svc := app.NewService(st, clock, cfg, app.NewSequentialIDGenerator())
	svc.SeedDefaultLoads()
	return svc, clock, cfg
}

func TestSchedulerEscalatesTimedOutWorkOrders(t *testing.T) {
	svc, clock, cfg := testService(t)
	svc.RegisterBatteryCabin(domain.BatteryCabin{ID: "bc-1", Name: "Cabin-1"})

	wo, _ := svc.ReportAnomaly(domain.EntityTypeBatteryCabin, "bc-1", "fault")

	// Advance past the acceptance timeout.
	clock.Advance(16 * time.Minute)

	sch := New(svc, cfg)
	sch.RunOnce()

	updated, _ := svc.GetWorkOrder(wo.ID)
	if updated.Status != domain.WorkOrderStatusEscalated {
		t.Errorf("status = %s, want escalated", updated.Status)
	}
}

func TestSchedulerFailsExpiredBlackStarts(t *testing.T) {
	svc, clock, cfg := testService(t)
	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)

	clock.Advance(21 * time.Minute)

	sch := New(svc, cfg)
	sch.RunOnce()

	result, _ := svc.GetBlackStart(bs.ID)
	if result.Status != domain.BlackStartStatusFailed {
		t.Errorf("status = %s, want failed", result.Status)
	}
}

func TestSchedulerRetriesPendingSMS(t *testing.T) {
	svc, clock, cfg := testService(t)

	// Network down: SMS stays pending.
	networkUp := false
	svc.SetSMSDeliverer(func(msg domain.SMSMessage) error {
		if !networkUp {
			return context.DeadlineExceeded
		}
		return nil
	})

	bs, _ := svc.InitiateBlackStart("dispatcher")
	svc.StartBlackStart(bs.ID)

	if len(svc.ListPendingSMS()) == 0 {
		t.Fatal("expected pending SMS")
	}

	// Scheduler retries but network is still down.
	sch := New(svc, cfg)
	sch.RunOnce()
	if len(svc.ListPendingSMS()) == 0 {
		t.Fatal("SMS should still be pending while network is down")
	}

	// Network comes back; scheduler retries successfully.
	networkUp = true
	sch.RunOnce()
	if len(svc.ListPendingSMS()) != 0 {
		t.Fatal("all SMS should be delivered after network restoration")
	}

	_ = clock
}

func TestSchedulerStartStop(t *testing.T) {
	svc, _, cfg := testService(t)
	sch := New(svc, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	stopFn := sch.Start(ctx)

	// Let it run briefly.
	time.Sleep(200 * time.Millisecond)

	cancel()
	stopFn()

	// Scheduler should stop.
	deadline := time.Now().Add(2 * time.Second)
	for !sch.IsStopped() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if !sch.IsStopped() {
		t.Error("scheduler should be stopped after context cancellation")
	}
}
