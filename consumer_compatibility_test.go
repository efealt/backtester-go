package backtester_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestSeparateConsumerModuleUsesPublicAPI(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	repositoryRoot, err := filepath.Abs(filepath.Dir(currentFile))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	consumerRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve consumer root: %v", err)
	}
	goMod := `module example.com/backtester-consumer

go 1.22.0
`
	goWork := fmt.Sprintf(`go 1.22.0

use (
	%s
	%s
)
`, strconv.Quote(filepath.ToSlash(consumerRoot)), strconv.Quote(filepath.ToSlash(repositoryRoot)))
	consumerTest := `package consumer_test

import (
	"testing"
	"time"

	backtester "github.com/efealt/backtester-go"
)

func TestDirectRun(t *testing.T) {
	first := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	bars := []backtester.MarketBar{
		{Timestamp: first, Close: 100},
		{Timestamp: first.AddDate(0, 0, 1), Close: 110},
		{Timestamp: first.AddDate(0, 0, 2), Close: 121},
	}
	targets := []backtester.Target{
		{Timestamp: bars[0].Timestamp, Exposure: 1},
		{Timestamp: bars[1].Timestamp, Exposure: 1},
		{Timestamp: bars[2].Timestamp, Exposure: 0},
	}
	config := backtester.DefaultConfig()
	config.ExecutionTiming = backtester.ExecutionAtClose

	result, err := backtester.Run(bars, targets, config)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Path) != 3 || len(result.Trades) != 1 || len(result.ExitEvents) != 0 || result.Metrics.TradeCount != 1 || result.Ruined {
		t.Fatalf("unexpected complete result: %+v", result)
	}
}
`
	writeConsumerFile(t, filepath.Join(consumerRoot, "go.mod"), goMod)
	writeConsumerFile(t, filepath.Join(consumerRoot, "go.work"), goWork)
	writeConsumerFile(t, filepath.Join(consumerRoot, "consumer_test.go"), consumerTest)

	command := exec.Command("go", "test", "./...")
	command.Dir = consumerRoot
	command.Env = append(os.Environ(), "GOWORK="+filepath.Join(consumerRoot, "go.work"))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("separate consumer module failed: %v\n%s", err, output)
	}
}

func writeConsumerFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write consumer file: %v", err)
	}
}
