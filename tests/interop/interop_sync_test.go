package tests

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	interopMaxHeightLag   = uint64(8)
	interopObserveTimeout = 120 * time.Second
)

type validatorEthClient struct {
	name string
	eth  *ethclient.Client
}

func TestI_SyncCatchUp(t *testing.T) {
	syncNode, validators := pickSyncAndValidators(interopNodes)
	if syncNode == nil {
		t.Skip("no sync node configured in node_rpcs")
	}
	if len(validators) == 0 {
		t.Skip("no validator rpc configured")
	}

	syncEth := dialEth(t, syncNode.URL)
	defer syncEth.Close()
	syncRPC := dialRPC(t, syncNode.URL)
	defer syncRPC.Close()

	clients := make([]validatorEthClient, 0, len(validators))
	for _, v := range validators {
		c := dialEth(t, v.URL)
		clients = append(clients, validatorEthClient{name: v.Name, eth: c})
		defer c.Close()
	}

	peers := netPeerCount(t, syncRPC)
	if peers == 0 {
		t.Fatalf("sync node %s has zero peers", syncNode.Name)
	}

	startSync := latestHeight(t, syncEth)
	startMaxVal := startSync
	for _, c := range clients {
		h := latestHeight(t, c.eth)
		if h > startMaxVal {
			startMaxVal = h
		}
	}

	targetGrowth := uint64(6)
	deadline := time.Now().Add(interopObserveTimeout)
	bestLag := ^uint64(0)
	var lastMaxVal, lastSync uint64

	for time.Now().Before(deadline) {
		maxVal := uint64(0)
		for _, c := range clients {
			h := latestHeight(t, c.eth)
			if h > maxVal {
				maxVal = h
			}
		}
		syncHeight := latestHeight(t, syncEth)

		lag := uint64(0)
		if maxVal > syncHeight {
			lag = maxVal - syncHeight
		}
		if lag < bestLag {
			bestLag = lag
		}
		lastMaxVal, lastSync = maxVal, syncHeight

		if maxVal >= startMaxVal+targetGrowth && syncHeight >= startSync+targetGrowth/2 && lag <= interopMaxHeightLag {
			return
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("sync catch-up not reached: startVal=%d startSync=%d lastVal=%d lastSync=%d bestLag=%d maxLag=%d",
		startMaxVal, startSync, lastMaxVal, lastSync, bestLag, interopMaxHeightLag)
}
