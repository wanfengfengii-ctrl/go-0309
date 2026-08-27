package arbitration

import (
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

func TestPropagateAdjacencyClosure(t *testing.T) {
	in := PropagationInput{
		Seed: "u1",
		Adjacency: map[domain.UnitID][]domain.UnitID{
			"u1": {"u2"},
			"u2": {"u1", "u3"},
			"u3": {"u2"},
		},
		ZoneOf:       map[domain.UnitID]domain.Identifier{},
		PanOf:        map[domain.UnitID]domain.PanID{},
		PumpWindowOf: map[domain.UnitID]string{},
	}
	got := Propagate(in)
	want := []domain.UnitID{"u1", "u2", "u3"}
	assertUnits(t, got, want)
}

func TestPropagateSharedZoneAndPan(t *testing.T) {
	in := PropagationInput{
		Seed: "u1",
		Adjacency: map[domain.UnitID][]domain.UnitID{
			"u1": {"u2"},
		},
		ZoneOf: map[domain.UnitID]domain.Identifier{
			"u1": "z1", "u2": "z1", "u3": "z1",
		},
		PanOf: map[domain.UnitID]domain.PanID{
			"u1": "p1", "u2": "p1", "u3": "p2", "u4": "p1",
		},
		PumpWindowOf: map[domain.UnitID]string{},
	}
	got := Propagate(in)
	// u1 is adjacent to u2, shares zone z1 with u2,u3 and pan p1 with u2,u4.
	want := []domain.UnitID{"u1", "u2", "u3", "u4"}
	assertUnits(t, got, want)
}

func TestPropagateIsDeterministic(t *testing.T) {
	in := PropagationInput{
		Seed: "u1",
		Adjacency: map[domain.UnitID][]domain.UnitID{
			"u1": {"u2", "u3"}, "u2": {"u1"}, "u3": {"u1", "u4"}, "u4": {"u3"},
		},
		ZoneOf: map[domain.UnitID]domain.Identifier{
			"u1": "z1", "u2": "z1", "u3": "z2", "u4": "z2",
		},
		PanOf:        map[domain.UnitID]domain.PanID{},
		PumpWindowOf: map[domain.UnitID]string{},
	}
	first := Propagate(in)
	for i := 0; i < 50; i++ {
		got := Propagate(in)
		assertUnits(t, got, first)
	}
}

func TestUniqueRepairSetDeduplicatesAndSorts(t *testing.T) {
	got := UniqueRepairSet([]domain.UnitID{"u3", "u1", "u3", "u2"})
	assertUnits(t, got, []domain.UnitID{"u1", "u2", "u3"})
}

func assertUnits(t *testing.T, got, want []domain.UnitID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
