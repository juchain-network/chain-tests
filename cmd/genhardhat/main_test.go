package main

import (
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestDeriveHardhatKeyKnownAddresses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		index   int
		address string
	}{
		{index: 0, address: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"},
		{index: 1, address: "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"},
		{index: 2, address: "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"},
		{index: 3, address: "0x90F79bf6EB2c4f870365E785982E1f101E93b906"},
		{index: 4, address: "0x15d34AAf54267DB7D7c367839AAf71A00a2C6A65"},
		{index: 5, address: "0x9965507D1a55bcC2695C58ba16FB37d819B0A4dc"},
		{index: 6, address: "0x976EA74026E726554dB657fA54763abd0C3a0aa9"},
	}

	for _, tc := range cases {
		t.Run(tc.address, func(t *testing.T) {
			key, err := deriveHardhatKey(tc.index)
			if err != nil {
				t.Fatalf("derive key %d: %v", tc.index, err)
			}
			got := crypto.PubkeyToAddress(key.PublicKey).Hex()
			if got != tc.address {
				t.Fatalf("index %d address mismatch: got %s want %s", tc.index, got, tc.address)
			}
		})
	}
}
