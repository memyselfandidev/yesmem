package daemon

func findBashScript(meta *CapMeta, scriptName string) (string, string, bool) {
	if meta == nil {
		return "", "", false
	}
	if scriptName != "" {
		for _, sc := range meta.Scripts {
			if sc.Name == scriptName && sc.Runtime == "bash" {
				return sc.Body, sc.Sandbox, true
			}
		}
		return "", "", false
	}
	for _, sc := range meta.Scripts {
		if sc.Runtime == "bash" && sc.Kind == "handler" {
			return sc.Body, sc.Sandbox, true
		}
	}
	for _, sc := range meta.Scripts {
		if sc.Runtime == "bash" {
			return sc.Body, sc.Sandbox, true
		}
	}
	return "", "", false
}
