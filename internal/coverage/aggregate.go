package coverage

import (
	"sort"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// State is the append-only coverage aggregate for one excavation cycle. It
// holds one GenerationState per repair generation and never rewrites prior
// generations: a repair only establishes a strictly higher generation while
// the old bands, layers and units are permanently retained.
type State struct {
	Generations []GenerationState `json:"generations"`
}

// Generation returns the state for the given generation, or nil when absent.
func (s *State) Generation(g domain.Generation) *GenerationState {
	for i := range s.Generations {
		if s.Generations[i].Generation == g {
			return &s.Generations[i]
		}
	}
	return nil
}

// EnsureGeneration returns the state for g, creating it in generation order if
// missing. A generation lower than the maximum already present is rejected so
// that a repair can never rewrite an older generation.
func (s *State) EnsureGeneration(g domain.Generation) (*GenerationState, *domain.Error) {
	if g < 0 {
		return nil, domain.NewError(domain.CodeGenerationConflict, "negative generation")
	}
	if gs := s.Generation(g); gs != nil {
		return gs, nil
	}
	if len(s.Generations) > 0 {
		max := s.Generations[len(s.Generations)-1].Generation
		if g < max {
			return nil, domain.NewError(domain.CodeGenerationConflict, "generation may only increase")
		}
		if g != max+1 {
			return nil, domain.NewError(domain.CodeGenerationConflict, "generation must be consecutive")
		}
	}
	s.Generations = append(s.Generations, GenerationState{Generation: g})
	return &s.Generations[len(s.Generations)-1], nil
}

// Unit returns the unit with the given id within a generation, or nil.
func (gs *GenerationState) Unit(id domain.UnitID) *Unit {
	for i := range gs.Units {
		if gs.Units[i].ID == id {
			return &gs.Units[i]
		}
	}
	return nil
}

// EnsureUnit returns the unit with id, creating it if missing.
func (gs *GenerationState) EnsureUnit(id domain.UnitID) *Unit {
	if u := gs.Unit(id); u != nil {
		return u
	}
	gs.Units = append(gs.Units, Unit{ID: id})
	return &gs.Units[len(gs.Units)-1]
}

// AppendBand applies a validated band to a unit within a generation, enforcing
// the append-only layer and band invariants through AppendBand.
func (s *State) AppendBand(g domain.Generation, unitID domain.UnitID, layer int64, b Band, minOverlapMM int64) *domain.Error {
	gs, derr := s.EnsureGeneration(g)
	if derr != nil {
		return derr
	}
	u := gs.EnsureUnit(unitID)
	return AppendBand(u, layer, b, minOverlapMM)
}

// SortedUnitIDs returns all unit ids present in the given generation, sorted by
// canonical key. When g is negative it aggregates every generation.
func (s *State) SortedUnitIDs(g domain.Generation) []domain.UnitID {
	seen := map[domain.UnitID]bool{}
	for _, gen := range s.Generations {
		if g >= 0 && gen.Generation != g {
			continue
		}
		for _, u := range gen.Units {
			seen[u.ID] = true
		}
	}
	out := make([]domain.UnitID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Complete reports whether every target unit in the generation has a closed
// layer prefix through the required layer count.
func (s *State) Complete(g domain.Generation, targetUnits []domain.UnitID, throughLayers int64) bool {
	gs := s.Generation(g)
	if gs == nil {
		return false
	}
	have := map[domain.UnitID]bool{}
	for _, u := range gs.Units {
		have[u.ID] = true
	}
	for _, id := range targetUnits {
		if !have[id] {
			return false
		}
		if !LayerPrefixClosed(gs.Unit(id), throughLayers) {
			return false
		}
	}
	return len(targetUnits) > 0
}

// MaxGeneration returns the highest generation recorded, or -1 when empty.
func (s *State) MaxGeneration() domain.Generation {
	if len(s.Generations) == 0 {
		return -1
	}
	return s.Generations[len(s.Generations)-1].Generation
}
