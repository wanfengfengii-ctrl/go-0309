package mass

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// Stock is a concrete conditional-decrement inventory. It tracks remaining
// grams per material kind and an optimistic version that increments on every
// successful deduction, so concurrent deductors can detect over-issue even
// before an external database enforces the constraint.
type Stock struct {
	counts  map[design.MaterialKind]int64
	version int64
}

// NewStock builds a Stock from an initial count map (defaulting missing kinds
// to zero).
func NewStock(initial map[design.MaterialKind]int64) *Stock {
	counts := make(map[design.MaterialKind]int64, len(initial))
	for k, v := range initial {
		counts[k] = v
	}
	return &Stock{counts: counts}
}

// Count returns the remaining grams for a kind.
func (s *Stock) Count(kind design.MaterialKind) int64 { return s.counts[kind] }

// Version returns the current inventory version.
func (s *Stock) Version() int64 { return s.version }

// Deduct conditionally decrements the given amount, rejecting over-issue and
// negative amounts. It bumps the version only on success, so a failed deduction
// leaves the version stable for optimistic concurrency checks.
func (s *Stock) Deduct(kind design.MaterialKind, grams int64, expectedVersion int64) error {
	if grams < 0 {
		return domain.NewError(domain.CodeInvalidRequest, "negative deduction")
	}
	if expectedVersion >= 0 && expectedVersion != s.version {
		return domain.NewError(domain.CodeMassConflict, "inventory version conflict")
	}
	if s.counts[kind] < grams {
		return domain.NewError(domain.CodeMassConflict, "insufficient "+string(kind)+" stock")
	}
	s.counts[kind] -= grams
	s.version++
	return nil
}

// Restock adds grams to a kind's stock (deliveries only; never rebound).
func (s *Stock) Restock(kind design.MaterialKind, grams int64) {
	if grams < 0 {
		return
	}
	s.counts[kind] += grams
	s.version++
}

// Snapshot returns a copy of the current counts for persistence.
func (s *Stock) Snapshot() map[design.MaterialKind]int64 {
	out := make(map[design.MaterialKind]int64, len(s.counts))
	for k, v := range s.counts {
		out[k] = v
	}
	return out
}
