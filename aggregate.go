package qdecimal

// Sum returns the exact sum of values. An empty input returns Zero.
func Sum(values ...Decimal) Decimal {
	if len(values) == 0 {
		return Zero
	}
	total := values[0]
	for _, value := range values[1:] {
		total = total.Add(value)
	}
	return total
}

// Avg returns the average of values rounded to scale using mode.
func Avg(values []Decimal, scale int32, mode RoundingMode) (Decimal, error) {
	if len(values) == 0 {
		return Decimal{}, ErrEmptyInput
	}
	total := Sum(values...)
	return total.Div(NewFromInt(int64(len(values))), scale, mode)
}

// AvgExact returns the exact finite average of values. If the quotient repeats,
// ErrInexact is returned instead of rounding.
func AvgExact(values []Decimal) (Decimal, error) {
	if len(values) == 0 {
		return Decimal{}, ErrEmptyInput
	}
	total := Sum(values...)
	return total.DivExact(NewFromInt(int64(len(values))))
}

// SumMoney returns the exact sum of money values. All values must use one
// currency because summing across currencies is a category error.
func SumMoney(values ...Money) (Money, error) {
	if len(values) == 0 {
		return Money{}, ErrEmptyInput
	}
	total := values[0]
	if err := total.validCurrency(); err != nil {
		return Money{}, err
	}
	for _, value := range values[1:] {
		if err := total.sameCurrency(value); err != nil {
			return Money{}, err
		}
		total.amount = total.amount.Add(value.amount)
	}
	return total.copy(), nil
}

// AvgMoney returns the average of money values rounded to scale using mode.
// All values must use one currency.
func AvgMoney(values []Money, scale int32, mode RoundingMode) (Money, error) {
	total, err := SumMoney(values...)
	if err != nil {
		return Money{}, err
	}
	amount, err := total.amount.Div(NewFromInt(int64(len(values))), scale, mode)
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: total.currency}, nil
}

// AvgMoneyExact returns the exact finite average of money values. If the
// quotient repeats, ErrInexact is returned instead of rounding.
func AvgMoneyExact(values []Money) (Money, error) {
	total, err := SumMoney(values...)
	if err != nil {
		return Money{}, err
	}
	amount, err := total.amount.DivExact(NewFromInt(int64(len(values))))
	if err != nil {
		return Money{}, err
	}
	return Money{amount: amount, currency: total.currency}, nil
}

// Sum rounds the exact sum of values to the context scale.
func (c Context) Sum(values ...Decimal) (Decimal, error) {
	return c.Quantize(Sum(values...))
}

// SumExact returns the exact sum at the context scale, failing with ErrInexact
// if non-zero digits would be discarded.
func (c Context) SumExact(values ...Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	if len(values) == 0 {
		return Decimal{}, ErrEmptyInput
	}
	return c.QuantizeExact(Sum(values...))
}

// Avg returns the average of values rounded to the context scale.
func (c Context) Avg(values ...Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	return Avg(values, c.Scale, c.Rounding)
}

// AvgExact returns the exact average at the context scale, failing with
// ErrInexact if the average repeats or does not fit the context scale.
func (c Context) AvgExact(values ...Decimal) (Decimal, error) {
	if err := c.validate(); err != nil {
		return Decimal{}, err
	}
	avg, err := AvgExact(values)
	if err != nil {
		return Decimal{}, err
	}
	return c.QuantizeExact(avg)
}
