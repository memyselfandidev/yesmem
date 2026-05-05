package capfile

import (
	"fmt"
	"sort"
	"strings"
)

type AdapterConfig struct {
	Direct      map[string]string
	Dispatchers map[string]map[string]string
}

func DefaultAdapters() AdapterConfig {
	return AdapterConfig{
		Direct: map[string]string{
			"store": "mcp__yesmem__cap_store",
		},
		Dispatchers: map[string]map[string]string{
			"web": {
				"fetch":  "async(p)=>sh('curl -sL --max-time 20 '+shQuote(p.url))",
				"search": "async(p)=>WebSearch(p)",
			},
			"file": {
				"read":  "async(p)=>cat(p.path||p.file_path)",
				"write": "async(p)=>put(p.path||p.file_path,p.content)",
				"glob":  "async(p)=>gl(p.pattern,p.path)",
			},
		},
	}
}

func ProviderToGeneric(script string, cfg AdapterConfig) string {
	for generic, provider := range cfg.Direct {
		script = replaceCallsite(script, provider, generic)
	}
	return script
}

func GenericToProvider(script string, cfg AdapterConfig) string {
	for generic, provider := range cfg.Direct {
		script = replaceCallsite(script, generic, provider)
	}
	return script
}

// GenerateAdapterJS generates adapter alias JS for every entry in cfg.Direct
// (except "store" when skipStore is true) and every dispatcher in
// cfg.Dispatchers. When skipStore is true, the store() wrapper is omitted
// because it is injected per-cap via WrapToolWithStore for capability binding.
func GenerateAdapterJS(cfg AdapterConfig, skipStore bool) string {
	var b strings.Builder

	keys := make([]string, 0, len(cfg.Direct))
	for k := range cfg.Direct {
		if skipStore && k == "store" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, generic := range keys {
		provider := cfg.Direct[generic]
		b.WriteString("if(typeof " + generic + "==='undefined'){")
		b.WriteString("globalThis." + generic + "=async(a)=>" + provider + "(a);")
		b.WriteString("}\n")
	}

	dkeys := make([]string, 0, len(cfg.Dispatchers))
	for k := range cfg.Dispatchers {
		dkeys = append(dkeys, k)
	}
	sort.Strings(dkeys)
	for _, generic := range dkeys {
		actions := cfg.Dispatchers[generic]
		akeys := make([]string, 0, len(actions))
		for k := range actions {
			akeys = append(akeys, k)
		}
		sort.Strings(akeys)

		b.WriteString("if(typeof " + generic + "==='undefined'){")
		b.WriteString("globalThis." + generic + "=async({action,...p})=>{")
		b.WriteString("const d={")
		for i, ak := range akeys {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(fmt.Sprintf("%s:%s", ak, actions[ak]))
		}
		b.WriteString("};")
		b.WriteString("if(!d[action])throw new Error('" + generic + ": unknown action '+action);")
		b.WriteString("return d[action](p);")
		b.WriteString("};}\n")
	}

	return b.String()
}

// WrapToolWithStore wraps a REPL tool function body in an IIFE that binds
// "store" to a capability-aware forwarder. The result is a function
// expression suitable for registerTool's 4th argument. When the tool runs
// and calls store({...}), the wrapper auto-injects capability and
// stringifies array args before forwarding to the underlying MCP tool.
func WrapToolWithStore(body string, capName string) string {
	wrapper := fmt.Sprintf("async(a)=>mcp__yesmem__cap_store({capability:%q,...a,args:Array.isArray(a.args)?JSON.stringify(a.args):a.args})", capName)
	return fmt.Sprintf("((store)=>%s)(%s)", body, wrapper)
}

func GenerateAdapterBash() string {
	return "store() { yesmem store \"$1\"; }\n"
}

func UsesGenericAdapters(script string) bool {
	cfg := DefaultAdapters()
	for name := range cfg.Direct {
		if containsCallsite(script, name) {
			return true
		}
	}
	for name := range cfg.Dispatchers {
		if containsCallsite(script, name) {
			return true
		}
	}
	return false
}

// UsesStoreAdapter returns true when script contains a store() or
// mcp__yesmem__cap_store() callsite, covering both generic and provider
// forms. Used to guard WrapToolWithStore — web()/file() adapters do
// not need a store closure.
func UsesStoreAdapter(script string) bool {
	return containsCallsite(script, "store") || containsCallsite(script, "mcp__yesmem__cap_store")
}

func containsCallsite(script, name string) bool {
	needle := name + "("
	idx := strings.Index(script, needle)
	for idx >= 0 {
		if idx == 0 || !isIdentChar(script[idx-1]) {
			return true
		}
		next := idx + len(needle)
		idx = strings.Index(script[next:], needle)
		if idx >= 0 {
			idx += next
		}
	}
	return false
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// replaceCallsite swaps every callsite of `oldName(` with `newName(` where a
// callsite means oldName followed immediately by '(' and preceded by either
// the start-of-string or a non-identifier character. The word-boundary check
// is what makes ProviderToGeneric/GenericToProvider idempotent under repeated
// CapsDirWatcher round-trips: once a body has been rewritten to the provider
// name, the substring of the generic name inside the provider name is no
// longer at a word boundary, so it is left alone.
func replaceCallsite(script, oldName, newName string) string {
	needle := oldName + "("
	if !strings.Contains(script, needle) {
		return script
	}
	var b strings.Builder
	b.Grow(len(script))
	i := 0
	for {
		rel := strings.Index(script[i:], needle)
		if rel < 0 {
			b.WriteString(script[i:])
			return b.String()
		}
		idx := i + rel
		b.WriteString(script[i:idx])
		if idx == 0 || !isIdentChar(script[idx-1]) {
			b.WriteString(newName)
			b.WriteByte('(')
		} else {
			b.WriteString(needle)
		}
		i = idx + len(needle)
	}
}
