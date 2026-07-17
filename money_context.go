package qdecimal

import (
	"encoding/json"
	"fmt"
)

// MoneyContext is an explicit policy for one currency or asset.
//
// It carries currency, scale, and rounding together so services can pass an
// auditable money policy without package-global precision or currency metadata.
type MoneyContext struct {
	Currency string
	Scale    int32
	Rounding RoundingMode
}

// NewMoneyContext validates and returns a money arithmetic context.
func NewMoneyContext(currency string, scale int32, rounding RoundingMode) (MoneyContext, error) {
	code, err := NormalizeCurrency(currency)
	if err != nil {
		return MoneyContext{}, err
	}
	if scale < 0 {
		return MoneyContext{}, ErrInvalidScale
	}
	if !rounding.valid() {
		return MoneyContext{}, ErrInvalidRoundingMode
	}
	return MoneyContext{Currency: code, Scale: scale, Rounding: rounding}, nil
}

// MustMoneyContext is for package initialization and tests.
func MustMoneyContext(currency string, scale int32, rounding RoundingMode) MoneyContext {
	ctx, err := NewMoneyContext(currency, scale, rounding)
	if err != nil {
		panic(err)
	}
	return ctx
}

// MarshalJSON emits a stable policy object for configuration and audit logs.
func (c MoneyContext) MarshalJSON() ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(moneyContextJSON{
		Currency: c.Currency,
		Scale:    c.Scale,
		Rounding: c.Rounding,
	})
}

// UnmarshalJSON decodes, normalizes, and validates a money policy object.
func (c *MoneyContext) UnmarshalJSON(data []byte) error {
	if c == nil {
		return ErrNilValue
	}
	var payload moneyContextJSON
	if err := decodeStrictJSON(data, &payload); err != nil {
		return err
	}
	out, err := NewMoneyContext(payload.Currency, payload.Scale, payload.Rounding)
	if err != nil {
		return err
	}
	*c = out
	return nil
}

// DecimalContext returns the numeric scale/rounding policy.
func (c MoneyContext) DecimalContext() Context {
	return Context{Scale: c.Scale, Rounding: c.Rounding}
}

// WithScale returns c with a different scale.
func (c MoneyContext) WithScale(scale int32) (MoneyContext, error) {
	return NewMoneyContext(c.Currency, scale, c.Rounding)
}

// WithRounding returns c with a different rounding mode.
func (c MoneyContext) WithRounding(rounding RoundingMode) (MoneyContext, error) {
	return NewMoneyContext(c.Currency, c.Scale, rounding)
}

// Money rounds amount to the context scale and attaches the context currency.
func (c MoneyContext) Money(amount Decimal) (Money, error) {
	if err := c.validate(); err != nil {
		return Money{}, err
	}
	rounded, err := amount.Rescale(c.Scale, c.Rounding)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: rounded, currency: c.Currency}, nil
}

// MoneyExact attaches the context currency only when amount already fits the
// context scale without losing non-zero digits.
func (c MoneyContext) MoneyExact(amount Decimal) (Money, error) {
	if err := c.validate(); err != nil {
		return Money{}, err
	}
	rounded, err := amount.RescaleExact(c.Scale)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: rounded, currency: c.Currency}, nil
}

// Parse parses amount text, rounds it to the context scale, and attaches currency.
func (c MoneyContext) Parse(text string) (Money, error) {
	amount, err := Parse(text)
	if err != nil {
		return Money{}, err
	}
	return c.Money(amount)
}

// ParseFlexible parses human-entry text using ParseFlexible, rounds it, and attaches currency.
func (c MoneyContext) ParseFlexible(text string) (Money, error) {
	amount, err := ParseFlexible(text)
	if err != nil {
		return Money{}, err
	}
	return c.Money(amount)
}

// FromMinorUnits creates money from integer minor units at the context scale.
func (c MoneyContext) FromMinorUnits(units int64) (Money, error) {
	if err := c.validate(); err != nil {
		return Money{}, err
	}
	amount, err := NewFromMinorUnits(units, c.Scale)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: c.Currency}, nil
}

// Quantize rounds money to the context scale after validating currency.
func (c MoneyContext) Quantize(m Money) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Rescale(c.Scale, c.Rounding)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: c.Currency}, nil
}

// QuantizeExact changes money to the context scale without discarding non-zero
// digits.
func (c MoneyContext) QuantizeExact(m Money) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.RescaleExact(c.Scale)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: c.Currency}, nil
}

