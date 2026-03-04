package tests

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

const (
	singleSmokeMinTxSent = 3
	singleSmokeMinGrowth = uint64(2)
)

func TestS_SmokeSingleNodeLiveness(t *testing.T) {
	if ctx == nil || cfg == nil {
		t.Fatalf("context not initialized")
	}

	nodes, err := openSmokeNodesWithBounds(cfg, 1, 1)
	if err != nil {
		t.Fatalf("resolve single-node smoke RPC failed: %v", err)
	}
	defer closeSmokeNodes(nodes)

	startHeights, err := collectBlockHeights(nodes)
	if err != nil {
		t.Fatalf("collect initial single-node heights failed: %v", err)
	}

	maxHeights := cloneHeights(startHeights)
	sendCtx, cancelSend := context.WithCancel(context.Background())
	trafficCh := make(chan smokeTrafficStats, 1)
	go func() {
		trafficCh <- runSmokeTraffic(sendCtx)
	}()

	observeDuration := singleSmokeObserveDuration()
	pollInterval := ctx.BlockPollInterval()
	if pollInterval < 250*time.Millisecond {
		pollInterval = 250 * time.Millisecond
	}
	deadline := time.Now().Add(observeDuration)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		current, herr := collectBlockHeights(nodes)
		if herr != nil {
			cancelSend()
			<-trafficCh
			t.Fatalf("collect single-node heights during observe window failed: %v", herr)
		}
		mergeMaxHeights(maxHeights, current)
	}

	cancelSend()
	traffic := <-trafficCh
	if traffic.sent < singleSmokeMinTxSent {
		t.Fatalf("single-node smoke traffic too low: sent=%d failed=%d lastErr=%v", traffic.sent, traffic.failed, traffic.lastErr)
	}
	if traffic.lastErr != nil && traffic.sent == 0 {
		t.Fatalf("single-node smoke has no successful tx: failed=%d lastErr=%v", traffic.failed, traffic.lastErr)
	}

	finalHeights, err := collectBlockHeights(nodes)
	if err != nil {
		t.Fatalf("collect final single-node heights failed: %v", err)
	}
	mergeMaxHeights(maxHeights, finalHeights)

	node := nodes[0]
	start := startHeights[node.name]
	end := maxHeights[node.name]
	if end < start {
		t.Fatalf("single-node height regressed: start=%d end=%d", start, end)
	}
	growth := end - start
	if growth < singleSmokeMinGrowth {
		t.Fatalf("single-node growth too low: start=%d end=%d growth=%d", start, end, growth)
	}

	t.Logf("single-node smoke node=%s rpc=%s start=%d end=%d growth=%d sent=%d failed=%d", node.name, node.rpcURL, start, end, growth, traffic.sent, traffic.failed)
}

func singleSmokeObserveDuration() time.Duration {
	if raw := os.Getenv("SMOKE_SINGLE_OBSERVE_SECONDS"); raw != "" {
		if value, err := strconv.ParseInt(raw, 10, 64); err == nil && value > 0 {
			return time.Duration(value) * time.Second
		}
	}
	seconds := int64(60)
	if cfg != nil && cfg.Test.Smoke.ObserveSeconds > 0 && cfg.Test.Smoke.ObserveSeconds < seconds {
		seconds = cfg.Test.Smoke.ObserveSeconds
	}
	return time.Duration(seconds) * time.Second
}
