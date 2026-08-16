package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ejina-microgrid/internal/app"
	"ejina-microgrid/internal/config"
	"ejina-microgrid/internal/store"
)

func testHandler(t *testing.T) (*Handler, *app.FakeClock) {
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
		SchedulerInterval:      100 * time.Millisecond,
	}
	svc := app.NewService(st, clock, cfg, app.NewSequentialIDGenerator())
	svc.SeedDefaultLoads()
	return NewHandler(svc), clock
}

func doRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestHealthEndpoint(t *testing.T) {
	h, _ := testHandler(t)
	w := doRequest(t, h.Routes(), "GET", "/api/v1/health", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status field = %s, want ok", resp["status"])
	}
}

func TestBatteryCabinTelemetryAlarm(t *testing.T) {
	h, _ := testHandler(t)
	routes := h.Routes()

	// Register battery cabin.
	w := doRequest(t, routes, "POST", "/api/v1/battery-cabins", map[string]any{
		"id": "bc-1", "name": "Cabin-1", "cell_temp_c": 30, "soc_percent": 80,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("register: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Update telemetry above threshold.
	w = doRequest(t, routes, "PUT", "/api/v1/battery-cabins/bc-1/telemetry", map[string]any{
		"cell_temp_c": 48, "soc_percent": 78,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("telemetry: status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["alarmed"] != true {
		t.Error("alarmed should be true at 48°C")
	}
}

func TestInspectionAnomalyFlow(t *testing.T) {
	h, _ := testHandler(t)
	routes := h.Routes()

	doRequest(t, routes, "POST", "/api/v1/battery-cabins", map[string]any{
		"id": "bc-1", "name": "Cabin-1",
	})

	w := doRequest(t, routes, "POST", "/api/v1/inspections", map[string]any{
		"entity_type": "battery_cabin", "entity_id": "bc-1", "inspector": "zhang",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create inspection: %d %s", w.Code, w.Body.String())
	}
	var insResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &insResp)
	insID := insResp["id"].(string)

	w = doRequest(t, routes, "POST", "/api/v1/inspections/"+insID+"/result", map[string]any{
		"result": "thermal anomaly", "anomaly": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("record result: %d %s", w.Code, w.Body.String())
	}
	var resultResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resultResp)
	if resultResp["work_order"] == nil {
		t.Error("work_order should be present for anomaly")
	}
}

func TestWorkOrderLifecycle(t *testing.T) {
	h, _ := testHandler(t)
	routes := h.Routes()

	// Report anomaly to create + dispatch a work order.
	w := doRequest(t, routes, "POST", "/api/v1/anomalies", map[string]any{
		"entity_type": "diesel_generator", "entity_id": "dg-1", "description": "starter fault",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("report anomaly: %d %s", w.Code, w.Body.String())
	}
	var wo map[string]any
	json.Unmarshal(w.Body.Bytes(), &wo)
	woID := wo["id"].(string)

	// Accept.
	w = doRequest(t, routes, "POST", "/api/v1/workorders/"+woID+"/accept", map[string]any{
		"assignee": "repair-li",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", w.Code, w.Body.String())
	}

	// Start.
	w = doRequest(t, routes, "POST", "/api/v1/workorders/"+woID+"/start", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}

	// Complete.
	w = doRequest(t, routes, "POST", "/api/v1/workorders/"+woID+"/complete", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", w.Code, w.Body.String())
	}
	var final map[string]any
	json.Unmarshal(w.Body.Bytes(), &final)
	if final["status"] != "completed" {
		t.Errorf("status = %v, want completed", final["status"])
	}
}

func TestBlackStartHTTPFlow(t *testing.T) {
	h, _ := testHandler(t)
	routes := h.Routes()

	// Initiate.
	w := doRequest(t, routes, "POST", "/api/v1/blackstarts", map[string]any{
		"initiator": "dispatcher-chen",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("initiate: %d %s", w.Code, w.Body.String())
	}
	var bs map[string]any
	json.Unmarshal(w.Body.Bytes(), &bs)
	bsID := bs["id"].(string)

	// Start.
	w = doRequest(t, routes, "POST", "/api/v1/blackstarts/"+bsID+"/start", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}

	// Restore bus.
	w = doRequest(t, routes, "POST", "/api/v1/blackstarts/"+bsID+"/restore-bus", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("restore bus: %d %s", w.Code, w.Body.String())
	}

	// Request grid connection.
	w = doRequest(t, routes, "POST", "/api/v1/grid-connections", map[string]any{
		"black_start_id": bsID,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("request grid conn: %d %s", w.Code, w.Body.String())
	}
	var gcResp map[string]any
	json.Unmarshal(w.Body.Bytes(), &gcResp)
	gc := gcResp["grid_connection"].(map[string]any)
	gcID := gc["id"].(string)

	// Dual sign-off.
	doRequest(t, routes, "POST", "/api/v1/grid-connections/"+gcID+"/sign", map[string]any{
		"party": "dispatch", "signer": "dispatcher-chen",
	})
	doRequest(t, routes, "POST", "/api/v1/grid-connections/"+gcID+"/sign", map[string]any{
		"party": "operations", "signer": "om-liu",
	})

	// Synchronize.
	w = doRequest(t, routes, "POST", "/api/v1/grid-connections/"+gcID+"/synchronize", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("synchronize: %d %s", w.Code, w.Body.String())
	}

	// Complete black start.
	w = doRequest(t, routes, "POST", "/api/v1/blackstarts/"+bsID+"/complete", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("complete: %d %s", w.Code, w.Body.String())
	}
	var final map[string]any
	json.Unmarshal(w.Body.Bytes(), &final)
	if final["status"] != "completed" {
		t.Errorf("status = %v, want completed", final["status"])
	}
}

func TestNotFoundError(t *testing.T) {
	h, _ := testHandler(t)
	routes := h.Routes()

	w := doRequest(t, routes, "GET", "/api/v1/battery-cabins/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestConflictError(t *testing.T) {
	h, _ := testHandler(t)
	routes := h.Routes()

	// Report anomaly to create a work order.
	w := doRequest(t, routes, "POST", "/api/v1/anomalies", map[string]any{
		"entity_type": "diesel_generator", "entity_id": "dg-1", "description": "fault",
	})
	var wo map[string]any
	json.Unmarshal(w.Body.Bytes(), &wo)
	woID := wo["id"].(string)

	// Try to complete a dispatched (not accepted) work order → conflict.
	w = doRequest(t, routes, "POST", "/api/v1/workorders/"+woID+"/complete", nil)
	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", w.Code)
	}
}

func TestListLoads(t *testing.T) {
	h, _ := testHandler(t)
	routes := h.Routes()

	w := doRequest(t, routes, "GET", "/api/v1/loads", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var loads []map[string]any
	json.Unmarshal(w.Body.Bytes(), &loads)
	if len(loads) < 2 {
		t.Errorf("expected at least 2 loads, got %d", len(loads))
	}
	// Verify priority-1 loads exist.
	hasPriority1 := false
	for _, l := range loads {
		if int(l["priority"].(float64)) == 1 {
			hasPriority1 = true
			break
		}
	}
	if !hasPriority1 {
		t.Error("expected at least one priority-1 load")
	}
}
