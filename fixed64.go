package qdecimal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// Fixed64 is a compact fixed-scale decimal for hot ledger and trading paths.
//
// It stores integer minor units and a non-negative scale. Use Decimal for
// arbitrary precision; use Fixed64 when the business domain has a known bounded
// scale and int64 range is sufficient.
type Fixed64 struct {
	units int64
	scale int32
}

var fixedPow10 = [...]int64{
	1,
	10,
	100,
	1_000,
	10_000,
	100_000,
	1_000_000,
	10_000_000,
	100_000_000,
	1_000_000_000,
	10_000_000_000,
	100_000_000_000,
	1_000_000_000_000,
	10_000_000_000_000,
	100_000_000_000_000,
	1_000_000_000_000_000,
	10_000_000_000_000_000,
	100_000_000_000_000_000,
	1_000_000_000_000_000_000,
}

// NewFixed64 creates a fixed-scale decimal from integer units.
func NewFixed64(units int64, scale int32) (Fixed64, error) {
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}
	return Fixed64{units: units, scale: scale}, nil
}

// ParseFixed64 parses input and rounds it to scale.
func ParseFixed64(s string, scale int32, mode RoundingMode) (Fixed64, error) {
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Fixed64{}, ErrInvalidRoundingMode
	}
	if out, ok, err := parseFixed64Fast(s, scale, mode); ok || err != nil {
		return out, err
	}
	d, err := Parse(s)
	if err != nil {
		return Fixed64{}, err
	}
	return Fixed64FromDecimal(d, scale, mode)
}

// Fixed64FromDecimal converts d to Fixed64 at scale using mode.
func Fixed64FromDecimal(d Decimal, scale int32, mode RoundingMode) (Fixed64, error) {
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Fixed64{}, ErrInvalidRoundingMode
	}
	units, err := d.Int64MinorUnits(scale, mode)
	if err != nil {
		return Fixed64{}, err
	}
	return Fixed64{units: units, scale: scale}, nil
}

// Units returns the integer minor units.
func (f Fixed64) Units() int64 { return f.units }

// Scale returns the fixed decimal scale.
func (f Fixed64) Scale() int32 { return f.scale }

// Decimal converts f to arbitrary-precision Decimal exactly.
func (f Fixed64) Decimal() Decimal {
	d, _ := New(f.units, f.scale)
	return d
}

// String returns f's fixed-scale decimal representation.
func (f Fixed64) String() string {
	return string(f.appendText(make([]byte, 0, 32)))
}

func (f Fixed64) appendText(dst []byte) []byte {
	negative := f.units < 0
	magnitude := absInt64Magnitude(f.units)
	if negative {
		dst = append(dst, '-')
	}
	digitStart := len(dst)
	dst = strconv.AppendUint(dst, magnitude, 10)
	if f.scale == 0 {
		return dst
	}

	scale := int(f.scale)
	digitLen := len(dst) - digitStart
	if digitLen <= scale {
		prefixLen := 2 + scale - digitLen
		oldEnd := len(dst)
		for i := 0; i < prefixLen; i++ {
			dst = append(dst, 0)
		}
		copy(dst[digitStart+prefixLen:], dst[digitStart:oldEnd])
		dst[digitStart] = '0'
		dst[digitStart+1] = '.'
		for i := digitStart + 2; i < digitStart+prefixLen; i++ {
			dst[i] = '0'
		}
		return dst
	}
	point := len(dst) - scale
	dst = append(dst, 0)
	copy(dst[point+1:], dst[point:len(dst)-1])
	dst[point] = '.'
	return dst
}

// IsZero reports whether f is zero.
func (f Fixed64) IsZero() bool { return f.units == 0 }

// Sign returns -1, 0, or +1.
func (f Fixed64) Sign() int {
	switch {
	case f.units < 0:
		return -1
	case f.units > 0:
		return 1
	default:
		return 0
	}
}

// Neg returns -f, checking int64 overflow.
func (f Fixed64) Neg() (Fixed64, error) {
	if f.units == math.MinInt64 {
		return Fixed64{}, ErrOverflow
	}
	return Fixed64{units: -f.units, scale: f.scale}, nil
}

// Abs returns |f|, checking int64 overflow.
func (f Fixed64) Abs() (Fixed64, error) {
	if f.units >= 0 {
		return f, nil
	}
	return f.Neg()
}

