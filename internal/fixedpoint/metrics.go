package fixedpoint

import "github.com/example/mountain-tunnel-steel-fiber-shotcrete-closure/internal/domain"

// This file computes the derived engineering metrics required by the domain
// rules. Every function uses the checked fixed-point arithmetic so that
// negative operands, division by zero, boundary values and 64-bit overflow are
// surfaced as stable errors rather than silently truncated results.

// WaterCementRatio returns water mass / cement mass.
func WaterCementRatio(waterGrams, cementGrams int64) (Q, error) {
	return Div(FromInt(waterGrams), FromInt(cementGrams))
}

// AcceleratorDose returns accelerator mass / cement mass.
func AcceleratorDose(accelGrams, cementGrams int64) (Q, error) {
	return Div(FromInt(accelGrams), FromInt(cementGrams))
}

// FiberContent returns steel-fiber mass / total dry mass.
func FiberContent(fiberGrams, totalGrams int64) (Q, error) {
	return Div(FromInt(fiberGrams), FromInt(totalGrams))
}

// UnitAreaWallMass returns grams per square metre of effective wall material.
// areaMM2 is the covered area in square millimetres; one square metre equals
// one million square millimetres, so the result is mass*1e6/area.
func UnitAreaWallMass(grams, areaMM2 int64) (Q, error) {
	if areaMM2 <= 0 {
		return Q{}, ErrDivideByZero
	}
	if grams < 0 {
		return Q{}, ErrNegative
	}
	// grams per square metre: grams / (areaMM2 / 1e6) == grams * 1e6 / areaMM2.
	// Compute as (grams * 1e6) / areaMM2 using the checked divider.
	perMM2, err := Div(FromInt(grams), FromInt(areaMM2))
	if err != nil {
		return Q{}, err
	}
	// Multiply the per-mm² ratio by the number of mm² per m².
	return Mul(perMM2, FromRaw(1_000_000))
}

// ReboundRate returns rebound mass / total sprayed mass.
func ReboundRate(reboundGrams, totalGrams int64) (Q, error) {
	return Div(FromInt(reboundGrams), FromInt(totalGrams))
}

// StrengthGrowthRate returns the early-strength growth as strength change per
// unit logical time: (strength1 - strength0) / dt. A non-positive interval is
// rejected, as is a negative strength delta.
func StrengthGrowthRate(strength0, strength1 int64, dt int64) (Q, error) {
	if dt <= 0 {
		return Q{}, ErrDivideByZero
	}
	if strength1 < strength0 {
		return Q{}, ErrNegative
	}
	return Div(FromInt(strength1-strength0), FromInt(dt))
}

// VoidRatio returns the void area fraction: voidArea / totalArea.
func VoidRatio(voidArea, totalArea int64) (Q, error) {
	if totalArea <= 0 {
		return Q{}, ErrDivideByZero
	}
	if voidArea < 0 {
		return Q{}, ErrNegative
	}
	return Div(FromInt(voidArea), FromInt(totalArea))
}

// EffectiveThickness returns the average sprayed thickness in millimetres from
// a wall volume (mm³) and covered area (mm²).
func EffectiveThickness(wallVolumeMM3, areaMM2 int64) (Q, error) {
	if areaMM2 <= 0 {
		return Q{}, ErrDivideByZero
	}
	if wallVolumeMM3 < 0 {
		return Q{}, ErrNegative
	}
	return Div(FromInt(wallVolumeMM3), FromInt(areaMM2))
}

// ThresholdOutcome summarizes whether a computed metric meets its fixed-point
// threshold, distinguishing a strict below-threshold failure from an exact
// equality pass so boundary tests are deterministic.
type ThresholdOutcome struct {
	Pass bool
	// AtBoundary is true when the metric exactly equals the threshold.
	AtBoundary bool
}

// CheckMin reports whether value >= min (inclusive).
func CheckMin(value, min Q) ThresholdOutcome {
	c := Compare(value, min)
	return ThresholdOutcome{Pass: c >= 0, AtBoundary: c == 0}
}

// CheckMax reports whether value <= max (inclusive).
func CheckMax(value, max Q) ThresholdOutcome {
	c := Compare(value, max)
	return ThresholdOutcome{Pass: c <= 0, AtBoundary: c == 0}
}

// MetricError converts a fixed-point arithmetic error into a stable domain
// error. It maps sentinel errors to the documented FIXED_POINT_OVERFLOW or
// INVALID_REQUEST codes.
func MetricError(err error) *domain.Error {
	switch err {
	case ErrOverflow:
		return domain.NewError(domain.CodeFixedPointOverflow, "metric overflow")
	case ErrDivideByZero:
		return domain.NewError(domain.CodeInvalidRequest, "metric division by zero")
	case ErrNegative:
		return domain.NewError(domain.CodeInvalidRequest, "metric negative operand")
	default:
		return domain.NewError(domain.CodeInvalidRequest, "invalid metric operand")
	}
}
