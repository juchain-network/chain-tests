package forkcap

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	testctx "juchain.org/chain/tools/ci/internal/context"
)

const (
	DefaultTxTimeout      = 60 * time.Second
	DefaultCallGas uint64 = 3_000_000
)

func DefaultFallbackGasPrice() *big.Int {
	return big.NewInt(1_000_000_000)
}

type Harness struct {
	Ctx    *testctx.CIContext
	Client *ethclient.Client
}

func NewHarness(ctx *testctx.CIContext) (*Harness, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil CI context")
	}
	if len(ctx.Clients) == 0 || ctx.Clients[0] == nil {
		return nil, fmt.Errorf("no rpc client available")
	}
	return &Harness{Ctx: ctx, Client: ctx.Clients[0]}, nil
}

func (h *Harness) ChainID() *big.Int {
	if h == nil || h.Ctx == nil {
		return nil
	}
	return h.Ctx.ChainID
}

func (h *Harness) FunderKey() *ecdsa.PrivateKey {
	if h == nil || h.Ctx == nil {
		return nil
	}
	return h.Ctx.FunderKey
}

func (h *Harness) LatestHeader(ctx context.Context) (*types.Header, error) {
	if h == nil || h.Client == nil {
		return nil, fmt.Errorf("harness client not initialized")
	}
	return h.Client.HeaderByNumber(ctx, nil)
}

func (h *Harness) CallContract(ctx context.Context, msg ethereum.CallMsg) ([]byte, error) {
	if h == nil || h.Client == nil {
		return nil, fmt.Errorf("harness client not initialized")
	}
	return h.Client.CallContract(ctx, msg, nil)
}

func (h *Harness) PendingNonceAt(ctx context.Context, addr common.Address) (uint64, error) {
	if h == nil || h.Client == nil {
		return 0, fmt.Errorf("harness client not initialized")
	}
	return h.Client.PendingNonceAt(ctx, addr)
}

func (h *Harness) SuggestGasPrice(ctx context.Context) (*big.Int, error) {
	if h == nil || h.Client == nil {
		return nil, fmt.Errorf("harness client not initialized")
	}
	return h.Client.SuggestGasPrice(ctx)
}

func (h *Harness) SendSignedTransaction(ctx context.Context, tx *types.Transaction) error {
	if h == nil || h.Client == nil {
		return fmt.Errorf("harness client not initialized")
	}
	return h.Client.SendTransaction(ctx, tx)
}

func (h *Harness) WaitMined(txHash common.Hash) error {
	if h == nil || h.Ctx == nil {
		return fmt.Errorf("harness context not initialized")
	}
	return h.Ctx.WaitMined(txHash)
}

func (h *Harness) Receipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	if h == nil || h.Client == nil {
		return nil, fmt.Errorf("harness client not initialized")
	}
	return h.Client.TransactionReceipt(ctx, txHash)
}

func (h *Harness) NewLegacyTx(key *ecdsa.PrivateKey, to *common.Address, value *big.Int, gasLimit uint64, data []byte) (*types.Transaction, error) {
	if h == nil {
		return nil, fmt.Errorf("nil harness")
	}
	if key == nil {
		return nil, fmt.Errorf("nil signer key")
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	nonce, err := h.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	gasPrice, err := h.SuggestGasPrice(ctx)
	if err != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(1_000_000_000)
	}
	if value == nil {
		value = big.NewInt(0)
	}
	var tx *types.Transaction
	if to == nil {
		tx = types.NewContractCreation(nonce, value, gasLimit, gasPrice, data)
	} else {
		tx = types.NewTransaction(nonce, *to, value, gasLimit, gasPrice, data)
	}
	chainID := h.ChainID()
	if chainID == nil {
		return nil, fmt.Errorf("missing chain id")
	}
	return types.SignTx(tx, types.NewEIP155Signer(chainID), key)
}
