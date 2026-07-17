# Changelog

## [Unreleased]

### Added

- Exact `Decimal` type using coefficient and scale.
- Explicit rounding modes for finance.
- Audit-friendly rounding-mode names, aliases, text encoding, and JSON encoding.
- Strict, flexible, and custom-separator parsers.
- JSON, text, SQL scanner, SQL valuer, and fmt formatter support.
- Hardened formatter behavior for width, sign flags, quoted output,
  fixed-precision banker rounding, and capped precision on untrusted format
  strings.
- Scanner support for exact integer source types and `json.Number`, while
  keeping float scanner sources rejected by default.
- Configurable JSON string/number output without package-global toggles.
- MongoDB-style Decimal128 Extended JSON wrapper without driver dependency.
- Dependency-free BSON string value and single-field document helpers for
  document-database boundaries.
- Versioned binary marshal/unmarshal support.
- `NullDecimal` for nullable database, JSON, text, and formatter fields.
- Explicit `Context` for local scale and rounding policy.
- Strict JSON policy encoding/decoding for `Context` and `MoneyContext`.
- Minor-unit constructors and extractors for ledgers.
- Non-panicking `ValidateMinorScale` for runtime currency/asset configuration.
- Exact `Rat`, `FromRat`, `PowInt`, and safe integer-exponent `Pow` helpers.
- Exact-only `DivExact` with `ErrInexact` for repeating decimal quotients.
- Exact-only scale and context methods such as `RescaleExact`, `QuantizeExact`,
  `Context.AddExact`, `Context.QuantizeStepExact`, and
  `MoneyContext.AddExact`.
- Exact-only exchange tick helpers: `QuantizeStepExact`,
  `Money.QuantizeStepExact`, and `MoneyContext.QuantizeStepExact`.
- Exact-only minor-unit extraction helpers such as `Int64MinorUnitsExact`.
- `Sum`, `Avg`, `AvgExact`, and context-aware aggregate helpers including
  `Context.SumExact` and `Context.AvgExact`.
- `QuantizeStep` and context-aware `Context.QuantizeStep` for exchange tick
  sizes and non-scale increments.
- Conservative fast path for common ASCII decimal parsing.
- `Fixed64` bounded-scale hot path with checked overflow, JSON/text/SQL support,
  scale alignment, rounding, multiplication, division, range checks, min/max
  helpers, and exact `Decimal` conversion.
- `Fixed64.QuantizeStep` for bounded-scale exchange ticks, lot sizes, and
  banking increments.
- `Fixed64` aggregate helpers: `SumFixed64`, `AvgFixed64`, and
  `AvgFixed64Exact`.
- `Money` type with normalized currency/asset codes, currency-mismatch guards,
  JSON/text support, minor-unit conversion, scaling, tick quantization,
  exact tick validation, and exact-total allocation helpers.
- Currency-safe money aggregates: `SumMoney`, `AvgMoney`, and `AvgMoneyExact`.
- Currency-safe money range helpers: `Money.Between`, `Money.Clamp`,
  `MinMoney`, and `MaxMoney`.
- `MoneyContext` for reusable currency/asset scale and rounding policies without
  package-global currency metadata.
- Policy-aware `MoneyContext` aggregates and limits: `Sum`, `SumExact`, `Avg`,
  `AvgExact`, `Between`, `Clamp`, `Min`, and `Max`.
- SQL scanner/valuer support for `Money` and nullable JSON/SQL/text/formatter
  support through `NullMoney`.
- Nullable JSON/SQL/text/formatter support for `Fixed64` through `NullFixed64`.
- Compatibility constructors `NewFromString`, `NewFromFloat`, and
  `NewFromFloatWithScale`, plus public `ParseMoney` and `MustParseMoney`.
- `fmt.Formatter` support for `Fixed64` and `Money`, plus canonical `Key`
  helpers for map-key usage.
- `AppendText` support for `Fixed64` and `Money` reusable-buffer text paths.
- Stable binary and gob encoding for `Fixed64` and `Money`.
- `BinarySize` and zero-allocation `AppendBinary` paths for reusable-buffer
  binary serialization.
- Bounded binary/gob decode options and BSON decimal text limits for untrusted
  wire payloads.
- Architecture-neutral binary/BSON length validation for malformed wire
  payloads.
- `IsZero` helpers and documentation for optional JSON fields, nullable wrappers,
  and `omitempty`-safe pointer patterns.
- Deterministic stress suite for Fixed64/Decimal agreement, Fixed64 range
  invariants, Money allocation invariants, Money aggregate invariants,
  parse/string round trips, and concurrent hot paths.
- Regression tests for common decimal-library failure classes.
- Parser and binary-decoder fuzz targets, race-friendly concurrency stress test,
  benchmark smoke gate, and benchmarks.
- Configurable parser resource limits with `ErrLimitExceeded` for hostile raw
  digit, exponent, and scale expansion.
- Dependency policy gate that fails if the dependency-free core gains an
  external Go module.
- `govulncheck` vulnerability-scan target and CI/release gate.
- `make audit` and `HARDENING.md` issue-class coverage evidence.
- Self-hosted release publishing through a checked-in GitHub REST API helper,
  with compile validation and fake-GitHub REST flow tests through
  `make release-helper-check`.
- Atomic coverage target wired into audit, CI, and release verification with an
  85.0% minimum coverage gate.
