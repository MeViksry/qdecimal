# qdecimal Design

## Scope

`qdecimal` is a deterministic decimal arithmetic package for finance. It is not a symbolic math package, trigonometry package, floating-point replacement for scientific computing, or cryptographic primitive.

## Representation

Each value is:

```text
coefficient * 10^-scale
```

The coefficient is a `math/big.Int`; the scale is a non-negative `int32`. This representation makes addition, subtraction, multiplication, comparison, serialization, and fixed-scale rounding exact.

## Fixed64 Hot Path

`Fixed64` is a secondary representation for domains whose range and scale are
known in advance. It stores signed int64 minor units plus scale, rejects scales
above 18, formats without converting through `math/big.Int`, and checks every
scale alignment or arithmetic overflow.

The arbitrary-precision `Decimal` type remains the general-purpose boundary.
`Fixed64` can convert to `Decimal` exactly and uses the same rounding-mode
semantics for lossy operations, so hot-path code and boundary code do not drift.

## Money Layer

`Money` couples a `Decimal` amount with a normalized currency or exchange asset
code. It rejects arithmetic and comparisons across different codes so service
bugs cannot silently combine unrelated units.

`MoneyContext` carries one currency/asset code plus scale and rounding policy.
It is the money equivalent of `Context`: useful for audited service boundaries
such as USD cents, BTC 8-decimal balances, exchange fee precision, or reporting
precision. It validates currency on every operation and rejects zero-value or
manually malformed policies.

The package intentionally avoids a built-in ISO-4217 minor-unit table. Currency
metadata changes, crypto assets have venue-specific precision rules, and banks
often need policy overrides. `Money` therefore requires explicit scale and
rounding for rounding, minor-unit extraction, multiplication, division, and
allocation.

## No Global Precision

Division is not controlled by a package variable. Callers must provide:

- output scale;
- rounding mode.

This avoids hidden process-wide state, data races, and surprising behavior changes between packages.

`Context` is a small value that carries scale and rounding policy explicitly. Services can pass contexts for USD cents, exchange tick sizes, fees, or reporting precision without changing package-global behavior.

`QuantizeStep` handles exchange tick sizes and other increments that are not expressible by scale alone, such as `0.05`. `Context.QuantizeStep` applies that rule through an explicit scale/rounding policy, while `QuantizeStepExact`, `Context.QuantizeStepExact`, and the Money exact tick helpers reject values that are not already valid multiples of the step. `Fixed64.QuantizeStep` provides the same policy for bounded-scale trading loops with a checked integer fast path and a Decimal fallback for overflow-sized alignments.

Exact-only scale operations (`RescaleExact`, `QuantizeExact`, and context
`*Exact` methods) return `ErrInexact` instead of rounding when non-zero digits
would be discarded. This gives ledgers and reconciliation jobs a strict mode
for boundaries where any implicit rounding would be a defect.

## Rounding

Rounding operates on integer quotient/remainder arithmetic. Negative numbers are handled by sign-aware magnitude increments so `-1.5` cannot accidentally round to `+2` or `+1`.

Rounding modes have stable string names and JSON/text encodings. This keeps
service configuration, logs, and audit records readable instead of storing
magic numeric constants for financial policy.

`Context` and `MoneyContext` provide strict JSON policy encoding. Unknown fields
are rejected on decode, currency codes are normalized through the same
constructor path as runtime code, and rounding names are parsed through
`ParseRoundingMode`.

`DivExact` is available when rounding is forbidden. It reduces the quotient and accepts only denominators whose prime factors are 2 and/or 5, the finite-decimal condition.

## Scale Preservation

`String` preserves scale. This allows `0.00`, `1.20`, and `123.4500` to remain visible at database/API boundaries. `Normalize` is available when callers want canonical numeric form.

## Input Boundaries

The strict parser is locale-neutral. `ParseWithOptions` handles controlled
human-entry boundaries with explicit decimal and thousands separators. Parsing
also has configurable raw-digit, scale-expansion, and exponent-length limits,
enabled by default, so hostile inputs such as huge exponents are rejected with
`ErrLimitExceeded` before allocating unbounded coefficients.

