package rpc

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"

	cicontext "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestRPC_LookupMethods(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	ciCtx, err := cicontext.NewCIContext(rpcCfg)
	if err != nil {
		t.Fatalf("Failed to create CI context: %v", err)
	}

	primaryClient := ciCtx.Clients[0]

	// 1. Create a zero-value transaction from the funder address to a dummy address
	dummyAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	opts, err := ciCtx.GetTransactorEx(ciCtx.FunderKey, true)
	if err != nil {
		t.Fatalf("Failed to get transactor for funder: %v", err)
	}

	gasPrice := opts.GasPrice
	if gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(1000000000) // 1 gwei fallback
	}

	tx := types.NewTransaction(opts.Nonce.Uint64(), dummyAddr, big.NewInt(0), 21000, gasPrice, nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(ciCtx.ChainID), ciCtx.FunderKey)
	if err != nil {
		t.Fatalf("Failed to sign tx: %v", err)
	}

	err = primaryClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		t.Fatalf("Failed to send tx: %v", err)
	}

	txHash := signedTx.Hash()
	
	// Wait for the transaction to be mined
	err = ciCtx.WaitMined(txHash)
	if err != nil {
		t.Fatalf("Failed waiting for tx to be mined: %v", err)
	}

	// Get the receipt from primary client to know the block hash and number
	receipt, err := primaryClient.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		t.Fatalf("Failed to get receipt from primary client: %v", err)
	}

	blockHash := receipt.BlockHash.Hex()
	blockNumber := hexutil.EncodeUint64(receipt.BlockNumber.Uint64())
	txHashHex := txHash.Hex()

	// Now run the lookups on every discovered node
	for _, node := range nodes {
		node := node
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()

			waitForLookupFixtureOnNode(t, node, txHashHex, blockHash, blockNumber)

			// Query eth_getTransactionByHash
			t.Run("eth_getTransactionByHash", func(t *testing.T) {
				var txObj map[string]interface{}
				assertRawCall(t, node, &txObj, "eth_getTransactionByHash", txHashHex)
				if txObj == nil {
					t.Fatalf("eth_getTransactionByHash returned nil for %s", txHashHex)
				}
				if txObj["hash"] != txHashHex {
					t.Errorf("Expected tx hash %s, got %v", txHashHex, txObj["hash"])
				}
				if txObj["blockHash"] != blockHash {
					t.Errorf("Expected block hash %s, got %v", blockHash, txObj["blockHash"])
				}
				if txObj["blockNumber"] != blockNumber {
					t.Errorf("Expected block number %s, got %v", blockNumber, txObj["blockNumber"])
				}
			})

			// Query eth_getTransactionReceipt
			t.Run("eth_getTransactionReceipt", func(t *testing.T) {
				var receiptObj map[string]interface{}
				assertRawCall(t, node, &receiptObj, "eth_getTransactionReceipt", txHashHex)
				if receiptObj == nil {
					t.Fatalf("eth_getTransactionReceipt returned nil for %s", txHashHex)
				}
				if receiptObj["transactionHash"] != txHashHex {
					t.Errorf("Expected tx hash %s, got %v", txHashHex, receiptObj["transactionHash"])
				}
				if receiptObj["blockHash"] != blockHash {
					t.Errorf("Expected block hash %s, got %v", blockHash, receiptObj["blockHash"])
				}
				if receiptObj["blockNumber"] != blockNumber {
					t.Errorf("Expected block number %s, got %v", blockNumber, receiptObj["blockNumber"])
				}
			})

			// Query eth_getBlockByHash
			t.Run("eth_getBlockByHash", func(t *testing.T) {
				var blockObj map[string]interface{}
				assertRawCall(t, node, &blockObj, "eth_getBlockByHash", blockHash, false)
				if blockObj == nil {
					t.Fatalf("eth_getBlockByHash returned nil for %s", blockHash)
				}
				if blockObj["hash"] != blockHash {
					t.Errorf("Expected block hash %s, got %v", blockHash, blockObj["hash"])
				}
				if blockObj["number"] != blockNumber {
					t.Errorf("Expected block number %s, got %v", blockNumber, blockObj["number"])
				}
				
				// Verify inclusion in transactions array (just hashes since fullTx=false)
				txs, ok := blockObj["transactions"].([]interface{})
				if !ok {
					t.Fatalf("Transactions field is missing or not an array: %v", blockObj["transactions"])
				}
				found := false
				for _, th := range txs {
					if th == txHashHex {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Transaction %s not found in block %s transactions array", txHashHex, blockHash)
				}
			})

			// Query eth_getBlockByNumber
			t.Run("eth_getBlockByNumber", func(t *testing.T) {
				var blockObj map[string]interface{}
				assertRawCall(t, node, &blockObj, "eth_getBlockByNumber", blockNumber, false)
				if blockObj == nil {
					t.Fatalf("eth_getBlockByNumber returned nil for %s", blockNumber)
				}
				if blockObj["hash"] != blockHash {
					t.Errorf("Expected block hash %s, got %v", blockHash, blockObj["hash"])
				}
				if blockObj["number"] != blockNumber {
					t.Errorf("Expected block number %s, got %v", blockNumber, blockObj["number"])
				}

				// Verify inclusion in transactions array
				txs, ok := blockObj["transactions"].([]interface{})
				if !ok {
					t.Fatalf("Transactions field is missing or not an array: %v", blockObj["transactions"])
				}
				found := false
				for _, th := range txs {
					if th == txHashHex {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Transaction %s not found in block %s transactions array", txHashHex, blockNumber)
				}
			})

			// Query eth_call
			t.Run("eth_call", func(t *testing.T) {
				var callResult string
				// A simple eth_call to get something, e.g., calling a non-existent method on dummyAddr
				// or just sending 0 data. It should return 0x.
				callMsg := map[string]string{
					"to":   dummyAddr.Hex(),
					"data": "0x",
				}
				assertRawCall(t, node, &callResult, "eth_call", callMsg, "latest")
				if callResult != "0x" {
					t.Errorf("Expected 0x for empty eth_call, got %v", callResult)
				}
			})

			// Query eth_coinbase
			t.Run("eth_coinbase", func(t *testing.T) {
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
		})
	}
}

func waitForLookupFixtureOnNode(t *testing.T, node RPCNode, txHashHex, blockHash, blockNumber string) {
	t.Helper()
	client := dialRPC(t, node)
	defer client.Close()

	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 40,
		Interval:    300 * time.Millisecond,
	}, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var txObj map[string]interface{}
		if err := client.CallContext(ctx, &txObj, "eth_getTransactionByHash", txHashHex); err != nil {
			return false, fmt.Errorf("eth_getTransactionByHash on %s: %w", node.Name, err)
		}
		if txObj == nil || txObj["blockHash"] != blockHash || txObj["blockNumber"] != blockNumber {
			return false, nil
		}

		var receiptObj map[string]interface{}
		if err := client.CallContext(ctx, &receiptObj, "eth_getTransactionReceipt", txHashHex); err != nil {
			return false, fmt.Errorf("eth_getTransactionReceipt on %s: %w", node.Name, err)
		}
		if receiptObj == nil || receiptObj["blockHash"] != blockHash || receiptObj["blockNumber"] != blockNumber {
			return false, nil
		}

		var blockByHash map[string]interface{}
		if err := client.CallContext(ctx, &blockByHash, "eth_getBlockByHash", blockHash, false); err != nil {
			return false, fmt.Errorf("eth_getBlockByHash on %s: %w", node.Name, err)
		}
		if blockByHash == nil || blockByHash["hash"] != blockHash || blockByHash["number"] != blockNumber {
			return false, nil
		}

		var blockByNumber map[string]interface{}
		if err := client.CallContext(ctx, &blockByNumber, "eth_getBlockByNumber", blockNumber, false); err != nil {
			return false, fmt.Errorf("eth_getBlockByNumber on %s: %w", node.Name, err)
		}
		if blockByNumber == nil || blockByNumber["hash"] != blockHash || blockByNumber["number"] != blockNumber {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		t.Fatalf("lookup fixture did not propagate to %s (%s): %v", node.Name, node.URL, err)
	}
}

