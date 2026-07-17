package qdecimal

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	stdBinary "encoding/binary"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
	"testing"
	"testing/quick"
)

var (
	_ fmt.Stringer               = Decimal{}
	_ fmt.Formatter              = Decimal{}
	_ encoding.TextMarshaler     = Decimal{}
	_ encoding.TextUnmarshaler   = (*Decimal)(nil)
	_ encoding.BinaryMarshaler   = Decimal{}
	_ encoding.BinaryUnmarshaler = (*Decimal)(nil)
	_ gob.GobEncoder             = Decimal{}
	_ gob.GobDecoder             = (*Decimal)(nil)
	_ json.Marshaler             = Decimal{}
	_ json.Unmarshaler           = (*Decimal)(nil)
	_ sql.Scanner                = (*Decimal)(nil)
	_ driver.Valuer              = Decimal{}

	_ json.Marshaler   = Number{}
	_ json.Unmarshaler = (*Number)(nil)
	_ json.Marshaler   = ExtendedJSON{}
	_ json.Unmarshaler = (*ExtendedJSON)(nil)

	_ json.Marshaler           = NullDecimal{}
	_ json.Unmarshaler         = (*NullDecimal)(nil)
	_ fmt.Stringer             = NullDecimal{}
	_ fmt.Formatter            = NullDecimal{}
	_ encoding.TextMarshaler   = NullDecimal{}
	_ encoding.TextUnmarshaler = (*NullDecimal)(nil)
	_ sql.Scanner              = (*NullDecimal)(nil)
	_ driver.Valuer            = NullDecimal{}

	_ fmt.Stringer               = Fixed64{}
	_ fmt.Formatter              = Fixed64{}
	_ encoding.TextMarshaler     = Fixed64{}
	_ encoding.TextUnmarshaler   = (*Fixed64)(nil)
	_ encoding.BinaryMarshaler   = Fixed64{}
	_ encoding.BinaryUnmarshaler = (*Fixed64)(nil)
	_ gob.GobEncoder             = Fixed64{}
	_ gob.GobDecoder             = (*Fixed64)(nil)
	_ json.Marshaler             = Fixed64{}
	_ json.Unmarshaler           = (*Fixed64)(nil)
	_ sql.Scanner                = (*Fixed64)(nil)
	_ driver.Valuer              = Fixed64{}

	_ json.Marshaler           = NullFixed64{}
	_ json.Unmarshaler         = (*NullFixed64)(nil)
	_ fmt.Stringer             = NullFixed64{}
	_ fmt.Formatter            = NullFixed64{}
	_ encoding.TextMarshaler   = NullFixed64{}
	_ encoding.TextUnmarshaler = (*NullFixed64)(nil)
	_ sql.Scanner              = (*NullFixed64)(nil)
	_ driver.Valuer            = NullFixed64{}

	_ fmt.Stringer               = Money{}
	_ fmt.Formatter              = Money{}
	_ encoding.TextMarshaler     = Money{}
	_ encoding.TextUnmarshaler   = (*Money)(nil)
	_ encoding.BinaryMarshaler   = Money{}
	_ encoding.BinaryUnmarshaler = (*Money)(nil)
	_ gob.GobEncoder             = Money{}
	_ gob.GobDecoder             = (*Money)(nil)
	_ json.Marshaler             = Money{}
	_ json.Unmarshaler           = (*Money)(nil)
	_ sql.Scanner                = (*Money)(nil)
	_ driver.Valuer              = Money{}

	_ json.Marshaler           = NullMoney{}
	_ json.Unmarshaler         = (*NullMoney)(nil)
	_ fmt.Stringer             = NullMoney{}
	_ fmt.Formatter            = NullMoney{}
	_ encoding.TextMarshaler   = NullMoney{}
	_ encoding.TextUnmarshaler = (*NullMoney)(nil)
	_ sql.Scanner              = (*NullMoney)(nil)
	_ driver.Valuer            = NullMoney{}
)

func TestJSONOmitEmptyAndZeroSemantics(t *testing.T) {
	type payload struct {
		Amount    *Decimal    `json:"amount,omitempty"`
		Money     *Money      `json:"money,omitempty"`
		NullValue NullDecimal `json:"nullValue"`
	}

	data, err := json.Marshal(payload{})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"nullValue":null}` {
		t.Fatalf("zero payload JSON got %s", data)
	}

	zero := Zero
	money, err := NewMoney(Zero, "USD")
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(payload{
		Amount:    &zero,
		Money:     &money,
		NullValue: NewNullDecimal(Zero),
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"amount":"0","money":{"amount":"0","currency":"USD"},"nullValue":"0"}` {
		t.Fatalf("non-nil zero payload JSON got %s", data)
	}

	if !(NullDecimal{}).IsZero() || NewNullDecimal(Zero).IsZero() {
		t.Fatal("NullDecimal IsZero semantics mismatch")
	}
	if !AsNumber(Zero).IsZero() || !AsExtendedJSON(Zero).IsZero() {
		t.Fatal("JSON wrapper IsZero should follow wrapped decimal")
	}
}

