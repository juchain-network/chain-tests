package forkcap

import (
	"bytes"
	"testing"
)

func TestBuildP256VerifyInput(t *testing.T) {
	input, err := BuildP256VerifyInput()
	if err != nil {
		t.Fatalf("build p256 verify input: %v", err)
	}
	if len(input) != 160 {
		t.Fatalf("expected 160-byte p256 input, got %d", len(input))
	}
	if bytes.Equal(input, make([]byte, 160)) {
		t.Fatal("expected non-zero p256 verify input")
	}
}
