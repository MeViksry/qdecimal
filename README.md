# qdecimal

**Exact base-10 decimal arithmetic for finance, banking, exchanges, ledgers, and trading systems.**

`qdecimal` is designed around one rule: money arithmetic must be explicit, deterministic, and boring in production. It does not use package-global division precision, it does not silently accept NaN or infinity, and every operation that can discard digits requires a caller-provided scale and rounding mode.

## Install

```bash
go get github.com/MeViksry/qdecimal
```

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	"github.com/MeViksry/qdecimal"
)

func main() {
	price := qdecimal.MustParse("123.4500")
	size := qdecimal.MustParse("0.25")

	notional := price.Mul(size)
	rounded, err := notional.Round(2, qdecimal.ToNearestEven)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(rounded) // 30.86
}
```

## Design Principles

- **Exact decimal model:** values are stored as `coefficient * 10^-scale`.
- **No global mutable precision:** division and rescaling require explicit scale and rounding mode.
- **No hidden float behavior:** `FromFloat64` rejects NaN and infinity and is documented as an integration boundary, not a finance input path.
- **Scale preserving:** `0.00`, `1.20`, and `123.4500` can be represented and serialized exactly.
- **No negative zero:** `-0.00` becomes `0.00` while preserving scale.
- **Immutable API:** operations never mutate their receiver.
- **Currency-safe money:** `Money` rejects arithmetic across different currency or asset codes.
- **Fixed64 hot path:** bounded-scale int64 decimal for tight ledger and trading loops.
- **Safe JSON default:** JSON marshaling emits strings, not lossy floating-point numbers.
- **SQL ready:** `Decimal` and `NullDecimal` implement scanner and valuer interfaces.
- **Formatter ready:** `Decimal`, `Fixed64`, and `Money` implement
  `fmt.Formatter` with width, sign, quote, and fixed-precision handling.
- **Portable binary format:** versioned binary marshaling for file and wire storage.

## Rounding

```go
amount := qdecimal.MustParse("-1.25")
rounded, err := amount.Round(1, qdecimal.ToNearestAway)
```

Supported modes:

| Mode | Meaning |
|---|---|
| `ToNearestEven` | banker's rounding |
| `ToNearestAway` | half-up / ties away from zero |
| `ToNearestTowardZero` | half-down / ties toward zero |
| `AwayFromZero` | round up by magnitude |
| `TowardZero` | truncate |
| `TowardPositive` | ceiling |
| `TowardNegative` | floor |

Aliases are provided for common finance names: `RoundBankers`, `RoundHalfUp`, `RoundHalfDown`, `RoundUp`, `RoundDown`, `RoundCeil`, and `RoundFloor`.

Rounding modes also marshal as stable audit-friendly strings such as
`"to_nearest_even"` and can be parsed from common aliases:

```go
mode, err := qdecimal.ParseRoundingMode("half-up")
```

## Finance Context

Use `Context` to carry scale and rounding policy explicitly through services:

```go
usd := qdecimal.MustContext(2, qdecimal.ToNearestEven)

fee, err := usd.Mul(
	qdecimal.MustParse("123.4567"),
	qdecimal.MustParse("0.0025"),
)
```

There is no package-global precision variable. Every lossy boundary is local and auditable.

`Context` and `MoneyContext` marshal as strict lower-case JSON policy objects:

```json
{"currency":"USD","scale":2,"rounding":"to_nearest_even"}
```

Unknown fields are rejected on decode, currency codes are normalized, and
rounding aliases such as `"half-up"` are accepted.

## Minor Units

Ledger systems often store integer minor units:

```go
amount, err := qdecimal.NewFromMinorUnits(12345, 2) // 123.45
cents, err := amount.Int64MinorUnits(2, qdecimal.ToNearestEven)
exactCents, err := amount.Int64MinorUnitsExact(2)
err = qdecimal.ValidateMinorScale(2)
```

Overflow is reported as `ErrOverflow`.

## Fixed64 Hot Path

Use `Fixed64` when a domain has a known scale and the value must fit in signed
64-bit minor units, such as cents, ticks, lots, basis points, or bounded ledger
buckets:

```go
price, err := qdecimal.ParseFixed64("123.456", 2, qdecimal.ToNearestAway) // 123.46
fee, err := qdecimal.NewFixed64(25, 4)                                    // 0.0025
notional, err := price.Mul(fee, 6, qdecimal.ToNearestEven)
```

`Fixed64` supports text, JSON, SQL scanner/valuer, scale alignment, checked
addition/subtraction, rounding, multiplication, division, exchange tick/lot
quantization, range checks, clamping, min/max helpers, and exact conversion to
`Decimal`. `AppendText` writes fixed-scale text into caller-owned buffers for
hot logging and message paths. All integer overflow is reported as `ErrOverflow`.

## Currency-Safe Money

Use `Money` when the decimal amount must carry a currency or exchange asset code:

```go
usd, err := qdecimal.NewMoney(qdecimal.MustParse("10.00"), "usd")
usd, err = qdecimal.ParseMoney("USD 10.00")
fee, err := usd.Mul(qdecimal.MustParse("0.0025"), 4, qdecimal.ToNearestEven)
parts, err := usd.Allocate(3, 2, qdecimal.ToNearestEven)
```

Use `MoneyContext` to carry a reusable policy for one currency or asset:

```go
usd := qdecimal.MustMoneyContext("USD", 2, qdecimal.ToNearestEven)
btc := qdecimal.MustMoneyContext("BTC", 8, qdecimal.TowardZero)

