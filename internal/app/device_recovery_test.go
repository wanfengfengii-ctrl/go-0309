package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/evidence"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestDeviceFaultsProduceDeterministicRetry(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	callID := domain.DeviceCallID("call-1")
	if _, err := svc.CreateDeviceCall(ctx, httpapi.DeviceCallRequest{
		OperationID: "op-call", ID: callID, Device: evidence.DeviceScale,
		Params: "weigh", LogicalTime: 100, FenceToken: "f1",
	}); err != nil {
		t.Fatalf("CreateDeviceCall: %v", err)
	}

	// A timeout advances the call to retrying with a deterministic deadline.
	view, err := svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{Fence: "f1", Attempt: 0, Fault: evidence.FaultTimeout})
	if err != nil {
		t.Fatalf("SubmitReceipt(timeout): %v", err)
	}
	if view.Status != evidence.CallRetrying || view.Attempt != 1 {
		t.Fatalf("expected retrying attempt 1, got %+v", view)
	}
	if view.RetryAfter != 100+deviceBackoff {
		t.Fatalf("retry_after %d != %d", view.RetryAfter, 100+deviceBackoff)
	}
}

func TestDeviceStaleFenceAndOutOfOrderReceipt(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	callID := domain.DeviceCallID("call-2")
	_, _ = svc.CreateDeviceCall(ctx, httpapi.DeviceCallRequest{
		OperationID: "op-call2", ID: callID, Device: evidence.DeviceFlowMeter,
		Params: "flow", LogicalTime: 100, FenceToken: "f1",
	})

	// A foreign fence token must not advance the call.
	if _, err := svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{Fence: "wrong", Attempt: 0, Value: "42"}); err == nil {
		t.Fatal("expected stale fence rejection")
	}
	// An out-of-order attempt must not advance the call.
	if _, err := svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{Fence: "f1", Attempt: 5, Value: "42"}); err == nil {
		t.Fatal("expected out-of-order attempt rejection")
	}
}

func TestDeviceCallSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recovery.db")

	svc, err := NewService(path)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	ctx := context.Background()
	callID := domain.DeviceCallID("call-3")
	_, _ = svc.CreateDeviceCall(ctx, httpapi.DeviceCallRequest{
		OperationID: "op-call3", ID: callID, Device: evidence.DeviceThickness,
		Params: "scan", LogicalTime: 100, FenceToken: "f1",
	})
	_, _ = svc.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{Fence: "f1", Attempt: 0, Fault: evidence.FaultDisconnect})
	if err := svc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen: the retrying call must be recovered with its attempt and deadline.
	svc2, err := NewService(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = svc2.Close() }()

	view, err := svc2.SubmitReceipt(ctx, callID, httpapi.ReceiptRequest{Fence: "f1", Attempt: 1, Value: "99"})
	if err != nil {
		t.Fatalf("post-restart receipt: %v", err)
	}
	if view.Status != evidence.CallSucceeded || view.Receipt != "99" {
		t.Fatalf("expected success after restart, got %+v", view)
	}
}
