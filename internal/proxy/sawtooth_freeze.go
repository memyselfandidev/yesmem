package proxy

// freezeStubsAndInjectBreakpoint stores the current req["messages"] as the
// frozen prefix for threadID and injects a cache breakpoint at the boundary.
//
// Embedded cache_control markers are stripped from the prefix BEFORE storing
// so that the stored frozen.Messages is byte-clean. This guarantees that
// subsequent RESTORE turns produce byte-identical outgoing prefix bytes,
// which is the precondition for Anthropic prompt cache hits across turns
// post-sawtooth.
//
// Pure data-plane: callers handle logging based on returned values.
//
//   frozenCount        = number of messages stored as frozen prefix
//   strippedBreakpoints = embedded cache_control entries removed pre-store
//   breakpointInjected  = whether a boundary breakpoint was added
func (s *Server) freezeStubsAndInjectBreakpoint(req map[string]any, threadID string, cutoff int, boundary any, tokenEstimate int, rawTokenEstimate int) (frozenCount int, strippedBreakpoints int, breakpointInjected bool) {
	finalMessages, _ := req["messages"].([]any)
	if len(finalMessages) == 0 {
		return 0, 0, false
	}
	strippedBreakpoints = StripMessagesCacheControl(req, 0, len(finalMessages))
	finalMessages, _ = req["messages"].([]any)
	s.frozenStubs.Store(threadID, finalMessages, cutoff, boundary, tokenEstimate, rawTokenEstimate)
	breakpointInjected = InjectFrozenStubCacheBreakpoint(req, len(finalMessages))
	return len(finalMessages), strippedBreakpoints, breakpointInjected
}
