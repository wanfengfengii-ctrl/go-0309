package mass

import (
	"sync"
	"testing"

	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/design"
	"github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"
)

func TestModel_StockConcurrencyAndValidation(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "competing deductions cannot over-issue steel fiber",
			run: func(t *testing.T) {
				stock := NewStock(map[design.MaterialKind]int64{design.MaterialSteelFiber: 100})
				start := make(chan struct{})
				results := make([]error, 2)
				var ready sync.WaitGroup
				var done sync.WaitGroup
				ready.Add(len(results))
				done.Add(len(results))

				for i := range results {
					go func(i int) {
						defer done.Done()
						ready.Done()
						<-start
						results[i] = stock.Deduct(design.MaterialSteelFiber, 80, -1)
					}(i)
				}
				ready.Wait()
				close(start)
				done.Wait()

				successes := 0
				conflicts := 0
				for _, err := range results {
					if err == nil {
						successes++
					} else if domain.AsError(err).Code == domain.CodeMassConflict {
						conflicts++
					}
				}
				if successes != 1 || conflicts != 1 {
					t.Fatalf("deduction results: got %d successes and %d mass conflicts, want one of each", successes, conflicts)
				}
				if got := stock.Count(design.MaterialSteelFiber); got != 20 {
					t.Fatalf("steel fiber balance: got %d, want 20", got)
				}
				if got := stock.Version(); got != 1 {
					t.Fatalf("inventory version: got %d, want 1", got)
				}
				if got := stock.Snapshot()[design.MaterialSteelFiber]; got != 20 {
					t.Fatalf("snapshot steel fiber balance: got %d, want 20", got)
				}
			},
		},
		{
			name: "all public inventory operations may run concurrently",
			run: func(t *testing.T) {
				const initial = int64(1_000_000)
				stock := NewStock(map[design.MaterialKind]int64{design.MaterialSteelFiber: initial})
				start := make(chan struct{})
				deductResult := make(chan error, 1)
				var ready sync.WaitGroup
				var done sync.WaitGroup
				ready.Add(5)
				done.Add(5)

				operations := []func(){
					func() { _ = stock.Count(design.MaterialSteelFiber) },
					func() { _ = stock.Version() },
					func() { _ = stock.Snapshot() },
					func() { deductResult <- stock.Deduct(design.MaterialSteelFiber, 1, -1) },
					func() { stock.Restock(design.MaterialSteelFiber, 1) },
				}
				for _, operation := range operations {
					go func(operation func()) {
						defer done.Done()
						ready.Done()
						<-start
						operation()
					}(operation)
				}
				ready.Wait()
				close(start)
				done.Wait()

				if err := <-deductResult; err != nil {
					t.Fatalf("deduction with ample stock failed: %v", err)
				}
				if got := stock.Count(design.MaterialSteelFiber); got != initial {
					t.Fatalf("balance after equal restock and deduction: got %d, want %d", got, initial)
				}
				if got := stock.Version(); got != 2 {
					t.Fatalf("version after two successful mutations: got %d, want 2", got)
				}
			},
		},
		{
			name: "validation failures leave stock and version unchanged",
			run: func(t *testing.T) {
				stock := NewStock(map[design.MaterialKind]int64{design.MaterialSteelFiber: 100})

				if err := stock.Deduct(design.MaterialSteelFiber, -1, -1); err == nil || domain.AsError(err).Code != domain.CodeInvalidRequest {
					t.Fatalf("negative deduction: got %v, want INVALID_REQUEST", err)
				}
				if err := stock.Deduct(design.MaterialSteelFiber, 101, -1); err == nil || domain.AsError(err).Code != domain.CodeMassConflict {
					t.Fatalf("insufficient stock: got %v, want MASS_CONFLICT", err)
				}
				if err := stock.Deduct(design.MaterialSteelFiber, 20, 0); err != nil {
					t.Fatalf("deduction at expected version: %v", err)
				}
				if err := stock.Deduct(design.MaterialSteelFiber, 1, 0); err == nil || domain.AsError(err).Code != domain.CodeMassConflict {
					t.Fatalf("stale expected version: got %v, want MASS_CONFLICT", err)
				}
				if got := stock.Count(design.MaterialSteelFiber); got != 80 {
					t.Fatalf("balance after rejected deductions: got %d, want 80", got)
				}
				if got := stock.Version(); got != 1 {
					t.Fatalf("version after rejected deductions: got %d, want 1", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
