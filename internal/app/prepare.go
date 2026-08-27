package app

import (
	"context"
	"encoding/json"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// ConfirmSurface records base-surface and seepage confirmation evidence. It is
// the first construction dependency: no mixing or spraying may proceed until
// both confirmations are recorded.
func (s *Service) ConfirmSurface(ctx context.Context, id domain.CycleID, req httpapi.SurfaceConfirmRequest) (httpapi.EvidenceView, error) {
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
		if req.Surface {
			st.SurfaceConfirmed = true
			st.Evid = append(st.Evid, newEvidence(req.OperationID, evidence.EvidenceSurfaceConfirm, req.LogicalTime, domain.UnitID(""), req.Generation, ""))
		}
		if req.Seepage {
			st.SeepageConfirmed = true
			st.Evid = append(st.Evid, newEvidence(req.OperationID, evidence.EvidenceSeepageConfirm, req.LogicalTime, domain.UnitID(""), req.Generation, ""))
		}
		view = httpapi.EvidenceView{ID: evID(req.OperationID, evidence.EvidenceSurfaceConfirm), Kind: evidence.EvidenceSurfaceConfirm, LogicalTime: req.LogicalTime, Generation: req.Generation}
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// CreateMixPan atomically deducts stock, acquires the preparation leases,
// creates the pan and appends the input ledger entry. Any failure rolls the
// whole operation back, so no material is deducted without a committed pan.
func (s *Service) CreateMixPan(ctx context.Context, id domain.CycleID, req httpapi.MixPanRequest) (httpapi.PanView, error) {
	_ = ctx
	digest := digestOf(req)

	// Validate inputs before touching any state.
	if req.PanID == "" {
		return httpapi.PanView{}, domain.NewError(domain.CodeInvalidRequest, "pan id required")
	}
	total, fiber, accel, err := sumInputs(req.InputGrams)
	if err != nil {
		return httpapi.PanView{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.PanView
	err = s.store.Update(func(tx *store.Tx) error {
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

		// Validate lease conflicts and stock sufficiency (no mutation yet).
		leaseRecords := make([]mass.ResourceLease, 0, len(req.Leases))
		for _, spec := range req.Leases {
			if conflicts, err := s.leases.Conflicts(spec.Resource, spec.Start, spec.End); err != nil {
				return err
			} else if len(conflicts) > 0 {
				return domain.NewError(domain.CodeLeaseConflict, "resource "+string(spec.Resource)+" already leased")
			}
			leaseRecords = append(leaseRecords, mass.ResourceLease{
				ID: spec.ID, Resource: spec.Resource, Holder: spec.Holder,
				Start: spec.Start, End: spec.End, FenceToken: spec.FenceToken,
			})
		}
		for kind, grams := range req.InputGrams {
			if s.stock.Count(kind) < grams {
				return domain.NewError(domain.CodeMassConflict, "insufficient "+string(kind)+" stock")
			}
		}

		// Apply to the durable store first.
		pan := mass.MixPan{
			ID:               req.PanID,
			InputGrams:       req.InputGrams,
			SteelFiberGrams:  fiber,
			AcceleratorGrams: accel,
			PumpWindowStart:  req.LogicalTime,
			PumpWindowEnd:    req.LogicalTime + 100,
		}
		st.Pans = append(st.Pans, pan)
		st.Ledger.Add(mass.MassLedgerEntry{
			PanID: req.PanID, Source: mass.DispositionInput, Destination: mass.DispositionInput, Grams: total,
		})
		for _, l := range leaseRecords {
			if err := tx.PutLease(l); err != nil {
				return err
			}
		}
		for kind, grams := range req.InputGrams {
			if err := tx.PutInventory(kind, s.stock.Count(kind)-grams); err != nil {
				return err
			}
		}
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}

		view = httpapi.PanView{ID: req.PanID, InputGrams: req.InputGrams, LogicalTime: req.LogicalTime}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	if err != nil {
		return httpapi.PanView{}, err
	}
	// Commit succeeded: apply the in-memory mutations.
	for _, l := range leaseRecordsFrom(req.Leases) {
		s.leases.Insert(l)
	}
	for kind, grams := range req.InputGrams {
		_ = s.stock.Deduct(kind, grams, -1)
	}
	return view, nil
}

// AcquireLeases grants a batch of leases all-or-nothing.
func (s *Service) AcquireLeases(ctx context.Context, req httpapi.LeaseAcquireRequest) (httpapi.LeaseAcquireResult, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	records := leaseRecordsFrom(req.Leases)
	var result httpapi.LeaseAcquireResult
	err := s.store.Update(func(tx *store.Tx) error {
		if replay, stored, err := s.guard(tx, req.OperationID, digest); err != nil {
			return err
		} else if replay {
			return decodeResult(stored, &result)
		}
		// Validate conflicts before acquiring.
		for _, l := range records {
			if conflicts, err := s.leases.Conflicts(l.Resource, l.Start, l.End); err != nil {
				return err
			} else if len(conflicts) > 0 {
				return domain.NewError(domain.CodeLeaseConflict, "resource "+string(l.Resource)+" already leased")
			}
		}
		for _, l := range records {
			if err := tx.PutLease(l); err != nil {
				return err
			}
		}
		result = httpapi.LeaseAcquireResult{Granted: leaseIDs(records)}
		b, _ := json.Marshal(result)
		return s.record(tx, req.OperationID, digest, string(b), records[0].Start)
	})
	if err != nil {
		return httpapi.LeaseAcquireResult{}, err
	}
	for _, l := range records {
		s.leases.Insert(l)
	}
	return result, nil
}

// RenewLease extends a lease's end time.
func (s *Service) RenewLease(ctx context.Context, req httpapi.LeaseRenewRequest) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.leases.Renew(req.LeaseID, req.FenceToken, req.NewEnd)
	if err != nil {
		return err
	}
	return s.store.Update(func(tx *store.Tx) error {
		l, ok, err := tx.GetLease(req.LeaseID)
		if err != nil {
			return err
		}
		if !ok {
			return domain.NewError(domain.CodeLeaseExpired, "lease not found")
		}
		l.End = req.NewEnd
		return tx.PutLease(*l)
	})
}

// ReleaseLease releases a lease.
func (s *Service) ReleaseLease(ctx context.Context, req httpapi.LeaseReleaseRequest) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.leases.Release(req.LeaseID, req.FenceToken); err != nil {
		return err
	}
	return s.store.Update(func(tx *store.Tx) error {
		return tx.DeleteLease(req.LeaseID)
	})
}

// sumInputs validates and totals a pan input map.
func sumInputs(inputs map[design.MaterialKind]int64) (total, fiber, accel int64, err error) {
	if len(inputs) == 0 {
		return 0, 0, 0, domain.NewError(domain.CodeInvalidRequest, "input grams required")
	}
	for kind, grams := range inputs {
		if grams < 0 {
			return 0, 0, 0, domain.NewError(domain.CodeInvalidRequest, "negative input grams")
		}
		total += grams
		switch kind {
		case design.MaterialSteelFiber:
			fiber += grams
		case design.MaterialAccelerator:
			accel += grams
		}
	}
	return total, fiber, accel, nil
}

// leaseRecordsFrom converts transport lease specs to domain leases.
func leaseRecordsFrom(specs []httpapi.LeaseSpec) []mass.ResourceLease {
	out := make([]mass.ResourceLease, 0, len(specs))
	for _, spec := range specs {
		out = append(out, mass.ResourceLease{
			ID: spec.ID, Resource: spec.Resource, Holder: spec.Holder,
			Start: spec.Start, End: spec.End, FenceToken: spec.FenceToken,
		})
	}
	return out
}

func leaseIDs(records []mass.ResourceLease) []domain.LeaseID {
	out := make([]domain.LeaseID, 0, len(records))
	for _, l := range records {
		out = append(out, l.ID)
	}
	return out
}

// newEvidence builds an evidence record with a stable id.
func newEvidence(op domain.OperationID, kind evidence.EvidenceKind, at domain.LogicalTime, unit domain.UnitID, gen domain.Generation, fence string) evidence.Evidence {
	return evidence.Evidence{
		ID:          evID(op, kind),
		Kind:        kind,
		LogicalTime: at,
		UnitID:      unit,
		Generation:  gen,
		FenceToken:  fence,
	}
}

func evID(op domain.OperationID, kind evidence.EvidenceKind) domain.EvidenceID {
	return domain.EvidenceID(string(op) + "-" + string(kind))
}
