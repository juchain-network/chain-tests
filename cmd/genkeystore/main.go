package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintf(os.Stderr, "usage: genkeystore <hex-privkey> <password> <output-dir> <address-file>\n")
		os.Exit(2)
	}

	privHex := os.Args[1]
	pass := os.Args[2]
	outDir := os.Args[3]
	addressFile := os.Args[4]

	keyBytes, err := hex.DecodeString(privHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode privkey: %v\n", err)
		os.Exit(1)
	}
	priv, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "to ecdsa: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir keystore dir: %v\n", err)
		os.Exit(1)
	}
	ks := keystore.NewKeyStore(outDir, keystore.StandardScryptN, keystore.StandardScryptP)
	acc, err := ks.ImportECDSA(priv, pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import ecdsa: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(addressFile, []byte(acc.Address.Hex()+"\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write address file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(filepath.Clean(acc.URL.Path))
}
