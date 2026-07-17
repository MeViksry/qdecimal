package qdecimal

// PowInt returns d^exp exactly for non-negative integer exponents.
func (d Decimal) PowInt(exp uint64) (Decimal, error) {
	result := One
	base := d.copy()
	for exp > 0 {
		if exp&1 == 1 {
			nextScale := int64(result.scale) + int64(base.scale)
			if nextScale > maxScale {
				return Decimal{}, ErrInvalidScale
			}
			result = result.Mul(base)
		}
		exp >>= 1
		if exp == 0 {
			break
		}
		nextScale := int64(base.scale) + int64(base.scale)
		if nextScale > maxScale {
			return Decimal{}, ErrInvalidScale
		}
		base = base.Mul(base)
	}
	return result.canonicalZero(), nil
}

// Pow returns d^exp rounded to scale using mode.
//
// exp must be an integer-valued Decimal. Fractional exponents are rejected with
// ErrInexact instead of using a hidden floating-point approximation. Use PowInt
// when a non-negative integer exponent should preserve the exact natural scale.
func (d Decimal) Pow(exp Decimal, scale int32, mode RoundingMode) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Decimal{}, ErrInvalidRoundingMode
	}
	if !exp.IsInteger() {
		return Decimal{}, ErrInexact
	}

	normalizedExp := exp.Normalize()
	if normalizedExp.IsZero() {
		return One.Rescale(scale, mode)
	}

	magnitude := absInt(&normalizedExp.coef)
	if !magnitude.IsUint64() {
		return Decimal{}, ErrOverflow
	}

	power, err := d.PowInt(magnitude.Uint64())
	if err != nil {
		return Decimal{}, err
	}
	if normalizedExp.Sign() > 0 {
		return power.Rescale(scale, mode)
	}
	if power.IsZero() {
		return Decimal{}, ErrDivisionByZero
	}
	return One.Div(power, scale, mode)
}
