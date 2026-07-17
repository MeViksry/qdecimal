#!/usr/bin/env sh
set -eu

work_dir="${1:-${TMPDIR:-/tmp}/qdecimal-consumer-smoke-$$}"

case "$work_dir" in
	/*) ;;
	*)
		echo "qdecimal: consumer smoke dir must be absolute: $work_dir" >&2
		exit 2
		;;
esac

work_base=$(basename -- "$work_dir")
case "$work_base" in
	qdecimal-consumer-smoke-*) ;;
	*)
		echo "qdecimal: refusing unsafe consumer smoke dir: $work_dir" >&2
		exit 2
		;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
module_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)

case "$work_dir" in
	"$module_dir"|"$module_dir"/*)
		echo "qdecimal: refusing to create consumer smoke project inside module source: $work_dir" >&2
		exit 2
		;;
esac

rm -rf "$work_dir"
mkdir -p "$work_dir"
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

cat > "$work_dir/go.mod" <<EOF
module qdecimal.consumer.smoke

go 1.23.0

require github.com/MeViksry/qdecimal v0.0.0

replace github.com/MeViksry/qdecimal => $module_dir
EOF

cat > "$work_dir/qdecimal_consumer_test.go" <<'EOF'
package qdecimal_consumer_smoke

import (
	"encoding/json"
	"testing"

	"github.com/MeViksry/qdecimal"
)

func TestConsumerFinanceFlow(t *testing.T) {
	price := qdecimal.MustParse("123.4500")
	size := qdecimal.MustParse("0.25")
	notional := price.Mul(size)
	rounded, err := notional.Round(2, qdecimal.ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if rounded.String() != "30.86" {
		t.Fatalf("rounded notional got %s", rounded)
	}

	usd := qdecimal.MustMoneyContext("usd", 2, qdecimal.ToNearestAway)
	amount, err := usd.Parse("10.005")
	if err != nil {
		t.Fatal(err)
	}
	if amount.String() != "USD 10.01" {
		t.Fatalf("money context got %s", amount)
	}

	fixed, err := qdecimal.ParseFixed64("123.456", 2, qdecimal.ToNearestAway)
	if err != nil {
		t.Fatal(err)
	}
	if fixed.String() != "123.46" {
		t.Fatalf("fixed64 got %s", fixed)
	}

	pow, err := qdecimal.MustParse("2").Pow(qdecimal.MustParse("-3"), 4, qdecimal.ToNearestEven)
	if err != nil {
		t.Fatal(err)
	}
	if pow.String() != "0.1250" {
		t.Fatalf("pow got %s", pow)
	}

	data, err := json.Marshal(qdecimal.MustParse("123.4500"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"123.4500"` {
		t.Fatalf("json got %s", data)
	}
}
EOF

(
	cd "$work_dir"
	GOCACHE="${GOCACHE:-/tmp/qdecimal-consumer-gocache}" go test ./...
)

printf '%s\n' "qdecimal consumer smoke ok"
