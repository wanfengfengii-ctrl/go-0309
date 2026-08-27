// Package fixedpoint provides deterministic fixed-point integer arithmetic for
// the shotcrete closure service. Angles and ratios are represented as
// scale-scaled int64 values so that multiplication, addition and division can
// be checked for negative operands, division by zero, rounding rules, interval
// boundaries and 64-bit overflow, as required by the domain rules.
package fixedpoint

import (
	"errors"
	"math/bits"
	"strconv"
)

// Scale is the number of fractional units in one whole for ratios and
// percentages. A value of one million keeps percentages and small ratios
// (water-cement ratio, fiber content, rebound rate, ...) exact enough for the
// deterministic rules while remaining representable in int64.
const Scale int64 = 1_000_000

// Sentinel errors reported by the arithmetic operations.
var (
	// ErrOverflow indicates a multiplication or addition exceeded int64 range.
	ErrOverflow = errors.New("fixed point overflow")
	// ErrDivideByZero indicates division by a zero scaled value.
	ErrDivideByZero = errors.New("fixed point divide by zero")
	// ErrNegative indicates an operation rejected a negative operand where the
	// domain forbids it.
	ErrNegative = errors.New("fixed point negative value")
)

// Q is a scaled fixed-point integer. Its value is raw / Scale.
type Q struct {
	raw int64
}

// Raw returns the underlying scaled integer.
func (q Q) Raw() int64 { return q.raw }

// FromRaw builds a Q from an already-scaled integer.
func FromRaw(raw int64) Q { return Q{raw: raw} }

// FromInt builds a Q representing a whole integer.
func FromInt(v int64) Q { return Q{raw: v * Scale} }

// FromFraction builds the scaled value num/den with round-half-away-from-zero
// semantics, rejecting a zero denominator, negative operands and overflow.
func FromFraction(num, den int64) (Q, error) {
	if den == 0 {
		return Q{}, ErrDivideByZero
	}
	if num < 0 || den < 0 {
		return Q{}, ErrNegative
	}
	hi, lo := bits.Mul64(uint64(num), uint64(Scale))
	if hi != 0 {
		return Q{}, ErrOverflow
	}
	q := roundDiv(lo, uint64(den))
	return Q{raw: q}, nil
}

// Add returns a+b, checking for signed 64-bit overflow.
func Add(a, b Q) (Q, error) {
	sum := a.raw + b.raw
	if (a.raw > 0 && b.raw > 0 && sum < 0) || (a.raw < 0 && b.raw < 0 && sum >= 0) {
		return Q{}, ErrOverflow
	}
	return Q{raw: sum}, nil
}

// Sub returns a-b, checking for signed 64-bit overflow.
func Sub(a, b Q) (Q, error) {
	diff := a.raw - b.raw
	if (a.raw >= 0 && b.raw < 0 && diff < 0) || (a.raw < 0 && b.raw >= 0 && diff >= 0) {
		return Q{}, ErrOverflow
	}
	return Q{raw: diff}, nil
}

// Mul returns (a*b)/Scale, rounding half away from zero, and rejects overflow.
func Mul(a, b Q) (Q, error) {
	hi, lo := bits.Mul64(uint64(a.raw), uint64(b.raw))
	if hi != 0 {
		return Q{}, ErrOverflow
	}
	return Q{raw: roundDiv(lo, uint64(Scale))}, nil
}

// Div returns (a*Scale)/b, rounding half away from zero, and rejects a zero
// divisor or overflow.
func Div(a, b Q) (Q, error) {
	if b.raw == 0 {
		return Q{}, ErrDivideByZero
	}
	hi, lo := bits.Mul64(uint64(a.raw), uint64(Scale))
	if hi != 0 {
		return Q{}, ErrOverflow
	}
	return Q{raw: roundDiv(lo, uint64(b.raw))}, nil
}

// Compare returns -1, 0 or 1 depending on the ordering of a relative to b.
func Compare(a, b Q) int {
	switch {
	case a.raw < b.raw:
		return -1
	case a.raw > b.raw:
		return 1
	default:
		return 0
	}
}

// Int returns the whole part of q, truncating toward zero.
func (q Q) Int() int64 { return q.raw / Scale }

// Float returns the approximate float64 value (for display only).
func (q Q) Float() float64 { return float64(q.raw) / float64(Scale) }

// roundDiv returns lo/den rounded half away from zero, given lo >= 0.
func roundDiv(lo, den uint64) int64 {
	q := lo / den
	r := lo % den
	if r*2 >= den {
		q++
	}
	return int64(q)
}

// MarshalJSON serializes a Q as its raw scaled integer, keeping JSON
// round-trips exact for persistence and transport.
func (q Q) MarshalJSON() ([]byte, error) {
	return []byte(strconv.FormatInt(q.raw, 10)), nil
}

// UnmarshalJSON decodes a raw scaled integer back into a Q.
func (q *Q) UnmarshalJSON(b []byte) error {
	n, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return err
	}
	q.raw = n
	return nil
}
