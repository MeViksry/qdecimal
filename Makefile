COVERPROFILE ?= /tmp/qdecimal-coverage.out
COVERAGE_MIN ?= 85.0
RELEASE_VERSION ?= dry-run
RELEASE_DIST_DIR ?= /tmp/qdecimal-release-$(RELEASE_VERSION)
CONSUMER_SMOKE_DIR ?= /tmp/qdecimal-consumer-smoke-default
CROSS_BUILD_DIR ?= /tmp/qdecimal-cross-build
CROSS_TARGETS ?= linux/amd64 linux/arm64 linux/arm linux/386 windows/amd64 windows/arm64 windows/386 darwin/amd64 darwin/arm64 freebsd/amd64 freebsd/arm64 openbsd/amd64 netbsd/amd64

.PHONY: fmt fmt-check deps test race vet vuln stress fuzz-smoke fuzz bench-smoke bench build cross-build coverage consumer-smoke release-dry-run release-helper-check check audit

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

deps:
	@mods="$$(go list -m all)"; \
	count="$$(printf '%s\n' "$$mods" | awk 'NF { count++ } END { print count + 0 }')"; \
	if [ "$$count" -ne 1 ]; then \
		printf '%s\n' "$$mods"; \
		printf '%s\n' "qdecimal: expected no external Go modules, found $$((count - 1))" >&2; \
		exit 1; \
	fi; \
	printf '%s\n' "qdecimal: dependency policy ok (no external Go modules)"

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

vuln:
	if command -v govulncheck >/dev/null 2>&1; then govulncheck ./...; else go run golang.org/x/vuln/cmd/govulncheck@latest ./...; fi

stress:
	QDECIMAL_STRESS=1 go test -run '^TestStress' ./...

fuzz-smoke:
	go test -run '^$$' -fuzz='^FuzzParse$$' -fuzztime=2s .
	go test -run '^$$' -fuzz='^FuzzDecimalBinary$$' -fuzztime=2s .
	go test -run '^$$' -fuzz='^FuzzFixed64Binary$$' -fuzztime=2s .
	go test -run '^$$' -fuzz='^FuzzMoneyBinary$$' -fuzztime=2s .
	go test -run '^$$' -fuzz='^FuzzDecimalBSONDocument$$' -fuzztime=2s .

fuzz:
	go test -fuzz='^FuzzParse$$' -fuzztime=10s .
	go test -fuzz='^FuzzDecimalBinary$$' -fuzztime=10s .
	go test -fuzz='^FuzzFixed64Binary$$' -fuzztime=10s .
	go test -fuzz='^FuzzMoneyBinary$$' -fuzztime=10s .
	go test -fuzz='^FuzzDecimalBSONDocument$$' -fuzztime=10s .

bench-smoke:
	go test -run '^$$' -bench 'Benchmark(Parse|Add|AvgExact|Div|ContextQuantizeStep|Fixed64(Add|Mul|AppendText|Clamp|QuantizeStep)|SumFixed64|AvgFixed64|Money(Context)?Add|SumMoney|MoneyContextAvg|MoneyContextQuantizeStepExact|MoneyAppendText|PowInt|AppendBinary)$$' -benchtime=100ms -benchmem ./...

bench:
	go test -run '^$$' -bench . -benchmem ./...

build:
	go build ./...

cross-build:
	@set -eu; \
	rm -rf "$(CROSS_BUILD_DIR)"; \
	mkdir -p "$(CROSS_BUILD_DIR)"; \
	for target in $(CROSS_TARGETS); do \
		goos="$${target%/*}"; \
		goarch="$${target#*/}"; \
		printf '%s\n' "qdecimal: cross-build $${goos}/$${goarch}"; \
		GOOS="$${goos}" GOARCH="$${goarch}" CGO_ENABLED=0 go test -c -o "$(CROSS_BUILD_DIR)/qdecimal-$${goos}-$${goarch}.test" .; \
		GOOS="$${goos}" GOARCH="$${goarch}" CGO_ENABLED=0 go test -tags releasehelper -c -o "$(CROSS_BUILD_DIR)/qdecimal-releasehelper-$${goos}-$${goarch}.test" ./internal/releasegithub; \
	done; \
	printf '%s\n' "qdecimal: cross-build ok ($(CROSS_BUILD_DIR))"

coverage:
	go test -covermode=atomic -coverprofile="$(COVERPROFILE)" .
	go tool cover -func="$(COVERPROFILE)"
	@total="$$(go tool cover -func="$(COVERPROFILE)" | awk '/^total:/ { gsub("%", "", $$3); print $$3 }')"; \
	awk -v total="$$total" -v min="$(COVERAGE_MIN)" 'BEGIN { if ((total + 0) < (min + 0)) { printf "qdecimal: coverage %.1f%% below required %.1f%%\n", total, min; exit 1 } printf "qdecimal: coverage %.1f%% meets required %.1f%%\n", total, min }'

consumer-smoke:
	sh scripts/consumer-smoke.sh "$(CONSUMER_SMOKE_DIR)"

release-dry-run:
	sh scripts/release-archive.sh "$(RELEASE_VERSION)" "$(RELEASE_DIST_DIR)"

release-helper-check:
	go test -tags releasehelper ./internal/releasegithub

check: fmt-check deps test race vet build release-helper-check

audit: check cross-build coverage stress fuzz-smoke bench-smoke vuln consumer-smoke release-dry-run