func TestExactArithmeticPreservesScale(t *testing.T) {
	a := MustParse("12.30")
	b := MustParse("0.070")
	if got := a.Add(b).String(); got != "12.370" {
		t.Fatalf("add got %s", got)
	}
	if got := a.Sub(b).String(); got != "12.230" {
		t.Fatalf("sub got %s", got)
	}
	if got := MustParse("1.20").Mul(MustParse("3.400")).String(); got != "4.08000" {
		t.Fatalf("mul got %s", got)
	}

	div, err := One.Div(MustParse("3"), 4, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if div.String() != "0.3333" {
		t.Fatalf("div got %s", div)
	}

	exact, err := One.DivExact(MustParse("8"))
	if err != nil {
		t.Fatal(err)
	}
	if exact.String() != "0.125" {
		t.Fatalf("exact div got %s", exact)
	}
	if _, err := One.DivExact(MustParse("3")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected inexact division error, got %v", err)
	}
}

func TestContextQuantizeAndMinorUnits(t *testing.T) {
	ctx := MustContext(2, ToNearestEven)
	got, err := ctx.Mul(MustParse("12.345"), MustParse("2"))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "24.69" {
		t.Fatalf("context mul got %s", got)
	}

	fixed, err := ctx.StringFixed(MustParse("1.2"))
	if err != nil {
		t.Fatal(err)
	}
	if fixed != "1.20" {
		t.Fatalf("fixed got %s", fixed)
	}

	stepped, err := ctx.QuantizeStep(MustParse("1.23"), MustParse("0.05"))
	if err != nil {
		t.Fatal(err)
	}
	if stepped.String() != "1.25" || stepped.Scale() != 2 {
		t.Fatalf("context quantize step got %s scale=%d", stepped, stepped.Scale())
	}
	stepped, err = ctx.QuantizeStep(MustParse("1.234"), MustParse("0.005"))
	if err != nil {
		t.Fatal(err)
	}
	if stepped.String() != "1.24" || stepped.Scale() != 2 {
		t.Fatalf("context quantize step rescale got %s scale=%d", stepped, stepped.Scale())
	}
	exactStep, err := ctx.QuantizeStepExact(MustParse("1.20"), MustParse("-0.05"))
	if err != nil {
		t.Fatal(err)
	}
	if exactStep.String() != "1.20" || exactStep.Scale() != 2 {
		t.Fatalf("context exact quantize step got %s scale=%d", exactStep, exactStep.Scale())
	}
	if _, err := ctx.QuantizeStepExact(MustParse("1.23"), MustParse("0.05")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact quantize step multiple error, got %v", err)
	}
	if _, err := ctx.QuantizeStepExact(MustParse("1.125"), MustParse("0.125")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact quantize step scale error, got %v", err)
	}
	if _, err := ctx.QuantizeStep(MustParse("1.23"), Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected context quantize step zero error, got %v", err)
	}

	amount, err := NewFromMinorUnits(12345, 2)
	if err != nil {
		t.Fatal(err)
	}
	if amount.String() != "123.45" {
		t.Fatalf("minor units got %s", amount)
	}
	units, err := MustParse("123.456").Int64MinorUnits(2, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if units != 12346 {
		t.Fatalf("minor units got %d", units)
	}
	exactUnits, err := MustParse("123.4500").Int64MinorUnitsExact(2)
	if err != nil {
		t.Fatal(err)
	}
	if exactUnits != 12345 {
		t.Fatalf("exact minor units got %d", exactUnits)
	}
	if _, err := MustParse("123.456").MinorUnitsExact(2); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact minor units inexact, got %v", err)
	}
	if _, err := MustParse("9223372036854775808").Int64MinorUnits(0, TowardZero); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected overflow, got %v", err)
	}
	if err := ValidateMinorScale(18); err != nil {
		t.Fatalf("valid minor scale got %v", err)
	}
	if err := ValidateMinorScale(19); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid minor scale, got %v", err)
	}
}

func TestExactScalePolicies(t *testing.T) {
	got, err := MustParse("123.4500").RescaleExact(2)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "123.45" || got.Scale() != 2 {
		t.Fatalf("exact rescale got %s scale=%d", got, got.Scale())
	}
	padded, err := MustParse("123.45").RescaleExact(4)
	if err != nil {
		t.Fatal(err)
	}
	if padded.String() != "123.4500" || padded.Scale() != 4 {
		t.Fatalf("exact padded rescale got %s scale=%d", padded, padded.Scale())
	}
	if _, err := MustParse("123.451").RescaleExact(2); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact rescale inexact error, got %v", err)
	}
	template := MustParse("0.00")
	quantized, err := MustParse("-1.230").QuantizeExact(template)
	if err != nil {
		t.Fatal(err)
	}
	if quantized.String() != "-1.23" || quantized.Scale() != 2 {
		t.Fatalf("exact quantize got %s scale=%d", quantized, quantized.Scale())
	}

	ctx := MustContext(2, ToNearestEven)
	if got, err := ctx.AddExact(MustParse("1.20"), MustParse("0.030")); err != nil || got.String() != "1.23" {
		t.Fatalf("context add exact got %s err=%v", got, err)
	}
	if _, err := ctx.AddExact(MustParse("1.20"), MustParse("0.031")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact add inexact, got %v", err)
	}
	if got, err := ctx.DivExact(MustParse("1"), MustParse("4")); err != nil || got.String() != "0.25" {
		t.Fatalf("context div exact got %s err=%v", got, err)
	}
	if _, err := ctx.DivExact(MustParse("1"), MustParse("8")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context div exact scale inexact, got %v", err)
	}
}

