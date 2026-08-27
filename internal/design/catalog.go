package design

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// ValidateSnapshot checks a candidate design snapshot for internal consistency
// and completeness before it may enter the construction state. It returns an
// ordered list of reasons; an empty list means the snapshot is lockable. This
// is the single gate behind the lock transaction boundary: any defect here
// aborts the whole cycle creation and leaves no half-locked state behind.
func ValidateSnapshot(s *DesignSnapshot) []domain.Reason {
	var reasons []domain.Reason

	if s.Tunnel == "" {
		reasons = append(reasons, reason(domain.CodeInvalidRequest, "tunnel required", s.IDKey()))
	}
	if s.EndMeter <= s.StartMeter {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "invalid chainage range", s.IDKey()))
	}
	if s.CycleNo < 0 {
		reasons = append(reasons, reason(domain.CodeInvalidRequest, "negative cycle number", s.IDKey()))
	}
	if s.Digest == "" {
		reasons = append(reasons, reason(domain.CodeStaleSnapshot, "design digest required", s.IDKey()))
	}
	if s.DesignThickness <= 0 {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "design thickness must be positive", s.IDKey()))
	}
	if len(s.LayerSequence) == 0 {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "layer sequence required", s.IDKey()))
	} else {
		for i, n := range s.LayerSequence {
			if n != int64(i)+1 {
				reasons = append(reasons, reason(domain.CodeInvalidGrid, "layer sequence must be consecutive from 1", s.IDKey()))
				break
			}
		}
	}
	if s.SprayDirection.X == 0 && s.SprayDirection.Y == 0 {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "spray direction must be non-zero", s.IDKey()))
	}
	reasons = append(reasons, validatePose(s.PoseWindow, s.IDKey())...)
	reasons = append(reasons, validateThresholds(s.Thresholds, s.IDKey())...)
	reasons = append(reasons, ValidateGrid(s.Units, s.NoSpray)...)
	reasons = append(reasons, validateAdjacencies(s)...)
	reasons = append(reasons, validateMaterials(s)...)

	domain.SortReasons(reasons)
	return reasons
}

// IDKey returns the canonical key fragment for the snapshot's own diagnostics.
func (s *DesignSnapshot) IDKey() string {
	return string(s.ID)
}

func validatePose(p PoseWindow, key string) []domain.Reason {
	var reasons []domain.Reason
	if p.MinDistanceMM <= 0 || p.MaxDistanceMM <= 0 {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "pose distance must be positive", key))
	}
	if p.MaxDistanceMM < p.MinDistanceMM {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "pose distance range invalid", key))
	}
	if p.MinIncidence < 0 || p.MaxIncidence < 0 || p.MaxIncidence < p.MinIncidence {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "pose incidence range invalid", key))
	}
	return reasons
}

func validateThresholds(t Thresholds, key string) []domain.Reason {
	var reasons []domain.Reason
	if t.MinThicknessMM <= 0 {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "minimum thickness must be positive", key))
	}
	if t.MaxReboundRate.Raw() < 0 || t.MaxReboundRate.Raw() > fixedPointOne() {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "rebound rate threshold out of range", key))
	}
	if t.MaxVoidRatio.Raw() < 0 || t.MaxVoidRatio.Raw() > fixedPointOne() {
		reasons = append(reasons, reason(domain.CodeInvalidGrid, "void ratio threshold out of range", key))
	}
	return reasons
}

// fixedPointOne returns the raw fixed-point value representing 1.0. Importing
// the fixedpoint package here would create an import cycle through metrics, so
// the constant is defined locally using the same scale.
const fixedPointScale = 1_000_000

func fixedPointOne() int64 { return fixedPointScale }

func validateAdjacencies(s *DesignSnapshot) []domain.Reason {
	unitSet := make(map[domain.UnitID]bool, len(s.Units))
	for _, u := range s.Units {
		unitSet[u.ID] = true
	}
	var reasons []domain.Reason
	for _, a := range s.Adjacencies {
		if !unitSet[a.A] || !unitSet[a.B] {
			reasons = append(reasons, reason(domain.CodeInvalidGrid, "adjacency references unknown unit", string(s.ID)))
		}
		if a.A == a.B {
			reasons = append(reasons, reason(domain.CodeInvalidGrid, "self-adjacency rejected", string(s.ID)))
		}
	}
	return reasons
}

func validateMaterials(s *DesignSnapshot) []domain.Reason {
	var reasons []domain.Reason
	zoneSet := make(map[domain.Identifier]bool, len(s.RockZones))
	for _, z := range s.RockZones {
		zoneSet[z.ID] = true
	}
	for _, u := range s.Units {
		if !zoneSet[u.Zone] {
			reasons = append(reasons, reason(domain.CodeBatchMismatch, "unit references unknown rock zone", string(u.ID)))
		}
	}
	kinds := make(map[MaterialKind]bool)
	for _, m := range s.Materials {
		if m.MassGrams <= 0 {
			reasons = append(reasons, reason(domain.CodeInvalidRequest, "material mass must be positive", string(m.ID)))
		}
		kinds[m.Kind] = true
	}
	// Every spray constituent must be present to form a valid mix.
	for _, k := range []MaterialKind{
		MaterialCement, MaterialAggregate, MaterialWater, MaterialAccelerator, MaterialSteelFiber,
	} {
		if !kinds[k] {
			reasons = append(reasons, reason(domain.CodeBatchMismatch, "missing material kind "+string(k), string(s.ID)))
		}
	}
	return reasons
}
