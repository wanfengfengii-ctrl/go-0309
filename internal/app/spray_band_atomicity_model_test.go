package app_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/app"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/fixedpoint"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
)

type sprayModelHarness struct {
	handler http.Handler
	cycle   domain.CycleID
	lease   domain.LeaseID
	fence   string
}

func newSprayModelHarness(t *testing.T) sprayModelHarness {
	t.Helper()
	svc, err := app.NewService(filepath.Join(t.TempDir(), "model.db"))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	h := sprayModelHarness{handler: httpapi.NewServer(svc).Handler(), lease: "sprayer-lease", fence: "sprayer-fence"}

	square := design.Polygon{{X: 0, Y: 0}, {X: 1000, Y: 0}, {X: 1000, Y: 1000}, {X: 0, Y: 1000}}
	lock := httpapi.LockCycleRequest{
		OperationID: "lock-cycle", Digest: "design-v1",
		Snapshot: httpapi.CycleSnapshotDTO{
			Tunnel: "T1", StartMeter: 0, EndMeter: 100000, CycleNo: 1,
			DesignThickness: 100, LayerSequence: []int64{1}, SprayDirection: design.Point{X: 1},
			PoseWindow: design.PoseWindow{MinDistanceMM: 100, MaxDistanceMM: 500, MinIncidence: 0, MaxIncidence: 900000},
			Thresholds: design.Thresholds{
				MinOverlapMM: 10, MinThicknessMM: 100,
				MaxReboundRate: fixedpoint.FromRaw(300000), MaxVoidRatio: fixedpoint.FromRaw(100000),
			},
			RockZones: []design.RockZone{{ID: "z1", Name: "zone-1", Looseness: 1}},
			Units:     []design.SurfaceUnit{{ID: "u1", Zone: "z1", Polygon: square}},
			Materials: []design.MaterialBatch{
				{ID: "m1", Kind: design.MaterialCement, BatchNo: "c-1", MassGrams: 1000000},
				{ID: "m2", Kind: design.MaterialAggregate, BatchNo: "a-1", MassGrams: 2000000},
				{ID: "m3", Kind: design.MaterialWater, BatchNo: "w-1", MassGrams: 500000},
				{ID: "m4", Kind: design.MaterialAccelerator, BatchNo: "ac-1", MassGrams: 10000},
				{ID: "m5", Kind: design.MaterialSteelFiber, BatchNo: "sf-1", MassGrams: 50000},
			},
		},
	}
	var cycle httpapi.CycleView
	modelRequireStatus(t, h.handler, http.MethodPost, "/v1/cycles", lock, http.StatusCreated, &cycle)
	h.cycle = cycle.ID
	base := "/v1/cycles/" + string(h.cycle)
	modelRequireStatus(t, h.handler, http.MethodPost, base+"/surface-confirmations", httpapi.SurfaceConfirmRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "confirm-surface", LogicalTime: 10}, Surface: true, Seepage: true,
	}, http.StatusCreated, nil)
	modelRequireStatus(t, h.handler, http.MethodPost, "/v1/leases/acquire", httpapi.LeaseAcquireRequest{
		OperationID: "acquire-sprayer",
		Leases:      []httpapi.LeaseSpec{{ID: h.lease, Resource: mass.ResourceSprayer, Holder: "crew", Start: 20, End: 200, FenceToken: h.fence}},
	}, http.StatusCreated, nil)
	modelRequireStatus(t, h.handler, http.MethodPost, base+"/mix-pans", httpapi.MixPanRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "create-pan", LogicalTime: 20}, PanID: "pan-1",
		InputGrams: map[design.MaterialKind]int64{
			design.MaterialCement: 1000000, design.MaterialAggregate: 2000000, design.MaterialWater: 500000,
			design.MaterialAccelerator: 10000, design.MaterialSteelFiber: 50000,
		},
	}, http.StatusCreated, nil)
	modelRequireStatus(t, h.handler, http.MethodPost, base+"/spray-bands", httpapi.SprayBandRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "seed-band", LogicalTime: 30},
		Band: httpapi.SprayBandBody{
			ID: "b1", Seq: 1, StartMM: 0, EndMM: 500, WidthMM: 1000, PanID: "pan-1", UnitID: "u1", Layer: 1,
			ThicknessMM: 100, WallGrams: 1000000, FenceToken: h.fence, LeaseID: h.lease,
		},
	}, http.StatusCreated, nil)
	return h
}

func modelRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, path, bytes.NewReader(b)))
	return rec
}

