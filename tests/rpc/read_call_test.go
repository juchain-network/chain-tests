package rpc

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"juchain.org/chain/tools/ci/contracts"
	cicontext "juchain.org/chain/tools/ci/internal/context"
)

func TestRPC_EthCall_GetActiveValidators(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	ciCtx, err := cicontext.NewCIContext(rpcCfg)
	if err != nil {
		t.Fatalf("Failed to create CI context: %v", err)
	}

	// Get active validators from CIContext to establish truth
	expectedValidators, err := ciCtx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("Failed to get active validators from system contract: %v", err)
	}

	// The ABI of the Validators contract
	validatorsABI, err := contracts.ValidatorsMetaData.GetAbi()
	if err != nil {
		t.Fatalf("Failed to get Validators ABI: %v", err)
	}

	validatorsAddr := "0x000000000000000000000000000000000000f010"
	callData := "0x9de70258" // GetActiveValidators() signature

	for _, node := range nodes {
		node := node // capture range variable
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()

			var callResult string
			callMsg := map[string]string{
				"to":   validatorsAddr,
				"data": callData,
			}

			assertRawCall(t, node, &callResult, "eth_call", callMsg, "latest")

			// Decode the returned data
			decodedBytes, err := hexutil.Decode(callResult)
			if err != nil {
				t.Fatalf("Failed to decode call result %s: %v", callResult, err)
			}

			// Unpack using the ABI
			unpacked, err := validatorsABI.Unpack("getActiveValidators", decodedBytes)
			if err != nil {
				t.Fatalf("Failed to unpack result: %v", err)
			}

			if len(unpacked) != 1 {
				t.Fatalf("Expected 1 unpacked result, got %d", len(unpacked))
			}

			actualValidators, ok := unpacked[0].([]common.Address)
			if !ok {
				t.Fatalf("Expected []common.Address, got %T", unpacked[0])
			}

			// Compare the actual validators from RPC with expected ones
			if !reflect.DeepEqual(expectedValidators, actualValidators) {
				t.Errorf("Mismatch in validators. Expected %v, got %v", expectedValidators, actualValidators)
			}
		})
	}
}

func TestRPC_EthCoinbase(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	for _, node := range nodes {
		node := node // capture range variable
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()

			client := dialRPC(t, node)
			defer client.Close()

			var coinbase string
			err := client.CallContext(context.Background(), &coinbase, "eth_coinbase")
			if err != nil {
				if strings.Contains(err.Error(), "etherbase must be explicitly specified") || isMethodUnavailableError(err) {
					// Expected on runtimes/nodes that do not expose eth_coinbase.
					return
				}
				t.Fatalf("RPC Call failed: %v", err)
			}
			if !common.IsHexAddress(coinbase) {
				t.Errorf("Expected valid hex address for coinbase, got %v", coinbase)
			}
		})
	}
}
