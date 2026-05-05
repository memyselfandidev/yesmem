package sanitize

import (
	"regexp"
	"strconv"
	"strings"
)

// Compile-time check: SecretRedactor erfüllt das Sanitizer-Interface.
var _ Sanitizer = (*SecretRedactor)(nil)

// SecretRedactor erkennt typische Secret-Muster (anthropic_api_key,
// openai_api_key, aws_access_key, aws_secret_access_key, github_pat, jwt,
// bearer_token, password_in_url, generic_api_key, ssh_private_key_block,
// gpg_private_key_block, ipv4_public, hex_secret, email, phone) und ersetzt
// sie durch [REDACTED:<kind>]-Marker. Optional kann eine Allowlist von exakten
// Strings übergeben werden, die nicht redacted werden. Die Allowlist gilt für
// alle Patterns auf Basis des vollständigen Regex-Treffers. Template-Patterns
// mit Capture-Groups (aws_secret_access_key, password_in_url) umgehen die
// Allowlist, da dort nur Teile des Treffers ersetzt werden; bearer_token und
// generic_api_key sind seit F3/F4 keine Template-Patterns mehr und
// respektieren die Allowlist.
type SecretRedactor struct {
	allowed  map[string]struct{}
	patterns []redactPattern
}

type redactPattern struct {
	kind     string
	re       *regexp.Regexp
	template string              // optional Expand-style Replacement; leer => Default-Marker [REDACTED:<kind>].
	// filter is invoked on each regex match. Return true to redact the match,
	// false to keep it unmodified in the output. nil means always redact.
	filter func(match string) bool
}

// NewSecretRedactor returns a Sanitizer that applies the default 15-kind pattern set:
// anthropic_api_key, openai_api_key, aws_access_key, aws_secret_access_key, github_pat,
// jwt, bearer_token, password_in_url, generic_api_key, ssh_private_key_block,
// gpg_private_key_block, ipv4_public, hex_secret, email, phone.
//
// allowed contains exact strings that will NOT be redacted. The comparison is against
// the FULL regex match string for the triggered pattern, not against substrings or
// capture groups.
//
// Two patterns in the default set use template-style replacement and capture surrounding
// context as part of their match:
//
//   - aws_secret_access_key: match includes the keyword+separator preamble
//     (e.g. "aws_secret_access_key=THEKEY40CHARS"), not the bare key alone
//   - password_in_url: match includes the scheme://user: prefix and @host suffix
//     (e.g. "https://user:thepassword@host"), not the bare password alone
//
// Template-style patterns bypass the AllowedExceptions check entirely: the full match
// is never compared to allowed because the replacement is applied via regex backrefs and
// only the secret capture group is substituted out.
//
// All other patterns (anthropic_api_key, openai_api_key, aws_access_key, github_pat,
// jwt, bearer_token, generic_api_key, ssh_private_key_block, gpg_private_key_block,
// ipv4_public, hex_secret, email, phone) are plain matches: adding the exact matched
// string to allowed skips redaction for that value. bearer_token and generic_api_key
// were template-style before F3/F4 and now also respect the allowlist on the full match.
//
// Template-style patterns cannot be exempted via allowed. To suppress redaction
// for a specific template-matched value, construct the redactor with a custom
// pattern set that omits the unwanted pattern.
func NewSecretRedactor(allowed []string) *SecretRedactor {
	allow := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allow[a] = struct{}{}
	}
	return &SecretRedactor{
		allowed:  allow,
		patterns: defaultPatterns(),
	}
}