amount, err := usd.Parse("123.456")
sats, err := btc.FromMinorUnits(123456789)
adjustment, err := usd.Parse("0.01")
total, err := usd.Add(amount, adjustment)
batchTotal, err := usd.Sum(amount, adjustment)
```

`Money` normalizes codes such as `usd` to `USD`, rejects malformed codes, and
returns `ErrCurrencyMismatch` if callers try to add or compare different
currencies. `SumMoney`, `AvgMoney`, `Money.Between`, `Money.Clamp`,
`MinMoney`, and `MaxMoney` keep aggregates, range checks, and limits
currency-safe. `MoneyContext` adds policy-aware `Sum`, `Avg`, `Between`,
`Clamp`, `Min`, `Max`, and tick-size helpers that return values at the
configured scale. `Money.QuantizeStepExact` and
`MoneyContext.QuantizeStepExact` reject invalid exchange increments with
`ErrInexact`. Allocation preserves the rounded total exactly at the requested
minor-unit scale.

`Money` also implements SQL scanner/valuer using the canonical text format
`CODE amount`, such as `USD 123.45`. `NullMoney` handles SQL `NULL` and JSON
`null` for nullable money fields and also supports text/formatter integration.

`Money.Key()` and `Fixed64.Key()` provide canonical comparable keys for maps.
`fmt` precision works for reporting, for example `fmt.Sprintf("%.2f", amount)`.
`Fixed64.QuantizeStep`, `SumFixed64`, `AvgFixed64`, and `AvgFixed64Exact`
provide checked tick-size and aggregate helpers for bounded-scale hot paths,
falling back to `Decimal` when alignment or overflow requires arbitrary
precision.

`qdecimal` intentionally does not ship a hard-coded ISO-4217 minor-unit table.
Banking and exchange systems should keep currency metadata in their own audited
configuration and pass scale/rounding explicitly at lossy boundaries.

## Exact-Only Boundaries and Aggregates

Use `DivExact` when rounding is not allowed:

```go
exact, err := qdecimal.One.DivExact(qdecimal.MustParse("8")) // 0.125
_, err = qdecimal.One.DivExact(qdecimal.MustParse("3"))      // ErrInexact
```

Use `RescaleExact`, `QuantizeExact`, or context `*Exact` methods when a value
must already fit a ledger scale or exchange step:

```go
amount, err := qdecimal.MustParse("123.4500").RescaleExact(2) // 123.45
_, err = qdecimal.MustParse("123.451").RescaleExact(2)        // ErrInexact
exact, err = qdecimal.MustContext(2, qdecimal.ToNearestEven).
	AddExact(qdecimal.MustParse("1.20"), qdecimal.MustParse("0.030"))
exactTick, err := qdecimal.MustContext(2, qdecimal.ToNearestEven).
	QuantizeStepExact(qdecimal.MustParse("1.20"), qdecimal.MustParse("0.05"))
moneyTick, err := qdecimal.MustMoneyContext("USD", 2, qdecimal.ToNearestEven).
	QuantizeStepExact(qdecimal.MustParseMoney("USD 1.20"), qdecimal.MustParse("0.05"))
```

Aggregates are explicit too:

```go
total := qdecimal.Sum(amounts...)
avg, err := qdecimal.Avg(amounts, 2, qdecimal.ToNearestEven)
exactAvg, err := qdecimal.AvgExact(amounts)
ledgerTotal, err := qdecimal.MustContext(2, qdecimal.ToNearestEven).SumExact(amounts...)
ledgerAvg, err := qdecimal.MustContext(2, qdecimal.ToNearestEven).AvgExact(amounts...)
moneyTotal, err := qdecimal.SumMoney(orders...)
moneyAvg, err := qdecimal.AvgMoney(orders, 2, qdecimal.ToNearestEven)
fixedTotal, err := qdecimal.SumFixed64(fills...)
fixedAvg, err := qdecimal.AvgFixed64(fills, 8, qdecimal.ToNearestEven)
```

## Exchange Tick Sizes

Use `QuantizeStep` for instruments whose valid increments are not just decimal
places:

```go
price := qdecimal.MustParse("1.23")
tick := qdecimal.MustParse("0.05")