func TestAggregates(t *testing.T) {
	values := []Decimal{MustParse("1.10"), MustParse("2.20"), MustParse("3.30")}
	if got := Sum(values...).String(); got != "6.60" {
		t.Fatalf("sum got %s", got)
	}
	avg, err := Avg(values, 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if avg.String() != "2.20" {
		t.Fatalf("avg got %s", avg)
	}
	exactAvg, err := AvgExact([]Decimal{MustParse("1"), MustParse("2")})
	if err != nil {
		t.Fatal(err)
	}
	if exactAvg.String() != "1.5" {
		t.Fatalf("exact avg got %s", exactAvg)
	}
	if _, err := AvgExact([]Decimal{MustParse("1"), MustParse("2"), MustParse("2")}); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected inexact exact avg, got %v", err)
	}
	ctx := MustContext(1, ToNearestAway)
	sum, err := ctx.Sum(values...)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "6.6" {
		t.Fatalf("context sum got %s", sum)
	}
	avg, err = ctx.Avg(MustParse("1"), MustParse("2"))
	if err != nil {
		t.Fatal(err)
	}
	if avg.String() != "1.5" {
		t.Fatalf("context avg got %s", avg)
	}
	ledger := MustContext(2, ToNearestEven)
	exactSum, err := ledger.SumExact(MustParse("1.20"), MustParse("0.030"))
	if err != nil {
		t.Fatal(err)
	}
	if exactSum.String() != "1.23" {
		t.Fatalf("context exact sum got %s", exactSum)
	}
	if _, err := ledger.SumExact(MustParse("1.201")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact sum scale error, got %v", err)
	}
	exactAvg, err = ledger.AvgExact(MustParse("1.00"), MustParse("2.00"))
	if err != nil {
		t.Fatal(err)
	}
	if exactAvg.String() != "1.50" {
		t.Fatalf("context exact avg got %s", exactAvg)
	}
	if _, err := ledger.AvgExact(MustParse("1"), MustParse("2"), MustParse("2")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact avg repeating error, got %v", err)
	}
	if _, err := ledger.AvgExact(MustParse("1.00"), MustParse("2.01")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact avg scale error, got %v", err)
	}
	if _, err := Avg(nil, 2, ToNearestEven); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected empty input error, got %v", err)
	}
	if _, err := AvgExact(nil); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected empty exact avg error, got %v", err)
	}
	if _, err := ledger.SumExact(); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected empty exact sum error, got %v", err)
	}
}

func TestRoundingModesPositiveAndNegative(t *testing.T) {
	tests := []struct {
		in   string
		mode RoundingMode
		want string
	}{
		{"1.25", ToNearestEven, "1.2"},
		{"1.35", ToNearestEven, "1.4"},
		{"1.25", ToNearestAway, "1.3"},
		{"1.25", ToNearestTowardZero, "1.2"},
		{"1.21", AwayFromZero, "1.3"},
		{"1.29", TowardZero, "1.2"},
		{"1.21", TowardPositive, "1.3"},
		{"1.29", TowardNegative, "1.2"},
		{"-1.25", ToNearestEven, "-1.2"},
		{"-1.35", ToNearestEven, "-1.4"},
		{"-1.25", ToNearestAway, "-1.3"},
		{"-1.25", ToNearestTowardZero, "-1.2"},
		{"-1.21", AwayFromZero, "-1.3"},
		{"-1.29", TowardZero, "-1.2"},
		{"-1.21", TowardPositive, "-1.2"},
		{"-1.21", TowardNegative, "-1.3"},
	}

	for _, tt := range tests {
		got, err := MustParse(tt.in).Rescale(1, tt.mode)
		if err != nil {
			t.Fatalf("%s: %v", tt.in, err)
		}
		if got.String() != tt.want {
			t.Fatalf("%s mode %d got %s want %s", tt.in, tt.mode, got, tt.want)
		}
		if got.Scale() != 1 {
			t.Fatalf("%s did not rescale to 1 digit", got)
		}
	}
}

