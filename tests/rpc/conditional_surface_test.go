package rpc

import (
	"context"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestRPC_ConditionalSurface(t *testing.T) {
	validators := getNodesByRole("validator")
	syncNodes := getNodesByRole("sync")

	if len(validators) == 0 {
		t.Skip("no validator rpc nodes configured for positive conditional surface half")
	}

	t.Run("eth_coinbase_validator_behavior", func(t *testing.T) {
		for _, node := range validators {
			node := node
			t.Run(node.Name, func(t *testing.T) {
				client := dialRPC(t, node)
				defer client.Close()

				var coinbase string
				err := client.CallContext(context.Background(), &coinbase, "eth_coinbase")
				if err != nil {
					if isMethodUnavailableError(err) {
						t.Skipf("eth_coinbase not exposed on %s: %v", node.Name, err)
					}
					t.Fatalf("unexpected eth_coinbase error on validator node %s: %v", node.Name, err)
				}
				if !common.IsHexAddress(coinbase) {
					t.Fatalf("expected validator coinbase to be a valid hex address on %s, got %q", node.Name, coinbase)
				}
				if expected := node.ExpectedCoinbase(); expected != "" && strings.ToLower(coinbase) != expected {
					t.Fatalf("expected validator coinbase %s on %s, got %s", expected, node.Name, strings.ToLower(coinbase))
				}
			})
		}
	})

	if len(syncNodes) == 0 {
		t.Skip("no sync rpc nodes configured for negative conditional surface half")
	}

	t.Run("eth_coinbase_sync_behavior", func(t *testing.T) {
		for _, node := range syncNodes {
			node := node
			t.Run(node.Name, func(t *testing.T) {
				client := dialRPC(t, node)
				defer client.Close()

				var coinbase string
				err := client.CallContext(context.Background(), &coinbase, "eth_coinbase")
				if err != nil {
					if isMethodUnavailableError(err) {
						t.Skipf("eth_coinbase not exposed on %s: %v", node.Name, err)
					}
					assertErrorContainsAny(t, err, "etherbase must be explicitly specified")
					return
				}
				if !common.IsHexAddress(coinbase) {
					t.Fatalf("expected sync eth_coinbase success path to return a valid hex address on %s, got %q", node.Name, coinbase)
				}
			})
		}
	})
}
