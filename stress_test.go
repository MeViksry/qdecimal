package qdecimal

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
)

func TestStressFixed64MatchesDecimal(t *testing.T) {
	iterations := stressIterations(2_000, 25_000)
	rng := rand.New(rand.NewSource(0x51cedec1))
	modes := []RoundingMode{
		ToNearestEven,
		ToNearestAway,
		ToNearestTowardZero,
		AwayFromZero,
		TowardZero,
		TowardPositive,
		TowardNegative,
	}

	for i := 0; i < iterations; i++ {
		a := stressFixed64(t, rng, 1_000_000)
		b := stressFixed64(t, rng, 1_000_000)

		sum, err := a.Add(b)
		if err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
		if !sum.Decimal().Equal(a.Decimal().Add(b.Decimal())) {
			t.Fatalf("add %d got %s want %s", i, sum, a.Decimal().Add(b.Decimal()))
		}

		diff, err := a.Sub(b)
		if err != nil {
			t.Fatalf("sub %d: %v", i, err)
		}
		if !diff.Decimal().Equal(a.Decimal().Sub(b.Decimal())) {
			t.Fatalf("sub %d got %s want %s", i, diff, a.Decimal().Sub(b.Decimal()))
		}

		scale := int32(rng.Intn(7))
		mode := modes[rng.Intn(len(modes))]
		rounded, err := a.Rescale(scale, mode)
		if err != nil {
			t.Fatalf("rescale %d: %v", i, err)
		}
		wantRounded, err := a.Decimal().Rescale(scale, mode)
		if err != nil {
			t.Fatalf("decimal rescale %d: %v", i, err)
		}
		if !rounded.Decimal().Equal(wantRounded) || rounded.Scale() != wantRounded.Scale() {
			t.Fatalf("rescale %d got %s scale=%d want %s scale=%d", i, rounded, rounded.Scale(), wantRounded, wantRounded.Scale())
		}

		productScale := int32(rng.Intn(7))
		product, err := a.Mul(b, productScale, mode)
		if err != nil {
			t.Fatalf("mul %d: %v", i, err)
		}
		wantProduct, err := a.Decimal().Mul(b.Decimal()).Rescale(productScale, mode)
		if err != nil {
			t.Fatalf("decimal mul %d: %v", i, err)
		}
		if !product.Decimal().Equal(wantProduct) || product.Scale() != wantProduct.Scale() {
			t.Fatalf("mul %d got %s scale=%d want %s scale=%d", i, product, product.Scale(), wantProduct, wantProduct.Scale())
		}

		step := stressFixed64(t, rng, 1_000)
		if step.IsZero() {
			step = mustFixed64(t, 1, int32(rng.Intn(7)))
		}
		ticked, err := a.QuantizeStep(step, mode)
		if err != nil {
			t.Fatalf("fixed64 quantize step %d: %v", i, err)
		}
		wantTicked, err := a.Decimal().QuantizeStep(step.Decimal(), mode)
		if err != nil {
			t.Fatalf("decimal quantize step %d: %v", i, err)
		}
		if !ticked.Decimal().Equal(wantTicked) || ticked.Scale() != step.Scale() {
			t.Fatalf("fixed64 quantize step %d got %s scale=%d want %s scale=%d step=%s", i, ticked, ticked.Scale(), wantTicked, wantTicked.Scale(), step)
		}

		candidate := stressFixed64(t, rng, 1_000_000)
		low, high := a, b
		if low.Cmp(high) > 0 {
			low, high = high, low
		}
		inside := candidate.Between(low, high, true)
		wantInside := candidate.Decimal().Between(low.Decimal(), high.Decimal(), true)
		if inside != wantInside {
			t.Fatalf("between %d got %t want %t candidate=%s low=%s high=%s", i, inside, wantInside, candidate, low, high)
		}

		clamped := candidate.Clamp(low, high)
		wantClamp := candidate.Decimal().Clamp(low.Decimal(), high.Decimal())
		if !clamped.Decimal().Equal(wantClamp) || clamped.Scale() != wantClamp.Scale() {
			t.Fatalf("clamp %d got %s scale=%d want %s scale=%d", i, clamped, clamped.Scale(), wantClamp, wantClamp.Scale())
		}

		min := MinFixed64(a, b, candidate)
		wantMin := Min(a.Decimal(), b.Decimal(), candidate.Decimal())
		if !min.Decimal().Equal(wantMin) || min.Scale() != wantMin.Scale() {
			t.Fatalf("min %d got %s scale=%d want %s scale=%d", i, min, min.Scale(), wantMin, wantMin.Scale())
		}
		max := MaxFixed64(a, b, candidate)
		wantMax := Max(a.Decimal(), b.Decimal(), candidate.Decimal())
		if !max.Decimal().Equal(wantMax) || max.Scale() != wantMax.Scale() {
			t.Fatalf("max %d got %s scale=%d want %s scale=%d", i, max, max.Scale(), wantMax, wantMax.Scale())
		}

		values := []Fixed64{a, b, candidate}
		fixedSum, err := SumFixed64(values...)
		if err != nil {
			t.Fatalf("sum fixed64 %d: %v", i, err)
		}
		wantSum := Sum(a.Decimal(), b.Decimal(), candidate.Decimal())
		if !fixedSum.Decimal().Equal(wantSum) || fixedSum.Scale() != wantSum.Scale() {
			t.Fatalf("sum fixed64 %d got %s scale=%d want %s scale=%d", i, fixedSum, fixedSum.Scale(), wantSum, wantSum.Scale())
		}

		avgScale := int32(rng.Intn(7))
		avgMode := modes[rng.Intn(len(modes))]
		avg, err := AvgFixed64(values, avgScale, avgMode)
		if err != nil {
			t.Fatalf("avg fixed64 %d: %v", i, err)
		}
		wantAvg, err := wantSum.Div(NewFromInt(int64(len(values))), avgScale, avgMode)
		if err != nil {
			t.Fatalf("decimal avg fixed64 %d: %v", i, err)
		}
		if !avg.Decimal().Equal(wantAvg) || avg.Scale() != avgScale {
			t.Fatalf("avg fixed64 %d got %s scale=%d want %s scale=%d", i, avg, avg.Scale(), wantAvg, wantAvg.Scale())
		}
	}
}

