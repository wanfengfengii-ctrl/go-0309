// Package arbitration implements the defect-repair and terminal arbiter:
// deterministic defect propagation sets and repair generations, isolation of
// late events from old generations, validation of two-person review and all
// closure conditions, and a single-writer barrier that adjudicates closure,
// risk isolation or cancellation.
package arbitration

import (
	"sort"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// DefectType enumerates the seven defect classes that trigger propagation.
type DefectType string

// The seven defect classes.
const (
	DefectUnderThickness   DefectType = "under_thickness"
	DefectVoid             DefectType = "void"
	DefectFiberSegregation DefectType = "fiber_segregation"
	DefectReboundExcess    DefectType = "rebound_excess"
	DefectColdJoint        DefectType = "cold_joint"
	DefectSeepageErosion   DefectType = "seepage_erosion"
	DefectEarlyStrength    DefectType = "early_strength"
)

// DefectCase is a detected defect with its seed evidence and the computed,
// canonically ordered repair set.
type DefectCase struct {
	ID           domain.DefectID   `json:"id"`
	Type         DefectType        `json:"type"`
	SeedEvidence domain.EvidenceID `json:"seed_evidence"`
	RuleVersion  int64             `json:"rule_version"`
	RepairSet    []domain.UnitID   `json:"repair_set"` // unique, sorted canonical keys
	Status       string            `json:"status"`
}

// RepairGeneration is the new generation established by a repair. The old
// generation records are immutable and permanently retained.
type RepairGeneration struct {
	ID           domain.RepairGenID                      `json:"id"`
	Generation   domain.Generation                       `json:"generation"`
	ChippedGrams int64                                   `json:"chipped_grams"`
	RepairSet    []domain.UnitID                         `json:"repair_set"`
	RecheckMap   map[domain.Identifier]domain.EvidenceID `json:"recheck_map"`
	Complete     bool                                    `json:"complete"`
}

// Review is an independent two-person review submission.
type Review struct {
	ID         domain.ReviewID `json:"id"`
	Reviewer   domain.PersonID `json:"reviewer"`
	Qualified  bool            `json:"qualified"`
	Conclusion string          `json:"conclusion"` // approve / reject
	Digest     string          `json:"digest"`
}

// TerminalKind is one of the three terminal outcomes.
type TerminalKind string

// Terminal outcomes.
const (
	TerminalClosure TerminalKind = "closure"
	TerminalIsolate TerminalKind = "risk_isolation"
	TerminalCancel  TerminalKind = "cancel"
)

// TerminalDecision is the single immutable terminal outcome for a cycle.
type TerminalDecision struct {
	ID          domain.DecisionID  `json:"id"`
	Kind        TerminalKind       `json:"kind"`
	Digest      string             `json:"digest"`
	LogicalTime domain.LogicalTime `json:"logical_time"`
}

// DecisionBarrier is the single-writer interface guarding the terminal
// decision. Exactly one caller wins; concurrent losers observe the same
// existing decision.
type DecisionBarrier interface {
	// Decide atomically writes the terminal decision if none exists, returning
	// TERMINAL_ALREADY_SET with the existing decision on conflict.
	Decide(cycleID domain.CycleID, d TerminalDecision) (TerminalDecision, error)
	// Current returns the existing terminal decision, if any.
	Current(cycleID domain.CycleID) (TerminalDecision, bool, error)
}

// UniqueRepairSet canonicalizes a candidate repair unit set: deduplicates by
// canonical key and sorts, so that the same seed evidence always yields the
// same set.
func UniqueRepairSet(units []domain.UnitID) []domain.UnitID {
	seen := make(map[domain.UnitID]bool, len(units))
	out := make([]domain.UnitID, 0, len(units))
	for _, u := range units {
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ValidateReviews confirms that two distinct qualified reviewers approved.
func ValidateReviews(reviews []Review) *domain.Error {
	seen := make(map[domain.PersonID]bool, len(reviews))
	approved := 0
	for _, r := range reviews {
		if !r.Qualified {
			continue
		}
		if seen[r.Reviewer] {
			return domain.NewError(domain.CodeInvalidRequest, "reviewer must be distinct")
		}
		seen[r.Reviewer] = true
		if r.Conclusion == "approve" {
			approved++
		}
	}
	if approved < 2 {
		return domain.NewError(domain.CodeInvalidRequest, "two independent approvals required")
	}
	return nil
}
