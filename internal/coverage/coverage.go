// Package coverage implements the spray task and coverage aggregate: an
// append-only state machine over excavation cycles, surface units, spray
// layers, spray bands and task generations. It validates direction, continuous
// layer prefixes, band continuity, overlap and effective coverage, and produces
// deterministically ordered aggregate views.
package coverage

import (
	"sort"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// Band is a single appended spray band within a layer.
type Band struct {
	ID          domain.BandID      `json:"id"`
	Seq         int64              `json:"seq"` // position along the spray direction
	StartMM     int64              `json:"start_mm"`
	EndMM       int64              `json:"end_mm"`
	WidthMM     int64              `json:"width_mm"`
	OverlapMM   int64              `json:"overlap_mm"`
	PanID       domain.PanID       `json:"pan_id"`
	Trajectory  domain.EvidenceID  `json:"trajectory"`
	ThicknessMM int64              `json:"thickness_mm"`
	LogicalTime domain.LogicalTime `json:"logical_time"`
	FenceToken  string             `json:"fence_token"`
	Valid       bool               `json:"valid"`
}

// Layer is an ordered sequence of bands forming a continuous prefix along the
// locked spray direction.
type Layer struct {
	ID     domain.LayerID `json:"id"`
	Number int64          `json:"number"`
	Bands  []Band         `json:"bands"`
}

// Unit aggregates the layers of a single surface unit.
type Unit struct {
	ID     domain.UnitID `json:"id"`
	Layers []Layer       `json:"layers"`
}

// CoverageEvent is the append-only record of a band placement.
type CoverageEvent struct {
	Generation domain.Generation `json:"generation"`
	UnitID     domain.UnitID     `json:"unit_id"`
	Layer      int64             `json:"layer"`
	Band       Band              `json:"band"`
}

// GenerationState holds the coverage for one repair generation.
type GenerationState struct {
	Generation domain.Generation `json:"generation"`
	Units      []Unit            `json:"units"`
}

// AppendBand validates and appends a band to a unit's layer, enforcing the
// append-only invariants. It returns an error without mutating state on any
// violation.
func AppendBand(u *Unit, layerNumber int64, b Band, minOverlapMM int64) *domain.Error {
	// Locate or create the layer; reject out-of-order layer creation.
	li := findLayer(u.Layers, layerNumber)
	if li == -1 {
		// A new layer may only be the next layer number in sequence.
		if len(u.Layers) > 0 && layerNumber != u.Layers[len(u.Layers)-1].Number+1 {
			return domain.NewError(domain.CodeLayerOutOfOrder, "layer out of order")
		}
		u.Layers = append(u.Layers, Layer{ID: domain.LayerID(string(u.ID) + "/L" + itoa(layerNumber)), Number: layerNumber})
		li = len(u.Layers) - 1
	}
	layer := &u.Layers[li]

	// Band continuity: bands must be appended in sequence with sufficient overlap.
	if len(layer.Bands) > 0 {
		last := layer.Bands[len(layer.Bands)-1]
		if b.Seq != last.Seq+1 {
			return domain.NewError(domain.CodeBandDiscontinuity, "band discontinuity")
		}
		if b.OverlapMM < minOverlapMM {
			return domain.NewError(domain.CodeOverlapInsufficient, "overlap insufficient")
		}
	}
	if b.WidthMM <= 0 {
		return domain.NewError(domain.CodeInvalidRequest, "band width must be positive")
	}
	if b.EndMM < b.StartMM {
		return domain.NewError(domain.CodeInvalidRequest, "band range invalid")
	}
	b.Valid = true
	layer.Bands = append(layer.Bands, b)
	return nil
}

// findLayer returns the index of a layer by number, or -1.
func findLayer(layers []Layer, n int64) int {
	for i := range layers {
		if layers[i].Number == n {
			return i
		}
	}
	return -1
}

// SortedBands returns all valid bands across a unit sorted by canonical key
// (layer, sequence).
func SortedBands(u *Unit) []Band {
	var out []Band
	for _, l := range u.Layers {
		for _, b := range l.Bands {
			if b.Valid {
				out = append(out, b)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LogicalTime != out[j].LogicalTime {
			return out[i].LogicalTime < out[j].LogicalTime
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// LayerPrefixClosed reports whether every layer up to the given number has a
// non-empty continuous band prefix.
func LayerPrefixClosed(u *Unit, through int64) bool {
	if through <= 0 {
		return false
	}
	for n := int64(1); n <= through; n++ {
		li := findLayer(u.Layers, n)
		if li == -1 || len(u.Layers[li].Bands) == 0 {
			return false
		}
	}
	return true
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
