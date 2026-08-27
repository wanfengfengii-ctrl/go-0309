package app

import (
	"context"
	"encoding/json"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/fixedpoint"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// AddTest appends a detection test result (scan, probe, core, plate, pressure
// or pull) with a measured metric value.
func (s *Service) AddTest(ctx context.Context, id domain.CycleID, req httpapi.TestRequest) (httpapi.EvidenceView, error) {
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
		ev := evidence.Evidence{
			ID:          evID(req.OperationID, req.Kind),
			Kind:        req.Kind,
			LogicalTime: req.LogicalTime,
			UnitID:      req.UnitID,
			Generation:  req.Generation,
			FenceToken:  req.FenceToken,
			Value:       fixedpoint.FromInt(req.RawValue),
		}
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

// CreateDefect records a defect case and computes its deterministic propagation
// set from surface adjacency, shared pump windows, shared material pans and
// shared rock looseness zones.
func (s *Service) CreateDefect(ctx context.Context, id domain.CycleID, req httpapi.DefectRequest) (httpapi.DefectView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.DefectView
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
		in := buildPropagationInput(st, req.SeedUnit)
		repairSet := arbitration.Propagate(in)
		dc := arbitration.DefectCase{
			ID:           domain.DefectID(evID(req.OperationID, evidence.EvidenceKind("defect"))),
			Type:         req.Type,
			SeedEvidence: req.SeedEvidence,
			RuleVersion:  1,
			RepairSet:    repairSet,
			Status:       "open",
		}
		st.Defects = append(st.Defects, dc)
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		view = httpapi.DefectView{ID: req.Type, Type: req.Type, RepairSet: repairSet}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// CreateRepair registers chipped mass and establishes a new repair generation.
func (s *Service) CreateRepair(ctx context.Context, id domain.CycleID, req httpapi.RepairRequest) (httpapi.RepairView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.RepairView
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
		if req.ChippedGrams < 0 {
			return domain.NewError(domain.CodeInvalidRequest, "negative chipped mass")
		}
		defect := findDefect(st.Defects, req.DefectID)
		if defect == nil {
			return domain.NewError(domain.CodeGenerationConflict, "unknown defect")
		}

		newGen := st.CurrentGeneration + 1
		repair := arbitration.RepairGeneration{
			ID:           domain.RepairGenID(evID(req.OperationID, evidence.EvidenceKind("repair"))),
			Generation:   newGen,
			ChippedGrams: req.ChippedGrams,
			RepairSet:    arbitration.UniqueRepairSet(req.RepairSet),
			RecheckMap:   map[domain.Identifier]domain.EvidenceID{},
			Complete:     false,
		}
		st.Repairs = append(st.Repairs, repair)
		st.CurrentGeneration = newGen
		// Record chipped mass as a ledger disposition for the defect's pan set.
		for _, pan := range st.Pans {
			st.Ledger.Add(mass.MassLedgerEntry{
				PanID: pan.ID, Destination: mass.DispositionChipped, Grams: req.ChippedGrams,
			})
		}
		defect.Status = "repaired"
		if err := tx.PutCycle(id, st); err != nil {
			return err
		}
		view = httpapi.RepairView{ID: repair.ID, Generation: newGen, ChippedGrams: req.ChippedGrams, Complete: false}
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// buildPropagationInput assembles the relationship maps for defect propagation.
func buildPropagationInput(st *store.CycleState, seed domain.UnitID) arbitration.PropagationInput {
	adj := map[domain.UnitID][]domain.UnitID{}
	for _, a := range st.Snapshot.Adjacencies {
		adj[a.A] = append(adj[a.A], a.B)
		adj[a.B] = append(adj[a.B], a.A)
	}
	zoneOf := map[domain.UnitID]domain.Identifier{}
	for _, u := range st.Snapshot.Units {
		zoneOf[u.ID] = u.Zone
	}
	panOf := map[domain.UnitID]domain.PanID{}
	windowOf := map[domain.UnitID]string{}
	for _, gen := range st.Cov.Generations {
		for _, u := range gen.Units {
			for _, l := range u.Layers {
				for _, b := range l.Bands {
					if b.Valid {
						panOf[u.ID] = b.PanID
					}
				}
			}
		}
	}
	for unit, pan := range panOf {
		for _, p := range st.Pans {
			if p.ID == pan {
				windowOf[unit] = windowKey(p)
			}
		}
	}
	return arbitration.PropagationInput{
		Seed:         seed,
		Adjacency:    adj,
		ZoneOf:       zoneOf,
		PanOf:        panOf,
		PumpWindowOf: windowOf,
	}
}

func windowKey(p mass.MixPan) string {
	return string(p.ID) + "@" + itoa(int64(p.PumpWindowStart))
}

func findDefect(defects []arbitration.DefectCase, id domain.DefectID) *arbitration.DefectCase {
	for i := range defects {
		if defects[i].ID == id {
			return &defects[i]
		}
	}
	return nil
}
