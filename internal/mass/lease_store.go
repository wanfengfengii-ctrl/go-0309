package mass

import (
	"sort"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

// LeaseSet is a concrete in-memory lease store that enforces the single-holder
// invariant within a resource's valid interval. Acquire is all-or-nothing: if
// any requested lease conflicts, none is granted.
type LeaseSet struct {
	leases map[domain.LeaseID]ResourceLease
}

// NewLeaseSet builds an empty lease set.
func NewLeaseSet() *LeaseSet { return &LeaseSet{leases: map[domain.LeaseID]ResourceLease{}} }

// Acquire grants all leases or none. It sorts the requested leases by resource
// then id to eliminate scheduling-dependent outcomes, then checks overlap.
func (s *LeaseSet) Acquire(leases []ResourceLease) error {
	if len(leases) == 0 {
		return domain.NewError(domain.CodeInvalidRequest, "no leases requested")
	}
	ordered := make([]ResourceLease, len(leases))
	copy(ordered, leases)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Resource != ordered[j].Resource {
			return ordered[i].Resource < ordered[j].Resource
		}
		return ordered[i].ID < ordered[j].ID
	})
	for _, l := range ordered {
		if conflicts, err := s.Conflicts(l.Resource, l.Start, l.End); err != nil {
			return err
		} else if len(conflicts) > 0 {
			return domain.NewError(domain.CodeLeaseConflict, "resource "+string(l.Resource)+" already leased")
		}
	}
	for _, l := range ordered {
		s.leases[l.ID] = l
	}
	return nil
}

// Renew extends a lease end time, rejecting expired or foreign fences.
func (s *LeaseSet) Renew(id domain.LeaseID, fence string, newEnd domain.LogicalTime) error {
	l, ok := s.leases[id]
	if !ok {
		return domain.NewError(domain.CodeLeaseExpired, "lease not found")
	}
	if l.FenceToken != fence {
		return domain.NewError(domain.CodeLeaseConflict, "foreign fence token")
	}
	if newEnd <= l.End {
		return domain.NewError(domain.CodeInvalidRequest, "renewal must extend lease")
	}
	// Reject if the extension would overlap another holder of the same resource.
	conflicts, err := s.Conflicts(l.Resource, l.End, newEnd)
	if err != nil {
		return err
	}
	for _, c := range conflicts {
		if c.ID != id {
			return domain.NewError(domain.CodeLeaseConflict, "extension overlaps another lease")
		}
	}
	l.End = newEnd
	s.leases[id] = l
	return nil
}

// Release removes a lease, rejecting expired or foreign fences.
func (s *LeaseSet) Release(id domain.LeaseID, fence string) error {
	l, ok := s.leases[id]
	if !ok {
		return domain.NewError(domain.CodeLeaseExpired, "lease not found")
	}
	if l.FenceToken != fence {
		return domain.NewError(domain.CodeLeaseConflict, "foreign fence token")
	}
	delete(s.leases, id)
	return nil
}

// Conflicts returns the leases of a resource overlapping [start, end).
func (s *LeaseSet) Conflicts(resource ResourceKind, start, end domain.LogicalTime) ([]ResourceLease, error) {
	var out []ResourceLease
	for _, l := range s.leases {
		if l.Resource != resource {
			continue
		}
		if l.Start < end && start < l.End {
			out = append(out, l)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Active reports whether a lease with the given fence is valid at time now.
func (s *LeaseSet) Active(id domain.LeaseID, fence string, now domain.LogicalTime) bool {
	l, ok := s.leases[id]
	if !ok {
		return false
	}
	return l.FenceToken == fence && l.Active(now)
}

// All returns every lease, sorted by id, for recovery snapshots.
func (s *LeaseSet) All() []ResourceLease {
	out := make([]ResourceLease, 0, len(s.leases))
	for _, l := range s.leases {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Insert adds a lease without re-checking conflicts. It is used only during
// recovery, where the persisted leases were already validated when first
// written, so no spurious conflict can arise from load order.
func (s *LeaseSet) Insert(l ResourceLease) {
	s.leases[l.ID] = l
}
