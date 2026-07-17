package qdecimal

import (
	"errors"
	"fmt"
	"testing"
)

var _ fmt.Stringer = MoneyContext{}

func TestMoneyContextConstructionAndParsing(t *testing.T) {
	usd, err := NewMoneyContext(" usd ", 2, ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if usd.Currency != "USD" || usd.Scale != 2 || usd.Rounding != ToNearestEven {
		t.Fatalf("context got %#v", usd)
	}
	if usd.String() != "USD scale=2 rounding=to_nearest_even" {
		t.Fatalf("context string got %s", usd)
	}

	amount, err := usd.Parse("123.456")
	if err != nil {
		t.Fatal(err)
	}
	if amount.String() != "USD 123.46" {
		t.Fatalf("parse rounded got %s", amount)
	}
	flexible, err := usd.ParseFlexible(" 1,234.565 ")
	if err != nil {
		t.Fatal(err)
	}
	if flexible.String() != "USD 1234.56" {
		t.Fatalf("flexible parse got %s", flexible)
	}

	fromUnits, err := usd.FromMinorUnits(12345)
	if err != nil {
		t.Fatal(err)
	}
	if fromUnits.String() != "USD 123.45" {
		t.Fatalf("minor unit money got %s", fromUnits)
	}

	btc := MustMoneyContext("BTC", 8, TowardZero)
	sats, err := btc.FromMinorUnits(123456789)
	if err != nil {
		t.Fatal(err)
	}
	if sats.String() != "BTC 1.23456789" {
		t.Fatalf("btc minor units got %s", sats)
	}

	if _, err := NewMoneyContext("US-D", 2, ToNearestEven); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid currency, got %v", err)
	}
	if _, err := NewMoneyContext("USD", -1, ToNearestEven); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid scale, got %v", err)
	}
	if _, err := NewMoneyContext("USD", 2, RoundingMode(99)); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid rounding, got %v", err)
	}
}

