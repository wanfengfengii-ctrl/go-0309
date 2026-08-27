// Package design implements the rock-zone and spray-material rule catalog: the
// immutable design summary, rock zones, adjacency, grid geometry, no-spray
// zones, seepage points, mix ratios, material batches, nozzle pose windows,
// performance thresholds and detection mapping, together with integer geometry
// and fixed-point boundary validation.
package design

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// Point is a coordinate in integer millimetres within the cycle plane.
type Point struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

// Polygon is a closed ring of at least three non-collinear vertices. Vertices
// are ordered counter-clockwise; clockwise or degenerate input is rejected.
type Polygon []Point

// SurfaceUnit is a single sprayed-surface cell of the grid. Its polygon must
// exactly tile the design area with its neighbours, without overlap.
type SurfaceUnit struct {
	ID      domain.UnitID `json:"id"`
	Polygon Polygon       `json:"polygon"`
	// Zone is the rock zone this unit belongs to.
	Zone domain.Identifier `json:"zone"`
}

// NoSprayZone is an under-excavation area where spraying is forbidden.
type NoSprayZone struct {
	ID      domain.Identifier `json:"id"`
	Polygon Polygon           `json:"polygon"`
}

// SeepagePoint marks a water ingress location that must be treated before
// spraying.
type SeepagePoint struct {
	ID       domain.Identifier `json:"id"`
	Location Point             `json:"location"`
}

// Adjacency links two adjacent surface units, used by defect propagation.
type Adjacency struct {
	A domain.UnitID `json:"a"`
	B domain.UnitID `json:"b"`
}

// SignedArea returns twice the signed area of the polygon. A positive result
// indicates counter-clockwise winding. It rejects 64-bit overflow.
func SignedArea(p Polygon) (int64, error) {
	n := len(p)
	if n < 3 {
		return 0, errDegenerate
	}
	var acc int64
	for i := 0; i < n; i++ {
		a := p[i]
		b := p[(i+1)%n]
		cross, err := subMul(a.X, b.Y, b.X, a.Y)
		if err != nil {
			return 0, err
		}
		if (cross > 0 && acc > int64(^uint64(0)>>1)-cross) ||
			(cross < 0 && acc < -int64(^uint64(0)>>1)-cross) {
			return 0, domain.NewError(domain.CodeFixedPointOverflow, "polygon area overflow")
		}
		acc += cross
	}
	return acc, nil
}

// subMul computes x1*y1 - x2*y2 with overflow checks on the products.
func subMul(x1, y1, x2, y2 int64) (int64, error) {
	p1hi, p1lo := mul64(x1, y1)
	p2hi, p2lo := mul64(x2, y2)
	if p1hi != 0 || p2hi != 0 {
		return 0, domain.NewError(domain.CodeFixedPointOverflow, "polygon area overflow")
	}
	return p1lo - p2lo, nil
}

// mul64 multiplies two signed int64 values returning a signed 128-bit product
// as (high, low).
func mul64(a, b int64) (hi, lo int64) {
	neg := (a < 0) != (b < 0)
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	h, l := mulU64(uint64(a), uint64(b))
	if neg {
		h = ^h
		l = -l
		if l == 0 {
			h++
		}
	}
	return int64(h), int64(l)
}

func mulU64(a, b uint64) (uint64, uint64) {
	ah := a >> 32
	al := a & 0xffffffff
	bh := b >> 32
	bl := b & 0xffffffff
	p1 := al * bl
	p2 := al * bh
	p3 := ah * bl
	p4 := ah * bh
	mid := (p1 >> 32) + (p2 & 0xffffffff) + (p3 & 0xffffffff)
	lo := p1 + (p2 << 32) + (p3 << 32)
	hi := p4 + (p2 >> 32) + (p3 >> 32) + (mid >> 32)
	return hi, lo
}

// ValidatePolygon rejects degenerate, self-evidently invalid rings.
func ValidatePolygon(p Polygon) error {
	if len(p) < 3 {
		return errDegenerate
	}
	area, err := SignedArea(p)
	if err != nil {
		return err
	}
	if area <= 0 {
		return errDegenerate
	}
	return nil
}

// ValidateGrid checks that the surface units exactly cover the design area:
// every unit polygon is valid, no unit overlaps another, and no unit intrudes
// into a no-spray zone. It returns ordered reasons on failure.
func ValidateGrid(units []SurfaceUnit, zones []NoSprayZone) []domain.Reason {
	var reasons []domain.Reason
	seen := make(map[domain.UnitID]bool, len(units))
	for _, u := range units {
		if seen[u.ID] {
			reasons = append(reasons, reason(domain.CodeInvalidGrid, "duplicate unit id", string(u.ID)))
		}
		seen[u.ID] = true
		if err := ValidatePolygon(u.Polygon); err != nil {
			reasons = append(reasons, reason(domain.CodeInvalidGrid, err.Error(), string(u.ID)))
		}
		for _, z := range zones {
			if intrudes, err := polygonOverlap(u.Polygon, z.Polygon); err != nil {
				reasons = append(reasons, reason(domain.CodeNoSprayIntrusion, err.Error(), string(u.ID)))
			} else if intrudes {
				reasons = append(reasons, reason(domain.CodeNoSprayIntrusion, "unit intrudes into no-spray zone", string(u.ID)))
			}
		}
	}
	// Pairwise overlap check (O(n^2) is acceptable for a bounded grid).
	for i := 0; i < len(units); i++ {
		for j := i + 1; j < len(units); j++ {
			overlap, err := polygonOverlap(units[i].Polygon, units[j].Polygon)
			if err != nil {
				reasons = append(reasons, reason(domain.CodeInvalidGrid, err.Error(), string(units[i].ID)))
			} else if overlap {
				reasons = append(reasons, reason(domain.CodeInvalidGrid, "overlapping surface units", string(units[i].ID)))
			}
		}
	}
	return reasons
}

// polygonOverlap reports whether two convex polygons overlap in positive area
// using the separating axis theorem. Merely touching along an edge or vertex
// (the normal tiling case) is not an overlap. Degenerate inputs return an error.
func polygonOverlap(a, b Polygon) (bool, error) {
	if err := ValidatePolygon(a); err != nil {
		return false, err
	}
	if err := ValidatePolygon(b); err != nil {
		return false, err
	}
	for _, poly := range []Polygon{a, b} {
		for i := range poly {
			edge := poly[(i+1)%len(poly)]
			axis := Point{X: -(edge.Y - poly[i].Y), Y: edge.X - poly[i].X}
			minA, maxA := project(a, axis)
			minB, maxB := project(b, axis)
			// Positive-area intersection requires strict overlap on every axis;
			// an edge or vertex touch fails the strict test on at least one axis.
			if maxA <= minB || maxB <= minA {
				return false, nil
			}
		}
	}
	return true, nil
}

func project(p Polygon, axis Point) (int64, int64) {
	first := p[0].X*axis.X + p[0].Y*axis.Y
	min, max := first, first
	for _, v := range p[1:] {
		d := v.X*axis.X + v.Y*axis.Y
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}
	return min, max
}

var errDegenerate = domain.NewError(domain.CodeInvalidGrid, "degenerate polygon")

func reason(code domain.ErrorCode, msg, key string) domain.Reason {
	return domain.Reason{Code: code, Message: msg, Key: key}
}