// Rescale changes f to scale using mode when minor digits must be discarded.
func (f Fixed64) Rescale(scale int32, mode RoundingMode) (Fixed64, error) {
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Fixed64{}, ErrInvalidRoundingMode
	}
	if scale == f.scale {
		return f, nil
	}
	if scale > f.scale {
		units, ok := scaleUnits64(f.units, scale-f.scale)
		if !ok {
			return Fixed64{}, ErrOverflow
		}
		return Fixed64{units: units, scale: scale}, nil
	}

	divisor := fixedPow10[f.scale-scale]
	q := f.units / divisor
	r := f.units % divisor
	if r == 0 || mode == TowardZero {
		return Fixed64{units: q, scale: scale}, nil
	}

	sign := 1
	if f.units < 0 {
		sign = -1
	}
	absR := absInt64Magnitude(r)
	absDivisor := uint64(divisor)
	increment := false

	switch mode {
	case AwayFromZero:
		increment = true
	case TowardPositive:
		increment = sign > 0
	case TowardNegative:
		increment = sign < 0
	case ToNearestAway, ToNearestTowardZero, ToNearestEven:
		twiceR := absR * 2
		switch {
		case twiceR > absDivisor:
			increment = true
		case twiceR == absDivisor:
			switch mode {
			case ToNearestAway:
				increment = true
			case ToNearestEven:
				increment = q%2 != 0
			}
		}
	}
	if !increment {
		return Fixed64{units: q, scale: scale}, nil
	}
	if sign >= 0 {
		out, ok := checkedAdd64(q, 1)
		if !ok {
			return Fixed64{}, ErrOverflow
		}
		return Fixed64{units: out, scale: scale}, nil
	}
	out, ok := checkedAdd64(q, -1)
	if !ok {
		return Fixed64{}, ErrOverflow
	}
	return Fixed64{units: out, scale: scale}, nil
}

// Round is an alias for Rescale.
func (f Fixed64) Round(scale int32, mode RoundingMode) (Fixed64, error) {
	return f.Rescale(scale, mode)
}

// Truncate rounds toward zero to scale.
func (f Fixed64) Truncate(scale int32) (Fixed64, error) {
	return f.Rescale(scale, TowardZero)
}

// Ceil rounds toward +infinity to scale.
func (f Fixed64) Ceil(scale int32) (Fixed64, error) {
	return f.Rescale(scale, TowardPositive)
}

// Floor rounds toward -infinity to scale.
func (f Fixed64) Floor(scale int32) (Fixed64, error) {
	return f.Rescale(scale, TowardNegative)
}

// QuantizeStep rounds f to a valid multiple of step using mode.
//
// This is intended for bounded-scale exchange ticks, lot sizes, and banking
// increments. The returned value uses step's scale.
func (f Fixed64) QuantizeStep(step Fixed64, mode RoundingMode) (Fixed64, error) {
	if step.units == 0 {
		return Fixed64{}, ErrDivisionByZero
	}
	if !mode.valid() {
		return Fixed64{}, ErrInvalidRoundingMode
	}
	if out, ok := fixed64QuantizeStepFast(f, step, mode); ok {
		return out, nil
	}
	return fixed64QuantizeStepDecimal(f, step, mode)
}

// Add returns f + other exactly, aligning scales when possible.
func (f Fixed64) Add(other Fixed64) (Fixed64, error) {
	left, right, scale, err := alignFixed64(f, other)
	if err != nil {
		return Fixed64{}, err
	}
	sum, ok := checkedAdd64(left, right)
	if !ok {
		return Fixed64{}, ErrOverflow
	}
	return Fixed64{units: sum, scale: scale}, nil
}

// Sub returns f - other exactly, aligning scales when possible.
func (f Fixed64) Sub(other Fixed64) (Fixed64, error) {
	neg, err := other.Neg()
	if err != nil {
		return Fixed64{}, err
	}
	return f.Add(neg)
}

// Mul returns f * other rounded to scale using mode.
func (f Fixed64) Mul(other Fixed64, scale int32, mode RoundingMode) (Fixed64, error) {
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Fixed64{}, ErrInvalidRoundingMode
	}
	if product, ok := checkedMul64(f.units, other.units); ok {
		productScale := f.scale + other.scale
		if validFixed64Scale(productScale) {
			return (Fixed64{units: product, scale: productScale}).Rescale(scale, mode)
		}
	}
	return Fixed64FromDecimal(f.Decimal().Mul(other.Decimal()), scale, mode)
}