func TestRegressionRoundingRescalesAndNegativeDivision(t *testing.T) {
	rounded, err := MustParse("1.230").Rescale(2, AwayFromZero)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.String() != "1.23" || rounded.Scale() != 2 {
		t.Fatalf("rounding without discarded digits must still rescale: %s scale=%d", rounded, rounded.Scale())
	}

	neg, err := MustParse("-1").Div(MustParse("2"), 0, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if neg.String() != "-1" {
		t.Fatalf("negative division rounded with wrong sign: %s", neg)
	}

	truncated, err := MustParse("-12.349").Truncate(2)
	if err != nil {
		t.Fatal(err)
	}
	if truncated.String() != "-12.34" || truncated.Scale() != 2 {
		t.Fatalf("negative truncate got %s scale=%d", truncated, truncated.Scale())
	}
	ceiled, err := MustParse("-12.349").Ceil(2)
	if err != nil {
		t.Fatal(err)
	}
	if ceiled.String() != "-12.34" || ceiled.Scale() != 2 {
		t.Fatalf("negative ceil got %s scale=%d", ceiled, ceiled.Scale())
	}
	floored, err := MustParse("-12.341").Floor(2)
	if err != nil {
		t.Fatal(err)
	}
	if floored.String() != "-12.35" || floored.Scale() != 2 {
		t.Fatalf("negative floor got %s scale=%d", floored, floored.Scale())
	}
}

func TestQuantizeRatPowAndBinaryRoundTrip(t *testing.T) {
	template := MustParse("0.0000")
	q, err := MustParse("1.23456").Quantize(template, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if q.String() != "1.2346" {
		t.Fatalf("quantize got %s", q)
	}

	rat := MustParse("12.50").Rat()
	if rat.Cmp(big.NewRat(25, 2)) != 0 {
		t.Fatalf("rat got %s", rat)
	}
	fromRat, err := FromRat(big.NewRat(1, 8), 3, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if fromRat.String() != "0.125" {
		t.Fatalf("from rat got %s", fromRat)
	}

	pow, err := MustParse("1.20").PowInt(3)
	if err != nil {
		t.Fatal(err)
	}
	if pow.String() != "1.728000" {
		t.Fatalf("pow got %s", pow)
	}
	roundedPow, err := MustParse("1.20").Pow(MustParse("3.0"), 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if roundedPow.String() != "1.73" {
		t.Fatalf("rounded pow got %s", roundedPow)
	}
	negativePow, err := MustParse("2").Pow(MustParse("-3"), 4, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if negativePow.String() != "0.1250" {
		t.Fatalf("negative pow got %s", negativePow)
	}
	zeroPow, err := MustParse("9.99").Pow(Zero, 3, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if zeroPow.String() != "1.000" {
		t.Fatalf("zero pow got %s", zeroPow)
	}
	if _, err := MustParse("2").Pow(MustParse("0.5"), 4, ToNearestEven); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected fractional exponent error, got %v", err)
	}
	if _, err := Zero.Pow(MustParse("-1"), 2, ToNearestEven); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected zero negative exponent division error, got %v", err)
	}
	if _, err := MustParse("2").Pow(MustParse("18446744073709551616"), 2, ToNearestEven); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected huge exponent overflow, got %v", err)
	}

	sample := MustParse("-123.4500")
	binary, err := sample.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(binary) != sample.BinarySize() {
		t.Fatalf("binary size got %d want %d", len(binary), sample.BinarySize())
	}
	appended, err := sample.AppendBinary(make([]byte, 0, sample.BinarySize()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(appended, binary) {
		t.Fatalf("append binary mismatch got %x want %x", appended, binary)
	}
	var decoded Decimal
	if err := decoded.UnmarshalBinary(binary); err != nil {
		t.Fatal(err)
	}
	if decoded.String() != "-123.4500" {
		t.Fatalf("binary round trip got %s", decoded)
	}
	if err := decoded.UnmarshalBinary([]byte("bad")); err == nil {
		t.Fatal("expected binary format error")
	}
	hugeLength := makeDecimalBinaryPayload(0, nil)
	stdBinary.BigEndian.PutUint32(hugeLength[10:14], math.MaxUint32)
	if err := decoded.UnmarshalBinary(hugeLength); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected huge declared coefficient length error, got %v", err)
	}
	limitedOptions := DefaultBinaryDecodeOptions()
	limitedOptions.MaxCoefficientBytes = 1
	twoByteCoefficient, err := MustParse("256").MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if err := decoded.UnmarshalBinaryWithOptions(twoByteCoefficient, limitedOptions); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected coefficient byte limit error, got %v", err)
	}
	limitedOptions.MaxCoefficientBytes = 2
	if err := decoded.UnmarshalBinaryWithOptions(twoByteCoefficient, limitedOptions); err != nil {
		t.Fatalf("trusted coefficient byte limit should decode: %v", err)
	}
	overscale := makeDecimalBinaryPayload(uint32(DefaultMaxParseScale+1), []byte{1})
	if err := decoded.UnmarshalBinary(overscale); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected binary scale limit error, got %v", err)
	}
	trustedBinary := DefaultBinaryDecodeOptions()
	trustedBinary.MaxScale = 0
	if err := decoded.UnmarshalBinaryWithOptions(overscale, trustedBinary); err != nil {
		t.Fatalf("trusted binary scale override should decode: %v", err)
	}
	if decoded.Scale() != DefaultMaxParseScale+1 {
		t.Fatalf("trusted binary scale got %d", decoded.Scale())
	}

	positive := MustParse("123456789.987654321")
	reused := make([]byte, 0, positive.BinarySize())
	allocs := testing.AllocsPerRun(1000, func() {
		var err error
		reused, err = positive.AppendBinary(reused[:0])
		if err != nil {
			panic(err)
		}
		if len(reused) != positive.BinarySize() {
			panic("bad decimal append binary size")
		}
	})
	if allocs != 0 {
		t.Fatalf("expected zero decimal append allocations, got %.2f", allocs)
	}
	negative := MustParse("-123456789.987654321")
	reused = make([]byte, 0, negative.BinarySize())
	allocs = testing.AllocsPerRun(1000, func() {
		var err error
		reused, err = negative.AppendBinary(reused[:0])
		if err != nil {
			panic(err)
		}
		if len(reused) != negative.BinarySize() {
			panic("bad negative decimal append binary size")
		}
	})
	if allocs != 0 {
		t.Fatalf("expected zero negative decimal append allocations, got %.2f", allocs)
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(MustParse("99.9900")); err != nil {
		t.Fatal(err)
	}
	var gobDecoded Decimal
	if err := gob.NewDecoder(&buf).Decode(&gobDecoded); err != nil {
		t.Fatal(err)
	}
	if gobDecoded.String() != "99.9900" {
		t.Fatalf("gob round trip got %s", gobDecoded)
	}
}

func makeDecimalBinaryPayload(scale uint32, coefficient []byte) []byte {
	out := make([]byte, 14+len(coefficient))
	copy(out[:4], binaryMagic[:])
	out[4] = binaryVersion
	stdBinary.BigEndian.PutUint32(out[6:10], scale)
	stdBinary.BigEndian.PutUint32(out[10:14], uint32(len(coefficient)))
	copy(out[14:], coefficient)
	return out
}

func TestQuantizeStepForExchangeTicks(t *testing.T) {
	tests := []struct {
		value string
		step  string
		mode  RoundingMode
		want  string
	}{
		{"1.23", "0.05", ToNearestAway, "1.25"},
		{"1.22", "0.05", ToNearestAway, "1.20"},
		{"1.225", "0.05", ToNearestEven, "1.20"},
		{"1.225", "0.05", ToNearestAway, "1.25"},
		{"-1.23", "0.05", TowardZero, "-1.20"},
		{"-1.23", "0.05", AwayFromZero, "-1.25"},
	}

	for _, tt := range tests {
		got, err := MustParse(tt.value).QuantizeStep(MustParse(tt.step), tt.mode)
		if err != nil {
			t.Fatalf("%s step %s: %v", tt.value, tt.step, err)
		}
		if got.String() != tt.want {
			t.Fatalf("%s step %s got %s want %s", tt.value, tt.step, got, tt.want)
		}
	}
	if _, err := MustParse("1.23").QuantizeStep(Zero, ToNearestEven); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected zero step error, got %v", err)
	}
	exact, err := MustParse("1.20").QuantizeStepExact(MustParse("-0.05"))
	if err != nil {
		t.Fatal(err)
	}
	if exact.String() != "1.20" || exact.Scale() != 2 {
		t.Fatalf("exact step got %s scale=%d", exact, exact.Scale())
	}
	if _, err := MustParse("1.23").QuantizeStepExact(MustParse("0.05")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact step multiple error, got %v", err)
	}
	if _, err := MustParse("1.23").QuantizeStepExact(Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected exact zero step error, got %v", err)
	}
}

func TestParsingBoundaries(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"−12.30", "-12.30"},
		{"-0.00", "0.00"},
		{"1.23e3", "1230"},
		{"1.23e-2", "0.0123"},
		{".5", "0.5"},
		{"1000", "1000"},
	}
	for _, tt := range tests {
		got, err := Parse(tt.input)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.input, err)
		}
		if got.String() != tt.want {
			t.Fatalf("parse %q got %s want %s", tt.input, got, tt.want)
		}
		byteGot, err := ParseBytes([]byte(tt.input))
		if err == nil && !got.Equal(byteGot) {
			t.Fatalf("ParseBytes %q got %s want %s", tt.input, byteGot, got)
		}
	}

	flexible, err := ParseFlexible(" 1,234,567.890 ")
	if err != nil {
		t.Fatal(err)
	}
	if flexible.String() != "1234567.890" {
		t.Fatalf("flexible parse got %s", flexible)
	}

	opts := DefaultParseOptions
	opts.DecimalSeparator = ','
	opts.ThousandsSeparator = '.'
	opts.AllowThousands = true
	localized, err := ParseWithOptions("1.234,50", opts)
	if err != nil {
		t.Fatal(err)
	}
	if localized.String() != "1234.50" {
		t.Fatalf("localized parse got %s", localized)
	}

	for _, input := range []string{"1,23", "1.", "nan", "+Inf", ""} {
		if _, err := ParseFlexible(input); err == nil {
			t.Fatalf("expected parse error for %q", input)
		}
	}

	if _, err := Parse("1e4095"); err != nil {
		t.Fatalf("default parse limit should allow 4096 coefficient digits: %v", err)
	}
	if _, err := Parse("1e4096"); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected positive exponent parse limit, got %v", err)
	}
	if _, err := Parse("1e-4097"); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected scale parse limit, got %v", err)
	}
	if _, err := Parse(strings.Repeat("9", DefaultMaxParseDigits)); err != nil {
		t.Fatalf("default parse limit should allow max raw digits: %v", err)
	}
	if _, err := Parse(strings.Repeat("9", DefaultMaxParseDigits+1)); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected raw digit parse limit, got %v", err)
	}
	if _, err := Parse("0." + strings.Repeat("1", int(DefaultMaxParseScale)+1)); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected fractional digit parse limit, got %v", err)
	}
	if _, err := Parse("1e" + strings.Repeat("0", DefaultMaxParseExponentDigits+1)); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected exponent digit parse limit, got %v", err)
	}

	limited := DefaultParseOptions
	limited.MaxDigits = 3
	if _, err := ParseWithOptions("1234", limited); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected custom digit limit, got %v", err)
	}
	expanded := DefaultParseOptions
	expanded.MaxDigits = 6
	expanded.MaxScale = 0
	bigShift, err := ParseWithOptions("1e5", expanded)
	if err != nil {
		t.Fatal(err)
	}
	if bigShift.String() != "100000" {
		t.Fatalf("custom expanded parse got %s", bigShift)
	}
	unlimitedExponent := DefaultParseOptions
	unlimitedExponent.MaxExponentDigits = 0
	zeroShift, err := ParseWithOptions("1e"+strings.Repeat("0", DefaultMaxParseExponentDigits+1), unlimitedExponent)
	if err != nil {
		t.Fatal(err)
	}
	if zeroShift.String() != "1" {
		t.Fatalf("custom exponent parse got %s", zeroShift)
	}
}

