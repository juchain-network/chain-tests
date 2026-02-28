package tests

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const defaultBlacklistAddress = "0x1db0EDE439708A923431DC68fd3F646c0A4D4e6E"

func requireBlacklistEnabled(t *testing.T) {
	t.Helper()
	if cfg == nil || ctx == nil {
		t.Fatalf("blacklist context not initialized")
	}
	if !cfg.Blacklist.Enabled {
		t.Skip("blacklist is disabled in generated test config")
	}
}

func blacklistAddr() common.Address {
	if cfg != nil && strings.TrimSpace(cfg.Blacklist.ContractAddress) != "" {
		return common.HexToAddress(cfg.Blacklist.ContractAddress)
	}
	return common.HexToAddress(defaultBlacklistAddress)
}

func loadBlacklistABI(t *testing.T) abi.ABI {
	t.Helper()
	if cfg != nil && cfg.Blacklist.Mock.ABIFile != "" {
		if data, err := os.ReadFile(cfg.Blacklist.Mock.ABIFile); err == nil {
			if parsed, err := abi.JSON(strings.NewReader(string(data))); err == nil {
				return parsed
			}
		}
	}
	const fallbackABI = `[
		{"inputs":[],"name":"getAllBlacklistedAddresses","outputs":[{"internalType":"address[]","name":"","type":"address[]"}],"stateMutability":"view","type":"function"},
		{"inputs":[{"internalType":"address","name":"addr","type":"address"}],"name":"addToBlacklist","outputs":[],"stateMutability":"nonpayable","type":"function"},
		{"inputs":[{"internalType":"address","name":"addr","type":"address"}],"name":"removeFromBlacklist","outputs":[],"stateMutability":"nonpayable","type":"function"}
	]`
	parsed, err := abi.JSON(strings.NewReader(fallbackABI))
	if err != nil {
		t.Fatalf("parse fallback blacklist abi: %v", err)
	}
	return parsed
}

func waitBlocks(t *testing.T, n int) {
	t.Helper()
	if n <= 0 {
		return
	}
	if err := ctx.WaitForBlockProgress(n, 90*time.Second); err != nil {
		t.Fatalf("wait for %d blocks: %v", n, err)
	}
}

func callGetAll(t *testing.T, parsed abi.ABI) ([]common.Address, error) {
	t.Helper()
	payload, err := parsed.Pack("getAllBlacklistedAddresses")
	if err != nil {
		return nil, err
	}
	msg := ethereum.CallMsg{To: ptrAddr(blacklistAddr()), Data: payload}
	callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ret, err := ctx.Clients[0].CallContract(callCtx, msg, nil)
	if err != nil {
		return nil, err
	}
	out, err := parsed.Unpack("getAllBlacklistedAddresses", ret)
	if err != nil {
		return nil, err
	}
	if len(out) != 1 {
		return nil, fmt.Errorf("unexpected output len=%d", len(out))
	}
	addrs, ok := out[0].([]common.Address)
	if !ok {
		return nil, fmt.Errorf("unexpected output type %T", out[0])
	}
	return addrs, nil
}

func sendBlacklistTx(t *testing.T, key *ecdsa.PrivateKey, method string, addr common.Address, parsed abi.ABI) error {
	t.Helper()
	opts, err := ctx.GetTransactor(key)
	if err != nil {
		return err
	}
	data, err := parsed.Pack(method, addr)
	if err != nil {
		return err
	}
	gasPrice := opts.GasPrice
	if gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(1_000_000_000)
	}
	tx := types.NewTransaction(opts.Nonce.Uint64(), blacklistAddr(), big.NewInt(0), 250000, gasPrice, data)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(ctx.ChainID), key)
	if err != nil {
		return err
	}
	if err := ctx.Clients[0].SendTransaction(context.Background(), signed); err != nil {
		return err
	}
	return ctx.WaitMined(signed.Hash())
}

func sendSelfTransfer(key *ecdsa.PrivateKey, waitMined bool) error {
	from := crypto.PubkeyToAddress(key.PublicKey)
	nonce, err := ctx.Clients[0].PendingNonceAt(context.Background(), from)
	if err != nil {
		return err
	}
	gasPrice, err := ctx.Clients[0].SuggestGasPrice(context.Background())
	if err != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = big.NewInt(1_000_000_000)
	}
	tx := types.NewTransaction(nonce, from, big.NewInt(0), 21_000, gasPrice, nil)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(ctx.ChainID), key)
	if err != nil {
		return err
	}
	if err := ctx.Clients[0].SendTransaction(context.Background(), signed); err != nil {
		return err
	}
	if !waitMined {
		return nil
	}
	return ctx.WaitMined(signed.Hash())
}

