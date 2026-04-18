package testkit

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"juchain.org/chain/tools/ci/internal/config"
)

const (
	CancunBlobTarget                = uint64(3)
	CancunBlobMax                   = uint64(6)
	CancunBlobBaseFeeUpdateFraction = uint64(3338477)
	PragueBlobTarget                = uint64(6)
	PragueBlobMax                   = uint64(9)
	PragueBlobBaseFeeUpdateFraction = uint64(5007716)
	BPO1BlobTarget                  = uint64(10)
	BPO1BlobMax                     = uint64(15)
	BPO1BlobBaseFeeUpdateFraction   = uint64(8346193)
	BPO2BlobTarget                  = uint64(14)
	BPO2BlobMax                     = uint64(21)
	BPO2BlobBaseFeeUpdateFraction   = uint64(11684671)
	OsakaMaxTxGas                   = uint64(1 << 24)
)

type rpcBlobSchedule struct {
	Target                uint64 `json:"target"`
	Max                   uint64 `json:"max"`
	BaseFeeUpdateFraction uint64 `json:"baseFeeUpdateFraction"`
}

type rpcForkConfig struct {
	ActivationTime uint64           `json:"activationTime"`
	BlobSchedule   *rpcBlobSchedule `json:"blobSchedule"`
}

type rpcConfigResponse struct {
	Current *rpcForkConfig `json:"current"`
	Next    *rpcForkConfig `json:"next"`
	Last    *rpcForkConfig `json:"last"`
}

func RuntimeImplForNode(cfg *config.Config, index int) string {
	if cfg == nil {
		return "geth"
	}
	if index >= 0 && index < len(cfg.RuntimeNodes) {
		if impl := strings.ToLower(strings.TrimSpace(cfg.RuntimeNodes[index].Impl)); impl != "" {
			return impl
		}
	}
	if impl := strings.ToLower(strings.TrimSpace(cfg.Runtime.Impl)); impl != "" {
		return impl
	}
	return "geth"
}

func VerifyForkRPCSurface(cfg *config.Config, rpcURL string, runtimeImpl string) error {
	block, timestamp, err := latestBlockRPC(rpcURL)
	if err != nil {
		return err
	}

	expectCancun := forkIsActive(cfg, "cancun", timestamp)
	expectFixHeader := forkIsActive(cfg, "fixheader", timestamp)
	expectPrague := forkIsActive(cfg, "prague", timestamp)
	expectOsaka := forkIsActive(cfg, "osaka", timestamp)
	expectBPO1 := forkIsActive(cfg, "bpo1", timestamp)
	expectBPO2 := forkIsActive(cfg, "bpo2", timestamp)

	if expectCancun {
		if err := requireRPCStringField(block, "blobGasUsed"); err != nil {
			return err
		}
		if err := requireRPCStringField(block, "excessBlobGas"); err != nil {
			return err
		}
	}
	if expectFixHeader {
		if err := requireZeroHashField(block, "parentBeaconBlockRoot"); err != nil {
			return err
		}
	}
	if expectPrague {
		if err := requireRPCStringField(block, "requestsHash"); err != nil {
			return err
		}
	}

	if strings.EqualFold(runtimeImpl, "geth") {
		switch {
		case expectBPO2:
			if err := verifyEthConfigBlobSchedule(rpcURL, BPO2BlobTarget, BPO2BlobMax, BPO2BlobBaseFeeUpdateFraction); err != nil {
				return err
			}
		case expectBPO1:
			if err := verifyEthConfigBlobSchedule(rpcURL, BPO1BlobTarget, BPO1BlobMax, BPO1BlobBaseFeeUpdateFraction); err != nil {
				return err
			}
		case expectOsaka:
			if err := verifyEthConfigBlobSchedule(rpcURL, PragueBlobTarget, PragueBlobMax, PragueBlobBaseFeeUpdateFraction); err != nil {
				return err
			}
		case expectPrague:
			if err := verifyEthConfigBlobSchedule(rpcURL, PragueBlobTarget, PragueBlobMax, PragueBlobBaseFeeUpdateFraction); err != nil {
				return err
			}
		case expectCancun:
			if err := verifyEthConfigBlobSchedule(rpcURL, CancunBlobTarget, CancunBlobMax, CancunBlobBaseFeeUpdateFraction); err != nil {
				return err
			}
		}
	}

	return nil
}

func VerifyBlobScheduleForDebug(rpcURL string, wantTarget, wantMax, wantBaseFeeUpdateFraction uint64) error {
	return verifyEthConfigBlobSchedule(rpcURL, wantTarget, wantMax, wantBaseFeeUpdateFraction)
}

func VerifyOsakaTxGasCap(client *ethclient.Client, key *ecdsa.PrivateKey, chainID *big.Int) error {
	if client == nil || key == nil || chainID == nil {
		return fmt.Errorf("osaka gas-cap check requires client, signer key, and chain id")
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		return fmt.Errorf("read nonce for osaka gas-cap check failed: %w", err)
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(1_000_000_000)
	}
	tx := types.NewTransaction(nonce, from, big.NewInt(0), OsakaMaxTxGas+1, gasPrice, nil)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
	if err != nil {
		return fmt.Errorf("sign osaka gas-cap tx failed: %w", err)
	}
	if err := client.SendTransaction(context.Background(), signed); err == nil {
		return fmt.Errorf("osaka gas-cap tx unexpectedly accepted: gas=%d", OsakaMaxTxGas+1)
	} else if !isGasCapError(err) {
		return fmt.Errorf("unexpected osaka gas-cap error: %w", err)
	}
	return nil
}

