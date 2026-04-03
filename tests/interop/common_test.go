package tests

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"juchain.org/chain/tools/ci/internal/config"
)

type interopNode struct {
	Name string
	Role string
	URL  string
	Impl string
}

var (
	interopCfg        *config.Config
	interopConfigPath = flag.String("config", "../../data/test_config.yaml", "Path to generated test configuration file")
	interopNodes      []interopNode
)

func TestMain(m *testing.M) {
	flag.Parse()

	loaded, err := config.LoadConfig(*interopConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}
	interopCfg = loaded
	interopNodes = buildInteropNodes(loaded)
	if len(interopNodes) == 0 {
		fmt.Fprintf(os.Stderr, "no interop nodes configured in %s\n", *interopConfigPath)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func buildInteropNodes(cfg *config.Config) []interopNode {
	nodes := make([]interopNode, 0)
	implByRole := make(map[string]string)
	for _, n := range cfg.RuntimeNodes {
		if strings.TrimSpace(n.Role) != "" && strings.TrimSpace(n.Impl) != "" {
			implByRole[strings.ToLower(strings.TrimSpace(n.Role))] = strings.ToLower(strings.TrimSpace(n.Impl))
		}
	}

	for i, n := range cfg.NodeRPCs {
		role := strings.ToLower(strings.TrimSpace(n.Role))
		name := strings.TrimSpace(n.Name)
		if name == "" {
			name = fmt.Sprintf("node%d", i)
		}
		nodes = append(nodes, interopNode{
			Name: name,
			Role: role,
			URL:  strings.TrimSpace(n.URL),
			Impl: implByRole[role],
		})
	}
	if len(nodes) > 0 {
		return nodes
	}

	for i, rpcURL := range cfg.RPCs {
		nodes = append(nodes, interopNode{
			Name: fmt.Sprintf("rpc%d", i+1),
			Role: "validator",
			URL:  strings.TrimSpace(rpcURL),
		})
	}
	return nodes
}

func dialEth(t *testing.T, url string) *ethclient.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := ethclient.DialContext(ctx, url)
	if err != nil {
		t.Fatalf("dial eth %s: %v", url, err)
	}
	return c
}

func dialRPC(t *testing.T, url string) *rpc.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := rpc.DialContext(ctx, url)
	if err != nil {
		t.Fatalf("dial rpc %s: %v", url, err)
	}
	return c
}

func parseHexToUint64(v string) (uint64, error) {
	s := strings.TrimSpace(v)
	if s == "" || s == "0x" {
		return 0, nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

func pickSyncAndValidators(nodes []interopNode) (syncNode *interopNode, validators []interopNode) {
	for i := range nodes {
		n := nodes[i]
		if n.URL == "" {
			continue
		}
		if strings.Contains(strings.ToLower(n.Role), "sync") {
			node := n
			syncNode = &node
			continue
		}
		validators = append(validators, n)
	}
	if len(validators) == 0 && len(nodes) > 0 {
		validators = append(validators, nodes[0])
	}
	return syncNode, validators
}

func latestHeight(t *testing.T, c *ethclient.Client) uint64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	h, err := c.BlockNumber(ctx)
	if err != nil {
		t.Fatalf("blockNumber failed: %v", err)
	}
	return h
}

func blockByNumberRaw(t *testing.T, client *rpc.Client, blockTag string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var out map[string]any
		err := client.CallContext(ctx, &out, "eth_getBlockByNumber", blockTag, false)
		cancel()
		if err == nil && out != nil {
			return out
		}
		lastErr = err
		if time.Now().After(deadline) {
			if lastErr != nil {
				t.Fatalf("eth_getBlockByNumber(%s) did not become readable within 10s: %v", blockTag, lastErr)
			}
			t.Fatalf("eth_getBlockByNumber(%s) returned nil for 10s after target height was reached", blockTag)
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func fetchStateRoot(t *testing.T, client *rpc.Client, blockTag string) (hash string, stateRoot string) {
	t.Helper()
	block := blockByNumberRaw(t, client, blockTag)
	hash, _ = block["hash"].(string)
	stateRoot, _ = block["stateRoot"].(string)
	if strings.TrimSpace(hash) == "" || strings.EqualFold(hash, "null") {
		t.Fatalf("missing block hash at %s", blockTag)
	}
	if strings.TrimSpace(stateRoot) == "" || strings.EqualFold(stateRoot, "null") {
		t.Fatalf("missing stateRoot at %s", blockTag)
	}
	return hash, stateRoot
}

func parseCheckpointSpec(spec string, start, latest uint64) ([]uint64, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		mid := start
		if latest > start {
			mid = start + (latest-start)/2
		}
		return []uint64{0, mid, latest}, nil
	}
	parts := strings.Split(spec, ",")
	vals := make([]uint64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.EqualFold(p, "latest") {
			vals = append(vals, latest)
			continue
		}
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid checkpoint %q: %w", p, err)
		}
		vals = append(vals, v)
	}
	if len(vals) == 0 {
		return nil, fmt.Errorf("empty checkpoint list")
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	uniq := vals[:0]
	for _, v := range vals {
		if len(uniq) == 0 || uniq[len(uniq)-1] != v {
			uniq = append(uniq, v)
		}
	}
	return uniq, nil
}

func toBlockTag(height uint64) string {
	return fmt.Sprintf("0x%x", height)
}

func netPeerCount(t *testing.T, client *rpc.Client) uint64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var hexCount string
	if err := client.CallContext(ctx, &hexCount, "net_peerCount"); err != nil {
		t.Fatalf("net_peerCount failed: %v", err)
	}
	count, err := parseHexToUint64(hexCount)
	if err != nil {
		t.Fatalf("parse net_peerCount %q failed: %v", hexCount, err)
	}
	return count
}

func waitForHeightAtLeast(t *testing.T, c *ethclient.Client, target uint64, timeout time.Duration) uint64 {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		h := latestHeight(t, c)
		if h >= target {
			return h
		}
		time.Sleep(1 * time.Second)
	}
	return latestHeight(t, c)
}
