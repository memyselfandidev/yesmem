package capfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProxyHealth(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	path := filepath.Join(home, ".claude", "caps", "proxy_health", "CAP.md")
	d, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("local cap not installed at %s: %v", path, err)
	}
	cf, err := Parse(d)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	t.Logf("OK scripts=%d", len(cf.Scripts))
}
