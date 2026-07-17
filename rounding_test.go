package qdecimal

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

var (
	_ fmt.Stringer             = ToNearestEven
	_ encoding.TextMarshaler   = ToNearestEven
	_ encoding.TextUnmarshaler = (*RoundingMode)(nil)
	_ json.Marshaler           = ToNearestEven
	_ json.Unmarshaler         = (*RoundingMode)(nil)
	_ json.Marshaler           = Context{}
	_ json.Unmarshaler         = (*Context)(nil)
	_ json.Marshaler           = MoneyContext{}
	_ json.Unmarshaler         = (*MoneyContext)(nil)
)

func TestRoundingModeNamesAliasesAndJSON(t *testing.T) {
	tests := []struct {
		input string
		want  RoundingMode
		name  string
	}{
		{"to_nearest_even", ToNearestEven, "to_nearest_even"},
		{"half-even", ToNearestEven, "to_nearest_even"},
		{"bankers", ToNearestEven, "to_nearest_even"},
		{"half_up", ToNearestAway, "to_nearest_away"},
		{"round half down", ToNearestTowardZero, "to_nearest_toward_zero"},
		{"up", AwayFromZero, "away_from_zero"},
		{"truncate", TowardZero, "toward_zero"},
		{"ceil", TowardPositive, "toward_positive"},
		{"floor", TowardNegative, "toward_negative"},
	}

	for _, tt := range tests {
		got, err := ParseRoundingMode(tt.input)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.input, err)
		}
		if got != tt.want || got.String() != tt.name {
			t.Fatalf("parse %q got %s/%d want %s/%d", tt.input, got, got, tt.name, tt.want)
		}
		text, err := got.MarshalText()
		if err != nil {
			t.Fatalf("marshal text %q: %v", tt.input, err)
		}
		if string(text) != tt.name {
			t.Fatalf("marshal text got %s want %s", text, tt.name)
		}
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal json %q: %v", tt.input, err)
		}
		if string(data) != `"`+tt.name+`"` {
			t.Fatalf("marshal json got %s want %q", data, tt.name)
		}
		var decoded RoundingMode
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal json %q: %v", tt.input, err)
		}
		if decoded != tt.want {
			t.Fatalf("json round trip got %s want %s", decoded, tt.want)
		}
	}

	if _, err := ParseRoundingMode("mystery"); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid rounding mode, got %v", err)
	}
	if _, err := RoundingMode(99).MarshalText(); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid marshal text, got %v", err)
	}
	var decoded RoundingMode
	if err := json.Unmarshal([]byte(`123`), &decoded); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected invalid JSON syntax, got %v", err)
	}
}

func TestContextPoliciesUseNamedRoundingModes(t *testing.T) {
	ctx := MustContext(2, ToNearestEven)
	if got := ctx.String(); got != "scale=2 rounding=to_nearest_even" {
		t.Fatalf("context string got %s", got)
	}
	money := MustMoneyContext("USD", 2, ToNearestAway)
	if got := money.String(); got != "USD scale=2 rounding=to_nearest_away" {
		t.Fatalf("money context string got %s", got)
	}

	data, err := json.Marshal(money)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"currency":"USD","scale":2,"rounding":"to_nearest_away"}` {
		t.Fatalf("money context JSON got %s", data)
	}
	var decoded MoneyContext
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != money {
		t.Fatalf("money context JSON round trip got %#v want %#v", decoded, money)
	}
	if err := json.Unmarshal([]byte(`{"currency":"usd","scale":2,"rounding":"half-up"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Currency != "USD" || decoded.Rounding != ToNearestAway {
		t.Fatalf("money context JSON did not normalize aliases: %#v", decoded)
	}
	if err := json.Unmarshal([]byte(`{"currency":"USD","scale":2,"rounding":"half-up","extra":true}`), &decoded); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected strict money context JSON error, got %v", err)
	}
	if err := json.Unmarshal([]byte(`{"currency":"USD","scale":2,"rounding":"wat"}`), &decoded); !errors.Is(err, ErrInvalidRoundingMode) {
		t.Fatalf("expected invalid money context rounding, got %v", err)
	}

	ctxData, err := json.Marshal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(ctxData) != `{"scale":2,"rounding":"to_nearest_even"}` {
		t.Fatalf("context JSON got %s", ctxData)
	}
	var decodedCtx Context
	if err := json.Unmarshal([]byte(`{"scale":4,"rounding":"truncate"}`), &decodedCtx); err != nil {
		t.Fatal(err)
	}
	if decodedCtx.Scale != 4 || decodedCtx.Rounding != TowardZero {
		t.Fatalf("context JSON got %#v", decodedCtx)
	}
	if err := json.Unmarshal([]byte(`{"scale":-1,"rounding":"truncate"}`), &decodedCtx); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("expected invalid context scale, got %v", err)
	}
}