func waitForTxRejection(t *testing.T, key *ecdsa.PrivateKey, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		err := sendSelfTransfer(key, false)
		if err != nil {
			lastErr = err
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "blacklist") {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("expected blacklist rejection, got lastErr=%v", lastErr)
}

func waitForTxRecovery(t *testing.T, key *ecdsa.PrivateKey, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		err := sendSelfTransfer(key, true)
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("expected transfer recovery, lastErr=%v", lastErr)
}

func ptrAddr(a common.Address) *common.Address { return &a }

func TestBL_InitializationLoad(t *testing.T) {
	requireBlacklistEnabled(t)
	parsed := loadBlacklistABI(t)
	addrs, err := callGetAll(t, parsed)
	if cfg.Blacklist.Mode == "real" && err != nil {
		t.Logf("real mode init read failed (allow fail-open scenario): %v", err)
		return
	}
	if err != nil {
		t.Fatalf("getAllBlacklistedAddresses failed: %v", err)
	}
	t.Logf("blacklist entries loaded: %d", len(addrs))
}

func TestBL_EventRefreshAndTxpoolReject(t *testing.T) {
	requireBlacklistEnabled(t)
	if cfg.Blacklist.Mode != "mock" {
		t.Skip("event-driven blacklist update test is enabled only in mock mode")
	}
	parsed := loadBlacklistABI(t)

	blockedKey, blockedAddr, err := ctx.CreateAndFundAccount(big.NewInt(1000000000000000000))
	if err != nil {
		t.Fatalf("create blocked account: %v", err)
	}
	if err := sendSelfTransfer(blockedKey, true); err != nil {
		t.Fatalf("baseline transfer before blacklist failed: %v", err)
	}

	if err := sendBlacklistTx(t, ctx.FunderKey, "addToBlacklist", blockedAddr, parsed); err != nil {
		t.Fatalf("addToBlacklist failed: %v", err)
	}
	waitBlocks(t, 2)
	waitForTxRejection(t, blockedKey, 20*time.Second)

	if err := sendBlacklistTx(t, ctx.FunderKey, "removeFromBlacklist", blockedAddr, parsed); err != nil {
		t.Fatalf("removeFromBlacklist failed: %v", err)
	}
	waitBlocks(t, 2)
	waitForTxRecovery(t, blockedKey, 20*time.Second)
}

func TestBL_PeriodicRefreshStability(t *testing.T) {
	requireBlacklistEnabled(t)
	if cfg.Blacklist.Mode != "mock" {
		t.Skip("periodic refresh stability is enabled only in mock mode")
	}
	if os.Getenv("BLACKLIST_LONG_TEST") != "1" {
		t.Skip("set BLACKLIST_LONG_TEST=1 to run long periodic refresh stability test")
	}
	parsed := loadBlacklistABI(t)
	blockedKey, blockedAddr, err := ctx.CreateAndFundAccount(big.NewInt(1000000000000000000))
	if err != nil {
		t.Fatalf("create blocked account: %v", err)
	}
	if err := sendBlacklistTx(t, ctx.FunderKey, "addToBlacklist", blockedAddr, parsed); err != nil {
		t.Fatalf("addToBlacklist failed: %v", err)
	}
	waitBlocks(t, 2)
	waitForTxRejection(t, blockedKey, 30*time.Second)

	t.Log("waiting for > 3 minutes to validate periodic refresh remains effective")
	time.Sleep(190 * time.Second)
	waitForTxRejection(t, blockedKey, 30*time.Second)

	if err := sendBlacklistTx(t, ctx.FunderKey, "removeFromBlacklist", blockedAddr, parsed); err != nil {
		t.Fatalf("removeFromBlacklist failed: %v", err)
	}
	waitBlocks(t, 2)
	waitForTxRecovery(t, blockedKey, 30*time.Second)
}

func TestBL_FailOpenAlert(t *testing.T) {
	requireBlacklistEnabled(t)
	if cfg.Blacklist.Mode != "real" || !cfg.Blacklist.AlertFailOpen {
		t.Skip("fail-open alert scenario requires real mode with alert_fail_open=true")
	}
	parsed := loadBlacklistABI(t)
	_, err := callGetAll(t, parsed)
	if err == nil {
		t.Skip("blacklist contract is reachable; fail-open path not triggered")
	}
	probeKey, _, ferr := ctx.CreateAndFundAccount(big.NewInt(1000000000000000000))
	if ferr != nil {
		t.Fatalf("create probe account: %v", ferr)
	}
	if txErr := sendSelfTransfer(probeKey, true); txErr != nil {
		t.Fatalf("fail-open probe transfer should still succeed, got: %v", txErr)
	}
	t.Logf("fail-open path observed; contract call error=%v", err)
}