func latestBlockRPC(rpcURL string) (map[string]any, uint64, error) {
	rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
	if err != nil {
		return nil, 0, fmt.Errorf("dial rpc %s failed: %w", rpcURL, err)
	}
	defer rpcClient.Close()

	var block map[string]any
	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = rpcClient.CallContext(callCtx, &block, "eth_getBlockByNumber", "latest", false)
	cancel()
	if err != nil {
		return nil, 0, fmt.Errorf("eth_getBlockByNumber(latest) failed: %w", err)
	}
	if block == nil {
		return nil, 0, fmt.Errorf("latest block response is nil")
	}

	tsRaw, _ := block["timestamp"].(string)
	timestamp, err := parseUint64Hex(tsRaw)
	if err != nil {
		return nil, 0, fmt.Errorf("parse latest block timestamp failed: %w", err)
	}
	return block, timestamp, nil
}

func verifyEthConfigBlobSchedule(rpcURL string, wantTarget, wantMax, wantBaseFeeUpdateFraction uint64) error {
	rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
	if err != nil {
		return fmt.Errorf("dial rpc %s for eth_config failed: %w", rpcURL, err)
	}
	defer rpcClient.Close()

	var resp rpcConfigResponse
	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = rpcClient.CallContext(callCtx, &resp, "eth_config")
	cancel()
	if err != nil {
		return fmt.Errorf("eth_config failed: %w", err)
	}
	if resp.Current == nil || resp.Current.BlobSchedule == nil {
		return fmt.Errorf("eth_config current blob schedule is missing")
	}
	current := resp.Current.BlobSchedule
	if current.Target != wantTarget || current.Max != wantMax || current.BaseFeeUpdateFraction != wantBaseFeeUpdateFraction {
		return fmt.Errorf(
			"eth_config current blob schedule mismatch: have[target=%d max=%d baseFeeUpdateFraction=%d] want[target=%d max=%d baseFeeUpdateFraction=%d]",
			current.Target,
			current.Max,
			current.BaseFeeUpdateFraction,
			wantTarget,
			wantMax,
			wantBaseFeeUpdateFraction,
		)
	}
	return nil
}

func forkIsActive(cfg *config.Config, forkName string, timestamp uint64) bool {
	if cfg == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Fork.Mode)) {
	case "smoke":
		target := strings.ToLower(strings.TrimSpace(cfg.Fork.Target))
		switch forkName {
		case "cancun":
			return strings.Contains(target, "cancun")
		case "fixheader":
			return strings.Contains(target, "fixheader")
		case "posa":
			return strings.Contains(target, "posa")
		case "prague":
			return strings.Contains(target, "prague")
		case "osaka":
			return strings.Contains(target, "osaka")
		case "bpo1":
			return strings.Contains(target, "bpo1")
		case "bpo2":
			return strings.Contains(target, "bpo2")
		}
	}

	var at int64
	switch forkName {
	case "cancun":
		at = cfg.Fork.Schedule.CancunTime
	case "fixheader":
		at = cfg.Fork.Schedule.FixHeaderTime
	case "posa":
		at = cfg.Fork.Schedule.PosaTime
	case "prague":
		at = cfg.Fork.Schedule.PragueTime
	case "osaka":
		at = cfg.Fork.Schedule.OsakaTime
	case "bpo1":
		at = cfg.Fork.Schedule.BPO1Time
	case "bpo2":
		at = cfg.Fork.Schedule.BPO2Time
	default:
		return false
	}
	return at > 0 && int64(timestamp) >= at
}

func requireRPCStringField(block map[string]any, field string) error {
	value, exists := block[field]
	if !exists || value == nil {
		return fmt.Errorf("missing field %s in latest block", field)
	}
	raw, _ := value.(string)
	if strings.TrimSpace(raw) == "" || strings.EqualFold(raw, "null") {
		return fmt.Errorf("empty field %s in latest block", field)
	}
	return nil
}

func requireZeroHashField(block map[string]any, field string) error {
	if err := requireRPCStringField(block, field); err != nil {
		return err
	}
	raw, _ := block[field].(string)
	if !strings.EqualFold(raw, common.Hash{}.Hex()) {
		return fmt.Errorf("invalid %s post-activation: have=%s", field, raw)
	}
	return nil
}

func parseUint64Hex(raw string) (uint64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("empty hex string")
	}
	value = strings.TrimPrefix(strings.ToLower(value), "0x")
	if value == "" {
		return 0, nil
	}
	parsed, ok := new(big.Int).SetString(value, 16)
	if !ok || parsed == nil {
		return 0, fmt.Errorf("invalid hex string: %s", raw)
	}
	return parsed.Uint64(), nil
}

func isGasCapError(err error) bool {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "gas limit too high") ||
		strings.Contains(msg, "tx gas too high") ||
		strings.Contains(msg, "exceeds maximum") ||
		strings.Contains(msg, "max tx gas")
}
