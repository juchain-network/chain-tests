package rpc

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"juchain.org/chain/tools/ci/contracts"
	cicontext "juchain.org/chain/tools/ci/internal/context"
)

func TestRPC_NegativeMethods(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	t.Run("unknown_method_returns_method_not_found", func(t *testing.T) {
		node := nodes[0]
		client := dialRPC(t, node)
		defer client.Close()

		var result any
		err := client.CallContext(context.Background(), &result, "rpc_notARealMethod")
		if err == nil {
			t.Fatalf("expected unknown RPC method to fail on %s", RPCDiagnostic{NodeName: node.Name, Role: node.Role, URL: node.URL, Method: "rpc_notARealMethod"})
		}
		assertErrorContainsAny(t, err, "method not found", "does not exist", "not available")
	})

	t.Run("forbidden_system_transaction_rejected", func(t *testing.T) {
		ciCtx, err := cicontext.NewCIContext(rpcCfg)
		if err != nil {
			t.Fatalf("Failed to create CI context: %v", err)
		}
		if len(ciCtx.Clients) == 0 {
			t.Fatalf("No RPC clients configured")
		}

		senderKey := ciCtx.FunderKey
		if senderKey == nil && len(ciCtx.GenesisValidators) > 0 {
			senderKey = ciCtx.GenesisValidators[0]
		}
		if senderKey == nil {
			t.Fatalf("No funded key available for forbidden system transaction test")
		}

		data := packMethodData(t, contracts.PunishMetaData, "executePending", big.NewInt(1))
		err = sendForbiddenSystemTx(ciCtx, senderKey, cicontext.PunishAddr, data)
		if err == nil {
			t.Fatalf("expected forbidden system transaction to be rejected")
		}
		assertErrorContainsAny(t, err, "forbidden system transaction", "miner only")
	})
}

func sendForbiddenSystemTx(ciCtx *cicontext.CIContext, key *ecdsa.PrivateKey, to common.Address, data []byte) error {
	from := crypto.PubkeyToAddress(key.PublicKey)
	ciCtx.RefreshNonce(from)

	nonce, err := ciCtx.Clients[0].PendingNonceAt(context.Background(), from)
	if err != nil {
		return err
	}
	gasPrice, err := ciCtx.Clients[0].SuggestGasPrice(context.Background())
	if err != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(1_000_000_000)
	}

	tx := types.NewTransaction(nonce, to, big.NewInt(0), 500_000, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(ciCtx.ChainID), key)
	if err != nil {
		return err
	}
	return ciCtx.Clients[0].SendTransaction(context.Background(), signedTx)
}

func packMethodData(t *testing.T, meta interface{ GetAbi() (*abi.ABI, error) }, method string, args ...interface{}) []byte {
	t.Helper()
	contractABI, err := meta.GetAbi()
	if err != nil {
		t.Fatalf("failed to load ABI for %s: %v", method, err)
	}
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		t.Fatalf("failed to pack %s: %v", method, err)
	}
	return data
}

func assertErrorContainsAny(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing one of %v, got nil", fragments)
	}
	msg := strings.ToLower(err.Error())
	for _, fragment := range fragments {
		if strings.Contains(msg, strings.ToLower(fragment)) {
			return
		}
	}
	t.Fatalf("expected error containing one of %v, got %v", fragments, err)
}
