package app

import (
	"context"
	"reflect"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/arbitration"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/httpapi"
)

func TestModel_TerminalFailureIsIdempotent(t *testing.T) {
	tests := []struct {
		name             string
		addApprovals     bool
		secondKind       arbitration.TerminalKind
		wantSecondCode   domain.ErrorCode
		wantSameResponse bool
	}{
		{
			name:             "identical retry keeps the original rejection after reviews change",
			addApprovals:     true,
			secondKind:       arbitration.TerminalClosure,
			wantSecondCode:   domain.CodeInvalidRequest,
			wantSameResponse: true,
		},
		{
			name:           "different content conflicts after the original rejection",
			secondKind:     arbitration.TerminalCancel,
			wantSecondCode: domain.CodeIdempotencyConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t)
			ctx := context.Background()
			id := setupReadyForClosure(t, svc)
			req := httpapi.DecisionRequest{
				CommandHeader: httpapi.CommandHeader{OperationID: "op-terminal-sticky-failure", LogicalTime: 80},
				Kind:          arbitration.TerminalClosure,
			}

			before, err := svc.GetAudit(ctx, id)
			if err != nil {
				t.Fatalf("GetAudit before first attempt: %v", err)
			}
			firstView, firstErr := svc.SubmitTerminalDecision(ctx, id, req)
			if got := domain.AsError(firstErr); got == nil || got.Code != domain.CodeInvalidRequest {
				t.Fatalf("first attempt: want INVALID_REQUEST, got %v", firstErr)
			}
			afterFirst, err := svc.GetAudit(ctx, id)
			if err != nil {
				t.Fatalf("GetAudit after first attempt: %v", err)
			}
			if !reflect.DeepEqual(afterFirst, before) {
				t.Fatalf("failed terminal command changed business state: before=%+v after=%+v", before, afterFirst)
			}

			if tt.addApprovals {
				submitApprovals(t, svc, id)
			}
			beforeRetry, err := svc.GetAudit(ctx, id)
			if err != nil {
				t.Fatalf("GetAudit before retry: %v", err)
			}
			secondReq := req
			secondReq.Kind = tt.secondKind
			secondView, secondErr := svc.SubmitTerminalDecision(ctx, id, secondReq)
			if got := domain.AsError(secondErr); got == nil || got.Code != tt.wantSecondCode {
				t.Fatalf("retry: want %s, got %v", tt.wantSecondCode, secondErr)
			}
			if tt.wantSameResponse {
				if !reflect.DeepEqual(secondView, firstView) || !reflect.DeepEqual(domain.AsError(secondErr), domain.AsError(firstErr)) {
					t.Fatalf("identical retry did not return first result: first=(%+v, %v) second=(%+v, %v)", firstView, firstErr, secondView, secondErr)
				}
			}
			afterRetry, err := svc.GetAudit(ctx, id)
			if err != nil {
				t.Fatalf("GetAudit after retry: %v", err)
			}
			if !reflect.DeepEqual(afterRetry, beforeRetry) {
				t.Fatalf("rejected retry changed business state: before=%+v after=%+v", beforeRetry, afterRetry)
			}
		})
	}
}
