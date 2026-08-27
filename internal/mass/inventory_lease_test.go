package mass

import (
	"sync"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

func TestStockConcurrentDeductionPreventsOverIssue(t *testing.T) {
	stock := NewStock(map[design.MaterialKind]int64{design.MaterialSteelFiber: 100})
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = stock.Deduct(design.MaterialSteelFiber, 80, -1)
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
		t.Fatalf("expected exactly 1 successful deduction, got %d", successes)
	}
	if stock.Count(design.MaterialSteelFiber) != 20 {
		t.Fatalf("expected 20 remaining, got %d", stock.Count(design.MaterialSteelFiber))
	}
}

func TestLeaseSetAcquireIsAllOrNothing(t *testing.T) {
	set := NewLeaseSet()
	if err := set.Acquire([]ResourceLease{
		{ID: "l1", Resource: ResourceMixer, Start: 10, End: 30, FenceToken: "f1"},
	}); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	// The second batch conflicts on the mixer resource and must be fully rejected.
	err := set.Acquire([]ResourceLease{
		{ID: "l2", Resource: ResourceMixer, Start: 20, End: 40, FenceToken: "f2"},
		{ID: "l3", Resource: ResourceSprayer, Start: 20, End: 40, FenceToken: "f3"},
	})
	if err == nil || domain.AsError(err).Code != domain.CodeLeaseConflict {
		t.Fatalf("want LEASE_CONFLICT, got %v", err)
	}
	if _, ok := set.leases["l2"]; ok {
		t.Fatal("conflicting batch must not partially acquire")
	}
	if _, ok := set.leases["l3"]; ok {
		t.Fatal("conflicting batch must not partially acquire")
	}
}

func TestLeaseRenewRejectsForeignFence(t *testing.T) {
	set := NewLeaseSet()
	_ = set.Acquire([]ResourceLease{{ID: "l1", Resource: ResourceMixer, Start: 10, End: 30, FenceToken: "f1"}})
	if err := set.Renew("l1", "wrong", 40); err == nil || domain.AsError(err).Code != domain.CodeLeaseConflict {
		t.Fatalf("want LEASE_CONFLICT on foreign fence, got %v", err)
	}
	if err := set.Renew("l1", "f1", 40); err != nil {
		t.Fatalf("valid renew: %v", err)
	}
	if l := set.leases["l1"]; l.End != 40 {
		t.Fatalf("expected end 40, got %d", l.End)
	}
}
