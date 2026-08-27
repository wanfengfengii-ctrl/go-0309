package store

import (
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/mass"
)

// PutOperation persists an idempotency guard.
func (t *Tx) PutOperation(op domain.OperationRecord) error {
	return t.PutJSON(bucketOps, []byte(op.OperationID), op)
}

// GetOperation loads an idempotency guard. It returns (nil, false) when absent.
func (t *Tx) GetOperation(id domain.OperationID) (*domain.OperationRecord, bool, error) {
	var op domain.OperationRecord
	ok, err := t.GetJSON(bucketOps, []byte(id), &op)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return &op, true, nil
}

// PutLease persists a resource lease.
func (t *Tx) PutLease(l mass.ResourceLease) error {
	return t.PutJSON(bucketLeases, []byte(l.ID), l)
}

// GetLease loads a resource lease by id.
func (t *Tx) GetLease(id domain.LeaseID) (*mass.ResourceLease, bool, error) {
	var l mass.ResourceLease
	ok, err := t.GetJSON(bucketLeases, []byte(id), &l)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return &l, true, nil
}

// DeleteLease removes a resource lease.
func (t *Tx) DeleteLease(id domain.LeaseID) error {
	return t.Delete(bucketLeases, []byte(id))
}

// Leases returns every lease whose resource matches kind.
func (t *Tx) Leases(kind mass.ResourceKind) ([]mass.ResourceLease, error) {
	var out []mass.ResourceLease
	err := t.ForEach(bucketLeases, func(_, v []byte) error {
		var l mass.ResourceLease
		if err := decode(v, &l); err != nil {
			return err
		}
		if kind == "" || l.Resource == kind {
			out = append(out, l)
		}
		return nil
	})
	return out, err
}

// PutInventory persists the remaining stock for a material kind.
func (t *Tx) PutInventory(kind design.MaterialKind, grams int64) error {
	return t.PutJSON(bucketInventory, []byte(kind), grams)
}

// GetInventory loads the remaining stock for a material kind. Absent stock is
// reported as zero.
func (t *Tx) GetInventory(kind design.MaterialKind) (int64, error) {
	var grams int64
	ok, err := t.GetJSON(bucketInventory, []byte(kind), &grams)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, nil
	}
	return grams, nil
}

// PutDeviceCall persists a device call.
func (t *Tx) PutDeviceCall(c evidence.DeviceCall) error {
	return t.PutJSON(bucketDevices, []byte(c.ID), c)
}

// GetDeviceCall loads a device call by id.
func (t *Tx) GetDeviceCall(id domain.DeviceCallID) (*evidence.DeviceCall, bool, error) {
	var c evidence.DeviceCall
	ok, err := t.GetJSON(bucketDevices, []byte(id), &c)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return &c, true, nil
}

// DeviceCalls returns every device call, ordered by key.
func (t *Tx) DeviceCalls() ([]evidence.DeviceCall, error) {
	var out []evidence.DeviceCall
	err := t.ForEach(bucketDevices, func(_, v []byte) error {
		var c evidence.DeviceCall
		if err := decode(v, &c); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}
