package capfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLiveWikiExport(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}
	path := filepath.Join(home, ".claude", "caps", "wiki_export", "CAP.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("local cap not installed at %s: %v", path, err)
	}
	cf, err := Parse(data)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	t.Logf("OK name=%q scope=%q tags=%v scripts=%d", cf.Name, cf.Scope, cf.Tags, len(cf.Scripts))
	for i, s := range cf.Scripts {
		t.Logf("  script[%d] name=%q kind=%q runtime=%q bodyLen=%d", i, s.Name, s.Kind, s.Runtime, len(s.Body))
	}
}
