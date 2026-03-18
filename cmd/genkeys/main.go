package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	var key *ecdsa.PrivateKey
	if len(os.Args) > 1 && os.Args[1] != "" {
		sum := sha256.Sum256([]byte(os.Args[1]))
		for {
			k, err := crypto.ToECDSA(sum[:])
			if err == nil {
				key = k
				break
			}
			sum = sha256.Sum256(sum[:])
		}
	} else {
		var err error
		key, err = crypto.GenerateKey()
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate key: %v\n", err)
			os.Exit(1)
		}
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)
	priv := hex.EncodeToString(crypto.FromECDSA(key))
	pub := hex.EncodeToString(crypto.FromECDSAPub(&key.PublicKey)[1:])
	fmt.Printf("%s,%s,%s\n", addr.Hex(), priv, pub)
}
