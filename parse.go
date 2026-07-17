package qdecimal

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ParseOptions controls accepted human-input syntax. Parse uses strict,
// locale-neutral defaults except for the Unicode minus sign.
type ParseOptions struct {
	TrimSpace          bool
	AllowUnicodeMinus  bool
	AllowPlus          bool
	AllowThousands     bool
	DecimalSeparator   rune
	ThousandsSeparator rune
	MaxDigits          int
	MaxScale           int32
	MaxExponentDigits  int
}

const (
	DefaultMaxParseDigits         = 4096
	DefaultMaxParseScale          = 4096
	DefaultMaxParseExponentDigits = 10
)

// DefaultParseOptions accepts canonical ASCII decimals plus the Unicode minus sign.
var DefaultParseOptions = ParseOptions{
	AllowUnicodeMinus: true,
	AllowPlus:         true,
	DecimalSeparator:  '.',
	MaxDigits:         DefaultMaxParseDigits,
	MaxScale:          DefaultMaxParseScale,
	MaxExponentDigits: DefaultMaxParseExponentDigits,
}

// Parse parses a decimal string using DefaultParseOptions.
func Parse(s string) (Decimal, error) {
	if d, ok, err := parseSimpleASCIIString(s); ok || err != nil {
		return d, err
	}
	return ParseWithOptions(s, DefaultParseOptions)
}

// ParseBytes parses a decimal byte slice using DefaultParseOptions.
func ParseBytes(text []byte) (Decimal, error) {
	if d, ok, err := parseSimpleASCIIBytes(text); ok || err != nil {
		return d, err
	}
	return Parse(string(text))
}

// ParseFlexible parses common human-entry input: surrounding whitespace,
// Unicode minus, plus sign, and comma thousands separators.
func ParseFlexible(s string) (Decimal, error) {
	opts := DefaultParseOptions
	opts.TrimSpace = true
	opts.AllowThousands = true
	opts.ThousandsSeparator = ','
	return ParseWithOptions(s, opts)
}

// ParseWithOptions parses a decimal string with explicit syntax options.
func ParseWithOptions(s string, opts ParseOptions) (Decimal, error) {
	if opts.DecimalSeparator == 0 {
		opts.DecimalSeparator = '.'
	}
	if opts.ThousandsSeparator != 0 && opts.ThousandsSeparator == opts.DecimalSeparator {
		return Decimal{}, fmt.Errorf("%w: decimal and thousands separators must differ", ErrInvalidSyntax)
	}
	if opts.TrimSpace {
		s = strings.TrimSpace(s)
	}
	if s == "" {
		return Decimal{}, ErrInvalidSyntax
	}
	if strings.EqualFold(s, "nan") || strings.EqualFold(s, "inf") ||
		strings.EqualFold(s, "+inf") || strings.EqualFold(s, "-inf") ||
		strings.EqualFold(s, "infinity") || strings.EqualFold(s, "+infinity") ||
		strings.EqualFold(s, "-infinity") {
		return Decimal{}, ErrNonFiniteFloat
	}

	p := parser{s: s, opts: opts}
	return p.parse()
}

type parser struct {
	s          string
	opts       ParseOptions
	pos        int
	digitCount int
}

