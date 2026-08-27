package app_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/app"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestModel_DeviceCallIdentityPreservesRetryState(t *testing.T) {
	tests := []struct {
		name         string
		operationID  domain.OperationID
		params       string
		fence        string
		wantConflict bool
	}{
		{
			name:        "same operation and content replays original result",
			operationID: "op-call-3",
			params:      "scan",
			fence:       "fence-1",
		},
		{
			name:         "same operation with different content conflicts",
			operationID:  "op-call-3",
			params:       "scan-adjusted",
			fence:        "fence-1",
			wantConflict: true,
		},
		{
			name:         "new operation with the same call content conflicts",
			operationID:  "op-call-3-recreated",
			params:       "scan",
			fence:        "fence-1",
			wantConflict: true,
		},
		{
			name:         "new operation with different call content conflicts",
			operationID:  "op-call-3-replaced",
			params:       "scan-adjusted",
			fence:        "replacement-fence",
			wantConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "device-calls.db")
			svc, err := app.NewService(path)
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}

			callID := domain.DeviceCallID("call-3")
			original := httpapi.DeviceCallRequest{
				OperationID: "op-call-3",
				ID:          callID,
				Device:      evidence.DeviceThickness,
				Params:      "scan",
				LogicalTime: 100,
				FenceToken:  "fence-1",
			}
			created, err := svc.CreateDeviceCall(ctx, original)
			if err != nil {
				_ = svc.Close()
				t.Fatalf("CreateDeviceCall(original): %v", err)
			}

			retrying, err := svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{
				Fence: "fence-1", Attempt: 0, Fault: evidence.FaultDisconnect,
			})
			if err != nil {
				_ = svc.Close()
				t.Fatalf("SubmitReceipt(disconnect): %v", err)
			}
			if retrying.Status != evidence.CallRetrying || retrying.Attempt != 1 ||
				retrying.RetryAfter == 0 || retrying.Fault != evidence.FaultDisconnect {
				_ = svc.Close()
				t.Fatalf("unexpected retry state: %+v", retrying)
			}

			duplicate := original
			duplicate.OperationID = tt.operationID
			duplicate.Params = tt.params
			duplicate.FenceToken = tt.fence
			for attempt := 0; attempt < 2; attempt++ {
				got, duplicateErr := svc.CreateDeviceCall(ctx, duplicate)
				if tt.wantConflict {
					if duplicateErr == nil || domain.AsError(duplicateErr).Code != domain.CodeIdempotencyConflict {
						t.Errorf("duplicate creation %d: want IDEMPOTENCY_CONFLICT, got view=%+v err=%v", attempt+1, got, duplicateErr)
					}
				} else if duplicateErr != nil || !reflect.DeepEqual(got, created) {
					t.Errorf("replay %d: want original result %+v, got view=%+v err=%v", attempt+1, created, got, duplicateErr)
				}
			}

			if err := svc.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			svc, err = app.NewService(path)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			t.Cleanup(func() { _ = svc.Close() })

			got, duplicateErr := svc.CreateDeviceCall(ctx, duplicate)
			if tt.wantConflict {
				if duplicateErr == nil || domain.AsError(duplicateErr).Code != domain.CodeIdempotencyConflict {
					t.Errorf("post-restart duplicate: want IDEMPOTENCY_CONFLICT, got view=%+v err=%v", got, duplicateErr)
				}
			} else if duplicateErr != nil || !reflect.DeepEqual(got, created) {
				t.Errorf("post-restart replay: want original result %+v, got view=%+v err=%v", created, got, duplicateErr)
			}

			if _, err := svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{
				Fence: "replacement-fence", Attempt: 1, Value: "99",
			}); err == nil || domain.AsError(err).Code != domain.CodeGenerationConflict {
				t.Errorf("stale fence: want GENERATION_CONFLICT, got %v", err)
			}
			if _, err := svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{
				Fence: "fence-1", Attempt: 0, Value: "99",
			}); err == nil || domain.AsError(err).Code != domain.CodeGenerationConflict {
				t.Errorf("old attempt: want GENERATION_CONFLICT, got %v", err)
			}

			succeeded, err := svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{
				Fence: "fence-1", Attempt: 1, Value: "99",
			})
			if err != nil {
				t.Fatalf("current receipt after restart: %v", err)
			}
			if succeeded.Status != evidence.CallSucceeded || succeeded.Attempt != retrying.Attempt ||
				succeeded.RetryAfter != retrying.RetryAfter || succeeded.Fault != retrying.Fault ||
				succeeded.Receipt != "99" {
				t.Fatalf("retry state was not preserved through success: before=%+v after=%+v", retrying, succeeded)
			}
		})
	}
}
