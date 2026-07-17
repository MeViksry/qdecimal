package qdecimal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode"
)

// Money couples an exact Decimal amount with a normalized currency or asset code.
//
// Money prevents accidental arithmetic across currencies. It does not embed an
// ISO-4217 table; callers choose scale and rounding policies explicitly so the
// library does not ship stale monetary metadata.
type Money struct {
	amount   Decimal
	currency string
}

// NewMoney creates a Money value and normalizes the currency code.
func NewMoney(amount Decimal, currency string) (Money, error) {
	code, err := NormalizeCurrency(currency)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount.copy(), currency: code}, nil
}

// NewMoneyFromMinorUnits creates Money from integer minor units and an explicit scale.
func NewMoneyFromMinorUnits(units int64, scale int32, currency string) (Money, error) {
	amount, err := NewFromMinorUnits(units, scale)
	if err != nil {
		return Money{}, err
	}
	return NewMoney(amount, currency)
}

// ParseMoney parses canonical "CODE amount" text.
//
// Example: ParseMoney("USD 123.45").
func ParseMoney(text string) (Money, error) {
	return parseMoneyText(text)
}

// MustParseMoney is for tests and package-level initialization. It panics only
// when explicitly requested by the caller.
func MustParseMoney(text string) Money {
	m, err := ParseMoney(text)
	if err != nil {
		panic(err)
	}
	return m
}

// NormalizeCurrency returns an uppercase currency/asset code.
//
// Codes must be 3 to 12 ASCII letters or digits. This covers fiat codes such as
// USD and IDR, plus common exchange asset codes such as BTC, ETH, USDT, and USDC.
func NormalizeCurrency(currency string) (string, error) {
	code := strings.ToUpper(strings.TrimSpace(currency))
	if len(code) < 3 || len(code) > 12 {
		return "", ErrInvalidCurrency
	}
	for _, r := range code {
		if r > unicode.MaxASCII || !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return "", ErrInvalidCurrency
		}
	}
	return code, nil
}

// Amount returns a defensive copy of the decimal amount.
func (m Money) Amount() Decimal { return m.amount.copy() }

// Currency returns the normalized currency or asset code.
func (m Money) Currency() string { return m.currency }

// String returns a human-readable money representation.
func (m Money) String() string { return m.currency + " " + m.amount.String() }

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool { return m.amount.IsZero() }

// Sign returns -1, 0, or +1 for the amount.
func (m Money) Sign() int { return m.amount.Sign() }

// Neg returns -m.
func (m Money) Neg() Money {
	return Money{amount: m.amount.Neg(), currency: m.currency}
}

// Abs returns |m|.
func (m Money) Abs() Money {
	return Money{amount: m.amount.Abs(), currency: m.currency}
}

// Equal reports whether amount and currency are equal.
func (m Money) Equal(other Money) bool {
	return m.currency == other.currency && m.amount.Equal(other.amount)
}

// Key returns a canonical comparable representation suitable for map keys.
func (m Money) Key() string {
	if err := m.validCurrency(); err != nil {
		return ""
	}
	return m.currency + " " + m.amount.Key()
}

// Cmp compares two money values with the same currency.
func (m Money) Cmp(other Money) (int, error) {
	if err := m.sameCurrency(other); err != nil {
		return 0, err
	}
	return m.amount.Cmp(other.amount), nil
}

// Between reports whether m is inside [min, max] when inclusive is true, or
// inside (min, max) when inclusive is false. Reversed bounds are accepted.
func (m Money) Between(min, max Money, inclusive bool) (bool, error) {
	if err := m.sameCurrency(min); err != nil {
		return false, err
	}
	if err := m.sameCurrency(max); err != nil {
		return false, err
	}
	if min.amount.Cmp(max.amount) > 0 {
		min, max = max, min
	}
	lower := m.amount.Cmp(min.amount)
	upper := m.amount.Cmp(max.amount)
	if inclusive {
		return lower >= 0 && upper <= 0, nil
	}
	return lower > 0 && upper < 0, nil
}