// defaultPatterns returns the compiled set of secret-detection patterns. Kinds in registration order:
// anthropic_api_key, openai_api_key, aws_access_key, aws_secret_access_key,
// github_pat, jwt, bearer_token, password_in_url, generic_api_key,
// ssh_private_key_block, gpg_private_key_block, ipv4_public, hex_secret,
// email, phone.
func defaultPatterns() []redactPattern {
	return []redactPattern{
		{kind: "anthropic_api_key", re: regexp.MustCompile(`sk-ant-[a-zA-Z0-9_\-]{50,}`)},
		// openai_api_key MUST stay registered after anthropic_api_key: sk-ant-* keys
		// match both, and order decides which kind tag wins.
		{kind: "openai_api_key", re: regexp.MustCompile(`sk-(?:proj-|svcacct-|admin-|None-)?[A-Za-z0-9_-]{20,}`)},
		{kind: "aws_access_key", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
		{kind: "aws_secret_access_key", re: regexp.MustCompile(`(?i)(aws[_\-]?secret[_\-]?access[_\-]?key["']?\s*[:=]\s*["']?)([A-Za-z0-9/+]{40})`), template: "${1}[REDACTED:aws_secret_access_key]"},
		{kind: "github_pat", re: regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}\b`)},
		{kind: "jwt", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`)},
		{kind: "bearer_token", re: regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-+/=]{8,}`)},
		{kind: "password_in_url", re: regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+\-.]*://[^:/@\s]*:)([^@\s]+)(@[^/\s]+)`), template: "${1}[REDACTED:password_in_url]${3}"},
		{kind: "generic_api_key", re: regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|secret|token)["']?\s*[:=]\s*["']?[A-Za-z0-9._\-+/=]{20,}["']?`)},
		{kind: "ssh_private_key_block", re: regexp.MustCompile(`(?s)-----BEGIN (?:OPENSSH |RSA |DSA |EC |)PRIVATE KEY-----.*?-----END (?:OPENSSH |RSA |DSA |EC |)PRIVATE KEY-----`)},
		{kind: "gpg_private_key_block", re: regexp.MustCompile(`(?s)-----BEGIN PGP PRIVATE KEY BLOCK-----.*?-----END PGP PRIVATE KEY BLOCK-----`)},
		{kind: "ipv4_public", re: regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)\.){3}(?:25[0-5]|2[0-4]\d|1\d{2}|[1-9]?\d)\b`), filter: filterPublicIPv4},
		{kind: "hex_secret", re: regexp.MustCompile(`\b[0-9a-f]{32,128}\b`)},
		{kind: "email", re: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)},
		{kind: "phone", re: regexp.MustCompile(`\+?\d{1,3}[\s.-]?\(?\d{2,4}\)?[\s.-]?\d{3,4}[\s.-]?\d{4}\b`)},
	}
}

// Sanitize wendet alle Pattern an und ersetzt Treffer durch [REDACTED:<kind>],
// es sei denn der Treffer steht exakt in der Allowlist. Template-Patterns
// (aws_secret_access_key, password_in_url) verwenden Regex-Backrefs zum
// Erhalten von Capture-Group-Inhalten und ignorieren die Allowlist.
// bearer_token und generic_api_key sind seit F3/F4 keine Template-Patterns
// mehr und respektieren die Allowlist auf Basis des vollständigen Treffers.
func (r *SecretRedactor) Sanitize(s string) string {
	for _, p := range r.patterns {
		if p.template != "" {
			s = p.re.ReplaceAllString(s, p.template)
			continue
		}
		s = p.re.ReplaceAllStringFunc(s, func(match string) string {
			if _, ok := r.allowed[match]; ok {
				return match
			}
			if p.filter != nil && !p.filter(match) {
				return match
			}
			return "[REDACTED:" + p.kind + "]"
		})
	}
	return s
}

// Name implementiert Sanitizer.
func (r *SecretRedactor) Name() string {
	return "secret_redactor"
}

// filterPublicIPv4 returns true for public IPs that should be redacted and
// false for RFC-1918 / loopback / link-local ranges that should be preserved.
func filterPublicIPv4(match string) bool {
	parts := strings.Split(match, ".")
	if len(parts) != 4 {
		return true
	}
	// regex byte-range alternation guarantees 0..255 per octet; Atoi cannot fail here.
	a, _ := strconv.Atoi(parts[0])
	b, _ := strconv.Atoi(parts[1])
	switch {
	case a == 0:
		return false
	case a == 10:
		return false
	case a == 100 && b >= 64 && b <= 127:
		return false
	case a == 127:
		return false
	case a == 169 && b == 254:
		return false
	case a == 172 && b >= 16 && b <= 31:
		return false
	case a == 192 && b == 168:
		return false
	case a >= 224:
		return false
	}
	return true
}
