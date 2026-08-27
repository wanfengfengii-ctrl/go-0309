package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// Server serves the HTTP API on top of a Service implementation.
type Server struct {
	svc Service
}

// NewServer builds a Server wrapping the given service.
func NewServer(svc Service) *Server { return &Server{svc: svc} }

// Handler returns the routed HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("POST /v1/cycles", s.handleLockCycle)
	mux.HandleFunc("GET /v1/cycles/{id}", s.handleGetCycle)
	mux.HandleFunc("GET /v1/cycles/{id}/coverage", s.handleGetCoverage)
	mux.HandleFunc("GET /v1/cycles/{id}/audit", s.handleGetAudit)

	mux.HandleFunc("POST /v1/cycles/{id}/surface-confirmations", s.handleConfirmSurface)
	mux.HandleFunc("POST /v1/cycles/{id}/mix-pans", s.handleCreateMixPan)
	mux.HandleFunc("POST /v1/cycles/{id}/spray-bands", s.handleAppendSprayBand)
	mux.HandleFunc("POST /v1/cycles/{id}/rebound-seals", s.handleSealRebound)
	mux.HandleFunc("POST /v1/cycles/{id}/cure-evidence", s.handleAddCureEvidence)
	mux.HandleFunc("POST /v1/cycles/{id}/tests", s.handleAddTest)
	mux.HandleFunc("POST /v1/cycles/{id}/defects", s.handleCreateDefect)
	mux.HandleFunc("POST /v1/cycles/{id}/repairs", s.handleCreateRepair)
	mux.HandleFunc("POST /v1/cycles/{id}/reviews", s.handleSubmitReview)
	mux.HandleFunc("POST /v1/cycles/{id}/terminal-decisions", s.handleSubmitDecision)

	mux.HandleFunc("POST /v1/leases/acquire", s.handleAcquireLeases)
	mux.HandleFunc("POST /v1/leases/renew", s.handleRenewLease)
	mux.HandleFunc("POST /v1/leases/release", s.handleReleaseLease)

	mux.HandleFunc("POST /v1/device-calls", s.handleCreateDeviceCall)
	mux.HandleFunc("POST /v1/device-calls/{id}/receipts", s.handleSubmitReceipt)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Health(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleLockCycle(w http.ResponseWriter, r *http.Request) {
	var req LockCycleRequest
	if !decode(w, r, &req) {
		return
	}
	if req.OperationID == "" {
		writeError(w, domain.NewError(domain.CodeInvalidRequest, "operation_id required"))
		return
	}
	view, err := s.svc.LockCycle(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) handleGetCycle(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.GetCycle(r.Context(), domain.CycleID(r.PathValue("id")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleGetCoverage(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.GetCoverage(r.Context(), domain.CycleID(r.PathValue("id")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleGetAudit(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.GetAudit(r.Context(), domain.CycleID(r.PathValue("id")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleConfirmSurface(w http.ResponseWriter, r *http.Request) {
	var req SurfaceConfirmRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.ConfirmSurface(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleCreateMixPan(w http.ResponseWriter, r *http.Request) {
	var req MixPanRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.CreateMixPan(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleAppendSprayBand(w http.ResponseWriter, r *http.Request) {
	var req SprayBandRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.AppendSprayBand(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleSealRebound(w http.ResponseWriter, r *http.Request) {
	var req ReboundSealRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.SealRebound(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleAddCureEvidence(w http.ResponseWriter, r *http.Request) {
	var req CureEvidenceRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.AddCureEvidence(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleAddTest(w http.ResponseWriter, r *http.Request) {
	var req TestRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.AddTest(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleCreateDefect(w http.ResponseWriter, r *http.Request) {
	var req DefectRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.CreateDefect(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleCreateRepair(w http.ResponseWriter, r *http.Request) {
	var req RepairRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.CreateRepair(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleSubmitReview(w http.ResponseWriter, r *http.Request) {
	var req ReviewRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.SubmitReview(r.Context(), domain.CycleID(r.PathValue("id")), req)
	respondCreated(w, view, err)
}

func (s *Server) handleSubmitDecision(w http.ResponseWriter, r *http.Request) {
	var req DecisionRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.SubmitTerminalDecision(r.Context(), domain.CycleID(r.PathValue("id")), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) handleAcquireLeases(w http.ResponseWriter, r *http.Request) {
	var req LeaseAcquireRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.AcquireLeases(r.Context(), req)
	respondCreated(w, view, err)
}

func (s *Server) handleRenewLease(w http.ResponseWriter, r *http.Request) {
	var req LeaseRenewRequest
	if !decode(w, r, &req) {
		return
	}
	if err := s.svc.RenewLease(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReleaseLease(w http.ResponseWriter, r *http.Request) {
	var req LeaseReleaseRequest
	if !decode(w, r, &req) {
		return
	}
	if err := s.svc.ReleaseLease(r.Context(), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCreateDeviceCall(w http.ResponseWriter, r *http.Request) {
	var req DeviceCallRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.CreateDeviceCall(r.Context(), req)
	respondCreated(w, view, err)
}

func (s *Server) handleSubmitReceipt(w http.ResponseWriter, r *http.Request) {
	var req ReceiptRequest
	if !decode(w, r, &req) {
		return
	}
	view, err := s.svc.SubmitReceipt(r.Context(), domain.DeviceCallID(r.PathValue("id")), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// decode reads a JSON request body, writing a 400 response on failure.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, domain.NewError(domain.CodeInvalidRequest, "invalid JSON body"))
		return false
	}
	return true
}

// respondCreated writes a 201 response or a unified error.
func respondCreated(w http.ResponseWriter, v any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
