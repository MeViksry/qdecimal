# Security Policy

## Supported Scope

`qdecimal` protects finance calculations from precision loss, hidden global state, and malformed input. It is not encryption, signing, authentication, authorization, or a post-quantum cryptographic primitive.

## Security-Relevant Guarantees

- NaN and infinity are rejected at constructors.
- Division requires explicit scale and rounding mode.
- JSON output is precision-preserving text.
- SQL `NULL` is rejected by non-nullable `Decimal` and represented by `NullDecimal`.
- Public operations do not mutate receivers.
- The package has no third-party Go module dependency, enforced by `make deps`.
- Parser digit, scale, and exponent expansion is bounded by default and returns `ErrLimitExceeded` when configured limits are crossed.
- Binary/gob coefficient bytes, binary/gob scale, and BSON decimal text are bounded by default for untrusted payloads.
- Binary, gob, and BSON length fields are validated with architecture-neutral arithmetic before slice conversion.
- Binary and BSON decode boundaries are covered by fuzz targets and resource-limit regression tests.
- CI/release workflows run Go vulnerability scanning with `govulncheck`.
- Self-hosted release publishing uses checked-in source and scoped `GITHUB_TOKEN`
  permissions instead of third-party release action code.

## Reporting

Report vulnerabilities privately through the repository's security reporting channel. Do not publish exploitable defects in public issues before a fix is available.
