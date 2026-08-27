package app

import (
	"context"
	"encoding/json"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/store"
)

// CreateDeviceCall records a scripted instrument invocation. Rejection,
// disconnect, timeout and bad-format results only create a retryable call; a
// valid reading is required to advance any prefix.
func (s *Service) CreateDeviceCall(ctx context.Context, req httpapi.DeviceCallRequest) (httpapi.DeviceCallView, error) {
	_ = ctx
	digest := digestOf(req)
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.DeviceCallView
	err := s.store.Update(func(tx *store.Tx) error {
		if replay, stored, err := s.guard(tx, req.OperationID, digest); err != nil {
			return err
		} else if replay {
			return decodeResult(stored, &view)
		}
		call := evidence.DeviceCall{
			ID:          req.ID,
			Device:      req.Device,
			Params:      req.Params,
			LogicalTime: req.LogicalTime,
			FenceToken:  req.FenceToken,
			Status:      evidence.CallPending,
		}
		if err := tx.PutDeviceCall(call); err != nil {
			return err
		}
		s.devices[call.ID] = call
		view = deviceCallView(call)
		b, _ := json.Marshal(view)
		return s.record(tx, req.OperationID, digest, string(b), req.LogicalTime)
	})
	return view, err
}

// SubmitReceipt applies a scripted instrument result to an existing call,
// filtering stale fences and out-of-order attempts.
func (s *Service) SubmitReceipt(ctx context.Context, id domain.DeviceCallID, req httpapi.ReceiptRequest) (httpapi.DeviceCallView, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()

	var view httpapi.DeviceCallView
	err := s.store.Update(func(tx *store.Tx) error {
		call, ok, err := tx.GetDeviceCall(id)
		if err != nil {
			return err
		}
		if !ok {
			return domain.NewError(domain.CodeNotFound, "device call not found")
		}
		receipt := evidence.Receipt{
			Fence:   req.Fence,
			Attempt: req.Attempt,
			Fault:   req.Fault,
			Value:   req.Value,
		}
		if derr := call.SubmitReceipt(receipt, call.LogicalTime, deviceBackoff); derr != nil {
			return derr
		}
		if err := tx.PutDeviceCall(*call); err != nil {
			return err
		}
		s.devices[id] = *call
		view = deviceCallView(*call)
		return nil
	})
	return view, err
}

func deviceCallView(c evidence.DeviceCall) httpapi.DeviceCallView {
	return httpapi.DeviceCallView{
		ID:         c.ID,
		Device:     c.Device,
		Status:     c.Status,
		Attempt:    c.Attempt,
		RetryAfter: c.RetryAfter,
		Fault:      c.Fault,
		Receipt:    c.Receipt,
	}
}
