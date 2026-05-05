package storage

import "testing"

func TestIsFileInCodeIndex_EmptyProject(t *testing.T) {
	s := mustOpen(t)
	defer s.Close()
	if s.IsFileInCodeIndex("nonexistent", "foo.go") {
		t.Error("should return false for nonexistent project")
	}
}

func TestIsFileInCodeIndex_ExactMatch(t *testing.T) {
	s := mustOpen(t)
	defer s.Close()
	scanJSON := `{"Packages":[{"Files":[{"Path":"internal/proxy/proxy.go"},{"Path":"main.go"}]}]}`
	if err := s.PersistScan("testproject", scanJSON, "abc123", 0); err != nil {
		t.Fatalf("persist scan: %v", err)
	}

	if !s.IsFileInCodeIndex("testproject", "internal/proxy/proxy.go") {
		t.Error("should find exact path match")
	}
	if !s.IsFileInCodeIndex("testproject", "main.go") {
		t.Error("should find root-level file")
	}
	if s.IsFileInCodeIndex("testproject", "nonexistent.go") {
		t.Error("should not find nonexistent file")
	}
}

func TestIsFileInCodeIndex_SuffixMatch(t *testing.T) {
	s := mustOpen(t)
	defer s.Close()
	scanJSON := `{"Packages":[{"Files":[{"Path":"internal/proxy/proxy.go"}]}]}`
	if err := s.PersistScan("testproject", scanJSON, "abc123", 0); err != nil {
		t.Fatalf("persist scan: %v", err)
	}

	if !s.IsFileInCodeIndex("testproject", "proxy.go") {
		t.Error("should match by filename suffix when unambiguous")
	}
}

func TestDismissCodeNav_NotDismissedInitially(t *testing.T) {
	s := mustOpen(t)
	defer s.Close()
	if s.IsCodeNavDismissed("session-123", 5) {
		t.Error("should not be dismissed initially")
	}
}

func TestDismissCodeNav_IncrementBelowThreshold(t *testing.T) {
	s := mustOpen(t)
	defer s.Close()
	for i := 0; i < 4; i++ {
		if err := s.DismissCodeNav("session-123"); err != nil {
			t.Fatalf("dismiss %d: %v", i+1, err)
		}
		if s.IsCodeNavDismissed("session-123", 5) {
			t.Errorf("should not be dismissed after %d dismissals", i+1)
		}
	}
}

func TestDismissCodeNav_AtThreshold(t *testing.T) {
	s := mustOpen(t)
	defer s.Close()
	for i := 0; i < 5; i++ {
		if err := s.DismissCodeNav("session-123"); err != nil {
			t.Fatalf("dismiss %d: %v", i+1, err)
		}
	}
	if !s.IsCodeNavDismissed("session-123", 5) {
		t.Error("should be dismissed after 5 dismissals")
	}
}

func TestDismissCodeNav_SessionIsolation(t *testing.T) {
	s := mustOpen(t)
	defer s.Close()
	for i := 0; i < 5; i++ {
		s.DismissCodeNav("session-A")
	}
	if s.IsCodeNavDismissed("session-B", 5) {
		t.Error("dismissal of session A should not affect session B")
	}
}
