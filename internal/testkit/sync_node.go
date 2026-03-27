package testkit

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"juchain.org/chain/tools/ci/internal/config"
	testctx "juchain.org/chain/tools/ci/internal/context"
)

func SyncNodeRPC(c *testctx.CIContext) string {
	if c == nil || c.Config == nil {
		return ""
	}
	if value := strings.TrimSpace(c.Config.SyncRPC); value != "" {
		return value
	}
	for _, node := range c.Config.NodeRPCs {
		if strings.EqualFold(strings.TrimSpace(node.Role), "sync") && strings.TrimSpace(node.URL) != "" {
			return strings.TrimSpace(node.URL)
		}
	}
	return ""
}

func SyncNodeRuntime(c *testctx.CIContext) *config.RuntimeNode {
	if c == nil || c.Config == nil {
		return nil
	}
	for i := range c.Config.RuntimeNodes {
		node := &c.Config.RuntimeNodes[i]
		if strings.EqualFold(strings.TrimSpace(node.Role), "sync") {
			return node
		}
	}
	return nil
}

func DialSyncNode(c *testctx.CIContext) (*ethclient.Client, string, error) {
	rpcURL := SyncNodeRPC(c)
	if rpcURL == "" {
		return nil, "", fmt.Errorf("sync node rpc not configured")
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, "", fmt.Errorf("dial sync node %s failed: %w", rpcURL, err)
	}
	return client, rpcURL, nil
}

func StartSyncNodeWithSigner(c *testctx.CIContext, key *ecdsa.PrivateKey, password string, timeout time.Duration) error {
	client, rpcURL, err := DialSyncNode(c)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := ImportUnlockAndSetEtherbase(client, key, password); err != nil {
		return fmt.Errorf("prepare sync miner signer failed: %w", err)
	}
	if err := MinerStart(client); err != nil {
		return fmt.Errorf("start sync miner failed: %w", err)
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := client.BlockNumber(context.Background()); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("sync node rpc %s not ready after miner_start", rpcURL)
}