// Clamp constrains m to [min, max]. Reversed bounds are accepted.
func (m Money) Clamp(min, max Money) (Money, error) {
	if err := m.sameCurrency(min); err != nil {
		return Money{}, err
	}
	if err := m.sameCurrency(max); err != nil {
		return Money{}, err
	}
	if min.amount.Cmp(max.amount) > 0 {
		min, max = max, min
	}
	if m.amount.Cmp(min.amount) < 0 {
		return min.copy(), nil
	}
	if m.amount.Cmp(max.amount) > 0 {
		return max.copy(), nil
	}
	return m.copy(), nil
}

// MinMoney returns the smallest money value. All values must use one currency.
func MinMoney(values ...Money) (Money, error) {
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	min := values[0]
	if err := min.validCurrency(); err != nil {
		return Money{}, err
	}
	for _, value := range values[1:] {
		if err := min.sameCurrency(value); err != nil {
			return Money{}, err
		}
		if value.amount.Cmp(min.amount) < 0 {
			min = value
		}
	}
	return min.copy(), nil
}

// MaxMoney returns the largest money value. All values must use one currency.
func MaxMoney(values ...Money) (Money, error) {
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	max := values[0]
	if err := max.validCurrency(); err != nil {
		return Money{}, err
	}
	for _, value := range values[1:] {
		if err := max.sameCurrency(value); err != nil {
			return Money{}, err
		}
		if value.amount.Cmp(max.amount) > 0 {
			max = value
		}
	}
	return max.copy(), nil
}

// Add returns m + other. It fails if currencies differ.
func (m Money) Add(other Money) (Money, error) {
	if err := m.sameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{amount: m.amount.Add(other.amount), currency: m.currency}, nil
}

// Sub returns m - other. It fails if currencies differ.
func (m Money) Sub(other Money) (Money, error) {
	if err := m.sameCurrency(other); err != nil {
		return Money{}, err
	}
	return Money{amount: m.amount.Sub(other.amount), currency: m.currency}, nil
}

// Mul multiplies m by factor and rounds the result to scale using mode.
func (m Money) Mul(factor Decimal, scale int32, mode RoundingMode) (Money, error) {
	if err := m.validCurrency(); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Mul(factor).Rescale(scale, mode)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: m.currency}, nil
}

// Div divides m by divisor and rounds the result to scale using mode.
func (m Money) Div(divisor Decimal, scale int32, mode RoundingMode) (Money, error) {
	if err := m.validCurrency(); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Div(divisor, scale, mode)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: m.currency}, nil
}

// Round rounds m's amount to scale using mode.
func (m Money) Round(scale int32, mode RoundingMode) (Money, error) {
	if err := m.validCurrency(); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Rescale(scale, mode)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: m.currency}, nil
}

// RoundExact changes m's amount to scale only when no non-zero digit would be
// lost.
func (m Money) RoundExact(scale int32) (Money, error) {
	if err := m.validCurrency(); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.RescaleExact(scale)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: m.currency}, nil
}

// QuantizeStep rounds m's amount to a valid increment, such as an exchange tick.
func (m Money) QuantizeStep(step Decimal, mode RoundingMode) (Money, error) {
	if err := m.validCurrency(); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.QuantizeStep(step, mode)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: m.currency}, nil
}

// QuantizeStepExact changes m's amount to step's scale only when it is already
// an exact multiple of step.
func (m Money) QuantizeStepExact(step Decimal) (Money, error) {
	if err := m.validCurrency(); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.QuantizeStepExact(step)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: m.currency}, nil
}

// MinorUnits returns m rounded to scale as integer minor units.
func (m Money) MinorUnits(scale int32, mode RoundingMode) (*big.Int, error) {
	if err := m.validCurrency(); err != nil {
		return nil, err
	}
	return m.amount.MinorUnits(scale, mode)
}

// MinorUnitsExact returns m as integer minor units only when no non-zero digit
// would be discarded at scale.
func (m Money) MinorUnitsExact(scale int32) (*big.Int, error) {
	if err := m.validCurrency(); err != nil {
		return nil, err
	}
	return m.amount.MinorUnitsExact(scale)
}

