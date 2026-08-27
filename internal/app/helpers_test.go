package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/fixedpoint"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	svc, err := NewService(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func validLockRequest() httpapi.LockCycleRequest {
	square := design.Polygon{
		{X: 0, Y: 0}, {X: 1000, Y: 0}, {X: 1000, Y: 1000}, {X: 0, Y: 1000},
	}
	return httpapi.LockCycleRequest{
		OperationID: "op-lock",
		Digest:      "design-v1",
		Snapshot: httpapi.CycleSnapshotDTO{
			Tunnel:          "T1",
			StartMeter:      0,
			EndMeter:        100000,
			CycleNo:         1,
			DesignThickness: 100,
			LayerSequence:   []int64{1},
			SprayDirection:  design.Point{X: 1, Y: 0},
			PoseWindow: design.PoseWindow{
				MinDistanceMM: 100, MaxDistanceMM: 500,
				MinIncidence: 0, MaxIncidence: 900000,
			},
			Thresholds: design.Thresholds{
				MinOverlapMM:   10,
				MinThicknessMM: 100,
				MaxReboundRate: fixedpoint.FromRaw(300000),
				MaxVoidRatio:   fixedpoint.FromRaw(100000),
			},
			Mappings: design.DetectionMapping{},
			RockZones: []design.RockZone{
				{ID: "z1", Name: "zone-1", Looseness: 1},
			},
			Units: []design.SurfaceUnit{
				{ID: "u1", Zone: "z1", Polygon: square},
			},
			Materials: []design.MaterialBatch{
				{ID: "m1", Kind: design.MaterialCement, BatchNo: "c-1", MassGrams: 1000000},
				{ID: "m2", Kind: design.MaterialAggregate, BatchNo: "a-1", MassGrams: 2000000},
				{ID: "m3", Kind: design.MaterialWater, BatchNo: "w-1", MassGrams: 500000},
				{ID: "m4", Kind: design.MaterialAccelerator, BatchNo: "ac-1", MassGrams: 10000},
				{ID: "m5", Kind: design.MaterialSteelFiber, BatchNo: "sf-1", MassGrams: 50000},
			},
		},
	}
}

func mustLock(t *testing.T, svc *Service, req httpapi.LockCycleRequest) domain.CycleID {
	t.Helper()
	view, err := svc.LockCycle(context.Background(), req)
	if err != nil {
		t.Fatalf("LockCycle: %v", err)
	}
	return view.ID
}

// acquireLease grants a single lease and returns its id and fence token.
func acquireLease(t *testing.T, svc *Service, resource mass.ResourceKind, start, end domain.LogicalTime) (domain.LeaseID, string) {
	t.Helper()
	id := domain.LeaseID("lease-" + string(resource))
	fence := "fence-" + string(resource)
	_, err := svc.AcquireLeases(context.Background(), httpapi.LeaseAcquireRequest{
		OperationID: domain.OperationID("op-lease-" + string(resource)),
		Leases: []httpapi.LeaseSpec{
			{ID: id, Resource: resource, Holder: "op", Start: start, End: end, FenceToken: fence},
		},
	})
	if err != nil {
		t.Fatalf("AcquireLeases(%s): %v", resource, err)
	}
	return id, fence
}

// setupReadyForClosure locks a cycle and drives it through the full
// construction, spray, rebound, cure and detection flow so that only the two
// reviews and the terminal decision remain.
func setupReadyForClosure(t *testing.T, svc *Service) domain.CycleID {
	t.Helper()
	ctx := context.Background()
	id := mustLock(t, svc, validLockRequest())

	if _, err := svc.ConfirmSurface(ctx, id, httpapi.SurfaceConfirmRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-confirm", LogicalTime: 10},
		Surface:       true, Seepage: true,
	}); err != nil {
		t.Fatalf("ConfirmSurface: %v", err)
	}

	sprayID, sprayFence := acquireLease(t, svc, mass.ResourceSprayer, 20, 100)

	if _, err := svc.CreateMixPan(ctx, id, httpapi.MixPanRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-pan", LogicalTime: 20},
		PanID:         "pan-1",
		InputGrams: map[design.MaterialKind]int64{
			design.MaterialCement:      1000000,
			design.MaterialAggregate:   2000000,
			design.MaterialWater:       500000,
			design.MaterialAccelerator: 10000,
			design.MaterialSteelFiber:  50000,
		},
	}); err != nil {
		t.Fatalf("CreateMixPan: %v", err)
	}

	if _, err := svc.AppendSprayBand(ctx, id, httpapi.SprayBandRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-band", LogicalTime: 30, Generation: 0},
		Band: httpapi.SprayBandBody{
			ID: "b1", Seq: 1, StartMM: 0, EndMM: 1000, WidthMM: 1000, OverlapMM: 0,
			PanID: "pan-1", UnitID: "u1", Layer: 1, ThicknessMM: 100,
			WallGrams: 3000000, FenceToken: sprayFence, LeaseID: sprayID,
		},
	}); err != nil {
		t.Fatalf("AppendSprayBand: %v", err)
	}

	if _, err := svc.SealRebound(ctx, id, httpapi.ReboundSealRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-rebound", LogicalTime: 40, Generation: 0},
		PanID:         "pan-1", ReboundGrams: 560000, FenceToken: "seal-fence",
	}); err != nil {
		t.Fatalf("SealRebound: %v", err)
	}

	if _, err := svc.AddCureEvidence(ctx, id, httpapi.CureEvidenceRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-cure", LogicalTime: 50, Generation: 0},
		UnitID:        "u1", Duration: 28, FenceToken: "cure-fence",
	}); err != nil {
		t.Fatalf("AddCureEvidence: %v", err)
	}

	if _, err := svc.AddTest(ctx, id, httpapi.TestRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-test", LogicalTime: 60, Generation: 0},
		Kind:          "thickness_scan", UnitID: "u1", RawValue: 120, FenceToken: "test-fence",
	}); err != nil {
		t.Fatalf("AddTest: %v", err)
	}

	return id
}
