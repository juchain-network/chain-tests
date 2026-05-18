package rpc

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"

	"juchain.org/chain/tools/ci/contracts"
)

const readonlyValidatorsAddr = "0x000000000000000000000000000000000000f010"
const readonlyGetActiveValidatorsCallData = "0x9de70258"

func TestRPC_Readonly_BaselinePublicMethods(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	validatorsABI, err := contracts.ValidatorsMetaData.GetAbi()
	if err != nil {
		t.Fatalf("Failed to load Validators ABI: %v", err)
	}

	for _, node := range nodes {
		node := node
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()

			var latestBlockNumberHex string
			assertRawCall(t, node, &latestBlockNumberHex, "eth_blockNumber")
			latestBlockNumber := mustParseHexUint64(t, latestBlockNumberHex, "eth_blockNumber")

			var latestBlock map[string]interface{}
			assertRawCall(t, node, &latestBlock, "eth_getBlockByNumber", "latest", false)
			if latestBlock == nil {
				t.Fatalf("eth_getBlockByNumber(latest) returned nil")
			}

			latestBlockHash, _ := latestBlock["hash"].(string)
			if !has0xPrefix(latestBlockHash) {
				t.Fatalf("expected latest block hash to have 0x prefix, got %v", latestBlock["hash"])
			}
			if got := mustParseHexUint64(t, latestBlock["number"].(string), "eth_getBlockByNumber.number"); got != latestBlockNumber {
				t.Fatalf("expected latest block number %d from eth_getBlockByNumber, got %d", latestBlockNumber, got)
			}

			t.Run("eth_chainId", func(t *testing.T) {
				var chainIDHex string
				assertRawCall(t, node, &chainIDHex, "eth_chainId")
				chainID := mustParseHexUint64(t, chainIDHex, "eth_chainId")
				if chainID == 0 {
					t.Errorf("Expected non-zero chainID, got %d", chainID)
				}
			})

			t.Run("eth_blockNumber", func(t *testing.T) {
				if latestBlockNumber == 0 {
					t.Log("latest block number is zero; chain may be at genesis height")
				}
			})

			t.Run("eth_syncing", func(t *testing.T) {
				var syncing interface{}
				assertRawCall(t, node, &syncing, "eth_syncing")

				switch v := syncing.(type) {
				case bool:
					if v {
						t.Errorf("eth_syncing returned true boolean instead of object for syncing state")
					}
				case map[string]interface{}:
					expectedFields := []string{"startingBlock", "currentBlock", "highestBlock"}
					for _, field := range expectedFields {
						if _, ok := v[field]; !ok {
							t.Errorf("eth_syncing object missing field: %s", field)
						}
					}
				default:
					t.Errorf("eth_syncing returned unexpected type: %T", syncing)
				}
			})

			t.Run("eth_gasPrice", func(t *testing.T) {
				var gasPriceHex string
				assertRawCall(t, node, &gasPriceHex, "eth_gasPrice")
				_ = mustParseHexUint64(t, gasPriceHex, "eth_gasPrice")
			})

			t.Run("eth_getBlockByNumber_latest", func(t *testing.T) {
				if txs, ok := latestBlock["transactions"].([]interface{}); !ok {
					t.Fatalf("expected latest block transactions array, got %T", latestBlock["transactions"])
				} else {
					_ = txs
				}
			})

			t.Run("eth_getBlockByHash_latest", func(t *testing.T) {
				var blockByHash map[string]interface{}
				assertRawCall(t, node, &blockByHash, "eth_getBlockByHash", latestBlockHash, false)
				if blockByHash == nil {
					t.Fatalf("eth_getBlockByHash returned nil for latest hash %s", latestBlockHash)
				}
				if blockByHash["hash"] != latestBlockHash {
					t.Fatalf("expected block hash %s, got %v", latestBlockHash, blockByHash["hash"])
				}
			})

			t.Run("eth_getBlockTransactionCountByNumber_latest", func(t *testing.T) {
				var txCountHex string
				assertRawCall(t, node, &txCountHex, "eth_getBlockTransactionCountByNumber", "latest")
				_ = mustParseHexUint64(t, txCountHex, "eth_getBlockTransactionCountByNumber")
			})

			t.Run("eth_getBlockTransactionCountByHash_latest", func(t *testing.T) {
				var txCountHex string
				assertRawCall(t, node, &txCountHex, "eth_getBlockTransactionCountByHash", latestBlockHash)
				_ = mustParseHexUint64(t, txCountHex, "eth_getBlockTransactionCountByHash")
			})

			t.Run("eth_getBalance_zero_latest", func(t *testing.T) {
				var balanceHex string
				assertRawCall(t, node, &balanceHex, "eth_getBalance", "0x0000000000000000000000000000000000000000", "latest")
				_ = mustParseBigHex(t, balanceHex, "eth_getBalance")
			})

			t.Run("eth_getCode_system_contract_latest", func(t *testing.T) {
				var codeHex string
				assertRawCall(t, node, &codeHex, "eth_getCode", readonlyValidatorsAddr, "latest")
				if !has0xPrefix(codeHex) {
					t.Fatalf("expected eth_getCode result to have 0x prefix, got %s", codeHex)
				}
				if len(codeHex) <= 2 {
					t.Fatalf("expected non-empty runtime code at %s, got %s", readonlyValidatorsAddr, codeHex)
				}
			})

			t.Run("eth_call_getActiveValidators_latest", func(t *testing.T) {
				var callResult string
				callMsg := map[string]string{
					"to":   readonlyValidatorsAddr,
					"data": readonlyGetActiveValidatorsCallData,
				}
				assertRawCall(t, node, &callResult, "eth_call", callMsg, "latest")
				decodedBytes, err := hexutil.Decode(callResult)
				if err != nil {
					t.Fatalf("failed to decode eth_call result %s: %v", callResult, err)
				}
				unpacked, err := validatorsABI.Unpack("getActiveValidators", decodedBytes)
				if err != nil {
					t.Fatalf("failed to unpack getActiveValidators result: %v", err)
				}
				if len(unpacked) != 1 {
					t.Fatalf("expected 1 unpacked value, got %d", len(unpacked))
				}
			})

			t.Run("eth_getTransactionByHash_latest_block_first_tx_conditional", func(t *testing.T) {
				txHash := firstTxHashFromBlock(latestBlock)
				if txHash == "" {
					t.Skip("latest block has no transactions to inspect")
				}
				var txObj map[string]interface{}
				assertRawCall(t, node, &txObj, "eth_getTransactionByHash", txHash)
				if txObj == nil {
					t.Fatalf("eth_getTransactionByHash returned nil for %s", txHash)
				}
				if txObj["hash"] != txHash {
					t.Fatalf("expected tx hash %s, got %v", txHash, txObj["hash"])
				}
			})

			t.Run("eth_getTransactionReceipt_latest_block_first_tx_conditional", func(t *testing.T) {
				txHash := firstTxHashFromBlock(latestBlock)
				if txHash == "" {
					t.Skip("latest block has no transactions to inspect")
				}
				var receiptObj map[string]interface{}
				assertRawCall(t, node, &receiptObj, "eth_getTransactionReceipt", txHash)
				if receiptObj == nil {
					t.Fatalf("eth_getTransactionReceipt returned nil for %s", txHash)
				}
				if receiptObj["transactionHash"] != txHash {
					t.Fatalf("expected tx hash %s, got %v", txHash, receiptObj["transactionHash"])
				}
			})
		})
	}
}

