package app

import (
	"context"
	"sort"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/fixedpoint"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// GetAudit exports the full, canonically sorted evidence chain for a cycle.
func (s *Service) GetAudit(ctx context.Context, id domain.CycleID) (httpapi.AuditView, error) {
	_ = ctx
	var view httpapi.AuditView
	err := s.store.View(func(tx *store.Tx) error {
		st, err := tx.GetCycle(id)
		if err != nil {
			return err
		}
		if st == nil {
			return domain.NewError(domain.CodeNotFound, "cycle not found")
		}
		view.CycleID = id
		view.Snapshot = cycleView(st.Snapshot)
		view.Pans = st.Pans
		view.Evidence = sortEvidence(st.Evid)
		view.Defects = st.Defects
		view.Repairs = st.Repairs
		view.Reviews = st.Reviews
		view.Decision = st.Decision
		view.Metrics = computeMetrics(st)
		return nil
	})
	return view, err
}

// sortEvidence orders evidence by canonical key (unit, layer, band,
// generation), then by logical time and kind, so the audit chain is fully
// deterministic.
func sortEvidence(ev []evidence.Evidence) []evidence.Evidence {
	out := make([]evidence.Evidence, len(ev))
	copy(out, ev)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ka, kb := a.CanonicalKey(), b.CanonicalKey()
		if ka != kb {
			return domain.Less(ka, kb)
		}
		if a.LogicalTime != b.LogicalTime {
			return a.LogicalTime < b.LogicalTime
		}
		return a.Kind < b.Kind
	})
	return out
}

// computeMetrics derives the engineering metrics required for acceptance from
// the ledger and coverage, using checked fixed-point arithmetic.
func computeMetrics(st *store.CycleState) map[string]fixedpoint.Q {
	out := map[string]fixedpoint.Q{}
	if len(st.Pans) == 0 {
		return out
	}

	var totalInput, totalRebound int64
	for _, pan := range st.Pans {
		for _, g := range pan.InputGrams {
			totalInput += g
		}
	}
	for _, e := range st.Ledger.Entries {
		if e.Destination == mass.DispositionRebound {
			totalRebound += e.Grams
		}
	}
	if totalInput > 0 {
		if r, err := fixedpoint.ReboundRate(totalRebound, totalInput); err == nil {
			out["rebound_rate"] = r
		}
	}

	pan := st.Pans[0]
	total := int64(0)
	for _, g := range pan.InputGrams {
		total += g
	}
	if total > 0 {
		if f, err := fixedpoint.FiberContent(pan.SteelFiberGrams, total); err == nil {
			out["fiber_content"] = f
		}
		if a, err := fixedpoint.AcceleratorDose(pan.AcceleratorGrams, total); err == nil {
			out["accelerator_dose"] = a
		}
	}

	// Effective thickness from band geometry: sum(area*thickness)/sum(area).
	var volume, area int64
	for _, gen := range st.Cov.Generations {
		for _, u := range gen.Units {
			for _, l := range u.Layers {
				for _, b := range l.Bands {
					if !b.Valid {
						continue
					}
					a := (b.EndMM - b.StartMM) * b.WidthMM
					area += a
					volume += a * b.ThicknessMM
				}
			}
		}
	}
	if area > 0 {
		if t, err := fixedpoint.EffectiveThickness(volume, area); err == nil {
			out["effective_thickness"] = t
		}
	}
	return out
}