// Div returns f / other rounded to scale using mode.
func (f Fixed64) Div(other Fixed64, scale int32, mode RoundingMode) (Fixed64, error) {
	if other.units == 0 {
		return Fixed64{}, ErrDivisionByZero
	}
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Fixed64{}, ErrInvalidRoundingMode
	}
	out, err := f.Decimal().Div(other.Decimal(), scale, mode)
	if err != nil {
		return Fixed64{}, err
	}
	return Fixed64FromDecimal(out, scale, TowardZero)
}

// Cmp compares f and other numerically.
func (f Fixed64) Cmp(other Fixed64) int {
	left, right, _, err := alignFixed64(f, other)
	if err != nil {
		return f.Decimal().Cmp(other.Decimal())
	}
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

// Equal reports numeric equality.
func (f Fixed64) Equal(other Fixed64) bool { return f.Cmp(other) == 0 }

// Between reports whether f is inside [min, max] when inclusive is true, or
// inside (min, max) when inclusive is false. Reversed bounds are accepted.
func (f Fixed64) Between(min, max Fixed64, inclusive bool) bool {
	if min.Cmp(max) > 0 {
		min, max = max, min
	}
	lower := f.Cmp(min)
	upper := f.Cmp(max)
	if inclusive {
		return lower >= 0 && upper <= 0
	}
	return lower > 0 && upper < 0
}

// Clamp constrains f to [min, max]. Reversed bounds are accepted.
func (f Fixed64) Clamp(min, max Fixed64) Fixed64 {
	if min.Cmp(max) > 0 {
		min, max = max, min
	}
	if f.Cmp(min) < 0 {
		return min
	}
	if f.Cmp(max) > 0 {
		return max
	}
	return f
}

// MinFixed64 returns the smallest value. An empty input returns the zero value.
func MinFixed64(values ...Fixed64) Fixed64 {
	if len(values) == 0 {
		return Fixed64{}
	}
	min := values[0]
	for _, value := range values[1:] {
		if value.Cmp(min) < 0 {
			min = value
		}
	}
	return min
}

// MaxFixed64 returns the largest value. An empty input returns the zero value.
func MaxFixed64(values ...Fixed64) Fixed64 {
	if len(values) == 0 {
		return Fixed64{}
	}
	max := values[0]
	for _, value := range values[1:] {
		if value.Cmp(max) > 0 {
			max = value
		}
	}
	return max
}

// SumFixed64 returns the exact sum of values. The fast path keeps same-scale
// sums in int64 units; mixed-scale or overflowing sums fall back to Decimal and
// still return ErrOverflow if the final exact result cannot fit in Fixed64.
func SumFixed64(values ...Fixed64) (Fixed64, error) {
	if len(values) == 0 {
		return Fixed64{}, ErrEmptyInput
	}
	if total, ok := sumFixed64Fast(values); ok {
		return total, nil
	}
	total := sumFixed64Decimal(values)
	return Fixed64FromDecimal(total, total.Scale(), TowardZero)
}

// AvgFixed64 returns the average of values rounded to scale using mode.
func AvgFixed64(values []Fixed64, scale int32, mode RoundingMode) (Fixed64, error) {
	if len(values) == 0 {
		return Fixed64{}, ErrEmptyInput
	}
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Fixed64{}, ErrInvalidRoundingMode
	}

	if total, ok := sumFixed64Fast(values); ok {
		if avg, ok := avgFixed64Fast(total, len(values), scale, mode); ok {
			return avg, nil
		}
	}
	total := sumFixed64Decimal(values)
	avg, err := total.Div(NewFromInt(int64(len(values))), scale, mode)
	if err != nil {
		return Fixed64{}, err
	}
	return Fixed64FromDecimal(avg, scale, TowardZero)
}

// AvgFixed64Exact returns the exact finite average of values at scale. If the
// quotient repeats or does not fit scale, ErrInexact is returned.
func AvgFixed64Exact(values []Fixed64, scale int32) (Fixed64, error) {
	if len(values) == 0 {
		return Fixed64{}, ErrEmptyInput
	}
	if !validFixed64Scale(scale) {
		return Fixed64{}, ErrInvalidScale
	}

	var total Decimal
	if fast, ok := sumFixed64Fast(values); ok {
		total = fast.Decimal()
	} else {
		total = sumFixed64Decimal(values)
	}
	avg, err := total.DivExact(NewFromInt(int64(len(values))))
	if err != nil {
		return Fixed64{}, err
	}
	scaled, err := avg.RescaleExact(scale)
	if err != nil {
		return Fixed64{}, err
	}
	return Fixed64FromDecimal(scaled, scale, TowardZero)
}