func TestMoneyContextOperations(t *testing.T) {
	usd := MustMoneyContext("USD", 2, ToNearestEven)
	a, _ := NewMoney(MustParse("10.005"), "USD")
	b, _ := NewMoney(MustParse("0.005"), "USD")

	sum, err := usd.Add(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "USD 10.01" {
		t.Fatalf("context sum got %s", sum)
	}

	diff, err := usd.Sub(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if diff.String() != "USD 10.00" {
		t.Fatalf("context sub got %s", diff)
	}

	fee, err := usd.Mul(a, MustParse("0.0025"))
	if err != nil {
		t.Fatal(err)
	}
	if fee.String() != "USD 0.03" {
		t.Fatalf("context mul got %s", fee)
	}

	half, err := usd.Div(a, MustParse("2"))
	if err != nil {
		t.Fatal(err)
	}
	if half.String() != "USD 5.00" {
		t.Fatalf("context div got %s", half)
	}

	ticked, err := usd.QuantizeStep(a, MustParse("0.05"))
	if err != nil {
		t.Fatal(err)
	}
	if ticked.String() != "USD 10.00" {
		t.Fatalf("context tick got %s", ticked)
	}
	exactTick, err := usd.QuantizeStepExact(ticked, MustParse("-0.05"))
	if err != nil {
		t.Fatal(err)
	}
	if exactTick.String() != "USD 10.00" {
		t.Fatalf("context exact tick got %s", exactTick)
	}
	if _, err := usd.QuantizeStepExact(MustParseMoney("USD 10.02"), MustParse("0.05")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact tick multiple error, got %v", err)
	}
	if _, err := usd.QuantizeStepExact(MustParseMoney("USD 10.025"), MustParse("0.025")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact tick scale error, got %v", err)
	}

	units, err := usd.Int64MinorUnits(a)
	if err != nil {
		t.Fatal(err)
	}
	if units != 1000 {
		t.Fatalf("context minor units got %d", units)
	}
	exactUnits, err := usd.Int64MinorUnitsExact(MustParseMoney("USD 10.000"))
	if err != nil {
		t.Fatal(err)
	}
	if exactUnits != 1000 {
		t.Fatalf("context exact minor units got %d", exactUnits)
	}
	if _, err := usd.Int64MinorUnitsExact(MustParseMoney("USD 10.005")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact minor units inexact, got %v", err)
	}

	exact, err := usd.MoneyExact(MustParse("10.000"))
	if err != nil {
		t.Fatal(err)
	}
	if exact.String() != "USD 10.00" || exact.Amount().Scale() != 2 {
		t.Fatalf("money exact got %s scale=%d", exact, exact.Amount().Scale())
	}
	if _, err := usd.MoneyExact(MustParse("10.005")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected money exact inexact, got %v", err)
	}
	exactSum, err := usd.AddExact(MustParseMoney("USD 1.20"), MustParseMoney("USD 0.030"))
	if err != nil {
		t.Fatal(err)
	}
	if exactSum.String() != "USD 1.23" {
		t.Fatalf("exact sum got %s", exactSum)
	}
	if _, err := usd.AddExact(MustParseMoney("USD 1.20"), MustParseMoney("USD 0.031")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact add inexact, got %v", err)
	}
	if exactDiv, err := usd.DivExact(MustParseMoney("USD 1.00"), MustParse("4")); err != nil || exactDiv.String() != "USD 0.25" {
		t.Fatalf("exact div got %s err=%v", exactDiv, err)
	}
	if _, err := usd.DivExact(MustParseMoney("USD 1.00"), MustParse("8")); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected exact div inexact, got %v", err)
	}
}

func TestMoneyContextAggregatesAndRanges(t *testing.T) {
	usd := MustMoneyContext("USD", 2, ToNearestEven)
	a := MustParseMoney("USD 1.205")
	b := MustParseMoney("USD 2.305")
	c := MustParseMoney("USD 0.005")

	sum, err := usd.Sum(a, b, c)
	if err != nil {
		t.Fatal(err)
	}
	if sum.String() != "USD 3.52" {
		t.Fatalf("context sum got %s", sum)
	}
	exactSum, err := usd.SumExact(MustParseMoney("USD 1.20"), MustParseMoney("USD 0.030"))
	if err != nil {
		t.Fatal(err)
	}
	if exactSum.String() != "USD 1.23" {
		t.Fatalf("context exact sum got %s", exactSum)
	}
	if _, err := usd.SumExact(a); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact sum inexact, got %v", err)
	}

	avg, err := usd.Avg(
		MustParseMoney("USD 1.00"),
		MustParseMoney("USD 2.00"),
		MustParseMoney("USD 2.00"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if avg.String() != "USD 1.67" {
		t.Fatalf("context avg got %s", avg)
	}
	exactAvg, err := usd.AvgExact(MustParseMoney("USD 1.00"), MustParseMoney("USD 2.00"))
	if err != nil {
		t.Fatal(err)
	}
	if exactAvg.String() != "USD 1.50" {
		t.Fatalf("context exact avg got %s", exactAvg)
	}
	if _, err := usd.AvgExact(
		MustParseMoney("USD 1.00"),
		MustParseMoney("USD 2.00"),
		MustParseMoney("USD 2.00"),
	); !errors.Is(err, ErrInexact) {
		t.Fatalf("expected context exact avg inexact, got %v", err)
	}

	inRange, err := usd.Between(MustParseMoney("USD 10.005"), MustParseMoney("USD 10.00"), MustParseMoney("USD 10.01"), true)
	if err != nil {
		t.Fatal(err)
	}
	if !inRange {
		t.Fatal("expected context between to use quantized value")
	}
	inRange, err = usd.Between(MustParseMoney("USD 10.005"), MustParseMoney("USD 10.00"), MustParseMoney("USD 10.01"), false)
	if err != nil {
		t.Fatal(err)
	}
	if inRange {
		t.Fatal("expected exclusive context range to reject quantized lower bound")
	}

	clamped, err := usd.Clamp(MustParseMoney("USD 15.004"), MustParseMoney("USD 9.99"), MustParseMoney("USD 10.01"))
	if err != nil {
		t.Fatal(err)
	}
	if clamped.String() != "USD 10.01" {
		t.Fatalf("context clamp got %s", clamped)
	}
	min, err := usd.Min(MustParseMoney("USD 1.015"), MustParseMoney("USD 0.999"), MustParseMoney("USD 1.004"))
	if err != nil {
		t.Fatal(err)
	}
	if min.String() != "USD 1.00" {
		t.Fatalf("context min got %s", min)
	}
	max, err := usd.Max(MustParseMoney("USD 1.015"), MustParseMoney("USD 0.999"), MustParseMoney("USD 1.004"))
	if err != nil {
		t.Fatal(err)
	}
	if max.String() != "USD 1.02" {
		t.Fatalf("context max got %s", max)
	}

	if _, err := usd.Sum(); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected context sum empty input, got %v", err)
	}
	if _, err := usd.Min(); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected context min empty input, got %v", err)
	}
	if _, err := usd.Avg(MustParseMoney("EUR 1.00")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected context avg currency mismatch, got %v", err)
	}
	if _, err := usd.Clamp(MustParseMoney("EUR 1.00"), MustParseMoney("USD 0.00"), MustParseMoney("USD 2.00")); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected context clamp currency mismatch, got %v", err)
	}
}

