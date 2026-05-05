package daemon

import "fmt"

type SandboxProfile int

const (
	ProfileNone     SandboxProfile = iota
	ProfileStandard
	ProfileStrict
)

func (p SandboxProfile) String() string {
	switch p {
	case ProfileNone:
		return "none"
	case ProfileStandard:
		return "standard"
	case ProfileStrict:
		return "strict"
	default:
		return "none"
	}
}

func ParseSandboxProfile(s string) (SandboxProfile, error) {
	switch s {
	case "", "none":
		return ProfileNone, nil
	case "standard":
		return ProfileStandard, nil
	case "strict":
		return ProfileStrict, nil
	default:
		return ProfileNone, fmt.Errorf("unknown sandbox profile: %q (valid: none, standard, strict)", s)
	}
}