// Key returns a canonical comparable representation suitable for map keys.
func (f Fixed64) Key() string { return f.Decimal().Key() }

// MarshalText implements encoding.TextMarshaler.
func (f Fixed64) MarshalText() ([]byte, error) { return f.appendText(make([]byte, 0, 32)), nil }

// AppendText appends f's text representation to dst.
func (f Fixed64) AppendText(dst []byte) ([]byte, error) { return f.appendText(dst), nil }

// Format implements fmt.Formatter.
func (f Fixed64) Format(s fmt.State, verb rune) {
	text := f.String()
	if verb == 'f' || verb == 'F' {
		if scale, ok, err := formatScale(s); err != nil {
			writeFormatError(s, verb, "qdecimal.Fixed64", text, err)
			return
		} else if ok {
			rounded, err := f.Decimal().Rescale(scale, ToNearestEven)
			if err == nil {
				text = rounded.String()
			} else {
				writeFormatError(s, verb, "qdecimal.Fixed64", text, err)
				return
			}
		}
	}
	writeFormattedNumber(s, verb, text, "qdecimal.Fixed64", f.Sign())
}

// UnmarshalText implements encoding.TextUnmarshaler and preserves the parsed scale.
func (f *Fixed64) UnmarshalText(text []byte) error {
	if f == nil {
		return fmt.Errorf("qdecimal: UnmarshalText on nil *Fixed64")
	}
	d, err := ParseBytes(text)
	if err != nil {
		return err
	}
	out, err := Fixed64FromDecimal(d, d.Scale(), TowardZero)
	if err != nil {
		return err
	}
	*f = out
	return nil
}

// MarshalJSON emits a precision-preserving JSON string.
func (f Fixed64) MarshalJSON() ([]byte, error) { return json.Marshal(f.String()) }

// UnmarshalJSON accepts a JSON string or number.
func (f *Fixed64) UnmarshalJSON(data []byte) error {
	if f == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *Fixed64")
	}
	if string(data) == "null" {
		return ErrNilValue
	}
	var d Decimal
	if err := d.UnmarshalJSON(data); err != nil {
		return err
	}
	out, err := Fixed64FromDecimal(d, d.Scale(), TowardZero)
	if err != nil {
		return err
	}
	*f = out
	return nil
}

// Scan implements database/sql.Scanner.
func (f *Fixed64) Scan(src any) error {
	if f == nil {
		return fmt.Errorf("qdecimal: Scan on nil *Fixed64")
	}
	d, err := scanSource(src)
	if err != nil {
		return err
	}
	out, err := Fixed64FromDecimal(d, d.Scale(), TowardZero)
	if err != nil {
		return err
	}
	*f = out
	return nil
}

// Value implements database/sql/driver.Valuer.
func (f Fixed64) Value() (driver.Value, error) { return f.String(), nil }

func validFixed64Scale(scale int32) bool {
	return scale >= 0 && scale <= int32(len(fixedPow10)-1)
}

func absInt64Magnitude(v int64) uint64 {
	if v >= 0 {
		return uint64(v)
	}
	return uint64(-(v + 1)) + 1
}

