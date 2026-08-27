package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestModel_CycleLockReplayPreservesDeliveredInventory(t *testing.T) {
	tests := []struct {
		name               string
		secondCycleNo      int64
		wantSecondStatus   int
		wantSecondCode     domain.ErrorCode
		panSteelFiberGrams int64
		wantPanStatus      int
		wantPanCode        domain.ErrorCode
	}{
		{
			name:               "first lock makes the delivered batch available",
			panSteelFiberGrams: 50000,
			wantPanStatus:      http.StatusCreated,
		},
		{
			name:               "identical replay returns the first lock without duplicating its batch",
			wantSecondStatus:   http.StatusCreated,
			panSteelFiberGrams: 50001,
			wantPanStatus:      http.StatusBadRequest,
			wantPanCode:        domain.CodeMassConflict,
		},
		{
			name:             "same operation id with different content remains a conflict",
			secondCycleNo:    2,
			wantSecondStatus: http.StatusConflict,
			wantSecondCode:   domain.CodeIdempotencyConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := NewService(filepath.Join(t.TempDir(), "api.db"))
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}
			t.Cleanup(func() { _ = svc.Close() })
			handler := httpapi.NewServer(svc).Handler()

			post := func(path string, body any) *httptest.ResponseRecorder {
				t.Helper()
				payload, err := json.Marshal(body)
				if err != nil {
					t.Fatalf("marshal request: %v", err)
				}
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
				handler.ServeHTTP(rec, req)
				return rec
			}
			decodeError := func(rec *httptest.ResponseRecorder) httpapi.ErrorResponse {
				t.Helper()
				var response httpapi.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
					t.Fatalf("decode error response: %v; body=%s", err, rec.Body.String())
				}
				return response
			}

			lockRequest := validLockRequest()
			first := post("/v1/cycles", lockRequest)
			if first.Code != http.StatusCreated {
				t.Fatalf("first lock status = %d, want 201; body=%s", first.Code, first.Body.String())
			}
			var firstCycle httpapi.CycleView
			if err := json.Unmarshal(first.Body.Bytes(), &firstCycle); err != nil {
				t.Fatalf("decode first lock: %v", err)
			}

			if tt.wantSecondStatus != 0 {
				secondRequest := lockRequest
				if tt.secondCycleNo != 0 {
					secondRequest.Snapshot.CycleNo = tt.secondCycleNo
				}
				second := post("/v1/cycles", secondRequest)
				if second.Code != tt.wantSecondStatus {
					t.Fatalf("second lock status = %d, want %d; body=%s", second.Code, tt.wantSecondStatus, second.Body.String())
				}
				if tt.wantSecondCode != "" {
					if got := decodeError(second).ErrorCode; got != tt.wantSecondCode {
						t.Fatalf("second lock error_code = %q, want %q", got, tt.wantSecondCode)
					}
				} else {
					var replayed httpapi.CycleView
					if err := json.Unmarshal(second.Body.Bytes(), &replayed); err != nil {
						t.Fatalf("decode replayed lock: %v", err)
					}
					if replayed != firstCycle {
						t.Fatalf("replayed lock = %#v, want original %#v", replayed, firstCycle)
					}
				}
			}

			if tt.panSteelFiberGrams == 0 {
				return
			}
			confirmation := post("/v1/cycles/"+string(firstCycle.ID)+"/surface-confirmations", httpapi.SurfaceConfirmRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "op-confirm", LogicalTime: 10},
				Surface:       true,
				Seepage:       true,
			})
			if confirmation.Code != http.StatusCreated {
				t.Fatalf("surface confirmation status = %d, want 201; body=%s", confirmation.Code, confirmation.Body.String())
			}

			pan := post("/v1/cycles/"+string(firstCycle.ID)+"/mix-pans", httpapi.MixPanRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "op-pan", LogicalTime: 20},
				PanID:         "pan-1",
				InputGrams: map[design.MaterialKind]int64{
					design.MaterialSteelFiber: tt.panSteelFiberGrams,
				},
			})
			if pan.Code != tt.wantPanStatus {
				t.Fatalf("mix-pan status = %d, want %d; body=%s", pan.Code, tt.wantPanStatus, pan.Body.String())
			}
			if tt.wantPanCode != "" {
				if got := decodeError(pan).ErrorCode; got != tt.wantPanCode {
					t.Fatalf("mix-pan error_code = %q, want %q", got, tt.wantPanCode)
				}
			}
		})
	}
}
