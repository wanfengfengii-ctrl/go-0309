package httpapi

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/fixedpoint"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
)

// EvidenceView is the public read model of an appended evidence record.
type EvidenceView struct {
	ID          domain.EvidenceID     `json:"id"`
	Kind        evidence.EvidenceKind `json:"kind"`
	LogicalTime domain.LogicalTime    `json:"logical_time"`
	UnitID      domain.UnitID         `json:"unit_id,omitempty"`
	Generation  domain.Generation     `json:"generation,omitempty"`
}

// SurfaceConfirmRequest records base-surface and seepage confirmation for the
// construction-preparation stage.
type SurfaceConfirmRequest struct {
	CommandHeader
	Surface bool `json:"surface"`
	Seepage bool `json:"seepage"`
}

// MixPanRequest creates a pan: it atomically deducts the listed inputs, grants
// the requested preparation leases and appends the input ledger entries.
type MixPanRequest struct {
	CommandHeader
	PanID      domain.PanID                  `json:"pan_id"`
	InputGrams map[design.MaterialKind]int64 `json:"input_grams"`
	Leases     []LeaseSpec                   `json:"leases"`
}

// PanView is the public read model of a created pan.
type PanView struct {
	ID          domain.PanID                  `json:"id"`
	InputGrams  map[design.MaterialKind]int64 `json:"input_grams"`
	LogicalTime domain.LogicalTime            `json:"logical_time"`
}

// SprayBandRequest appends one spray band to a unit's layer.
type SprayBandRequest struct {
	CommandHeader
	Band SprayBandBody `json:"band"`
}

// SprayBandBody carries the band geometry and references without exposing the
// internal append-only status field.
type SprayBandBody struct {
	ID          domain.BandID  `json:"id"`
	Seq         int64          `json:"seq"`
	StartMM     int64          `json:"start_mm"`
	EndMM       int64          `json:"end_mm"`
	WidthMM     int64          `json:"width_mm"`
	OverlapMM   int64          `json:"overlap_mm"`
	PanID       domain.PanID   `json:"pan_id"`
	UnitID      domain.UnitID  `json:"unit_id"`
	Layer       int64          `json:"layer"`
	ThicknessMM int64          `json:"thickness_mm"`
	WallGrams   int64          `json:"wall_grams"`
	FenceToken  string         `json:"fence_token"`
	LeaseID     domain.LeaseID `json:"lease_id"`
}

// BandView is the public read model of an appended band.
type BandView struct {
	ID     domain.BandID `json:"id"`
	UnitID domain.UnitID `json:"unit_id"`
	Layer  int64         `json:"layer"`
	Seq    int64         `json:"seq"`
	Valid  bool          `json:"valid"`
}

// ReboundSealRequest seals rebound mass and records its disposition.
type ReboundSealRequest struct {
	CommandHeader
	PanID        domain.PanID `json:"pan_id"`
	ReboundGrams int64        `json:"rebound_grams"`
	FenceToken   string       `json:"fence_token"`
}

// CureEvidenceRequest appends cure coverage evidence for a unit.
type CureEvidenceRequest struct {
	CommandHeader
	UnitID     domain.UnitID      `json:"unit_id"`
	Duration   domain.LogicalTime `json:"duration"`
	FenceToken string             `json:"fence_token"`
}

// TestRequest appends a detection test result with a measured metric.
type TestRequest struct {
	CommandHeader
	Kind       evidence.EvidenceKind `json:"kind"`
	UnitID     domain.UnitID         `json:"unit_id"`
	RawValue   int64                 `json:"raw_value"`
	FenceToken string                `json:"fence_token"`
}

// DefectRequest records a defect case and computes its propagation set.
type DefectRequest struct {
	CommandHeader
	Type         arbitration.DefectType `json:"type"`
	SeedUnit     domain.UnitID          `json:"seed_unit"`
	SeedEvidence domain.EvidenceID      `json:"seed_evidence"`
}

// DefectView is the public read model of a defect case.
type DefectView struct {
	ID        arbitration.DefectType `json:"id"`
	Type      arbitration.DefectType `json:"type"`
	RepairSet []domain.UnitID        `json:"repair_set"`
}

// RepairRequest registers chipped mass and establishes a new generation.
type RepairRequest struct {
	CommandHeader
	DefectID     domain.DefectID `json:"defect_id"`
	ChippedGrams int64           `json:"chipped_grams"`
	RepairSet    []domain.UnitID `json:"repair_set"`
}

