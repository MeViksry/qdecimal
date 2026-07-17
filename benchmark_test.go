package qdecimal

import "testing"

var (
	benchmarkBytesSink      []byte
	benchmarkDecimalSink    Decimal
	benchmarkFixed64Sink    Fixed64
	benchmarkInt64Sink      int64
	benchmarkMoneySink      Money
	benchmarkMoneySliceSink []Money
	benchmarkStringSink     string
)

func BenchmarkParse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		got, err := Parse("123456789.987654321")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkParseBytes(b *testing.B) {
	text := []byte("123456789.987654321")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := ParseBytes(text)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkParseBigFallback(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		got, err := Parse("123456789123456789123456789.987654321")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkAdd(b *testing.B) {
	a := MustParse("123456789.987654321")
	c := MustParse("0.000000009")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkDecimalSink = a.Add(c)
	}
}

func BenchmarkMul(b *testing.B) {
	a := MustParse("123456789.987654321")
	c := MustParse("987654321.123456789")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkDecimalSink = a.Mul(c)
	}
}

func BenchmarkDiv(b *testing.B) {
	a := MustParse("123456789.987654321")
	c := MustParse("7")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.Div(c, 18, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkDivExact(b *testing.B) {
	a := MustParse("123456789.987654321")
	c := MustParse("8")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.DivExact(c)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkContextMul(b *testing.B) {
	ctx := MustContext(2, ToNearestEven)
	a := MustParse("123456789.987654321")
	c := MustParse("987654321.123456789")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := ctx.Mul(a, c)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkContextQuantizeStep(b *testing.B) {
	ctx := MustContext(2, ToNearestEven)
	a := MustParse("123456789.987654321")
	step := MustParse("0.05")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := ctx.QuantizeStep(a, step)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkSum(b *testing.B) {
	values := []Decimal{
		MustParse("123456789.987654321"),
		MustParse("0.000000009"),
		MustParse("42.42"),
		MustParse("-7.0001"),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkDecimalSink = Sum(values...)
	}
}

func BenchmarkAvgExact(b *testing.B) {
	values := []Decimal{
		MustParse("123456789.98"),
		MustParse("0.06"),
		MustParse("42.42"),
		MustParse("-7.00"),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := AvgExact(values)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkSumMoney(b *testing.B) {
	values := []Money{
		MustParseMoney("USD 123456789.98"),
		MustParseMoney("USD 0.09"),
		MustParseMoney("USD 42.42"),
		MustParseMoney("USD -7.0001"),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := SumMoney(values...)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMoneySink = got
	}
}

func BenchmarkMinorUnits(b *testing.B) {
	d := MustParse("123456789.987654321")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := d.Int64MinorUnits(2, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkInt64Sink = got
	}
}

func BenchmarkFixed64Clamp(b *testing.B) {
	value, _ := NewFixed64(123456, 3)
	low, _ := NewFixed64(10000, 2)
	high, _ := NewFixed64(15000, 2)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkFixed64Sink = value.Clamp(low, high)
	}
}

func BenchmarkSumFixed64(b *testing.B) {
	values := []Fixed64{
		mustBenchmarkFixed64(12345678998, 2),
		mustBenchmarkFixed64(9, 2),
		mustBenchmarkFixed64(4242, 2),
		mustBenchmarkFixed64(-700, 2),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := SumFixed64(values...)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFixed64Sink = got
	}
}

func BenchmarkAvgFixed64(b *testing.B) {
	values := []Fixed64{
		mustBenchmarkFixed64(12345678998, 2),
		mustBenchmarkFixed64(9, 2),
		mustBenchmarkFixed64(4242, 2),
		mustBenchmarkFixed64(-700, 2),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := AvgFixed64(values, 2, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFixed64Sink = got
	}
}

func BenchmarkFixed64Parse(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		got, err := ParseFixed64("123456789.987654321", 8, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFixed64Sink = got
	}
}

func BenchmarkFixed64Add(b *testing.B) {
	a, _ := NewFixed64(12345678998, 2)
	c, _ := NewFixed64(9, 3)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.Add(c)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFixed64Sink = got
	}
}

func BenchmarkFixed64Rescale(b *testing.B) {
	a, _ := NewFixed64(123456789987654321, 9)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.Rescale(2, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFixed64Sink = got
	}
}

func BenchmarkFixed64Mul(b *testing.B) {
	a, _ := NewFixed64(123456789, 4)
	c, _ := NewFixed64(987654321, 6)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.Mul(c, 6, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFixed64Sink = got
	}
}

func BenchmarkFixed64QuantizeStep(b *testing.B) {
	a, _ := NewFixed64(123456789, 4)
	step, _ := NewFixed64(5, 4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.QuantizeStep(step, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkFixed64Sink = got
	}
}

func BenchmarkFixed64String(b *testing.B) {
	a, _ := NewFixed64(123456789987654321, 9)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkStringSink = a.String()
	}
}

func BenchmarkFixed64AppendText(b *testing.B) {
	a, _ := NewFixed64(123456789987654321, 9)
	buf := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.AppendText(buf[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkFixed64MarshalBinary(b *testing.B) {
	a, _ := NewFixed64(123456789987654321, 9)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.MarshalBinary()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkFixed64AppendBinary(b *testing.B) {
	a, _ := NewFixed64(123456789987654321, 9)
	buf := make([]byte, 0, a.BinarySize())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.AppendBinary(buf[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkMoneyAdd(b *testing.B) {
	a, _ := NewMoney(MustParse("123456789.98"), "USD")
	c, _ := NewMoney(MustParse("0.09"), "USD")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.Add(c)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMoneySink = got
	}
}

func BenchmarkMoneyMarshalBinary(b *testing.B) {
	a, _ := NewMoney(MustParse("123456789.98"), "USD")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.MarshalBinary()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkMoneyAppendBinary(b *testing.B) {
	a, _ := NewMoney(MustParse("123456789.98"), "USD")
	buf := make([]byte, 0, a.BinarySize())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.AppendBinary(buf[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkMoneyAppendText(b *testing.B) {
	a, _ := NewMoney(MustParse("123456789.98"), "USD")
	buf := make([]byte, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.AppendText(buf[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkMoneyContextAdd(b *testing.B) {
	ctx := MustMoneyContext("USD", 2, ToNearestEven)
	a, _ := NewMoney(MustParse("123456789.98"), "USD")
	c, _ := NewMoney(MustParse("0.09"), "USD")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := ctx.Add(a, c)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMoneySink = got
	}
}

func BenchmarkMoneyContextAvg(b *testing.B) {
	ctx := MustMoneyContext("USD", 2, ToNearestEven)
	values := []Money{
		MustParseMoney("USD 123456789.98"),
		MustParseMoney("USD 0.09"),
		MustParseMoney("USD 42.42"),
		MustParseMoney("USD -7.0001"),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := ctx.Avg(values...)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMoneySink = got
	}
}

func BenchmarkMoneyContextQuantizeStepExact(b *testing.B) {
	ctx := MustMoneyContext("USD", 2, ToNearestEven)
	a := MustParseMoney("USD 123456789.95")
	step := MustParse("0.05")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := ctx.QuantizeStepExact(a, step)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMoneySink = got
	}
}

func BenchmarkMoneyContextParse(b *testing.B) {
	ctx := MustMoneyContext("USD", 2, ToNearestEven)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		got, err := ctx.Parse("123456789.987")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMoneySink = got
	}
}

func BenchmarkMoneyAllocate(b *testing.B) {
	a, _ := NewMoney(MustParse("123456789.98"), "USD")
	ratios := []int64{3, 2, 1}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := a.AllocateRatios(ratios, 2, ToNearestEven)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkMoneySliceSink = got
	}
}

func BenchmarkMarshalBinary(b *testing.B) {
	d := MustParse("123456789.987654321")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := d.MarshalBinary()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkAppendBinary(b *testing.B) {
	d := MustParse("123456789.987654321")
	buf := make([]byte, 0, d.BinarySize())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := d.AppendBinary(buf[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func mustBenchmarkFixed64(units int64, scale int32) Fixed64 {
	out, err := NewFixed64(units, scale)
	if err != nil {
		panic(err)
	}
	return out
}

func BenchmarkAppendBinaryNegative(b *testing.B) {
	d := MustParse("-123456789.987654321")
	buf := make([]byte, 0, d.BinarySize())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := d.AppendBinary(buf[:0])
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkMarshalBSONStringValue(b *testing.B) {
	d := MustParse("123456789.987654321")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := d.MarshalBSONStringValue()
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkMarshalBSONDocument(b *testing.B) {
	d := MustParse("123456789.987654321")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := d.MarshalBSONDocument("amount")
		if err != nil {
			b.Fatal(err)
		}
		benchmarkBytesSink = got
	}
}

func BenchmarkPowInt(b *testing.B) {
	d := MustParse("1.0001")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := d.PowInt(8)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkDecimalSink = got
	}
}

func BenchmarkString(b *testing.B) {
	d := MustParse("123456789.987654321")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkStringSink = d.String()
	}
}
