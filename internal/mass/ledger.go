// Package mass implements the component conservation and resource-lease
// manager: an integer-gram double-entry ledger for raw materials, effective
// wall mass, specimens, rebound, wash-out, chipped material and reasonable
// loss, plus atomic pan creation, inventory deduction, key-component
// conservation, idempotent recording and six kinds of time-bounded exclusive
// leases.
package mass

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// DispositionKind enumerates where material from a pan can go. Rebound may only
// be sealed or disposed of and must never re-enter mixing.
type DispositionKind string

// Material dispositions tracked by the ledger.
const (
	DispositionWall     DispositionKind = "wall"     // effective on-wall mass
	DispositionSpecimen DispositionKind = "specimen" // specimens
	DispositionRebound  DispositionKind = "rebound"  // rebound, sealed
	DispositionWashOut  DispositionKind = "wash_out" // wash-out recovery
	DispositionChipped  DispositionKind = "chipped"  // chipped repair material
	DispositionLoss     DispositionKind = "loss"     // reasonable loss
	DispositionInput    DispositionKind = "input"    // raw material input
)

// MixPan is a planned and executed batch with integer-gram inputs and a mix
// ratio summary.
type MixPan struct {
	ID               domain.PanID                  `json:"id"`
	Plan             string                        `json:"plan"`
	PumpWindowStart  domain.LogicalTime            `json:"pump_window_start"`
	PumpWindowEnd    domain.LogicalTime            `json:"pump_window_end"`
	InputGrams       map[design.MaterialKind]int64 `json:"input_grams"`
	SteelFiberGrams  int64                         `json:"steel_fiber_grams"`
	AcceleratorGrams int64                         `json:"accelerator_grams"`
	InventoryVersion int64                         `json:"inventory_version"`
	RatioSummary     design.MixRatio               `json:"ratio_summary"`
}

// MassLedgerEntry is an immutable double-entry record for a pan. Every entry
// has a source, a destination and an immutable serial number.
type MassLedgerEntry struct {
	Serial           int64           `json:"serial"`
	PanID            domain.PanID    `json:"pan_id"`
	Source           DispositionKind `json:"source"`
	Destination      DispositionKind `json:"destination"`
	Grams            int64           `json:"grams"`
	SteelFiberGrams  int64           `json:"steel_fiber_grams"`
	AcceleratorGrams int64           `json:"accelerator_grams"`
}

// Ledger is the append-only sequence of entries for a cycle.
type Ledger struct {
	Entries    []MassLedgerEntry `json:"entries"`
	nextSerial int64
}

// Add appends a new entry with the next immutable serial number.
func (l *Ledger) Add(e MassLedgerEntry) MassLedgerEntry {
	e.Serial = l.nextSerial
	l.nextSerial++
	l.Entries = append(l.Entries, e)
	return e
}

// CheckConservation verifies the total-mass conservation invariant for a pan:
//
//	input = wall + specimen + rebound + wash_out + loss
//
// and separately verifies steel-fiber and accelerator conservation. It returns
// an ordered list of violations (empty means the pan conserves).
func (l *Ledger) CheckConservation(panID domain.PanID, input MixPan) []domain.Reason {
	var reasons []domain.Reason
	var inputGrams, wall, specimen, rebound, washOut, loss int64
	var fiberOut, accelOut int64

	for _, e := range l.Entries {
		if e.PanID != panID {
			continue
		}
		switch e.Destination {
		case DispositionWall:
			wall += e.Grams
		case DispositionSpecimen:
			specimen += e.Grams
		case DispositionRebound:
			rebound += e.Grams
		case DispositionWashOut:
			washOut += e.Grams
		case DispositionLoss:
			loss += e.Grams
		case DispositionInput:
			inputGrams += e.Grams
		}
		fiberOut += e.SteelFiberGrams
		accelOut += e.AcceleratorGrams
	}

	if inputGrams != 0 && inputGrams != wall+specimen+rebound+washOut+loss {
		reasons = append(reasons, domain.Reason{
			Code:    domain.CodeMassConflict,
			Message: "total mass not conserved",
			Key:     string(panID),
		})
	}
	if fiberOut != input.SteelFiberGrams {
		reasons = append(reasons, domain.Reason{
			Code:    domain.CodeMassConflict,
			Message: "steel fiber not conserved",
			Key:     string(panID),
		})
	}
	if accelOut != input.AcceleratorGrams {
		reasons = append(reasons, domain.Reason{
			Code:    domain.CodeMassConflict,
			Message: "accelerator not conserved",
			Key:     string(panID),
		})
	}
	return reasons
}

// RejectReboundReuse verifies that rebound material never re-enters mixing.
func (l *Ledger) RejectReboundReuse(panID domain.PanID) *domain.Error {
	for _, e := range l.Entries {
		if e.PanID == panID && e.Source == DispositionRebound && e.Destination == DispositionInput {
			return domain.NewError(domain.CodeMassConflict, "rebound material may not be reused")
		}
	}
	return nil
}
