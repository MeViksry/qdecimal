package qdecimal_test

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/MeViksry/qdecimal"
)

func ExampleDecimal() {
	price := qdecimal.MustParse("123.4500")
	size := qdecimal.MustParse("0.25")

	notional := price.Mul(size)
	rounded, err := notional.Round(2, qdecimal.ToNearestEven)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(rounded)
	// Output: 30.86
}

func ExampleContext() {
	usdCents := qdecimal.MustContext(2, qdecimal.ToNearestEven)

	fee, err := usdCents.Mul(
		qdecimal.MustParse("123.4567"),
		qdecimal.MustParse("0.0025"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(fee)
	// Output: 0.31
}

func ExampleFixed64() {
	price, err := qdecimal.ParseFixed64("123.456", 2, qdecimal.ToNearestAway)
	if err != nil {
		log.Fatal(err)
	}
	rate, err := qdecimal.NewFixed64(25, 4)
	if err != nil {
		log.Fatal(err)
	}
	fee, err := price.Mul(rate, 4, qdecimal.ToNearestEven)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(fee)
	// Output: 0.3086
}

func ExampleMoneyContext() {
	usd := qdecimal.MustMoneyContext("usd", 2, qdecimal.ToNearestAway)

	amount, err := usd.Parse("10.005")
	if err != nil {
		log.Fatal(err)
	}
	rebate, err := usd.Parse("0.005")
	if err != nil {
		log.Fatal(err)
	}
	total, err := usd.Add(amount, rebate)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(total)
	// Output: USD 10.02
}

func ExampleDecimal_MarshalJSON() {
	data, err := json.Marshal(qdecimal.MustParse("123.4500"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
	// Output: "123.4500"
}