func TestMoneyContextCurrencyAndValidation(t *testing.T) {
	usd := MustMoneyContext("USD", 2, ToNearestEven)
	eur, _ := NewMoney(MustParse("1.00"), "EUR")
	if _, err := usd.Quantize(eur); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected currency mismatch, got %v", err)
	}
	if _, err := usd.Int64MinorUnits(Money{}); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid zero-value money, got %v", err)
	}

	badCurrency := MoneyContext{Currency: "usd", Scale: 2, Rounding: ToNearestEven}
	if _, err := badCurrency.Parse("1.00"); !errors.Is(err, ErrInvalidCurrency) {
		t.Fatalf("expected invalid manual context currency, got %v", err)
	}
	badScale := MoneyContext{Currency: "USD", Scale: -1, Rounding: ToNearestEven}
	if _, err := badScale.Parse("1.00"); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid manual context scale, got %v", err)
	}
	badRounding := MoneyContext{Currency: "USD", Scale: 2, Rounding: RoundingMode(99)}
	if _, err := badRounding.Parse("1.00"); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid manual context rounding, got %v", err)
	}

	wider, err := usd.WithScale(4)
	if err != nil {
		t.Fatal(err)
	}
	if wider.String() != "USD scale=4 rounding=to_nearest_even" {
		t.Fatalf("wider context got %s", wider)
	}
	truncate, err := wider.WithRounding(TowardZero)
	if err != nil {
		t.Fatal(err)
	}
	if truncate.String() != "USD scale=4 rounding=toward_zero" {
		t.Fatalf("rounding context got %s", truncate)
	}
	if dc := truncate.DecimalContext(); dc.Scale != 4 || dc.Rounding != TowardZero {
		t.Fatalf("decimal context got %#v", dc)
	}
}

func TestMoneyContextAllocation(t *testing.T) {
	usd := MustMoneyContext("USD", 2, ToNearestEven)
	amount, _ := usd.Parse("10")

	parts, err := usd.Allocate(amount, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertMoneyStrings(t, parts, []string{"USD 3.34", "USD 3.33", "USD 3.33"})
	assertMoneyTotal(t, parts, "USD 10.00")

	ratioParts, err := usd.AllocateRatios(amount, []int64{3, 2, 1})
	if err != nil {
		t.Fatal(err)
	}
	assertMoneyStrings(t, ratioParts, []string{"USD 5.00", "USD 3.33", "USD 1.67"})
	assertMoneyTotal(t, ratioParts, "USD 10.00")

	eur, _ := NewMoney(MustParse("10.00"), "EUR")
	if _, err := usd.Allocate(eur, 2); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("expected allocation currency mismatch, got %v", err)
	}
}
