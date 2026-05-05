package sanitize

import (
	"strings"
	"testing"
)

type fakeSanitizer struct {
	prefix string
}

func (f *fakeSanitizer) Sanitize(s string) string {
	return f.prefix + s
}

func (f *fakeSanitizer) Name() string {
	return "fake:" + f.prefix
}

func TestPipeline_RunAppliesAllInOrder(t *testing.T) {
	p := NewPipeline()
	p.Add(&fakeSanitizer{prefix: "A:"})
	p.Add(&fakeSanitizer{prefix: "B:"})
	got := p.Run("x")
	if got != "B:A:x" {
		t.Fatalf("expected B:A:x, got %q", got)
	}
}

func TestPipeline_RunEmptyPipelineReturnsInput(t *testing.T) {
	p := NewPipeline()
	if got := p.Run("hello"); got != "hello" {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

func TestPipeline_NamesListsAllSanitizers(t *testing.T) {
	p := NewPipeline()
	p.Add(&fakeSanitizer{prefix: "X:"})
	p.Add(&fakeSanitizer{prefix: "Y:"})
	names := p.Names()
	if len(names) != 2 || names[0] != "fake:X:" || names[1] != "fake:Y:" {
		t.Fatalf("unexpected names: %v", names)
	}
}

func TestPipeline_RunHandlesEmptyInput(t *testing.T) {
	p := NewPipeline()
	p.Add(&fakeSanitizer{prefix: "A:"})
	if got := p.Run(""); !strings.HasPrefix(got, "A:") {
		t.Fatalf("expected A: prefix on empty input, got %q", got)
	}
}
