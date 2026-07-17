package qdecimal

import "math/big"

// NewFromMinorUnits creates a Decimal from integer minor units.
//
// Example: NewFromMinorUnits(12345, 2) represents 123.45.
func NewFromMinorUnits(units int64, scale int32) (Decimal, error) {
	return New(units, scale)
}

// MinorUnits returns d rounded to scale and represented as an integer number of
// minor units.
func (d Decimal) MinorUnits(scale int32, mode RoundingMode) (*big.Int, error) {
	rounded, err := d.Rescale(scale, mode)
	if err != nil {
		return nil, err
	}
	return rounded.Coefficient(), nil
}

// MinorUnitsExact returns d as minor units only when no non-zero digit would be
// discarded at scale.
func (d Decimal) MinorUnitsExact(scale int32) (*big.Int, error) {
	exact, err := d.RescaleExact(scale)
	if err != nil {
		return nil, err
	}
	return exact.Coefficient(), nil
}

// Int64MinorUnits is like MinorUnits but fails if the result does not fit int64.
func (d Decimal) Int64MinorUnits(scale int32, mode RoundingMode) (int64, error) {
	units, err := d.MinorUnits(scale, mode)
	if err != nil {
		return 0, err
	}
	if !units.IsInt64() {
		return 0, ErrOverflow
	}
	return units.Int64(), nil
}

// Int64MinorUnitsExact is like MinorUnitsExact but fails if the result does not
// fit int64.
func (d Decimal) Int64MinorUnitsExact(scale int32) (int64, error) {
	units, err := d.MinorUnitsExact(scale)
	if err != nil {
		return 0, err
	}
	if !units.IsInt64() {
		return 0, ErrOverflow
	}
	return units.Int64(), nil
}

// ValidateMinorScale validates common currency minor-unit scales.
func ValidateMinorScale(scale int32) error {
	if scale < 0 || scale > 18 {
		return ErrInvalidScale
	}
	return nil
}

// MustMinorScale validates common currency minor-unit scales for package
// initialization and tests.
func MustMinorScale(scale int32) int32 {
	if err := ValidateMinorScale(scale); err != nil {
		panic(err)
	}
	return scale
}
