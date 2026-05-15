package rpc

import (
	"strconv"
	"strings"
	"testing"
)

func TestA_BaselinePublicMethods(t *testing.T) {
	nodes := getAllNodes()
	if len(nodes) == 0 {
		t.Skip("No RPC nodes available to test")
	}

	for _, node := range nodes {
		node := node
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()

			t.Run("web3_clientVersion", func(t *testing.T) {
				var clientVersion string
				assertRawCall(t, node, &clientVersion, "web3_clientVersion")
				if clientVersion == "" {
					t.Errorf("Expected valid clientVersion from %s, got empty", node.Name)
				}
				// Typically starts with Geth/v...
				if len(clientVersion) < 3 {
					t.Errorf("Client version seems too short: %s", clientVersion)
				}
			})

			t.Run("eth_chainId", func(t *testing.T) {
				var chainIDHex string
				assertRawCall(t, node, &chainIDHex, "eth_chainId")
				if !strings.HasPrefix(chainIDHex, "0x") {
					t.Errorf("Expected chainID to have 0x prefix, got %s", chainIDHex)
				}
				chainID, err := strconv.ParseUint(strings.TrimPrefix(chainIDHex, "0x"), 16, 64)
				if err != nil {
					t.Errorf("Failed to parse chainID: %v", err)
				}
				if chainID == 0 {
					t.Errorf("Expected non-zero chainID, got %d", chainID)
				}
			})

			t.Run("eth_blockNumber", func(t *testing.T) {
				var blockNumberHex string
				assertRawCall(t, node, &blockNumberHex, "eth_blockNumber")
				if !strings.HasPrefix(blockNumberHex, "0x") {
					t.Errorf("Expected blockNumber to have 0x prefix, got %s", blockNumberHex)
				}
				_, err := strconv.ParseUint(strings.TrimPrefix(blockNumberHex, "0x"), 16, 64)
				if err != nil {
					t.Errorf("Failed to parse blockNumber: %v", err)
				}
			})

			t.Run("eth_syncing", func(t *testing.T) {
				var syncing interface{}
				assertRawCall(t, node, &syncing, "eth_syncing")
				
				switch v := syncing.(type) {
				case bool:
					if v {
						t.Errorf("eth_syncing returned true boolean instead of object for syncing state")
					}
				case map[string]interface{}:
					expectedFields := []string{"startingBlock", "currentBlock", "highestBlock"}
					for _, field := range expectedFields {
						if _, ok := v[field]; !ok {
							t.Errorf("eth_syncing object missing field: %s", field)
						}
					}
				default:
					t.Errorf("eth_syncing returned unexpected type: %T", syncing)
				}
			})

			t.Run("net_version", func(t *testing.T) {
				var netVersion string
				assertRawCall(t, node, &netVersion, "net_version")
				if netVersion == "" {
					t.Errorf("Expected valid netVersion from %s, got empty", node.Name)
				}
				version, err := strconv.ParseUint(netVersion, 10, 64)
				if err != nil {
					t.Errorf("Expected netVersion to be a base10 integer string, got %s", netVersion)
				}
				if version == 0 {
					t.Errorf("Expected non-zero netVersion, got %d", version)
				}
			})

			t.Run("net_peerCount", func(t *testing.T) {
				var peerCountHex string
				assertRawCall(t, node, &peerCountHex, "net_peerCount")
				if !strings.HasPrefix(peerCountHex, "0x") {
					t.Errorf("Expected peerCount to have 0x prefix, got %s", peerCountHex)
				}
				_, err := strconv.ParseUint(strings.TrimPrefix(peerCountHex, "0x"), 16, 64)
				if err != nil {
					t.Errorf("Failed to parse peerCount: %v", err)
				}
			})
		})
	}
}
