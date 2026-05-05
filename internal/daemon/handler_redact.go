package daemon

func (h *Handler) redact(s string) string {
	if h.redactor == nil {
		return s
	}
	return h.redactor.Sanitize(s)
}
