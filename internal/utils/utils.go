package utils

import (
	"math/big"
	"testing"
)

// AssertNoError checks if err is nil
func AssertNoError(t *testing.T, err error, msg string) {
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertTrue checks if condition is true
func AssertTrue(t *testing.T, condition bool, msg string) {
	if !condition {
		t.Fatalf("%s", msg)
	}
}

// AssertBigIntEq checks if a == b
func AssertBigIntEq(t *testing.T, a, b *big.Int, msg string) {
	if a.Cmp(b) != 0 {
		t.Fatalf("%s: expected %s, got %s", msg, b.String(), a.String())
	}
}

// ToWei converts Ether to Wei

func ToWei(ether float64) *big.Int {

	wei := new(big.Int)

	wei.SetString("1000000000000000000", 10) // 10^18

	

	e := new(big.Float).SetFloat64(ether)

	w := new(big.Float).SetInt(wei)

	

	res := new(big.Float).Mul(e, w)

	

	result := new(big.Int)

	res.Int(result)

	return result

}



// WeiToEther converts Wei to Ether string

func WeiToEther(wei *big.Int) string {

	if wei == nil {

		return "0"

	}

	f := new(big.Float).SetInt(wei)

	f.Quo(f, big.NewFloat(1e18))

	return f.Text('f', 4)

}