func TestFloatConstructorsNeverPanicOnNonFinite(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := FromFloat64(value); !errors.Is(err, ErrNonFiniteFloat) {
			t.Fatalf("got %v for %v", err, value)
		}
		if _, err := NewFromFloat(value); !errors.Is(err, ErrNonFiniteFloat) {
			t.Fatalf("NewFromFloat got %v for %v", err, value)
		}
	}
	got, err := FromFloat64(1000)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "1000" {
		t.Fatalf("from float got %s", got)
	}
	got, err = NewFromFloat(1000)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "1000" {
		t.Fatalf("new from float got %s", got)
	}
	rounded, err := NewFromFloatWithScale(1.235, 2, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.String() != "1.24" {
		t.Fatalf("new from float with scale got %s", rounded)
	}

	bf := big.NewFloat(1.0 / 8.0)
	rounded, err = FromBigFloat(bf, 3, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.String() != "0.125" {
		t.Fatalf("from big float got %s", rounded)
	}
	for _, bf := range []*big.Float{new(big.Float).SetInf(false), new(big.Float).SetInf(true)} {
		if _, err := FromBigFloat(bf, 2, ToNearestEven); !errors.Is(err, ErrNonFiniteFloat) {
			t.Fatalf("FromBigFloat got %v for %v", err, bf)
		}
	}
}

func TestInvalidInputsReturnErrorsWithoutPanic(t *testing.T) {
	calls := []func() error{
		func() error {
			_, err := NewContext(-1, ToNearestEven)
			return err
		},
		func() error {
			_, err := NewContext(2, RoundingMode(99))
			return err
		},
		func() error {
			_, err := NewMoneyContext("usd!", 2, ToNearestEven)
			return err
		},
		func() error {
			_, err := NewMoneyContext("USD", -1, ToNearestEven)
			return err
		},
		func() error {
			_, err := Parse("not-a-decimal")
			return err
		},
		func() error {
			_, err := NewFixed64(1, 19)
			return err
		},
		func() error {
			var d Decimal
			return d.UnmarshalBinary([]byte("bad"))
		},
	}

	for i, call := range calls {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("call %d panicked: %v", i, r)
				}
			}()
			if err := call(); err == nil {
				t.Fatalf("call %d returned nil error", i)
			}
		}()
	}
}

