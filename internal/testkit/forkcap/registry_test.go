package forkcap

import "testing"

func TestDefaultRegistryContainsDeferredCapabilitiesWithReasons(t *testing.T) {
	caps := DefaultRegistry()
	if len(caps) == 0 {
		t.Fatal("expected non-empty default registry")
	}

	required := map[string]string{
		"blob_tx_submission": "blob capability must remain deferred while txpool blocks blob tx",
	}
	seen := make(map[string]bool, len(required))
	for _, cap := range caps {
		message, ok := required[cap.Name]
		if !ok {
			continue
		}
		seen[cap.Name] = true
		if !cap.Deferred {
			t.Fatal(message)
		}
		if cap.Reason == "" {
			t.Fatalf("%s: missing deferred reason", cap.Name)
		}
	}
	for name := range required {
		if !seen[name] {
			t.Fatalf("expected %s capability in registry", name)
		}
	}
}

func TestDefaultSuiteCancunIncludesShanghaiCapabilities(t *testing.T) {
	suite, err := DefaultSuite("cancun")
	if err != nil {
		t.Fatalf("load cancun suite: %v", err)
	}
	seen := make(map[string]struct{}, len(suite.Capabilities))
	for _, cap := range suite.Capabilities {
		seen[cap.Name] = struct{}{}
	}
	if _, ok := seen["push0_execution"]; !ok {
		t.Fatal("expected cancun suite to include inherited shanghai capability push0_execution")
	}
	if _, ok := seen["mcopy_execution"]; !ok {
		t.Fatal("expected cancun suite to include cancun capability mcopy_execution")
	}
}

func TestDefaultSuitePosaIncludesIntermediateForkLayers(t *testing.T) {
	suite, err := DefaultSuite("posa")
	if err != nil {
		t.Fatalf("load posa suite: %v", err)
	}
	seen := make(map[string]struct{}, len(suite.Capabilities))
	for _, cap := range suite.Capabilities {
		seen[cap.Name] = struct{}{}
	}
	for _, required := range []string{"push0_execution", "mcopy_execution", "transient_storage_lifecycle", "cancun_header_surface", "blob_tx_submission", "fixheader_rpc_surface", "posa_contract_surface"} {
		if _, ok := seen[required]; !ok {
			t.Fatalf("expected posa suite to include %s", required)
		}
	}
}

func TestDefaultSuiteOsakaIncludesAllPriorForkLayers(t *testing.T) {
	suite, err := DefaultSuite("osaka")
	if err != nil {
		t.Fatalf("load osaka suite: %v", err)
	}
	seen := make(map[string]struct{}, len(suite.Capabilities))
	for _, cap := range suite.Capabilities {
		seen[cap.Name] = struct{}{}
	}
	for _, required := range []string{
		"push0_execution",
		"mcopy_execution",
		"transient_storage_lifecycle",
		"cancun_header_surface",
		"blob_tx_submission",
		"prague_rpc_surface",
		"prague_setcode_tx",
		"prague_capability_matrix",
		"osaka_engine_blob_api_transition",
		"osaka_engine_getpayload_transition",
		"osaka_p256verify_precompile",
		"osaka_tx_gas_cap",
		"osaka_capability_matrix",
	} {
		if _, ok := seen[required]; !ok {
			t.Fatalf("expected osaka suite to include %s", required)
		}
	}
}
