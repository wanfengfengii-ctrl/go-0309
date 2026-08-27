package app

import (
	"context"
	"encoding/json"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// LockCycle creates and locks an excavation cycle snapshot. Any snapshot,
// geometry, mapping or numeric error aborts the whole creation inside one
// transaction, leaving no half-locked cycle behind.
func (s *Service) LockCycle(ctx context.Context, req httpapi.LockCycleRequest) (httpapi.CycleView, error) {
	_ = ctx
	digest := digestOf(req)
	snap := design.DesignSnapshot{
		Tunnel:          req.Snapshot.Tunnel,
		StartMeter:      req.Snapshot.StartMeter,
		EndMeter:        req.Snapshot.EndMeter,
		CycleNo:         req.Snapshot.CycleNo,
		Digest:          req.Digest,
		RockZones:       req.Snapshot.RockZones,
		Units:           req.Snapshot.Units,
		NoSpray:         req.Snapshot.NoSpray,
		Seepage:         req.Snapshot.Seepage,
		Adjacencies:     req.Snapshot.Adjacencies,
		DesignThickness: req.Snapshot.DesignThickness,
		LayerSequence:   req.Snapshot.LayerSequence,
		SprayDirection:  req.Snapshot.SprayDirection,
		PoseWindow:      req.Snapshot.PoseWindow,
		Thresholds:      req.Snapshot.Thresholds,
		Mappings:        req.Snapshot.Mappings,
		Materials:       req.Snapshot.Materials,
	}

	// A missing or blank design summary is treated as a stale snapshot: the
	// caller is not working from a current, identifiable design version.
	if req.Digest == "" {
		return httpapi.CycleView{}, domain.NewError(domain.CodeStaleSnapshot, "stale design digest")
	}
	if reasons := design.ValidateSnapshot(&snap); len(reasons) > 0 {
		e := domain.NewError(domain.CodeInvalidGrid, "snapshot validation failed")
		e.WithReasons(reasons...)
		return httpapi.CycleView{}, e
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.CycleView
	var resultJSON string

	err := s.store.Update(func(tx *store.Tx) error {
		if replay, stored, err := s.guard(tx, req.OperationID, digest); err != nil {
			return err
		} else if replay {
			return decodeResult(stored, &view)
		}

		s.nextCycleID++
		id := domain.CycleID(cycleID(s.nextCycleID))
		now := domain.LogicalTime(s.nextCycleID)
		snap.ID = id
		snap.LockTime = now

		view = httpapi.CycleView{
			ID:         id,
			Tunnel:     snap.Tunnel,
			StartMeter: snap.StartMeter,
			EndMeter:   snap.EndMeter,
			CycleNo:    snap.CycleNo,
			Digest:     snap.Digest,
			LockTime:   now,
		}

		st := &store.CycleState{Snapshot: snap}
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		// Seed the global inventory from the delivered material batches.
		for _, m := range snap.Materials {
			cur, err := tx.GetInventory(m.Kind)
			if err != nil {
				return err
			}
			if err := tx.PutInventory(m.Kind, cur+m.MassGrams); err != nil {
				return err
			}
		}
		b, err := json.Marshal(view)
		if err != nil {
			return err
		}
		resultJSON = string(b)
		return s.record(tx, req.OperationID, digest, resultJSON, now)
	})
	if err != nil {
		return httpapi.CycleView{}, err
	}
	// Apply the inventory seeding to the in-memory stock only after the durable
	// commit succeeds.
	for _, m := range snap.Materials {
		s.stock.Restock(m.Kind, m.MassGrams)
	}
	return view, nil
}

// GetCycle returns a locked cycle view.
func (s *Service) GetCycle(ctx context.Context, id domain.CycleID) (httpapi.CycleView, error) {
	_ = ctx
	var view httpapi.CycleView
	err := s.store.View(func(tx *store.Tx) error {
		st, err := tx.GetCycle(id)
		if err != nil {
			return err
		}
		if st == nil {
			return domain.NewError(domain.CodeNotFound, "cycle not found")
		}
		view = cycleView(st.Snapshot)
		return nil
	})
	return view, err
}

// GetCoverage returns the canonical-key-sorted coverage view for a cycle.
func (s *Service) GetCoverage(ctx context.Context, id domain.CycleID) (httpapi.CoverageView, error) {
	_ = ctx
	var view httpapi.CoverageView
	err := s.store.View(func(tx *store.Tx) error {
		st, err := tx.GetCycle(id)
		if err != nil {
			return err
		}
		if st == nil {
			return domain.NewError(domain.CodeNotFound, "cycle not found")
		}
		view.CycleID = id
		for _, g := range st.Cov.Generations {
			view.Generations = append(view.Generations, g.Generation)
		}
		for _, u := range st.Cov.SortedUnitIDs(-1) {
			view.Units = append(view.Units, string(u))
		}
		return nil
	})
	return view, err
}

// cycleView maps a snapshot to its public read model.
func cycleView(s design.DesignSnapshot) httpapi.CycleView {
	return httpapi.CycleView{
		ID:         s.ID,
		Tunnel:     s.Tunnel,
		StartMeter: s.StartMeter,
		EndMeter:   s.EndMeter,
		CycleNo:    s.CycleNo,
		Digest:     s.Digest,
		LockTime:   s.LockTime,
	}
}

// cycleID renders a stable human-readable cycle id.
func cycleID(n int64) string {
	return "cycle-" + itoa(n)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