func parseFixed64Fast(s string, scale int32, mode RoundingMode) (Fixed64, bool, error) {
	if s == "" {
		return Fixed64{}, true, ErrInvalidSyntax
	}

	negative := false
	pos := 0
	switch s[0] {
	case '-':
		negative = true
		pos = 1
	case '+':
		pos = 1
	}
	if pos == len(s) {
		return Fixed64{}, true, ErrInvalidSyntax
	}

	limit := uint64(math.MaxInt64)
	if negative {
		limit++
	}
	magnitude := uint64(0)
	sawDigit := false
	decimalSeen := false
	fracDigits := int32(0)
	firstDiscard := byte(0)
	discardSeen := false
	anyMoreDiscard := false
	anyDiscardNonzero := false

	for ; pos < len(s); pos++ {
		c := s[pos]
		if c == '.' {
			if decimalSeen {
				return Fixed64{}, true, ErrInvalidSyntax
			}
			decimalSeen = true
			continue
		}
		if c < '0' || c > '9' {
			return Fixed64{}, false, nil
		}

		sawDigit = true
		digit := c - '0'
		if !decimalSeen {
			var ok bool
			magnitude, ok = appendFixed64Digit(magnitude, digit, limit)
			if !ok {
				return Fixed64{}, false, nil
			}
			continue
		}

		fracDigits++
		if fracDigits <= scale {
			var ok bool
			magnitude, ok = appendFixed64Digit(magnitude, digit, limit)
			if !ok {
				return Fixed64{}, false, nil
			}
			continue
		}
		if !discardSeen {
			firstDiscard = digit
			discardSeen = true
		} else if digit != 0 {
			anyMoreDiscard = true
		}
		if digit != 0 {
			anyDiscardNonzero = true
		}
	}
	if !sawDigit || (decimalSeen && fracDigits == 0) {
		return Fixed64{}, true, ErrInvalidSyntax
	}

	for fracDigits < scale {
		var ok bool
		magnitude, ok = appendFixed64Digit(magnitude, 0, limit)
		if !ok {
			return Fixed64{}, false, nil
		}
		fracDigits++
	}

	if shouldIncrementFixed64(magnitude, discardSeen, firstDiscard, anyMoreDiscard, anyDiscardNonzero, negative, mode) {
		if magnitude == limit {
			return Fixed64{}, true, ErrOverflow
		}
		magnitude++
	}
	units, err := signedFixed64Units(magnitude, negative)
	if err != nil {
		return Fixed64{}, true, err
	}
	return Fixed64{units: units, scale: scale}, true, nil
}

func appendFixed64Digit(magnitude uint64, digit byte, limit uint64) (uint64, bool) {
	if magnitude > (limit-uint64(digit))/10 {
		return 0, false
	}
	return magnitude*10 + uint64(digit), true
}

func shouldIncrementFixed64(magnitude uint64, discardSeen bool, firstDiscard byte, anyMoreDiscard, anyDiscardNonzero, negative bool, mode RoundingMode) bool {
	if !discardSeen {
		return false
	}
	switch mode {
	case AwayFromZero:
		return anyDiscardNonzero
	case TowardPositive:
		return !negative && anyDiscardNonzero
	case TowardNegative:
		return negative && anyDiscardNonzero
	case ToNearestAway:
		return firstDiscard > 5 || firstDiscard == 5
	case ToNearestTowardZero:
		return firstDiscard > 5 || (firstDiscard == 5 && anyMoreDiscard)
	case ToNearestEven:
		return firstDiscard > 5 || (firstDiscard == 5 && (anyMoreDiscard || magnitude%2 == 1))
	default:
		return false
	}
}

func signedFixed64Units(magnitude uint64, negative bool) (int64, error) {
	if negative {
		limit := uint64(math.MaxInt64) + 1
		if magnitude > limit {
			return 0, ErrOverflow
		}
		if magnitude == limit {
			return math.MinInt64, nil
		}
		return -int64(magnitude), nil
	}
	if magnitude > uint64(math.MaxInt64) {
		return 0, ErrOverflow
	}
	return int64(magnitude), nil
}

func alignFixed64(a, b Fixed64) (int64, int64, int32, error) {
	switch {
	case a.scale == b.scale:
		return a.units, b.units, a.scale, nil
	case a.scale > b.scale:
		right, ok := scaleUnits64(b.units, a.scale-b.scale)
		if !ok {
			return 0, 0, 0, ErrOverflow
		}
		return a.units, right, a.scale, nil
	default:
		left, ok := scaleUnits64(a.units, b.scale-a.scale)
		if !ok {
			return 0, 0, 0, ErrOverflow
		}
		return left, b.units, b.scale, nil
	}
}

func sumFixed64Fast(values []Fixed64) (Fixed64, bool) {
	if len(values) == 0 {
		return Fixed64{}, false
	}
	scale := values[0].scale
	units := values[0].units
	for _, value := range values[1:] {
		if value.scale != scale {
			return Fixed64{}, false
		}
		var ok bool
		units, ok = checkedAdd64(units, value.units)
		if !ok {
			return Fixed64{}, false
		}
	}
	return Fixed64{units: units, scale: scale}, true
}

