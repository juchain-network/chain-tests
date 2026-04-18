package tests

import (
	"bytes"
	"testing"

	fc "juchain.org/chain/tools/ci/internal/testkit/forkcap"
)

func TestK_ForkcapCapability_Push0(t *testing.T) {
	expectation := requireForkcapCapability(t, "shanghai")
	if ctx == nil {
		t.Fatal("forkcap context not initialized")
	}

	h, err := fc.NewHarness(ctx)
	if err != nil {
		t.Fatalf("create forkcap harness: %v", err)
	}
	addr, _, err := h.DeployRawContract(fc.Push0CreationBytecode(), 500_000)
	if err != nil {
		t.Fatalf("deploy PUSH0 probe contract: %v", err)
	}

	out, callErr := h.Call(addr, nil)
	if expectation.ShouldFail {
		if callErr == nil {
			t.Fatalf("expected pre-fork PUSH0 execution rejection, got success with output %x", out)
		}
		if !fc.IsExecutionReject(callErr) {
			t.Fatalf("expected opcode execution rejection, got %v", callErr)
		}
		return
	}
	if callErr != nil {
		t.Fatalf("expected post-fork PUSH0 execution success, got %v", callErr)
	}
	if len(out) != 32 {
		t.Fatalf("expected 32-byte PUSH0 probe result, got len=%d hex=%x", len(out), out)
	}
	if !bytes.Equal(out, make([]byte, 32)) {
		t.Fatalf("expected PUSH0 probe to return zero word, got %x", out)
	}
}

func TestK_ForkcapCapability_Mcopy(t *testing.T) {
	expectation := requireForkcapCapability(t, "cancun")
	if ctx == nil {
		t.Fatal("forkcap context not initialized")
	}

	h, err := fc.NewHarness(ctx)
	if err != nil {
		t.Fatalf("create forkcap harness: %v", err)
	}
	addr, _, err := h.DeployRawContract(fc.McopyCreationBytecode(), 700_000)
	if err != nil {
		t.Fatalf("deploy MCOPY probe contract: %v", err)
	}

	out, callErr := h.Call(addr, nil)
	if expectation.ShouldFail {
		if callErr == nil {
			t.Fatalf("expected pre-fork MCOPY execution rejection, got success with output %x", out)
		}
		if !fc.IsExecutionReject(callErr) {
			t.Fatalf("expected MCOPY execution rejection, got %v", callErr)
		}
		return
	}
	if callErr != nil {
		t.Fatalf("expected post-fork MCOPY execution success, got %v", callErr)
	}
	want := fc.McopyExpectedWord()
	if len(out) != len(want) {
		t.Fatalf("expected %d-byte MCOPY result, got len=%d hex=%x", len(want), len(out), out)
	}
	if !bytes.Equal(out, want) {
		t.Fatalf("unexpected MCOPY result: got=%x want=%x", out, want)
	}
}

func TestK_ForkcapCapability_TransientStorage(t *testing.T) {
	expectation := requireForkcapCapability(t, "cancun")
	if ctx == nil {
		t.Fatal("forkcap context not initialized")
	}

	h, err := fc.NewHarness(ctx)
	if err != nil {
		t.Fatalf("create forkcap harness: %v", err)
	}
	addr, _, err := h.DeployRawContract(fc.TransientStorageCreationBytecode(), 800_000)
	if err != nil {
		t.Fatalf("deploy transient storage probe contract: %v", err)
	}

	storeLoadOut, callErr := h.Call(addr, nil)
	if expectation.ShouldFail {
		if callErr == nil {
			t.Fatalf("expected pre-fork transient storage rejection, got success with output %x", storeLoadOut)
		}
		if !fc.IsExecutionReject(callErr) {
			t.Fatalf("expected transient storage execution rejection, got %v", callErr)
		}
		return
	}
	if callErr != nil {
		t.Fatalf("expected post-fork transient storage success, got %v", callErr)
	}
	want := fc.TransientStoreWord()
	if len(storeLoadOut) != len(want) {
		t.Fatalf("expected %d-byte transient store/load result, got len=%d hex=%x", len(want), len(storeLoadOut), storeLoadOut)
	}
	if !bytes.Equal(storeLoadOut, want) {
		t.Fatalf("unexpected same-tx transient store/load result: got=%x want=%x", storeLoadOut, want)
	}

	loadOnlyOut, secondErr := h.Call(addr, []byte{0x01})
	if secondErr != nil {
		t.Fatalf("expected second transient load-only call to succeed, got %v", secondErr)
	}
	if len(loadOnlyOut) != 32 {
		t.Fatalf("expected 32-byte transient load-only result, got len=%d hex=%x", len(loadOnlyOut), loadOnlyOut)
	}
	if !bytes.Equal(loadOnlyOut, make([]byte, 32)) {
		t.Fatalf("expected cross-tx transient storage to clear, got %x", loadOnlyOut)
	}
}

