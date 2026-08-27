package mass

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// ResourceKind enumerates the six mutually exclusive resource classes.
type ResourceKind string

// The six leased resource kinds.
const (
	ResourceMixer       ResourceKind = "mixer"
	ResourceSprayer     ResourceKind = "sprayer"
	ResourceAccelPump   ResourceKind = "accelerator_pump"
	ResourceArmArea     ResourceKind = "arm_area"
	ResourceReboundArea ResourceKind = "rebound_area"
	ResourceTestChannel ResourceKind = "test_channel"
)

// ResourceLease is a time-bounded exclusive lease of a resource. It is
// identified by resource, holder and a fence token, and its validity is
// bounded by a logical time interval.
type ResourceLease struct {
	ID         domain.LeaseID     `json:"id"`
	Resource   ResourceKind       `json:"resource"`
	Holder     string             `json:"holder"`
	Start      domain.LogicalTime `json:"start"`
	End        domain.LogicalTime `json:"end"`
	FenceToken string             `json:"fence_token"`
}

// Active reports whether the lease is valid at the given logical time.
func (l ResourceLease) Active(at domain.LogicalTime) bool {
	return at >= l.Start && at < l.End
}

// LeaseStore is the interface for persisting and querying leases. A concrete
// store enforces the single-holder invariant within a resource's valid
// interval and reconstructs lease state from persisted logical time on
// recovery.
type LeaseStore interface {
	// Acquire atomically grants the leases or fails entirely (all-or-nothing).
	Acquire(leases []ResourceLease) error
	// Renew extends a lease's end time, rejecting expired or foreign fences.
	Renew(id domain.LeaseID, fence string, newEnd domain.LogicalTime) error
	// Release releases a lease, rejecting expired or foreign fences.
	Release(id domain.LeaseID, fence string) error
	// Conflicts returns the leases that overlap the requested interval for the
	// same resources, if any.
	Conflicts(resource ResourceKind, start, end domain.LogicalTime) ([]ResourceLease, error)
}

// Inventory is the conditional-decrement store for steel-fiber and accelerator
// stock, preventing over-issue under concurrency.
type Inventory interface {
	// Deduct conditionally decrements the given material amount, failing when
	// the batch would be over-issued.
	Deduct(kind design.MaterialKind, grams int64, expectedVersion int64) error
}
