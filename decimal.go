package qdecimal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
)

const maxScale = math.MaxInt32
const smallPow10Limit = 38

// Decimal represents coef * 10^-scale.
//
// Decimal has no NaN or infinity state. Non-finite values are rejected at input
// boundaries so finance code cannot silently propagate invalid amounts.
type Decimal struct {
	coef  big.Int
	scale int32
}

var (
	Zero = NewFromInt(0)
	One  = NewFromInt(1)
	Ten  = NewFromInt(10)

	smallPow10 = initSmallPow10()
)

// New creates a Decimal from an integer coefficient and non-negative scale.
func New(coef int64, scale int32) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	var d Decimal
	d.coef.SetInt64(coef)
	d.scale = scale
	return d.canonicalZero(), nil
}

// NewFromInt creates an integer Decimal.
func NewFromInt(v int64) Decimal {
	var d Decimal
	d.coef.SetInt64(v)
	return d
}

// NewFromUint64 creates an integer Decimal from an unsigned value.
func NewFromUint64(v uint64) Decimal {
	var d Decimal
	d.coef.SetUint64(v)
	return d
}

// NewFromBigInt creates a Decimal from a coefficient copy and non-negative scale.
func NewFromBigInt(coef *big.Int, scale int32) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if coef == nil {
		coef = new(big.Int)
	}
	var d Decimal
	d.coef.Set(coef)
	d.scale = scale
	return d.canonicalZero(), nil
}

// FromFloat64 converts a finite float through Go's shortest round-trip decimal
// representation. Prefer Parse or integer minor-unit constructors in finance
// code; this method is explicit because binary floats are not decimal inputs.
func FromFloat64(v float64) (Decimal, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Decimal{}, ErrNonFiniteFloat
	}
	return Parse(strconv.FormatFloat(v, 'g', -1, 64))
}

// NewFromFloat is a compatibility alias for FromFloat64.
//
// It returns ErrNonFiniteFloat for NaN and infinity instead of panicking.
func NewFromFloat(v float64) (Decimal, error) {
	return FromFloat64(v)
}

// NewFromFloatWithScale converts a finite float and immediately rounds it to
// scale using mode.
//
// This keeps float boundaries explicit: binary floats are accepted only at an
// integration edge, and any decimal rounding policy is supplied by the caller.
func NewFromFloatWithScale(v float64, scale int32, mode RoundingMode) (Decimal, error) {
	d, err := FromFloat64(v)
	if err != nil {
		return Decimal{}, err
	}
	return d.Rescale(scale, mode)
}

// FromBigFloat rounds f to scale using mode.
func FromBigFloat(f *big.Float, scale int32, mode RoundingMode) (Decimal, error) {
	if f == nil {
		return Decimal{}, ErrNilValue
	}
	if f.IsInf() {
		return Decimal{}, ErrNonFiniteFloat
	}
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Decimal{}, ErrInvalidRoundingMode
	}
	rat, _ := f.Rat(nil)
	return fromRat(rat, scale, mode)
}

// FromRat rounds r to scale using mode.
func FromRat(r *big.Rat, scale int32, mode RoundingMode) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Decimal{}, ErrInvalidRoundingMode
	}
	return fromRat(r, scale, mode)
}

