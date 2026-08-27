package design

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/fixedpoint"
)

// DesignSnapshot is the immutable locked design for a single excavation cycle.
// It is produced by the lock operation and cannot be modified afterwards.
type DesignSnapshot struct {
	ID         domain.CycleID     `json:"id"`
	Tunnel     string             `json:"tunnel"`
	StartMeter int64              `json:"start_meter"` // integer millimetre chainage
	EndMeter   int64              `json:"end_meter"`
	CycleNo    int64              `json:"cycle_no"`
	Digest     string             `json:"digest"` // normalized version summary
	LockTime   domain.LogicalTime `json:"lock_time"`

	RockZones   []RockZone     `json:"rock_zones"`
	Units       []SurfaceUnit  `json:"units"`
	NoSpray     []NoSprayZone  `json:"no_spray"`
	Seepage     []SeepagePoint `json:"seepage"`
	Adjacencies []Adjacency    `json:"adjacencies"`

	DesignThickness int64            `json:"design_thickness"` // millimetres
	LayerSequence   []int64          `json:"layer_sequence"`   // ordered layer numbers
	SprayDirection  Point            `json:"spray_direction"`  // unit direction vector
	PoseWindow      PoseWindow       `json:"pose_window"`
	Thresholds      Thresholds       `json:"thresholds"`
	Mappings        DetectionMapping `json:"mappings"`

	Materials []MaterialBatch `json:"materials"`
}

// RockZone is a surrounding-rock partition with a looseness grade.
type RockZone struct {
	ID        domain.Identifier `json:"id"`
	Name      string            `json:"name"`
	Looseness int               `json:"looseness"` // higher is looser
}

// MaterialBatch is an immutable delivery of a spray constituent.
type MaterialBatch struct {
	ID               domain.Identifier `json:"id"`
	Kind             MaterialKind      `json:"kind"`
	BatchNo          string            `json:"batch_no"`
	MassGrams        int64             `json:"mass_grams"`
	SteelFiberGrams  int64             `json:"steel_fiber_grams,omitempty"`
	AcceleratorGrams int64             `json:"accelerator_grams,omitempty"`
}

// MaterialKind enumerates the spray constituents.
type MaterialKind string

// Spray constituents tracked by the mass ledger.
const (
	MaterialCement      MaterialKind = "cement"
	MaterialAggregate   MaterialKind = "aggregate"
	MaterialWater       MaterialKind = "water"
	MaterialAccelerator MaterialKind = "accelerator"
	MaterialSteelFiber  MaterialKind = "steel_fiber"
)

// MixRatio is the design proportion of the concrete mix. Ratios are stored as
// fixed-point fractions relative to cement mass.
type MixRatio struct {
	WaterCement     fixedpoint.Q `json:"water_cement"`
	AcceleratorDose fixedpoint.Q `json:"accelerator_dose"` // ratio of cement mass
	FiberContent    fixedpoint.Q `json:"fiber_content"`    // ratio of total mass
}

// PoseWindow bounds the nozzle distance and incidence angle during spraying.
type PoseWindow struct {
	MinDistanceMM int64 `json:"min_distance_mm"`
	MaxDistanceMM int64 `json:"max_distance_mm"`
	MinIncidence  int64 `json:"min_incidence"` // scaled degrees, fixed point
	MaxIncidence  int64 `json:"max_incidence"`
}

// Thresholds holds the acceptance performance limits.
type Thresholds struct {
	MinOverlapMM          int64        `json:"min_overlap_mm"`
	MinThicknessMM        int64        `json:"min_thickness_mm"`
	MinFiberContent       fixedpoint.Q `json:"min_fiber_content"`
	MaxReboundRate        fixedpoint.Q `json:"max_rebound_rate"`
	MinStrengthGrowthRate fixedpoint.Q `json:"min_strength_growth_rate"`
	MinBondStrength       fixedpoint.Q `json:"min_bond_strength"`
	MaxVoidRatio          fixedpoint.Q `json:"max_void_ratio"`
}

// DetectionMapping associates measurement points with the evidence types that
// apply to them (thickness scan, probe, core sample, plate specimen).
type DetectionMapping struct {
	ThicknessPoints []domain.Identifier `json:"thickness_points"`
	CoreSpecimens   []domain.Identifier `json:"core_specimens"`
	PlateSpecimens  []domain.Identifier `json:"plate_specimens"`
}

// IsStale reports whether a supplied design digest matches the locked one.
func (s *DesignSnapshot) IsStale(digest string) bool { return s.Digest != digest }
