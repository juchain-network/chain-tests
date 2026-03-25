package main

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

const (
	hardhatMnemonic   = "test test test test test test test test test test test junk"
	hardhatPathFormat = "m/44'/60'/0'/0/%d"
)

func deriveHardhatKey(index int) (*ecdsa.PrivateKey, error) {
	if index < 0 {
		return nil, fmt.Errorf("index must be non-negative: %d", index)
	}

	path, err := accounts.ParseDerivationPath(fmt.Sprintf(hardhatPathFormat, index))
	if err != nil {
		return nil, fmt.Errorf("parse derivation path: %w", err)
	}

	seed := bip39.NewSeed(hardhatMnemonic, "")
	key, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("create master key: %w", err)
	}
	for _, component := range path {
		key, err = key.NewChildKey(component)
		if err != nil {
			return nil, fmt.Errorf("derive child %d: %w", component, err)
		}
	}

	priv, err := crypto.ToECDSA(key.Key)
	if err != nil {
		return nil, fmt.Errorf("convert private key: %w", err)
	}
	return priv, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: genhardhat <index>")
		os.Exit(1)
	}

	index, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse index: %v\n", err)
		os.Exit(1)
	}

	key, err := deriveHardhatKey(index)
	if err != nil {
		fmt.Fprintf(os.Stderr, "derive hardhat key: %v\n", err)
		os.Exit(1)
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)
	priv := hex.EncodeToString(crypto.FromECDSA(key))
	pub := hex.EncodeToString(crypto.FromECDSAPub(&key.PublicKey)[1:])
	fmt.Printf("%s,%s,%s\n", addr.Hex(), priv, pub)
}
