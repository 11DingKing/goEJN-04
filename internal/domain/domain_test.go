package domain

import (
	"testing"
	"time"
)

func TestWorkOrderStateTransitions(t *testing.T) {
	tests := []struct {
		name string
		from WorkOrderStatus
		to   WorkOrderStatus
		want bool
	}{
		{"draft to dispatched", WorkOrderStatusDraft, WorkOrderStatusDispatched, true},
		{"dispatched to accepted", WorkOrderStatusDispatched, WorkOrderStatusAccepted, true},
		{"dispatched to escalated", WorkOrderStatusDispatched, WorkOrderStatusEscalated, true},
		{"escalated to accepted", WorkOrderStatusEscalated, WorkOrderStatusAccepted, true},
		{"accepted to in_progress", WorkOrderStatusAccepted, WorkOrderStatusInProgress, true},
		{"in_progress to worsened", WorkOrderStatusInProgress, WorkOrderStatusWorsened, true},
		{"in_progress to completed", WorkOrderStatusInProgress, WorkOrderStatusCompleted, true},
		{"worsened to in_progress", WorkOrderStatusWorsened, WorkOrderStatusInProgress, true},
		{"draft to completed (invalid)", WorkOrderStatusDraft, WorkOrderStatusCompleted, false},
		{"completed to dispatched (invalid)", WorkOrderStatusCompleted, WorkOrderStatusDispatched, false},
		{"completed to anything (invalid)", WorkOrderStatusCompleted, WorkOrderStatusDispatched, false},
		{"cancelled to accepted (invalid)", WorkOrderStatusCancelled, WorkOrderStatusAccepted, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%s→%s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestWorkOrderTerminal(t *testing.T) {
	if !WorkOrderStatusCompleted.Terminal() {
		t.Error("completed should be terminal")
	}
	if !WorkOrderStatusCancelled.Terminal() {
		t.Error("cancelled should be terminal")
	}
	if WorkOrderStatusInProgress.Terminal() {
		t.Error("in_progress should not be terminal")
	}
}

func TestBlackStartStateTransitions(t *testing.T) {
	tests := []struct {
		from BlackStartStatus
		to   BlackStartStatus
		want bool
	}{
		{BlackStartStatusInitiated, BlackStartStatusInProgress, true},
		{BlackStartStatusInProgress, BlackStartStatusBusRestored, true},
		{BlackStartStatusBusRestored, BlackStartStatusGridSyncing, true},
		{BlackStartStatusGridSyncing, BlackStartStatusCompleted, true},
		{BlackStartStatusInitiated, BlackStartStatusCompleted, false},
		{BlackStartStatusCompleted, BlackStartStatusInProgress, false},
		{BlackStartStatusFailed, BlackStartStatusInProgress, false},
	}
	for _, tt := range tests {
		got := tt.from.CanTransitionTo(tt.to)
		if got != tt.want {
			t.Errorf("CanTransitionTo(%s→%s) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestGridConnDualSignOff(t *testing.T) {
	gc := GridConnection{Status: GridConnStatusRequested}
	if gc.BothSigned() {
		t.Error("nothing signed should not be both-signed")
	}
	gc.DispatchSigned = true
	if gc.BothSigned() {
		t.Error("only dispatch signed should not be both-signed")
	}
	gc.OMSigned = true
	if !gc.BothSigned() {
		t.Error("both signed should be both-signed")
	}
}

func TestGridConnStateTransitions(t *testing.T) {
	if !GridConnStatusRequested.CanTransitionTo(GridConnStatusDispatchSigned) {
		t.Error("requested → dispatch_signed should be allowed")
	}
	if !GridConnStatusDualSigned.CanTransitionTo(GridConnStatusSynchronized) {
		t.Error("dual_signed → synchronized should be allowed")
	}
	if GridConnStatusCompleted.CanTransitionTo(GridConnStatusSynchronized) {
		t.Error("completed → synchronized should not be allowed")
	}
}

func TestBatteryCabinEvaluateTemperature(t *testing.T) {
	now := time.Now()

	// Normal temperature: no alarm.
	bc := BatteryCabin{ID: "bc-1", CellTempC: 40, Charging: true}
	if bc.EvaluateTemperature(45, now) {
		t.Error("40°C should not trigger alarm")
	}
	if bc.Alarmed || bc.ChargingFrozen {
		t.Error("40°C should not freeze charging")
	}

	// Over-threshold: alarm + freeze.
	bc.CellTempC = 46
	if !bc.EvaluateTemperature(45, now) {
		t.Error("46°C should trigger alarm")
	}
	if !bc.Alarmed {
		t.Error("alarm should be set")
	}
	if !bc.ChargingFrozen {
		t.Error("charging should be frozen")
	}
	if bc.Charging {
		t.Error("charging should be stopped when frozen")
	}
}

func TestBatteryCabinCanOperateOffGrid(t *testing.T) {
	bc := BatteryCabin{ID: "bc-1", SOC: 15.0}
	if !bc.CanOperateOffGrid(15.0) {
		t.Error("SOC exactly at minimum should allow off-grid")
	}
	bc.SOC = 14.9
	if bc.CanOperateOffGrid(15.0) {
		t.Error("SOC below minimum should not allow off-grid")
	}
	bc.SOC = 80.0
	if !bc.CanOperateOffGrid(15.0) {
		t.Error("SOC well above minimum should allow off-grid")
	}
}

func TestShedLoadsByPriority(t *testing.T) {
	now := time.Now()
	loads := []Load{
		{ID: "hospital", Name: "Hospital", Priority: 1, Energized: true},
		{ID: "water", Name: "Water", Priority: 1, Energized: true},
		{ID: "residential", Name: "Residential", Priority: 2, Energized: true},
		{ID: "commercial", Name: "Commercial", Priority: 2, Energized: true},
		{ID: "industrial", Name: "Industrial", Priority: 3, Energized: true},
		{ID: "agriculture", Name: "Agriculture", Priority: 3, Energized: true},
	}

	result, shed := ShedLoadsByPriority(loads, 1, now)
	if shed != 4 {
		t.Errorf("expected 4 loads shed, got %d", shed)
	}
	for _, l := range result {
		if l.Priority == 1 {
			if !l.Energized || l.Shed {
				t.Errorf("priority-1 load %s should remain energized", l.Name)
			}
		} else {
			if l.Energized || !l.Shed {
				t.Errorf("non-priority-1 load %s should be shed", l.Name)
			}
		}
	}
}

func TestShedLoadsByPriorityKeepAll(t *testing.T) {
	now := time.Now()
	loads := []Load{
		{ID: "a", Name: "A", Priority: 1, Energized: true},
		{ID: "b", Name: "B", Priority: 2, Energized: true},
	}
	result, shed := ShedLoadsByPriority(loads, 3, now)
	if shed != 0 {
		t.Errorf("keepPriority=3 should shed 0, got %d", shed)
	}
	for _, l := range result {
		if !l.Energized {
			t.Errorf("load %s should remain energized", l.Name)
		}
	}
}

func TestBlackStartIsOverdue(t *testing.T) {
	now := time.Now()
	bs := BlackStart{
		Status:   BlackStartStatusInProgress,
		Deadline: now.Add(-1 * time.Minute),
	}
	if !bs.IsOverdue(now) {
		t.Error("past deadline in_progress should be overdue")
	}
	bs.Status = BlackStartStatusBusRestored
	if bs.IsOverdue(now) {
		t.Error("bus_restored should not be overdue")
	}
	bs.Status = BlackStartStatusInProgress
	bs.Deadline = now.Add(10 * time.Minute)
	if bs.IsOverdue(now) {
		t.Error("future deadline should not be overdue")
	}
}