func TestCompatibilityConstructors(t *testing.T) {
	d, err := NewFromString("−12.30")
	if err != nil {
		t.Fatal(err)
	}
	if d.String() != "-12.30" {
		t.Fatalf("NewFromString got %s", d)
	}
	if RequireFromString("1.230").String() != "1.230" {
		t.Fatal("RequireFromString did not preserve scale")
	}
	if _, err := NewFromString("not-decimal"); err == nil {
		t.Fatal("expected NewFromString error")
	}
}

func TestJSONSQLAndNullDecimal(t *testing.T) {
	amount := MustParse("123.4500")
	data, err := json.Marshal(amount)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"123.4500"` {
		t.Fatalf("JSON must be a precision-preserving string, got %s", data)
	}
	data, err = amount.MarshalJSONWithMode(EmitJSONNumber)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `123.4500` {
		t.Fatalf("JSON number mode got %s", data)
	}
	data, err = json.Marshal(AsNumber(amount))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `123.4500` {
		t.Fatalf("JSON number wrapper got %s", data)
	}
	if _, err := amount.MarshalJSONWithMode(JSONMode(99)); err == nil {
		t.Fatal("expected invalid JSON mode error")
	}

	var fromString Decimal
	if err := json.Unmarshal([]byte(`"123.4500"`), &fromString); err != nil {
		t.Fatal(err)
	}
	var fromNumber Decimal
	if err := json.Unmarshal([]byte(`123.4500`), &fromNumber); err != nil {
		t.Fatal(err)
	}
	if fromString.String() != "123.4500" || fromNumber.String() != "123.4500" {
		t.Fatalf("JSON round trip mismatch: %s %s", fromString, fromNumber)
	}
	var wrapped Number
	if err := json.Unmarshal([]byte(`123.4500`), &wrapped); err != nil {
		t.Fatal(err)
	}
	if wrapped.Decimal.String() != "123.4500" {
		t.Fatalf("wrapped number got %s", wrapped.Decimal)
	}
	data, err = json.Marshal(AsExtendedJSON(amount))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"$numberDecimal":"123.4500"}` {
		t.Fatalf("extended JSON got %s", data)
	}
	var extended ExtendedJSON
	if err := json.Unmarshal(data, &extended); err != nil {
		t.Fatal(err)
	}
	if extended.Decimal.String() != "123.4500" {
		t.Fatalf("extended JSON round trip got %s", extended.Decimal)
	}
	if err := json.Unmarshal([]byte(`{"x":"1"}`), &extended); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected extended JSON syntax error, got %v", err)
	}
	if err := json.Unmarshal([]byte(`null`), &fromString); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected null error, got %v", err)
	}

	value, err := amount.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != "123.4500" {
		t.Fatalf("driver value got %#v", value)
	}
	var scanned Decimal
	if err := scanned.Scan([]byte("123.4500")); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "123.4500" {
		t.Fatalf("scan got %s", scanned)
	}
	if err := scanned.Scan(int32(-42)); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "-42" {
		t.Fatalf("scan int32 got %s", scanned)
	}
	if err := scanned.Scan(uint64(18446744073709551615)); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "18446744073709551615" {
		t.Fatalf("scan uint64 got %s", scanned)
	}
	if err := scanned.Scan(json.Number("123.4500")); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "123.4500" {
		t.Fatalf("scan json number got %s", scanned)
	}
	if err := scanned.Scan(float64(1.25)); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("expected float scan source rejection, got %v", err)
	}
	if err := scanned.Scan(nil); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected nil scan error, got %v", err)
	}

	var nullable NullDecimal
	if err := nullable.Scan(nil); err != nil || nullable.Valid {
		t.Fatalf("null scan got %#v err=%v", nullable, err)
	}
	if err := nullable.Scan("1.20"); err != nil || !nullable.Valid || nullable.Decimal.String() != "1.20" {
		t.Fatalf("valid scan got %#v err=%v", nullable, err)
	}
	data, err = json.Marshal(nullable)
	if err != nil || string(data) != `"1.20"` {
		t.Fatalf("nullable JSON got %s err=%v", data, err)
	}
}