// Int64MinorUnits is like MinorUnits but fails if the result does not fit int64.
func (m Money) Int64MinorUnits(scale int32, mode RoundingMode) (int64, error) {
	if err := m.validCurrency(); err != nil {
		return 0, err
	}
	return m.amount.Int64MinorUnits(scale, mode)
}

// Int64MinorUnitsExact is like MinorUnitsExact but fails if the result does not
// fit int64.
func (m Money) Int64MinorUnitsExact(scale int32) (int64, error) {
	if err := m.validCurrency(); err != nil {
		return 0, err
	}
	return m.amount.Int64MinorUnitsExact(scale)
}

// Allocate splits m into parts at scale while preserving the rounded total.
//
// Remainder minor units are distributed from the first part forward. Negative
// values distribute negative remainders the same way, preserving exact totals.
func (m Money) Allocate(parts int, scale int32, mode RoundingMode) ([]Money, error) {
	if err := m.validCurrency(); err != nil {
		return nil, err
	}
	if parts <= 0 {
		return nil, ErrInvalidAllocation
	}
	ratios := make([]int64, parts)
	for i := range ratios {
		ratios[i] = 1
	}
	return m.AllocateRatios(ratios, scale, mode)
}

// AllocateRatios splits m according to non-negative ratios at scale while
// preserving the rounded total.
func (m Money) AllocateRatios(ratios []int64, scale int32, mode RoundingMode) ([]Money, error) {
	if err := m.validCurrency(); err != nil {
		return nil, err
	}
	if len(ratios) == 0 {
		return nil, ErrInvalidAllocation
	}
	totalRatio := int64(0)
	for _, ratio := range ratios {
		if ratio < 0 {
			return nil, ErrInvalidAllocation
		}
		totalRatio += ratio
		if totalRatio < 0 {
			return nil, ErrOverflow
		}
	}
	if totalRatio == 0 {
		return nil, ErrInvalidAllocation
	}

	totalUnits, err := m.amount.MinorUnits(scale, mode)
	if err != nil {
		return nil, err
	}
	total := big.NewInt(totalRatio)
	shares := make([]*big.Int, len(ratios))
	remainders := make([]allocationRemainder, 0, len(ratios))
	sumShares := new(big.Int)

	for i, ratio := range ratios {
		product := new(big.Int).Mul(totalUnits, big.NewInt(ratio))
		quotient := new(big.Int)
		remainder := new(big.Int)
		quotient.QuoRem(product, total, remainder)
		shares[i] = quotient
		sumShares.Add(sumShares, quotient)
		if ratio != 0 && remainder.Sign() != 0 {
			remainders = append(remainders, allocationRemainder{
				index:     i,
				magnitude: absInt(remainder),
			})
		}
	}

	leftover := new(big.Int).Sub(totalUnits, sumShares)
	leftoverCount := int(absInt(leftover).Int64())
	if leftoverCount > 0 {
		sort.SliceStable(remainders, func(i, j int) bool {
			cmp := remainders[i].magnitude.Cmp(remainders[j].magnitude)
			if cmp == 0 {
				return remainders[i].index < remainders[j].index
			}
			return cmp > 0
		})
		if leftoverCount > len(remainders) {
			return nil, ErrInvalidAllocation
		}
		delta := big.NewInt(1)
		if leftover.Sign() < 0 {
			delta.SetInt64(-1)
		}
		for i := 0; i < leftoverCount; i++ {
			shares[remainders[i].index].Add(shares[remainders[i].index], delta)
		}
	}

	out := make([]Money, len(shares))
	for i, share := range shares {
		amount, err := NewFromBigInt(share, scale)
		if err != nil {
			return nil, err
		}
		out[i] = Money{amount: amount, currency: m.currency}
	}
	return out, nil
}

// MarshalText implements encoding.TextMarshaler.
func (m Money) MarshalText() ([]byte, error) {
	return m.AppendText(make([]byte, 0, len(m.currency)+1+m.amount.textCapacity()))
}

// AppendText appends m's canonical "CODE amount" text representation to dst.
func (m Money) AppendText(dst []byte) ([]byte, error) {
	if err := m.validCurrency(); err != nil {
		return nil, err
	}
	dst = append(dst, m.currency...)
	dst = append(dst, ' ')
	return m.amount.AppendText(dst)
}