rounded, err := price.QuantizeStep(tick, qdecimal.ToNearestAway) // 1.25
policyRounded, err := qdecimal.MustContext(2, qdecimal.ToNearestEven).
	QuantizeStep(qdecimal.MustParse("1.234"), qdecimal.MustParse("0.005"))
```

## Parsing

Strict parser:

```go
d, err := qdecimal.Parse("123456.7890")
d, err = qdecimal.NewFromString("123456.7890") // compatibility alias
```

Flexible parser:

```go
d, err := qdecimal.ParseFlexible(" 1,234,567.89 ")
```

Custom separators:

```go
opts := qdecimal.DefaultParseOptions
opts.DecimalSeparator = ','
opts.ThousandsSeparator = '.'
opts.AllowThousands = true

d, err := qdecimal.ParseWithOptions("1.234,50", opts)
```

`Parse` accepts the Unicode minus sign (`−`) by default and rejects NaN,
infinity, malformed thousands groups, and ambiguous syntax. Default parsing also
enforces `DefaultMaxParseDigits`, `DefaultMaxParseScale` (4096 each), and
`DefaultMaxParseExponentDigits` (10) so hostile exponents or scale-heavy
payloads cannot force unbounded coefficient expansion. Set
`ParseOptions.MaxDigits`, `ParseOptions.MaxScale`, or
`ParseOptions.MaxExponentDigits` to a larger value, or `0` to disable that
specific limit, only at trusted boundaries.

`NewFromFloat` is available as a compatibility alias for `FromFloat64`, but it
returns an error for NaN/infinity and should stay at integration boundaries.
Use `NewFromFloatWithScale` only with an explicit scale and rounding mode.

## JSON and SQL

`Decimal` marshals as a JSON string to avoid precision loss:

```json
"123.4500"
```

Unmarshal accepts both strings and numeric JSON tokens. When a system explicitly requires numeric JSON output and preserves arbitrary precision end to end, use:

```go
data, err := amount.MarshalJSONWithMode(qdecimal.EmitJSONNumber)
data, err = json.Marshal(qdecimal.AsNumber(amount))
```

SQL values are written as canonical decimal text. `NullDecimal`, `NullFixed64`,
and `NullMoney` handle SQL `NULL`, JSON `null`, text `"null"`, and formatter
output for nullable integration boundaries.

Scanners accept canonical text/bytes, exact integer source types, and
`json.Number`. Floating-point scanner sources are rejected by default so
precision loss stays explicit at integration boundaries.

For optional JSON fields, use pointer fields with `omitempty` when omission is
required:

```go
type Quote struct {
	Price *qdecimal.Decimal `json:"price,omitempty"`
}
```

Use `NullDecimal`, `NullFixed64`, and `NullMoney` when the wire format should
contain explicit `null`. Non-pointer struct values that implement
`json.Marshaler` are not omitted by Go's `omitempty` behavior; qdecimal
provides `IsZero` methods for nullable wrappers and tooling that honors
zero-value semantics.

## Binary and Exact Interop

`Decimal`, `Fixed64`, and `Money` implement binary marshal/unmarshal with
versioned, network-order formats. The same stable binary representation is used
for `GobEncode`/`GobDecode`, which makes cache snapshots and Go-native message
payloads deterministic.

```go
binary, err := amount.MarshalBinary()
rat := amount.Rat()
```

For high-throughput services that reuse buffers, `BinarySize` and `AppendBinary`
avoid per-message allocations:

```go
buf := make([]byte, 0, amount.BinarySize())
buf, err = amount.AppendBinary(buf)
```

`Decimal`, `Fixed64`, and `Money` also expose `AppendText` for canonical text
serialization into reusable buffers.

`fmt` formatting supports `%s`, `%v`, `%f`, `%F`, and `%q`. Fixed precision such
as `%.2f` uses banker's rounding and is capped by `DefaultMaxFormatScale` for
untrusted format strings.

Binary and gob decoders use `DefaultBinaryDecodeOptions()`, which bounds
coefficient bytes and scale for untrusted payloads. Trusted file/cache readers
can call `UnmarshalBinaryWithOptions` and raise or disable a specific limit.
Length fields are validated before slice conversion so malformed payloads behave
consistently across supported CPU architectures.

`FromRat` and `FromBigFloat` require explicit scale and rounding mode;
`FromBigFloat` rejects infinity with `ErrNonFiniteFloat`. `PowInt`
preserves exact natural scale for non-negative integer powers; `Pow` accepts an
integer-valued `Decimal` exponent and rounds through an explicit scale/mode.
Fractional exponents return `ErrInexact` instead of using hidden floating-point
approximations.

## Document Database Boundaries

For MongoDB-style Decimal128 Extended JSON without adding a driver dependency:

```go
data, err := json.Marshal(qdecimal.AsExtendedJSON(amount))
```

This emits:

```json
{"$numberDecimal":"123.4500"}
```

For raw BSON/document-store boundaries without pulling a driver into the core
module, use the dependency-free BSON string helpers:

```go
doc, err := amount.MarshalBSONDocument("amount")
err = amount.UnmarshalBSONDocument(doc, "amount")
```

These helpers store the decimal as canonical text inside a BSON string field so
scale and arbitrary precision are preserved beyond Decimal128's finite range.
Oversized BSON decimal text is rejected before it is copied into a Go string.
Declared BSON lengths are validated with architecture-neutral arithmetic.

## Security Statement

Decimal arithmetic is not cryptography and cannot be made "quantum-resistant" in the cryptographic sense. `qdecimal` focuses on security properties that matter for finance software:

- deterministic exact arithmetic;
- no NaN/infinity propagation;
- no package-global precision races;
- no panics from normal input constructors;
- no third-party Go module dependency in the core package;
- explicit rounding at every lossy boundary;
- fuzz, race, stress, and benchmark coverage.

## Shopspring Issue-Class Hardening

The test suite includes regression coverage for these issue classes:

- inconsistent rounding and rescaling;
- negative rounding and division sign errors;
- global division precision;
- JSON string-vs-number safety;
- configurable JSON number emission without global state;
- SQL scanner behavior;
- MongoDB-style Extended JSON boundary;
- dependency-free BSON string/document boundary;
- Unicode minus and custom decimal/thousands separators;
- negative zero;
- non-finite float inputs;
- `fmt.Formatter` support for decimal, fixed, and money values;
- map-key usage through canonical `Key` helpers;
- portable binary serialization;
- fuzz-tested parser and binary decode boundaries;
- exact `big.Rat` conversion and safe integer-exponent powers;
- exact-only division with `ErrInexact`;
- bounded-scale `Fixed64` hot path with checked overflow;
- currency-safe `Money` arithmetic and allocation;
- aggregate sum and average helpers;
- exchange tick-size quantization;
- min, max, clamp, and between helpers;
- concurrent immutable use.

## Verification

```bash
make check
make deps
make coverage
make audit
make stress
make fuzz-smoke
make bench-smoke
make cross-build
make vuln
make consumer-smoke
go test -run '^$' -bench . -benchmem ./...
go test -fuzz='^FuzzParse$' -fuzztime=30s .
go test -fuzz='^FuzzDecimalBinary$' -fuzztime=30s .
go test -fuzz='^FuzzFixed64Binary$' -fuzztime=30s .
go test -fuzz='^FuzzMoneyBinary$' -fuzztime=30s .
go test -fuzz='^FuzzDecimalBSONDocument$' -fuzztime=30s .
```

`make deps` fails if any external Go module appears in `go list -m all`.
`make coverage` fails below `COVERAGE_MIN` percent total statement coverage
(`85.0` by default). `make stress` runs deterministic property and concurrency
tests with `QDECIMAL_STRESS=1`. `make bench-smoke` executes representative
parser, arithmetic, Fixed64, Money, power, and append-binary benchmarks so hot
paths keep compiling and publishing cannot drift away from the benchmark suite.
`make cross-build` compile-checks qdecimal and its checked-in release helper for
Linux, Windows, macOS, BSD targets, and amd64/386/arm/arm64 class devices
without requiring separate physical runners for every target.
The normal test suite runs smaller versions of the same stress checks so CI
catches regressions quickly, while self-hosted release gates exercise the heavier profile.

See `HARDENING.md` for the issue-class coverage matrix and audit evidence.

## Releases

In this repository, qdecimal releases use the self-hosted `qdecimal Release`
workflow. Pushes to `main` publish `nightly`; versioned releases use
Go-native tags such as `v0.1.0`. See `RELEASE.md`.

Go libraries are not normally published through GitHub Packages. qdecimal is
published the standard Go way: a signed Git tag, a GitHub Release archive, and a
Go proxy/pkg.go.dev indexing step from the self-hosted release workflow.
