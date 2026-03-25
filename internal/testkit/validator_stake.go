package testkit

import (
	"math/big"
	"testing"
)

func RequireMinValidatorStake(t testing.TB, getter func() (*big.Int, error)) *big.Int {
	t.Helper()

	if getter == nil {
		t.Fatalf("min validator stake getter is nil")
	}

	minStake, err := getter()
	if err != nil {
		t.Fatalf("failed to read min validator stake: %v", err)
	}
	if minStake == nil || minStake.Sign() <= 0 {
		t.Fatalf("invalid min validator stake: %v", minStake)
	}

	return new(big.Int).Set(minStake)
}
