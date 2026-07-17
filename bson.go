package qdecimal

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
)

const bsonTypeString byte = 0x02

// DefaultMaxBSONDecimalTextBytes bounds untrusted BSON decimal text before it
// is copied into a Go string and parsed.
const DefaultMaxBSONDecimalTextBytes = DefaultMaxParseDigits + DefaultMaxParseExponentDigits + 16

// MarshalBSONStringValue returns the raw BSON string value bytes for d.
//
// The bytes are the BSON value payload only:
//
//	int32 byte-length including NUL | UTF-8 decimal text | NUL
//
// qdecimal uses string values for this dependency-free BSON bridge so arbitrary
// precision and preserved scale are not limited by Decimal128's finite range.
func (d Decimal) MarshalBSONStringValue() ([]byte, error) {
	out := make([]byte, 4, 4+d.textCapacity()+1)
	out = d.appendText(out)
	out = append(out, 0)
	valueLen := len(out) - 4
	if valueLen > math.MaxInt32 {
		return nil, ErrOverflow
	}
	binary.LittleEndian.PutUint32(out[:4], uint32(valueLen))
	return out, nil
}

// UnmarshalBSONStringValue decodes a raw BSON string value payload into d.
func (d *Decimal) UnmarshalBSONStringValue(data []byte) error {
	if d == nil {
		return ErrNilValue
	}
	if len(data) < 5 {
		return ErrInvalidSyntax
	}
	valueLen := binary.LittleEndian.Uint32(data[:4])
	if valueLen < 1 || uint64(valueLen) != uint64(len(data)-4) {
		return ErrInvalidSyntax
	}
	if data[len(data)-1] != 0 {
		return ErrInvalidSyntax
	}
	textLen := valueLen - 1
	if uint64(textLen) > uint64(DefaultMaxBSONDecimalTextBytes) {
		return ErrLimitExceeded
	}
	parsed, err := Parse(string(data[4 : len(data)-1]))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalBSONDocument returns a minimal BSON document with one string field.
//
// This is a driver-neutral boundary for document databases and message stores
// that accept raw BSON. It intentionally stores the decimal as precision-safe
// text rather than lossy floating-point data.
func (d Decimal) MarshalBSONDocument(field string) ([]byte, error) {
	if err := validateBSONField(field); err != nil {
		return nil, err
	}
	out := make([]byte, 4, 4+1+len(field)+1+4+d.textCapacity()+1+1)
	out = append(out, bsonTypeString)
	out = append(out, field...)
	out = append(out, 0)
	valueStart := len(out)
	out = append(out, 0, 0, 0, 0)
	out = d.appendText(out)
	out = append(out, 0)
	out = append(out, 0)
	valueLen := len(out) - valueStart - 4 - 1
	totalLen := len(out)
	if valueLen > math.MaxInt32 || totalLen > math.MaxInt32 {
		return nil, ErrOverflow
	}
	binary.LittleEndian.PutUint32(out[:4], uint32(totalLen))
	binary.LittleEndian.PutUint32(out[valueStart:valueStart+4], uint32(valueLen))
	return out, nil
}

// UnmarshalBSONDocument decodes a minimal single-field BSON document into d.
func (d *Decimal) UnmarshalBSONDocument(data []byte, field string) error {
	if d == nil {
		return ErrNilValue
	}
	if err := validateBSONField(field); err != nil {
		return err
	}
	if len(data) < 8 {
		return ErrInvalidSyntax
	}
	totalLen := binary.LittleEndian.Uint32(data[:4])
	if uint64(totalLen) != uint64(len(data)) || data[len(data)-1] != 0 {
		return ErrInvalidSyntax
	}
	pos := 4
	if data[pos] != bsonTypeString {
		return ErrInvalidSyntax
	}
	pos++
	end := bytes.IndexByte(data[pos:], 0)
	if end < 0 {
		return ErrInvalidSyntax
	}
	key := string(data[pos : pos+end])
	if key != field {
		return ErrInvalidSyntax
	}
	pos += end + 1
	if len(data)-pos < 5 {
		return ErrInvalidSyntax
	}
	valueLen := binary.LittleEndian.Uint32(data[pos : pos+4])
	if valueLen < 1 {
		return ErrInvalidSyntax
	}
	valueEnd := uint64(pos) + 4 + uint64(valueLen)
	if valueEnd != uint64(len(data)-1) {
		return ErrInvalidSyntax
	}
	return d.UnmarshalBSONStringValue(data[pos:int(valueEnd)])
}

func validateBSONField(field string) error {
	if field == "" || strings.IndexByte(field, 0) >= 0 {
		return ErrInvalidSyntax
	}
	return nil
}
