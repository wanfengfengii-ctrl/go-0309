package app

import (
	"context"
	"sync"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func submitApprovals(t *testing.T, svc *Service, id domain.CycleID) {
	t.Helper()
	ctx := context.Background()
	for _, p := range []domain.PersonID{"p1", "p2"} {
		if _, err := svc.SubmitReview(ctx, id, httpapi.ReviewRequest{
			CommandHeader: httpapi.CommandHeader{OperationID: domain.OperationID("op-review-" + string(p)), LogicalTime: 70},
			Reviewer:      p, Qualified: true, Conclusion: "approve",
		}); err != nil {
			t.Fatalf("SubmitReview(%s): %v", p, err)
		}
	}
}

func TestClosureRequiresAllConditions(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	id := setupReadyForClosure(t, svc)

	// Without reviews the closure must be rejected.
	_, err := svc.SubmitTerminalDecision(ctx, id, httpapi.DecisionRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-dec", LogicalTime: 80},
		Kind:          arbitration.TerminalClosure,
	})
	if err == nil {
		t.Fatal("expected closure rejection without reviews")
	}

	// With two distinct approvals it must succeed.
	submitApprovals(t, svc, id)
	view, err := svc.SubmitTerminalDecision(ctx, id, httpapi.DecisionRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-dec2", LogicalTime: 81},
		Kind:          arbitration.TerminalClosure,
	})
	if err != nil {
		t.Fatalf("SubmitTerminalDecision: %v", err)
	}
	if view.Kind != arbitration.TerminalClosure {
		t.Fatalf("got kind %q", view.Kind)
	}
}

func TestClosureRejectsSameReviewer(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	id := setupReadyForClosure(t, svc)

	for i := 0; i < 2; i++ {
		if _, err := svc.SubmitReview(ctx, id, httpapi.ReviewRequest{
			CommandHeader: httpapi.CommandHeader{OperationID: domain.OperationID("op-r"), LogicalTime: 70},
			Reviewer:      "p1", Qualified: true, Conclusion: "approve",
		}); err != nil {
			t.Fatalf("SubmitReview: %v", err)
		}
	}
	_, err := svc.SubmitTerminalDecision(ctx, id, httpapi.DecisionRequest{
		CommandHeader: httpapi.CommandHeader{OperationID: "op-dec", LogicalTime: 80},
		Kind:          arbitration.TerminalClosure,
	})
	if err == nil {
		t.Fatal("expected same-reviewer rejection")
	}
}

func TestTerminalDecisionRaceYieldsSingleOutcome(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	id := setupReadyForClosure(t, svc)
	submitApprovals(t, svc, id)

	const n = 20
	var wg sync.WaitGroup
	results := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			kind := arbitration.TerminalClosure
			if i%2 == 1 {
				kind = arbitration.TerminalIsolate
			}
			_, err := svc.SubmitTerminalDecision(ctx, id, httpapi.DecisionRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: domain.OperationID("op-race-" + itoa(int64(i))), LogicalTime: 90},
				Kind:          kind,
			})
			results[i] = err
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", successes)
	}

	audit, err := svc.GetAudit(ctx, id)
	if err != nil {
		t.Fatalf("GetAudit: %v", err)
	}
	if audit.Decision == nil {
		t.Fatal("expected a terminal decision")
	}
}
