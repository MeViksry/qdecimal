package qdecimal

import (
	"encoding/json"
	"fmt"
	"strings"
)

// RoundingMode controls how discarded fractional digits are handled.
type RoundingMode byte

const (
	// ToNearestEven rounds to the nearest value, with ties going to the even digit.
	ToNearestEven RoundingMode = iota
	// ToNearestAway rounds to the nearest value, with ties away from zero.
	ToNearestAway
	// ToNearestTowardZero rounds to the nearest value, with ties toward zero.
	ToNearestTowardZero
	// AwayFromZero rounds any discarded non-zero digit away from zero.
	AwayFromZero
	// TowardZero truncates discarded digits.
	TowardZero
	// TowardPositive rounds toward +infinity.
	TowardPositive
	// TowardNegative rounds toward -infinity.
	TowardNegative
)

// Common finance-oriented aliases.
const (
	RoundBankers  = ToNearestEven
	RoundHalfUp   = ToNearestAway
	RoundHalfDown = ToNearestTowardZero
	RoundUp       = AwayFromZero
	RoundDown     = TowardZero
	RoundCeil     = TowardPositive
	RoundFloor    = TowardNegative
)

func (m RoundingMode) valid() bool {
	return m <= TowardNegative
}

// String returns a stable audit-friendly name for m.
func (m RoundingMode) String() string {
	switch m {
	case ToNearestEven:
		return "to_nearest_even"
	case ToNearestAway:
		return "to_nearest_away"
	case ToNearestTowardZero:
		return "to_nearest_toward_zero"
	case AwayFromZero:
		return "away_from_zero"
	case TowardZero:
		return "toward_zero"
	case TowardPositive:
		return "toward_positive"
	case TowardNegative:
		return "toward_negative"
	default:
		return fmt.Sprintf("rounding_mode(%d)", m)
	}
}

// ParseRoundingMode parses stable names and common finance aliases.
func ParseRoundingMode(text string) (RoundingMode, error) {
	key := strings.ToLower(strings.TrimSpace(text))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	switch key {
	case "to_nearest_even", "nearest_even", "half_even", "bankers", "round_bankers":
		return ToNearestEven, nil
	case "to_nearest_away", "nearest_away", "half_up", "round_half_up":
		return ToNearestAway, nil
	case "to_nearest_toward_zero", "nearest_toward_zero", "half_down", "round_half_down":
		return ToNearestTowardZero, nil
	case "away_from_zero", "up", "round_up":
		return AwayFromZero, nil
	case "toward_zero", "towards_zero", "down", "truncate", "round_down":
		return TowardZero, nil
	case "toward_positive", "towards_positive", "ceil", "ceiling", "round_ceil":
		return TowardPositive, nil
	case "toward_negative", "towards_negative", "floor", "round_floor":
		return TowardNegative, nil
	default:
		return 0, ErrInvalidRoundingMode
	}
}

// MarshalText implements encoding.TextMarshaler.
func (m RoundingMode) MarshalText() ([]byte, error) {
	if !m.valid() {
		return nil, ErrInvalidRoundingMode
	}
	return []byte(m.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (m *RoundingMode) UnmarshalText(text []byte) error {
	if m == nil {
		return ErrNilValue
	}
	parsed, err := ParseRoundingMode(string(text))
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// MarshalJSON implements json.Marshaler as a stable string.
func (m RoundingMode) MarshalJSON() ([]byte, error) {
	text, err := m.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(text))
}

// UnmarshalJSON implements json.Unmarshaler from a stable string.
func (m *RoundingMode) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSyntax, err)
	}
	return m.UnmarshalText([]byte(text))
}
