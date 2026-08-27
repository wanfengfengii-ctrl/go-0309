// Package domain holds the stable value types, identifiers, error codes and
// operation records shared by every business package of the shotcrete closure
// service. It intentionally contains no persistence or transport concerns.
package domain

// Identifier is a string key used for all aggregate roots. Each value is an
// opaque, orderable identifier produced by the caller and treated as an atomic
// canonical key throughout the system.
type Identifier string

// Named identifier aliases document the distinct aggregate roots of the model.
type (
	// CycleID identifies a locked excavation cycle snapshot.
	CycleID = Identifier
	// UnitID identifies a sprayed-surface unit within a cycle.
	UnitID = Identifier
	// LayerID identifies a spray layer within a unit.
	LayerID = Identifier
	// BandID identifies a spray band within a layer.
	BandID = Identifier
	// PanID identifies a mix pan.
	PanID = Identifier
	// LeaseID identifies a resource lease.
	LeaseID = Identifier
	// DeviceCallID identifies a scripted device call.
	DeviceCallID = Identifier
	// EvidenceID identifies a piece of evidence.
	EvidenceID = Identifier
	// DefectID identifies a defect case.
	DefectID = Identifier
	// RepairGenID identifies a repair generation.
	RepairGenID = Identifier
	// ReviewID identifies an independent review.
	ReviewID = Identifier
	// DecisionID identifies the terminal decision.
	DecisionID = Identifier
	// OperationID identifies an idempotent write command.
	OperationID = Identifier
)

// LogicalTime is a monotonically increasing logical clock value used to order
// evidence and to bound lease validity. It has no wall-clock meaning.
type LogicalTime int64

// PersonID identifies a qualified reviewer or operator.
type PersonID string

// Generation is a repair generation number; generation zero is the original
// placement, and every repair establishes a strictly higher generation.
type Generation int64
