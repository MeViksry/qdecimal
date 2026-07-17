package qdecimal

import (
	"fmt"
	"io"
	"strconv"
)

// DefaultMaxFormatScale caps precision-driven fmt rescaling for untrusted format
// strings. Call explicit Rescale/StringFixed APIs when a larger trusted scale is
// truly required.
const DefaultMaxFormatScale int32 = DefaultMaxParseScale

func formatScale(s fmt.State) (int32, bool, error) {
	precision, ok := s.Precision()
	if !ok {
		return 0, false, nil
	}
	if precision > int(DefaultMaxFormatScale) {
		return 0, true, ErrLimitExceeded
	}
	return int32(precision), true, nil
}

func writeFormattedNumber(s fmt.State, verb rune, text, typeName string, sign int) {
	if !supportedFormatVerb(verb) {
		writeUnsupportedFormat(s, verb, typeName, text)
		return
	}
	if verb == 'q' {
		writePadded(s, strconv.Quote(text))
		return
	}
	writePadded(s, applyNumericSign(s, text, sign))
}

func writeFormattedMoney(s fmt.State, verb rune, currency, amountText string, sign int) {
	if !supportedFormatVerb(verb) {
		writeUnsupportedFormat(s, verb, "qdecimal.Money", currency+" "+amountText)
		return
	}

	text := currency + " " + applyNumericSign(s, amountText, sign)
	if verb == 'q' {
		text = strconv.Quote(text)
	}
	writePadded(s, text)
}

func writeFormattedType(s fmt.State, verb rune, text, typeName string) {
	if !supportedFormatVerb(verb) {
		writeUnsupportedFormat(s, verb, typeName, text)
		return
	}
	if verb == 'q' {
		text = strconv.Quote(text)
	}
	writePadded(s, text)
}

func supportedFormatVerb(verb rune) bool {
	return verb == 'v' || verb == 's' || verb == 'f' || verb == 'F' || verb == 'q'
}

func applyNumericSign(s fmt.State, text string, sign int) string {
	if sign < 0 || len(text) == 0 || text[0] == '-' || text[0] == '+' {
		return text
	}
	if s.Flag('+') {
		return "+" + text
	}
	if s.Flag(' ') {
		return " " + text
	}
	return text
}

func writeUnsupportedFormat(s fmt.State, verb rune, typeName, text string) {
	fmt.Fprintf(s, "%%!%c(%s=%s)", verb, typeName, text)
}

func writeFormatError(s fmt.State, verb rune, typeName, text string, err error) {
	fmt.Fprintf(s, "%%!%c(%s=%s: %v)", verb, typeName, text, err)
}

func writePadded(s fmt.State, text string) {
	width, hasWidth := s.Width()
	if !hasWidth || width <= len(text) {
		_, _ = s.Write([]byte(text))
		return
	}

	padByte := byte(' ')
	if s.Flag('0') && !s.Flag('-') {
		padByte = '0'
	}
	padding := width - len(text)

	if s.Flag('-') {
		_, _ = s.Write([]byte(text))
		writeRepeatedByte(s, padding, padByte)
		return
	}

	if padByte == '0' && len(text) > 0 && (text[0] == '-' || text[0] == '+') {
		_, _ = s.Write([]byte{text[0]})
		writeRepeatedByte(s, padding, padByte)
		_, _ = s.Write([]byte(text[1:]))
		return
	}

	writeRepeatedByte(s, padding, padByte)
	_, _ = s.Write([]byte(text))
}

func writeRepeatedByte(s fmt.State, count int, padByte byte) {
	const spaces = "                                                                "
	const zeros = "0000000000000000000000000000000000000000000000000000000000000000"

	if count <= 0 {
		return
	}
	chunk := spaces
	if padByte == '0' {
		chunk = zeros
	}
	for count > len(chunk) {
		_, _ = io.WriteString(s, chunk)
		count -= len(chunk)
	}
	_, _ = io.WriteString(s, chunk[:count])
}
