package forkcap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"

	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/config"
	basekit "juchain.org/chain/tools/ci/internal/testkit"
)

const (
	BPO1BlobTarget                = uint64(10)
	BPO1BlobMax                   = uint64(15)
	BPO1BlobBaseFeeUpdateFraction = uint64(8346193)
	BPO2BlobTarget                = uint64(14)
	BPO2BlobMax                   = uint64(21)
	BPO2BlobBaseFeeUpdateFraction = uint64(11684671)
)

func RuntimeImpl(cfg *config.Config) string {
	return basekit.RuntimeImplForNode(cfg, 0)
}

func CheckForkRPCSurface(cfg *config.Config, rpcURL string) error {
	return basekit.VerifyForkRPCSurface(cfg, rpcURL, RuntimeImpl(cfg))
}

func CheckFixHeaderSurface(cfg *config.Config, rpcURL string) error {
	return CheckForkRPCSurface(cfg, rpcURL)
}

func CheckBPOBlobSchedule(cfg *config.Config, rpcURL string, fork string) error {
	block, err := LatestBlockFieldMap(rpcURL)
	if err != nil {
		return fmt.Errorf("read latest block for %s blob schedule: %w", fork, err)
	}
	if !FieldPresent(block, "blobGasUsed") || !FieldPresent(block, "excessBlobGas") {
		return fmt.Errorf("missing Cancun+ blob fields while checking %s blob schedule", fork)
	}
	switch NormalizeFork(fork) {
	case "bpo1":
		return basekit.VerifyBlobScheduleForDebug(rpcURL, BPO1BlobTarget, BPO1BlobMax, BPO1BlobBaseFeeUpdateFraction)
	case "bpo2":
		return basekit.VerifyBlobScheduleForDebug(rpcURL, BPO2BlobTarget, BPO2BlobMax, BPO2BlobBaseFeeUpdateFraction)
	default:
		return fmt.Errorf("unsupported bpo blob schedule fork %q", fork)
	}
}

func CheckPosaContractSurface(h *Harness, shouldFail bool) error {
	if h == nil || h.Client == nil {
		return fmt.Errorf("nil harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addresses := map[string]common.Address{
		"validators": testctx.ValidatorsAddr,
		"punish":     testctx.PunishAddr,
		"proposal":   testctx.ProposalAddr,
		"staking":    testctx.StakingAddr,
	}
	for name, addr := range addresses {
		code, err := h.Client.CodeAt(ctx, addr, nil)
		if err != nil {
			return fmt.Errorf("read %s code at %s: %w", name, addr.Hex(), err)
		}
		if shouldFail {
			if len(code) != 0 {
				return fmt.Errorf("expected pre-PoSA %s contract at %s to be absent, got code len=%d", name, addr.Hex(), len(code))
			}
			continue
		}
		if len(code) == 0 {
			return fmt.Errorf("expected post-PoSA %s contract at %s to be deployed", name, addr.Hex())
		}
	}
	return nil
}

func CheckOsakaTxGasCap(h *Harness) error {
	if h == nil {
		return fmt.Errorf("nil harness")
	}
	return basekit.VerifyOsakaTxGasCap(h.Client, h.FunderKey(), h.ChainID())
}

func CheckOsakaTxGasCapInactive(h *Harness) error {
	if h == nil {
		return fmt.Errorf("nil harness")
	}
	key := h.FunderKey()
	if key == nil {
		return fmt.Errorf("missing funded signer key")
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	tx, err := h.NewLegacyTx(key, &from, nil, basekit.OsakaMaxTxGas+1, nil)
	if err != nil {
		return fmt.Errorf("build pre-Osaka oversized tx: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	if err := h.SendSignedTransaction(ctx, tx); err != nil {
		return fmt.Errorf("expected oversized tx to remain valid pre-Osaka, got %w", err)
	}
	if err := h.WaitMined(tx.Hash()); err != nil {
		return fmt.Errorf("wait pre-Osaka oversized tx mined: %w", err)
	}
	receipt, err := h.Receipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("read pre-Osaka oversized tx receipt: %w", err)
	}
	if receipt == nil {
		return fmt.Errorf("nil receipt for pre-Osaka oversized tx %s", tx.Hash().Hex())
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("pre-Osaka oversized tx failed: hash=%s status=%d to=%s", tx.Hash().Hex(), receipt.Status, from.Hex())
	}
	return nil
}

func LatestBlockFieldMap(rpcURL string) (map[string]any, error) {
	rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
	if err != nil {
		return nil, err
	}
	defer rpcClient.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var block map[string]any
	if err := rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", "latest", false); err != nil {
		return nil, err
	}
	if block == nil {
		return nil, fmt.Errorf("latest block response is nil")
	}
	return block, nil
}

func FieldPresent(block map[string]any, field string) bool {
	value, exists := block[field]
	if !exists || value == nil {
		return false
	}
	if raw, ok := value.(string); ok {
		trimmed := strings.TrimSpace(raw)
		return trimmed != "" && !strings.EqualFold(trimmed, "null")
	}
	return true
}