// RepairView is the public read model of a repair generation.
type RepairView struct {
	ID           domain.RepairGenID `json:"id"`
	Generation   domain.Generation  `json:"generation"`
	ChippedGrams int64              `json:"chipped_grams"`
	Complete     bool               `json:"complete"`
}

// ReviewRequest submits an independent review.
type ReviewRequest struct {
	CommandHeader
	Reviewer   domain.PersonID `json:"reviewer"`
	Qualified  bool            `json:"qualified"`
	Conclusion string          `json:"conclusion"`
}

// ReviewView is the public read model of a review.
type ReviewView struct {
	ID       domain.ReviewID `json:"id"`
	Reviewer domain.PersonID `json:"reviewer"`
}

// DecisionRequest submits a terminal decision.
type DecisionRequest struct {
	CommandHeader
	Kind arbitration.TerminalKind `json:"kind"`
}

// DecisionView is the public read model of the terminal decision.
type DecisionView struct {
	ID   domain.DecisionID        `json:"id"`
	Kind arbitration.TerminalKind `json:"kind"`
}

// LeaseAcquireRequest grants a batch of leases all-or-nothing.
type LeaseAcquireRequest struct {
	OperationID domain.OperationID `json:"operation_id"`
	Leases      []LeaseSpec        `json:"leases"`
}

// LeaseSpec is a requested lease within an acquire batch.
type LeaseSpec struct {
	ID         domain.LeaseID     `json:"id"`
	Resource   mass.ResourceKind  `json:"resource"`
	Holder     string             `json:"holder"`
	Start      domain.LogicalTime `json:"start"`
	End        domain.LogicalTime `json:"end"`
	FenceToken string             `json:"fence_token"`
}

// LeaseAcquireResult reports which leases were granted.
type LeaseAcquireResult struct {
	Granted []domain.LeaseID `json:"granted"`
}

// LeaseRenewRequest extends a lease.
type LeaseRenewRequest struct {
	OperationID domain.OperationID `json:"operation_id"`
	LeaseID     domain.LeaseID     `json:"lease_id"`
	FenceToken  string             `json:"fence_token"`
	NewEnd      domain.LogicalTime `json:"new_end"`
}

// LeaseReleaseRequest releases a lease.
type LeaseReleaseRequest struct {
	OperationID domain.OperationID `json:"operation_id"`
	LeaseID     domain.LeaseID     `json:"lease_id"`
	FenceToken  string             `json:"fence_token"`
}

// DeviceCallRequest records a scripted instrument invocation.
type DeviceCallRequest struct {
	OperationID domain.OperationID  `json:"operation_id"`
	ID          domain.DeviceCallID `json:"id"`
	Device      evidence.DeviceKind `json:"device"`
	Params      string              `json:"params"`
	LogicalTime domain.LogicalTime  `json:"logical_time"`
	FenceToken  string              `json:"fence_token"`
}

// ReceiptRequest applies a scripted instrument result.
type ReceiptRequest struct {
	Fence   string             `json:"fence_token"`
	Attempt int64              `json:"attempt"`
	Fault   evidence.FaultType `json:"fault,omitempty"`
	Value   string             `json:"value,omitempty"`
}

// DeviceCallView is the public read model of a device call.
type DeviceCallView struct {
	ID         domain.DeviceCallID `json:"id"`
	Device     evidence.DeviceKind `json:"device"`
	Status     evidence.CallStatus `json:"status"`
	Attempt    int64               `json:"attempt"`
	RetryAfter domain.LogicalTime  `json:"retry_after"`
	Fault      evidence.FaultType  `json:"fault,omitempty"`
	Receipt    string              `json:"receipt,omitempty"`
}

// AuditView is the full, canonically sorted evidence chain for a cycle.
type AuditView struct {
	CycleID  domain.CycleID                 `json:"cycle_id"`
	Snapshot CycleView                      `json:"snapshot"`
	Pans     []mass.MixPan                  `json:"pans"`
	Evidence []evidence.Evidence            `json:"evidence"`
	Defects  []arbitration.DefectCase       `json:"defects"`
	Repairs  []arbitration.RepairGeneration `json:"repairs"`
	Reviews  []arbitration.Review           `json:"reviews"`
	Decision *arbitration.TerminalDecision  `json:"decision,omitempty"`
	Metrics  map[string]fixedpoint.Q        `json:"metrics,omitempty"`
}