func (p *parser) parse() (Decimal, error) {
	negative := false
	if r, size := p.peek(); r == '-' || (p.opts.AllowUnicodeMinus && r == '−') {
		negative = true
		p.pos += size
	} else if r == '+' {
		if !p.opts.AllowPlus {
			return Decimal{}, ErrInvalidSyntax
		}
		p.pos += size
	}

	intDigits, groups, sawThousands, err := p.parseIntegerDigits()
	if err != nil {
		return Decimal{}, err
	}

	fracDigits := ""
	if r, size := p.peek(); r == p.opts.DecimalSeparator {
		p.pos += size
		fracDigits, err = p.parseDigits()
		if err != nil {
			return Decimal{}, err
		}
		if fracDigits == "" {
			return Decimal{}, ErrInvalidSyntax
		}
	}

	if intDigits == "" && fracDigits == "" {
		return Decimal{}, ErrInvalidSyntax
	}
	if sawThousands && !validThousandsGroups(groups) {
		return Decimal{}, fmt.Errorf("%w: invalid thousands grouping", ErrInvalidSyntax)
	}

	exp := int64(0)
	if r, size := p.peek(); r == 'e' || r == 'E' {
		p.pos += size
		exp, err = p.parseExponent()
		if err != nil {
			return Decimal{}, err
		}
	}
	if p.pos != len(p.s) {
		return Decimal{}, fmt.Errorf("%w: unexpected character %q", ErrInvalidSyntax, p.s[p.pos:])
	}

	digits := intDigits + fracDigits
	if digits == "" {
		digits = "0"
	}
	coef := new(big.Int)
	if _, ok := coef.SetString(digits, 10); !ok {
		return Decimal{}, ErrInvalidSyntax
	}
	if negative && coef.Sign() != 0 {
		coef.Neg(coef)
	}

	scale64 := int64(len(fracDigits)) - exp
	expandedDigits := int64(len(digits))
	if scale64 < 0 {
		expandedDigits += -scale64
	}
	if p.opts.MaxDigits > 0 && expandedDigits > int64(p.opts.MaxDigits) {
		return Decimal{}, ErrLimitExceeded
	}
	if p.opts.MaxScale > 0 && scale64 > int64(p.opts.MaxScale) {
		return Decimal{}, ErrLimitExceeded
	}
	if scale64 < 0 {
		coef.Mul(coef, pow10Int64(-scale64))
		scale64 = 0
	}
	if scale64 > maxScale {
		return Decimal{}, ErrInvalidScale
	}
	return Decimal{coef: *coef, scale: int32(scale64)}.canonicalZero(), nil
}

func (p *parser) parseIntegerDigits() (digits string, groups []int, sawThousands bool, err error) {
	var b strings.Builder
	groupLen := 0
	for p.pos < len(p.s) {
		r, size := p.peek()
		if isASCIIDigit(r) {
			if err := p.writeDigit(&b, r); err != nil {
				return "", nil, false, err
			}
			groupLen++
			p.pos += size
			continue
		}
		if p.opts.AllowThousands && p.opts.ThousandsSeparator != 0 && r == p.opts.ThousandsSeparator {
			if groupLen == 0 {
				return "", nil, false, ErrInvalidSyntax
			}
			groups = append(groups, groupLen)
			groupLen = 0
			sawThousands = true
			p.pos += size
			continue
		}
		break
	}
	if sawThousands {
		if groupLen == 0 {
			return "", nil, false, ErrInvalidSyntax
		}
		groups = append(groups, groupLen)
	}
	return b.String(), groups, sawThousands, nil
}

func (p *parser) parseDigits() (string, error) {
	var b strings.Builder
	for p.pos < len(p.s) {
		r, size := p.peek()
		if !isASCIIDigit(r) {
			break
		}
		if err := p.writeDigit(&b, r); err != nil {
			return "", err
		}
		p.pos += size
	}
	return b.String(), nil
}

func (p *parser) writeDigit(b *strings.Builder, r rune) error {
	p.digitCount++
	if p.opts.MaxDigits > 0 && p.digitCount > p.opts.MaxDigits {
		return ErrLimitExceeded
	}
	b.WriteRune(r)
	return nil
}

func (p *parser) parseExponent() (int64, error) {
	sign := int64(1)
	if r, size := p.peek(); r == '-' {
		sign = -1
		p.pos += size
	} else if r == '+' {
		p.pos += size
	}

	start := p.pos
	digits := 0
	for p.pos < len(p.s) {
		r, size := p.peek()
		if !isASCIIDigit(r) {
			break
		}
		digits++
		if p.opts.MaxExponentDigits > 0 && digits > p.opts.MaxExponentDigits {
			return 0, ErrLimitExceeded
		}
		p.pos += size
	}
	if start == p.pos {
		return 0, ErrInvalidSyntax
	}
	value, err := strconv.ParseInt(p.s[start:p.pos], 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: exponent out of range", ErrInvalidScale)
	}
	if value > math.MaxInt32 {
		return 0, ErrInvalidScale
	}
	return sign * value, nil
}