// MustParse is for tests and package-level initialization. It panics only when
// explicitly requested by the caller.
func MustParse(s string) Decimal {
	d, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

// NewFromString is a compatibility alias for Parse.
func NewFromString(s string) (Decimal, error) {
	return Parse(s)
}

// RequireFromString is an alias for MustParse.
func RequireFromString(s string) Decimal {
	return MustParse(s)
}

// Scale returns the number of fractional decimal digits preserved by d.
func (d Decimal) Scale() int32 { return d.scale }

// Coefficient returns a defensive copy of d's unscaled integer coefficient.
func (d Decimal) Coefficient() *big.Int { return new(big.Int).Set(&d.coef) }

// Rat returns an exact rational copy of d.
func (d Decimal) Rat() *big.Rat {
	return new(big.Rat).SetFrac(new(big.Int).Set(&d.coef), pow10(d.scale))
}

// Sign returns -1, 0, or +1.
func (d Decimal) Sign() int { return d.coef.Sign() }

// IsZero reports whether d is numerically zero.
func (d Decimal) IsZero() bool { return d.coef.Sign() == 0 }

// IsInteger reports whether d has no non-zero fractional digits.
func (d Decimal) IsInteger() bool {
	if d.scale == 0 || d.IsZero() {
		return true
	}
	div := pow10(d.scale)
	var rem big.Int
	rem.Rem(absInt(&d.coef), div)
	return rem.Sign() == 0
}

// Neg returns -d.
func (d Decimal) Neg() Decimal {
	var out Decimal
	out.coef.Neg(&d.coef)
	out.scale = d.scale
	return out.canonicalZero()
}

// Abs returns |d|.
func (d Decimal) Abs() Decimal {
	var out Decimal
	out.coef.Abs(&d.coef)
	out.scale = d.scale
	return out.canonicalZero()
}

// Normalize removes insignificant trailing fractional zeros.
func (d Decimal) Normalize() Decimal {
	if d.coef.Sign() == 0 {
		d.scale = 0
		return d
	}
	ten := big.NewInt(10)
	for d.scale > 0 {
		var q, r big.Int
		q.QuoRem(&d.coef, ten, &r)
		if r.Sign() != 0 {
			break
		}
		d.coef = q
		d.scale--
	}
	return d
}

// Add returns d + other exactly.
func (d Decimal) Add(other Decimal) Decimal {
	left, right, scale := align(d, other)
	var out Decimal
	out.coef.Add(left, right)
	out.scale = scale
	return out.canonicalZero()
}

// Sub returns d - other exactly.
func (d Decimal) Sub(other Decimal) Decimal {
	left, right, scale := align(d, other)
	var out Decimal
	out.coef.Sub(left, right)
	out.scale = scale
	return out.canonicalZero()
}

// Mul returns d * other exactly.
func (d Decimal) Mul(other Decimal) Decimal {
	var out Decimal
	out.coef.Mul(&d.coef, &other.coef)
	out.scale = d.scale + other.scale
	return out.canonicalZero()
}

// Div divides d by other and rounds the result to scale using mode.
func (d Decimal) Div(other Decimal, scale int32, mode RoundingMode) (Decimal, error) {
	if other.coef.Sign() == 0 {
		return Decimal{}, ErrDivisionByZero
	}
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Decimal{}, ErrInvalidRoundingMode
	}

	num := new(big.Int).Set(&d.coef)
	den := new(big.Int).Set(&other.coef)
	exp := int64(other.scale) + int64(scale) - int64(d.scale)
	if exp >= 0 {
		num.Mul(num, pow10Int64(exp))
	} else {
		den.Mul(den, pow10Int64(-exp))
	}
	coef := roundQuotient(num, den, mode)
	return Decimal{coef: *coef, scale: scale}.canonicalZero(), nil
}

// DivExact divides d by other and returns an exact finite decimal. If the
// quotient has a repeating decimal expansion, ErrInexact is returned instead of
// rounding.
func (d Decimal) DivExact(other Decimal) (Decimal, error) {
	if other.coef.Sign() == 0 {
		return Decimal{}, ErrDivisionByZero
	}

	num := new(big.Int).Set(&d.coef)
	den := new(big.Int).Set(&other.coef)
	exp := int64(other.scale) - int64(d.scale)
	if exp >= 0 {
		num.Mul(num, pow10Int64(exp))
	} else {
		den.Mul(den, pow10Int64(-exp))
	}
	if den.Sign() < 0 {
		den.Neg(den)
		num.Neg(num)
	}

	gcd := new(big.Int).GCD(nil, nil, absInt(num), den)
	num.Quo(num, gcd)
	den.Quo(den, gcd)

	twos := removeFactor(den, 2)
	fives := removeFactor(den, 5)
	if den.Cmp(big.NewInt(1)) != 0 {
		return Decimal{}, ErrInexact
	}

	scale := twos
	if fives > scale {
		scale = fives
	}
	if scale > maxScale {
		return Decimal{}, ErrInvalidScale
	}
	if twos < scale {
		num.Mul(num, powSmall(2, scale-twos))
	}
	if fives < scale {
		num.Mul(num, powSmall(5, scale-fives))
	}
	return Decimal{coef: *num, scale: int32(scale)}.canonicalZero(), nil
}

// Rescale changes d to scale using mode when digits must be discarded.
func (d Decimal) Rescale(scale int32, mode RoundingMode) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Decimal{}, ErrInvalidRoundingMode
	}
	if scale == d.scale {
		return d.copy(), nil
	}
	out := d.copy()
	if scale > d.scale {
		out.coef.Mul(&out.coef, pow10(scale-d.scale))
		out.scale = scale
		return out.canonicalZero(), nil
	}

	divisor := pow10(d.scale - scale)
	coef := roundQuotient(&out.coef, divisor, mode)
	return Decimal{coef: *coef, scale: scale}.canonicalZero(), nil
}

