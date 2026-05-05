// Package sanitize stellt eine Sanitization-Pipeline für YesMem bereit.
//
// Sanitizer ist die kleinste Einheit: nimmt einen String, gibt einen
// modifizierten String zurück. Pipeline kettet Sanitizer in
// Reihenfolge der Add-Calls (FIFO).
package sanitize

// Sanitizer transformiert Eingabetext.
type Sanitizer interface {
	// Sanitize wendet die Transformation an und gibt das Ergebnis zurück.
	// Empty-Input ist erlaubt.
	Sanitize(s string) string

	// Name liefert einen identifizierenden Namen für Logs/Telemetry.
	Name() string
}

// Pipeline kettet mehrere Sanitizer in Add-Reihenfolge.
type Pipeline struct {
	sanitizers []Sanitizer
}

// NewPipeline erzeugt eine leere Pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{}
}

// Add hängt einen Sanitizer ans Ende der Pipeline und gibt p für Chaining zurück.
// nil-Sanitizer werden ignoriert.
func (p *Pipeline) Add(s Sanitizer) *Pipeline {
	if s == nil {
		return p
	}
	p.sanitizers = append(p.sanitizers, s)
	return p
}

// Run wendet alle Sanitizer in Reihenfolge an. Leere Pipeline gibt s unverändert zurück.
func (p *Pipeline) Run(s string) string {
	for _, san := range p.sanitizers {
		s = san.Sanitize(s)
	}
	return s
}

// Names listet alle registrierten Sanitizer-Namen.
func (p *Pipeline) Names() []string {
	out := make([]string, 0, len(p.sanitizers))
	for _, san := range p.sanitizers {
		out = append(out, san.Name())
	}
	return out
}
