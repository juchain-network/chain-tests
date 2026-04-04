package testkit

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	testctx "juchain.org/chain/tools/ci/internal/context"
)

func CallAt(height uint64) *bind.CallOpts {
	return &bind.CallOpts{BlockNumber: new(big.Int).SetUint64(height)}
}

func LongWindowTimeout(blocks uint64) time.Duration {
	if blocks < 1 {
		blocks = 1
	}
	return time.Duration(blocks)*6*time.Second + 2*time.Minute
}

func AddressIndex(items []common.Address, target common.Address) int {
	for i, item := range items {
		if item == target {
			return i
		}
	}
	return -1
}

func CollectCoinbaseSet(c *testctx.CIContext, startHeight, endHeight uint64) (map[common.Address]bool, error) {
	if c == nil || len(c.Clients) == 0 {
		return nil, fmt.Errorf("context not initialized")
	}
	if endHeight < startHeight {
		return map[common.Address]bool{}, nil
	}
	observed := make(map[common.Address]bool)
	for height := startHeight; height <= endHeight; height++ {
		header, err := c.Clients[0].HeaderByNumber(context.Background(), new(big.Int).SetUint64(height))
		if err != nil || header == nil {
			return nil, fmt.Errorf("read header at height %d failed: %w", height, err)
		}
		observed[header.Coinbase] = true
	}
	return observed, nil
}

func WaitForValidatorCanonicalSync(c *testctx.CIContext, validator common.Address, stableBlocks uint64, timeout time.Duration) error {
	if c == nil || c.Config == nil {
		return fmt.Errorf("context not initialized")
	}
	targetRPC := strings.TrimSpace(c.ValidatorRPCByValidator(validator))
	if targetRPC == "" {
		return fmt.Errorf("missing validator rpc for %s", validator.Hex())
	}
	targetClient, err := ethclient.Dial(targetRPC)
	if err != nil {
		return fmt.Errorf("dial validator rpc %s failed: %w", targetRPC, err)
	}
	defer targetClient.Close()

	referenceRPCs := make([]string, 0, len(c.Config.ValidatorRPCs)+1)
	for _, rpcURL := range c.Config.ValidatorRPCs {
		rpcURL = strings.TrimSpace(rpcURL)
		if rpcURL == "" || strings.EqualFold(rpcURL, targetRPC) {
			continue
		}
		referenceRPCs = append(referenceRPCs, rpcURL)
	}
	if syncRPC := strings.TrimSpace(c.Config.SyncRPC); syncRPC != "" && !strings.EqualFold(syncRPC, targetRPC) {
		referenceRPCs = append(referenceRPCs, syncRPC)
	}
	if len(referenceRPCs) == 0 {
		return fmt.Errorf("no reference rpc available to verify sync for %s", validator.Hex())
	}

	referenceClients := make([]*ethclient.Client, 0, len(referenceRPCs))
	for _, rpcURL := range referenceRPCs {
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			continue
		}
		referenceClients = append(referenceClients, client)
	}
	if len(referenceClients) == 0 {
		return fmt.Errorf("failed to dial any reference rpc for %s", validator.Hex())
	}
	defer func() {
		for _, client := range referenceClients {
			client.Close()
		}
	}()

	if stableBlocks < 1 {
		stableBlocks = 1
	}
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	poll := c.BlockPollInterval()
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}

	var matchedStart uint64
	var haveMatchedStart bool
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		targetHeader, err := targetClient.HeaderByNumber(context.Background(), nil)
		if err == nil && targetHeader != nil {
			matched := false
			var highestRef uint64
			for _, client := range referenceClients {
				refHeader, refErr := client.HeaderByNumber(context.Background(), nil)
				if refErr != nil || refHeader == nil {
					continue
				}
				if refHeader.Number.Uint64() > highestRef {
					highestRef = refHeader.Number.Uint64()
				}
				if refHeader.Number.Uint64() == targetHeader.Number.Uint64() && refHeader.Hash() == targetHeader.Hash() {
					matched = true
				}
			}
			if matched {
				if !haveMatchedStart {
					matchedStart = targetHeader.Number.Uint64()
					haveMatchedStart = true
				}
				if targetHeader.Number.Uint64() >= matchedStart+stableBlocks {
					return nil
				}
			} else if highestRef > targetHeader.Number.Uint64() {
				haveMatchedStart = false
			}
		}
		time.Sleep(poll)
	}

	targetHeader, err := targetClient.HeaderByNumber(context.Background(), nil)
	if err != nil || targetHeader == nil {
		return fmt.Errorf("validator %s did not reach canonical sync within %s", validator.Hex(), timeout)
	}
	return fmt.Errorf(
		"validator %s did not stay on canonical head for %d blocks within %s; latest_height=%d latest_hash=%s",
		validator.Hex(),
		stableBlocks,
		timeout,
		targetHeader.Number.Uint64(),
		targetHeader.Hash().Hex(),
	)
}

func CoinbaseSetKeys(items map[common.Address]bool) []common.Address {
	keys := make([]common.Address, 0, len(items))
	for addr := range items {
		keys = append(keys, addr)
	}
	return keys
}

