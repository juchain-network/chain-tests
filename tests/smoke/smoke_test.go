package tests

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"juchain.org/chain/tools/ci/internal/config"
)

const (
	smokeSendInterval   = 400 * time.Millisecond
	smokeMinTxSent      = 8
	smokeMinGrowth      = uint64(3)
	smokeMaxHeightLag   = uint64(6)
	defaultSmokeSeconds = int64(300)
)

type smokeNode struct {
	name   string
	rpcURL string
	client *ethclient.Client
}

type smokeTrafficStats struct {
	sent    int
	failed  int
	lastErr error
}

func TestS_SmokeChainLivenessAllNodes(t *testing.T) {
	if ctx == nil || cfg == nil {
		t.Fatalf("context not initialized")
	}

	nodes, err := openSmokeNodes(cfg)
	if err != nil {
		t.Fatalf("resolve smoke RPC endpoints failed: %v", err)
	}
	defer closeSmokeNodes(nodes)

	startHeights, err := collectBlockHeights(nodes)
	if err != nil {
		t.Fatalf("failed to collect initial heights: %v", err)
	}

	maxHeights := cloneHeights(startHeights)

	sendCtx, cancelSend := context.WithCancel(context.Background())
	trafficCh := make(chan smokeTrafficStats, 1)
	go func() {
		trafficCh <- runSmokeTraffic(sendCtx)
	}()

	pollInterval := ctx.BlockPollInterval()
	if pollInterval < 250*time.Millisecond {
		pollInterval = 250 * time.Millisecond
	}

	observeDuration := smokeObserveDuration(cfg)
	deadline := time.Now().Add(observeDuration)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		current, err := collectBlockHeights(nodes)
		if err != nil {
			cancelSend()
			<-trafficCh
			t.Fatalf("failed to collect heights during smoke window: %v", err)
		}
		mergeMaxHeights(maxHeights, current)
	}

	cancelSend()
	traffic := <-trafficCh
	if traffic.sent < smokeMinTxSent {
		t.Fatalf("smoke traffic too low: sent=%d failed=%d lastErr=%v", traffic.sent, traffic.failed, traffic.lastErr)
	}
	if traffic.lastErr != nil && traffic.sent == 0 {
		t.Fatalf("smoke traffic did not send any successful tx: failed=%d lastErr=%v", traffic.failed, traffic.lastErr)
	}

	finalHeights, err := collectBlockHeights(nodes)
	if err != nil {
		t.Fatalf("failed to collect final heights: %v", err)
	}
	mergeMaxHeights(maxHeights, finalHeights)

	nodeNames := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.name)
	}
	sort.Strings(nodeNames)

	maxFinal := uint64(0)
	minFinal := ^uint64(0)
	for _, name := range nodeNames {
		start := startHeights[name]
		end := maxHeights[name]
		if end < start {
			t.Fatalf("node %s height regressed: start=%d end=%d", name, start, end)
		}
		growth := end - start
		if growth < smokeMinGrowth {
			t.Fatalf("node %s height growth too low: start=%d end=%d growth=%d", name, start, end, growth)
		}
		if end > maxFinal {
			maxFinal = end
		}
		if end < minFinal {
			minFinal = end
		}
		t.Logf("smoke node=%s rpc=%s start=%d end=%d growth=%d", name, findNodeRPC(nodes, name), start, end, growth)
	}

	if maxFinal-minFinal > smokeMaxHeightLag {
		t.Fatalf("node height lag too large: max=%d min=%d lag=%d", maxFinal, minFinal, maxFinal-minFinal)
	}
	if traffic.failed > traffic.sent*2 {
		t.Fatalf("smoke traffic too many failures: sent=%d failed=%d lastErr=%v", traffic.sent, traffic.failed, traffic.lastErr)
	}

	t.Logf("smoke traffic summary: sent=%d failed=%d", traffic.sent, traffic.failed)
}

func smokeObserveDuration(cfg *config.Config) time.Duration {
	seconds := defaultSmokeSeconds
	if cfg != nil && cfg.Test.Smoke.ObserveSeconds > 0 {
		seconds = cfg.Test.Smoke.ObserveSeconds
	}
	return time.Duration(seconds) * time.Second
}

func openSmokeNodes(cfg *config.Config) ([]smokeNode, error) {
	return openSmokeNodesWithBounds(cfg, 4, 4)
}

func openSmokeNodesWithBounds(cfg *config.Config, minRequired int, maxNodes int) ([]smokeNode, error) {
	endpoints, err := resolveSmokeNodeEndpoints(cfg, minRequired, maxNodes)
	if err != nil {
		return nil, err
	}

	nodes := make([]smokeNode, 0, len(endpoints))
	for _, ep := range endpoints {
		dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		client, dialErr := ethclient.DialContext(dialCtx, ep.rpcURL)
		cancel()
		if dialErr != nil {
			closeSmokeNodes(nodes)
			return nil, fmt.Errorf("dial %s (%s): %w", ep.name, ep.rpcURL, dialErr)
		}
		nodes = append(nodes, smokeNode{name: ep.name, rpcURL: ep.rpcURL, client: client})
	}
	return nodes, nil
}

