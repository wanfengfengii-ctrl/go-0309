package httpapi

import (
	"context"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// Service is the stable application contract exposed by the HTTP API. Each
// method corresponds to a documented endpoint and business flow. Concrete
// implementations own persistence, transactions and the recovery startup flow.
type Service interface {
	Health(ctx context.Context) error

	// LockCycle creates and locks an excavation cycle snapshot (POST /v1/cycles).
	LockCycle(ctx context.Context, req LockCycleRequest) (CycleView, error)
	// GetCycle returns a locked cycle (GET /v1/cycles/{id}).
	GetCycle(ctx context.Context, id domain.CycleID) (CycleView, error)
	// GetCoverage returns the canonical-key-sorted coverage view.
	GetCoverage(ctx context.Context, id domain.CycleID) (CoverageView, error)
	// GetAudit exports the full, canonically sorted evidence chain.
	GetAudit(ctx context.Context, id domain.CycleID) (AuditView, error)

	// ConfirmSurface records base-surface and seepage confirmation.
	ConfirmSurface(ctx context.Context, id domain.CycleID, req SurfaceConfirmRequest) (EvidenceView, error)
	// CreateMixPan atomically acquires leases, deducts stock and creates a pan.
	CreateMixPan(ctx context.Context, id domain.CycleID, req MixPanRequest) (PanView, error)
	// AppendSprayBand appends a validated spray band to a unit layer.
	AppendSprayBand(ctx context.Context, id domain.CycleID, req SprayBandRequest) (BandView, error)
	// SealRebound seals rebound mass and records the disposition.
	SealRebound(ctx context.Context, id domain.CycleID, req ReboundSealRequest) (EvidenceView, error)
	// AddCureEvidence appends cure coverage evidence.
	AddCureEvidence(ctx context.Context, id domain.CycleID, req CureEvidenceRequest) (EvidenceView, error)
	// AddTest appends a detection test result.
	AddTest(ctx context.Context, id domain.CycleID, req TestRequest) (EvidenceView, error)
	// CreateDefect records a defect and computes its propagation set.
	CreateDefect(ctx context.Context, id domain.CycleID, req DefectRequest) (DefectView, error)
	// CreateRepair registers chipped mass and a new repair generation.
	CreateRepair(ctx context.Context, id domain.CycleID, req RepairRequest) (RepairView, error)
	// SubmitReview records an independent review.
	SubmitReview(ctx context.Context, id domain.CycleID, req ReviewRequest) (ReviewView, error)
	// SubmitTerminalDecision competes for the single terminal decision.
	SubmitTerminalDecision(ctx context.Context, id domain.CycleID, req DecisionRequest) (DecisionView, error)

	// AcquireLeases grants a batch of leases all-or-nothing.
	AcquireLeases(ctx context.Context, req LeaseAcquireRequest) (LeaseAcquireResult, error)
	// RenewLease extends a lease.
	RenewLease(ctx context.Context, req LeaseRenewRequest) error
	// ReleaseLease releases a lease.
	ReleaseLease(ctx context.Context, req LeaseReleaseRequest) error

	// CreateDeviceCall records a scripted instrument invocation.
	CreateDeviceCall(ctx context.Context, req DeviceCallRequest) (DeviceCallView, error)
	// SubmitReceipt applies a scripted instrument result.
	SubmitReceipt(ctx context.Context, id domain.DeviceCallID, req ReceiptRequest) (DeviceCallView, error)
}

// CommandHeader carries the common idempotency, ordering and generation fields
// required by every construction write command.
type CommandHeader struct {
	OperationID domain.OperationID `json:"operation_id"`
	LogicalTime domain.LogicalTime `json:"logical_time"`
	Generation  domain.Generation  `json:"generation"`
}

// LockCycleRequest is the normalized input for the lock operation.
type LockCycleRequest struct {
	OperationID domain.OperationID `json:"operation_id"`
	Digest      string             `json:"digest"`
	Snapshot    CycleSnapshotDTO   `json:"snapshot"`
}

// CycleSnapshotDTO is the transport representation of a full design snapshot.
// It reuses the typed design structures so the lock gate validates the same
// geometry the service will persist.
type CycleSnapshotDTO struct {
	Tunnel          string                  `json:"tunnel"`
	StartMeter      int64                   `json:"start_meter"`
	EndMeter        int64                   `json:"end_meter"`
	CycleNo         int64                   `json:"cycle_no"`
	DesignThickness int64                   `json:"design_thickness"`
	LayerSequence   []int64                 `json:"layer_sequence"`
	SprayDirection  design.Point            `json:"spray_direction"`
	PoseWindow      design.PoseWindow       `json:"pose_window"`
	Thresholds      design.Thresholds       `json:"thresholds"`
	Mappings        design.DetectionMapping `json:"mappings"`
	RockZones       []design.RockZone       `json:"rock_zones"`
	Units           []design.SurfaceUnit    `json:"units"`
	NoSpray         []design.NoSprayZone    `json:"no_spray"`
	Seepage         []design.SeepagePoint   `json:"seepage"`
	Adjacencies     []design.Adjacency      `json:"adjacencies"`
	Materials       []design.MaterialBatch  `json:"materials"`
}

// CycleView is the public read model of a locked cycle.
type CycleView struct {
	ID         domain.CycleID     `json:"id"`
	Tunnel     string             `json:"tunnel"`
	StartMeter int64              `json:"start_meter"`
	EndMeter   int64              `json:"end_meter"`
	CycleNo    int64              `json:"cycle_no"`
	Digest     string             `json:"digest"`
	LockTime   domain.LogicalTime `json:"lock_time"`
}

// CoverageView is the public read model of a cycle's coverage aggregate.
type CoverageView struct {
	CycleID     domain.CycleID      `json:"cycle_id"`
	Generations []domain.Generation `json:"generations"`
	Units       []string            `json:"units"`
}