// Format implements fmt.Formatter.
func (m Money) Format(s fmt.State, verb rune) {
	if err := m.validCurrency(); err != nil {
		writeFormattedType(s, verb, fmt.Sprintf("<invalid money: %v>", err), "qdecimal.Money")
		return
	}
	amountText := m.amount.String()
	if verb == 'f' || verb == 'F' {
		if scale, ok, err := formatScale(s); err != nil {
			writeFormatError(s, verb, "qdecimal.Money", m.String(), err)
			return
		} else if ok {
			rounded, err := m.amount.Rescale(scale, ToNearestEven)
			if err == nil {
				amountText = rounded.String()
			} else {
				writeFormatError(s, verb, "qdecimal.Money", m.String(), err)
				return
			}
		}
	}
	writeFormattedMoney(s, verb, m.currency, amountText, m.amount.Sign())
}

// UnmarshalText implements encoding.TextUnmarshaler for "CODE amount" text.
func (m *Money) UnmarshalText(text []byte) error {
	if m == nil {
		return fmt.Errorf("qdecimal: UnmarshalText on nil *Money")
	}
	out, err := parseMoneyText(string(text))
	if err != nil {
		return err
	}
	*m = out
	return nil
}

// Scan implements database/sql.Scanner using the canonical "CODE amount" text format.
func (m *Money) Scan(src any) error {
	if m == nil {
		return fmt.Errorf("qdecimal: Scan on nil *Money")
	}
	out, err := scanMoneySource(src)
	if err != nil {
		return err
	}
	*m = out
	return nil
}

// Value implements database/sql/driver.Valuer using the canonical "CODE amount" text format.
func (m Money) Value() (driver.Value, error) {
	text, err := m.MarshalText()
	if err != nil {
		return nil, err
	}
	return string(text), nil
}

// MarshalJSON emits {"amount":"...","currency":"..."}.
func (m Money) MarshalJSON() ([]byte, error) {
	if err := m.validCurrency(); err != nil {
		return nil, err
	}
	return json.Marshal(moneyJSON{
		Amount:   m.amount,
		Currency: m.currency,
	})
}

// UnmarshalJSON accepts {"amount":"...","currency":"..."}.
func (m *Money) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *Money")
	}
	var payload moneyJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	out, err := NewMoney(payload.Amount, payload.Currency)
	if err != nil {
		return err
	}
	*m = out
	return nil
}

func (m Money) sameCurrency(other Money) error {
	if err := m.validCurrency(); err != nil {
		return err
	}
	if err := other.validCurrency(); err != nil {
		return err
	}
	if m.currency != other.currency {
		return ErrCurrencyMismatch
	}
	return nil
}

func (m Money) copy() Money {
	return Money{amount: m.amount.copy(), currency: m.currency}
}

func (m Money) validCurrency() error {
	if len(m.currency) < 3 || len(m.currency) > 12 {
		return ErrInvalidCurrency
	}
	for i := 0; i < len(m.currency); i++ {
		c := m.currency[i]
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return ErrInvalidCurrency
		}
	}
	return nil
}

type moneyJSON struct {
	Amount   Decimal `json:"amount"`
	Currency string  `json:"currency"`
}

type allocationRemainder struct {
	index     int
	magnitude *big.Int
}

func scanMoneySource(src any) (Money, error) {
	switch value := src.(type) {
	case nil:
		return Money{}, ErrNilValue
	case Money:
		if err := value.validCurrency(); err != nil {
			return Money{}, err
		}
		return Money{amount: value.amount.copy(), currency: value.currency}, nil
	case string:
		return parseMoneyText(value)
	case []byte:
		return parseMoneyText(string(value))
	default:
		return Money{}, fmt.Errorf("%w: %T", ErrInvalidSource, src)
	}
}

func parseMoneyText(text string) (Money, error) {
	fields := strings.Fields(text)
	if len(fields) != 2 {
		return Money{}, ErrInvalidSyntax
	}
	amount, err := Parse(fields[1])
	if err != nil {
		return Money{}, err
	}
	return NewMoney(amount, fields[0])
}
