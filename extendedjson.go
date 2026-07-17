package qdecimal

import (
	"encoding/json"
	"fmt"
)

// ExtendedJSON wraps Decimal using MongoDB-style Decimal128 Extended JSON:
//
//	{"$numberDecimal":"123.45"}
//
// It intentionally avoids importing a database driver. Driver-specific BSON
// adapters can build on this stable representation.
type ExtendedJSON struct {
	Decimal Decimal
}

// AsExtendedJSON returns a MongoDB-style Extended JSON wrapper for d.
func AsExtendedJSON(d Decimal) ExtendedJSON { return ExtendedJSON{Decimal: d} }

// IsZero reports whether the wrapped decimal is numerically zero.
func (e ExtendedJSON) IsZero() bool { return e.Decimal.IsZero() }

// MarshalJSON implements json.Marshaler.
func (e ExtendedJSON) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"$numberDecimal": e.Decimal.String()})
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *ExtendedJSON) UnmarshalJSON(data []byte) error {
	if e == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *ExtendedJSON")
	}
	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSyntax, err)
	}
	value, ok := payload["$numberDecimal"]
	if !ok || len(payload) != 1 {
		return ErrInvalidSyntax
	}
	d, err := Parse(value)
	if err != nil {
		return err
	}
	e.Decimal = d
	return nil
}

// MarshalExtendedJSON emits MongoDB-style Decimal128 Extended JSON.
func (d Decimal) MarshalExtendedJSON() ([]byte, error) {
	return AsExtendedJSON(d).MarshalJSON()
}