func closeSmokeNodes(nodes []smokeNode) {
	for _, node := range nodes {
		if node.client != nil {
			node.client.Close()
		}
	}
}

func resolveSmokeNodeEndpoints(cfg *config.Config, minRequired int, maxNodes int) ([]smokeNode, error) {
	type endpoint struct {
		name   string
		rpcURL string
	}

	seen := make(map[string]struct{})
	out := make([]endpoint, 0, 4)
	add := func(name, rpcURL string) {
		name = strings.TrimSpace(name)
		rpcURL = strings.TrimSpace(rpcURL)
		if rpcURL == "" {
			return
		}
		if name == "" {
			name = fmt.Sprintf("node%d", len(out)+1)
		}
		if _, exists := seen[rpcURL]; exists {
			return
		}
		seen[rpcURL] = struct{}{}
		out = append(out, endpoint{name: name, rpcURL: rpcURL})
	}

	for _, nodeRPC := range cfg.NodeRPCs {
		add(nodeRPC.Name, nodeRPC.URL)
	}

	for i, rpc := range cfg.ValidatorRPCs {
		add(fmt.Sprintf("validator%d", i+1), rpc)
	}
	add("sync", cfg.SyncRPC)
	for i, rpc := range cfg.RPCs {
		if i == 0 {
			add("primary", rpc)
			continue
		}
		add(fmt.Sprintf("rpc%d", i+1), rpc)
	}

	if minRequired > 0 && len(out) < minRequired {
		return nil, fmt.Errorf("need at least %d unique RPC endpoints, got %d", minRequired, len(out))
	}
	if maxNodes > 0 && len(out) > maxNodes {
		out = out[:maxNodes]
	}

	resolved := make([]smokeNode, 0, len(out))
	for _, ep := range out {
		resolved = append(resolved, smokeNode{name: ep.name, rpcURL: ep.rpcURL})
	}
	return resolved, nil
}

func collectBlockHeights(nodes []smokeNode) (map[string]uint64, error) {
	heights := make(map[string]uint64, len(nodes))
	for _, node := range nodes {
		callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		height, err := node.client.BlockNumber(callCtx)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("get block number from %s (%s): %w", node.name, node.rpcURL, err)
		}
		heights[node.name] = height
	}
	return heights, nil
}

func cloneHeights(src map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}

func mergeMaxHeights(target, current map[string]uint64) {
	for name, height := range current {
		if existing, ok := target[name]; !ok || height > existing {
			target[name] = height
		}
	}
}

func runSmokeTraffic(sendCtx context.Context) smokeTrafficStats {
	stats := smokeTrafficStats{}
	if ctx == nil || ctx.FunderKey == nil || len(ctx.Clients) == 0 {
		stats.lastErr = fmt.Errorf("smoke context is not ready")
		return stats
	}

	client := ctx.Clients[0]
	from := crypto.PubkeyToAddress(ctx.FunderKey.PublicKey)

	nonce, err := client.PendingNonceAt(context.Background(), from)
	if err != nil {
		stats.lastErr = err
		return stats
	}

	ticker := time.NewTicker(smokeSendInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sendCtx.Done():
			return stats
		case <-ticker.C:
			gasPrice, gasErr := client.SuggestGasPrice(context.Background())
			if gasErr != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
				gasPrice = big.NewInt(1_000_000_000)
			}

			tx := types.NewTransaction(nonce, from, big.NewInt(0), 21000, gasPrice, nil)
			signedTx, signErr := types.SignTx(tx, types.NewEIP155Signer(ctx.ChainID), ctx.FunderKey)
			if signErr != nil {
				stats.failed++
				stats.lastErr = signErr
				continue
			}

			if sendErr := client.SendTransaction(context.Background(), signedTx); sendErr != nil {
				stats.failed++
				stats.lastErr = sendErr
				msg := strings.ToLower(sendErr.Error())
				if strings.Contains(msg, "nonce too low") ||
					strings.Contains(msg, "replacement transaction underpriced") ||
					strings.Contains(msg, "already known") {
					if refreshed, refreshErr := client.PendingNonceAt(context.Background(), from); refreshErr == nil {
						nonce = refreshed
					}
				}
				continue
			}

			stats.sent++
			nonce++
		}
	}
}

func findNodeRPC(nodes []smokeNode, name string) string {
	for _, node := range nodes {
		if node.name == name {
			return node.rpcURL
		}
	}
	return ""
}
