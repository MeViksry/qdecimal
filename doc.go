// Package qdecimal provides exact base-10 decimal arithmetic for finance,
// banking, exchanges, ledgers, and trading systems.
//
// The core Decimal type stores values as coefficient * 10^-scale and preserves
// visible scale such as 0.00 or 123.4500. Public operations never mutate their
// receiver and never rely on package-global precision settings.
//
// Operations that can lose information require an explicit scale and
// RoundingMode, or a Context/MoneyContext that carries that policy. Exact-only
// methods such as DivExact, AvgExact, RescaleExact, QuantizeExact,
// QuantizeStepExact, and context *Exact variants return ErrInexact instead of
// rounding.
//
// The package also includes Fixed64 for bounded-scale int64 hot paths, exchange
// tick quantization, range checks, and aggregates, Money for currency-safe
// arithmetic, nullable wrappers for SQL/JSON boundaries, formatter support,
// versioned binary encodings, raw BSON helpers, fuzz tests, stress tests, and
// release gates designed for production finance use.
package qdecimal
