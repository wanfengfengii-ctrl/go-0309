package fixedpoint

import (
	"errors"
	"testing"
)

func TestFromFractionRoundsHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		num, den int64
		want     int64
	}{
		{1, 2, Scale / 2}, // 0.5
		{1, 3, 333333},    // 0.333333
		{2, 3, 666667},    // 0.666667
		{1, 1, Scale},     // 1.0
		{3, 1, 3 * Scale}, // 3.0
	}
	for _, c := range cases {
		got, err := FromFraction(c.num, c.den)
		if err != nil {
			t.Fatalf("FromFraction(%d,%d): %v", c.num, c.den, err)
		}
		if got.Raw() != c.want {
			t.Errorf("FromFraction(%d,%d) = %d, want %d", c.num, c.den, got.Raw(), c.want)
		}
	}
}

func TestFromFractionRejectsZeroDenominator(t *testing.T) {
	_, err := FromFraction(1, 0)
	if !errors.Is(err, ErrDivideByZero) {
		t.Fatalf("want ErrDivideByZero, got %v", err)
	}
}

func TestFromFractionRejectsNegative(t *testing.T) {
	if _, err := FromFraction(-1, 2); !errors.Is(err, ErrNegative) {
		t.Fatalf("want ErrNegative, got %v", err)
	}
	if _, err := FromFraction(1, -2); !errors.Is(err, ErrNegative) {
		t.Fatalf("want ErrNegative, got %v", err)
	}
}

func TestMulRoundsAndScales(t *testing.T) {
	a := FromInt(2)
	b, err := FromFraction(1, 2)
	if err != nil {
		t.Fatalf("FromFraction: %v", err)
	}
	got, err := Mul(a, b)
	if err != nil {
		t.Fatalf("Mul: %v", err)
	}
	if got.Raw() != Scale {
		t.Errorf("Mul(2, 1/2) = %d, want %d", got.Raw(), Scale)
	}
}

func TestDivByZero(t *testing.T) {
	_, err := Div(FromInt(1), FromInt(0))
	if !errors.Is(err, ErrDivideByZero) {
		t.Fatalf("want ErrDivideByZero, got %v", err)
	}
}

func TestAddOverflow(t *testing.T) {
	a := FromRaw(1 << 62)
	b := FromRaw(1 << 62)
	_, err := Add(a, b)
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("want ErrOverflow, got %v", err)
	}
}
