package qdecimal

import (
	"encoding/json"
	"errors"
	"math/big"
	"testing"
)

func TestDecimalAdditionalPublicSurface(t *testing.T) {
	d := MustParse("-12.3400")

	if !MustParse("12.00").IsInteger() || MustParse("12.01").IsInteger() {
		t.Fatal("integer detection mismatch")
	}
	if got := d.Neg().String(); got != "12.3400" {
		t.Fatalf("neg got %s", got)
	}
	if got := d.Abs().String(); got != "12.3400" {
		t.Fatalf("abs got %s", got)
	}
	if got, err := MustParse("1.25").Round(1, ToNearestAway); err != nil || got.String() != "1.3" || got.Scale() != 1 {
		t.Fatalf("round alias got %s scale=%d err=%v", got, got.Scale(), err)
	}
	if got, err := MustParse("1.2").StringFixed(4, TowardZero); err != nil || got != "1.2000" {
		t.Fatalf("StringFixed got %q err=%v", got, err)
	}
	if got := d.JSONNumber(); got.String() != "-12.3400" {
		t.Fatalf("JSONNumber got %s", got)
	}

	text, err := d.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "-12.3400" {
		t.Fatalf("MarshalText got %s", text)
	}
	appended, err := d.AppendText([]byte("amount="))
	if err != nil {
		t.Fatal(err)
	}
	if string(appended) != "amount=-12.3400" {
		t.Fatalf("AppendText got %s", appended)
	}
	var fromText Decimal
	if err := fromText.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}
	if !fromText.Equal(d) || fromText.Scale() != d.Scale() {
		t.Fatalf("text round trip got %s scale=%d", fromText, fromText.Scale())
	}
	var nilDecimal *Decimal
	if err := nilDecimal.UnmarshalText([]byte("1")); err == nil {
		t.Fatal("expected nil Decimal text unmarshal error")
	}

	extended, err := d.MarshalExtendedJSON()
	if err != nil {
		t.Fatal(err)
	}
	if string(extended) != `{"$numberDecimal":"-12.3400"}` {
		t.Fatalf("MarshalExtendedJSON got %s", extended)
	}
}

func TestAppendTextPublicSurface(t *testing.T) {
	fixed := mustFixed64(t, -12345, 3)
	fixedText, err := fixed.AppendText([]byte("fixed="))
	if err != nil {
		t.Fatal(err)
	}
	if string(fixedText) != "fixed=-12.345" {
		t.Fatalf("fixed AppendText got %s", fixedText)
	}

	money := MustParseMoney("USD 12.3400")
	moneyText, err := money.AppendText([]byte("money="))
	if err != nil {
		t.Fatal(err)
	}
	if string(moneyText) != "money=USD 12.3400" {
		t.Fatalf("money AppendText got %s", moneyText)
	}
	if _, err := (Money{}).AppendText(nil); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid money AppendText error, got %v", err)
	}
}

