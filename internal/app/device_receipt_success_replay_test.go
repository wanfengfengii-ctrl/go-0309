package app_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/app"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestModel_SucceededDeviceCallReceiptIsImmutable(t *testing.T) {
	tests := []struct {
		name          string
		replayedValue string
		wantStatus    int
		wantError     domain.ErrorCode
	}{
		{
			name:          "identical success receipt is a side-effect-free replay",
			replayedValue: "99",
			wantStatus:    http.StatusOK,
		},
		{
			name:          "different late receipt is a stable conflict",
			replayedValue: "101",
			wantStatus:    http.StatusConflict,
			wantError:     domain.CodeGenerationConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := app.NewService(filepath.Join(t.TempDir(), "device.db"))
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}
			t.Cleanup(func() { _ = svc.Close() })
			handler := httpapi.NewServer(svc).Handler()

			create := httptest.NewRequest(http.MethodPost, "/v1/device-calls", strings.NewReader(
				`{"operation_id":"op-thickness","id":"thickness-1","device":"thickness_scanner","params":"scan","logical_time":100,"fence_token":"fence-1"}`,
			))
			created := httptest.NewRecorder()
			handler.ServeHTTP(created, create)
			if created.Code != http.StatusCreated {
				t.Fatalf("create status = %d, want %d: %s", created.Code, http.StatusCreated, created.Body.String())
			}

			first := httptest.NewRequest(http.MethodPost, "/v1/device-calls/thickness-1/receipts", strings.NewReader(
				`{"fence_token":"fence-1","attempt":0,"value":"99"}`,
			))
			firstResult := httptest.NewRecorder()
			handler.ServeHTTP(firstResult, first)
			if firstResult.Code != http.StatusOK {
				t.Fatalf("initial receipt status = %d, want %d: %s", firstResult.Code, http.StatusOK, firstResult.Body.String())
			}
			var accepted httpapi.DeviceCallView
			if err := json.NewDecoder(firstResult.Body).Decode(&accepted); err != nil {
				t.Fatalf("decode initial receipt: %v", err)
			}
			if accepted.Status != evidence.CallSucceeded || accepted.Receipt != "99" {
				t.Fatalf("initial result = %+v, want succeeded receipt 99", accepted)
			}

			replay := httptest.NewRequest(http.MethodPost, "/v1/device-calls/thickness-1/receipts", strings.NewReader(
				fmt.Sprintf(`{"fence_token":"fence-1","attempt":0,"value":%q}`, tt.replayedValue),
			))
			replayed := httptest.NewRecorder()
			handler.ServeHTTP(replayed, replay)
			if replayed.Code != tt.wantStatus {
				t.Fatalf("late receipt status = %d, want %d: %s", replayed.Code, tt.wantStatus, replayed.Body.String())
			}

			if tt.wantError != "" {
				var got httpapi.ErrorResponse
				if err := json.NewDecoder(replayed.Body).Decode(&got); err != nil {
					t.Fatalf("decode conflict: %v", err)
				}
				if got.ErrorCode != tt.wantError {
					t.Fatalf("late receipt error = %q, want %q", got.ErrorCode, tt.wantError)
				}

				verify := httptest.NewRequest(http.MethodPost, "/v1/device-calls/thickness-1/receipts", strings.NewReader(
					`{"fence_token":"fence-1","attempt":0,"value":"99"}`,
				))
				verified := httptest.NewRecorder()
				handler.ServeHTTP(verified, verify)
				if verified.Code != http.StatusOK {
					t.Fatalf("original receipt after conflict status = %d, want %d: %s", verified.Code, http.StatusOK, verified.Body.String())
				}
				var preserved httpapi.DeviceCallView
				if err := json.NewDecoder(verified.Body).Decode(&preserved); err != nil {
					t.Fatalf("decode preserved receipt: %v", err)
				}
				if preserved != accepted {
					t.Fatalf("result after conflict = %+v, want original result %+v", preserved, accepted)
				}
				return
			}

			var got httpapi.DeviceCallView
			if err := json.NewDecoder(replayed.Body).Decode(&got); err != nil {
				t.Fatalf("decode replay: %v", err)
			}
			if got != accepted {
				t.Fatalf("replay result = %+v, want original result %+v", got, accepted)
			}
		})
	}
}
