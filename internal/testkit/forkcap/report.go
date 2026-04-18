package forkcap

func DeferredReason(cap Capability) string {
	if cap.Reason != "" {
		return cap.Reason
	}
	return "Deferred: capability is registered but not implemented yet."
}