func TestStressMoneyAllocationInvariants(t *testing.T) {
	iterations := stressIterations(1_000, 12_000)
	rng := rand.New(rand.NewSource(0xa110ca7e))
	currencies := []string{"USD", "IDR", "BTC", "USDT", "EUR"}
	modes := []RoundingMode{ToNearestEven, ToNearestAway, TowardZero}

	for i := 0; i < iterations; i++ {
		scale := int32(rng.Intn(6))
		units := rng.Int63n(2_000_000_000) - 1_000_000_000
		if rng.Intn(10) == 0 {
			units = -units
		}
		ctx := MustMoneyContext(currencies[rng.Intn(len(currencies))], scale, modes[rng.Intn(len(modes))])
		amount, err := ctx.FromMinorUnits(units)
		if err != nil {
			t.Fatalf("money %d: %v", i, err)
		}

		ratioLen := 1 + rng.Intn(8)
		ratios := make([]int64, ratioLen)
		for j := range ratios {
			ratios[j] = int64(rng.Intn(9))
		}
		if allZeroRatios(ratios) {
			ratios[rng.Intn(len(ratios))] = 1
		}

		parts, err := ctx.AllocateRatios(amount, ratios)
		if err != nil {
			t.Fatalf("allocate %d ratios=%v: %v", i, ratios, err)
		}
		if len(parts) != len(ratios) {
			t.Fatalf("allocate %d got %d parts want %d", i, len(parts), len(ratios))
		}

		total := parts[0]
		for j, part := range parts {
			if part.Currency() != ctx.Currency {
				t.Fatalf("allocate %d part %d currency got %s want %s", i, j, part.Currency(), ctx.Currency)
			}
			if j == 0 {
				continue
			}
			total, err = total.Add(part)
			if err != nil {
				t.Fatalf("allocate %d add part %d: %v", i, j, err)
			}
		}
		want, err := ctx.Quantize(amount)
		if err != nil {
			t.Fatalf("allocate %d quantize: %v", i, err)
		}
		if !total.Equal(want) {
			t.Fatalf("allocate %d total got %s want %s ratios=%v", i, total, want, ratios)
		}
	}
}

