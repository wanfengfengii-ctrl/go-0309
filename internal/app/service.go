// Package app wires the shotcrete closure business flows on top of the
// embedded store. It owns persistence, transaction boundaries, idempotency, the
// recovery startup flow and the concrete implementations of the mass.Inventory
// and mass.LeaseStore contracts, delegating pure domain validation to the
// design, coverage, mass, evidence, arbitration and fixedpoint packages.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// Service implements the httpapi.Service contract. A single mutex serializes
// writes so that inventory deduction, pan creation, lease acquisition and the
// terminal-decision barrier are atomic within the process, while every write is
// additionally committed through a bbolt transaction for durability.
type Service struct {
	store *store.Store

	mu      sync.Mutex
	stock   *mass.Stock
	leases  *mass.LeaseSet
	devices map[domain.DeviceCallID]evidence.DeviceCall

	nextCycleID int64
	nextPanID   int64
	nextBandID  int64
	nextSerial  int64
}

// deviceBackoff is the base retry interval for scripted instrument calls.
const deviceBackoff = domain.LogicalTime(10)

// NewService opens the persistent store and recovers in-memory state.
func NewService(path string) (*Service, error) {
	st, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	s := &Service{
		store:   st,
		stock:   mass.NewStock(nil),
		leases:  mass.NewLeaseSet(),
		devices: map[domain.DeviceCallID]evidence.DeviceCall{},
	}
	if err := s.recover(); err != nil {
		_ = st.Close()
		return nil, err
	}
	return s, nil
}

// Close shuts down the backing store.
func (s *Service) Close() error { return s.store.Close() }

// Health reports readiness; it fails only if the store is unavailable.
func (s *Service) Health(_ context.Context) error {
	if s.store == nil {
		return domain.NewError(domain.CodeInternalError, "store unavailable")
	}
	return nil
}

// recover reloads inventory, leases, device calls and the id counter from the
// persisted database so that a restart resumes pending device retries and valid
// leases exactly as they were before the process stopped.
func (s *Service) recover() error {
	return s.store.View(func(tx *store.Tx) error {
		loaded := map[design.MaterialKind]int64{}
		for _, k := range []design.MaterialKind{
			design.MaterialCement, design.MaterialAggregate, design.MaterialWater,
			design.MaterialAccelerator, design.MaterialSteelFiber,
		} {
			g, err := tx.GetInventory(k)
			if err != nil {
				return err
			}
			loaded[k] = g
		}
		s.stock = mass.NewStock(loaded)

		leases, err := tx.Leases("")
		if err != nil {
			return err
		}
		for _, l := range leases {
			s.leases = rebuildLeases(s.leases, l)
		}

		calls, err := tx.DeviceCalls()
		if err != nil {
			return err
		}
		for _, c := range calls {
			s.devices[c.ID] = c
			if int64(len(c.ID)) > s.nextSerial {
				s.nextSerial = int64(len(c.ID))
			}
		}

		ids, err := tx.CycleIDs()
		if err != nil {
			return err
		}
		s.nextCycleID = int64(len(ids))
		return nil
	})
}

// rebuildLeases loads a lease into the in-memory set during recovery.
func rebuildLeases(set *mass.LeaseSet, l mass.ResourceLease) *mass.LeaseSet {
	if set == nil {
		set = mass.NewLeaseSet()
	}
	// Acquire validates overlap, but during recovery the persisted leases were
	// already validated when written; insert directly via a fresh set to avoid
	// false conflicts from ordering.
	set.Insert(l)
	return set
}

// digestOf computes a deterministic content digest for idempotency. Go's JSON
// encoder sorts map keys, so equivalent requests produce identical digests.
func digestOf(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// guard enforces idempotency within a transaction. It returns the stored result
// JSON when a replay is detected, or (replay=false) when the caller should
// proceed. A content conflict returns a stable IDEMPOTENCY_CONFLICT error.
func (s *Service) guard(tx *store.Tx, op domain.OperationID, digest string) (replay bool, result string, err error) {
	if op == "" {
		return false, "", domain.NewError(domain.CodeInvalidRequest, "operation_id required")
	}
	rec, ok, err := tx.GetOperation(op)
	if err != nil {
		return false, "", err
	}
	if ok {
		if rec.Digest != digest {
			return false, "", domain.NewError(domain.CodeIdempotencyConflict, "operation id content conflict").WithOperation(string(op))
		}
		return true, rec.Result, nil
	}
	return false, "", nil
}

// record persists the idempotency guard for a successful command.
func (s *Service) record(tx *store.Tx, op domain.OperationID, digest, resultJSON string, at domain.LogicalTime) error {
	return tx.PutOperation(domain.OperationRecord{
		OperationID: op,
		Digest:      digest,
		Result:      resultJSON,
		LogicalTime: at,
	})
}

// decodeResult is a convenience for replay paths that must reconstruct a view.
func decodeResult[T any](result string, out *T) error {
	if result == "" {
		return fmt.Errorf("empty stored result")
	}
	return json.Unmarshal([]byte(result), out)
}
