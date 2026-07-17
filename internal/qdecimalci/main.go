package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const benchPattern = `Benchmark(Parse|Add|AvgExact|Div|ContextQuantizeStep|Fixed64(Add|Mul|AppendText|Clamp|QuantizeStep)|SumFixed64|AvgFixed64|Money(Context)?Add|SumMoney|MoneyContextAvg|MoneyContextQuantizeStepExact|MoneyAppendText|PowInt|AppendBinary)$`

var crossTargets = []string{
	"linux/amd64",
	"linux/arm64",
	"linux/arm",
	"linux/386",
	"windows/amd64",
	"windows/arm64",
	"windows/386",
	"darwin/amd64",
	"darwin/arm64",
	"freebsd/amd64",
	"freebsd/arm64",
	"openbsd/amd64",
	"netbsd/amd64",
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatalf("usage: qdecimalci <deps|race|coverage|stress|fuzz-smoke|bench-smoke|consumer-smoke|cross-build|vuln>")
	}

	var err error
	switch os.Args[1] {
	case "deps":
		err = deps()
	case "race":
		err = race()
	case "coverage":
		err = coverage()
	case "stress":
		err = run("go", "test", "-run", "^TestStress", "./...")
	case "fuzz-smoke":
		err = fuzzSmoke()
	case "bench-smoke":
		err = run("go", "test", "-run", "^$", "-bench", benchPattern, "-benchtime=100ms", "-benchmem", "./...")
	case "consumer-smoke":
		err = consumerSmoke()
	case "cross-build":
		err = crossBuild()
	case "vuln":
		err = vuln()
	default:
		err = fmt.Errorf("unknown qdecimalci command %q", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

func deps() error {
	out, err := combined("go", "list", "-m", "all")
	if err != nil {
		return err
	}
	lines := nonEmptyLines(out)
	if len(lines) != 1 {
		fmt.Print(out)
		return fmt.Errorf("qdecimal: expected no external Go modules, found %d", len(lines)-1)
	}
	fmt.Println("qdecimal: dependency policy ok (no external Go modules)")
	return nil
}

func race() error {
	cc, err := goEnv("CC")
	if err != nil {
		return err
	}
	compiler := firstField(cc)
	if compiler == "" {
		return errors.New("qdecimal: race detector requires a C compiler, but go env CC is empty")
	}
	if _, err := exec.LookPath(compiler); err != nil {
		message := fmt.Sprintf("qdecimal: race detector skipped because C compiler %q is not available; install gcc/clang on the self-hosted runner or set QDECIMAL_RACE_STRICT=1 to fail instead", compiler)
		if strictEnv("QDECIMAL_RACE_STRICT") {
			return errors.New(message)
		}
		githubWarning(message)
		return run("go", "test", "./...")
	}
	return runWithEnv([]string{"CGO_ENABLED=1"}, "go", "test", "-race", "./...")
}

func coverage() error {
	profile := getenv("COVERPROFILE", filepath.Join(os.TempDir(), "qdecimal-coverage.out"))
	if err := run("go", "test", "-covermode=atomic", "-coverprofile="+profile, "."); err != nil {
		return err
	}
	out, err := combined("go", "tool", "cover", "-func="+profile)
	if err != nil {
		return err
	}
	fmt.Print(out)
	total, err := parseCoverageTotal(out)
	if err != nil {
		return err
	}
	min, err := strconv.ParseFloat(getenv("COVERAGE_MIN", "85.0"), 64)
	if err != nil {
		return fmt.Errorf("invalid COVERAGE_MIN: %w", err)
	}
	if total < min {
		return fmt.Errorf("qdecimal: coverage %.1f%% below required %.1f%%", total, min)
	}
	fmt.Printf("qdecimal: coverage %.1f%% meets required %.1f%%\n", total, min)
	return nil
}

func fuzzSmoke() error {
	targets := []string{
		"^FuzzParse$",
		"^FuzzDecimalBinary$",
		"^FuzzFixed64Binary$",
		"^FuzzMoneyBinary$",
		"^FuzzDecimalBSONDocument$",
	}
	fuzzTime := getenv("QDECIMAL_FUZZTIME", "5s")
	fuzzParallel := getenv("QDECIMAL_FUZZ_PARALLEL", "1")
	for _, target := range targets {
		if err := run("go", "test", "-run", "^$", "-fuzz", target, "-fuzztime="+fuzzTime, "-parallel="+fuzzParallel, "."); err != nil {
			return err
		}
	}
	return nil
}

func consumerSmoke() error {
	moduleDir, err := os.Getwd()
	if err != nil {
		return err
	}
	moduleDir, err = filepath.Abs(moduleDir)
	if err != nil {
		return err
	}

	workDir := getenv("CONSUMER_SMOKE_DIR", filepath.Join(os.TempDir(), "qdecimal-consumer-smoke-default"))
	workDir, err = filepath.Abs(workDir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(filepath.Base(workDir), "qdecimal-consumer-smoke-") {
		return fmt.Errorf("qdecimal: refusing unsafe consumer smoke dir: %s", workDir)
	}
	rel, err := filepath.Rel(moduleDir, workDir)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("qdecimal: refusing to create consumer smoke project inside module source: %s", workDir)
	}
	if err := os.RemoveAll(workDir); err != nil {
		return err
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	goMod := fmt.Sprintf(`module qdecimal.consumer.smoke

go 1.23.0

require github.com/MeViksry/qdecimal v0.0.0

replace github.com/MeViksry/qdecimal => %s
`, filepath.ToSlash(moduleDir))
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(workDir, "qdecimal_consumer_test.go"), []byte(consumerSmokeTest), 0o644); err != nil {
		return err
	}

	if err := runInDir(workDir, "go", "test", "./..."); err != nil {
		return err
	}
	fmt.Println("qdecimal consumer smoke ok")
	return nil
}

func crossBuild() error {
	outDir := getenv("CROSS_BUILD_DIR", filepath.Join(tempRoot(), "qdecimal-cross-build"))
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := run("go", "clean", "-cache", "-testcache", "-fuzzcache"); err != nil {
		return err
	}
	targets := crossTargets
	if raw := strings.TrimSpace(os.Getenv("CROSS_TARGETS")); raw != "" {
		targets = strings.Fields(raw)
	}
	for _, target := range targets {
		if err := crossBuildTarget(outDir, target); err != nil {
			return err
		}
	}
	fmt.Printf("qdecimal: cross-build ok (%s)\n", outDir)
	return nil
}

func crossBuildTarget(outDir, target string) error {
	goos, goarch, ok := strings.Cut(target, "/")
	if !ok || goos == "" || goarch == "" {
		return fmt.Errorf("invalid cross target %q", target)
	}

	fmt.Printf("qdecimal: cross-build %s/%s\n", goos, goarch)
	targetDir := filepath.Join(outDir, strings.ReplaceAll(target, "/", "-"))
	if err := os.RemoveAll(targetDir); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(targetDir, "tmp"), 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(targetDir)

	env := []string{
		"GOOS=" + goos,
		"GOARCH=" + goarch,
		"CGO_ENABLED=0",
		"GOCACHE=" + filepath.Join(targetDir, "cache"),
		"GOTMPDIR=" + filepath.Join(targetDir, "tmp"),
	}
	if err := runWithEnv(env, "go", "test", "-vet=off", "-c", "-o", filepath.Join(targetDir, "qdecimal.test"), "."); err != nil {
		return err
	}
	if err := runWithEnv(env, "go", "test", "-vet=off", "-tags", "releasehelper", "-c", "-o", filepath.Join(targetDir, "qdecimal-releasehelper.test"), "./internal/releasegithub"); err != nil {
		return err
	}
	return nil
}

func vuln() error {
	if path, err := exec.LookPath("govulncheck"); err == nil {
		return run(path, "./...")
	}
	return run("go", "run", "golang.org/x/vuln/cmd/govulncheck@latest", "./...")
}

func run(name string, args ...string) error {
	return runInDir("", name, args...)
}

func runWithEnv(env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}

func runInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = dir
	return cmd.Run()
}

func combined(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		fmt.Print(out.String())
		return "", err
	}
	return out.String(), nil
}

func goEnv(key string) (string, error) {
	out, err := combined("go", "env", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func tempRoot() string {
	if value := os.Getenv("RUNNER_TEMP"); value != "" {
		return value
	}
	return os.TempDir()
}

func firstField(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func strictEnv(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func githubWarning(message string) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Printf("::warning::%s\n", message)
		return
	}
	fmt.Println(message)
}

func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseCoverageTotal(out string) (float64, error) {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "total:" {
			return strconv.ParseFloat(strings.TrimSuffix(fields[2], "%"), 64)
		}
	}
	return 0, errors.New("coverage total not found")
}

const consumerSmokeTest = `package qdecimal_consumer_smoke

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
	if string(data) != "\"123.4500\"" {
		t.Fatalf("json got %s", data)
	}
}
`
