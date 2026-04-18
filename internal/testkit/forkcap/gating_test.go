package forkcap

import (
	"encoding/hex"
	"testing"
)

func TestPush0CreationBytecodeDecodes(t *testing.T) {
	code := Push0CreationBytecode()
	if len(code) == 0 {
		t.Fatal("expected non-empty PUSH0 creation bytecode")
	}
	if got := hex.EncodeToString(code); len(got) == 0 {
		t.Fatal("unexpected empty PUSH0 creation bytecode encoding")
	}
}

func TestMcopyCreationBytecodeBuilds(t *testing.T) {
	code := McopyCreationBytecode()
	if len(code) == 0 {
		t.Fatal("expected non-empty MCOPY creation bytecode")
	}
	want := McopyExpectedWord()
	if len(want) != 32 {
		t.Fatalf("expected 32-byte MCOPY expected word, got %d", len(want))
	}
}

func TestExpectationForPhase(t *testing.T) {
	pre, err := ExpectationForPhase(PhasePre)
	if err != nil {
		t.Fatalf("pre phase expectation failed: %v", err)
	}
	if !pre.ShouldFail {
		t.Fatal("pre phase must expect failure")
	}
	post, err := ExpectationForPhase(PhasePost)
	if err != nil {
		t.Fatalf("post phase expectation failed: %v", err)
	}
	if post.ShouldFail {
		t.Fatal("post phase must expect success")
	}
}

func TestForkIncludesCapability(t *testing.T) {
	includes, err := ForkIncludesCapability("osaka", "cancun")
	if err != nil {
		t.Fatalf("check inherited capability: %v", err)
	}
	if !includes {
		t.Fatal("expected osaka suite to include cancun capability")
	}
	includes, err = ForkIncludesCapability("shanghai", "osaka")
	if err != nil {
		t.Fatalf("check gated capability: %v", err)
	}
	if includes {
		t.Fatal("did not expect shanghai suite to include osaka capability")
	}
}

func TestExpectationForCapability(t *testing.T) {
	preCurrent, err := ExpectationForCapability("prague", PhasePre, "prague")
	if err != nil {
		t.Fatalf("prague current-fork expectation failed: %v", err)
	}
	if !preCurrent.ShouldFail {
		t.Fatal("expected pre-prague capability to fail before activation")
	}

	preInherited, err := ExpectationForCapability("prague", PhasePre, "cancun")
	if err != nil {
		t.Fatalf("prague inherited expectation failed: %v", err)
	}
	if preInherited.ShouldFail {
		t.Fatal("expected pre-prague inherited cancun capability to stay active")
	}

	postInherited, err := ExpectationForCapability("osaka", PhasePost, "shanghai")
	if err != nil {
		t.Fatalf("osaka post inherited expectation failed: %v", err)
	}
	if postInherited.ShouldFail {
		t.Fatal("expected post-fork inherited capability to succeed")
	}

	preBPOInherited, err := ExpectationForCapability("bpo2", PhasePre, "osaka")
	if err != nil {
		t.Fatalf("bpo2 inherited expectation failed: %v", err)
	}
	if preBPOInherited.ShouldFail {
		t.Fatal("expected pre-bpo2 inherited osaka capability to stay active")
	}
}