func TestK_ForkcapCapability_CancunHeaderSurface(t *testing.T) {
	expectation := requireForkcapCapability(t, "cancun")
	if cfg == nil || len(cfg.RPCs) == 0 {
		t.Fatal("forkcap config not initialized")
	}
	rpcURL := cfg.RPCs[0]
	block, err := fc.LatestBlockFieldMap(rpcURL)
	if err != nil {
		t.Fatalf("read latest block surface: %v", err)
	}
	if expectation.ShouldFail {
		if fc.FieldPresent(block, "blobGasUsed") || fc.FieldPresent(block, "excessBlobGas") {
			t.Fatalf("expected pre-Cancun latest block to omit blob fields, got blobGasUsed=%v excessBlobGas=%v", block["blobGasUsed"], block["excessBlobGas"])
		}
		return
	}
	if err := fc.CheckForkRPCSurface(cfg, rpcURL); err != nil {
		t.Fatalf("verify Cancun rpc surface: %v", err)
	}
	if !fc.FieldPresent(block, "blobGasUsed") || !fc.FieldPresent(block, "excessBlobGas") {
		t.Fatalf("expected post-Cancun latest block to expose blob fields, got blobGasUsed=%v excessBlobGas=%v", block["blobGasUsed"], block["excessBlobGas"])
	}
}

func TestK_ForkcapCapability_FixHeaderSurface(t *testing.T) {
	expectation := requireForkcapCapability(t, "fixheader")
	if cfg == nil || len(cfg.RPCs) == 0 {
		t.Fatal("forkcap config not initialized")
	}
	rpcURL := cfg.RPCs[0]
	block, err := fc.LatestBlockFieldMap(rpcURL)
	if err != nil {
		t.Fatalf("read latest block surface: %v", err)
	}
	if expectation.ShouldFail {
		if fc.FieldPresent(block, "parentBeaconBlockRoot") {
			t.Fatalf("expected pre-FixHeader latest block to omit parentBeaconBlockRoot, got %v", block["parentBeaconBlockRoot"])
		}
		return
	}
	if err := fc.CheckFixHeaderSurface(cfg, rpcURL); err != nil {
		t.Fatalf("verify FixHeader rpc surface: %v", err)
	}
	if !fc.FieldPresent(block, "parentBeaconBlockRoot") {
		t.Fatalf("expected post-FixHeader latest block to expose parentBeaconBlockRoot, got %v", block["parentBeaconBlockRoot"])
	}
}

func TestK_ForkcapCapability_PosaContractSurface(t *testing.T) {
	expectation := requireForkcapCapability(t, "posa")
	if ctx == nil {
		t.Fatal("forkcap context not initialized")
	}
	h, err := fc.NewHarness(ctx)
	if err != nil {
		t.Fatalf("create forkcap harness: %v", err)
	}
	if err := fc.CheckPosaContractSurface(h, expectation.ShouldFail); err != nil {
		t.Fatalf("verify PoSA contract surface: %v", err)
	}
}

func TestK_ForkcapCapability_PragueSurface(t *testing.T) {
	expectation := requireForkcapCapability(t, "prague")
	if cfg == nil || len(cfg.RPCs) == 0 {
		t.Fatal("forkcap config not initialized")
	}
	rpcURL := cfg.RPCs[0]
	block, err := fc.LatestBlockFieldMap(rpcURL)
	if err != nil {
		t.Fatalf("read latest block surface: %v", err)
	}
	if expectation.ShouldFail {
		if fc.FieldPresent(block, "requestsHash") {
			t.Fatalf("expected pre-Prague latest block to omit requestsHash, got %v", block["requestsHash"])
		}
		return
	}
	if err := fc.CheckForkRPCSurface(cfg, rpcURL); err != nil {
		t.Fatalf("verify Prague rpc surface: %v", err)
	}
	if !fc.FieldPresent(block, "requestsHash") {
		t.Fatalf("expected post-Prague latest block to expose requestsHash, got %v", block["requestsHash"])
	}
}

