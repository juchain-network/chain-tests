package rpc

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"

	cicontext "juchain.org/chain/tools/ci/internal/context"
)

type rpcFixture struct {
	TxHashHex          string
	BlockHash          string
	BlockNumberHex     string
	ExpectedChainID    string
	ExpectedNetVersion string
	ExpectedTx         map[string]any
	ExpectedReceipt    map[string]any
	ExpectedBlockHash  map[string]any
	ExpectedBlockNum   map[string]any
	ExpectedCallResult string
	CallMsg            map[string]string
}

func TestRPC_CrossNodeConsistency(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	topology := getRPCTopology()
	fixture := prepareRPCFixture(t)

	t.Run("chain_identity_converges", func(t *testing.T) {
		assertFixtureBackedEquality(t, nodes, "eth_chainId", nil, fixture.ExpectedChainID, "chain id equality across all discovered rpc nodes")
		assertFixtureBackedEquality(t, nodes, "net_version", nil, fixture.ExpectedNetVersion, "network id equality across all discovered rpc nodes")
	})

	t.Run("lookup_semantics_converge", func(t *testing.T) {
		assertFixtureBackedEquality(t, nodes, "eth_getTransactionByHash", []interface{}{fixture.TxHashHex}, fixture.ExpectedTx, "fixture transaction lookup equality across all discovered rpc nodes")
		assertFixtureBackedEquality(t, nodes, "eth_getTransactionReceipt", []interface{}{fixture.TxHashHex}, fixture.ExpectedReceipt, "fixture receipt lookup equality across all discovered rpc nodes")
		assertFixtureBackedEquality(t, nodes, "eth_getBlockByHash", []interface{}{fixture.BlockHash, false}, fixture.ExpectedBlockHash, "fixture block-by-hash equality across all discovered rpc nodes")
		assertFixtureBackedEquality(t, nodes, "eth_getBlockByNumber", []interface{}{fixture.BlockNumberHex, false}, fixture.ExpectedBlockNum, "fixture block-by-number equality across all discovered rpc nodes")
	})

	t.Run("stable_contract_read_truth_converges", func(t *testing.T) {
		assertFixtureBackedEquality(t, nodes, "eth_call", []interface{}{fixture.CallMsg, "latest"}, fixture.ExpectedCallResult, "stable eth_call truth equality across all discovered rpc nodes")
	})

	t.Run("role_aware_expectations", func(t *testing.T) {
		t.Run("eth_syncing", func(t *testing.T) {
			expectation := ethSyncingExpectation()
			if len(topology.Sync) == 0 {
				expectation.AllowRolesToBeUnmatched = true
			}
			assertRoleAwareConvergence(
				t,
				nodes,
				expectation,
				func(client *rpc.Client, _ RPCNode) (any, error) {
					var result any
					err := client.CallContext(context.Background(), &result, "eth_syncing")
					return result, err
				},
				normalizeSyncingObservation,
			)
		})

		t.Run("net_peerCount", func(t *testing.T) {
			expectation := netPeerCountExpectation(topology)
			if len(topology.Sync) == 0 {
				expectation.AllowRolesToBeUnmatched = true
			}
			assertRoleAwareConvergence(
				t,
				nodes,
				expectation,
				func(client *rpc.Client, _ RPCNode) (any, error) {
					var result any
					err := client.CallContext(context.Background(), &result, "net_peerCount")
					return result, err
				},
				normalizePeerCountObservation,
			)
		})

		t.Run("eth_coinbase", func(t *testing.T) {
			expectation := ethCoinbaseExpectation()
			if len(topology.Sync) == 0 {
				expectation.AllowRolesToBeUnmatched = true
			}
			assertRoleAwareConvergence(
				t,
				nodes,
				expectation,
				func(client *rpc.Client, _ RPCNode) (any, error) {
					var result string
					err := client.CallContext(context.Background(), &result, "eth_coinbase")
					return result, err
				},
				normalizeCoinbaseObservation,
			)
		})
	})
}

func prepareRPCFixture(t *testing.T) rpcFixture {
	t.Helper()

	ciCtx, err := cicontext.NewCIContext(rpcCfg)
	if err != nil {
		t.Fatalf("Failed to create CI context: %v", err)
	}
	primaryClient := ciCtx.Clients[0]
	primaryNode := getAllNodes()[0]
	primaryRPC := dialRPC(t, primaryNode)
	defer primaryRPC.Close()

	dummyAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	opts, err := ciCtx.GetTransactorEx(ciCtx.FunderKey, true)
	if err != nil {
		t.Fatalf("Failed to get transactor for funder: %v", err)
	}

	gasPrice := opts.GasPrice
	if gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(1000000000)
	}

	tx := types.NewTransaction(opts.Nonce.Uint64(), dummyAddr, big.NewInt(0), 21000, gasPrice, nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(ciCtx.ChainID), ciCtx.FunderKey)
	if err != nil {
		t.Fatalf("Failed to sign tx: %v", err)
	}
	if err := primaryClient.SendTransaction(context.Background(), signedTx); err != nil {
		t.Fatalf("Failed to send tx: %v", err)
	}

	txHash := signedTx.Hash()
	if err := ciCtx.WaitMined(txHash); err != nil {
		t.Fatalf("Failed waiting for tx to be mined: %v", err)
	}

	receipt, err := primaryClient.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		t.Fatalf("Failed to get receipt from primary client: %v", err)
	}

	fixture := rpcFixture{
		TxHashHex:      txHash.Hex(),
		BlockHash:      receipt.BlockHash.Hex(),
		BlockNumberHex: hexutil.EncodeUint64(receipt.BlockNumber.Uint64()),
		CallMsg: map[string]string{
			"to":   "0x000000000000000000000000000000000000f010",
			"data": "0x9de70258",
		},
	}

	assertRawCall(t, primaryNode, &fixture.ExpectedChainID, "eth_chainId")
	assertRawCall(t, primaryNode, &fixture.ExpectedNetVersion, "net_version")
	assertRawCall(t, primaryNode, &fixture.ExpectedTx, "eth_getTransactionByHash", fixture.TxHashHex)
	assertRawCall(t, primaryNode, &fixture.ExpectedReceipt, "eth_getTransactionReceipt", fixture.TxHashHex)
	assertRawCall(t, primaryNode, &fixture.ExpectedBlockHash, "eth_getBlockByHash", fixture.BlockHash, false)
	assertRawCall(t, primaryNode, &fixture.ExpectedBlockNum, "eth_getBlockByNumber", fixture.BlockNumberHex, false)
	assertRawCall(t, primaryNode, &fixture.ExpectedCallResult, "eth_call", fixture.CallMsg, "latest")

	return fixture
}
