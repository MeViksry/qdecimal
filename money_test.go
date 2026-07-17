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
	"testing"
)

var (
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
	_ json.Marshaler             = NullMoney{}
	_ json.Unmarshaler           = (*NullMoney)(nil)
	_ sql.Scanner                = (*NullMoney)(nil)
	_ driver.Valuer              = NullMoney{}
)

func TestMoneyConstructionAndCurrencySafety(t *testing.T) {
	usd, err := NewMoney(MustParse("10.00"), " usd ")
	if err != nil {
		t.Fatal(err)
	}
	if usd.Currency() != "USD" || usd.Amount().String() != "10.00" {
		t.Fatalf("money got %s amount=%s", usd.Currency(), usd.Amount())
	}
	if usd.String() != "USD 10.00" {
		t.Fatalf("string got %s", usd)
	}

	alsoUSD, _ := NewMoney(MustParse("2.50"), "USD")
	sum, err := usd.Add(alsoUSD)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "USD 12.50" {
		t.Fatalf("sum got %s", sum)
	}

	btc, _ := NewMoney(MustParse("1.00"), "BTC")
	if _, err := usd.Add(btc); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected currency mismatch, got %v", err)
	}
	if _, err := usd.Cmp(btc); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected cmp currency mismatch, got %v", err)
	}
	if _, err := NewMoney(MustParse("1"), "US-D"); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid currency, got %v", err)
	}
	if _, err := (Money{}).Add(Money{}); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected zero-value money to be invalid, got %v", err)
	}
	if _, err := json.Marshal(Money{}); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected zero-value money marshal error, got %v", err)
	}
	if _, err := json.Marshal(Money{amount: One, currency: "usd"}); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected non-normalized money marshal error, got %v", err)
	}
}

func TestParseMoneyCompatibility(t *testing.T) {
	money, err := ParseMoney("usd 123.4500")
	if err != nil {
		t.Fatal(err)
	}
	if money.String() != "USD 123.4500" {
		t.Fatalf("ParseMoney got %s", money)
	}
	if MustParseMoney("BTC 1.23456789").String() != "BTC 1.23456789" {
		t.Fatal("MustParseMoney mismatch")
	}
	for _, input := range []string{"USD", "USD not-decimal", "US-D 1.00"} {
		if _, err := ParseMoney(input); err == nil {
			t.Fatalf("expected ParseMoney error for %q", input)
		}
	}
}

