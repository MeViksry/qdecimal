package qdecimal

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"encoding"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
)

var (
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

	_ json.Marshaler   = NullFixed64{}
	_ json.Unmarshaler = (*NullFixed64)(nil)
	_ sql.Scanner      = (*NullFixed64)(nil)
	_ driver.Valuer    = NullFixed64{}
)

func TestFixed64ConstructionAndHotPath(t *testing.T) {
	amount, err := NewFixed64(12345, 2)
	if err != nil {
		t.Fatal(err)
	}
	if amount.Units() != 12345 || amount.Scale() != 2 {
		t.Fatalf("fixed metadata got units=%d scale=%d", amount.Units(), amount.Scale())
	}
	if amount.String() != "123.45" {
		t.Fatalf("string got %s", amount)
	}
	text, err := amount.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "123.45" {
		t.Fatalf("marshal text got %s", text)
	}
	appended, err := amount.AppendText([]byte("fixed="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "fixed=123.45" {
		t.Fatalf("append text got %s", appended)
	}
	reusedText := make([]byte, 0, 32)
	wantText := []byte("123.45")
	allocs := testing.AllocsPerRun(1000, func() {
		var err error
		reusedText, err = amount.AppendText(reusedText[:0])
		if err != nil {
			panic(err)
		}
		if !bytes.Equal(reusedText, wantText) {
			panic("bad fixed64 append text")
		}
	})
	if allocs != 0 {
		t.Fatalf("expected zero fixed64 append text allocations, got %.2f", allocs)
	}
	if amount.Decimal().String() != "123.45" {
		t.Fatalf("decimal got %s", amount.Decimal())
	}

	rounded, err := ParseFixed64("123.456", 2, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.Units() != 12346 || rounded.String() != "123.46" {
		t.Fatalf("rounded got units=%d string=%s", rounded.Units(), rounded)
	}

	zero, err := NewFixed64(0, 4)
	if err != nil {
		t.Fatal(err)
	}
	if zero.String() != "0.0000" {
		t.Fatalf("zero scale was not preserved: %s", zero)
	}

	tiny, err := ParseFixed64("1.2001", 2, AwayFromZero)
	if err != nil {
		t.Fatal(err)
	}
	if tiny.String() != "1.21" {
		t.Fatalf("discarded non-zero rounding got %s", tiny)
	}
	halfEven, err := ParseFixed64("1.2050", 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if halfEven.String() != "1.20" {
		t.Fatalf("half-even tie got %s", halfEven)
	}
	unicodeMinus, err := ParseFixed64("−1.20", 2, TowardZero)
	if err != nil {
		t.Fatal(err)
	}
	if unicodeMinus.String() != "-1.20" {
		t.Fatalf("unicode minus fallback got %s", unicodeMinus)
	}

	min, err := ParseFixed64("-9223372036854775808", 0, TowardZero)
	if err != nil {
		t.Fatal(err)
	}
	if min.Units() != math.MinInt64 {
		t.Fatalf("min int64 parse got %d", min.Units())
	}
	if _, err := ParseFixed64("9223372036854775807.5", 0, ToNearestAway); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected rounded parse overflow, got %v", err)
	}

	if _, err := NewFixed64(1, 19); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid fixed scale, got %v", err)
	}
	if _, err := ParseFixed64("1.23", 2, RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid rounding mode, got %v", err)
	}
}

func TestFixed64ArithmeticAndRounding(t *testing.T) {
	a, _ := NewFixed64(120, 2)
	b, _ := NewFixed64(34, 3)
	sum, err := a.Add(b)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "1.234" || sum.Scale() != 3 {
		t.Fatalf("aligned add got %s scale=%d", sum, sum.Scale())
	}

	diff, err := a.Sub(b)
	if err != nil {
		t.Fatal(err)
	}
	if diff.String() != "1.166" {
		t.Fatalf("aligned sub got %s", diff)
	}

	rounded, err := sum.Round(2, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.String() != "1.23" {
		t.Fatalf("round got %s", rounded)
	}
	ceiled, err := mustFixed64(t, -1234, 3).Ceil(2)
	if err != nil {
		t.Fatal(err)
	}
	if ceiled.String() != "-1.23" {
		t.Fatalf("ceil negative got %s", ceiled)
	}
	floored, err := mustFixed64(t, -1234, 3).Floor(2)
	if err != nil {
		t.Fatal(err)
	}
	if floored.String() != "-1.24" {
		t.Fatalf("floor negative got %s", floored)
	}

	price := mustFixed64(t, 1234, 2)
	rate := mustFixed64(t, 50, 3)
	product, err := price.Mul(rate, 4, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if product.String() != "0.6170" {
		t.Fatalf("mul got %s", product)
	}

	third, err := mustFixed64(t, 100, 2).Div(mustFixed64(t, 3, 0), 4, ToNearestTowardZero)
	if err != nil {
		t.Fatal(err)
	}
	if third.String() != "0.3333" {
		t.Fatalf("div got %s", third)
	}
	if _, err := third.Div(Fixed64{}, 2, ToNearestEven); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected division by zero, got %v", err)
	}
}

func TestFixed64QuantizeStep(t *testing.T) {
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
		{"123.4567", "0.0005", ToNearestEven, "123.4565"},
		{"123.4568", "-0.0005", ToNearestAway, "123.4570"},
	}

	for _, tt := range tests {
		value := mustParseFixed64(t, tt.value)
		step := mustParseFixed64(t, tt.step)
		got, err := value.QuantizeStep(step, tt.mode)
		if err != nil {
			t.Fatalf("%s step %s: %v", tt.value, tt.step, err)
		}
		if got.String() != tt.want || got.Scale() != step.Scale() {
			t.Fatalf("%s step %s got %s scale=%d want %s scale=%d", tt.value, tt.step, got, got.Scale(), tt.want, step.Scale())
		}
	}

	value := mustParseFixed64(t, "123.456")
	step := mustParseFixed64(t, "0.05")
	fastAllocs := testing.AllocsPerRun(1000, func() {
		got, err := value.QuantizeStep(step, ToNearestEven)
		if err != nil {
			panic(err)
		}
		if got.Units() != 12345 || got.Scale() != 2 {
			panic("bad fixed64 quantize step")
		}
	})
	if fastAllocs != 0 {
		t.Fatalf("expected zero fixed64 quantize-step allocations, got %.2f", fastAllocs)
	}

	wideScale := mustFixed64(t, 9_000_000_000_000_000_000, 18)
	wideStep := mustFixed64(t, 10, 0)
	got, err := wideScale.QuantizeStep(wideStep, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "10" || got.Scale() != 0 {
		t.Fatalf("fallback quantize-step got %s scale=%d", got, got.Scale())
	}

	if _, err := value.QuantizeStep(Fixed64{}, ToNearestEven); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected zero fixed64 step error, got %v", err)
	}
	if _, err := value.QuantizeStep(step, RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid fixed64 step rounding mode, got %v", err)
	}
}

func TestFixed64RangeHelpers(t *testing.T) {
	low := mustFixed64(t, 1000, 2)
	mid := mustFixed64(t, 12500, 3)
	high := mustFixed64(t, 1500, 2)

	if !mid.Between(low, high, true) {
		t.Fatal("expected fixed64 inside inclusive range")
	}
	if low.Between(low, high, false) {
		t.Fatal("expected fixed64 exclusive range to reject lower bound")
	}
	if !mid.Between(high, low, true) {
		t.Fatal("expected fixed64 reversed bounds to be accepted")
	}

	clamped := mustFixed64(t, 999, 2).Clamp(high, low)
	if !clamped.Equal(low) || clamped.Scale() != low.Scale() {
		t.Fatalf("fixed64 clamp low got %s scale=%d", clamped, clamped.Scale())
	}
	clamped = mustFixed64(t, 2000, 2).Clamp(low, high)
	if !clamped.Equal(high) || clamped.Scale() != high.Scale() {
		t.Fatalf("fixed64 clamp high got %s scale=%d", clamped, clamped.Scale())
	}
	clamped = mid.Clamp(low, high)
	if !clamped.Equal(mid) || clamped.Scale() != mid.Scale() {
		t.Fatalf("fixed64 clamp inside got %s scale=%d", clamped, clamped.Scale())
	}

	wideLow := mustFixed64(t, 100, 1)
	clamped = mustFixed64(t, 999, 2).Clamp(wideLow, high)
	if !clamped.Equal(wideLow) || clamped.Scale() != 1 {
		t.Fatalf("fixed64 clamp should preserve bound scale, got %s scale=%d", clamped, clamped.Scale())
	}

	min := MinFixed64(high, mid, low)
	if !min.Equal(low) {
		t.Fatalf("MinFixed64 got %s", min)
	}
	max := MaxFixed64(low, mid, high)
	if !max.Equal(high) {
		t.Fatalf("MaxFixed64 got %s", max)
	}
	if got := MinFixed64(); got.String() != "0" {
		t.Fatalf("empty MinFixed64 got %s", got)
	}
	if got := MaxFixed64(); got.String() != "0" {
		t.Fatalf("empty MaxFixed64 got %s", got)
	}
}

func TestFixed64Aggregates(t *testing.T) {
	values := []Fixed64{
		mustFixed64(t, 120, 2),
		mustFixed64(t, 340, 2),
		mustFixed64(t, 290, 2),
	}
	sum, err := SumFixed64(values...)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "7.50" || sum.Scale() != 2 {
		t.Fatalf("SumFixed64 got %s scale=%d", sum, sum.Scale())
	}

	avg, err := AvgFixed64(values, 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if avg.String() != "2.50" || avg.Scale() != 2 {
		t.Fatalf("AvgFixed64 got %s scale=%d", avg, avg.Scale())
	}
	exactAvg, err := AvgFixed64Exact(values, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !exactAvg.Equal(avg) || exactAvg.Scale() != 2 {
		t.Fatalf("AvgFixed64Exact got %s scale=%d want %s", exactAvg, exactAvg.Scale(), avg)
	}
	fastAllocs := testing.AllocsPerRun(1000, func() {
		got, err := SumFixed64(values...)
		if err != nil {
			panic(err)
		}
		if got.Units() != 750 || got.Scale() != 2 {
			panic("bad fixed64 fast sum")
		}
		got, err = AvgFixed64(values, 2, ToNearestEven)
		if err != nil {
			panic(err)
		}
		if got.Units() != 250 || got.Scale() != 2 {
			panic("bad fixed64 fast average")
		}
	})
	if fastAllocs != 0 {
		t.Fatalf("expected zero fixed64 fast aggregate allocations, got %.2f", fastAllocs)
	}

	negativeAvg, err := AvgFixed64([]Fixed64{mustFixed64(t, -1, 0), mustFixed64(t, -2, 0)}, 0, TowardNegative)
	if err != nil {
		t.Fatal(err)
	}
	if negativeAvg.String() != "-2" {
		t.Fatalf("negative fixed64 average floor got %s", negativeAvg)
	}
	negativeAvg, err = AvgFixed64([]Fixed64{mustFixed64(t, -1, 0), mustFixed64(t, -2, 0)}, 0, TowardPositive)
	if err != nil {
		t.Fatal(err)
	}
	if negativeAvg.String() != "-1" {
		t.Fatalf("negative fixed64 average ceil got %s", negativeAvg)
	}

	mixed := []Fixed64{
		mustFixed64(t, 120, 2),
		mustFixed64(t, 34, 3),
	}
	sum, err = SumFixed64(mixed...)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "1.234" || sum.Scale() != 3 {
		t.Fatalf("mixed SumFixed64 got %s scale=%d", sum, sum.Scale())
	}
	avg, err = AvgFixed64(mixed, 3, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if avg.String() != "0.617" || avg.Scale() != 3 {
		t.Fatalf("mixed AvgFixed64 got %s scale=%d", avg, avg.Scale())
	}

	half, err := AvgFixed64Exact([]Fixed64{mustFixed64(t, 1, 0), mustFixed64(t, 2, 0)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if half.String() != "1.5" {
		t.Fatalf("exact half got %s", half)
	}
	if _, err := AvgFixed64Exact([]Fixed64{mustFixed64(t, 1, 0), mustFixed64(t, 2, 0)}, 0); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact scale error, got %v", err)
	}

	hugeAvg, err := AvgFixed64([]Fixed64{
		mustFixed64(t, math.MaxInt64, 0),
		mustFixed64(t, math.MaxInt64, 0),
	}, 0, TowardZero)
	if err != nil {
		t.Fatal(err)
	}
	if hugeAvg.Units() != math.MaxInt64 {
		t.Fatalf("overflow fallback average got %d", hugeAvg.Units())
	}
	if _, err := SumFixed64(mustFixed64(t, math.MaxInt64, 0), mustFixed64(t, math.MaxInt64, 0)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected overflowing SumFixed64 error, got %v", err)
	}

	if _, err := SumFixed64(); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected empty SumFixed64 error, got %v", err)
	}
	if _, err := AvgFixed64(nil, 2, ToNearestEven); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected empty AvgFixed64 error, got %v", err)
	}
	if _, err := AvgFixed64(values, 19, ToNearestEven); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid AvgFixed64 scale, got %v", err)
	}
	if _, err := AvgFixed64(values, 2, RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid AvgFixed64 rounding mode, got %v", err)
	}
}

func TestFixed64JSONTextAndSQL(t *testing.T) {
	amount := mustFixed64(t, 12345, 2)
	data, err := json.Marshal(amount)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"123.45"` {
		t.Fatalf("json got %s", data)
	}

	var fromNumber Fixed64
	if err := json.Unmarshal([]byte(`123.450`), &fromNumber); err != nil {
		t.Fatal(err)
	}
	if fromNumber.String() != "123.450" || fromNumber.Scale() != 3 {
		t.Fatalf("json number got %s scale=%d", fromNumber, fromNumber.Scale())
	}

	text, err := amount.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	var fromText Fixed64
	if err := fromText.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}
	if !fromText.Equal(amount) || fromText.Scale() != amount.Scale() {
		t.Fatalf("text round trip got %s scale=%d", fromText, fromText.Scale())
	}

	value, err := amount.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != "123.45" {
		t.Fatalf("driver value got %#v", value)
	}
	var scanned Fixed64
	if err := scanned.Scan([]byte("123.45")); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "123.45" {
		t.Fatalf("scan got %s", scanned)
	}
	if err := scanned.Scan(int16(-42)); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "-42" || scanned.Scale() != 0 {
		t.Fatalf("scan int16 got %s scale=%d", scanned, scanned.Scale())
	}
	if err := scanned.Scan(uint8(99)); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "99" || scanned.Scale() != 0 {
		t.Fatalf("scan uint8 got %s scale=%d", scanned, scanned.Scale())
	}
	if err := scanned.Scan(nil); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected nil scan error, got %v", err)
	}

	var nullable NullFixed64
	if !nullable.IsZero() {
		t.Fatal("zero NullFixed64 should be zero/null")
	}
	if err := nullable.Scan(nil); err != nil || nullable.Valid {
		t.Fatalf("null fixed64 scan got %#v err=%v", nullable, err)
	}
	if err := nullable.Scan([]byte("123.45")); err != nil {
		t.Fatal(err)
	}
	if !nullable.Valid || nullable.Fixed64.String() != "123.45" {
		t.Fatalf("valid fixed64 scan got %#v", nullable)
	}
	data, err = json.Marshal(nullable)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"123.45"` {
		t.Fatalf("null fixed64 JSON got %s", data)
	}
	if err := json.Unmarshal([]byte(`null`), &nullable); err != nil || nullable.Valid {
		t.Fatalf("null fixed64 JSON decode got %#v err=%v", nullable, err)
	}
	nullable = NewNullFixed64(amount)
	if nullable.IsZero() {
		t.Fatal("valid NullFixed64 should not be zero/null")
	}
	value, err = nullable.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != "123.45" {
		t.Fatalf("null fixed64 SQL value got %#v", value)
	}
	nullable.Valid = false
	value, err = nullable.Value()
	if err != nil || value != nil {
		t.Fatalf("invalid null fixed64 value got %#v err=%v", value, err)
	}
}

func TestFixed64BinaryAndGobRoundTrip(t *testing.T) {
	amount := mustFixed64(t, -12345, 3)
	data, err := amount.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != amount.BinarySize() {
		t.Fatalf("fixed64 binary size got %d want %d", len(data), amount.BinarySize())
	}
	appended, err := amount.AppendBinary(make([]byte, 0, amount.BinarySize()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(appended, data) {
		t.Fatalf("fixed64 append binary mismatch got %x want %x", appended, data)
	}
	var decoded Fixed64
	if err := decoded.UnmarshalBinary(data); err != nil {
		t.Fatal(err)
	}
	if decoded.String() != "-12.345" || decoded.Scale() != 3 {
		t.Fatalf("binary round trip got %s scale=%d", decoded, decoded.Scale())
	}
	if err := decoded.UnmarshalBinary([]byte("bad")); err == nil {
		t.Fatal("expected fixed64 binary syntax error")
	}

	reused := make([]byte, 0, amount.BinarySize())
	allocs := testing.AllocsPerRun(1000, func() {
		var err error
		reused, err = amount.AppendBinary(reused[:0])
		if err != nil {
			panic(err)
		}
		if len(reused) != amount.BinarySize() {
			panic("bad fixed64 append binary size")
		}
	})
	if allocs != 0 {
		t.Fatalf("expected zero fixed64 append allocations, got %.2f", allocs)
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(amount); err != nil {
		t.Fatal(err)
	}
	var gobDecoded Fixed64
	if err := gob.NewDecoder(&buf).Decode(&gobDecoded); err != nil {
		t.Fatal(err)
	}
	if !gobDecoded.Equal(amount) || gobDecoded.Scale() != amount.Scale() {
		t.Fatalf("gob round trip got %s scale=%d", gobDecoded, gobDecoded.Scale())
	}
}

func TestFixed64FormatterAndKey(t *testing.T) {
	amount := mustFixed64(t, 12345, 3)
	if got := fmt.Sprintf("%s", amount); got != "12.345" {
		t.Fatalf("format s got %s", got)
	}
	if got := fmt.Sprintf("%.2f", amount); got != "12.34" {
		t.Fatalf("format f got %s", got)
	}
	if got := fmt.Sprintf("%10s", amount); got != "    12.345" {
		t.Fatalf("format width got %q", got)
	}
	if got := fmt.Sprintf("%q", amount); got != `"12.345"` {
		t.Fatalf("format quote got %s", got)
	}

	keyed := map[string]string{
		mustFixed64(t, 100, 1).Key(): "same",
	}
	if keyed[mustFixed64(t, 1000, 2).Key()] != "same" {
		t.Fatal("fixed64 normalized key mismatch")
	}
}

func TestFixed64OverflowBoundaries(t *testing.T) {
	max := mustFixed64(t, math.MaxInt64, 0)
	if _, err := max.Add(mustFixed64(t, 1, 0)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected add overflow, got %v", err)
	}

	min := mustFixed64(t, math.MinInt64, 0)
	if _, err := min.Neg(); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected neg overflow, got %v", err)
	}

	if _, err := mustFixed64(t, math.MaxInt64/10+1, 0).Add(mustFixed64(t, 1, 1)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected scale alignment overflow, got %v", err)
	}

	huge := MustParse("9223372036854775808")
	if _, err := Fixed64FromDecimal(huge, 0, TowardZero); !errors.Is(err, ErrOverflow) {
		t.Fatalf("expected decimal conversion overflow, got %v", err)
	}
	if _, err := Fixed64FromDecimal(One, 19, TowardZero); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid scale, got %v", err)
	}
}

func mustFixed64(t *testing.T, units int64, scale int32) Fixed64 {
	t.Helper()
	out, err := NewFixed64(units, scale)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustParseFixed64(t *testing.T, text string) Fixed64 {
	t.Helper()
	d := MustParse(text)
	out, err := Fixed64FromDecimal(d, d.Scale(), TowardZero)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
