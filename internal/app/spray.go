package app

import (
	"context"
	"encoding/json"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/coverage"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// AppendSprayBand appends a validated spray band. Layer order, band continuity,
// overlap, material pan, lease validity and no-spray boundaries are all checked;
// any failure writes no effective coverage and deducts no additional material.
func (s *Service) AppendSprayBand(ctx context.Context, id domain.CycleID, req httpapi.SprayBandRequest) (httpapi.BandView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.BandView
	err := s.store.Update(func(tx *store.Tx) error {
		if replay, stored, err := s.guard(tx, req.OperationID, digest); err != nil {
			return err
		} else if replay {
			return decodeResult(stored, &view)
		}
		st, err := tx.GetCycle(id)
		if err != nil {
			return err
		}
		if st == nil {
			return domain.NewError(domain.CodeNotFound, "cycle not found")
		}
		if !st.SurfaceConfirmed || !st.SeepageConfirmed {
			return domain.NewError(domain.CodeInvalidRequest, "surface and seepage must be confirmed first")
		}

		// The sprayer lease must be active with the presented fence token.
		if !s.leases.Active(req.Band.LeaseID, req.Band.FenceToken, req.LogicalTime) {
			return domain.NewError(domain.CodeLeaseExpired, "spray lease inactive or foreign fence")
		}

		pan := findPan(st.Pans, req.Band.PanID)
		if pan == nil {
			return domain.NewError(domain.CodeBatchMismatch, "unknown pan")
		}

		band := coverage.Band{
			ID:          req.Band.ID,
			Seq:         req.Band.Seq,
			StartMM:     req.Band.StartMM,
			EndMM:       req.Band.EndMM,
			WidthMM:     req.Band.WidthMM,
			OverlapMM:   req.Band.OverlapMM,
			PanID:       req.Band.PanID,
			ThicknessMM: req.Band.ThicknessMM,
			LogicalTime: req.LogicalTime,
			FenceToken:  req.Band.FenceToken,
		}
		if derr := st.Cov.AppendBand(req.Generation, req.Band.UnitID, req.Band.Layer, band, minOverlap(st.Snapshot)); derr != nil {
			return derr
		}

		// Record the effective on-wall mass and its proportional key components.
		fiber, accel := proportionOf(req.Band.WallGrams, pan)
		st.Ledger.Add(mass.MassLedgerEntry{
			PanID: req.Band.PanID, Destination: mass.DispositionWall,
			Grams: req.Band.WallGrams, SteelFiberGrams: fiber, AcceleratorGrams: accel,
		})

		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		view = httpapi.BandView{ID: req.Band.ID, UnitID: req.Band.UnitID, Layer: req.Band.Layer, Seq: req.Band.Seq, Valid: true}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// SealRebound seals rebound mass and records its disposition. Rebound may only
// be sealed or disposed of and must never re-enter mixing.
func (s *Service) SealRebound(ctx context.Context, id domain.CycleID, req httpapi.ReboundSealRequest) (httpapi.EvidenceView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.EvidenceView
	err := s.store.Update(func(tx *store.Tx) error {
		if replay, stored, err := s.guard(tx, req.OperationID, digest); err != nil {
			return err
		} else if replay {
			return decodeResult(stored, &view)
		}
		st, err := tx.GetCycle(id)
		if err != nil {
			return err
		}
		if st == nil {
			return domain.NewError(domain.CodeNotFound, "cycle not found")
		}
		if req.ReboundGrams < 0 {
			return domain.NewError(domain.CodeInvalidRequest, "negative rebound mass")
		}
		pan := findPan(st.Pans, req.PanID)
		if pan == nil {
			return domain.NewError(domain.CodeBatchMismatch, "unknown pan")
		}
		fiber, accel := proportionOf(req.ReboundGrams, pan)
		st.Ledger.Add(mass.MassLedgerEntry{
			PanID: req.PanID, Destination: mass.DispositionRebound,
			Grams: req.ReboundGrams, SteelFiberGrams: fiber, AcceleratorGrams: accel,
		})
		ev := newEvidence(req.OperationID, evidenceKind("rebound"), req.LogicalTime, domain.UnitID(""), req.Generation, req.FenceToken)
		st.Evid = append(st.Evid, ev)
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		view = httpapi.EvidenceView{ID: ev.ID, Kind: ev.Kind, LogicalTime: req.LogicalTime, Generation: req.Generation}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// AddCureEvidence appends cure coverage evidence after coverage is closed.
func (s *Service) AddCureEvidence(ctx context.Context, id domain.CycleID, req httpapi.CureEvidenceRequest) (httpapi.EvidenceView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.EvidenceView
	err := s.store.Update(func(tx *store.Tx) error {
		if replay, stored, err := s.guard(tx, req.OperationID, digest); err != nil {
			return err
		} else if replay {
			return decodeResult(stored, &view)
		}
		st, err := tx.GetCycle(id)
		if err != nil {
			return err
		}
		if st == nil {
			return domain.NewError(domain.CodeNotFound, "cycle not found")
		}
		if req.Duration <= 0 {
			return domain.NewError(domain.CodeInvalidRequest, "cure duration must be positive")
		}
		ev := newEvidence(req.OperationID, evidenceKind("cure"), req.LogicalTime, req.UnitID, req.Generation, req.FenceToken)
		st.Evid = append(st.Evid, ev)
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		view = httpapi.EvidenceView{ID: ev.ID, Kind: ev.Kind, LogicalTime: req.LogicalTime, UnitID: req.UnitID, Generation: req.Generation}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// minOverlap returns the minimum band overlap required by the snapshot.
func minOverlap(snap design.DesignSnapshot) int64 {
	if snap.Thresholds.MinOverlapMM > 0 {
		return snap.Thresholds.MinOverlapMM
	}
	return 10
}

// findPan locates a pan by id.
func findPan(pans []mass.MixPan, id domain.PanID) *mass.MixPan {
	for i := range pans {
		if pans[i].ID == id {
			return &pans[i]
		}
	}
	return nil
}

// proportionOf splits a mass into steel-fiber and accelerator parts according
// to the pan's input ratio, using deterministic integer rounding.
func proportionOf(grams int64, pan *mass.MixPan) (fiber, accel int64) {
	total := int64(0)
	for _, g := range pan.InputGrams {
		total += g
	}
	if total <= 0 || grams <= 0 {
		return 0, 0
	}
	fiber = roundDiv(pan.SteelFiberGrams*grams, total)
	accel = roundDiv(pan.AcceleratorGrams*grams, total)
	return fiber, accel
}

// roundDiv returns a/b rounded half away from zero.
func roundDiv(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	q := a / b
	r := a % b
	if r < 0 {
		r = -r
	}
	if r*2 >= b && b > 0 {
		q++
	}
	return q
}

// evidenceKind maps a shorthand to an evidence kind for the seal/cure flows.
func evidenceKind(k string) evidence.EvidenceKind {
	switch k {
	case "rebound":
		return evidence.EvidenceReboundSeal
	default:
		return evidence.EvidenceCureCoverage
	}
}