func has0xPrefix(value string) bool {
	return strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X")
}

func mustParseHexUint64(t *testing.T, value string, field string) uint64 {
	t.Helper()
	if !has0xPrefix(value) {
		t.Fatalf("expected %s to have 0x prefix, got %s", field, value)
	}
	parsed, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(value, "0x"), "0X"), 16, 64)
	if err != nil {
		t.Fatalf("failed to parse %s value %s: %v", field, value, err)
	}
	return parsed
}

func mustParseBigHex(t *testing.T, value string, field string) string {
	t.Helper()
	if !has0xPrefix(value) {
		t.Fatalf("expected %s to have 0x prefix, got %s", field, value)
	}
	if _, err := hexutil.DecodeBig(value); err != nil {
		t.Fatalf("failed to parse %s value %s: %v", field, value, err)
	}
	return value
}

func TestRPC_Readonly_EthNamespaceOptional(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	for _, node := range nodes {
		node := node
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()

			t.Run("eth_protocolVersion", func(t *testing.T) {
				var protocolVersion string
				if err := callReadonly(t, node, &protocolVersion, "eth_protocolVersion"); err != nil {
					skipIfUnsupported(t, err, node, "eth_protocolVersion")
					return
				}
				if protocolVersion == "" {
					t.Fatalf("expected non-empty eth_protocolVersion on %s", node.Name)
				}
			})

			t.Run("eth_maxPriorityFeePerGas", func(t *testing.T) {
				var feeHex string
				if err := callReadonly(t, node, &feeHex, "eth_maxPriorityFeePerGas"); err != nil {
					skipIfUnsupported(t, err, node, "eth_maxPriorityFeePerGas")
					return
				}
				_ = mustParseHexUint64(t, feeHex, "eth_maxPriorityFeePerGas")
			})

			t.Run("eth_feeHistory", func(t *testing.T) {
				var feeHistory map[string]interface{}
				if err := callReadonly(t, node, &feeHistory, "eth_feeHistory", "0x1", "latest", []interface{}{}); err != nil {
					skipIfUnsupported(t, err, node, "eth_feeHistory")
					return
				}
				if feeHistory == nil {
					t.Fatalf("expected eth_feeHistory result on %s", node.Name)
				}
			})

			t.Run("eth_estimateGas", func(t *testing.T) {
				var gasHex string
				msg := map[string]string{
					"from": "0x0000000000000000000000000000000000000000",
					"to":   "0x0000000000000000000000000000000000000000",
					"data": "0x",
				}
				if err := callReadonly(t, node, &gasHex, "eth_estimateGas", msg); err != nil {
					skipIfUnsupported(t, err, node, "eth_estimateGas")
					return
				}
				_ = mustParseHexUint64(t, gasHex, "eth_estimateGas")
			})

			t.Run("eth_getStorageAt_system_contract", func(t *testing.T) {
				var storageHex string
				if err := callReadonly(t, node, &storageHex, "eth_getStorageAt", readonlyValidatorsAddr, "0x0", "latest"); err != nil {
					skipIfUnsupported(t, err, node, "eth_getStorageAt")
					return
				}
				if !has0xPrefix(storageHex) {
					t.Fatalf("expected eth_getStorageAt to return hex on %s, got %s", node.Name, storageHex)
				}
			})

			t.Run("eth_accounts", func(t *testing.T) {
				var accounts []string
				if err := callReadonly(t, node, &accounts, "eth_accounts"); err != nil {
					skipIfUnsupported(t, err, node, "eth_accounts")
					return
				}
				for _, account := range accounts {
					if !has0xPrefix(account) {
						t.Fatalf("expected eth_accounts entry to be hex on %s, got %s", node.Name, account)
					}
				}
			})
		})
	}
}

func callReadonly(t *testing.T, node RPCNode, result interface{}, method string, args ...interface{}) error {
	t.Helper()
	client := dialRPC(t, node)
	defer client.Close()
	return client.CallContext(context.Background(), result, method, args...)
}

func firstTxHashFromBlock(block map[string]interface{}) string {
	txs, ok := block["transactions"].([]interface{})
	if !ok || len(txs) == 0 {
		return ""
	}
	first, ok := txs[0].(string)
	if !ok || !has0xPrefix(first) {
		return ""
	}
	return first
}

func skipIfUnsupported(t *testing.T, err error, node RPCNode, method string) {
	t.Helper()
	if isMethodUnavailableError(err) {
		t.Skipf("%s not exposed on %s: %v", method, node.Name, err)
		return
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		t.Skipf("%s not supported on %s: %v", method, node.Name, err)
		return
	}
	t.Fatalf("%s failed on %s: %v", method, node.Name, err)
}
