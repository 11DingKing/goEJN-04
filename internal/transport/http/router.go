package http

import (
	"encoding/json"
	"errors"
	"io"
	apphttp "net/http"

	"ejina-microgrid/internal/app"
	"ejina-microgrid/internal/domain"
)

// Handler exposes the microgrid dispatch service over HTTP using the standard
// library ServeMux with Go 1.22+ pattern routing.
type Handler struct {
	svc *app.Service
}

// NewHandler creates a Handler for the given service.
func NewHandler(svc *app.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes returns the configured HTTP handler.
func (h *Handler) Routes() apphttp.Handler {
	mux := apphttp.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", h.health)

	// Battery cabins
	mux.HandleFunc("GET /api/v1/battery-cabins", h.listBatteryCabins)
	mux.HandleFunc("POST /api/v1/battery-cabins", h.registerBatteryCabin)
	mux.HandleFunc("GET /api/v1/battery-cabins/{id}", h.getBatteryCabin)
	mux.HandleFunc("PUT /api/v1/battery-cabins/{id}/telemetry", h.updateBatteryTelemetry)

	// Loads
	mux.HandleFunc("GET /api/v1/loads", h.listLoads)

	// Inspections & anomalies
	mux.HandleFunc("GET /api/v1/inspections", h.listInspections)
	mux.HandleFunc("POST /api/v1/inspections", h.createInspection)
	mux.HandleFunc("POST /api/v1/inspections/{id}/result", h.recordInspectionResult)
	mux.HandleFunc("POST /api/v1/anomalies", h.reportAnomaly)

	// Work orders
	mux.HandleFunc("GET /api/v1/workorders", h.listWorkOrders)
	mux.HandleFunc("GET /api/v1/workorders/{id}", h.getWorkOrder)
	mux.HandleFunc("POST /api/v1/workorders/{id}/dispatch", h.dispatchWorkOrder)
	mux.HandleFunc("POST /api/v1/workorders/{id}/accept", h.acceptWorkOrder)
	mux.HandleFunc("POST /api/v1/workorders/{id}/start", h.startWorkOrder)
	mux.HandleFunc("POST /api/v1/workorders/{id}/worsen", h.worsenWorkOrder)
	mux.HandleFunc("POST /api/v1/workorders/{id}/complete", h.completeWorkOrder)
	mux.HandleFunc("POST /api/v1/workorders/{id}/cancel", h.cancelWorkOrder)

	// Black starts
	mux.HandleFunc("GET /api/v1/blackstarts", h.listBlackStarts)
	mux.HandleFunc("POST /api/v1/blackstarts", h.initiateBlackStart)
	mux.HandleFunc("POST /api/v1/blackstarts/{id}/start", h.startBlackStart)
	mux.HandleFunc("POST /api/v1/blackstarts/{id}/restore-bus", h.restoreBus)
	mux.HandleFunc("POST /api/v1/blackstarts/{id}/network-interrupt", h.reportNetworkInterrupt)
	mux.HandleFunc("POST /api/v1/blackstarts/{id}/restore-network", h.restoreNetwork)
	mux.HandleFunc("POST /api/v1/blackstarts/{id}/complete", h.completeBlackStart)

	// Grid connections
	mux.HandleFunc("GET /api/v1/grid-connections", h.listGridConns)
	mux.HandleFunc("POST /api/v1/grid-connections", h.requestGridConnection)
	mux.HandleFunc("POST /api/v1/grid-connections/{id}/sign", h.signGridConnection)
	mux.HandleFunc("POST /api/v1/grid-connections/{id}/synchronize", h.synchronizeGrid)

	// SMS
	mux.HandleFunc("GET /api/v1/sms", h.listSMS)

	return mux
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeJSON(w apphttp.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w apphttp.ResponseWriter, err error) {
	status := apphttp.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrEntityNotFound),
		errors.Is(err, domain.ErrWorkOrderNotFound),
		errors.Is(err, domain.ErrBlackStartNotFound),
		errors.Is(err, domain.ErrGridConnNotFound),
		errors.Is(err, domain.ErrInspectionNotFound),
		errors.Is(err, domain.ErrSMSNotFound),
		errors.Is(err, domain.ErrLoadNotFound):
		status = apphttp.StatusNotFound
	case errors.Is(err, domain.ErrInvalidTransition),
		errors.Is(err, domain.ErrAlreadySigned),
		errors.Is(err, domain.ErrDualSignRequired),
		errors.Is(err, domain.ErrOffGridProhibited),
		errors.Is(err, domain.ErrAlreadyAccepted),
		errors.Is(err, domain.ErrNotDispatched):
		status = apphttp.StatusConflict
	case errors.Is(err, domain.ErrTempExceedsThreshold),
		errors.Is(err, domain.ErrSOCBelowMinimum),
		errors.Is(err, domain.ErrDeadlineExceeded):
		status = apphttp.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrDuplicateID):
		status = apphttp.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decodeJSON(r *apphttp.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, v)
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func (h *Handler) health(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, map[string]string{"status": "ok"})
}

