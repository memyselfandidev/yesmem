package daemon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandbox_DisabledRunsDirectly(t *testing.T) {
	sb := NewSandbox(SandboxConfig{Enabled: false})
	output, exitCode, err := sb.Run("echo no-sandbox", 10)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("exit = %d", exitCode)
	}
	if !strings.Contains(output, "no-sandbox") {
		t.Errorf("output = %q, want 'no-sandbox'", output)
	}
}

func TestSandbox_FallbackWhenMissing(t *testing.T) {
	sb := NewSandbox(SandboxConfig{
		Enabled:             true,
		FallbackUnsandboxed: true,
	})
	output, exitCode, err := sb.Run("echo fallback-works", 10)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if exitCode != 0 || !strings.Contains(output, "fallback-works") {
		t.Errorf("fallback failed: exit=%d output=%q", exitCode, output)
	}
}

func TestSandbox_NoFallbackErrors(t *testing.T) {
	sb := NewSandbox(SandboxConfig{
		Enabled:             true,
		FallbackUnsandboxed: false,
	})
	if sb.Available() {
		t.Skip("ai-jail is installed, cannot test no-fallback path")
	}
	_, _, err := sb.Run("echo should-fail", 10)
	if err == nil {
		t.Error("expected error when ai-jail missing and fallback disabled")
	}
}

func TestSandbox_Timeout(t *testing.T) {
	sb := NewSandbox(SandboxConfig{Enabled: false})
	_, _, err := sb.Run("sleep 30", 1)
	if err == nil {
		t.Error("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

func TestSandbox_NonZeroExit(t *testing.T) {
	sb := NewSandbox(SandboxConfig{Enabled: false})
	output, exitCode, err := sb.Run("echo stderr-msg >&2; exit 42", 10)
	if exitCode != 42 {
		t.Errorf("exit = %d, want 42", exitCode)
	}
	if err == nil {
		t.Error("expected error for non-zero exit")
	}
	if !strings.Contains(output, "stderr-msg") {
		t.Errorf("output = %q, want stderr-msg", output)
	}
}

// aiJailAssetName must match the actual akitaonrails/ai-jail GitHub release
// naming: amd64→x86_64, arm64→aarch64, darwin→macos, linux→linux. Other
// combinations (windows, 386, freebsd, …) must error explicitly so the daemon
// can fall through to fallback_unsandboxed instead of hitting a 404 download.
func TestAiJailAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
		wantErr            bool
	}{
		{"linux", "amd64", "ai-jail-linux-x86_64.tar.gz", false},
		{"linux", "arm64", "ai-jail-linux-aarch64.tar.gz", false},
		{"darwin", "amd64", "ai-jail-macos-x86_64.tar.gz", false},
		{"darwin", "arm64", "ai-jail-macos-aarch64.tar.gz", false},
		{"windows", "amd64", "", true},
		{"linux", "386", "", true},
		{"freebsd", "amd64", "", true},
	}
	for _, tc := range cases {
		got, err := aiJailAssetName(tc.goos, tc.goarch)
		if tc.wantErr {
			if err == nil {
				t.Errorf("aiJailAssetName(%q,%q) expected error, got %q", tc.goos, tc.goarch, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("aiJailAssetName(%q,%q) unexpected error: %v", tc.goos, tc.goarch, err)
			continue
		}
		if got != tc.want {
			t.Errorf("aiJailAssetName(%q,%q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

// extractAiJailFromTarGz must find a regular file named "ai-jail" anywhere in
// the tarball (top-level in the real release, but the helper should not depend
// on that), write it to destPath atomically with the executable bit set.
func TestExtractAiJailFromTarGz_TopLevelBinary(t *testing.T) {
	payload := []byte("#!/bin/sh\necho synthetic ai-jail\n")
	buf := buildTarGz(t, []tarEntry{{name: "ai-jail", mode: 0o755, body: payload}})

	dest := filepath.Join(t.TempDir(), "ai-jail")
	if err := extractAiJailFromTarGz(buf, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload = %q, want %q", got, payload)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("dest mode = %v, expected user-executable", info.Mode())
	}
}

func TestExtractAiJailFromTarGz_NestedBinary(t *testing.T) {
	payload := []byte("nested-binary-bytes")
	buf := buildTarGz(t, []tarEntry{
		{name: "ai-jail-linux-x86_64/README.md", mode: 0o644, body: []byte("hi")},
		{name: "ai-jail-linux-x86_64/ai-jail", mode: 0o755, body: payload},
	})
	dest := filepath.Join(t.TempDir(), "ai-jail")
	if err := extractAiJailFromTarGz(buf, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestExtractAiJailFromTarGz_Missing(t *testing.T) {
	buf := buildTarGz(t, []tarEntry{
		{name: "README.md", mode: 0o644, body: []byte("no binary here")},
	})
	dest := filepath.Join(t.TempDir(), "ai-jail")
	if err := extractAiJailFromTarGz(buf, dest); err == nil {
		t.Error("expected error when ai-jail binary is not in tarball")
	} else if !strings.Contains(err.Error(), "ai-jail") {
		t.Errorf("error should mention ai-jail, got %v", err)
	}
}

type tarEntry struct {
	name string
	mode int64
	body []byte
}

func buildTarGz(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", e.name, err)
		}
		if _, err := tw.Write(e.body); err != nil {
			t.Fatalf("write body %s: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}