// RescaleExact changes d to scale only when no non-zero digit would be lost.
//
// If reducing scale would require rounding, ErrInexact is returned.
func (d Decimal) RescaleExact(scale int32) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if scale == d.scale {
		return d.copy(), nil
	}
	out := d.copy()
	if scale > d.scale {
		out.coef.Mul(&out.coef, pow10(scale-d.scale))
		out.scale = scale
		return out.canonicalZero(), nil
	}

	divisor := pow10(d.scale - scale)
	var q, r big.Int
	q.QuoRem(&out.coef, divisor, &r)
	if r.Sign() != 0 {
		return Decimal{}, ErrInexact
	}
	return Decimal{coef: q, scale: scale}.canonicalZero(), nil
}

// Round is an alias for Rescale.
func (d Decimal) Round(scale int32, mode RoundingMode) (Decimal, error) {
	return d.Rescale(scale, mode)
}

// Quantize rounds d to the same scale as template.
func (d Decimal) Quantize(template Decimal, mode RoundingMode) (Decimal, error) {
	return d.Rescale(template.scale, mode)
}

// QuantizeExact changes d to template's scale only when no non-zero digit would
// be lost.
func (d Decimal) QuantizeExact(template Decimal) (Decimal, error) {
	return d.RescaleExact(template.scale)
}

// QuantizeStep rounds d to the nearest multiple of step using mode.
//
// This is intended for exchange tick sizes and banking increments that are not
// expressible by scale alone, such as 0.05.
func (d Decimal) QuantizeStep(step Decimal, mode RoundingMode) (Decimal, error) {
	if step.IsZero() {
		return Decimal{}, ErrDivisionByZero
	}
	if !mode.valid() {
		return Decimal{}, ErrInvalidRoundingMode
	}
	if step.Sign() < 0 {
		step = step.Abs()
	}
	units, err := d.Div(step, 0, mode)
	if err != nil {
		return Decimal{}, err
	}
	return units.Mul(step).Rescale(step.scale, TowardZero)
}

// QuantizeStepExact changes d to step's scale only when d is already an exact
// multiple of step. ErrInexact is returned instead of rounding.
func (d Decimal) QuantizeStepExact(step Decimal) (Decimal, error) {
	if step.IsZero() {
		return Decimal{}, ErrDivisionByZero
	}
	if step.Sign() < 0 {
		step = step.Abs()
	}
	units, err := d.DivExact(step)
	if err != nil {
		return Decimal{}, err
	}
	if !units.IsInteger() {
		return Decimal{}, ErrInexact
	}
	return units.Mul(step).RescaleExact(step.scale)
}

// Truncate rounds toward zero to scale.
func (d Decimal) Truncate(scale int32) (Decimal, error) {
	return d.Rescale(scale, TowardZero)
}

// Ceil rounds toward +infinity to scale.
func (d Decimal) Ceil(scale int32) (Decimal, error) {
	return d.Rescale(scale, TowardPositive)
}

// Floor rounds toward -infinity to scale.
func (d Decimal) Floor(scale int32) (Decimal, error) {
	return d.Rescale(scale, TowardNegative)
}

// Cmp compares d and other numerically.
func (d Decimal) Cmp(other Decimal) int {
	left, right, _ := align(d, other)
	return left.Cmp(right)
}

// Equal reports numeric equality.
func (d Decimal) Equal(other Decimal) bool { return d.Cmp(other) == 0 }

// Between reports whether d is inside [min, max] when inclusive is true, or
// inside (min, max) when inclusive is false. Reversed bounds are accepted.
func (d Decimal) Between(min, max Decimal, inclusive bool) bool {
	if min.Cmp(max) > 0 {
		min, max = max, min
	}
	lower := d.Cmp(min)
	upper := d.Cmp(max)
	if inclusive {
		return lower >= 0 && upper <= 0
	}
	return lower > 0 && upper < 0
}

// Clamp constrains d to [min, max]. Reversed bounds are accepted.
func (d Decimal) Clamp(min, max Decimal) Decimal {
	if min.Cmp(max) > 0 {
		min, max = max, min
	}
	if d.Cmp(min) < 0 {
		return min.copy()
	}
	if d.Cmp(max) > 0 {
		return max.copy()
	}
	return d.copy()
}