func TestStressMoneyAggregatesMatchDecimal(t *testing.T) {
	iterations := stressIterations(1_000, 15_000)
	rng := rand.New(rand.NewSource(0x6a66e6a7))
	currencies := []string{"USD", "IDR", "BTC", "USDT", "EUR"}
	modes := []RoundingMode{ToNearestEven, ToNearestAway, TowardZero, TowardNegative}

	for i := 0; i < iterations; i++ {
		scale := int32(rng.Intn(6))
		mode := modes[rng.Intn(len(modes))]
		ctx := MustMoneyContext(currencies[rng.Intn(len(currencies))], scale, mode)
		count := 1 + rng.Intn(16)
		values := make([]Money, count)
		total := Zero

		for j := range values {
			amountScale := int32(rng.Intn(8))
			amount, err := New(rng.Int63n(2_000_001)-1_000_000, amountScale)
			if err != nil {
				t.Fatalf("aggregate %d.%d decimal: %v", i, j, err)
			}
			money, err := NewMoney(amount, ctx.Currency)
			if err != nil {
				t.Fatalf("aggregate %d.%d money: %v", i, j, err)
			}
			values[j] = money
			total = total.Add(amount)
		}

		sum, err := SumMoney(values...)
		if err != nil {
			t.Fatalf("aggregate %d SumMoney: %v", i, err)
		}
		if sum.Currency() != ctx.Currency || !sum.amount.Equal(total) || sum.amount.Scale() != total.Scale() {
			t.Fatalf("aggregate %d sum got %s scale=%d want %s scale=%d", i, sum, sum.amount.Scale(), total, total.Scale())
		}

		avg, err := AvgMoney(values, scale, mode)
		if err != nil {
			t.Fatalf("aggregate %d AvgMoney: %v", i, err)
		}
		wantAvg, err := total.Div(NewFromInt(int64(len(values))), scale, mode)
		if err != nil {
			t.Fatalf("aggregate %d decimal avg: %v", i, err)
		}
		if avg.Currency() != ctx.Currency || !avg.amount.Equal(wantAvg) || avg.amount.Scale() != scale {
			t.Fatalf("aggregate %d avg got %s want %s", i, avg, wantAvg)
		}

		ctxSum, err := ctx.Sum(values...)
		if err != nil {
			t.Fatalf("aggregate %d context sum: %v", i, err)
		}
		wantSum, err := total.Rescale(scale, mode)
		if err != nil {
			t.Fatalf("aggregate %d decimal context sum: %v", i, err)
		}
		if !ctxSum.amount.Equal(wantSum) || ctxSum.amount.Scale() != scale {
			t.Fatalf("aggregate %d context sum got %s want %s", i, ctxSum, wantSum)
		}

		ctxAvg, err := ctx.Avg(values...)
		if err != nil {
			t.Fatalf("aggregate %d context avg: %v", i, err)
		}
		if !ctxAvg.Equal(avg) {
			t.Fatalf("aggregate %d context avg got %s want %s", i, ctxAvg, avg)
		}

		min, err := ctx.Min(values...)
		if err != nil {
			t.Fatalf("aggregate %d context min: %v", i, err)
		}
		max, err := ctx.Max(values...)
		if err != nil {
			t.Fatalf("aggregate %d context max: %v", i, err)
		}
		for j, value := range values {
			inside, err := ctx.Between(value, min, max, true)
			if err != nil {
				t.Fatalf("aggregate %d.%d context between: %v", i, j, err)
			}
			if !inside {
				t.Fatalf("aggregate %d.%d value %s outside [%s, %s]", i, j, value, min, max)
			}
		}
	}
}

