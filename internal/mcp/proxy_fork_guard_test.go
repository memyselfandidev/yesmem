package mcp

import (
	"strings"
	"testing"
)

// TestStartDaemonRefusesUnderGoTest verifies the fork-bomb guard.
//
// Without the guard, startDaemon would call os.Executable() (which under
// `go test` resolves to the *.test binary) and exec it with "daemon
// --replace". The child *.test re-runs the entire suite, hitting this
// same code path, which forks again. cmd.Process.Release() removes any
// backpressure, so the only ceiling is the kernel's process table.
//
// We cannot write a true RED test for this because triggering the bomb
// once would require recovering the host. Instead we assert the guard
// is in place by calling startDaemon directly and confirming it returns
// an error that explicitly references the test context.
func TestStartDaemonRefusesUnderGoTest(t *testing.T) {
	err := startDaemon(t.TempDir())
	if err == nil {
		t.Fatal("startDaemon returned nil under go test — guard missing, fork bomb risk")
	}
	msg := err.Error()
	if !strings.Contains(msg, "go test") {
		t.Fatalf("guard error message should reference 'go test', got: %v", err)
	}
}
