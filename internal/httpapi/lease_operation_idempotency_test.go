package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/app"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestModel_LeaseOperationIdempotency(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		request         map[string]any
		conflictRequest map[string]any
		probes          []struct {
			path string
			body map[string]any
			code domain.ErrorCode
		}
	}{
		{
			name: "renew replay is successful and preserves validation",
			path: "/v1/leases/renew",
			request: map[string]any{
				"operation_id": "renew-once", "lease_id": "primary", "fence_token": "f-primary", "new_end": 30,
			},
			conflictRequest: map[string]any{
				"operation_id": "renew-once", "lease_id": "primary", "fence_token": "f-primary", "new_end": 25,
			},
			probes: []struct {
				path string
				body map[string]any
				code domain.ErrorCode
			}{
				{path: "/v1/leases/renew", body: map[string]any{"operation_id": "foreign-renew", "lease_id": "primary", "fence_token": "foreign", "new_end": 31}, code: domain.CodeLeaseConflict},
				{path: "/v1/leases/renew", body: map[string]any{"operation_id": "overlap-renew", "lease_id": "primary", "fence_token": "f-primary", "new_end": 31}, code: domain.CodeLeaseConflict},
				{path: "/v1/leases/renew", body: map[string]any{"operation_id": "invalid-renew", "lease_id": "primary", "fence_token": "f-primary", "new_end": 30}, code: domain.CodeInvalidRequest},
				{path: "/v1/leases/renew", body: map[string]any{"operation_id": "missing-renew", "lease_id": "missing", "fence_token": "none", "new_end": 30}, code: domain.CodeLeaseExpired},
			},
		},
		{
			name: "release replay is successful and preserves validation",
			path: "/v1/leases/release",
			request: map[string]any{
				"operation_id": "release-once", "lease_id": "primary", "fence_token": "f-primary",
			},
			conflictRequest: map[string]any{
				"operation_id": "release-once", "lease_id": "primary", "fence_token": "different",
			},
			probes: []struct {
				path string
				body map[string]any
				code domain.ErrorCode
			}{
				{path: "/v1/leases/release", body: map[string]any{"operation_id": "foreign-release", "lease_id": "validation", "fence_token": "foreign"}, code: domain.CodeLeaseConflict},
				{path: "/v1/leases/release", body: map[string]any{"operation_id": "missing-release", "lease_id": "missing", "fence_token": "none"}, code: domain.CodeLeaseExpired},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := app.NewService(filepath.Join(t.TempDir(), "leases.db"))
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}
			t.Cleanup(func() { _ = svc.Close() })
			handler := httpapi.NewServer(svc).Handler()

			post := func(path string, body map[string]any) *httptest.ResponseRecorder {
				t.Helper()
				payload, err := json.Marshal(body)
				if err != nil {
					t.Fatalf("marshal request: %v", err)
				}
				req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				res := httptest.NewRecorder()
				handler.ServeHTTP(res, req)
				return res
			}

			acquire := post("/v1/leases/acquire", map[string]any{
				"operation_id": "acquire-fixture",
				"leases": []map[string]any{
					{"id": "primary", "resource": "sprayer", "holder": "crew-a", "start": 0, "end": 20, "fence_token": "f-primary"},
					{"id": "blocker", "resource": "sprayer", "holder": "crew-b", "start": 30, "end": 40, "fence_token": "f-blocker"},
					{"id": "validation", "resource": "mixer", "holder": "crew-c", "start": 0, "end": 20, "fence_token": "f-validation"},
				},
			})
			if acquire.Code != http.StatusCreated {
				t.Fatalf("fixture acquire status = %d, body = %s", acquire.Code, acquire.Body.String())
			}

			for attempt := 1; attempt <= 2; attempt++ {
				res := post(tt.path, tt.request)
				if res.Code != http.StatusOK {
					t.Fatalf("attempt %d status = %d, want %d; body = %s", attempt, res.Code, http.StatusOK, res.Body.String())
				}
				var success map[string]string
				if err := json.Unmarshal(res.Body.Bytes(), &success); err != nil {
					t.Fatalf("attempt %d decode success: %v", attempt, err)
				}
				if success["status"] != "ok" {
					t.Fatalf("attempt %d status body = %q, want ok", attempt, success["status"])
				}
			}

			conflict := post(tt.path, tt.conflictRequest)
			var conflictBody httpapi.ErrorResponse
			if err := json.Unmarshal(conflict.Body.Bytes(), &conflictBody); err != nil {
				t.Fatalf("decode content-conflict response: %v", err)
			}
			if conflict.Code != http.StatusConflict || conflictBody.ErrorCode != domain.CodeIdempotencyConflict {
				t.Fatalf("different content status/code = %d/%s, want %d/%s; body = %s", conflict.Code, conflictBody.ErrorCode, http.StatusConflict, domain.CodeIdempotencyConflict, conflict.Body.String())
			}

			for _, probe := range tt.probes {
				res := post(probe.path, probe.body)
				var failure httpapi.ErrorResponse
				if err := json.Unmarshal(res.Body.Bytes(), &failure); err != nil {
					t.Fatalf("decode %s response: %v", probe.code, err)
				}
				if failure.ErrorCode != probe.code {
					t.Errorf("probe status/code = %d/%s, want error code %s; body = %s", res.Code, failure.ErrorCode, probe.code, res.Body.String())
				}
			}
		})
	}
}
