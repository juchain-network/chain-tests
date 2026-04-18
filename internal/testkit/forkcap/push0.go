package forkcap

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

const push0RuntimeHex = "5f5f5260205ff3"

func MustDecodeHex(raw string) []byte {
	out, err := hex.DecodeString(raw)
	if err != nil {
		panic(err)
	}
	return out
}

func BuildSimpleCreationBytecode(runtime []byte) []byte {
	if len(runtime) == 0 {
		panic("empty runtime bytecode")
	}
	if len(runtime) > 0xff {
		panic(fmt.Sprintf("runtime too large for simple creation builder: %d", len(runtime)))
	}
	const prefixLen = 12
	prefix := []byte{0x60, byte(len(runtime)), 0x60, prefixLen, 0x60, 0x00, 0x39, 0x60, byte(len(runtime)), 0x60, 0x00, 0xf3}
	return append(prefix, runtime...)
}

func Push0CreationBytecode() []byte {
	return BuildSimpleCreationBytecode(MustDecodeHex(push0RuntimeHex))
}

func McopyExpectedWord() []byte {
	return MustDecodeHex("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
}

func McopyCreationBytecode() []byte {
	word := McopyExpectedWord()
	runtime := bytes.NewBuffer(nil)
	runtime.WriteByte(0x7f)                 // PUSH32
	runtime.Write(word)                     // source word
	runtime.Write([]byte{0x60, 0x20, 0x52}) // PUSH1 0x20 MSTORE
	runtime.Write([]byte{0x60, 0x20})       // len = 32
	runtime.Write([]byte{0x60, 0x20})       // src = 32
	runtime.Write([]byte{0x60, 0x00})       // dst = 0
	runtime.WriteByte(0x5e)                 // MCOPY
	runtime.Write([]byte{0x60, 0x20, 0x60, 0x00, 0xf3})
	return BuildSimpleCreationBytecode(runtime.Bytes())
}

func TransientStoreWord() []byte {
	return MustDecodeHex("11223344556677889900aabbccddeeff00112233445566778899aabbccddeeff")
}

func TransientStorageCreationBytecode() []byte {
	word := TransientStoreWord()
	runtime := bytes.NewBuffer(nil)
	runtime.WriteByte(0x36)                   // CALLDATASIZE
	runtime.WriteByte(0x15)                   // ISZERO
	runtime.Write([]byte{0x60, 0x10, 0x57})   // PUSH1 storeLabel JUMPI
	runtime.Write([]byte{0x60, 0x01, 0x5c})   // PUSH1 0x01 TLOAD
	runtime.Write([]byte{0x60, 0x00, 0x52})   // PUSH1 0x00 MSTORE
	runtime.Write([]byte{0x60, 0x20, 0x60, 0x00, 0xf3})
	runtime.WriteByte(0x5b)                   // JUMPDEST storeLabel
	runtime.WriteByte(0x7f)                   // PUSH32 value
	runtime.Write(word)
	runtime.Write([]byte{0x60, 0x01, 0x5d})   // PUSH1 0x01 TSTORE
	runtime.Write([]byte{0x60, 0x01, 0x5c})   // PUSH1 0x01 TLOAD
	runtime.Write([]byte{0x60, 0x00, 0x52})   // PUSH1 0x00 MSTORE
	runtime.Write([]byte{0x60, 0x20, 0x60, 0x00, 0xf3})
	return BuildSimpleCreationBytecode(runtime.Bytes())
}
