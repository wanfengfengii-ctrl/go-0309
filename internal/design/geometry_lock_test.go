package design

import (
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

func square(x, y, w, h int64) Polygon {
	return Polygon{
		{X: x, Y: y},
		{X: x + w, Y: y},
		{X: x + w, Y: y + h},
		{X: x, Y: y + h},
	}
}

func TestValidateGridAcceptCompleteCoverage(t *testing.T) {
	units := []SurfaceUnit{
		{ID: "u1", Zone: "z1", Polygon: square(0, 0, 100, 100)},
		{ID: "u2", Zone: "z1", Polygon: square(100, 0, 100, 100)},
	}
	reasons := ValidateGrid(units, nil)
	if len(reasons) != 0 {
		t.Fatalf("expected valid grid, got %v", reasons)
	}
}

func TestValidateGridRejectsOverlap(t *testing.T) {
	units := []SurfaceUnit{
		{ID: "u1", Zone: "z1", Polygon: square(0, 0, 100, 100)},
		{ID: "u2", Zone: "z1", Polygon: square(50, 0, 100, 100)},
	}
	reasons := ValidateGrid(units, nil)
	if len(reasons) == 0 {
		t.Fatal("expected overlap to be rejected")
	}
}

func TestValidateGridRejectsNoSprayIntrusion(t *testing.T) {
	units := []SurfaceUnit{
		{ID: "u1", Zone: "z1", Polygon: square(0, 0, 100, 100)},
	}
	zones := []NoSprayZone{
		{ID: "nz1", Polygon: square(90, 90, 20, 20)},
	}
	reasons := ValidateGrid(units, zones)
	found := false
	for _, r := range reasons {
		if r.Code == domain.CodeNoSprayIntrusion {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected NO_SPRAY_INTRUSION reason, got %v", reasons)
	}
}

func TestValidateGridRejectsDegeneratePolygon(t *testing.T) {
	units := []SurfaceUnit{
		{ID: "u1", Zone: "z1", Polygon: Polygon{{X: 0, Y: 0}, {X: 1, Y: 1}}},
	}
	reasons := ValidateGrid(units, nil)
	if len(reasons) == 0 {
		t.Fatal("expected degenerate polygon to be rejected")
	}
}

func TestSignedAreaRejectsOverflow(t *testing.T) {
	big := int64(1 << 40)
	p := Polygon{
		{X: big, Y: big},
		{X: big, Y: -big},
		{X: -big, Y: -big},
	}
	if _, err := SignedArea(p); err == nil {
		t.Fatal("expected overflow error")
	}
}
