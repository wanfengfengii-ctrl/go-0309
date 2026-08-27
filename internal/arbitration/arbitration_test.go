package arbitration

import (
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

func TestUniqueRepairSetCanonical(t *testing.T) {
	in := []domain.UnitID{"u3", "u1", "u3", "u2"}
	out := UniqueRepairSet(in)
	want := []domain.UnitID{"u1", "u2", "u3"}
	if len(out) != len(want) {
		t.Fatalf("got %v, want %v", out, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("got %v, want %v", out, want)
		}
	}
}

func TestValidateReviewsRequiresTwoDistinctQualified(t *testing.T) {
	reviews := []Review{
		{Reviewer: "p1", Qualified: true, Conclusion: "approve"},
		{Reviewer: "p2", Qualified: true, Conclusion: "approve"},
	}
	if err := ValidateReviews(reviews); err != nil {
		t.Fatalf("expected valid reviews, got %v", err)
	}
}

func TestValidateReviewsRejectsSameReviewer(t *testing.T) {
	reviews := []Review{
		{Reviewer: "p1", Qualified: true, Conclusion: "approve"},
		{Reviewer: "p1", Qualified: true, Conclusion: "approve"},
	}
	if err := ValidateReviews(reviews); err == nil {
		t.Fatal("expected same-reviewer rejection")
	}
}

func TestValidateReviewsRejectsSingleApproval(t *testing.T) {
	reviews := []Review{
		{Reviewer: "p1", Qualified: true, Conclusion: "approve"},
		{Reviewer: "p2", Qualified: true, Conclusion: "reject"},
	}
	if err := ValidateReviews(reviews); err == nil {
		t.Fatal("expected insufficient approval rejection")
	}
}
