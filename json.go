package qdecimal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// JSONMode selects how decimals are emitted as JSON.
type JSONMode byte

const (
	// EmitJSONString emits a quoted decimal string. This is the safest default.
	EmitJSONString JSONMode = iota
	// EmitJSONNumber emits an unquoted JSON number token.
	EmitJSONNumber
)

// MarshalJSONWithMode emits d using an explicit JSON policy.
func (d Decimal) MarshalJSONWithMode(mode JSONMode) ([]byte, error) {
	switch mode {
	case EmitJSONString:
		return json.Marshal(d.String())
	case EmitJSONNumber:
		return []byte(d.String()), nil
	default:
		return nil, fmt.Errorf("qdecimal: invalid JSON mode %d", mode)
	}
}

// Number wraps Decimal to marshal as a JSON number token instead of the default
// quoted string. Use it only with systems that preserve arbitrary-precision JSON
// numbers end to end.
type Number struct {
	Decimal Decimal
}

// AsNumber returns a JSON-number wrapper for d.
func AsNumber(d Decimal) Number { return Number{Decimal: d} }

// IsZero reports whether the wrapped decimal is numerically zero.
func (n Number) IsZero() bool { return n.Decimal.IsZero() }

// MarshalJSON implements json.Marshaler.
func (n Number) MarshalJSON() ([]byte, error) {
	return n.Decimal.MarshalJSONWithMode(EmitJSONNumber)
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *Number) UnmarshalJSON(data []byte) error {
	if n == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *Number")
	}
	var d Decimal
	if err := d.UnmarshalJSON(data); err != nil {
		return err
	}
	n.Decimal = d
	return nil
}

func decodeStrictJSON(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSyntax, err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return ErrInvalidSyntax
		}
		return fmt.Errorf("%w: %w", ErrInvalidSyntax, err)
	}
	return nil
}