func modelRequireStatus(t *testing.T, handler http.Handler, method, path string, body any, want int, out any) {
	t.Helper()
	rec := modelRequest(t, handler, method, path, body)
	if rec.Code != want {
		t.Fatalf("%s %s returned %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
	}
}

func modelGet[T any](t *testing.T, handler http.Handler, path string) T {
	t.Helper()
	var out T
	modelRequireStatus(t, handler, http.MethodGet, path, nil, http.StatusOK, &out)
	return out
}

func TestModel_AppendSprayBandRejectsAtomically(t *testing.T) {
	cases := []struct {
		name     string
		change   func(*httpapi.SprayBandRequest)
		wantCode domain.ErrorCode
	}{
		{
			name: "band discontinuity",
			change: func(req *httpapi.SprayBandRequest) {
				req.Band.Seq = 3
			},
			wantCode: domain.CodeBandDiscontinuity,
		},
		{
			name: "layer jump",
			change: func(req *httpapi.SprayBandRequest) {
				req.Band.Layer = 3
				req.Band.Seq = 1
			},
			wantCode: domain.CodeLayerOutOfOrder,
		},
		{
			name: "insufficient overlap",
			change: func(req *httpapi.SprayBandRequest) {
				req.Band.OverlapMM = 9
			},
			wantCode: domain.CodeOverlapInsufficient,
		},
		{name: "valid band"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newSprayModelHarness(t)
			base := "/v1/cycles/" + string(h.cycle)
			valid := httpapi.SprayBandRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "candidate-band", LogicalTime: 40},
				Band: httpapi.SprayBandBody{
					ID: "b2", Seq: 2, StartMM: 490, EndMM: 1000, WidthMM: 1000, OverlapMM: 10,
					PanID: "pan-1", UnitID: "u1", Layer: 1, ThicknessMM: 100, WallGrams: 2000000,
					FenceToken: h.fence, LeaseID: h.lease,
				},
			}
			candidate := valid
			if tc.change != nil {
				beforeCoverage := modelGet[httpapi.CoverageView](t, h.handler, base+"/coverage")
				beforeAudit := modelGet[httpapi.AuditView](t, h.handler, base+"/audit")
				tc.change(&candidate)
				rec := modelRequest(t, h.handler, http.MethodPost, base+"/spray-bands", candidate)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("rejected band returned %d, want 400: %s", rec.Code, rec.Body.String())
				}
				var apiErr httpapi.ErrorResponse
				if err := json.Unmarshal(rec.Body.Bytes(), &apiErr); err != nil {
					t.Fatalf("decode rejection: %v", err)
				}
				if apiErr.ErrorCode != tc.wantCode {
					t.Fatalf("error code = %q, want %q", apiErr.ErrorCode, tc.wantCode)
				}
				afterCoverage := modelGet[httpapi.CoverageView](t, h.handler, base+"/coverage")
				afterAudit := modelGet[httpapi.AuditView](t, h.handler, base+"/audit")
				if !reflect.DeepEqual(afterCoverage, beforeCoverage) {
					t.Fatalf("coverage changed after rejection: before=%+v after=%+v", beforeCoverage, afterCoverage)
				}
				if !reflect.DeepEqual(afterAudit, beforeAudit) {
					t.Fatalf("audit changed after rejection: before=%+v after=%+v", beforeAudit, afterAudit)
				}
				candidate = valid // Reusing the operation ID must work: rejection stores no idempotency success view.
			}

			var band httpapi.BandView
			modelRequireStatus(t, h.handler, http.MethodPost, base+"/spray-bands", candidate, http.StatusCreated, &band)
			if !band.Valid || band.ID != valid.Band.ID || band.Seq != valid.Band.Seq {
				t.Fatalf("created band = %+v, want valid b2 sequence 2", band)
			}
			coverage := modelGet[httpapi.CoverageView](t, h.handler, base+"/coverage")
			if !reflect.DeepEqual(coverage.Generations, []domain.Generation{0}) || !reflect.DeepEqual(coverage.Units, []string{"u1"}) {
				t.Fatalf("committed coverage = %+v, want generation 0 and unit u1", coverage)
			}

			modelRequireStatus(t, h.handler, http.MethodPost, base+"/rebound-seals", httpapi.ReboundSealRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "seal-rebound", LogicalTime: 50}, PanID: "pan-1", ReboundGrams: 560000,
			}, http.StatusCreated, nil)
			modelRequireStatus(t, h.handler, http.MethodPost, base+"/cure-evidence", httpapi.CureEvidenceRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "record-cure", LogicalTime: 60}, UnitID: "u1", Duration: 28,
			}, http.StatusCreated, nil)
			modelRequireStatus(t, h.handler, http.MethodPost, base+"/tests", httpapi.TestRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "record-test", LogicalTime: 70}, Kind: evidence.EvidenceThicknessScan, UnitID: "u1", RawValue: 120,
			}, http.StatusCreated, nil)
			for i, reviewer := range []domain.PersonID{"reviewer-a", "reviewer-b"} {
				modelRequireStatus(t, h.handler, http.MethodPost, base+"/reviews", httpapi.ReviewRequest{
					CommandHeader: httpapi.CommandHeader{OperationID: domain.OperationID("review-" + string(rune('a'+i))), LogicalTime: domain.LogicalTime(80 + i)},
					Reviewer:      reviewer, Qualified: true, Conclusion: "approve",
				}, http.StatusCreated, nil)
			}
			modelRequireStatus(t, h.handler, http.MethodPost, base+"/terminal-decisions", httpapi.DecisionRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "close-cycle", LogicalTime: 90}, Kind: arbitration.TerminalClosure,
			}, http.StatusCreated, nil)
		})
	}
}