// Sum returns the exact sum of values rounded to the context scale.
func (c MoneyContext) Sum(values ...Money) (Money, error) {
	if err := c.validate(); err != nil {
		return Money{}, err
	}
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	total := Zero
	for _, value := range values {
		if err := c.sameCurrency(value); err != nil {
			return Money{}, err
		}
		total = total.Add(value.amount)
	}
	return c.Money(total)
}

// SumExact returns the exact sum of values at the context scale, failing with
// ErrInexact if non-zero digits would be discarded.
func (c MoneyContext) SumExact(values ...Money) (Money, error) {
	if err := c.validate(); err != nil {
		return Money{}, err
	}
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	total := Zero
	for _, value := range values {
		if err := c.sameCurrency(value); err != nil {
			return Money{}, err
		}
		total = total.Add(value.amount)
	}
	return c.MoneyExact(total)
}

// Avg returns the average of values rounded to the context scale.
func (c MoneyContext) Avg(values ...Money) (Money, error) {
	if err := c.validate(); err != nil {
		return Money{}, err
	}
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	total := Zero
	for _, value := range values {
		if err := c.sameCurrency(value); err != nil {
			return Money{}, err
		}
		total = total.Add(value.amount)
	}
	amount, err := total.Div(NewFromInt(int64(len(values))), c.Scale, c.Rounding)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: c.Currency}, nil
}

// AvgExact returns the average at the context scale without rounding.
func (c MoneyContext) AvgExact(values ...Money) (Money, error) {
	if err := c.validate(); err != nil {
		return Money{}, err
	}
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	total := Zero
	for _, value := range values {
		if err := c.sameCurrency(value); err != nil {
			return Money{}, err
		}
		total = total.Add(value.amount)
	}
	amount, err := total.DivExact(NewFromInt(int64(len(values))))
	if err != nil {
		return Money{}, err
	}
	return c.MoneyExact(amount)
}

// Between reports whether m is inside the range after all values are quantized
// to the context scale. Reversed bounds are accepted.
func (c MoneyContext) Between(m, min, max Money, inclusive bool) (bool, error) {
	value, err := c.Quantize(m)
	if err != nil {
		return false, err
	}
	lower, err := c.Quantize(min)
	if err != nil {
		return false, err
	}
	upper, err := c.Quantize(max)
	if err != nil {
		return false, err
	}
	return value.Between(lower, upper, inclusive)
}

// Clamp constrains m to [min, max] after quantizing all values to the context
// scale. Reversed bounds are accepted.
func (c MoneyContext) Clamp(m, min, max Money) (Money, error) {
	value, err := c.Quantize(m)
	if err != nil {
		return Money{}, err
	}
	lower, err := c.Quantize(min)
	if err != nil {
		return Money{}, err
	}
	upper, err := c.Quantize(max)
	if err != nil {
		return Money{}, err
	}
	return value.Clamp(lower, upper)
}

// Min returns the smallest value after quantizing all inputs to the context scale.
func (c MoneyContext) Min(values ...Money) (Money, error) {
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	min, err := c.Quantize(values[0])
	if err != nil {
		return Money{}, err
	}
	for _, value := range values[1:] {
		quantized, err := c.Quantize(value)
		if err != nil {
			return Money{}, err
		}
		if quantized.amount.Cmp(min.amount) < 0 {
			min = quantized
		}
	}
	return min.copy(), nil
}

// Max returns the largest value after quantizing all inputs to the context scale.
func (c MoneyContext) Max(values ...Money) (Money, error) {
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	max := Money{}
	for i, value := range values {
		quantized, err := c.Quantize(value)
		if err != nil {
			return Money{}, err
		}
		if i == 0 || quantized.amount.Cmp(max.amount) > 0 {
			max = quantized
		}
	}
	return max.copy(), nil
}

// Add returns a + b rounded to the context scale.
func (c MoneyContext) Add(a, b Money) (Money, error) {
	if err := c.sameCurrency(a); err != nil {
		return Money{}, err
	}
	if err := c.sameCurrency(b); err != nil {
		return Money{}, err
	}
	return c.Money(a.amount.Add(b.amount))
}

// AddExact returns a + b at the context scale, failing with ErrInexact if
// non-zero digits would be discarded.
func (c MoneyContext) AddExact(a, b Money) (Money, error) {
	if err := c.sameCurrency(a); err != nil {
		return Money{}, err
	}
	if err := c.sameCurrency(b); err != nil {
		return Money{}, err
	}
	return c.MoneyExact(a.amount.Add(b.amount))
}

