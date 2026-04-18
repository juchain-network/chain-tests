package tests

import (
	"testing"

	fc "juchain.org/chain/tools/ci/internal/testkit/forkcap"
)

func TestK_ForkcapRegistryShape(t *testing.T) {
	suite, err := fc.DefaultSuite("all")
	if err != nil {
		t.Fatalf("load default forkcap suite: %v", err)
	}
	if len(suite.Capabilities) == 0 {
		t.Fatal("expected non-empty fork capability registry")
	}

	seen := make(map[string]struct{}, len(suite.Capabilities))
	for _, cap := range suite.Capabilities {
		if cap.Name == "" {
			t.Fatal("encountered unnamed capability")
		}
		if _, exists := seen[cap.Name]; exists {
			t.Fatalf("duplicate capability %q", cap.Name)
		}
		seen[cap.Name] = struct{}{}
		if cap.MinimumFork == "" {
			t.Fatalf("capability %q missing minimum fork", cap.Name)
		}
	}
}

func TestK_ForkcapSuiteSelection(t *testing.T) {
	selected := selectedForkcapFork()
	if selected == "all" {
		selected = "osaka"
	}
	suite, err := fc.DefaultSuite(selected)
	if err != nil {
		t.Fatalf("load selected forkcap suite %s: %v", selected, err)
	}
	if len(suite.Capabilities) == 0 {
		t.Fatalf("expected non-empty capability set for selected fork %s", selected)
	}
	for _, cap := range suite.Capabilities {
		if cap.Name == "" {
			t.Fatalf("selected suite %s contains unnamed capability", selected)
		}
	}
}