func (p *parser) peek() (rune, int) {
	if p.pos >= len(p.s) {
		return 0, 0
	}
	r, size := utf8.DecodeRuneInString(p.s[p.pos:])
	return r, size
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func validThousandsGroups(groups []int) bool {
	if len(groups) < 2 {
		return false
	}
	if groups[0] < 1 || groups[0] > 3 {
		return false
	}
	for _, group := range groups[1:] {
		if group != 3 {
			return false
		}
	}
	return true
}

func scanSource(src any) (Decimal, error) {
	switch value := src.(type) {
	case nil:
		return Decimal{}, ErrNilValue
	case Decimal:
		return value.copy(), nil
	case string:
		return Parse(value)
	case []byte:
		return ParseBytes(value)
	case json.Number:
		return Parse(value.String())
	case int:
		return NewFromInt(int64(value)), nil
	case int8:
		return NewFromInt(int64(value)), nil
	case int16:
		return NewFromInt(int64(value)), nil
	case int32:
		return NewFromInt(int64(value)), nil
	case int64:
		return NewFromInt(value), nil
	case uint:
		return NewFromUint64(uint64(value)), nil
	case uint8:
		return NewFromUint64(uint64(value)), nil
	case uint16:
		return NewFromUint64(uint64(value)), nil
	case uint32:
		return NewFromUint64(uint64(value)), nil
	case uint64:
		return NewFromUint64(value), nil
	default:
		return Decimal{}, fmt.Errorf("%w: %T", ErrInvalidSource, src)
	}
}

func parseSimpleASCIIString(s string) (Decimal, bool, error) {
	if s == "" {
		return Decimal{}, true, ErrInvalidSyntax
	}
	negative := false
	pos := 0
	if s[pos] == '-' || s[pos] == '+' {
		negative = s[pos] == '-'
		pos++
		if pos == len(s) {
			return Decimal{}, true, ErrInvalidSyntax
		}
	}

	var coef uint64
	var digits int
	var scale int32
	seenPoint := false
	for ; pos < len(s); pos++ {
		c := s[pos]
		switch {
		case c >= '0' && c <= '9':
			digits++
			if digits > 18 {
				return Decimal{}, false, nil
			}
			coef = coef*10 + uint64(c-'0')
			if seenPoint {
				scale++
			}
		case c == '.':
			if seenPoint {
				return Decimal{}, true, ErrInvalidSyntax
			}
			seenPoint = true
		default:
			return Decimal{}, false, nil
		}
	}
	if digits == 0 || (seenPoint && scale == 0) {
		return Decimal{}, true, ErrInvalidSyntax
	}
	var out Decimal
	out.coef.SetUint64(coef)
	if negative && coef != 0 {
		out.coef.Neg(&out.coef)
	}
	out.scale = scale
	return out.canonicalZero(), true, nil
}

func parseSimpleASCIIBytes(text []byte) (Decimal, bool, error) {
	if len(text) == 0 {
		return Decimal{}, true, ErrInvalidSyntax
	}
	negative := false
	pos := 0
	if text[pos] == '-' || text[pos] == '+' {
		negative = text[pos] == '-'
		pos++
		if pos == len(text) {
			return Decimal{}, true, ErrInvalidSyntax
		}
	}

	var coef uint64
	var digits int
	var scale int32
	seenPoint := false
	for ; pos < len(text); pos++ {
		c := text[pos]
		switch {
		case c >= '0' && c <= '9':
			digits++
			if digits > 18 {
				return Decimal{}, false, nil
			}
			coef = coef*10 + uint64(c-'0')
			if seenPoint {
				scale++
			}
		case c == '.':
			if seenPoint {
				return Decimal{}, true, ErrInvalidSyntax
			}
			seenPoint = true
		default:
			return Decimal{}, false, nil
		}
	}
	if digits == 0 || (seenPoint && scale == 0) {
		return Decimal{}, true, ErrInvalidSyntax
	}
	var out Decimal
	out.coef.SetUint64(coef)
	if negative && coef != 0 {
		out.coef.Neg(&out.coef)
	}
	out.scale = scale
	return out.canonicalZero(), true, nil
}
