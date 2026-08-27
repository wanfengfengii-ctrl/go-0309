package app

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestModel_CycleIDContinuityAcrossRestart(t *testing.T) {
	tests := []struct {
		name    string
		restart bool
	}{
		{name: "running service allocates the next cycle", restart: false},
		{name: "restarted service recovers the next cycle position", restart: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			dbPath := filepath.Join(t.TempDir(), "cycles.db")
			svc, err := NewService(dbPath)
			if err != nil {
				t.Fatalf("NewService: %v", err)
			}
			t.Cleanup(func() { _ = svc.Close() })

			oldID := setupReadyForClosure(t, svc)
			if oldID != "cycle-1" {
				t.Fatalf("first cycle ID = %q, want cycle-1", oldID)
			}
			submitApprovals(t, svc, oldID)
			if _, err := svc.SubmitTerminalDecision(ctx, oldID, httpapi.DecisionRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "op-restart-decision", LogicalTime: 81},
				Kind:          arbitration.TerminalClosure,
			}); err != nil {
				t.Fatalf("SubmitTerminalDecision: %v", err)
			}

			beforeAudit, err := svc.GetAudit(ctx, oldID)
			if err != nil {
				t.Fatalf("GetAudit before next lock: %v", err)
			}
			beforeCoverage, err := svc.GetCoverage(ctx, oldID)
			if err != nil {
				t.Fatalf("GetCoverage before next lock: %v", err)
			}

			if tt.restart {
				if err := svc.Close(); err != nil {
					t.Fatalf("Close before restart: %v", err)
				}
				svc, err = NewService(dbPath)
				if err != nil {
					t.Fatalf("NewService after restart: %v", err)
				}
			}

			nextReq := validLockRequest()
			nextReq.OperationID = "op-lock-next"
			nextReq.Digest = "design-v2"
			nextReq.Snapshot.Tunnel = "T2"
			nextReq.Snapshot.CycleNo = 2
			next, err := svc.LockCycle(ctx, nextReq)
			if err != nil {
				t.Fatalf("LockCycle next: %v", err)
			}
			if next.ID != "cycle-2" {
				t.Errorf("next cycle ID = %q, want cycle-2", next.ID)
			}

			afterAudit, err := svc.GetAudit(ctx, oldID)
			if err != nil {
				t.Fatalf("GetAudit old cycle after next lock: %v", err)
			}
			if !reflect.DeepEqual(afterAudit, beforeAudit) {
				t.Errorf("old cycle audit changed after locking %q", next.ID)
			}
			afterCoverage, err := svc.GetCoverage(ctx, oldID)
			if err != nil {
				t.Fatalf("GetCoverage old cycle after next lock: %v", err)
			}
			if !reflect.DeepEqual(afterCoverage, beforeCoverage) {
				t.Errorf("old cycle coverage changed after locking %q", next.ID)
			}
		})
	}
}
