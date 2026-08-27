package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

type stubService struct {
	cycles map[domain.CycleID]CycleView
}

func (s *stubService) Health(_ context.Context) error { return nil }

func (s *stubService) LockCycle(_ context.Context, req LockCycleRequest) (CycleView, error) {
	if req.Digest == "" {
		return CycleView{}, domain.NewError(domain.CodeStaleSnapshot, "digest required")
	}
	v := CycleView{ID: "c1", Tunnel: req.Snapshot.Tunnel, Digest: req.Digest}
	s.cycles["c1"] = v
	return v, nil
}

func (s *stubService) GetCycle(_ context.Context, id domain.CycleID) (CycleView, error) {
	if v, ok := s.cycles[id]; ok {
		return v, nil
	}
	return CycleView{}, domain.NewError(domain.CodeNotFound, "cycle not found")
}

func (s *stubService) GetCoverage(_ context.Context, id domain.CycleID) (CoverageView, error) {
	return CoverageView{CycleID: id}, nil
}

func (s *stubService) GetAudit(_ context.Context, id domain.CycleID) (AuditView, error) {
	return AuditView{CycleID: id}, nil
}

func (s *stubService) ConfirmSurface(_ context.Context, _ domain.CycleID, _ SurfaceConfirmRequest) (EvidenceView, error) {
	return EvidenceView{}, nil
}
func (s *stubService) CreateMixPan(_ context.Context, _ domain.CycleID, _ MixPanRequest) (PanView, error) {
	return PanView{}, nil
}
func (s *stubService) AppendSprayBand(_ context.Context, _ domain.CycleID, _ SprayBandRequest) (BandView, error) {
	return BandView{}, nil
}
func (s *stubService) SealRebound(_ context.Context, _ domain.CycleID, _ ReboundSealRequest) (EvidenceView, error) {
	return EvidenceView{}, nil
}
func (s *stubService) AddCureEvidence(_ context.Context, _ domain.CycleID, _ CureEvidenceRequest) (EvidenceView, error) {
	return EvidenceView{}, nil
}
func (s *stubService) AddTest(_ context.Context, _ domain.CycleID, _ TestRequest) (EvidenceView, error) {
	return EvidenceView{}, nil
}
func (s *stubService) CreateDefect(_ context.Context, _ domain.CycleID, _ DefectRequest) (DefectView, error) {
	return DefectView{}, nil
}
func (s *stubService) CreateRepair(_ context.Context, _ domain.CycleID, _ RepairRequest) (RepairView, error) {
	return RepairView{}, nil
}
func (s *stubService) SubmitReview(_ context.Context, _ domain.CycleID, _ ReviewRequest) (ReviewView, error) {
	return ReviewView{}, nil
}
func (s *stubService) SubmitTerminalDecision(_ context.Context, _ domain.CycleID, _ DecisionRequest) (DecisionView, error) {
	return DecisionView{}, nil
}
func (s *stubService) AcquireLeases(_ context.Context, _ LeaseAcquireRequest) (LeaseAcquireResult, error) {
	return LeaseAcquireResult{}, nil
}
func (s *stubService) RenewLease(_ context.Context, _ LeaseRenewRequest) error { return nil }
func (s *stubService) ReleaseLease(_ context.Context, _ LeaseReleaseRequest) error {
	return nil
}
func (s *stubService) CreateDeviceCall(_ context.Context, _ DeviceCallRequest) (DeviceCallView, error) {
	return DeviceCallView{}, nil
}
func (s *stubService) SubmitReceipt(_ context.Context, _ domain.DeviceCallID, _ ReceiptRequest) (DeviceCallView, error) {
	return DeviceCallView{}, nil
}

func TestHealthOK(t *testing.T) {
	srv := NewServer(&stubService{cycles: map[domain.CycleID]CycleView{}})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
}

func TestLockCycleCreated(t *testing.T) {
	srv := NewServer(&stubService{cycles: map[domain.CycleID]CycleView{}})
	body := `{"operation_id":"op-1","digest":"d1","snapshot":{"tunnel":"T1","start_meter":0,"end_meter":1000,"cycle_no":1}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/cycles", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got status %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var v CycleView
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v.ID != "c1" {
		t.Fatalf("got id %q, want c1", v.ID)
	}
}

func TestLockCycleRequiresOperationID(t *testing.T) {
	srv := NewServer(&stubService{cycles: map[domain.CycleID]CycleView{}})
	body := `{"digest":"d1"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/cycles", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got status %d, want 400", rec.Code)
	}
	var er ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &er); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if er.ErrorCode != domain.CodeInvalidRequest {
		t.Fatalf("got code %q, want INVALID_REQUEST", er.ErrorCode)
	}
}

func TestGetCycleNotFound(t *testing.T) {
	srv := NewServer(&stubService{cycles: map[domain.CycleID]CycleView{}})
	req := httptest.NewRequest(http.MethodGet, "/v1/cycles/missing", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got status %d, want 404", rec.Code)
	}
}

func TestGetCoverage(t *testing.T) {
	srv := NewServer(&stubService{cycles: map[domain.CycleID]CycleView{"c1": {}}})
	req := httptest.NewRequest(http.MethodGet, "/v1/cycles/c1/coverage", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200", rec.Code)
	}
}
