# qdecimal Hardening Matrix

This file maps common finance-decimal failure classes to qdecimal design choices
and verification evidence. It is intended for maintainers, auditors, and teams
reviewing qdecimal before use in banking, exchange, ledger, or trading systems.

## Verification Commands

Run the full local audit gate:

```bash
make audit
```

That target runs:

- `make check` for format, dependency policy, unit tests, race detector, vet,
  and build;
- `make coverage` for atomic coverage profiling with a minimum total threshold;
- `make stress` for deterministic high-volume and concurrent tests;
- `make fuzz-smoke` for parser, binary, money, fixed64, and BSON fuzz smoke;
- `make bench-smoke` for representative hot-path benchmark execution;
- `make cross-build` for compile-only portability checks across major OS and
  CPU targets;
- `make vuln` for Go vulnerability scanning with `govulncheck`;
- `make consumer-smoke` for external-module import validation;
- `make release-dry-run` for guarded source archive and checksum creation.

For performance baselines:

```bash
go test -run '^$' -bench . -benchmem ./...
```

## Issue-Class Coverage

| Failure class | qdecimal mitigation | Evidence |
|---|---|---|
| Hidden global division precision | No package-global precision knobs. Lossy operations require explicit scale and rounding mode or a `Context` value, including tick-size quantization. | `context.go`, `decimal.go`, `TestContextQuantizeAndMinorUnits` |
| Inconsistent rounding aliases | `RoundUp`, `RoundDown`, `RoundCeil`, and `RoundFloor` are aliases for one validated `RoundingMode` enum. | `rounding.go`, `TestRoundingModesPositiveAndNegative` |
| Rounding does not rescale when no digit changes | `Rescale` always returns the requested scale. | `TestRegressionRoundingRescalesAndNegativeDivision` |
| Negative rounding sign errors | Quotient/remainder rounding is sign-aware and tested for all modes. | `roundQuotient`, `TestRoundingModesPositiveAndNegative`, `TestRescaleAllRoundingModesMatchRatOracle` |
| Need exact-only ledger boundaries | `DivExact`, `AvgExact`, `RescaleExact`, `QuantizeExact`, `QuantizeStepExact`, money/context exact tick helpers, exact aggregates, and exact minor-unit extraction return `ErrInexact` instead of rounding. | `TestExactArithmeticPreservesScale`, `TestExactScalePolicies`, `TestAggregates`, `TestContextQuantizeAndMinorUnits`, money tests |
| Broken or ambiguous power semantics | `PowInt` is exact; `Pow` accepts only integer-valued decimal exponents, requires explicit scale/rounding, and rejects fractional exponents with `ErrInexact`. | `pow.go`, `TestQuantizeRatPowAndBinaryRoundTrip` |
| Non-finite floats | Float constructors reject NaN and infinity, including `math/big.Float` infinity, with `ErrNonFiniteFloat` instead of panicking or storing invalid values. | `FromFloat64`, `FromBigFloat`, `TestFloatConstructorsNeverPanicOnNonFinite` |
| Float scanner precision loss | SQL scanners accept exact text, bytes, integers, and `json.Number`; float scanner sources are rejected by default. | `scanSource`, `TestJSONSQLAndNullDecimal` |
| Negative zero handling | Zero is canonicalized internally while preserved scale remains visible in string output. | `canonicalZero`, `TestParsingBoundaries` |
| Preserve visible scale such as `0.00` and `123.4500` | `String`, JSON, SQL values, binary, gob, and text preserve scale unless callers call normalization. | `String`, `TestExactArithmeticPreservesScale`, `TestJSONSQLAndNullDecimal` |
| JSON precision loss | Default JSON output is a string. Explicit numeric JSON wrappers are opt-in. | `MarshalJSONWithMode`, `Number`, `TestJSONSQLAndNullDecimal` |
| `omitempty` surprises | Nullable wrappers expose `IsZero`; docs recommend pointer fields when omission is required. | `NullDecimal`, `NullFixed64`, `NullMoney`, `TestJSONOmitEmptyAndZeroSemantics` |
| SQL NULL handling | Non-nullable values reject nil; nullable wrappers represent SQL NULL, JSON null, text `"null"`, and formatter output explicitly. | `NullDecimal`, `NullFixed64`, `NullMoney`, `TestJSONSQLAndNullDecimal`, `TestFixed64JSONTextAndSQL`, `TestMoneySQLAndNullMoney`, `TestNullableTextAndFormatInteroperability` |
| BSON/document database boundaries | Dependency-free Extended JSON and raw BSON string/document helpers preserve arbitrary precision. | `extendedjson.go`, `bson.go`, `TestBSONStringValueAndDocumentRoundTrip`, `FuzzDecimalBSONDocument` |
| Map-key usage | `Key` helpers produce normalized comparable strings without relying on struct comparability. | `Key`, `TestFormatterHelpersAndMapKey`, `TestFixed64FormatterAndKey`, `TestMoneyFormatterAndKey` |
| Formatter support | `Decimal`, `Fixed64`, `Money`, and nullable wrappers implement `fmt.Formatter` with tested width, sign, quote, fixed precision, capped precision rescaling, and chunked width padding. | `format.go`, `null.go`, formatter tests |
| Custom separators and Unicode minus | Strict and flexible parsers support Unicode minus and validated custom decimal/thousands separators. | `parse.go`, `TestParsingBoundaries` |
| Parser resource exhaustion | Default parse options cap raw digits, exponent length, expanded coefficient digits, and scale; trusted callers can override limits explicitly. | `ParseOptions.MaxDigits`, `ParseOptions.MaxScale`, `ParseOptions.MaxExponentDigits`, `ErrLimitExceeded`, `TestParsingBoundaries` |
| Invalid inputs causing panics | Public non-`Must*` APIs return errors; `Must*` helpers are explicit. | `TestInvalidInputsReturnErrorsWithoutPanic` |
| Race-prone global caches | Runtime arithmetic does not mutate package-global precision or factorial tables. | Race detector via `make check`; `TestStressConcurrentHotPaths` |
| Test visibility drift | `make coverage` produces an atomic coverage profile, function report, and minimum total coverage gate. | `Makefile`, self-hosted CI/release coverage steps |
| Performance benchmark drift | Representative parser, arithmetic, exact aggregates, Fixed64 aggregates, Fixed64 tick quantization, Fixed64 hot paths, Money, money aggregates, power, append-text, and append-binary benchmarks run in the audit gate. | `make bench-smoke`, self-hosted CI/release benchmark smoke steps |
| Device and OS portability drift | The core package and checked-in release helper compile with `CGO_ENABLED=0` across Linux, Windows, macOS, BSD targets, and amd64/386/arm/arm64 class devices. Runtime tests still run on the available self-hosted runner. | `make cross-build`, self-hosted CI/release cross-build steps |
| Fixed-scale hot path | `Fixed64` provides checked int64 operations, tick/lot quantization, range checks, clamps, min/max helpers, sum/average helpers, and zero-allocation append-text for bounded-scale ledger/trading loops. | `fixed64.go`, `TestStressFixed64MatchesDecimal`, Fixed64 tests/benchmarks |
| Currency mismatch bugs | `Money` validates currency codes and rejects cross-currency arithmetic, aggregates, range checks, clamps, and min/max operations. `MoneyContext` repeats those guards while returning context-scaled outputs. | `Money.sameCurrency`, `SumMoney`, `AvgMoney`, `Money.Between`, `Money.Clamp`, `MinMoney`, `MaxMoney`, `MoneyContext.sameCurrency`, money tests |
| Allocation total drift | Money allocation preserves the rounded total exactly. | `TestMoneyAllocationPreservesRoundedTotal`, `TestStressMoneyAllocationInvariants` |
| Binary/wire decode robustness | Binary and BSON decoders reject malformed payloads, cap untrusted resource expansion by default, validate length fields with architecture-neutral arithmetic, and are fuzz-tested. | `binary.go`, `bson.go`, `BinaryDecodeOptions`, `DefaultBinaryDecodeOptions()`, `DefaultMaxBSONDecimalTextBytes`, binary/BSON regression tests, fuzz targets |
| Security/vulnerability drift | CI/release gates run `govulncheck`; core has no third-party runtime dependency. | `Makefile`, `.github/workflows/qdecimal.yml`, `.github/workflows/qdecimal-release.yml`, `SECURITY.md` |
| Supply-chain dependency drift | `make deps` fails when `go list -m all` contains any external Go module. | `Makefile`, self-hosted CI/release dependency policy steps |
| Broken public import path | A generated external Go module imports `github.com/MeViksry/qdecimal`, uses finance APIs, and runs under `go test`. | `scripts/consumer-smoke.sh`, `make consumer-smoke` |
| Release packaging mistakes | Source archives are built by a checked-in guarded script and validated with checksums in `make audit` before publishing; release publishing uses a checked-in Go helper instead of a third-party action on the self-hosted write-token job, and the helper is tested against a fake GitHub REST server. | `scripts/release-archive.sh`, `internal/releasegithub`, `make release-dry-run`, `make release-helper-check`, `qdecimal Release` workflow |

## Operational Notes

- Decimal arithmetic is not cryptography and cannot be quantum-resistant in the
  cryptographic sense. qdecimal's security posture is deterministic exact
  arithmetic, explicit rounding policy, hardened parsing/serialization, fuzzed
  decode boundaries, and dependency minimization.
- Native driver adapters can wrap qdecimal's dependency-free core. The core
  intentionally avoids hard dependencies on ORM, SQL, BSON, or message-broker
  packages so large systems can choose their own audited integration layer.
- For public release, run `make audit` locally when desired and publish through the `qdecimal Release` workflow.
