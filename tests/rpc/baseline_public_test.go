package rpc

import (
	"testing"
)

func TestBaseline_PublicMethods(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	for _, node := range nodes {
		node := node
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()

			// Test eth_chainId
			var chainID string
			assertRawCall(t, node, &chainID, "eth_chainId")
			if chainID == "" {
				t.Errorf("Expected valid chainID from %s, got empty", node.Name)
			}

			// Test eth_blockNumber
			var blockNumber string
			assertRawCall(t, node, &blockNumber, "eth_blockNumber")
			if blockNumber == "" {
				t.Errorf("Expected valid blockNumber from %s, got empty", node.Name)
			}

			// Test net_version
			var netVersion string
			assertRawCall(t, node, &netVersion, "net_version")
			if netVersion == "" {
				t.Errorf("Expected valid netVersion from %s, got empty", node.Name)
			}

			// Test web3_clientVersion
			var clientVersion string
			assertRawCall(t, node, &clientVersion, "web3_clientVersion")
			if clientVersion == "" {
				t.Errorf("Expected valid clientVersion from %s, got empty", node.Name)
			}
		})
	}
}
