package proxy

import "testing"

func TestNormalizeShellCommand_NumericIDsStripped(t *testing.T) {
	a := NormalizeShellCommand(`sqlite3 ~/db "DELETE FROM t WHERE id IN (1,2,3)"`)
	b := NormalizeShellCommand(`sqlite3 ~/db "DELETE FROM t WHERE id IN (99,88)"`)
	if a != b {
		t.Errorf("expected same normalization, got:\n  a=%q\n  b=%q", a, b)
	}
}

func TestNormalizeShellCommand_DifferentVerbsDiffer(t *testing.T) {
	a := NormalizeShellCommand(`sqlite3 db "DELETE FROM t WHERE id=1"`)
	b := NormalizeShellCommand(`sqlite3 db "SELECT id FROM t WHERE id=1"`)
	if a == b {
		t.Errorf("DELETE vs SELECT should differ, both = %q", a)
	}
}

func TestNormalizeShellCommand_DifferentTablesDiffer(t *testing.T) {
	a := NormalizeShellCommand(`sqlite3 db "DELETE FROM learnings WHERE id=1"`)
	b := NormalizeShellCommand(`sqlite3 db "DELETE FROM sessions WHERE id=1"`)
	if a == b {
		t.Errorf("different tables should produce different hashes, both = %q", a)
	}
}

func TestNormalizeShellCommand_URLsStripped(t *testing.T) {
	a := NormalizeShellCommand(`curl -sL "https://example.com/a"`)
	b := NormalizeShellCommand(`curl -sL "https://example.com/b"`)
	if a != b {
		t.Errorf("URLs should strip to same form, got:\n  a=%q\n  b=%q", a, b)
	}
}

func TestNormalizeShellCommand_PathsStripped(t *testing.T) {
	a := NormalizeShellCommand(`cp /home/user/a.db /tmp/a.db`)
	b := NormalizeShellCommand(`cp /var/data/b.db /opt/c.db`)
	if a != b {
		t.Errorf("paths should strip, got:\n  a=%q\n  b=%q", a, b)
	}
}

func TestNormalizeShellCommand_SingleQuotedStringsStripped(t *testing.T) {
	a := NormalizeShellCommand(`grep 'foo' file.txt`)
	b := NormalizeShellCommand(`grep 'bar' file.txt`)
	if a != b {
		t.Errorf("single-quoted literals should strip to same form, got:\n  a=%q\n  b=%q", a, b)
	}
}

func TestNormalizeShellCommand_DoesNotPanicOnEmbeddedApostrophe(t *testing.T) {
	_ = NormalizeShellCommand(`echo "don't"`)
	_ = NormalizeShellCommand(`echo "can't won't"`)
}

func TestShellCommandHash_IsHex16(t *testing.T) {
	h := ShellCommandHash("true")
	if len(h) != 16 {
		t.Errorf("hash length: got %d, want 16", len(h))
	}
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char in hash: %q", h)
			break
		}
	}
}

func TestShellCommandHash_StructurallySimilarMatch(t *testing.T) {
	h1 := ShellCommandHash(`sqlite3 db "DELETE FROM learnings WHERE id IN (53282,53415)"`)
	h2 := ShellCommandHash(`sqlite3 db "DELETE FROM learnings WHERE id IN (1)"`)
	if h1 != h2 {
		t.Errorf("structurally similar DELETEs should match: h1=%s h2=%s", h1, h2)
	}
}

func TestShellCommandHash_StructurallyDifferentDiffer(t *testing.T) {
	h1 := ShellCommandHash(`sqlite3 db "DELETE FROM learnings WHERE id=1"`)
	h2 := ShellCommandHash(`curl -sL "https://example.com"`)
	if h1 == h2 {
		t.Errorf("different commands should differ")
	}
}
