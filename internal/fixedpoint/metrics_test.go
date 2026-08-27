package fixedpoint

import (
	"errors"
	"testing"
)

func TestWaterCementRatio(t *testing.T) {
	q, err := WaterCementRatio(500, 1000)
	if err != nil {
		t.Fatalf("WaterCementRatio: %v", err)
	}
	if q.Raw() != Scale/2 {
		t.Fatalf("got %d, want %d", q.Raw(), Scale/2)
	}
}

func TestWaterCementRatioDivideByZero(t *testing.T) {
	if _, err := WaterCementRatio(100, 0); !errors.Is(err, ErrDivideByZero) {
		t.Fatalf("want ErrDivideByZero, got %v", err)
	}
}

func TestFiberContent(t *testing.T) {
	q, err := FiberContent(50, 1000)
	if err != nil {
		t.Fatalf("FiberContent: %v", err)
	}
	if q.Raw() != Scale/20 {
		t.Fatalf("got %d, want %d", q.Raw(), Scale/20)
	}
}

func TestReboundRate(t *testing.T) {
	q, err := ReboundRate(100, 1000)
	if err != nil {
		t.Fatalf("ReboundRate: %v", err)
	}
	if q.Raw() != Scale/10 {
		t.Fatalf("got %d, want %d", q.Raw(), Scale/10)
	}
}

func TestStrengthGrowthRate(t *testing.T) {
	q, err := StrengthGrowthRate(10, 30, 4)
	if err != nil {
		t.Fatalf("StrengthGrowthRate: %v", err)
	}
	if q.Raw() != 5*Scale {
		t.Fatalf("got %d, want %d", q.Raw(), 5*Scale)
	}
}

func TestStrengthGrowthRateRejectsNonPositiveInterval(t *testing.T) {
	if _, err := StrengthGrowthRate(10, 30, 0); !errors.Is(err, ErrDivideByZero) {
		t.Fatalf("want ErrDivideByZero, got %v", err)
	}
	if _, err := StrengthGrowthRate(30, 10, 4); !errors.Is(err, ErrNegative) {
		t.Fatalf("want ErrNegative, got %v", err)
	}
}

func TestEffectiveThickness(t *testing.T) {
	q, err := EffectiveThickness(100000, 2000)
	if err != nil {
		t.Fatalf("EffectiveThickness: %v", err)
	}
	if q.Raw() != 50*Scale {
		t.Fatalf("got %d, want %d", q.Raw(), 50*Scale)
	}
}

func TestCheckMinBoundary(t *testing.T) {
	val := FromInt(10)
	out := CheckMin(val, FromInt(10))
	if !out.Pass || !out.AtBoundary {
		t.Fatalf("expected boundary pass, got %+v", out)
	}
	out = CheckMin(FromInt(9), FromInt(10))
	if out.Pass {
		t.Fatal("expected below-minimum failure")
	}
}