func TestK_ForkcapCapability_PragueSetCodeTx(t *testing.T) {
	expectation := requireForkcapCapability(t, "prague")
	if ctx == nil {
		t.Fatal("forkcap context not initialized")
	}
	h, err := fc.NewHarness(ctx)
	if err != nil {
		t.Fatalf("create forkcap harness: %v", err)
	}
	if err := fc.CheckPragueSetCodeTx(h, expectation.ShouldFail); err != nil {
		t.Fatalf("verify Prague SetCodeTx behavior: %v", err)
	}
}

func TestK_ForkcapCapability_BPO1BlobSchedule(t *testing.T) {
	requireForkcapSelection(t, "bpo1")
	expectation := requireForkcapCapability(t, "bpo1")
	if cfg == nil || len(cfg.RPCs) == 0 {
		t.Fatal("forkcap config not initialized")
	}
	err := fc.CheckBPOBlobSchedule(cfg, cfg.RPCs[0], "bpo1")
	if expectation.ShouldFail {
		if err == nil {
			t.Fatal("expected pre-BPO1 blob schedule mismatch, got success")
		}
		return
	}
	if err != nil {
		t.Fatalf("verify BPO1 blob schedule: %v", err)
	}
}

func TestK_ForkcapCapability_BPO2BlobSchedule(t *testing.T) {
	requireForkcapSelection(t, "bpo2")
	expectation := requireForkcapCapability(t, "bpo2")
	if cfg == nil || len(cfg.RPCs) == 0 {
		t.Fatal("forkcap config not initialized")
	}
	err := fc.CheckBPOBlobSchedule(cfg, cfg.RPCs[0], "bpo2")
	if expectation.ShouldFail {
		if err == nil {
			t.Fatal("expected pre-BPO2 blob schedule mismatch, got success")
		}
		return
	}
	if err != nil {
		t.Fatalf("verify BPO2 blob schedule: %v", err)
	}
}

func TestK_ForkcapCapability_OsakaEngineBlobAPITransition(t *testing.T) {
	expectation := requireForkcapCapability(t, "osaka")
	if cfg == nil {
		t.Fatal("forkcap config not initialized")
	}
	if err := fc.CheckOsakaEngineBlobAPITransition(cfg, expectation.ShouldFail); err != nil {
		t.Fatalf("verify Osaka engine blob API transition: %v", err)
	}
}

func TestK_ForkcapCapability_OsakaEngineGetPayloadTransition(t *testing.T) {
	expectation := requireForkcapCapability(t, "osaka")
	if cfg == nil {
		t.Fatal("forkcap config not initialized")
	}
	if err := fc.CheckOsakaEngineGetPayloadTransition(cfg, expectation.ShouldFail); err != nil {
		t.Fatalf("verify Osaka engine getPayload transition: %v", err)
	}
}

func TestK_ForkcapCapability_OsakaP256VerifyPrecompile(t *testing.T) {
	expectation := requireForkcapCapability(t, "osaka")
	if ctx == nil {
		t.Fatal("forkcap context not initialized")
	}
	h, err := fc.NewHarness(ctx)
	if err != nil {
		t.Fatalf("create forkcap harness: %v", err)
	}
	input, err := fc.BuildP256VerifyInput()
	if err != nil {
		t.Fatalf("build p256 verify input: %v", err)
	}
	out, callErr := h.Call(fc.P256VerifyPrecompileAddress(), input)
	if callErr != nil {
		t.Fatalf("call p256verify precompile: %v", callErr)
	}
	if expectation.ShouldFail {
		if len(out) != 0 {
			t.Fatalf("expected pre-Osaka p256verify call to behave like empty address, got %x", out)
		}
		return
	}
	want := fc.TrueWord()
	if !bytes.Equal(out, want) {
		t.Fatalf("unexpected Osaka p256verify result: got=%x want=%x", out, want)
	}
}

func TestK_ForkcapCapability_OsakaTxGasCap(t *testing.T) {
	expectation := requireForkcapCapability(t, "osaka")
	if ctx == nil {
		t.Fatal("forkcap context not initialized")
	}
	h, err := fc.NewHarness(ctx)
	if err != nil {
		t.Fatalf("create forkcap harness: %v", err)
	}
	if expectation.ShouldFail {
		if err := fc.CheckOsakaTxGasCapInactive(h); err != nil {
			t.Fatalf("verify pre-Osaka tx gas cap inactivity: %v", err)
		}
		return
	}
	if err := fc.CheckOsakaTxGasCap(h); err != nil {
		t.Fatalf("verify Osaka tx gas cap: %v", err)
	}
}
