package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestModel_LockCycleIdempotencyHasNoInventorySideEffects(t *testing.T) {
	cases := []struct {
		name         string
		changeReplay func(*httpapi.LockCycleRequest)
		wantConflict bool
	}{
		{
			name:         "identical replay",
			changeReplay: func(*httpapi.LockCycleRequest) {},
		},
		{
			name: "content conflict",
			changeReplay: func(req *httpapi.LockCycleRequest) {
				req.Snapshot.CycleNo++
			},
			wantConflict: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dbPath := filepath.Join(t.TempDir(), "service.db")
			svc, err := NewService(dbPath)
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}

			lockReq := validLockRequest()
			first, err := svc.LockCycle(ctx, lockReq)
			if err != nil {
				t.Fatalf("first LockCycle: %v", err)
			}

			replayReq := validLockRequest()
			tc.changeReplay(&replayReq)
			replayed, replayErr := svc.LockCycle(ctx, replayReq)
			if tc.wantConflict {
				if replayErr == nil || domain.AsError(replayErr).Code != domain.CodeIdempotencyConflict {
					t.Fatalf("conflicting LockCycle error = %v, want %s", replayErr, domain.CodeIdempotencyConflict)
				}
			} else {
				if replayErr != nil {
					t.Fatalf("replayed LockCycle: %v", replayErr)
				}
				if replayed.ID != first.ID {
					t.Fatalf("replayed cycle ID = %q, want original %q", replayed.ID, first.ID)
				}
			}

			if _, err := svc.ConfirmSurface(ctx, first.ID, httpapi.SurfaceConfirmRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "op-confirm-idempotency-stock", LogicalTime: 10},
				Surface:       true,
				Seepage:       true,
			}); err != nil {
				t.Fatalf("ConfirmSurface: %v", err)
			}

			_, oversizedErr := svc.CreateMixPan(ctx, first.ID, httpapi.MixPanRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "op-pan-oversized", LogicalTime: 20},
				PanID:         "pan-oversized",
				InputGrams: map[design.MaterialKind]int64{
					design.MaterialSteelFiber: 50001,
				},
			})
			if oversizedErr == nil || domain.AsError(oversizedErr).Code != domain.CodeMassConflict {
				t.Errorf("oversized pan error before restart = %v, want %s", oversizedErr, domain.CodeMassConflict)
			}

			if err := svc.Close(); err != nil {
				t.Fatalf("Close before restart: %v", err)
			}
			svc, err = NewService(dbPath)
			if err != nil {
				t.Fatalf("NewService after restart: %v", err)
			}
			t.Cleanup(func() { _ = svc.Close() })

			pan, err := svc.CreateMixPan(ctx, first.ID, httpapi.MixPanRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "op-pan-persisted-stock", LogicalTime: 30},
				PanID:         "pan-persisted-stock",
				InputGrams: map[design.MaterialKind]int64{
					design.MaterialSteelFiber: 50000,
				},
			})
			if err != nil {
				t.Fatalf("legal pan after restart: %v", err)
			}
			if got := pan.InputGrams[design.MaterialSteelFiber]; got != 50000 {
				t.Fatalf("steel fiber input = %d, want 50000", got)
			}
		})
	}
}