func TestNullableTextAndFormatInteroperability(t *testing.T) {
	var decimalNull NullDecimal
	text, err := decimalNull.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "null" || decimalNull.String() != "null" {
		t.Fatalf("null decimal text got %s string=%s", text, decimalNull)
	}
	appended, err := decimalNull.AppendText([]byte("d="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "d=null" {
		t.Fatalf("null decimal append got %s", appended)
	}
	if got := fmt.Sprintf("%q", decimalNull); got != `"null"` {
		t.Fatalf("null decimal quote got %q", got)
	}
	if err := decimalNull.UnmarshalText([]byte("1.2300")); err != nil {
		t.Fatal(err)
	}
	if !decimalNull.Valid || decimalNull.Decimal.String() != "1.2300" {
		t.Fatalf("decimal text decode got %#v", decimalNull)
	}
	text, err = decimalNull.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "1.2300" {
		t.Fatalf("valid decimal text got %s", text)
	}
	appended, err = decimalNull.AppendText([]byte("d="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "d=1.2300" {
		t.Fatalf("valid decimal append got %s", appended)
	}
	if got := fmt.Sprintf("%+.2f", decimalNull); got != "+1.23" {
		t.Fatalf("valid decimal nullable format got %q", got)
	}
	if err := decimalNull.UnmarshalText([]byte("bad")); err == nil || decimalNull.Valid {
		t.Fatalf("expected decimal invalid text reset, got %#v err=%v", decimalNull, err)
	}
	var nilDecimal *NullDecimal
	if err := nilDecimal.UnmarshalText([]byte("null")); err == nil {
		t.Fatal("expected nil NullDecimal text unmarshal error")
	}

	var fixedNull NullFixed64
	if fixedNull.String() != "null" {
		t.Fatalf("null fixed64 string got %s", fixedNull)
	}
	text, err = fixedNull.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "null" {
		t.Fatalf("null fixed64 text got %s", text)
	}
	appended, err = fixedNull.AppendText([]byte("f="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "f=null" {
		t.Fatalf("null fixed64 append got %s", appended)
	}
	if got := fmt.Sprintf("%q", fixedNull); got != `"null"` {
		t.Fatalf("null fixed64 quote got %q", got)
	}
	if err := fixedNull.UnmarshalText([]byte("12.340")); err != nil {
		t.Fatal(err)
	}
	if !fixedNull.Valid || fixedNull.Fixed64.String() != "12.340" {
		t.Fatalf("fixed64 text decode got %#v", fixedNull)
	}
	if fixedNull.String() != "12.340" {
		t.Fatalf("valid fixed64 string got %s", fixedNull)
	}
	text, err = fixedNull.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "12.340" {
		t.Fatalf("fixed64 text got %s", text)
	}
	appended, err = fixedNull.AppendText([]byte("f="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "f=12.340" {
		t.Fatalf("fixed64 append got %s", appended)
	}
	if got := fmt.Sprintf("%+.4f", fixedNull); got != "+12.3400" {
		t.Fatalf("valid fixed64 nullable format got %q", got)
	}
	if err := fixedNull.UnmarshalText([]byte("null")); err != nil || fixedNull.Valid {
		t.Fatalf("fixed64 null text decode got %#v err=%v", fixedNull, err)
	}
	fixedNull = NewNullFixed64(mustFixed64(t, 1, 2))
	if err := fixedNull.UnmarshalText([]byte("bad")); err == nil || fixedNull.Valid {
		t.Fatalf("expected fixed64 invalid text reset, got %#v err=%v", fixedNull, err)
	}
	var nilFixed *NullFixed64
	if err := nilFixed.UnmarshalText([]byte("null")); err == nil {
		t.Fatal("expected nil NullFixed64 text unmarshal error")
	}

	var moneyNull NullMoney
	if moneyNull.String() != "null" {
		t.Fatalf("null money string got %s", moneyNull)
	}
	text, err = moneyNull.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "null" {
		t.Fatalf("null money text got %s", text)
	}
	if got := fmt.Sprintf("%q", moneyNull); got != `"null"` {
		t.Fatalf("null money quote got %q", got)
	}
	if err := moneyNull.UnmarshalText([]byte("USD 10.555")); err != nil {
		t.Fatal(err)
	}
	if !moneyNull.Valid || moneyNull.Money.String() != "USD 10.555" {
		t.Fatalf("money text decode got %#v", moneyNull)
	}
	if moneyNull.String() != "USD 10.555" {
		t.Fatalf("valid money string got %s", moneyNull)
	}
	text, err = moneyNull.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "USD 10.555" {
		t.Fatalf("money text got %s", text)
	}
	appended, err = moneyNull.AppendText([]byte("m="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "m=USD 10.555" {
		t.Fatalf("money append got %s", appended)
	}
	if got := fmt.Sprintf("%.2f", moneyNull); got != "USD 10.56" {
		t.Fatalf("valid money nullable format got %q", got)
	}
	if err := moneyNull.UnmarshalText([]byte("null")); err != nil || moneyNull.Valid {
		t.Fatalf("money null text decode got %#v err=%v", moneyNull, err)
	}
	appended, err = moneyNull.AppendText([]byte("m="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "m=null" {
		t.Fatalf("null money append got %s", appended)
	}
	moneyNull = NewNullMoney(MustParseMoney("USD 1.00"))
	if err := moneyNull.UnmarshalText([]byte("USD bad")); err == nil || moneyNull.Valid {
		t.Fatalf("expected money invalid text reset, got %#v err=%v", moneyNull, err)
	}
	var nilMoney *NullMoney
	if err := nilMoney.UnmarshalText([]byte("null")); err == nil {
		t.Fatal("expected nil NullMoney text unmarshal error")
	}
}

func TestFormatterHelpersAndMapKey(t *testing.T) {
	d := MustParse("12.345")
	if got := fmt.Sprintf("%s", d); got != "12.345" {
		t.Fatalf("format s got %s", got)
	}
	if got := fmt.Sprintf("%.2f", d); got != "12.34" {
		t.Fatalf("format f got %s", got)
	}
	if got := fmt.Sprintf("%10s", d); got != "    12.345" {
		t.Fatalf("format width got %q", got)
	}
	if got := fmt.Sprintf("%q", d); got != `"12.345"` {
		t.Fatalf("format quote got %s", got)
	}

	keyed := map[string]string{
		MustParse("1.0").Key(): "same",
	}
	if keyed[MustParse("1.00").Key()] != "same" {
		t.Fatal("normalized key mismatch")
	}
}

func TestFormatterAdvancedFlagsAndLimits(t *testing.T) {
	d := MustParse("12.345")
	if got := fmt.Sprintf("%+010.2f", d); got != "+000012.34" {
		t.Fatalf("format signed zero-padded positive got %q", got)
	}
	if got := fmt.Sprintf("%010.2f", MustParse("-12.345")); got != "-000012.34" {
		t.Fatalf("format signed zero-padded negative got %q", got)
	}
	if got := fmt.Sprintf("% .1f", d); got != " 12.3" {
		t.Fatalf("format space sign got %q", got)
	}
	if got := fmt.Sprintf("%10q", d); got != `  "12.345"` {
		t.Fatalf("format quoted width got %q", got)
	}
	if got := fmt.Sprintf("%-8s", d); got != "12.345  " {
		t.Fatalf("format left padded got %q", got)
	}
	if got := fmt.Sprintf("%x", d); got != "%!x(qdecimal.Decimal=12.345)" {
		t.Fatalf("format unsupported verb got %q", got)
	}
	if got := fmt.Sprintf("%.4097f", d); !strings.Contains(got, ErrLimitExceeded.Error()) {
		t.Fatalf("expected format precision limit error, got %q", got)
	}

	fixed, err := NewFixed64(12345, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%+.4f", fixed); got != "+123.4500" {
		t.Fatalf("fixed format signed precision got %q", got)
	}
	if got := fmt.Sprintf("%.19f", fixed); got != "123.4500000000000000000" {
		t.Fatalf("fixed format high precision got %q", got)
	}

	money, err := NewMoney(MustParse("123.455"), "usd")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%+.2f", money); got != "USD +123.46" {
		t.Fatalf("money format signed precision got %q", got)
	}
	if got := fmt.Sprintf("%+q", money); got != `"USD +123.455"` {
		t.Fatalf("money format signed quote got %q", got)
	}
	if got := fmt.Sprintf("%q", Money{}); !strings.HasPrefix(got, `"<invalid money: qdecimal: invalid currency code`) {
		t.Fatalf("invalid money quote got %q", got)
	}
}

func TestCompareBetweenMinMaxClamp(t *testing.T) {
	one := MustParse("1.00")
	two := MustParse("2")
	three := MustParse("3.0")

	if !two.Between(one, three, true) || one.Between(one, three, false) {
		t.Fatal("between failed")
	}
	if Min(three, one, two).String() != "1.00" {
		t.Fatal("min failed")
	}
	if Max(one, three, two).String() != "3.0" {
		t.Fatal("max failed")
	}
	if MustParse("5").Clamp(one, three).String() != "3.0" {
		t.Fatal("clamp failed")
	}
}

func TestDefensiveCopiesAndConcurrentUse(t *testing.T) {
	d := MustParse("10.00")
	coef := d.Coefficient()
	coef.SetInt64(999)
	if d.String() != "10.00" {
		t.Fatal("coefficient copy mutated decimal")
	}

	before := d.String()
	_ = d.Add(MustParse("1.00"))
	if d.String() != before {
		t.Fatal("operation mutated receiver")
	}

	const workers = 64
	const iterations = 512
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				got, err := d.Div(MustParse("4"), 2, ToNearestEven)
				if err != nil {
					errs <- err
					return
				}
				if got.String() != "2.50" || d.String() != before {
					errs <- fmt.Errorf("concurrent operation mismatch")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestAddSubMulMatchBigRat(t *testing.T) {
	cfg := &quick.Config{MaxCount: 500}
	err := quick.Check(func(a, b int32, as, bs uint8) bool {
		left, err := New(int64(a), int32(as%6))
		if err != nil {
			return false
		}
		right, err := New(int64(b), int32(bs%6))
		if err != nil {
			return false
		}

		addRat := new(big.Rat).Add(left.Rat(), right.Rat())
		subRat := new(big.Rat).Sub(left.Rat(), right.Rat())
		mulRat := new(big.Rat).Mul(left.Rat(), right.Rat())

		return left.Add(right).Rat().Cmp(addRat) == 0 &&
			left.Sub(right).Rat().Cmp(subRat) == 0 &&
			left.Mul(right).Rat().Cmp(mulRat) == 0
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDivMatchesBigRatRoundedToScale(t *testing.T) {
	cfg := &quick.Config{MaxCount: 500}
	err := quick.Check(func(a, b int16, as, bs, scale uint8) bool {
		if b == 0 {
			b = 1
		}
		left, _ := New(int64(a), int32(as%4))
		right, _ := New(int64(b), int32(bs%4))
		outScale := int32(scale % 6)

		got, err := left.Div(right, outScale, ToNearestEven)
		if err != nil {
			return false
		}
		wantRat := new(big.Rat).Quo(left.Rat(), right.Rat())
		want, err := FromRat(wantRat, outScale, ToNearestEven)
		if err != nil {
			return false
		}
		return got.Equal(want) && got.Scale() == outScale
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRescaleAllRoundingModesMatchRatOracle(t *testing.T) {
	modes := []RoundingMode{
		ToNearestEven,
		ToNearestAway,
		ToNearestTowardZero,
		AwayFromZero,
		TowardZero,
		TowardPositive,
		TowardNegative,
	}
	cfg := &quick.Config{MaxCount: 500}

	for _, mode := range modes {
		mode := mode
		err := quick.Check(func(coef int32, sourceScale, targetScale uint8) bool {
			d, err := New(int64(coef), int32(sourceScale%8))
			if err != nil {
				return false
			}
			scale := int32(targetScale % 8)
			got, err := d.Rescale(scale, mode)
			if err != nil {
				return false
			}
			want, err := roundRatOracle(d.Rat(), scale, mode)
			if err != nil {
				return false
			}
			return got.Equal(want) && got.Scale() == scale
		}, cfg)
		if err != nil {
			t.Fatalf("mode %d: %v", mode, err)
		}
	}
}

func roundRatOracle(r *big.Rat, scale int32, mode RoundingMode) (Decimal, error) {
	if scale < 0 {
		return Decimal{}, ErrInvalidScale
	}
	if !mode.valid() {
		return Decimal{}, ErrInvalidRoundingMode
	}
	scaled := new(big.Rat).Mul(new(big.Rat).Set(r), new(big.Rat).SetInt(pow10(scale)))
	num := scaled.Num()
	den := scaled.Denom()
	var q, rem big.Int
	q.QuoRem(num, den, &rem)

	sign := scaled.Sign()
	discarded := rem.Sign() != 0
	increment := false
	switch mode {
	case AwayFromZero:
		increment = discarded
	case TowardZero:
		increment = false
	case TowardPositive:
		increment = sign > 0 && discarded
	case TowardNegative:
		increment = sign < 0 && discarded
	case ToNearestEven, ToNearestAway, ToNearestTowardZero:
		var twiceRem big.Int
		twiceRem.Mul(new(big.Int).Abs(&rem), big.NewInt(2))
		cmp := twiceRem.Cmp(den)
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
			q.Add(&q, big.NewInt(1))
		} else {
			q.Sub(&q, big.NewInt(1))
		}
	}
	return NewFromBigInt(&q, scale)
}