// Min returns the smaller value.
func Min(values ...Decimal) Decimal {
	if len(values) == 0 {
		return Zero
	}
	min := values[0]
	for _, value := range values[1:] {
		if value.Cmp(min) < 0 {
			min = value
		}
	}
	return min.copy()
}

// Max returns the larger value.
func Max(values ...Decimal) Decimal {
	if len(values) == 0 {
		return Zero
	}
	max := values[0]
	for _, value := range values[1:] {
		if value.Cmp(max) > 0 {
			max = value
		}
	}
	return max.copy()
}

// Key returns a canonical comparable representation suitable for map keys.
func (d Decimal) Key() string { return d.Normalize().String() }

// String returns the decimal string while preserving scale, including values
// like 0.00.
func (d Decimal) String() string {
	return string(d.appendText(make([]byte, 0, d.textCapacity())))
}

func (d Decimal) appendText(dst []byte) []byte {
	sign := d.coef.Sign()
	if sign == 0 {
		if d.scale == 0 {
			return append(dst, '0')
		}
		dst = append(dst, '0', '.')
		return appendZeros(dst, int(d.scale))
	}

	digits := d.coef.String()
	if sign < 0 {
		dst = append(dst, '-')
		digits = digits[1:]
	}
	if d.scale == 0 {
		return append(dst, digits...)
	}

	scale := int(d.scale)
	if len(digits) <= scale {
		dst = append(dst, '0', '.')
		dst = appendZeros(dst, scale-len(digits))
		return append(dst, digits...)
	}
	point := len(digits) - scale
	dst = append(dst, digits[:point]...)
	dst = append(dst, '.')
	return append(dst, digits[point:]...)
}

func (d Decimal) textCapacity() int {
	if d.coef.Sign() == 0 {
		if d.scale == 0 {
			return 1
		}
		return int(d.scale) + 2
	}
	digits := int(float64(d.coef.BitLen())*0.30203) + 2
	if d.scale == 0 {
		if d.coef.Sign() < 0 {
			return digits + 1
		}
		return digits
	}
	scale := int(d.scale)
	if digits <= scale {
		return scale + 3
	}
	if d.coef.Sign() < 0 {
		return digits + 2
	}
	return digits + 1
}

// StringFixed rounds d to scale and returns the fixed-scale representation.
func (d Decimal) StringFixed(scale int32, mode RoundingMode) (string, error) {
	rounded, err := d.Rescale(scale, mode)
	if err != nil {
		return "", err
	}
	return rounded.String(), nil
}

// Format implements fmt.Formatter.
func (d Decimal) Format(s fmt.State, verb rune) {
	text := d.String()
	if verb == 'f' || verb == 'F' {
		if scale, ok, err := formatScale(s); err != nil {
			writeFormatError(s, verb, "qdecimal.Decimal", text, err)
			return
		} else if ok {
			rounded, err := d.Rescale(scale, ToNearestEven)
			if err == nil {
				text = rounded.String()
			} else {
				writeFormatError(s, verb, "qdecimal.Decimal", text, err)
				return
			}
		}
	}
	writeFormattedNumber(s, verb, text, "qdecimal.Decimal", d.Sign())
}

// MarshalText implements encoding.TextMarshaler.
func (d Decimal) MarshalText() ([]byte, error) {
	return d.appendText(make([]byte, 0, d.textCapacity())), nil
}

// AppendText appends d's text representation to dst.
func (d Decimal) AppendText(dst []byte) ([]byte, error) { return d.appendText(dst), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Decimal) UnmarshalText(text []byte) error {
	if d == nil {
		return fmt.Errorf("qdecimal: UnmarshalText on nil *Decimal")
	}
	parsed, err := ParseBytes(text)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalJSON emits a JSON string. This avoids lossy float interpretation in
// JavaScript, database gateways, and message buses.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return d.MarshalJSONWithMode(EmitJSONString)
}

// JSONNumber returns a json.Number for systems that explicitly require numeric
// JSON tokens and can preserve arbitrary precision.
func (d Decimal) JSONNumber() json.Number { return json.Number(d.String()) }

// UnmarshalJSON accepts either a JSON string or a JSON number.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	if d == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *Decimal")
	}
	if string(data) == "null" {
		return ErrNilValue
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidSyntax, err)
		}
	} else {
		text = string(data)
	}
	parsed, err := Parse(text)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Scan implements database/sql.Scanner.
