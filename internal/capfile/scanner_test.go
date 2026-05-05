package capfile

import (
	"os"
	"path/filepath"
	"testing"
)

func makeBashHandlerCap(name, desc string) *CapFile {
	return &CapFile{
		Name: name, Description: desc, Version: 1, Scope: "user", Purpose: "Test.",
		Scripts: []Script{{
			Name: name, Kind: "handler", Runtime: "bash", Lang: "bash",
			Body: "echo ok",
		}},
	}
}

func makeREPLToolCap(name, desc string) *CapFile {
	return &CapFile{
		Name: name, Description: desc, Version: 1, Scope: "user", Purpose: "Test.",
		Scripts: []Script{{
			Name: name, Kind: "tool", Runtime: "repl", Lang: "javascript",
			Body: "async ({ x }) => x",
		}},
	}
}

func TestScanDirectory_FindsCAPFiles(t *testing.T) {
	dir := t.TempDir()

	cap1 := makeBashHandlerCap("cap_a", "A")
	cap2 := makeREPLToolCap("cap_b", "B")

	if err := WriteFile(cap1, filepath.Join(dir, "cap_a")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(cap2, filepath.Join(dir, "cap_b")); err != nil {
		t.Fatal(err)
	}

	caps, errs := ScanDirectory(dir)
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(caps) != 2 {
		t.Fatalf("want 2 caps, got %d", len(caps))
	}

	names := map[string]bool{}
	for _, c := range caps {
		names[c.Name] = true
	}
	if !names["cap_a"] || !names["cap_b"] {
		t.Errorf("want cap_a and cap_b, got %v", names)
	}
}

func TestScanDirectory_SkipsInvalidDirs(t *testing.T) {
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "bad_cap"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad_cap", "CAP.md"), []byte("not valid"), 0o644); err != nil {
		t.Fatal(err)
	}

	cap1 := makeBashHandlerCap("good_cap", "Good")
	if err := WriteFile(cap1, filepath.Join(dir, "good_cap")); err != nil {
		t.Fatal(err)
	}

	caps, errs := ScanDirectory(dir)
	if len(errs) != 1 {
		t.Errorf("want 1 error for bad_cap, got %d", len(errs))
	}
	if len(caps) != 1 {
		t.Fatalf("want 1 good cap, got %d", len(caps))
	}
	if caps[0].Name != "good_cap" {
		t.Errorf("want good_cap, got %q", caps[0].Name)
	}
}

func TestScanDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	caps, errs := ScanDirectory(dir)
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(caps) != 0 {
		t.Errorf("want 0 caps, got %d", len(caps))
	}
}

func TestScanDirectory_NonexistentDir(t *testing.T) {
	caps, errs := ScanDirectory("/tmp/nonexistent-caps-dir-12345")
	if len(errs) > 0 {
		t.Errorf("nonexistent dir should not error: %v", errs)
	}
	if len(caps) != 0 {
		t.Errorf("want 0 caps, got %d", len(caps))
	}
}

func TestScanDirectory_SetsSourcePath(t *testing.T) {
	dir := t.TempDir()
	cf := makeBashHandlerCap("path_cap", "Path test")
	if err := WriteFile(cf, filepath.Join(dir, "path_cap")); err != nil {
		t.Fatal(err)
	}

	caps, _ := ScanDirectory(dir)
	if len(caps) != 1 {
		t.Fatalf("want 1 cap, got %d", len(caps))
	}
	want := filepath.Join(dir, "path_cap", "CAP.md")
	if caps[0].SourcePath != want {
		t.Errorf("SourcePath: want %q, got %q", want, caps[0].SourcePath)
	}
}

func TestScanAll_MergesBothDirs(t *testing.T) {
	userDir := t.TempDir()
	projDir := t.TempDir()

	userCap := makeBashHandlerCap("user_cap", "User")
	projCap := makeBashHandlerCap("proj_cap", "Project")
	projCap.Scope = "project"

	if err := WriteFile(userCap, filepath.Join(userDir, "user_cap")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(projCap, filepath.Join(projDir, "proj_cap")); err != nil {
		t.Fatal(err)
	}

	caps, errs := ScanAll(userDir, projDir)
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(caps) != 2 {
		t.Fatalf("want 2 caps, got %d", len(caps))
	}
}
