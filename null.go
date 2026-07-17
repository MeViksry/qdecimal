package qdecimal

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// NullDecimal represents a Decimal that may be SQL NULL or JSON null.
type NullDecimal struct {
	Decimal Decimal
	Valid   bool
}

// NewNullDecimal marks d as valid even when d is zero.
func NewNullDecimal(d Decimal) NullDecimal {
	return NullDecimal{Decimal: d, Valid: true}
}

// IsZero reports whether n is invalid/null.
func (n NullDecimal) IsZero() bool { return !n.Valid }

// String returns n's decimal text or "null".
func (n NullDecimal) String() string {
	if !n.Valid {
		return "null"
	}
	return n.Decimal.String()
}

// Format implements fmt.Formatter.
func (n NullDecimal) Format(s fmt.State, verb rune) {
	if !n.Valid {
		writeFormattedType(s, verb, "null", "qdecimal.NullDecimal")
		return
	}
	n.Decimal.Format(s, verb)
}

// MarshalText implements encoding.TextMarshaler.
func (n NullDecimal) MarshalText() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Decimal.MarshalText()
}

// AppendText appends n's text representation to dst.
func (n NullDecimal) AppendText(dst []byte) ([]byte, error) {
	if !n.Valid {
		return append(dst, "null"...), nil
	}
	return n.Decimal.AppendText(dst)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *NullDecimal) UnmarshalText(text []byte) error {
	if n == nil {
		return fmt.Errorf("qdecimal: UnmarshalText on nil *NullDecimal")
	}
	if isNullText(text) {
		n.Decimal = Decimal{}
		n.Valid = false
		return nil
	}
	var d Decimal
	if err := d.UnmarshalText(text); err != nil {
		n.Decimal = Decimal{}
		n.Valid = false
		return err
	}
	n.Decimal = d
	n.Valid = true
	return nil
}

// Scan implements database/sql.Scanner.
func (n *NullDecimal) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("qdecimal: Scan on nil *NullDecimal")
	}
	if src == nil {
		n.Decimal = Decimal{}
		n.Valid = false
		return nil
	}
	d, err := scanSource(src)
	if err != nil {
		n.Decimal = Decimal{}
		n.Valid = false
		return err
	}
	n.Decimal = d
	n.Valid = true
	return nil
}

// Value implements database/sql/driver.Valuer.
func (n NullDecimal) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Decimal.Value()
}

// MarshalJSON implements json.Marshaler.
func (n NullDecimal) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Decimal.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullDecimal) UnmarshalJSON(data []byte) error {
	if n == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *NullDecimal")
	}
	if string(data) == "null" {
		n.Decimal = Decimal{}
		n.Valid = false
		return nil
	}
	var d Decimal
	if err := json.Unmarshal(data, &d); err != nil {
		n.Decimal = Decimal{}
		n.Valid = false
		return err
	}
	n.Decimal = d
	n.Valid = true
	return nil
}

// NullFixed64 represents a Fixed64 that may be SQL NULL or JSON null.
type NullFixed64 struct {
	Fixed64 Fixed64
	Valid   bool
}

// NewNullFixed64 marks f as valid even when f is zero.
func NewNullFixed64(f Fixed64) NullFixed64 {
	return NullFixed64{Fixed64: f, Valid: true}
}

// IsZero reports whether n is invalid/null.
func (n NullFixed64) IsZero() bool { return !n.Valid }

// String returns n's fixed decimal text or "null".
func (n NullFixed64) String() string {
	if !n.Valid {
		return "null"
	}
	return n.Fixed64.String()
}

// Format implements fmt.Formatter.
func (n NullFixed64) Format(s fmt.State, verb rune) {
	if !n.Valid {
		writeFormattedType(s, verb, "null", "qdecimal.NullFixed64")
		return
	}
	n.Fixed64.Format(s, verb)
}

// MarshalText implements encoding.TextMarshaler.
func (n NullFixed64) MarshalText() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Fixed64.MarshalText()
}

// AppendText appends n's text representation to dst.
func (n NullFixed64) AppendText(dst []byte) ([]byte, error) {
	if !n.Valid {
		return append(dst, "null"...), nil
	}
	return n.Fixed64.AppendText(dst)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *NullFixed64) UnmarshalText(text []byte) error {
	if n == nil {
		return fmt.Errorf("qdecimal: UnmarshalText on nil *NullFixed64")
	}
	if isNullText(text) {
		n.Fixed64 = Fixed64{}
		n.Valid = false
		return nil
	}
	var f Fixed64
	if err := f.UnmarshalText(text); err != nil {
		n.Fixed64 = Fixed64{}
		n.Valid = false
		return err
	}
	n.Fixed64 = f
	n.Valid = true
	return nil
}