`FromFloat64` rejects NaN and infinity. `FromBigFloat` also rejects
`math/big.Float` infinity. These APIs exist for integration boundaries; finance
systems should prefer string, integer minor units, or exact decimal input.

Common ASCII decimal input uses a small-coefficient fast path before falling back to the full parser. `ParseFixed64` has a bounded int64 fast path for canonical ASCII inputs and falls back to `Decimal` parsing for supported wider syntax such as Unicode minus or exponents. The fast paths are conservative and do not accept syntax that the full parser would reject.

## JSON

JSON marshaling emits strings by default. Numeric JSON tokens are accepted during unmarshal because some upstream systems emit them, but `qdecimal` does not emit numbers unless callers explicitly request `EmitJSONNumber` or wrap a value with `AsNumber`.

Optional fields should use pointers when omission is required, and `NullDecimal`
or `NullMoney` when explicit JSON `null` is required. The package provides
`IsZero` methods for nullable wrappers and JSON wrappers, but Go's traditional
`omitempty` handling does not omit non-pointer struct values that implement
`json.Marshaler`.

## Formatting and Keys

`Decimal`, `Fixed64`, and `Money` implement `fmt.Formatter` for predictable
reporting and logging. `Key` helpers produce canonical comparable strings for
map keys without relying on struct comparability or preserved scale.

## Binary Formats

The decimal binary format is versioned and network-order:

```text
QDEC | version | sign | scale uint32 | coefficient length uint32 | absolute coefficient bytes
```

`Fixed64` uses `QF64 | version | scale uint32 | units int64`.

`Money` uses `QMON | version | currency length uint16 | currency bytes |
decimal length uint32 | decimal binary bytes`.

These formats are intended for deterministic file, cache, and wire storage.
`GobEncode` and `GobDecode` use the same stable bytes. JSON and SQL text remain
the most portable external interchange formats.

Default binary and gob decoders bound coefficient bytes and scale to protect
untrusted wire payloads from resource expansion. Trusted storage readers can use
`UnmarshalBinaryWithOptions` and `DefaultBinaryDecodeOptions()` to raise or
disable the specific limit they own. Wire length fields are validated with
architecture-neutral integer arithmetic before any slice conversion.

`BinarySize` reports the exact encoded length, and `AppendBinary` writes into a
caller-owned buffer. On already-sized buffers this gives hot-path systems a
zero-allocation option while preserving standard `encoding.BinaryMarshaler`
compatibility.

## Document Databases

`ExtendedJSON` supports MongoDB-style Decimal128 Extended JSON without importing a specific driver. This keeps the core dependency-free while giving driver adapters a stable bridge format.

The BSON helpers encode decimal values as BSON string values/documents. This is
deliberate: raw Decimal128 has finite precision and exponent limits, while
qdecimal's core representation is arbitrary precision. Driver-specific adapters
can still map to native Decimal128 when an application chooses that constraint.
Decode paths reject oversized text before string conversion and then reuse the
strict parser limits. Declared BSON document/string lengths are validated before
slice indexing so malformed payloads behave the same on 32-bit and 64-bit
architectures.

## Exact Interop

`Rat` returns an exact `math/big.Rat` representation. `FromRat` and `FromBigFloat` round through the same scale/mode policy as division.

`PowInt` computes non-negative integer powers exactly. `Pow` accepts an
integer-valued `Decimal` exponent, requires explicit output scale and rounding,
supports negative integer exponents through division, and rejects fractional
exponents with `ErrInexact`. qdecimal intentionally does not expose approximate
trigonometric or arbitrary fractional-power APIs in the finance core.

## Concurrency

The public API is immutable: methods allocate new coefficients for results and never mutate receivers. There are no mutable global precision or formatting knobs.

## Stress Testing

The test suite includes deterministic stress checks for Decimal/Fixed64
agreement, Money allocation invariants, parse/string round trips, and concurrent
hot paths. Default runs keep iteration counts CI-friendly; `QDECIMAL_STRESS=1`
activates the heavier profile used by `make stress` and release workflows.

## Quantum Claims

Decimal arithmetic does not become cryptographically secure against quantum computers. The relevant security posture is exactness, deterministic behavior, input rejection, absence of global races, and test coverage.
