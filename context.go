package qdecimal

import (
	"encoding/json"
	"fmt"
)

// Context is an explicit finance arithmetic policy.
//
// It deliberately replaces package-global precision knobs: callers pass the
// policy they want at each boundary where rounding may occur.
type Context struct {
	Scale    int32
	Rounding RoundingMode
}

// NewContext validates and returns a finance arithmetic context.
func NewContext(scale int32, rounding RoundingMode) (Context, error) {
	if scale < 0 {
		return Context{}, ErrInvalidScale
	}
	if !rounding.valid() {
		return Context{}, ErrInvalidRoundingMode
	}
	return Context{Scale: scale, Rounding: rounding}, nil
}

// MustContext is for package initialization and tests.
func MustContext(scale int32, rounding RoundingMode) Context {
	ctx, err := NewContext(scale, rounding)
	if err != nil {
		panic(err)
	}
	return ctx
}

// MarshalJSON emits a stable policy object for configuration and audit logs.
func (c Context) MarshalJSON() ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return json.Marshal(contextJSON{Scale: c.Scale, Rounding: c.Rounding})
}

// UnmarshalJSON decodes and validates a policy object.
func (c *Context) UnmarshalJSON(data []byte) error {
	if c == nil {
		return ErrNilValue
	}
	var payload contextJSON
	if err := decodeStrictJSON(data, &payload); err != nil {
		return err
	}
	out, err := NewContext(payload.Scale, payload.Rounding)
	if err != nil {
		return err
	}
	*c = out
	return nil
}

// Quantize rounds d to the context scale.
func (c Context) Quantize(d Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	return d.Rescale(c.Scale, c.Rounding)
}

// QuantizeExact changes d to the context scale without discarding non-zero
// digits.
func (c Context) QuantizeExact(d Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	return d.RescaleExact(c.Scale)
}

// QuantizeStep rounds d to a valid increment, then to the context scale.
func (c Context) QuantizeStep(d, step Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	stepped, err := d.QuantizeStep(step, c.Rounding)
	if err != nil {
		return Decimal{}, err
	}
	return stepped.Rescale(c.Scale, c.Rounding)
}

// QuantizeStepExact changes d to the context scale only when d is already an
// exact multiple of step and no non-zero digits would be discarded.
func (c Context) QuantizeStepExact(d, step Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	stepped, err := d.QuantizeStepExact(step)
	if err != nil {
		return Decimal{}, err
	}
	return c.QuantizeExact(stepped)
}

// Add returns a + b rounded to the context scale.
func (c Context) Add(a, b Decimal) (Decimal, error) {
	return c.Quantize(a.Add(b))
}

// AddExact returns a + b at the context scale, failing with ErrInexact if
// non-zero digits would be discarded.
func (c Context) AddExact(a, b Decimal) (Decimal, error) {
	return c.QuantizeExact(a.Add(b))
}

// Sub returns a - b rounded to the context scale.
func (c Context) Sub(a, b Decimal) (Decimal, error) {
	return c.Quantize(a.Sub(b))
}

// SubExact returns a - b at the context scale, failing with ErrInexact if
// non-zero digits would be discarded.
func (c Context) SubExact(a, b Decimal) (Decimal, error) {
	return c.QuantizeExact(a.Sub(b))
}

// Mul returns a * b rounded to the context scale.
func (c Context) Mul(a, b Decimal) (Decimal, error) {
	return c.Quantize(a.Mul(b))
}

// MulExact returns a * b at the context scale, failing with ErrInexact if
// non-zero digits would be discarded.
func (c Context) MulExact(a, b Decimal) (Decimal, error) {
	return c.QuantizeExact(a.Mul(b))
}

// Div returns a / b rounded to the context scale.
func (c Context) Div(a, b Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	return a.Div(b, c.Scale, c.Rounding)
}

// DivExact returns a / b at the context scale without rounding.
func (c Context) DivExact(a, b Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	exact, err := a.DivExact(b)
	if err != nil {
		return Decimal{}, err
	}
	return exact.RescaleExact(c.Scale)
}

// StringFixed returns d rendered at the context scale.
func (c Context) StringFixed(d Decimal) (string, error) {
	rounded, err := c.Quantize(d)
	if err != nil {
		return "", err
	}
	return rounded.String(), nil
}

func (c Context) validate() error {
	if c.Scale < 0 {
		return ErrInvalidScale
	}
	if !c.Rounding.valid() {
		return ErrInvalidRoundingMode
	}
	return nil
}

func (c Context) String() string {
	return fmt.Sprintf("scale=%d rounding=%s", c.Scale, c.Rounding)
}

type contextJSON struct {
	Scale    int32        `json:"scale"`
	Rounding RoundingMode `json:"rounding"`
}