// Scan implements database/sql.Scanner.
func (n *NullFixed64) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("qdecimal: Scan on nil *NullFixed64")
	}
	if src == nil {
		n.Fixed64 = Fixed64{}
		n.Valid = false
		return nil
	}
	var f Fixed64
	if err := f.Scan(src); err != nil {
		n.Fixed64 = Fixed64{}
		n.Valid = false
		return err
	}
	n.Fixed64 = f
	n.Valid = true
	return nil
}

// Value implements database/sql/driver.Valuer.
func (n NullFixed64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Fixed64.Value()
}

// MarshalJSON implements json.Marshaler.
func (n NullFixed64) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Fixed64.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullFixed64) UnmarshalJSON(data []byte) error {
	if n == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *NullFixed64")
	}
	if string(data) == "null" {
		n.Fixed64 = Fixed64{}
		n.Valid = false
		return nil
	}
	var f Fixed64
	if err := json.Unmarshal(data, &f); err != nil {
		n.Fixed64 = Fixed64{}
		n.Valid = false
		return err
	}
	n.Fixed64 = f
	n.Valid = true
	return nil
}

// NullMoney represents Money that may be SQL NULL or JSON null.
type NullMoney struct {
	Money Money
	Valid bool
}

// NewNullMoney marks m as valid.
func NewNullMoney(m Money) NullMoney {
	return NullMoney{Money: m, Valid: true}
}

// IsZero reports whether n is invalid/null.
func (n NullMoney) IsZero() bool { return !n.Valid }

// String returns n's money text or "null".
func (n NullMoney) String() string {
	if !n.Valid {
		return "null"
	}
	return n.Money.String()
}

// Format implements fmt.Formatter.
func (n NullMoney) Format(s fmt.State, verb rune) {
	if !n.Valid {
		writeFormattedType(s, verb, "null", "qdecimal.NullMoney")
		return
	}
	n.Money.Format(s, verb)
}

// MarshalText implements encoding.TextMarshaler.
func (n NullMoney) MarshalText() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Money.MarshalText()
}

// AppendText appends n's text representation to dst.
func (n NullMoney) AppendText(dst []byte) ([]byte, error) {
	if !n.Valid {
		return append(dst, "null"...), nil
	}
	return n.Money.AppendText(dst)
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *NullMoney) UnmarshalText(text []byte) error {
	if n == nil {
		return fmt.Errorf("qdecimal: UnmarshalText on nil *NullMoney")
	}
	if isNullText(text) {
		n.Money = Money{}
		n.Valid = false
		return nil
	}
	var m Money
	if err := m.UnmarshalText(text); err != nil {
		n.Money = Money{}
		n.Valid = false
		return err
	}
	n.Money = m
	n.Valid = true
	return nil
}

// Scan implements database/sql.Scanner.
func (n *NullMoney) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("qdecimal: Scan on nil *NullMoney")
	}
	if src == nil {
		n.Money = Money{}
		n.Valid = false
		return nil
	}
	m, err := scanMoneySource(src)
	if err != nil {
		n.Money = Money{}
		n.Valid = false
		return err
	}
	n.Money = m
	n.Valid = true
	return nil
}

// Value implements database/sql/driver.Valuer.
func (n NullMoney) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Money.Value()
}

// MarshalJSON implements json.Marshaler.
func (n NullMoney) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return n.Money.MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullMoney) UnmarshalJSON(data []byte) error {
	if n == nil {
		return fmt.Errorf("qdecimal: UnmarshalJSON on nil *NullMoney")
	}
	if string(data) == "null" {
		n.Money = Money{}
		n.Valid = false
		return nil
	}
	var m Money
	if err := json.Unmarshal(data, &m); err != nil {
		n.Money = Money{}
		n.Valid = false
		return err
	}
	n.Money = m
	n.Valid = true
	return nil
}

func isNullText(text []byte) bool {
	return len(text) == 4 && text[0] == 'n' && text[1] == 'u' && text[2] == 'l' && text[3] == 'l'
}
