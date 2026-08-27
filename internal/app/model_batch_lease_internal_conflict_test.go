package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
)

func TestModel_BatchLeaseInternalOverlapIsAtomic(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "lease batch rejects an internal overlap without persisting either lease",
			run: func(t *testing.T) {
				svc := newTestService(t)
				ctx := context.Background()
				conflicting := httpapi.LeaseAcquireRequest{
					OperationID: "op-conflicting-lease-batch",
					Leases: []httpapi.LeaseSpec{
						{ID: "conflict-a", Resource: mass.ResourceSprayer, Holder: "crew-a", Start: 10, End: 30, FenceToken: "fence-a"},
						{ID: "conflict-b", Resource: mass.ResourceSprayer, Holder: "crew-b", Start: 20, End: 40, FenceToken: "fence-b"},
					},
				}

				if _, err := svc.AcquireLeases(ctx, conflicting); err == nil || domain.AsError(err).Code != domain.CodeLeaseConflict {
					t.Fatalf("internal overlap: want LEASE_CONFLICT, got %v", err)
				}

				valid := httpapi.LeaseAcquireRequest{
					OperationID: "op-valid-lease-batch",
					Leases: []httpapi.LeaseSpec{
						{ID: "valid-a", Resource: mass.ResourceSprayer, Holder: "crew-a", Start: 10, End: 20, FenceToken: "valid-fence-a"},
						{ID: "valid-b", Resource: mass.ResourceSprayer, Holder: "crew-b", Start: 20, End: 40, FenceToken: "valid-fence-b"},
						{ID: "valid-c", Resource: mass.ResourceMixer, Holder: "crew-c", Start: 10, End: 40, FenceToken: "valid-fence-c"},
					},
				}
				first, err := svc.AcquireLeases(ctx, valid)
				if err != nil {
					t.Fatalf("touching intervals and a different resource should succeed after rejected batch: %v", err)
				}
				replay, err := svc.AcquireLeases(ctx, valid)
				if err != nil {
					t.Fatalf("idempotent replay: %v", err)
				}
				if !reflect.DeepEqual(replay, first) {
					t.Fatalf("replay result = %#v, want %#v", replay, first)
				}
			},
		},
		{
			name: "mix pan rejects an internal overlap without a pan leases or inventory deduction",
			run: func(t *testing.T) {
				svc := newTestService(t)
				ctx := context.Background()
				cycleID := mustLock(t, svc, validLockRequest())
				if _, err := svc.ConfirmSurface(ctx, cycleID, httpapi.SurfaceConfirmRequest{
					CommandHeader: httpapi.CommandHeader{OperationID: "op-confirm-for-overlap", LogicalTime: 5},
					Surface:       true,
					Seepage:       true,
				}); err != nil {
					t.Fatalf("ConfirmSurface: %v", err)
				}

				conflicting := httpapi.MixPanRequest{
					CommandHeader: httpapi.CommandHeader{OperationID: "op-conflicting-pan", LogicalTime: 10},
					PanID:         "pan-conflicting",
					InputGrams: map[design.MaterialKind]int64{
						design.MaterialCement:      1,
						design.MaterialAggregate:   1,
						design.MaterialWater:       1,
						design.MaterialAccelerator: 1,
						design.MaterialSteelFiber:  1,
					},
					Leases: []httpapi.LeaseSpec{
						{ID: "pan-conflict-a", Resource: mass.ResourceSprayer, Holder: "crew-a", Start: 10, End: 30, FenceToken: "pan-fence-a"},
						{ID: "pan-conflict-b", Resource: mass.ResourceSprayer, Holder: "crew-b", Start: 20, End: 40, FenceToken: "pan-fence-b"},
					},
				}
				if _, err := svc.CreateMixPan(ctx, cycleID, conflicting); err == nil || domain.AsError(err).Code != domain.CodeLeaseConflict {
					t.Fatalf("internal overlap: want LEASE_CONFLICT, got %v", err)
				}

				audit, err := svc.GetAudit(ctx, cycleID)
				if err != nil {
					t.Fatalf("GetAudit after conflict: %v", err)
				}
				if len(audit.Pans) != 0 {
					t.Fatalf("rejected transaction persisted %d pans, want 0", len(audit.Pans))
				}

				valid := httpapi.MixPanRequest{
					CommandHeader: httpapi.CommandHeader{OperationID: "op-valid-pan", LogicalTime: 10},
					PanID:         "pan-valid",
					InputGrams: map[design.MaterialKind]int64{
						design.MaterialCement:      1000000,
						design.MaterialAggregate:   2000000,
						design.MaterialWater:       500000,
						design.MaterialAccelerator: 10000,
						design.MaterialSteelFiber:  50000,
					},
					Leases: []httpapi.LeaseSpec{
						{ID: "pan-valid-a", Resource: mass.ResourceSprayer, Holder: "crew-a", Start: 10, End: 20, FenceToken: "pan-valid-fence-a"},
						{ID: "pan-valid-b", Resource: mass.ResourceSprayer, Holder: "crew-b", Start: 20, End: 40, FenceToken: "pan-valid-fence-b"},
						{ID: "pan-valid-c", Resource: mass.ResourceMixer, Holder: "crew-c", Start: 10, End: 40, FenceToken: "pan-valid-fence-c"},
					},
				}
				first, err := svc.CreateMixPan(ctx, cycleID, valid)
				if err != nil {
					t.Fatalf("valid transaction should retain all inventory and lease capacity: %v", err)
				}
				replay, err := svc.CreateMixPan(ctx, cycleID, valid)
				if err != nil {
					t.Fatalf("idempotent replay: %v", err)
				}
				if !reflect.DeepEqual(replay, first) {
					t.Fatalf("replay result = %#v, want %#v", replay, first)
				}
				audit, err = svc.GetAudit(ctx, cycleID)
				if err != nil {
					t.Fatalf("GetAudit after success: %v", err)
				}
				if len(audit.Pans) != 1 || audit.Pans[0].ID != valid.PanID {
					t.Fatalf("pans after success and replay = %#v, want only %q", audit.Pans, valid.PanID)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, tc.run)
	}
}
