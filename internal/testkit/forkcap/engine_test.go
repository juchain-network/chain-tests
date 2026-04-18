package forkcap

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/config"
)

func TestDecodeJWTSecret(t *testing.T) {
	secret, err := decodeJWTSecret([]byte("0x00112233\n"))
	if err != nil {
		t.Fatalf("decode jwt secret: %v", err)
	}
	if got := hex.EncodeToString(secret); got != "00112233" {
		t.Fatalf("unexpected decoded jwt secret: %s", got)
	}
}

func TestAuthToken(t *testing.T) {
	token, err := authToken([]byte("secret"), time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("auth token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty auth token")
	}
}

func TestPreferredFeeRecipient(t *testing.T) {
	cfg := &config.Config{}
	cfg.Funder.Address = "0x00000000000000000000000000000000000000f1"
	cfg.RuntimeNodes = []config.RuntimeNode{{FeeAddress: "0x00000000000000000000000000000000000000f2"}}
	got := preferredFeeRecipient(cfg)
	want := common.HexToAddress("0x00000000000000000000000000000000000000f2")
	if got != want {
		t.Fatalf("unexpected fee recipient: got=%s want=%s", got.Hex(), want.Hex())
	}
}
