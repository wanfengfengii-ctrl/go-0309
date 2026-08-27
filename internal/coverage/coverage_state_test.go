package coverage

import (
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

func band(seq, start, end, width, overlap int64) Band {
	return Band{
		ID: domain.BandID("b" + itoa(seq)), Seq: seq,
		StartMM: start, EndMM: end, WidthMM: width, OverlapMM: overlap,
	}
}

func TestAppendBandFormsLayerPrefix(t *testing.T) {
	u := &Unit{ID: "u1"}
	if err := AppendBand(u, 1, band(1, 0, 100, 50, 0), 10); err != nil {
		t.Fatalf("append band 1: %v", err)
	}
	if err := AppendBand(u, 1, band(2, 90, 190, 50, 20), 10); err != nil {
		t.Fatalf("append band 2: %v", err)
	}
	if !LayerPrefixClosed(u, 1) {
		t.Fatal("expected layer 1 prefix closed")
	}
}

func TestAppendBandRejectsOutOfOrderLayer(t *testing.T) {
	u := &Unit{ID: "u1"}
	_ = AppendBand(u, 1, band(1, 0, 100, 50, 0), 10)
	err := AppendBand(u, 3, band(1, 0, 100, 50, 0), 10)
	if err == nil || err.Code != domain.CodeLayerOutOfOrder {
		t.Fatalf("expected LAYER_OUT_OF_ORDER, got %v", err)
	}
}

func TestAppendBandRejectsDiscontinuity(t *testing.T) {
	u := &Unit{ID: "u1"}
	_ = AppendBand(u, 1, band(1, 0, 100, 50, 0), 10)
	err := AppendBand(u, 1, band(3, 0, 100, 50, 0), 10)
	if err == nil || err.Code != domain.CodeBandDiscontinuity {
		t.Fatalf("expected BAND_DISCONTINUITY, got %v", err)
	}
}

func TestAppendBandRejectsInsufficientOverlap(t *testing.T) {
	u := &Unit{ID: "u1"}
	_ = AppendBand(u, 1, band(1, 0, 100, 50, 0), 10)
	err := AppendBand(u, 1, band(2, 90, 190, 50, 5), 10)
	if err == nil || err.Code != domain.CodeOverlapInsufficient {
		t.Fatalf("expected OVERLAP_INSUFFICIENT, got %v", err)
	}
}

func TestAppendBandRejectsZeroWidth(t *testing.T) {
	u := &Unit{ID: "u1"}
	err := AppendBand(u, 1, band(1, 0, 100, 0, 0), 10)
	if err == nil || err.Code != domain.CodeInvalidRequest {
		t.Fatalf("expected INVALID_REQUEST, got %v", err)
	}
}
