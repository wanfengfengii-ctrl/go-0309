package store

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/coverage"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
)

// CycleState is the full append-only aggregate for a single locked cycle. It is
// persisted as one JSON document so that a cycle's snapshot, pans, ledger,
// coverage, evidence, defects, repairs, reviews and terminal decision are always
// written and recovered together.
type CycleState struct {
	Snapshot design.DesignSnapshot `json:"snapshot"`

	SurfaceConfirmed bool `json:"surface_confirmed"`
	SeepageConfirmed bool `json:"seepage_confirmed"`

	Pans   []mass.MixPan       `json:"pans"`
	Ledger mass.Ledger         `json:"ledger"`
	Cov    coverage.State      `json:"coverage"`
	Evid   []evidence.Evidence `json:"evidence"`

	Defects  []arbitration.DefectCase       `json:"defects"`
	Repairs  []arbitration.RepairGeneration `json:"repairs"`
	Reviews  []arbitration.Review           `json:"reviews"`
	Decision *arbitration.TerminalDecision  `json:"decision,omitempty"`

	CurrentGeneration domain.Generation `json:"current_generation"`
}

// PutCycle persists a cycle state atomically within the current transaction.
func (t *Tx) PutCycle(id domain.CycleID, st *CycleState) error {
	return t.PutJSON(bucketCycles, []byte(id), st)
}

// GetCycle loads a cycle state. It returns (nil, nil) when the cycle is absent.
func (t *Tx) GetCycle(id domain.CycleID) (*CycleState, error) {
	var st CycleState
	ok, err := t.GetJSON(bucketCycles, []byte(id), &st)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return &st, nil
}

// CycleIDs returns every persisted cycle id in ascending key order.
func (t *Tx) CycleIDs() ([]domain.CycleID, error) {
	var ids []domain.CycleID
	err := t.ForEach(bucketCycles, func(k, _ []byte) error {
		ids = append(ids, domain.CycleID(string(k)))
		return nil
	})
	return ids, err
}