func sumFixed64Decimal(values []Fixed64) Decimal {
	total := values[0].Decimal()
	for _, value := range values[1:] {
		total = total.Add(value.Decimal())
	}
	return total
}

func avgFixed64Fast(total Fixed64, count int, scale int32, mode RoundingMode) (Fixed64, bool) {
	numerator := total.units
	divisor := int64(count)
	switch {
	case scale > total.scale:
		var ok bool
		numerator, ok = scaleUnits64(numerator, scale-total.scale)
		if !ok {
			return Fixed64{}, false
		}
	case scale < total.scale:
		var ok bool
		divisor, ok = scaleUnits64(divisor, total.scale-scale)
		if !ok {
			return Fixed64{}, false
		}
	}
	out, err := divRoundFixed64Units(numerator, divisor, scale, mode)
	return out, err == nil
}

func divRoundFixed64Units(numerator, divisor int64, scale int32, mode RoundingMode) (Fixed64, error) {
	if divisor <= 0 {
		return Fixed64{}, ErrOverflow
	}
	q := numerator / divisor
	r := numerator % divisor
	if r == 0 || mode == TowardZero {
		return Fixed64{units: q, scale: scale}, nil
	}

	sign := 1
	if numerator < 0 {
		sign = -1
	}
	absR := absInt64Magnitude(r)
	absDivisor := uint64(divisor)
	increment := false

	switch mode {
	case AwayFromZero:
		increment = true
	case TowardPositive:
		increment = sign > 0
	case TowardNegative:
		increment = sign < 0
	case ToNearestAway, ToNearestTowardZero, ToNearestEven:
		twiceR := absR * 2
		switch {
		case twiceR > absDivisor:
			increment = true
		case twiceR == absDivisor:
			switch mode {
			case ToNearestAway:
				increment = true
			case ToNearestEven:
				increment = q%2 != 0
			}
		}
	}
	if !increment {
		return Fixed64{units: q, scale: scale}, nil
	}
	delta := int64(1)
	if sign < 0 {
		delta = -1
	}
	units, ok := checkedAdd64(q, delta)
	if !ok {
		return Fixed64{}, ErrOverflow
	}
	return Fixed64{units: units, scale: scale}, nil
}

func fixed64QuantizeStepFast(value, step Fixed64, mode RoundingMode) (Fixed64, bool) {
	stepUnits := step.units
	if stepUnits < 0 {
		if stepUnits == math.MinInt64 {
			return Fixed64{}, false
		}
		stepUnits = -stepUnits
	}

	numerator := value.units
	divisor := stepUnits
	switch {
	case step.scale > value.scale:
		var ok bool
		numerator, ok = scaleUnits64(numerator, step.scale-value.scale)
		if !ok {
			return Fixed64{}, false
		}
	case step.scale < value.scale:
		var ok bool
		divisor, ok = scaleUnits64(divisor, value.scale-step.scale)
		if !ok {
			return Fixed64{}, false
		}
	}

	quotient, err := divRoundFixed64Units(numerator, divisor, 0, mode)
	if err != nil {
		return Fixed64{}, false
	}
	units, ok := checkedMul64(quotient.units, stepUnits)
	if !ok {
		return Fixed64{}, false
	}
	return Fixed64{units: units, scale: step.scale}, true
}

func fixed64QuantizeStepDecimal(value, step Fixed64, mode RoundingMode) (Fixed64, error) {
	stepDecimal := step.Decimal()
	if stepDecimal.Sign() < 0 {
		stepDecimal = stepDecimal.Abs()
	}
	out, err := value.Decimal().QuantizeStep(stepDecimal, mode)
	if err != nil {
		return Fixed64{}, err
	}
	return Fixed64FromDecimal(out, step.scale, TowardZero)
}

func scaleUnits64(units int64, delta int32) (int64, bool) {
	if delta < 0 || delta >= int32(len(fixedPow10)) {
		return 0, false
	}
	return checkedMul64(units, fixedPow10[delta])
}

func checkedAdd64(a, b int64) (int64, bool) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, false
	}
	return a + b, true
}

func checkedMul64(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if a == math.MinInt64 && b == -1 {
		return 0, false
	}
	if b == math.MinInt64 && a == -1 {
		return 0, false
	}
	result := a * b
	if result/b != a {
		return 0, false
	}
	return result, true
}
