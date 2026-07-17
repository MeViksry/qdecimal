package qdecimal

import "errors"

var (
	// ErrInvalidSyntax indicates malformed decimal text.
	ErrInvalidSyntax = errors.New("qdecimal: invalid decimal syntax")
	// ErrInvalidScale indicates a negative or unsupported decimal scale.
	ErrInvalidScale = errors.New("qdecimal: invalid decimal scale")
	// ErrDivisionByZero indicates division by zero.
	ErrDivisionByZero = errors.New("qdecimal: division by zero")
	// ErrNonFiniteFloat indicates NaN or infinity was passed to a float constructor.
	ErrNonFiniteFloat = errors.New("qdecimal: non-finite float")
	// ErrNilValue indicates SQL NULL or JSON null was assigned to a non-nullable Decimal.
	ErrNilValue = errors.New("qdecimal: nil cannot be assigned to a non-null decimal")
	// ErrInvalidSource indicates an unsupported database scanner source type.
	ErrInvalidSource = errors.New("qdecimal: unsupported database source type")
	// ErrInvalidRoundingMode indicates an unknown rounding mode.
	ErrInvalidRoundingMode = errors.New("qdecimal: invalid rounding mode")
	// ErrOverflow indicates a requested conversion cannot fit in the target type.
	ErrOverflow = errors.New("qdecimal: overflow")
	// ErrInexact indicates an exact-only operation would require rounding.
	ErrInexact = errors.New("qdecimal: inexact decimal result")
	// ErrEmptyInput indicates an aggregate operation received no values.
	ErrEmptyInput = errors.New("qdecimal: empty input")
	// ErrInvalidCurrency indicates a malformed money currency code.
	ErrInvalidCurrency = errors.New("qdecimal: invalid currency code")
	// ErrCurrencyMismatch indicates money values with different currencies were combined.
	ErrCurrencyMismatch = errors.New("qdecimal: currency mismatch")
	// ErrInvalidAllocation indicates a money allocation with invalid parts or ratios.
	ErrInvalidAllocation = errors.New("qdecimal: invalid money allocation")
	// ErrLimitExceeded indicates input exceeded a configured parser resource limit.
	ErrLimitExceeded = errors.New("qdecimal: input exceeds configured limit")
)
