package rpc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

type RPCDiagnostic struct {
	NodeName string
	Role     string
	URL      string
	Method   string
	Args     []interface{}
}

func (d RPCDiagnostic) String() string {
	return fmt.Sprintf("[Node: %s | Role: %s | URL: %s] %s(%v)", d.NodeName, d.Role, d.URL, d.Method, d.Args)
}

// assertRawCall dials a specific RPC endpoint, executes the method, and performs standard formatting on failure
func assertRawCall(t *testing.T, node RPCNode, result interface{}, method string, args ...interface{}) {
	t.Helper()
	diag := RPCDiagnostic{
		NodeName: node.Name,
		Role:     node.Role,
		URL:      node.URL,
		Method:   method,
		Args:     args,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := rpc.DialContext(ctx, node.URL)
	if err != nil {
		t.Fatalf("RPC Dial failed: %v\nDiagnostics: %s", err, diag)
	}
	defer client.Close()

	if err := client.CallContext(ctx, result, method, args...); err != nil {
		t.Fatalf("RPC Call failed: %v\nDiagnostics: %s", err, diag)
	}
}

// dialRPC dials the node returning a connected RPC client, with logging on failure
func dialRPC(t *testing.T, node RPCNode) *rpc.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := rpc.DialContext(ctx, node.URL)
	if err != nil {
		t.Fatalf("Failed to dial node %s at %s (role: %s): %v", node.Name, node.URL, node.Role, err)
	}
	return client
}

// getNodesByRole filters the discovered nodes by a given role substring
func getNodesByRole(role string) []RPCNode {
	var filtered []RPCNode
	for _, n := range rpcNodes {
		if strings.EqualFold(n.Role, role) || strings.Contains(strings.ToLower(n.Role), strings.ToLower(role)) {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

// getAllNodes returns all discovered nodes
func getAllNodes() []RPCNode {
	return rpcNodes
}