// ---------------------------------------------------------------------------
// Battery cabins
// ---------------------------------------------------------------------------

func (h *Handler) listBatteryCabins(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, h.svc.ListBatteryCabins())
}

func (h *Handler) registerBatteryCabin(w apphttp.ResponseWriter, r *apphttp.Request) {
	var bc domain.BatteryCabin
	if err := decodeJSON(r, &bc); err != nil {
		writeJSON(w, apphttp.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.svc.RegisterBatteryCabin(bc); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusCreated, bc)
}

func (h *Handler) getBatteryCabin(w apphttp.ResponseWriter, r *apphttp.Request) {
	bc, err := h.svc.GetBatteryCabin(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, bc)
}

type telemetryReq struct {
	CellTempC float64 `json:"cell_temp_c"`
	SOC       float64 `json:"soc_percent"`
}

func (h *Handler) updateBatteryTelemetry(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req telemetryReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, apphttp.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	bc, alarmed, err := h.svc.UpdateBatteryCabinTelemetry(r.PathValue("id"), req.CellTempC, req.SOC)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, map[string]any{"battery_cabin": bc, "alarmed": alarmed})
}

// ---------------------------------------------------------------------------
// Loads
// ---------------------------------------------------------------------------

func (h *Handler) listLoads(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, h.svc.ListLoads())
}

// ---------------------------------------------------------------------------
// Inspections & anomalies
// ---------------------------------------------------------------------------

func (h *Handler) listInspections(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, h.svc.ListInspections())
}

type createInspectionReq struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Inspector  string `json:"inspector"`
}

func (h *Handler) createInspection(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req createInspectionReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, apphttp.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ins, err := h.svc.CreateInspection(domain.EntityType(req.EntityType), req.EntityID, req.Inspector)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusCreated, ins)
}

type inspectionResultReq struct {
	Result  string `json:"result"`
	Anomaly bool   `json:"anomaly"`
}

func (h *Handler) recordInspectionResult(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req inspectionResultReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, apphttp.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ins, wo, err := h.svc.RecordInspectionResult(r.PathValue("id"), req.Result, req.Anomaly)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := map[string]any{"inspection": ins}
	if wo != nil {
		resp["work_order"] = *wo
	}
	writeJSON(w, apphttp.StatusOK, resp)
}

type reportAnomalyReq struct {
	EntityType  string `json:"entity_type"`
	EntityID    string `json:"entity_id"`
	Description string `json:"description"`
}

func (h *Handler) reportAnomaly(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req reportAnomalyReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, apphttp.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	wo, err := h.svc.ReportAnomaly(domain.EntityType(req.EntityType), req.EntityID, req.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusCreated, wo)
}

// ---------------------------------------------------------------------------
// Work orders
// ---------------------------------------------------------------------------

func (h *Handler) listWorkOrders(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, h.svc.ListWorkOrders())
}

func (h *Handler) getWorkOrder(w apphttp.ResponseWriter, r *apphttp.Request) {
	wo, err := h.svc.GetWorkOrder(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, wo)
}

