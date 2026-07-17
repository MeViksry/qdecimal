package qdecimal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func TestBSONStringValueAndDocumentRoundTrip(t *testing.T) {
	amount := MustParse("-123.4500")
	value, err := amount.MarshalBSONStringValue()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := int(binary.LittleEndian.Uint32(value[:4])), len("-123.4500")+1; got != want {
		t.Fatalf("bson string length got %d want %d", got, want)
	}
	if value[len(value)-1] != 0 {
		t.Fatal("bson string value missing terminator")
	}

	var decoded Decimal
	if err := decoded.UnmarshalBSONStringValue(value); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(amount) || decoded.Scale() != amount.Scale() {
		t.Fatalf("bson value round trip got %s scale=%d", decoded, decoded.Scale())
	}

	document, err := amount.MarshalBSONDocument("amount")
	if err != nil {
		t.Fatal(err)
	}
	if got := int(binary.LittleEndian.Uint32(document[:4])); got != len(document) {
		t.Fatalf("bson document length got %d want %d", got, len(document))
	}
	if document[len(document)-1] != 0 {
		t.Fatal("bson document missing terminator")
	}
	var documentDecoded Decimal
	if err := documentDecoded.UnmarshalBSONDocument(document, "amount"); err != nil {
		t.Fatal(err)
	}
	if !documentDecoded.Equal(amount) || documentDecoded.Scale() != amount.Scale() {
		t.Fatalf("bson document round trip got %s scale=%d", documentDecoded, documentDecoded.Scale())
	}
}

func TestBSONRejectsMalformedInput(t *testing.T) {
	if _, err := One.MarshalBSONDocument(""); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected empty BSON field error, got %v", err)
	}
	if _, err := One.MarshalBSONDocument("bad\x00field"); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected NUL BSON field error, got %v", err)
	}

	var decoded Decimal
	if err := decoded.UnmarshalBSONStringValue([]byte{1, 0, 0, 0, '1'}); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected unterminated string error, got %v", err)
	}
	hugeDeclaredString := []byte{0xff, 0xff, 0xff, 0xff, '1', 0}
	if err := decoded.UnmarshalBSONStringValue(hugeDeclaredString); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected huge declared BSON string length error, got %v", err)
	}
	oversizedText := bytes.Repeat([]byte{'1'}, DefaultMaxBSONDecimalTextBytes+1)
	oversizedValue := make([]byte, 4, 4+len(oversizedText)+1)
	binary.LittleEndian.PutUint32(oversizedValue[:4], uint32(len(oversizedText)+1))
	oversizedValue = append(oversizedValue, oversizedText...)
	oversizedValue = append(oversizedValue, 0)
	if err := decoded.UnmarshalBSONStringValue(oversizedValue); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected oversized BSON decimal text error, got %v", err)
	}
	doc, err := One.MarshalBSONDocument("amount")
	if err != nil {
		t.Fatal(err)
	}
	if err := decoded.UnmarshalBSONDocument(doc, "price"); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected wrong field error, got %v", err)
	}
	badType := append([]byte(nil), doc...)
	badType[4] = 0x10
	if err := decoded.UnmarshalBSONDocument(badType, "amount"); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected wrong type error, got %v", err)
	}
	badLength := append([]byte(nil), doc...)
	binary.LittleEndian.PutUint32(badLength[:4], uint32(len(badLength)+1))
	if err := decoded.UnmarshalBSONDocument(badLength, "amount"); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected wrong document length error, got %v", err)
	}
	hugeDocumentLength := append([]byte(nil), doc...)
	binary.LittleEndian.PutUint32(hugeDocumentLength[:4], math.MaxUint32)
	if err := decoded.UnmarshalBSONDocument(hugeDocumentLength, "amount"); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected huge declared document length error, got %v", err)
	}
	hugeValueLength := append([]byte(nil), doc...)
	valueOffset := 4 + 1 + len("amount") + 1
	binary.LittleEndian.PutUint32(hugeValueLength[valueOffset:valueOffset+4], math.MaxUint32)
	if err := decoded.UnmarshalBSONDocument(hugeValueLength, "amount"); !errors.Is(err, ErrInvalidSyntax) {
		t.Fatalf("expected huge declared BSON value length error, got %v", err)
	}
}