func TestDecimalConstructorAndBoundarySurface(t *testing.T) {
	if _, err := New(1, -1); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid New scale, got %v", err)
	}

	nilBig, err := NewFromBigInt(nil, 4)
	if err != nil {
		t.Fatal(err)
	}
	if nilBig.String() != "0.0000" {
		t.Fatalf("nil big int got %s", nilBig)
	}
	if _, err := NewFromBigInt(big.NewInt(1), -1); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid big int scale, got %v", err)
	}
	coef := big.NewInt(12345)
	fromBig, err := NewFromBigInt(coef, 2)
	if err != nil {
		t.Fatal(err)
	}
	coef.SetInt64(1)
	if fromBig.String() != "123.45" {
		t.Fatalf("NewFromBigInt did not copy coefficient: %s", fromBig)
	}

	floatInput, _, err := big.ParseFloat("1.25", 10, 128, big.ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	rounded, err := FromBigFloat(floatInput, 1, ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.String() != "1.3" {
		t.Fatalf("FromBigFloat got %s", rounded)
	}
	if _, err := FromBigFloat(nil, 2, ToNearestEven); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected nil big float error, got %v", err)
	}
	if _, err := FromBigFloat(new(big.Float).SetInf(false), 2, ToNearestEven); !errors.Is(err, ErrNonFiniteFloat) {
		t.Fatalf("expected infinite big float error, got %v", err)
	}
	if _, err := FromBigFloat(floatInput, -1, ToNearestEven); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid big float scale, got %v", err)
	}
	if _, err := FromBigFloat(floatInput, 2, RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid big float rounding, got %v", err)
	}

	if _, err := FromRat(nil, 2, ToNearestEven); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected nil rat error, got %v", err)
	}
	if _, err := FromRat(big.NewRat(1, 3), -1, ToNearestEven); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid rat scale, got %v", err)
	}
	if _, err := FromRat(big.NewRat(1, 3), 2, RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid rat rounding, got %v", err)
	}
	fromRat, err := FromRat(big.NewRat(-1, 8), 2, TowardNegative)
	if err != nil {
		t.Fatal(err)
	}
	if fromRat.String() != "-0.13" {
		t.Fatalf("FromRat negative rounding got %s", fromRat)
	}

	if got := MustParse("1.23").Clamp(MustParse("2"), MustParse("3")); got.String() != "2" {
		t.Fatalf("clamp lower got %s", got)
	}
	if got := MustParse("4").Clamp(MustParse("2"), MustParse("3")); got.String() != "3" {
		t.Fatalf("clamp upper got %s", got)
	}
	if got := MustParse("2.50").Clamp(MustParse("3"), MustParse("2")); got.String() != "2.50" {
		t.Fatalf("clamp reversed bounds got %s", got)
	}
	if !MustParse("2").Between(MustParse("3"), MustParse("1"), true) {
		t.Fatal("between should accept reversed inclusive bounds")
	}
	if MustParse("1").Between(MustParse("3"), MustParse("1"), false) {
		t.Fatal("exclusive between should reject boundary")
	}
	if got := Min().String(); got != "0" {
		t.Fatalf("empty Min got %s", got)
	}
	if got := Max().String(); got != "0" {
		t.Fatalf("empty Max got %s", got)
	}
}