// Sub returns a - b rounded to the context scale.
func (c MoneyContext) Sub(a, b Money) (Money, error) {
	if err := c.sameCurrency(a); err != nil {
		return Money{}, err
	}
	if err := c.sameCurrency(b); err != nil {
		return Money{}, err
	}
	return c.Money(a.amount.Sub(b.amount))
}

// SubExact returns a - b at the context scale, failing with ErrInexact if
// non-zero digits would be discarded.
func (c MoneyContext) SubExact(a, b Money) (Money, error) {
	if err := c.sameCurrency(a); err != nil {
		return Money{}, err
	}
	if err := c.sameCurrency(b); err != nil {
		return Money{}, err
	}
	return c.MoneyExact(a.amount.Sub(b.amount))
}

// Mul multiplies money by factor and rounds to the context scale.
func (c MoneyContext) Mul(m Money, factor Decimal) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	return c.Money(m.amount.Mul(factor))
}

// MulExact multiplies money by factor at the context scale, failing with
// ErrInexact if non-zero digits would be discarded.
func (c MoneyContext) MulExact(m Money, factor Decimal) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	return c.MoneyExact(m.amount.Mul(factor))
}

// Div divides money by divisor and rounds to the context scale.
func (c MoneyContext) Div(m Money, divisor Decimal) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.Div(divisor, c.Scale, c.Rounding)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: c.Currency}, nil
}

// DivExact divides money by divisor at the context scale without rounding.
func (c MoneyContext) DivExact(m Money, divisor Decimal) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.DivExact(divisor)
	if err != nil {
		return Money{}, err
	}
	return c.MoneyExact(amount)
}

// QuantizeStep rounds money to a valid increment, then to the context scale.
func (c MoneyContext) QuantizeStep(m Money, step Decimal) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.QuantizeStep(step, c.Rounding)
	if err != nil {
		return Money{}, err
	}
	return c.Money(amount)
}

// QuantizeStepExact changes money to the context scale only when it is already
// an exact multiple of step and no non-zero digits would be discarded.
func (c MoneyContext) QuantizeStepExact(m Money, step Decimal) (Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return Money{}, err
	}
	amount, err := m.amount.QuantizeStepExact(step)
	if err != nil {
		return Money{}, err
	}
	return c.MoneyExact(amount)
}

// Int64MinorUnits returns money as int64 minor units at the context scale.
func (c MoneyContext) Int64MinorUnits(m Money) (int64, error) {
	if err := c.sameCurrency(m); err != nil {
		return 0, err
	}
	return m.amount.Int64MinorUnits(c.Scale, c.Rounding)
}

// Int64MinorUnitsExact returns money as int64 minor units at the context scale
// only when no non-zero digit would be discarded.
func (c MoneyContext) Int64MinorUnitsExact(m Money) (int64, error) {
	if err := c.sameCurrency(m); err != nil {
		return 0, err
	}
	return m.amount.Int64MinorUnitsExact(c.Scale)
}

// Allocate splits money into equal parts at the context scale.
func (c MoneyContext) Allocate(m Money, parts int) ([]Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return nil, err
	}
	return m.Allocate(parts, c.Scale, c.Rounding)
}

// AllocateRatios splits money by ratios at the context scale.
func (c MoneyContext) AllocateRatios(m Money, ratios []int64) ([]Money, error) {
	if err := c.sameCurrency(m); err != nil {
		return nil, err
	}
	return m.AllocateRatios(ratios, c.Scale, c.Rounding)
}

func (c MoneyContext) validate() error {
	if len(c.Currency) < 3 || len(c.Currency) > 12 {
		return ErrInvalidCurrency
	}
	for i := 0; i < len(c.Currency); i++ {
		ch := c.Currency[i]
		if !(ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			return ErrInvalidCurrency
		}
	}
	if c.Scale < 0 {
		return ErrInvalidScale
	}
	if !c.Rounding.valid() {
		return ErrInvalidRoundingMode
	}
	return nil
}

func (c MoneyContext) sameCurrency(m Money) error {
	if err := c.validate(); err != nil {
		return err
	}
	if err := m.validCurrency(); err != nil {
		return err
	}
	if c.Currency != m.currency {
		return ErrCurrencyMismatch
	}
	return nil
}

func (c MoneyContext) String() string {
	return fmt.Sprintf("%s scale=%d rounding=%s", c.Currency, c.Scale, c.Rounding)
}

type moneyContextJSON struct {
	Currency string       `json:"currency"`
	Scale    int32        `json:"scale"`
	Rounding RoundingMode `json:"rounding"`
}
