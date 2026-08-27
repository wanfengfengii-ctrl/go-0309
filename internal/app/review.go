package app

import (
	"context"
	"encoding/json"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// SubmitReview records an independent review.
func (s *Service) SubmitReview(ctx context.Context, id domain.CycleID, req httpapi.ReviewRequest) (httpapi.ReviewView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.ReviewView
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
		review := arbitration.Review{
			ID:         domain.ReviewID("review-" + string(req.OperationID)),
			Reviewer:   req.Reviewer,
			Qualified:  req.Qualified,
			Conclusion: req.Conclusion,
			Digest:     digest,
		}
		st.Reviews = append(st.Reviews, review)
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		view = httpapi.ReviewView{ID: review.ID, Reviewer: review.Reviewer}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// SubmitTerminalDecision competes for the single terminal decision. Exactly one
// writer wins; concurrent losers observe the same existing decision.
func (s *Service) SubmitTerminalDecision(ctx context.Context, id domain.CycleID, req httpapi.DecisionRequest) (httpapi.DecisionView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.DecisionView
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
		if st.Decision != nil {
			view = decisionView(*st.Decision)
			b, _ := json.Marshal(view)
			_ = s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
			return domain.NewError(domain.CodeTerminalAlreadySet, "terminal decision already set")
		}
		if req.Kind == arbitration.TerminalClosure {
			if err := checkClosure(st); err != nil {
				return err
			}
		}
		decision := arbitration.TerminalDecision{
			ID:          domain.DecisionID("decision-" + string(id)),
			Kind:        req.Kind,
			Digest:      digest,
			LogicalTime: req.LogicalTime,
		}
		st.Decision = &decision
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		view = decisionView(decision)
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	if err != nil && domain.AsError(err) != nil && domain.AsError(err).Code == domain.CodeTerminalAlreadySet {
		// The view was populated before the conflict was returned.
		return view, err
	}
	return view, err
}

func decisionView(d arbitration.TerminalDecision) httpapi.DecisionView {
	return httpapi.DecisionView{ID: d.ID, Kind: d.Kind}
}

// checkClosure verifies every closure precondition: mass and rebound
// conservation, complete coverage, cure prefix, detection tests, completed
// repairs and two distinct qualified approving reviewers.
func checkClosure(st *store.CycleState) *domain.Error {
	// Mass conservation across every pan.
	for _, pan := range st.Pans {
		if reasons := st.Ledger.CheckConservation(pan.ID, pan); len(reasons) > 0 {
			e := domain.NewError(domain.CodeMassConflict, "mass not conserved")
			e.WithReasons(reasons...)
			return e
		}
		if err := st.Ledger.RejectReboundReuse(pan.ID); err != nil {
			return err
		}
	}
	if len(st.Pans) == 0 {
		return domain.NewError(domain.CodeInvalidRequest, "no mix pan recorded")
	}

	// Coverage must cover every unit with a closed layer prefix.
	targetUnits := make([]domain.UnitID, 0, len(st.Snapshot.Units))
	for _, u := range st.Snapshot.Units {
		targetUnits = append(targetUnits, u.ID)
	}
	if !st.Cov.Complete(st.CurrentGeneration, targetUnits, int64(len(st.Snapshot.LayerSequence))) {
		return domain.NewError(domain.CodeInvalidRequest, "coverage prefix not closed")
	}

	// Cure evidence must exist for every unit.
	if !allUnitsHaveCure(st) {
		return domain.NewError(domain.CodeInvalidRequest, "cure evidence incomplete")
	}

	// Detection tests must be present.
	if !hasDetectionEvidence(st) {
		return domain.NewError(domain.CodeInvalidRequest, "detection tests missing")
	}

	// Every defect must be repaired.
	for _, d := range st.Defects {
		if d.Status != "repaired" {
			return domain.NewError(domain.CodeInvalidRequest, "defect not repaired")
		}
	}

	// Two distinct qualified approving reviewers.
	if err := arbitration.ValidateReviews(st.Reviews); err != nil {
		return err
	}
	return nil
}

func allUnitsHaveCure(st *store.CycleState) bool {
	covered := map[domain.UnitID]bool{}
	for _, e := range st.Evid {
		if e.Kind == evidence.EvidenceCureCoverage {
			covered[e.UnitID] = true
		}
	}
	for _, u := range st.Snapshot.Units {
		if !covered[u.ID] {
			return false
		}
	}
	return len(st.Snapshot.Units) > 0
}

func hasDetectionEvidence(st *store.CycleState) bool {
	for _, e := range st.Evid {
		switch e.Kind {
		case evidence.EvidenceThicknessScan, evidence.EvidenceProbe, evidence.EvidenceCoreSample,
			evidence.EvidencePlateSpecimen, evidence.EvidencePressure, evidence.EvidencePull:
			return true
		}
	}
	return false
}
