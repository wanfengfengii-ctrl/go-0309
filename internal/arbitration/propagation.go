package arbitration

import (
	"sort"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// PropagationInput describes the relationships used to spread a defect from its
// seed unit. Units are related when they are adjacent on the surface, share a
// pump window, were sprayed with the same material pan, or belong to the same
// surrounding-rock looseness zone.
type PropagationInput struct {
	Seed         domain.UnitID
	Adjacency    map[domain.UnitID][]domain.UnitID
	ZoneOf       map[domain.UnitID]domain.Identifier
	PanOf        map[domain.UnitID]domain.PanID
	PumpWindowOf map[domain.UnitID]string
}

// Propagate computes the deterministic closure of units related to the seed.
// The result is deduplicated and canonically sorted, so the same seed evidence
// always yields the identical repair set regardless of map iteration order.
func Propagate(in PropagationInput) []domain.UnitID {
	// Build the reverse indexes once so the BFS is order-independent.
	zoneIndex := map[domain.Identifier][]domain.UnitID{}
	panIndex := map[domain.PanID][]domain.UnitID{}
	windowIndex := map[string][]domain.UnitID{}
	for u, z := range in.ZoneOf {
		zoneIndex[z] = append(zoneIndex[z], u)
	}
	for u, p := range in.PanOf {
		panIndex[p] = append(panIndex[p], u)
	}
	for u, w := range in.PumpWindowOf {
		windowIndex[w] = append(windowIndex[w], u)
	}
	for _, list := range zoneIndex {
		sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	}
	for _, list := range panIndex {
		sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	}
	for _, list := range windowIndex {
		sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	}

	visited := map[domain.UnitID]bool{in.Seed: true}
	queue := []domain.UnitID{in.Seed}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		neighbors := related(cur, in, zoneIndex, panIndex, windowIndex)
		for _, n := range neighbors {
			if !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}

	out := make([]domain.UnitID, 0, len(visited))
	for u := range visited {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// related returns the distinct, sorted units directly related to u.
func related(
	u domain.UnitID,
	in PropagationInput,
	zoneIndex map[domain.Identifier][]domain.UnitID,
	panIndex map[domain.PanID][]domain.UnitID,
	windowIndex map[string][]domain.UnitID,
) []domain.UnitID {
	seen := map[domain.UnitID]bool{}
	add := func(list []domain.UnitID) {
		for _, v := range list {
			if v != u {
				seen[v] = true
			}
		}
	}
	add(in.Adjacency[u])
	add(zoneIndex[in.ZoneOf[u]])
	add(panIndex[in.PanOf[u]])
	add(windowIndex[in.PumpWindowOf[u]])
	out := make([]domain.UnitID, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
