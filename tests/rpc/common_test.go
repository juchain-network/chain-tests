package rpc

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"juchain.org/chain/tools/ci/internal/config"
)

var (
	rpcCfg        *config.Config
	rpcConfigPath = flag.String("config", "../../data/test_config.yaml", "Path to generated test configuration file")
	rpcNodes      []RPCNode
)

type RPCNode struct {
	Name string
	Role string
	URL  string
	Impl string
}

func TestMain(m *testing.M) {
	flag.Parse()

	loaded, err := config.LoadConfig(*rpcConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}
	rpcCfg = loaded
	rpcNodes = buildRPCNodes(loaded)
	if len(rpcNodes) == 0 {
		fmt.Fprintf(os.Stderr, "no rpc nodes configured in %s\n", *rpcConfigPath)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func buildRPCNodes(cfg *config.Config) []RPCNode {
	nodes := make([]RPCNode, 0)
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
		nodes = append(nodes, RPCNode{
			Name: name,
			Role: role,
			URL:  strings.TrimSpace(n.URL),
			Impl: implByRole[role],
		})
	}
	if len(nodes) > 0 {
		return nodes
	}

	for i, url := range cfg.RPCs {
		nodes = append(nodes, RPCNode{
			Name: fmt.Sprintf("rpc%d", i+1),
			Role: "validator",
			URL:  strings.TrimSpace(url),
		})
	}
	return nodes
}