func RecentCoinbases(c *testctx.CIContext, limit int) []common.Address {
	if c == nil || len(c.Clients) == 0 || limit <= 0 {
		return nil
	}
	latest, err := c.Clients[0].BlockNumber(context.Background())
	if err != nil {
		return nil
	}
	start := uint64(1)
	if latest >= uint64(limit) {
		start = latest - uint64(limit) + 1
	}
	items := make([]common.Address, 0, latest-start+1)
	for height := start; height <= latest; height++ {
		header, err := c.Clients[0].HeaderByNumber(context.Background(), new(big.Int).SetUint64(height))
		if err != nil || header == nil {
			continue
		}
		items = append(items, header.Coinbase)
	}
	return items
}

func WaitUntilHeightOrStall(c *testctx.CIContext, label string, target uint64, stallTimeout, overallTimeout time.Duration) (uint64, error) {
	if c == nil || len(c.Clients) == 0 {
		return 0, fmt.Errorf("context not initialized")
	}
	if stallTimeout <= 0 {
		stallTimeout = 15 * time.Second
	}
	if overallTimeout <= 0 {
		overallTimeout = 2 * time.Minute
	}

	start := time.Now()
	lastProgress := time.Now()
	lastHeight, err := c.Clients[0].BlockNumber(context.Background())
	if err != nil {
		return 0, fmt.Errorf("%s: read initial block height failed: %w", label, err)
	}
	if lastHeight >= target {
		return lastHeight, nil
	}

	poll := c.BlockPollInterval()
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}

	for time.Since(start) < overallTimeout {
		current, err := c.Clients[0].BlockNumber(context.Background())
		if err == nil {
			if current >= target {
				return current, nil
			}
			if current > lastHeight {
				lastHeight = current
				lastProgress = time.Now()
			}
		}

		if time.Since(lastProgress) >= stallTimeout {
			return 0, fmt.Errorf(
				"%s: chain stalled before target height: target=%d current=%d stalled_for=%s recent_coinbases=%v",
				label,
				target,
				lastHeight,
				time.Since(lastProgress).Round(time.Second),
				RecentCoinbases(c, 12),
			)
		}

		time.Sleep(poll)
	}

	return 0, fmt.Errorf(
		"%s: timeout waiting for target height: target=%d current=%d waited=%s recent_coinbases=%v",
		label,
		target,
		lastHeight,
		overallTimeout.Round(time.Second),
		RecentCoinbases(c, 12),
	)
}

func WaitForJailTransitionOrStall(
	c *testctx.CIContext,
	validator common.Address,
	startHeight, deadline uint64,
	stallTimeout time.Duration,
) (uint64, error) {
	if c == nil || len(c.Clients) == 0 {
		return 0, fmt.Errorf("context not initialized")
	}
	if startHeight == 0 {
		startHeight = 1
	}
	if deadline < startHeight {
		deadline = startHeight
	}
	if stallTimeout <= 0 {
		stallTimeout = 15 * time.Second
	}

	start := time.Now()
	overallTimeout := LongWindowTimeout(deadline - startHeight + 1)
	lastProgress := time.Now()
	lastHeight, err := c.Clients[0].BlockNumber(context.Background())
	if err != nil {
		return 0, fmt.Errorf("read initial block height before jail wait failed: %w", err)
	}

	poll := c.BlockPollInterval()
	if poll <= 0 {
		poll = 100 * time.Millisecond
	}

	for time.Since(start) < overallTimeout {
		current, err := c.Clients[0].BlockNumber(context.Background())
		if err == nil {
			if current > lastHeight {
				lastHeight = current
				lastProgress = time.Now()
			}
			if current >= startHeight {
				call := CallAt(current)
				record, recordErr := c.Punish.GetPunishRecord(call, validator)
				info, infoErr := c.Staking.GetValidatorInfo(call, validator)
				if recordErr == nil && infoErr == nil && info.IsJailed && record.Sign() == 0 {
					return current, nil
				}
				if current >= deadline {
					return 0, fmt.Errorf(
						"validator did not reach jailed+reset state within expected window: validator=%s start=%d deadline=%d current=%d punish_record=%v jailed=%v recent_coinbases=%v",
						validator.Hex(),
						startHeight,
						deadline,
						current,
						record,
						infoErr == nil && info.IsJailed,
						RecentCoinbases(c, 12),
					)
				}
			}
		}

		if time.Since(lastProgress) >= stallTimeout {
			return 0, fmt.Errorf(
				"chain stalled while waiting for validator jail transition: validator=%s start=%d deadline=%d current=%d stalled_for=%s recent_coinbases=%v",
				validator.Hex(),
				startHeight,
				deadline,
				lastHeight,
				time.Since(lastProgress).Round(time.Second),
				RecentCoinbases(c, 12),
			)
		}

		time.Sleep(poll)
	}

	return 0, fmt.Errorf(
		"timeout waiting for validator jail transition: validator=%s start=%d deadline=%d current=%d waited=%s recent_coinbases=%v",
		validator.Hex(),
		startHeight,
		deadline,
		lastHeight,
		overallTimeout.Round(time.Second),
		RecentCoinbases(c, 12),
	)
}