func TestContextAdditionalPublicSurface(t *testing.T) {
	ctx := MustContext(2, ToNearestEven)

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"scale":2,"rounding":"to_nearest_even"}` {
		t.Fatalf("context JSON got %s", data)
	}
	var decoded Context
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != ctx {
		t.Fatalf("context JSON round trip got %#v", decoded)
	}
	if err := json.Unmarshal([]byte(`{"scale":2,"rounding":"to_nearest_even","extra":true}`), &decoded); err == nil {
		t.Fatal("expected strict context JSON to reject unknown fields")
	}
	var nilContext *Context
	if err := nilContext.UnmarshalJSON(data); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected nil context error, got %v", err)
	}
	if _, err := json.Marshal(Context{Scale: -1, Rounding: ToNearestEven}); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid context marshal error, got %v", err)
	}

	sum, err := ctx.Add(MustParse("1.234"), MustParse("0.001"))
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "1.24" {
		t.Fatalf("context add got %s", sum)
	}
	diff, err := ctx.Sub(MustParse("1.234"), MustParse("0.004"))
	if err != nil {
		t.Fatal(err)
	}
	if diff.String() != "1.23" {
		t.Fatalf("context sub got %s", diff)
	}
	quot, err := ctx.Div(One, MustParse("8"))
	if err != nil {
		t.Fatal(err)
	}
	if quot.String() != "0.12" {
		t.Fatalf("context div got %s", quot)
	}
	if exact, err := ctx.SubExact(MustParse("1.230"), MustParse("0.230")); err != nil || exact.String() != "1.00" {
		t.Fatalf("context sub exact got %s err=%v", exact, err)
	}
	if exact, err := ctx.MulExact(MustParse("1.20"), MustParse("2.50")); err != nil || exact.String() != "3.00" {
		t.Fatalf("context mul exact got %s err=%v", exact, err)
	}
	if _, err := ctx.MulExact(MustParse("1.23"), MustParse("1.23")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact mul inexact, got %v", err)
	}
}

func TestContextInvalidPolicySurface(t *testing.T) {
	badScale := Context{Scale: -1, Rounding: ToNearestEven}
	if _, err := badScale.Quantize(One); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid quantize scale, got %v", err)
	}
	if _, err := badScale.QuantizeExact(One); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid exact quantize scale, got %v", err)
	}
	if _, err := badScale.QuantizeStep(One, MustParse("0.05")); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid quantize step scale, got %v", err)
	}
	if _, err := badScale.QuantizeStepExact(One, MustParse("0.05")); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid exact quantize step scale, got %v", err)
	}
	if _, err := badScale.Div(One, One); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid div scale, got %v", err)
	}
	if _, err := badScale.DivExact(One, One); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid exact div scale, got %v", err)
	}
	if _, err := badScale.StringFixed(One); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid string fixed scale, got %v", err)
	}

	badRounding := Context{Scale: 2, Rounding: RoundingMode(99)}
	if _, err := badRounding.Quantize(One); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid quantize rounding, got %v", err)
	}
	if _, err := badRounding.QuantizeStep(One, MustParse("0.05")); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid quantize step rounding, got %v", err)
	}
	if _, err := badRounding.QuantizeStepExact(One, MustParse("0.05")); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid exact quantize step rounding, got %v", err)
	}
	if _, err := badRounding.Div(One, One); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid div rounding, got %v", err)
	}
	ctx := MustContext(2, ToNearestEven)
	if _, err := ctx.Div(One, Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected context div zero error, got %v", err)
	}
	if _, err := ctx.DivExact(One, Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected context exact div zero error, got %v", err)
	}
}

func TestFixed64AdditionalPublicSurface(t *testing.T) {
	zero := mustFixed64(t, 0, 2)
	if !zero.IsZero() || zero.Sign() != 0 {
		t.Fatalf("zero fixed64 got zero=%v sign=%d", zero.IsZero(), zero.Sign())
	}
	negative := mustFixed64(t, -129, 2)
	if negative.Sign() != -1 || mustFixed64(t, 129, 2).Sign() != 1 {
		t.Fatal("fixed64 sign mismatch")
	}
	abs, err := negative.Abs()
	if err != nil {
		t.Fatal(err)
	}
	if abs.String() != "1.29" {
		t.Fatalf("fixed64 abs got %s", abs)
	}
	truncated, err := negative.Truncate(1)
	if err != nil {
		t.Fatal(err)
	}
	if truncated.String() != "-1.2" {
		t.Fatalf("fixed64 truncate got %s", truncated)
	}
	text, err := negative.AppendText([]byte("pnl="))
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != "pnl=-1.29" {
		t.Fatalf("fixed64 AppendText got %s", text)
	}
	var nilFixed *Fixed64
	if err := nilFixed.UnmarshalText([]byte("1.23")); err == nil {
		t.Fatal("expected nil Fixed64 text unmarshal error")
	}
}

func TestMoneyAdditionalPublicSurface(t *testing.T) {
	money, err := NewMoneyFromMinorUnits(12345, 2, "idr")
	if err != nil {
		t.Fatal(err)
	}
	if money.String() != "IDR 123.45" {
		t.Fatalf("money from minor units got %s", money)
	}

	zero, _ := NewMoney(Zero, "USD")
	if !zero.IsZero() || zero.Sign() != 0 {
		t.Fatalf("zero money got zero=%v sign=%d", zero.IsZero(), zero.Sign())
	}
	loss := MustParseMoney("USD -12.34")
	if loss.Sign() != -1 {
		t.Fatal("money sign mismatch")
	}
	if loss.Neg().String() != "USD 12.34" {
		t.Fatalf("money neg got %s", loss.Neg())
	}
	if loss.Abs().String() != "USD 12.34" {
		t.Fatalf("money abs got %s", loss.Abs())
	}
	diff, err := MustParseMoney("USD 5.00").Sub(MustParseMoney("USD 2.25"))
	if err != nil {
		t.Fatal(err)
	}
	if diff.String() != "USD 2.75" {
		t.Fatalf("money sub got %s", diff)
	}
	units, err := money.MinorUnitsExact(2)
	if err != nil {
		t.Fatal(err)
	}
	if units.String() != "12345" {
		t.Fatalf("money exact minor units got %s", units)
	}
	if _, err := MustParseMoney("IDR 123.456").MinorUnitsExact(2); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact money minor units inexact, got %v", err)
	}
	if got := (Money{}).Key(); got != "" {
		t.Fatalf("invalid money key got %q", got)
	}
}

func TestMoneyContextAdditionalPublicSurface(t *testing.T) {
	usd := MustMoneyContext("USD", 2, ToNearestEven)
	data, err := json.Marshal(usd)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"currency":"USD","scale":2,"rounding":"to_nearest_even"}` {
		t.Fatalf("money context JSON got %s", data)
	}
	var decoded MoneyContext
	if err := json.Unmarshal([]byte(`{"currency":"usd","scale":2,"rounding":"to_nearest_even"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != usd {
		t.Fatalf("money context round trip got %#v", decoded)
	}
	if err := json.Unmarshal([]byte(`{"currency":"USD","scale":2,"rounding":"to_nearest_even","extra":1}`), &decoded); err == nil {
		t.Fatal("expected strict money context JSON to reject unknown fields")
	}
	var nilContext *MoneyContext
	if err := nilContext.UnmarshalJSON(data); !errors.Is(err, ErrNilValue) {
		t.Fatalf("expected nil money context error, got %v", err)
	}

	exact, err := usd.QuantizeExact(MustParseMoney("USD 1.230"))
	if err != nil {
		t.Fatal(err)
	}
	if exact.String() != "USD 1.23" {
		t.Fatalf("money context quantize exact got %s", exact)
	}
	if _, err := usd.QuantizeExact(MustParseMoney("USD 1.231")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected quantize exact inexact, got %v", err)
	}
	if exact, err := usd.SubExact(MustParseMoney("USD 5.000"), MustParseMoney("USD 2.500")); err != nil || exact.String() != "USD 2.50" {
		t.Fatalf("money context sub exact got %s err=%v", exact, err)
	}
	if _, err := usd.SubExact(MustParseMoney("USD 5.000"), MustParseMoney("USD 2.555")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected sub exact inexact, got %v", err)
	}
	if exact, err := usd.MulExact(MustParseMoney("USD 1.20"), MustParse("2.50")); err != nil || exact.String() != "USD 3.00" {
		t.Fatalf("money context mul exact got %s err=%v", exact, err)
	}
	if _, err := usd.MulExact(MustParseMoney("USD 1.23"), MustParse("1.23")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected mul exact inexact, got %v", err)
	}
}

func TestMinorScaleAndMoneyContextBoundarySurface(t *testing.T) {
	if got := MustMinorScale(18); got != 18 {
		t.Fatalf("MustMinorScale got %d", got)
	}
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected MustMinorScale panic")
			}
		}()
		_ = MustMinorScale(19)
	}()
	if err := ValidateMinorScale(-1); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected negative minor scale error, got %v", err)
	}

	usd := MustMoneyContext("USD", 2, ToNearestAway)
	if _, err := json.Marshal(MoneyContext{Currency: "usd", Scale: 2, Rounding: ToNearestEven}); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid money context marshal currency, got %v", err)
	}
	if _, err := json.Marshal(MoneyContext{Currency: "USD", Scale: -1, Rounding: ToNearestEven}); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid money context marshal scale, got %v", err)
	}
	if _, err := json.Marshal(MoneyContext{Currency: "USD", Scale: 2, Rounding: RoundingMode(99)}); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid money context marshal rounding, got %v", err)
	}
	if _, err := usd.WithScale(-1); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid WithScale error, got %v", err)
	}
	if _, err := usd.WithRounding(RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid WithRounding error, got %v", err)
	}
	if _, err := (MoneyContext{Currency: "USD", Scale: 2, Rounding: RoundingMode(99)}).Money(One); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid money context Money error, got %v", err)
	}
	if _, err := usd.Div(MustParseMoney("USD 1.00"), Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected money context div zero error, got %v", err)
	}
	if _, err := usd.DivExact(MustParseMoney("USD 1.00"), Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected money context exact div zero error, got %v", err)
	}
	if _, err := usd.QuantizeStep(MustParseMoney("USD 1.00"), Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected money context zero step error, got %v", err)
	}
	if _, err := usd.QuantizeStepExact(MustParseMoney("USD 1.00"), Zero); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("expected money context exact zero step error, got %v", err)
	}
	if _, err := usd.QuantizeStepExact(MustParseMoney("EUR 1.00"), MustParse("0.05")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected money context exact step currency mismatch, got %v", err)
	}
	stepped, err := usd.QuantizeStep(MustParseMoney("USD 1.02"), MustParse("-0.05"))
	if err != nil {
		t.Fatal(err)
	}
	if stepped.String() != "USD 1.00" {
		t.Fatalf("negative tick step got %s", stepped)
	}
}

func TestNullWrappersAdditionalSurface(t *testing.T) {
	var nullable NullDecimal
	if err := json.Unmarshal([]byte(`"42.00"`), &nullable); err != nil {
		t.Fatal(err)
	}
	if !nullable.Valid || nullable.Decimal.String() != "42.00" {
		t.Fatalf("NullDecimal JSON got %#v", nullable)
	}
	value, err := nullable.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != "42.00" {
		t.Fatalf("NullDecimal value got %#v", value)
	}
	if err := json.Unmarshal([]byte(`"not-decimal"`), &nullable); err == nil || nullable.Valid {
		t.Fatalf("expected invalid NullDecimal reset, got %#v err=%v", nullable, err)
	}
	var nilDecimal *NullDecimal
	if err := nilDecimal.Scan(nil); err == nil {
		t.Fatal("expected nil NullDecimal scan error")
	}
	if err := nilDecimal.UnmarshalJSON([]byte(`null`)); err == nil {
		t.Fatal("expected nil NullDecimal JSON error")
	}

	var fixed NullFixed64
	if err := json.Unmarshal([]byte(`"1.230"`), &fixed); err != nil {
		t.Fatal(err)
	}
	if !fixed.Valid || fixed.Fixed64.String() != "1.230" {
		t.Fatalf("NullFixed64 JSON got %#v", fixed)
	}
	if err := json.Unmarshal([]byte(`{}`), &fixed); err == nil || fixed.Valid {
		t.Fatalf("expected invalid NullFixed64 reset, got %#v err=%v", fixed, err)
	}
	var nilFixed *NullFixed64
	if err := nilFixed.Scan(nil); err == nil {
		t.Fatal("expected nil NullFixed64 scan error")
	}
	if err := nilFixed.UnmarshalJSON([]byte(`null`)); err == nil {
		t.Fatal("expected nil NullFixed64 JSON error")
	}

	var money NullMoney
	if err := json.Unmarshal([]byte(`{"amount":"1.23","currency":"usd"}`), &money); err != nil {
		t.Fatal(err)
	}
	if !money.Valid || money.Money.String() != "USD 1.23" {
		t.Fatalf("NullMoney JSON got %#v", money)
	}
	if err := json.Unmarshal([]byte(`{"amount":"1.23","currency":"US-D"}`), &money); err == nil || money.Valid {
		t.Fatalf("expected invalid NullMoney reset, got %#v err=%v", money, err)
	}
	var nilMoney *NullMoney
	if err := nilMoney.Scan(nil); err == nil {
		t.Fatal("expected nil NullMoney scan error")
	}
	if err := nilMoney.UnmarshalJSON([]byte(`null`)); err == nil {
		t.Fatal("expected nil NullMoney JSON error")
	}
}
