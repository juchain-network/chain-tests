package tests

import (
	"os"
	"strings"
	"testing"

	fc "juchain.org/chain/tools/ci/internal/testkit/forkcap"
)

func selectedForkcapFork() string {
	fork := strings.TrimSpace(strings.ToLower(os.Getenv("FORKCAP_FORK")))
	if fork == "" {
		return "all"
	}
	return fork
}

func selectedForkcapPhase() fc.Phase {
	return fc.NormalizePhase(os.Getenv("FORKCAP_PHASE"))
}

func requireForkcapSelection(t *testing.T, expected string) {
	t.Helper()
	selected := selectedForkcapFork()
	if selected == "all" || selected == expected {
		return
	}
	t.Skipf("skip forkcap suite for %s: FORKCAP_FORK=%s", expected, selected)
}

func requireForkcapCapability(t *testing.T, minimumFork string) fc.GateExpectation {
	t.Helper()
	selected := selectedForkcapFork()
	phase := selectedForkcapPhase()
	if selected == "all" {
		t.Skip("forkcap fork not set; this test is intended to run through the orchestrated forkcap runner")
	}
	expectation, err := fc.ExpectationForCapability(selected, phase, minimumFork)
	if err != nil {
		if strings.Contains(err.Error(), "capability gated out") {
			t.Skipf("skip capability requiring %s in %s suite", minimumFork, selected)
		}
		t.Skipf("forkcap phase not set or unsupported: %v", err)
	}
	return expectation
}

func deferredCapabilityMap() map[string]fc.Capability {
	suite, err := fc.DefaultSuite("all")
	if err != nil {
		return map[string]fc.Capability{}
	}
	out := make(map[string]fc.Capability)
	for _, cap := range suite.Capabilities {
		if cap.Deferred {
			out[cap.Name] = cap
		}
	}
	return out
}
