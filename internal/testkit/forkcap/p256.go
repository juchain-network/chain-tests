package forkcap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var p256VerifyAddress = common.BytesToAddress([]byte{0x01, 0x00})

func P256VerifyPrecompileAddress() common.Address {
	return p256VerifyAddress
}

func TrueWord() []byte {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
}

func BuildP256VerifyInput() ([]byte, error) {
	key, digest, r, s, err := signP256Digest()
	if err != nil {
		return nil, err
	}
	pub, ok := key.Public().(*ecdsa.PublicKey)
	if !ok || pub == nil {
		return nil, fmt.Errorf("unexpected p256 public key type")
	}
	input := make([]byte, 0, 160)
	input = append(input, pad32(digest)...)
	input = append(input, pad32(r.Bytes())...)
	input = append(input, pad32(s.Bytes())...)
	input = append(input, pad32(pub.X.Bytes())...)
	input = append(input, pad32(pub.Y.Bytes())...)
	return input, nil
}

func signP256Digest() (*ecdsa.PrivateKey, []byte, *big.Int, *big.Int, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("generate p256 key: %w", err)
	}
	message := []byte("chain-tests forkcap osaka p256verify")
	digest := sha256.Sum256(message)
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("sign p256 digest: %w", err)
	}
	if !ecdsa.Verify(&key.PublicKey, digest[:], r, s) {
		return nil, nil, nil, nil, fmt.Errorf("local p256 signature self-check failed")
	}
	return key, digest[:], r, s, nil
}

func pad32(in []byte) []byte {
	out := make([]byte, 32)
	if len(in) > len(out) {
		copy(out, in[len(in)-32:])
		return out
	}
	copy(out[len(out)-len(in):], in)
	return out
}
