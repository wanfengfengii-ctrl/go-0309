// Package evidence implements the spray-trajectory and cure-evidence recorder:
// append-only pumping, accelerator linkage, nozzle pose, rebound sealing, cure,
// scan and test instrument evidence, together with a device-call state machine
// carrying logical time, a deterministic retry count and scripted results.
package evidence

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/fixedpoint"
)

// DeviceKind enumerates the scripted instrument classes.
type DeviceKind string

// Instrument device kinds.
const (
	DeviceScale      DeviceKind = "scale"
	DeviceFlowMeter  DeviceKind = "flow_meter"
	DevicePoseSensor DeviceKind = "pose_sensor"
	DeviceThickness  DeviceKind = "thickness_scanner"
	DevicePress      DeviceKind = "pressure_press"
	DevicePull       DeviceKind = "pull_tester"
)

// CallStatus is the stable state of a device call.
type CallStatus string

// Device call states.
const (
	CallPending   CallStatus = "pending"
	CallRetrying  CallStatus = "retrying"
	CallSucceeded CallStatus = "succeeded"
	CallFailed    CallStatus = "failed"
)

// FaultType is a stable device failure classification.
type FaultType string

// Stable device faults.
const (
	FaultRejected   FaultType = "rejected"
	FaultTimeout    FaultType = "timeout"
	FaultDisconnect FaultType = "disconnect"
	FaultBadFormat  FaultType = "bad_format"
)

// DeviceCall is a scripted instrument invocation. Rejection, disconnect,
// timeout or bad format only create a retryable call; a valid reading is
// required to advance any prefix or lift a risk barrier.
type DeviceCall struct {
	ID          domain.DeviceCallID `json:"id"`
	Device      DeviceKind          `json:"device"`
	Params      string              `json:"params"` // normalized parameter digest
	LogicalTime domain.LogicalTime  `json:"logical_time"`
	Attempt     int64               `json:"attempt"`
	RetryAfter  domain.LogicalTime  `json:"retry_after"` // deterministic backoff deadline
	FenceToken  string              `json:"fence_token"`
	Status      CallStatus          `json:"status"`
	Fault       FaultType           `json:"fault,omitempty"`
	Receipt     string              `json:"receipt,omitempty"`
}

// Retry advances the call to the next attempt with a deterministic backoff
// deadline, returning the updated attempt number and deadline.
func (c *DeviceCall) Retry(now domain.LogicalTime, backoff domain.LogicalTime) {
	c.Attempt++
	c.RetryAfter = now + backoff
	c.Status = CallRetrying
}

// Succeed marks the call succeeded and stores the receipt.
func (c *DeviceCall) Succeed(receipt string) {
	c.Status = CallSucceeded
	c.Receipt = receipt
}

// EvidenceKind enumerates the recorded evidence types.
type EvidenceKind string

// Evidence types.
const (
	EvidenceSurfaceConfirm   EvidenceKind = "surface_confirm"
	EvidenceSeepageConfirm   EvidenceKind = "seepage_confirm"
	EvidenceNozzleTrajectory EvidenceKind = "nozzle_trajectory"
	EvidenceReboundSeal      EvidenceKind = "rebound_seal"
	EvidenceCureCoverage     EvidenceKind = "cure_coverage"
	EvidenceThicknessScan    EvidenceKind = "thickness_scan"
	EvidenceProbe            EvidenceKind = "probe"
	EvidenceCoreSample       EvidenceKind = "core_sample"
	EvidencePlateSpecimen    EvidenceKind = "plate_specimen"
	EvidencePressure         EvidenceKind = "pressure"
	EvidencePull             EvidenceKind = "pull"
)

// CanonicalKey reports the domain sort key for an evidence record so that audit
// views order diagnostics uniformly.
func (e Evidence) CanonicalKey() domain.CanonicalKey {
	return domain.CanonicalKey{
		Unit:       string(e.UnitID),
		Layer:      e.Layer,
		Band:       string(e.BandID),
		Generation: int64(e.Generation),
	}
}

// Evidence is a single appended record linked to unit, layer, band, pan and
// generation.
type Evidence struct {
	ID          domain.EvidenceID  `json:"id"`
	Kind        EvidenceKind       `json:"kind"`
	LogicalTime domain.LogicalTime `json:"logical_time"`
	UnitID      domain.UnitID      `json:"unit_id,omitempty"`
	Layer       int64              `json:"layer,omitempty"`
	BandID      domain.BandID      `json:"band_id,omitempty"`
	PanID       domain.PanID       `json:"pan_id,omitempty"`
	Generation  domain.Generation  `json:"generation,omitempty"`
	Value       fixedpoint.Q       `json:"value,omitempty"` // measured metric
	FenceToken  string             `json:"fence_token"`
}
