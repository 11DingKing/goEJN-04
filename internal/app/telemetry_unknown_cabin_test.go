package app

import (
	"errors"
	"testing"
	"time"

	"ejina-microgrid/internal/domain"
)

// TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin checks that a
// telemetry update addressed to a cabin id that does not exist reports
// ErrEntityNotFound and leaves the service able to serve later write
// operations.
func TestWritesStillWorkAfterTelemetryUpdateForUnknownCabin(t *testing.T) {
	svc, _ := testService(t)
	if err := svc.RegisterBatteryCabin(domain.BatteryCabin{
		ID: "bc-1", Name: "Cabin-1", CellTempC: 30, SOC: 80, Charging: true,
	}); err != nil {
		t.Fatalf("register battery cabin: %v", err)
	}

	if _, _, err := svc.UpdateBatteryCabinTelemetry("bc-missing", 41, 70); !errors.Is(err, domain.ErrEntityNotFound) {
		t.Fatalf("telemetry for an unknown cabin: err = %v, want ErrEntityNotFound", err)
	}

	done := make(chan error, 1)
	go func() {
		bc, alarmed, err := svc.UpdateBatteryCabinTelemetry("bc-1", 48, 66)
		if err != nil {
			done <- err
			return
		}
		if !alarmed || !bc.ChargingFrozen {
			done <- errors.New("48°C must raise the thermal alarm and freeze charging")
			return
		}
		if _, err := svc.InitiateBlackStart("dispatcher-chen"); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("write operations after the unknown-cabin request: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("write operations issued after the unknown-cabin telemetry request did not return within 2s")
	}
}
