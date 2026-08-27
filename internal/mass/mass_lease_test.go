package mass

import (
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

func TestLedgerTotalMassConservation(t *testing.T) {
	panID := domain.PanID("pan-1")
	input := MixPan{
		ID:               panID,
		InputGrams:       map[design.MaterialKind]int64{design.MaterialCement: 1000},
		SteelFiberGrams:  50,
		AcceleratorGrams: 10,
	}
	l := &Ledger{}
	l.Add(MassLedgerEntry{PanID: panID, Source: DispositionInput, Destination: DispositionInput, Grams: 1000})
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionWall, Grams: 800, SteelFiberGrams: 40, AcceleratorGrams: 8})
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionSpecimen, Grams: 50, SteelFiberGrams: 5, AcceleratorGrams: 1})
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionRebound, Grams: 100, SteelFiberGrams: 5, AcceleratorGrams: 1})
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionWashOut, Grams: 30})
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionLoss, Grams: 20})
	if reasons := l.CheckConservation(panID, input); len(reasons) != 0 {
		t.Fatalf("expected conservation, got %v", reasons)
	}
}

func TestLedgerRejectsMassImbalance(t *testing.T) {
	panID := domain.PanID("pan-1")
	input := MixPan{ID: panID, SteelFiberGrams: 50, AcceleratorGrams: 10}
	l := &Ledger{}
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionInput, Grams: 1000})
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionWall, Grams: 800, SteelFiberGrams: 50, AcceleratorGrams: 10})
	reasons := l.CheckConservation(panID, input)
	if len(reasons) == 0 {
		t.Fatal("expected mass imbalance to be reported")
	}
}

func TestLedgerRejectsSteelFiberImbalance(t *testing.T) {
	panID := domain.PanID("pan-1")
	input := MixPan{ID: panID, SteelFiberGrams: 50, AcceleratorGrams: 10}
	l := &Ledger{}
	l.Add(MassLedgerEntry{PanID: panID, Destination: DispositionWall, Grams: 1000, SteelFiberGrams: 30, AcceleratorGrams: 10})
	reasons := l.CheckConservation(panID, input)
	found := false
	for _, r := range reasons {
		if r.Code == domain.CodeMassConflict && r.Message == "steel fiber not conserved" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected steel fiber imbalance, got %v", reasons)
	}
}

func TestLedgerRejectsReboundReuse(t *testing.T) {
	panID := domain.PanID("pan-1")
	l := &Ledger{}
	l.Add(MassLedgerEntry{PanID: panID, Source: DispositionRebound, Destination: DispositionInput, Grams: 10})
	if err := l.RejectReboundReuse(panID); err == nil || err.Code != domain.CodeMassConflict {
		t.Fatalf("expected MASS_CONFLICT, got %v", err)
	}
}

func TestLeaseActiveBounds(t *testing.T) {
	l := ResourceLease{Start: 10, End: 20}
	if l.Active(9) || !l.Active(10) || !l.Active(19) || l.Active(20) {
		t.Fatal("lease active bounds incorrect")
	}
}
