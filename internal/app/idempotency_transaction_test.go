package app

import (
	"context"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

func TestIdempotentReplayReturnsSameResult(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	req := validLockRequest()
	first := mustLock(t, svc, req)
	view, err := svc.LockCycle(ctx, req)
	if err != nil {
		t.Fatalf("replay LockCycle: %v", err)
	}
	if view.ID != first {
		t.Fatalf("replay id %q != %q", view.ID, first)
	}
	// A second replay must not create a second cycle.
	ids := cycleIDs(t, svc)
	if len(ids) != 1 {
		t.Fatalf("expected 1 cycle, got %d", len(ids))
	}
}

func TestIdempotencyConflictOnDifferentContent(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	mustLock(t, svc, validLockRequest())

	conflict := validLockRequest()
	conflict.OperationID = "op-lock" // same operation id
	conflict.Snapshot.CycleNo = 2    // different content
	_, err := svc.LockCycle(ctx, conflict)
	if err == nil || domain.AsError(err).Code != domain.CodeIdempotencyConflict {
		t.Fatalf("want IDEMPOTENCY_CONFLICT, got %v", err)
	}
}

func TestFailedMixPanLeavesNoPartialState(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	id := mustLock(t, svc, validLockRequest())

	if _, err := svc.ConfirmSurface(ctx, id, httpapi.SurfaceConfirmRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-confirm", LogicalTime: 10},
		Surface:       true, Seepage: true,
	}); err != nil {
		t.Fatalf("ConfirmSurface: %v", err)
	}

	// Request more steel fiber than the delivered batch provides.
	_, err := svc.CreateMixPan(ctx, id, httpapi.MixPanRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-pan", LogicalTime: 20},
		PanID:         "pan-1",
		InputGrams: map[design.MaterialKind]int64{
			design.MaterialCement:      1000000,
			design.MaterialSteelFiber:  999999999, // exceeds stock
			design.MaterialAggregate:   2000000,
			design.MaterialWater:       500000,
			design.MaterialAccelerator: 10000,
		},
	})
	if err == nil || domain.AsError(err).Code != domain.CodeMassConflict {
		t.Fatalf("want MASS_CONFLICT, got %v", err)
	}

	audit, err := svc.GetAudit(ctx, id)
	if err != nil {
		t.Fatalf("GetAudit: %v", err)
	}
	if len(audit.Pans) != 0 {
		t.Fatalf("expected no pans after failed mix, got %d", len(audit.Pans))
	}
	if len(audit.Evidence) != 2 {
		t.Fatalf("expected only confirm evidence, got %d", len(audit.Evidence))
	}
}

func cycleIDs(t *testing.T, svc *Service) []domain.CycleID {
	t.Helper()
	var ids []domain.CycleID
	err := svc.store.View(func(tx *store.Tx) error {
		var err error
		ids, err = tx.CycleIDs()
		return err
	})
	if err != nil {
		t.Fatalf("CycleIDs: %v", err)
	}
	return ids
}
