package qdecimal

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"math/bits"
	"slices"
)

var binaryMagic = [4]byte{'Q', 'D', 'E', 'C'}
var fixed64BinaryMagic = [4]byte{'Q', 'F', '6', '4'}
var moneyBinaryMagic = [4]byte{'Q', 'M', 'O', 'N'}

const binaryVersion byte = 1
const bigWordBytes = bits.UintSize / 8

const (
	// DefaultMaxBinaryCoefficientBytes bounds untrusted binary decimal payloads.
	// Trusted storage can opt out with BinaryDecodeOptions.
	DefaultMaxBinaryCoefficientBytes = 4096
)

// BinaryDecodeOptions controls resource limits while decoding versioned binary
// Decimal and Money payloads. A zero limit disables that specific limit.
type BinaryDecodeOptions struct {
	MaxCoefficientBytes int
	MaxScale            int32
}

// DefaultBinaryDecodeOptions returns the limits used by UnmarshalBinary and
// GobDecode.
func DefaultBinaryDecodeOptions() BinaryDecodeOptions {
	return BinaryDecodeOptions{
		MaxCoefficientBytes: DefaultMaxBinaryCoefficientBytes,
		MaxScale:            DefaultMaxParseScale,
	}
}

// BinarySize returns the exact number of bytes produced by MarshalBinary.
func (d Decimal) BinarySize() int {
	return 14 + decimalBinaryCoefficientLen(d)
}

// MarshalBinary implements encoding.BinaryMarshaler using a stable versioned
// network-order format:
//
//	QDEC | version | scale uint32 | coefficient length uint32 | coefficient bytes
//
// The coefficient is stored as signed magnitude: one sign byte plus big-endian
// absolute coefficient bytes.
func (d Decimal) MarshalBinary() ([]byte, error) {
	return d.AppendBinary(make([]byte, 0, d.BinarySize()))
}

