package testkit

import (
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	defaultDelegateRetries = 10
	defaultRetrySleep      = 250 * time.Millisecond
)

type DelegateOps struct {
	GetTransactor func(*ecdsa.PrivateKey) (*bind.TransactOpts, error)
	Delegate      func(*bind.TransactOpts, common.Address) (*types.Transaction, error)
	WaitMined     func(common.Hash) error
}

func isRetryableDelegateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "epoch block forbidden") ||
		strings.Contains(msg, "insufficient funds") ||
		strings.Contains(msg, "nonce too low") ||
		strings.Contains(msg, "replacement transaction underpriced") ||
		strings.Contains(msg, "already known")
}

func RobustDelegate(t *testing.T, key *ecdsa.PrivateKey, val common.Address, amount *big.Int, ops DelegateOps) {
	if ops.GetTransactor == nil || ops.Delegate == nil || ops.WaitMined == nil {
		if t != nil {
			t.Fatalf("delegate helper not initialized")
		}
		return
	}

	var lastErr error
	for retry := 0; retry < defaultDelegateRetries; retry++ {
		opts, errG := ops.GetTransactor(key)
		if errG != nil {
			lastErr = errG
			time.Sleep(defaultRetrySleep)
			continue
		}

		opts.Value = amount
		tx, err := ops.Delegate(opts, val)
		if err == nil {
			if errW := ops.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				if isRetryableDelegateError(errW) {
					time.Sleep(defaultRetrySleep)
					continue
				}
				lastErr = errW
				time.Sleep(defaultRetrySleep)
				continue
			}
		}

		lastErr = err
		if isRetryableDelegateError(err) {
			time.Sleep(defaultRetrySleep)
			continue
		}
		if t != nil {
			t.Fatalf("delegate call failed: %v", err)
		}
		return
	}

	if t != nil {
		if lastErr != nil {
			t.Fatalf("delegate tx failed: %v", lastErr)
		}
		t.Fatalf("delegate retries exhausted without successful tx")
	}
}
