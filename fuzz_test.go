package qdecimal

import "testing"

func FuzzParse(f *testing.F) {
	f.Add("0")
	f.Add("-0.00")
	f.Add("123456789.987654321")
	f.Add("1.23e-4")
	f.Add("−12.30")
	f.Fuzz(func(t *testing.T, input string) {
		d, err := Parse(input)
		if err != nil {
			return
		}
		roundTrip, err := Parse(d.String())
		if err != nil {
			t.Fatalf("canonical string did not parse: %v", err)
		}
		if !d.Equal(roundTrip) || d.Scale() != roundTrip.Scale() {
			t.Fatalf("round trip mismatch: %s -> %s", d, roundTrip)
		}
	})
}

func FuzzDecimalBinary(f *testing.F) {
	for _, value := range []Decimal{
		Zero,
		MustParse("123456789.987654321"),
		MustParse("-123456789.987654321"),
		MustParse("0.000000000000000001"),
	} {
		data, err := value.MarshalBinary()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte("bad"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded Decimal
		if err := decoded.UnmarshalBinary(data); err != nil {
			return
		}
		encoded, err := decoded.MarshalBinary()
		if err != nil {
			t.Fatalf("valid decoded decimal did not marshal: %v", err)
		}
		var roundTrip Decimal
		if err := roundTrip.UnmarshalBinary(encoded); err != nil {
			t.Fatalf("re-encoded decimal did not decode: %v", err)
		}
		if !roundTrip.Equal(decoded) || roundTrip.Scale() != decoded.Scale() {
			t.Fatalf("binary round trip mismatch: %s/%d -> %s/%d", decoded, decoded.Scale(), roundTrip, roundTrip.Scale())
		}
	})
}

func FuzzFixed64Binary(f *testing.F) {
	seeds := []Fixed64{
		mustFuzzFixed64(0, 0),
		mustFuzzFixed64(123456789, 4),
		mustFuzzFixed64(-123456789, 4),
		mustFuzzFixed64(-9223372036854775808, 0),
	}
	for _, value := range seeds {
		data, err := value.MarshalBinary()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte("bad"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded Fixed64
		if err := decoded.UnmarshalBinary(data); err != nil {
			return
		}
		encoded, err := decoded.MarshalBinary()
		if err != nil {
			t.Fatalf("valid decoded fixed64 did not marshal: %v", err)
		}
		var roundTrip Fixed64
		if err := roundTrip.UnmarshalBinary(encoded); err != nil {
			t.Fatalf("re-encoded fixed64 did not decode: %v", err)
		}
		if !roundTrip.Equal(decoded) || roundTrip.Scale() != decoded.Scale() {
			t.Fatalf("fixed64 binary round trip mismatch: %s/%d -> %s/%d", decoded, decoded.Scale(), roundTrip, roundTrip.Scale())
		}
	})
}

func FuzzMoneyBinary(f *testing.F) {
	for _, value := range []Money{
		MustParseMoney("USD 0"),
		MustParseMoney("USD 123456789.9876"),
		MustParseMoney("BTC -0.00000001"),
		MustParseMoney("USDT -123456789.987654321"),
	} {
		data, err := value.MarshalBinary()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte("bad"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded Money
		if err := decoded.UnmarshalBinary(data); err != nil {
			return
		}
		encoded, err := decoded.MarshalBinary()
		if err != nil {
			t.Fatalf("valid decoded money did not marshal: %v", err)
		}
		var roundTrip Money
		if err := roundTrip.UnmarshalBinary(encoded); err != nil {
			t.Fatalf("re-encoded money did not decode: %v", err)
		}
		if !roundTrip.Equal(decoded) || roundTrip.Amount().Scale() != decoded.Amount().Scale() {
			t.Fatalf("money binary round trip mismatch: %s -> %s", decoded, roundTrip)
		}
	})
}

func FuzzDecimalBSONDocument(f *testing.F) {
	for _, value := range []Decimal{
		Zero,
		MustParse("123456789.987654321"),
		MustParse("-123456789.987654321"),
		MustParse("0.000000000000000001"),
	} {
		data, err := value.MarshalBSONDocument("amount")
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte("bad"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		var decoded Decimal
		if err := decoded.UnmarshalBSONDocument(data, "amount"); err != nil {
			return
		}
		encoded, err := decoded.MarshalBSONDocument("amount")
		if err != nil {
			t.Fatalf("valid decoded BSON decimal did not marshal: %v", err)
		}
		var roundTrip Decimal
		if err := roundTrip.UnmarshalBSONDocument(encoded, "amount"); err != nil {
			t.Fatalf("re-encoded BSON decimal did not decode: %v", err)
		}
		if !roundTrip.Equal(decoded) || roundTrip.Scale() != decoded.Scale() {
			t.Fatalf("BSON round trip mismatch: %s/%d -> %s/%d", decoded, decoded.Scale(), roundTrip, roundTrip.Scale())
		}
	})
}

func mustFuzzFixed64(units int64, scale int32) Fixed64 {
	out, err := NewFixed64(units, scale)
	if err != nil {
		panic(err)
	}
	return out
}