// AppendBinary appends d's stable binary representation to dst.
func (d Decimal) AppendBinary(dst []byte) ([]byte, error) {
	coefLen := decimalBinaryCoefficientLen(d)
	if uint64(coefLen) > uint64(^uint32(0)) {
		return nil, ErrOverflow
	}
	dst = append(dst, binaryMagic[:]...)
	dst = append(dst, binaryVersion)
	sign := byte(0)
	if d.coef.Sign() < 0 {
		sign = 1
	}
	dst = append(dst, sign)
	dst = binary.BigEndian.AppendUint32(dst, uint32(d.scale))
	dst = binary.BigEndian.AppendUint32(dst, uint32(coefLen))
	if coefLen == 0 {
		return dst, nil
	}
	start := len(dst)
	dst = appendSized(dst, coefLen)
	if d.coef.Sign() < 0 {
		fillBigIntMagnitudeBytes(dst[start:], &d.coef)
		return dst, nil
	}
	d.coef.FillBytes(dst[start:])
	return dst, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler using
// DefaultBinaryDecodeOptions().
func (d *Decimal) UnmarshalBinary(data []byte) error {
	return d.UnmarshalBinaryWithOptions(data, DefaultBinaryDecodeOptions())
}

// UnmarshalBinaryWithOptions decodes d's stable binary representation with
// explicit resource limits for trusted or untrusted storage boundaries.
func (d *Decimal) UnmarshalBinaryWithOptions(data []byte, opts BinaryDecodeOptions) error {
	if d == nil {
		return fmt.Errorf("qdecimal: UnmarshalBinary on nil *Decimal")
	}
	if len(data) < 14 {
		return ErrInvalidSyntax
	}
	if string(data[:4]) != string(binaryMagic[:]) {
		return ErrInvalidSyntax
	}
	if data[4] != binaryVersion {
		return fmt.Errorf("%w: unsupported binary version %d", ErrInvalidSyntax, data[4])
	}
	sign := data[5]
	if sign > 1 {
		return ErrInvalidSyntax
	}
	scale := binary.BigEndian.Uint32(data[6:10])
	length := binary.BigEndian.Uint32(data[10:14])
	if uint64(length) != uint64(len(data)-14) {
		return ErrInvalidSyntax
	}
	if opts.MaxCoefficientBytes > 0 && uint64(length) > uint64(opts.MaxCoefficientBytes) {
		return ErrLimitExceeded
	}
	if opts.MaxScale > 0 && uint64(scale) > uint64(opts.MaxScale) {
		return ErrLimitExceeded
	}
	if scale > maxScale {
		return ErrInvalidScale
	}
	coef := new(big.Int).SetBytes(data[14:])
	if sign == 1 && coef.Sign() != 0 {
		coef.Neg(coef)
	}
	*d = Decimal{coef: *coef, scale: int32(scale)}.canonicalZero()
	return nil
}

// GobEncode implements gob.GobEncoder using the stable binary format.
func (d Decimal) GobEncode() ([]byte, error) { return d.MarshalBinary() }

// GobDecode implements gob.GobDecoder using the stable binary format.
func (d *Decimal) GobDecode(data []byte) error { return d.UnmarshalBinary(data) }

// BinarySize returns the exact number of bytes produced by MarshalBinary.
func (f Fixed64) BinarySize() int { return 17 }

// MarshalBinary implements encoding.BinaryMarshaler using a stable versioned
// network-order fixed64 format:
//
//	QF64 | version | scale uint32 | units int64
func (f Fixed64) MarshalBinary() ([]byte, error) {
	return f.AppendBinary(make([]byte, 0, f.BinarySize()))
}

// AppendBinary appends f's stable binary representation to dst.
func (f Fixed64) AppendBinary(dst []byte) ([]byte, error) {
	if !validFixed64Scale(f.scale) {
		return nil, ErrInvalidScale
	}
	dst = append(dst, fixed64BinaryMagic[:]...)
	dst = append(dst, binaryVersion)
	dst = binary.BigEndian.AppendUint32(dst, uint32(f.scale))
	dst = binary.BigEndian.AppendUint64(dst, uint64(f.units))
	return dst, nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler.
func (f *Fixed64) UnmarshalBinary(data []byte) error {
	if f == nil {
		return fmt.Errorf("qdecimal: UnmarshalBinary on nil *Fixed64")
	}
	if len(data) != 17 {
		return ErrInvalidSyntax
	}
	if string(data[:4]) != string(fixed64BinaryMagic[:]) {
		return ErrInvalidSyntax
	}
	if data[4] != binaryVersion {
		return fmt.Errorf("%w: unsupported fixed64 binary version %d", ErrInvalidSyntax, data[4])
	}
	scale := int32(binary.BigEndian.Uint32(data[5:9]))
	if !validFixed64Scale(scale) {
		return ErrInvalidScale
	}
	units := int64(binary.BigEndian.Uint64(data[9:17]))
	*f = Fixed64{units: units, scale: scale}
	return nil
}

// GobEncode implements gob.GobEncoder using the stable binary format.
func (f Fixed64) GobEncode() ([]byte, error) { return f.MarshalBinary() }

// GobDecode implements gob.GobDecoder using the stable binary format.
func (f *Fixed64) GobDecode(data []byte) error { return f.UnmarshalBinary(data) }

// BinarySize returns the exact number of bytes produced by MarshalBinary.
func (m Money) BinarySize() int {
	return 11 + len(m.currency) + m.amount.BinarySize()
}

// MarshalBinary implements encoding.BinaryMarshaler using a stable versioned
// network-order money format:
//
//	QMON | version | currency length uint16 | currency bytes |
//	decimal length uint32 | decimal binary bytes
func (m Money) MarshalBinary() ([]byte, error) {
	return m.AppendBinary(make([]byte, 0, m.BinarySize()))
}

// AppendBinary appends m's stable binary representation to dst.
func (m Money) AppendBinary(dst []byte) ([]byte, error) {
	if err := m.validCurrency(); err != nil {
		return nil, err
	}
	if len(m.currency) > 65535 {
		return nil, ErrInvalidCurrency
	}
	amountSize := m.amount.BinarySize()
	if uint64(amountSize) > uint64(^uint32(0)) {
		return nil, ErrOverflow
	}
	dst = append(dst, moneyBinaryMagic[:]...)
	dst = append(dst, binaryVersion)
	dst = binary.BigEndian.AppendUint16(dst, uint16(len(m.currency)))
	dst = append(dst, m.currency...)
	dst = binary.BigEndian.AppendUint32(dst, uint32(amountSize))
	return m.amount.AppendBinary(dst)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler using
// DefaultBinaryDecodeOptions() for the embedded Decimal payload.
func (m *Money) UnmarshalBinary(data []byte) error {
	return m.UnmarshalBinaryWithOptions(data, DefaultBinaryDecodeOptions())
}

// UnmarshalBinaryWithOptions decodes a Money binary payload with explicit
// resource limits for the embedded Decimal amount.
func (m *Money) UnmarshalBinaryWithOptions(data []byte, opts BinaryDecodeOptions) error {
	if m == nil {
		return fmt.Errorf("qdecimal: UnmarshalBinary on nil *Money")
	}
	if len(data) < 11 {
		return ErrInvalidSyntax
	}
	if string(data[:4]) != string(moneyBinaryMagic[:]) {
		return ErrInvalidSyntax
	}
	if data[4] != binaryVersion {
		return fmt.Errorf("%w: unsupported money binary version %d", ErrInvalidSyntax, data[4])
	}
	currencyLen := int(binary.BigEndian.Uint16(data[5:7]))
	pos := 7
	if len(data) < pos+currencyLen+4 {
		return ErrInvalidSyntax
	}
	currency := string(data[pos : pos+currencyLen])
	code, err := NormalizeCurrency(currency)
	if err != nil {
		return err
	}
	if code != currency {
		return ErrInvalidCurrency
	}
	pos += currencyLen
	amountLen := binary.BigEndian.Uint32(data[pos : pos+4])
	pos += 4
	if uint64(amountLen) != uint64(len(data)-pos) {
		return ErrInvalidSyntax
	}
	var amount Decimal
	if err := amount.UnmarshalBinaryWithOptions(data[pos:], opts); err != nil {
		return err
	}
	out, err := NewMoney(amount, currency)
	if err != nil {
		return err
	}
	*m = out
	return nil
}

// GobEncode implements gob.GobEncoder using the stable binary format.
func (m Money) GobEncode() ([]byte, error) { return m.MarshalBinary() }

// GobDecode implements gob.GobDecoder using the stable binary format.
func (m *Money) GobDecode(data []byte) error { return m.UnmarshalBinary(data) }

func decimalBinaryCoefficientLen(d Decimal) int {
	return (d.coef.BitLen() + 7) / 8
}

func fillBigIntMagnitudeBytes(dst []byte, v *big.Int) {
	out := len(dst)
	for _, word := range v.Bits() {
		w := uint(word)
		for i := 0; i < bigWordBytes; i++ {
			if out == 0 {
				return
			}
			out--
			dst[out] = byte(w)
			w >>= 8
		}
	}
}

func appendSized(dst []byte, count int) []byte {
	if count <= 0 {
		return dst
	}
	start := len(dst)
	dst = slices.Grow(dst, count)
	return dst[:start+count]
}