func TestStressConcurrentHotPaths(t *testing.T) {
	iterations := stressIterations(600, 8_000)
	workers := 8
	if stressEnabled() {
		workers = 24
	}

	ctx := MustMoneyContext("USD", 2, ToNearestEven)
	errs := make(chan error, workers)
	var wg sync.WaitGroup

	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(0x5a17 + worker)))
			for i := 0; i < iterations; i++ {
				text := fmt.Sprintf("%d.%03d", rng.Intn(1_000_000), rng.Intn(1_000))
				fixed, err := ParseFixed64(text, 2, ToNearestEven)
				if err != nil {
					errs <- fmt.Errorf("worker %d parse fixed %q: %w", worker, text, err)
					return
				}
				feeRate, _ := NewFixed64(25, 4)
				fee, err := fixed.Mul(feeRate, 2, ToNearestEven)
				if err != nil {
					errs <- fmt.Errorf("worker %d fixed mul: %w", worker, err)
					return
				}
				if fee.Scale() != 2 {
					errs <- fmt.Errorf("worker %d fixed fee scale=%d", worker, fee.Scale())
					return
				}

				dec := MustParse(text)
				third, err := dec.Div(MustParse("3"), 6, ToNearestEven)
				if err != nil {
					errs <- fmt.Errorf("worker %d decimal div: %w", worker, err)
					return
				}
				if third.Scale() != 6 {
					errs <- fmt.Errorf("worker %d decimal div scale=%d", worker, third.Scale())
					return
				}

				money, err := ctx.Parse(text)
				if err != nil {
					errs <- fmt.Errorf("worker %d money parse: %w", worker, err)
					return
				}
				adjustment, err := ctx.FromMinorUnits(int64(rng.Intn(100)))
				if err != nil {
					errs <- fmt.Errorf("worker %d money adjustment: %w", worker, err)
					return
				}
				total, err := ctx.Add(money, adjustment)
				if err != nil {
					errs <- fmt.Errorf("worker %d money add: %w", worker, err)
					return
				}
				if total.Currency() != "USD" || total.Amount().Scale() != 2 {
					errs <- fmt.Errorf("worker %d money total got %s", worker, total)
					return
				}
			}
		}(worker)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestStressParseStringRoundTrip(t *testing.T) {
	iterations := stressIterations(1_000, 20_000)
	rng := rand.New(rand.NewSource(0xdecafbad))

	for i := 0; i < iterations; i++ {
		scale := rng.Intn(10)
		intPart := rng.Int63n(1_000_000_000)
		negative := rng.Intn(2) == 0
		frac := stressRandPow10(rng, scale)
		text := stressDecimalText(negative, intPart, frac, scale)

		parsed, err := Parse(text)
		if err != nil {
			t.Fatalf("parse %d %q: %v", i, text, err)
		}
		roundTrip, err := Parse(parsed.String())
		if err != nil {
			t.Fatalf("roundtrip parse %d %q: %v", i, parsed.String(), err)
		}
		if !parsed.Equal(roundTrip) || parsed.Scale() != roundTrip.Scale() {
			t.Fatalf("roundtrip %d got %s scale=%d want %s scale=%d", i, roundTrip, roundTrip.Scale(), parsed, parsed.Scale())
		}
	}
}

func stressIterations(defaultCount, stressCount int) int {
	if stressEnabled() {
		return stressCount
	}
	return defaultCount
}

func stressEnabled() bool {
	return os.Getenv("QDECIMAL_STRESS") == "1"
}

func stressFixed64(t *testing.T, rng *rand.Rand, maxMagnitude int64) Fixed64 {
	t.Helper()
	units := rng.Int63n(maxMagnitude*2+1) - maxMagnitude
	scale := int32(rng.Intn(7))
	out, err := NewFixed64(units, scale)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func allZeroRatios(ratios []int64) bool {
	for _, ratio := range ratios {
		if ratio != 0 {
			return false
		}
	}
	return true
}

func stressDecimalText(negative bool, intPart, frac int64, scale int) string {
	if scale == 0 {
		if negative && intPart != 0 {
			return fmt.Sprintf("-%d", intPart)
		}
		return fmt.Sprintf("%d", intPart)
	}
	sign := ""
	if negative && (intPart != 0 || frac != 0) {
		sign = "-"
	}
	return fmt.Sprintf("%s%d.%0*d", sign, intPart, scale, frac)
}

func stressRandPow10(rng *rand.Rand, scale int) int64 {
	limit := int64(1)
	for i := 0; i < scale; i++ {
		limit *= 10
	}
	return rng.Int63n(limit)
}