func (h *Handler) dispatchWorkOrder(w apphttp.ResponseWriter, r *apphttp.Request) {
	wo, err := h.svc.DispatchWorkOrder(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, wo)
}

type acceptReq struct {
	Assignee string `json:"assignee"`
}

func (h *Handler) acceptWorkOrder(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req acceptReq
	_ = decodeJSON(r, &req)
	wo, err := h.svc.AcceptWorkOrder(r.PathValue("id"), req.Assignee)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, wo)
}

func (h *Handler) startWorkOrder(w apphttp.ResponseWriter, r *apphttp.Request) {
	wo, err := h.svc.StartWorkOrder(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, wo)
}

type worsenReq struct {
	Description string `json:"description"`
}

func (h *Handler) worsenWorkOrder(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req worsenReq
	_ = decodeJSON(r, &req)
	parent, child, err := h.svc.WorsenWorkOrder(r.PathValue("id"), req.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, map[string]any{"parent": parent, "child": child})
}

func (h *Handler) completeWorkOrder(w apphttp.ResponseWriter, r *apphttp.Request) {
	wo, err := h.svc.CompleteWorkOrder(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, wo)
}

func (h *Handler) cancelWorkOrder(w apphttp.ResponseWriter, r *apphttp.Request) {
	wo, err := h.svc.CancelWorkOrder(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, wo)
}

// ---------------------------------------------------------------------------
// Black starts
// ---------------------------------------------------------------------------

func (h *Handler) listBlackStarts(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, h.svc.ListBlackStarts())
}

type initiateBlackStartReq struct {
	Initiator string `json:"initiator"`
}

func (h *Handler) initiateBlackStart(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req initiateBlackStartReq
	_ = decodeJSON(r, &req)
	bs, err := h.svc.InitiateBlackStart(req.Initiator)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusCreated, bs)
}

func (h *Handler) startBlackStart(w apphttp.ResponseWriter, r *apphttp.Request) {
	bs, err := h.svc.StartBlackStart(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, bs)
}

func (h *Handler) restoreBus(w apphttp.ResponseWriter, r *apphttp.Request) {
	bs, err := h.svc.RestoreBus(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, bs)
}

func (h *Handler) reportNetworkInterrupt(w apphttp.ResponseWriter, r *apphttp.Request) {
	bs, err := h.svc.ReportNetworkInterrupt(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, bs)
}

func (h *Handler) restoreNetwork(w apphttp.ResponseWriter, r *apphttp.Request) {
	bs, delivered, err := h.svc.RestoreNetwork(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, map[string]any{"black_start": bs, "delivered_sms": delivered})
}

func (h *Handler) completeBlackStart(w apphttp.ResponseWriter, r *apphttp.Request) {
	bs, err := h.svc.CompleteBlackStart(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, bs)
}

// ---------------------------------------------------------------------------
// Grid connections
// ---------------------------------------------------------------------------

func (h *Handler) listGridConns(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, h.svc.ListGridConns())
}

type requestGridConnReq struct {
	BlackStartID string `json:"black_start_id"`
}

func (h *Handler) requestGridConnection(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req requestGridConnReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, apphttp.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	gc, loads, err := h.svc.RequestGridConnection(req.BlackStartID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusCreated, map[string]any{"grid_connection": gc, "shed_loads": loads})
}

type signReq struct {
	Party  string `json:"party"`
	Signer string `json:"signer"`
}

func (h *Handler) signGridConnection(w apphttp.ResponseWriter, r *apphttp.Request) {
	var req signReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, apphttp.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	gc, err := h.svc.SignGridConnection(r.PathValue("id"), domain.SignParty(req.Party), req.Signer)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, gc)
}

func (h *Handler) synchronizeGrid(w apphttp.ResponseWriter, r *apphttp.Request) {
	gc, err := h.svc.SynchronizeGrid(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, apphttp.StatusOK, gc)
}

// ---------------------------------------------------------------------------
// SMS
// ---------------------------------------------------------------------------

func (h *Handler) listSMS(w apphttp.ResponseWriter, r *apphttp.Request) {
	writeJSON(w, apphttp.StatusOK, h.svc.ListAllSMS())
}