func TestMoneyRoundingScalingAndMinorUnits(t *testing.T) {
	money, _ := NewMoney(MustParse("123.4567"), "USD")
	rounded, err := money.Round(2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.String() != "USD 123.46" {
		t.Fatalf("rounded got %s", rounded)
	}
	exact, err := NewMoney(MustParse("123.4500"), "USD")
	if err != nil {
		t.Fatal(err)
	}
	exact, err = exact.RoundExact(2)
	if err != nil {
		t.Fatal(err)
	}
	if exact.String() != "USD 123.45" || exact.Amount().Scale() != 2 {
		t.Fatalf("exact money round got %s scale=%d", exact, exact.Amount().Scale())
	}
	if _, err := money.RoundExact(2); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact money round inexact, got %v", err)
	}

	fee, err := money.Mul(MustParse("0.0025"), 4, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if fee.String() != "USD 0.3086" {
		t.Fatalf("fee got %s", fee)
	}

	half, err := money.Div(MustParse("2"), 3, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if half.String() != "USD 61.728" {
		t.Fatalf("div got %s", half)
	}

	ticked, err := money.QuantizeStep(MustParse("0.05"), ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if ticked.String() != "USD 123.45" {
		t.Fatalf("tick got %s", ticked)
	}
	exactTick, err := ticked.QuantizeStepExact(MustParse("-0.05"))
	if err != nil {
		t.Fatal(err)
	}
	if exactTick.String() != "USD 123.45" {
		t.Fatalf("exact money tick got %s", exactTick)
	}
	if _, err := money.QuantizeStepExact(MustParse("0.05")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected inexact money tick, got %v", err)
	}
	if _, err := money.QuantizeStepExact(Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected exact money zero tick, got %v", err)
	}

	units, err := rounded.Int64MinorUnits(2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if units != 12346 {
		t.Fatalf("minor units got %d", units)
	}
	exactUnits, err := exact.Int64MinorUnitsExact(2)
	if err != nil {
		t.Fatal(err)
	}
	if exactUnits != 12345 {
		t.Fatalf("exact money minor units got %d", exactUnits)
	}
	if _, err := money.Int64MinorUnitsExact(2); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact money minor units inexact, got %v", err)
	}
	bigUnits, err := rounded.MinorUnits(2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if bigUnits.Cmp(big.NewInt(12346)) != 0 {
		t.Fatalf("big minor units got %s", bigUnits)
	}
}

func TestMoneyAllocationPreservesRoundedTotal(t *testing.T) {
	amount, _ := NewMoney(MustParse("10.00"), "USD")
	parts, err := amount.Allocate(3, 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"USD 3.34", "USD 3.33", "USD 3.33"}
	assertMoneyStrings(t, parts, want)
	assertMoneyTotal(t, parts, "USD 10.00")

	negative, _ := NewMoney(MustParse("-10.00"), "USD")
	parts, err = negative.Allocate(3, 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"USD -3.34", "USD -3.33", "USD -3.33"}
	assertMoneyStrings(t, parts, want)
	assertMoneyTotal(t, parts, "USD -10.00")

	small, _ := NewMoney(MustParse("0.05"), "USD")
	parts, err = small.AllocateRatios([]int64{1, 1}, 2, TowardZero)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"USD 0.03", "USD 0.02"}
	assertMoneyStrings(t, parts, want)
	assertMoneyTotal(t, parts, "USD 0.05")

	ratio, _ := NewMoney(MustParse("10.00"), "USD")
	parts, err = ratio.AllocateRatios([]int64{3, 2, 1}, 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"USD 5.00", "USD 3.33", "USD 1.67"}
	assertMoneyStrings(t, parts, want)
	assertMoneyTotal(t, parts, "USD 10.00")

	if _, err := amount.Allocate(0, 2, ToNearestEven); !errors.Is(err, ErrInvalidAllocation) {
		t.Fatalf("expected invalid equal allocation, got %v", err)
	}
	if _, err := amount.AllocateRatios([]int64{1, -1}, 2, ToNearestEven); !errors.Is(err, ErrInvalidAllocation) {
		t.Fatalf("expected invalid ratio allocation, got %v", err)
	}
	if _, err := amount.AllocateRatios([]int64{0, 0}, 2, ToNearestEven); !errors.Is(err, ErrInvalidAllocation) {
		t.Fatalf("expected zero ratio allocation error, got %v", err)
	}
}

func TestMoneyJSONAndText(t *testing.T) {
	money, _ := NewMoney(MustParse("123.4500"), "usd")
	data, err := json.Marshal(money)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"amount":"123.4500","currency":"USD"}` {
		t.Fatalf("money JSON got %s", data)
	}

	var decoded Money
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(money) {
		t.Fatalf("money JSON round trip got %s", decoded)
	}
	if err := json.Unmarshal([]byte(`{"amount":"1.00","currency":"US-D"}`), &decoded); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid currency JSON error, got %v", err)
	}

	text, err := money.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "USD 123.4500" {
		t.Fatalf("money text got %s", text)
	}
	appended, err := money.AppendText([]byte("money="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "money=USD 123.4500" {
		t.Fatalf("money append text got %s", appended)
	}
	var fromText Money
	if err := fromText.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}
	if !fromText.Equal(money) {
		t.Fatalf("money text round trip got %s", fromText)
	}
	if err := fromText.UnmarshalText([]byte("USD")); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected invalid text syntax, got %v", err)
	}
}

func TestMoneyFormatterAndKey(t *testing.T) {
	money, _ := NewMoney(MustParse("123.455"), "usd")
	if got := fmt.Sprintf("%s", money); got != "USD 123.455" {
		t.Fatalf("format s got %s", got)
	}
	if got := fmt.Sprintf("%.2f", money); got != "USD 123.46" {
		t.Fatalf("format f got %s", got)
	}
	if got := fmt.Sprintf("%15s", money); got != "    USD 123.455" {
		t.Fatalf("format width got %q", got)
	}
	if got := fmt.Sprintf("%q", money); got != `"USD 123.455"` {
		t.Fatalf("format quote got %s", got)
	}

	left, _ := NewMoney(MustParse("1.0"), "USD")
	right, _ := NewMoney(MustParse("1.00"), "USD")
	other, _ := NewMoney(MustParse("1.00"), "EUR")
	keyed := map[string]string{left.Key(): "usd"}
	if keyed[right.Key()] != "usd" {
		t.Fatal("money normalized key mismatch")
	}
	if right.Key() == other.Key() {
		t.Fatal("money key must include currency")
	}
}

func TestMoneyRangeHelpers(t *testing.T) {
	low := MustParseMoney("USD 10.00")
	mid := MustParseMoney("USD 12.50")
	high := MustParseMoney("USD 15.00")
	other := MustParseMoney("EUR 12.50")

	ok, err := mid.Between(low, high, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected money to be inside inclusive range")
	}
	ok, err = low.Between(low, high, false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected exclusive range to reject lower bound")
	}
	ok, err = mid.Between(high, low, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected reversed bounds to be accepted")
	}
	if _, err := mid.Between(low, other, true); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected between currency mismatch, got %v", err)
	}

	clamped, err := MustParseMoney("USD 9.99").Clamp(high, low)
	if err != nil {
		t.Fatal(err)
	}
	if !clamped.Equal(low) {
		t.Fatalf("clamp low got %s", clamped)
	}
	clamped, err = MustParseMoney("USD 20.00").Clamp(low, high)
	if err != nil {
		t.Fatal(err)
	}
	if !clamped.Equal(high) {
		t.Fatalf("clamp high got %s", clamped)
	}
	clamped, err = mid.Clamp(low, high)
	if err != nil {
		t.Fatal(err)
	}
	if !clamped.Equal(mid) {
		t.Fatalf("clamp inside got %s", clamped)
	}
	if _, err := mid.Clamp(low, other); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected clamp currency mismatch, got %v", err)
	}

	min, err := MinMoney(high, mid, low)
	if err != nil {
		t.Fatal(err)
	}
	if !min.Equal(low) {
		t.Fatalf("MinMoney got %s", min)
	}
	max, err := MaxMoney(low, mid, high)
	if err != nil {
		t.Fatal(err)
	}
	if !max.Equal(high) {
		t.Fatalf("MaxMoney got %s", max)
	}
	if _, err := MinMoney(); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected empty MinMoney error, got %v", err)
	}
	if _, err := MaxMoney(Money{}); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid MaxMoney error, got %v", err)
	}
	if _, err := MinMoney(low, other); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected MinMoney currency mismatch, got %v", err)
	}
}

func TestMoneyAggregates(t *testing.T) {
	first := MustParseMoney("USD 1.20")
	second := MustParseMoney("USD 2.305")
	third := MustParseMoney("USD -0.005")

	sum, err := SumMoney(first, second, third)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "USD 3.500" {
		t.Fatalf("SumMoney got %s", sum)
	}

	avg, err := AvgMoney([]Money{first, second, third}, 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if avg.String() != "USD 1.17" {
		t.Fatalf("AvgMoney got %s", avg)
	}

	exact, err := AvgMoneyExact([]Money{MustParseMoney("USD 1.00"), MustParseMoney("USD 2.00")})
	if err != nil {
		t.Fatal(err)
	}
	if exact.String() != "USD 1.5" {
		t.Fatalf("AvgMoneyExact got %s", exact)
	}
	if _, err := AvgMoneyExact([]Money{
		MustParseMoney("USD 1.00"),
		MustParseMoney("USD 2.00"),
		MustParseMoney("USD 2.00"),
	}); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected AvgMoneyExact inexact, got %v", err)
	}

	if _, err := SumMoney(); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected SumMoney empty input, got %v", err)
	}
	if _, err := SumMoney(first, MustParseMoney("EUR 1.00")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected SumMoney currency mismatch, got %v", err)
	}
	if _, err := AvgMoney([]Money{first}, -1, ToNearestEven); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected AvgMoney invalid scale, got %v", err)
	}
	if _, err := AvgMoney([]Money{first}, 2, RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected AvgMoney invalid rounding, got %v", err)
	}
}

func TestMoneySQLAndNullMoney(t *testing.T) {
	money, _ := NewMoney(MustParse("123.4500"), "usd")
	value, err := money.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != "USD 123.4500" {
		t.Fatalf("money SQL value got %#v", value)
	}

	var scanned Money
	if err := scanned.Scan([]byte("USD 123.4500")); err != nil {
		t.Fatal(err)
	}
	if !scanned.Equal(money) {
		t.Fatalf("money scan got %s", scanned)
	}
	if err := scanned.Scan("BTC 1.23456789"); err != nil {
		t.Fatal(err)
	}
	if scanned.String() != "BTC 1.23456789" {
		t.Fatalf("money scan string got %s", scanned)
	}
	if err := scanned.Scan(nil); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected nil money scan error, got %v", err)
	}
	if err := scanned.Scan(42); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("expected invalid money scan source, got %v", err)
	}
	if _, err := (Money{}).Value(); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid money value error, got %v", err)
	}

	var nullable NullMoney
	if !nullable.IsZero() {
		t.Fatal("zero NullMoney should be zero/null")
	}
	if err := nullable.Scan(nil); err != nil || nullable.Valid {
		t.Fatalf("null money scan got %#v err=%v", nullable, err)
	}
	if err := nullable.Scan("USD 1.20"); err != nil || !nullable.Valid || nullable.Money.String() != "USD 1.20" {
		t.Fatalf("valid null money scan got %#v err=%v", nullable, err)
	}
	data, err := json.Marshal(nullable)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"amount":"1.20","currency":"USD"}` {
		t.Fatalf("null money JSON got %s", data)
	}
	if err := json.Unmarshal([]byte(`null`), &nullable); err != nil || nullable.Valid {
		t.Fatalf("null money JSON decode got %#v err=%v", nullable, err)
	}
	nullable = NewNullMoney(money)
	if nullable.IsZero() {
		t.Fatal("valid NullMoney should not be zero/null")
	}
	value, err = nullable.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != "USD 123.4500" {
		t.Fatalf("null money SQL value got %#v", value)
	}
	nullable.Valid = false
	value, err = nullable.Value()
	if err != nil || value != nil {
		t.Fatalf("invalid null money value got %#v err=%v", value, err)
	}
}

func TestMoneyBinaryAndGobRoundTrip(t *testing.T) {
	money := MustParseMoney("USD 123.4500")
	data, err := money.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != money.BinarySize() {
		t.Fatalf("money binary size got %d want %d", len(data), money.BinarySize())
	}
	appended, err := money.AppendBinary(make([]byte, 0, money.BinarySize()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(appended, data) {
		t.Fatalf("money append binary mismatch got %x want %x", appended, data)
	}
	var decoded Money
	if err := decoded.UnmarshalBinary(data); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(money) || decoded.Amount().Scale() != money.Amount().Scale() {
		t.Fatalf("money binary round trip got %s", decoded)
	}
	if err := decoded.UnmarshalBinary([]byte("bad")); err == nil {
		t.Fatal("expected money binary syntax error")
	}
	hugeAmountLength := append([]byte(nil), data...)
	amountLengthOffset := 4 + 1 + 2 + len(money.Currency())
	stdBinary.BigEndian.PutUint32(hugeAmountLength[amountLengthOffset:amountLengthOffset+4], math.MaxUint32)
	if err := decoded.UnmarshalBinary(hugeAmountLength); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected huge declared money amount length error, got %v", err)
	}
	overscale := append([]byte(nil), data...)
	decimalOffset := amountLengthOffset + 4
	stdBinary.BigEndian.PutUint32(overscale[decimalOffset+6:decimalOffset+10], uint32(DefaultMaxParseScale+1))
	if err := decoded.UnmarshalBinary(overscale); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected embedded decimal scale limit error, got %v", err)
	}
	trustedBinary := DefaultBinaryDecodeOptions()
	trustedBinary.MaxScale = 0
	if err := decoded.UnmarshalBinaryWithOptions(overscale, trustedBinary); err != nil {
		t.Fatalf("trusted money binary scale override should decode: %v", err)
	}
	if decoded.Amount().Scale() != DefaultMaxParseScale+1 {
		t.Fatalf("trusted money binary scale got %d", decoded.Amount().Scale())
	}
	if _, err := (Money{}).MarshalBinary(); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid money binary error, got %v", err)
	}

	reused := make([]byte, 0, money.BinarySize())
	allocs := testing.AllocsPerRun(1000, func() {
		var err error
		reused, err = money.AppendBinary(reused[:0])
		if err != nil {
			panic(err)
		}
		if len(reused) != money.BinarySize() {
			panic("bad money append binary size")
		}
	})
	if allocs != 0 {
		t.Fatalf("expected zero money append allocations, got %.2f", allocs)
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(money); err != nil {
		t.Fatal(err)
	}
	var gobDecoded Money
	if err := gob.NewDecoder(&buf).Decode(&gobDecoded); err != nil {
		t.Fatal(err)
	}
	if !gobDecoded.Equal(money) || gobDecoded.Amount().Scale() != money.Amount().Scale() {
		t.Fatalf("money gob round trip got %s", gobDecoded)
	}
}

func assertMoneyStrings(t *testing.T, got []Money, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d parts want %d", len(got), len(want))
	}
	for i := range got {
		if got[i].String() != want[i] {
			t.Fatalf("part %d got %s want %s", i, got[i], want[i])
		}
	}
}

func assertMoneyTotal(t *testing.T, parts []Money, want string) {
	t.Helper()
	if len(parts) == 0 {
		t.Fatal("empty parts")
	}
	total := parts[0]
	for _, part := range parts[1:] {
		var err error
		total, err = total.Add(part)
		if err != nil {
			t.Fatal(err)
		}
	}
	if total.String() != want {
		t.Fatalf("total got %s want %s", total, want)
	}
}