func (d *Decimal) Scan(src any) error {
	if d == nil {
		return fmt.Errorf("qdecimal: Scan on nil *Decimal")
	}
	parsed, err := scanSource(src)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Value implements database/sql/driver.Valuer.
func (d Decimal) Value() (driver.Value, error) { return d.String(), nil }

func (d Decimal) copy() Decimal {
	var out Decimal
	out.coef.Set(&d.coef)
	out.scale = d.scale
	return out
}

func (d Decimal) canonicalZero() Decimal {
	if d.coef.Sign() == 0 && d.coef.BitLen() != 0 {
		d.coef.SetInt64(0)
	}
	return d
}

func align(a, b Decimal) (*big.Int, *big.Int, int32) {
	left := new(big.Int).Set(&a.coef)
	right := new(big.Int).Set(&b.coef)
	switch {
	case a.scale > b.scale:
		right.Mul(right, pow10(a.scale-b.scale))
		return left, right, a.scale
	case b.scale > a.scale:
		left.Mul(left, pow10(b.scale-a.scale))
		return left, right, b.scale
	default:
		return left, right, a.scale
	}
}

func fromRat(r *big.Rat, scale int32, mode RoundingMode) (Decimal, error) {
	if r == nil {
		return Decimal{}, ErrNilValue
	}
	num := new(big.Int).Set(r.Num())
	num.Mul(num, pow10(scale))
	coef := roundQuotient(num, r.Denom(), mode)
	return Decimal{coef: *coef, scale: scale}.canonicalZero(), nil
}

func roundQuotient(num, den *big.Int, mode RoundingMode) *big.Int {
	if den.Sign() < 0 {
		num = new(big.Int).Neg(num)
		den = new(big.Int).Neg(den)
	}
	q := new(big.Int)
	r := new(big.Int)
	q.QuoRem(num, den, r)
	if r.Sign() == 0 || mode == TowardZero {
		return q
	}

	sign := num.Sign() * den.Sign()
	absR := absInt(r)
	absDen := absInt(den)
	increment := false

	switch mode {
	case AwayFromZero:
		increment = true
	case TowardPositive:
		increment = sign > 0
	case TowardNegative:
		increment = sign < 0
	case ToNearestAway, ToNearestTowardZero, ToNearestEven:
		twiceR := new(big.Int).Lsh(absR, 1)
		cmp := twiceR.Cmp(absDen)
		if cmp > 0 {
			increment = true
		} else if cmp == 0 {
			switch mode {
			case ToNearestAway:
				increment = true
			case ToNearestEven:
				increment = q.Bit(0) == 1
			}
		}
	}

	if increment {
		if sign >= 0 {
			q.Add(q, big.NewInt(1))
		} else {
			q.Sub(q, big.NewInt(1))
		}
	}
	return q
}

func absInt(v *big.Int) *big.Int {
	if v.Sign() >= 0 {
		return new(big.Int).Set(v)
	}
	return new(big.Int).Neg(v)
}

func pow10(scale int32) *big.Int {
	return pow10Int64(int64(scale))
}

func pow10Int64(scale int64) *big.Int {
	if scale <= 0 {
		return big.NewInt(1)
	}
	if scale <= smallPow10Limit {
		return new(big.Int).Set(&smallPow10[scale])
	}
	result := new(big.Int).Exp(big.NewInt(10), big.NewInt(scale), nil)
	return result
}

func powSmall(base int64, exp int32) *big.Int {
	if exp <= 0 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(big.NewInt(base), big.NewInt(int64(exp)), nil)
}

func removeFactor(value *big.Int, factor int64) int32 {
	divisor := big.NewInt(factor)
	zero := new(big.Int)
	count := int32(0)
	for value.Sign() != 0 {
		var q, r big.Int
		q.QuoRem(value, divisor, &r)
		if r.Cmp(zero) != 0 {
			break
		}
		value.Set(&q)
		count++
	}
	return count
}

func initSmallPow10() [smallPow10Limit + 1]big.Int {
	var table [smallPow10Limit + 1]big.Int
	table[0].SetInt64(1)
	for i := 1; i <= smallPow10Limit; i++ {
		table[i].Mul(&table[i-1], big.NewInt(10))
	}
	return table
}

func zeros(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = '0'
	}
	return string(buf)
}

func appendZeros(dst []byte, n int) []byte {
	for ; n > 0; n-- {
		dst = append(dst, '0')
	}
	return dst
}
