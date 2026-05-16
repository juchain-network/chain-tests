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
	Name             string
	Role             string
	URL              string
	Impl             string
	ValidatorAddress string
	FeeAddress       string
}

type RPCTopology struct {
	All        []RPCNode
	Validators []RPCNode
	Sync       []RPCNode
	ByRole     map[string][]RPCNode
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
	runtimeByName := make(map[string]config.RuntimeNode)
	for _, n := range cfg.RuntimeNodes {
		trimmedRole := normalizeNodeRole(n.Role)
		if trimmedRole != "" && strings.TrimSpace(n.Impl) != "" {
			implByRole[trimmedRole] = strings.ToLower(strings.TrimSpace(n.Impl))
		}
		if trimmedName := normalizeNodeName(n.Name); trimmedName != "" {
			runtimeByName[trimmedName] = n
		}
	}

	for i, n := range cfg.NodeRPCs {
		role := normalizeNodeRole(n.Role)
		name := strings.TrimSpace(n.Name)
		if name == "" {
			name = fmt.Sprintf("node%d", i)
		}

		runtimeNode := runtimeByName[normalizeNodeName(name)]
		nodes = append(nodes, RPCNode{
			Name:             name,
			Role:             role,
			URL:              strings.TrimSpace(n.URL),
			Impl:             implByRole[role],
			ValidatorAddress: strings.TrimSpace(runtimeNode.ValidatorAddress),
			FeeAddress:       strings.TrimSpace(runtimeNode.FeeAddress),
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

func normalizeNodeRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func normalizeNodeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (n RPCNode) ExpectedCoinbase() string {
	if strings.TrimSpace(n.FeeAddress) != "" {
		return strings.ToLower(strings.TrimSpace(n.FeeAddress))
	}
	if strings.TrimSpace(n.ValidatorAddress) != "" {
		return strings.ToLower(strings.TrimSpace(n.ValidatorAddress))
	}
	return ""
}

func buildRPCTopology(nodes []RPCNode) RPCTopology {
	topology := RPCTopology{
		All:    append([]RPCNode(nil), nodes...),
		ByRole: make(map[string][]RPCNode),
	}
	for _, node := range nodes {
		role := normalizeNodeRole(node.Role)
		topology.ByRole[role] = append(topology.ByRole[role], node)
		if isSyncRole(role) {
			topology.Sync = append(topology.Sync, node)
			continue
		}
		if isValidatorRole(role) {
			topology.Validators = append(topology.Validators, node)
		}
	}
	if len(topology.Validators) == 0 && len(nodes) > 0 {
		topology.Validators = append(topology.Validators, nodes...)
	}
	return topology
}

func getRPCTopology() RPCTopology {
	return buildRPCTopology(rpcNodes)
}

func isSyncRole(role string) bool {
	return strings.Contains(normalizeNodeRole(role), "sync")
}

func isValidatorRole(role string) bool {
	role = normalizeNodeRole(role)
	if role == "" {
		return true
	}
	return strings.Contains(role, "validator")
}
