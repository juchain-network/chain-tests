package rpc

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
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

type convergenceSettings struct {
	Timeout              time.Duration
	PollInterval         time.Duration
	StableReadsRequired  int
	SemanticDescription  string
	PropagationAllowance string
}

type convergenceObservation struct {
	Node      RPCNode
	Value     any
	Canonical string
	Err       error
}

type convergenceSnapshot struct {
	Attempt      int
	ObservedAt   time.Time
	Observations []convergenceObservation
}

type roleAwareExpectation struct {
	Method                  string
	Settings                convergenceSettings
	ExpectationsByRole      map[string]func(convergenceObservation) string
	Comparator              func([]convergenceObservation) string
	DescribeObservation     func(convergenceObservation) string
	AllowRolesToBeUnmatched bool
}

type normalizedSyncingState struct {
	Mode          string
	CurrentBlock  uint64
	HighestBlock  uint64
	StartingBlock uint64
	RawKeys       []string
}

type coinbaseObservation struct {
	Address string
	Mode    string
}

func isMethodUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "method not found") ||
		strings.Contains(message, "does not exist") ||
		strings.Contains(message, "not available")
}

func defaultConvergenceSettings(semantic string) convergenceSettings {
	return convergenceSettings{
		Timeout:              10 * time.Second,
		PollInterval:         300 * time.Millisecond,
		StableReadsRequired:  3,
		SemanticDescription:  semantic,
		PropagationAllowance: "short stabilization for cross-node propagation",
	}
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

func collectStableRoleAwareObservations(
	t *testing.T,
	nodes []RPCNode,
	method string,
	args []interface{},
	settings convergenceSettings,
	collect func(*rpc.Client, RPCNode) (any, error),
	normalize func(RPCNode, any, error) convergenceObservation,
	matcher func([]convergenceObservation) string,
) []convergenceObservation {
	t.Helper()
	if len(nodes) == 0 {
		t.Fatalf("no rpc nodes provided for %s convergence check", method)
	}
	if settings.Timeout <= 0 {
		settings.Timeout = 10 * time.Second
	}
	if settings.PollInterval <= 0 {
		settings.PollInterval = 300 * time.Millisecond
	}
	if settings.StableReadsRequired <= 0 {
		settings.StableReadsRequired = 3
	}

	clients := make([]*rpc.Client, 0, len(nodes))
	for _, node := range nodes {
		clients = append(clients, dialRPC(t, node))
	}
	defer func() {
		for _, client := range clients {
			client.Close()
		}
	}()

	deadline := time.Now().Add(settings.Timeout)
	stableReads := 0
	lastMatchedCanonical := ""
	var lastSnapshot convergenceSnapshot
	attempt := 0

	for {
		attempt++
		observations := make([]convergenceObservation, 0, len(nodes))
		for i, node := range nodes {
			value, err := collect(clients[i], node)
			observations = append(observations, normalize(node, value, err))
		}
		lastSnapshot = convergenceSnapshot{Attempt: attempt, ObservedAt: time.Now(), Observations: observations}

		canonical := matcher(observations)
		if canonical != "" {
			if canonical == lastMatchedCanonical {
				stableReads++
			} else {
				lastMatchedCanonical = canonical
				stableReads = 1
			}
			if stableReads >= settings.StableReadsRequired {
				return observations
			}
		} else {
			lastMatchedCanonical = ""
			stableReads = 0
		}

		if time.Now().After(deadline) {
			t.Fatalf("%s did not stabilize for %s within %s\n%s", method, settings.SemanticDescription, settings.Timeout, formatConvergenceFailure(method, args, settings, lastSnapshot, stableReads))
		}
		time.Sleep(settings.PollInterval)
	}
}

func assertRoleAwareConvergence(
	t *testing.T,
	nodes []RPCNode,
	expectation roleAwareExpectation,
	collect func(*rpc.Client, RPCNode) (any, error),
	normalize func(RPCNode, any, error) convergenceObservation,
) []convergenceObservation {
	t.Helper()
	matcher := func(observations []convergenceObservation) string {
		return matchRoleAwareObservations(observations, expectation)
	}
	return collectStableRoleAwareObservations(t, nodes, expectation.Method, nil, expectation.Settings, collect, normalize, matcher)
}

func matchRoleAwareObservations(observations []convergenceObservation, expectation roleAwareExpectation) string {
	if expectation.Comparator == nil {
		return ""
	}
	if canonical := expectation.Comparator(observations); canonical == "" {
		return ""
	}

	roles := make([]string, 0, len(expectation.ExpectationsByRole))
	for role := range expectation.ExpectationsByRole {
		roles = append(roles, role)
	}
	sort.Strings(roles)

	for _, role := range roles {
		matcher := expectation.ExpectationsByRole[role]
		matched := false
		for _, obs := range observations {
			if !roleMatchesExpectation(obs.Node.Role, role) {
				continue
			}
			matched = true
			if matcher != nil {
				if desc := matcher(obs); desc != "" {
					return ""
				}
			}
		}
		if !matched && !expectation.AllowRolesToBeUnmatched {
			return ""
		}
	}

	return expectation.Comparator(observations)
}

func roleMatchesExpectation(actualRole, expectedRole string) bool {
	actualRole = normalizeNodeRole(actualRole)
	expectedRole = normalizeNodeRole(expectedRole)
	if actualRole == expectedRole {
		return true
	}
	switch expectedRole {
	case "validator":
		return isValidatorRole(actualRole)
	case "sync":
		return isSyncRole(actualRole)
	default:
		return strings.Contains(actualRole, expectedRole)
	}
}

func formatConvergenceFailure(method string, args []interface{}, settings convergenceSettings, snapshot convergenceSnapshot, stableReads int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "stabilization: semantic=%s timeout=%s poll=%s stable_reads=%d/%d allowance=%s attempts=%d observed_at=%s\n", settings.SemanticDescription, settings.Timeout, settings.PollInterval, stableReads, settings.StableReadsRequired, settings.PropagationAllowance, snapshot.Attempt, snapshot.ObservedAt.Format(time.RFC3339Nano))
	fmt.Fprintf(&b, "method=%s args=%v\n", method, args)
	for _, obs := range snapshot.Observations {
		fmt.Fprintf(&b, "- node=%s role=%s url=%s value=%s\n", obs.Node.Name, obs.Node.Role, obs.Node.URL, describeObservation(obs))
	}
	return strings.TrimRight(b.String(), "\n")
}

func describeObservation(obs convergenceObservation) string {
	if obs.Err != nil {
		return fmt.Sprintf("error=%q", obs.Err.Error())
	}
	if obs.Canonical != "" {
		return obs.Canonical
	}
	return fmt.Sprintf("%#v", obs.Value)
}

func canonicalEquality(observations []convergenceObservation) string {
	if len(observations) == 0 {
		return ""
	}
	for _, obs := range observations {
		if obs.Err != nil || obs.Canonical == "" {
			return ""
		}
	}
	base := observations[0].Canonical
	for _, obs := range observations[1:] {
		if obs.Canonical != base {
			return ""
		}
	}
	return base
}

func canonicalRoleValues(observations []convergenceObservation) string {
	if len(observations) == 0 {
		return ""
	}
	perRole := make(map[string]string)
	roles := make([]string, 0)
	for _, obs := range observations {
		if obs.Canonical == "" {
			return ""
		}
		roleKey := classifyRole(obs.Node.Role)
		if existing, ok := perRole[roleKey]; ok && existing != obs.Canonical {
			return ""
		}
		if _, ok := perRole[roleKey]; !ok {
			roles = append(roles, roleKey)
		}
		perRole[roleKey] = obs.Canonical
	}
	sort.Strings(roles)
	parts := make([]string, 0, len(roles))
	for _, role := range roles {
		parts = append(parts, fmt.Sprintf("%s=%s", role, perRole[role]))
	}
	return strings.Join(parts, " | ")
}

func canonicalNodeValues(observations []convergenceObservation) string {
	if len(observations) == 0 {
		return ""
	}
	parts := make([]string, 0, len(observations))
	for _, obs := range observations {
		if obs.Canonical == "" {
			return ""
		}
		parts = append(parts, fmt.Sprintf("%s=%s", obs.Node.Name, obs.Canonical))
	}
	sort.Strings(parts)
	return strings.Join(parts, " | ")
}

func classifyRole(role string) string {
	if isSyncRole(role) {
		return "sync"
	}
	if isValidatorRole(role) {
		return "validator"
	}
	return normalizeNodeRole(role)
}

func normalizeSyncingObservation(node RPCNode, value any, err error) convergenceObservation {
	obs := convergenceObservation{Node: node, Value: value, Err: err}
	if err != nil {
		obs.Canonical = fmt.Sprintf("error=%q", err.Error())
		return obs
	}
	state, normalizeErr := normalizeSyncingState(value)
	if normalizeErr != nil {
		obs.Err = normalizeErr
		obs.Canonical = fmt.Sprintf("error=%q", normalizeErr.Error())
		return obs
	}
	obs.Value = state
	obs.Canonical = fmt.Sprintf("mode=%s,current=%d,highest=%d,start=%d,keys=%s", state.Mode, state.CurrentBlock, state.HighestBlock, state.StartingBlock, strings.Join(state.RawKeys, ","))
	return obs
}

func normalizeSyncingState(value any) (normalizedSyncingState, error) {
	switch v := value.(type) {
	case bool:
		if v {
			return normalizedSyncingState{}, fmt.Errorf("eth_syncing returned bare true")
		}
		return normalizedSyncingState{Mode: "idle"}, nil
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		current, err := uint64FromAny(v["currentBlock"])
		if err != nil {
			return normalizedSyncingState{}, fmt.Errorf("invalid currentBlock: %w", err)
		}
		highest, err := uint64FromAny(v["highestBlock"])
		if err != nil {
			return normalizedSyncingState{}, fmt.Errorf("invalid highestBlock: %w", err)
		}
		starting, err := uint64FromAny(v["startingBlock"])
		if err != nil {
			return normalizedSyncingState{}, fmt.Errorf("invalid startingBlock: %w", err)
		}
		return normalizedSyncingState{
			Mode:          "syncing",
			CurrentBlock:  current,
			HighestBlock:  highest,
			StartingBlock: starting,
			RawKeys:       keys,
		}, nil
	default:
		return normalizedSyncingState{}, fmt.Errorf("unexpected eth_syncing type %T", value)
	}
}

func normalizePeerCountObservation(node RPCNode, value any, err error) convergenceObservation {
	obs := convergenceObservation{Node: node, Value: value, Err: err}
	if err != nil {
		obs.Canonical = fmt.Sprintf("error=%q", err.Error())
		return obs
	}
	count, normalizeErr := uint64FromAny(value)
	if normalizeErr != nil {
		obs.Err = normalizeErr
		obs.Canonical = fmt.Sprintf("error=%q", normalizeErr.Error())
		return obs
	}
	obs.Value = count
	obs.Canonical = fmt.Sprintf("peers=%d", count)
	return obs
}

func normalizeCoinbaseObservation(node RPCNode, value any, err error) convergenceObservation {
	obs := convergenceObservation{Node: node, Value: value, Err: err}
	if err != nil {
		message := err.Error()
		obs.Value = coinbaseObservation{Mode: "error"}
		obs.Canonical = fmt.Sprintf("mode=error,message=%q", message)
		return obs
	}
	address, ok := value.(string)
	if !ok {
		typeErr := fmt.Errorf("unexpected eth_coinbase type %T", value)
		obs.Err = typeErr
		obs.Canonical = fmt.Sprintf("error=%q", typeErr.Error())
		return obs
	}
	address = strings.ToLower(strings.TrimSpace(address))
	if !common.IsHexAddress(address) {
		addrErr := fmt.Errorf("invalid eth_coinbase address %q", address)
		obs.Err = addrErr
		obs.Canonical = fmt.Sprintf("error=%q", addrErr.Error())
		return obs
	}
	obs.Value = coinbaseObservation{Address: address, Mode: "address"}
	obs.Canonical = fmt.Sprintf("mode=address,address=%s", address)
	return obs
}

func uint64FromAny(value any) (uint64, error) {
	switch v := value.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, fmt.Errorf("empty string")
		}
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			if len(s) == 2 {
				return 0, nil
			}
			return strconv.ParseUint(s[2:], 16, 64)
		}
		return strconv.ParseUint(s, 10, 64)
	case float64:
		return uint64(v), nil
	case int:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case uint64:
		return v, nil
	case nil:
		return 0, fmt.Errorf("missing value")
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", value)
	}
}

func fixtureEqualityExpectation(method string, semantic string) roleAwareExpectation {
	return roleAwareExpectation{
		Method:     method,
		Settings:   defaultConvergenceSettings(semantic),
		Comparator: canonicalEquality,
		DescribeObservation: func(obs convergenceObservation) string {
			return describeObservation(obs)
		},
	}
}

func ethSyncingExpectation() roleAwareExpectation {
	expectation := roleAwareExpectation{
		Method:     "eth_syncing",
		Settings:   defaultConvergenceSettings("role-aware syncing semantics"),
		Comparator: canonicalRoleValues,
		ExpectationsByRole: map[string]func(convergenceObservation) string{
			"validator": func(obs convergenceObservation) string {
				state, ok := obs.Value.(normalizedSyncingState)
				if !ok {
					return fmt.Sprintf("expected normalized syncing state, got %T", obs.Value)
				}
				if state.Mode != "idle" && state.Mode != "syncing" {
					return fmt.Sprintf("unexpected validator syncing mode %q", state.Mode)
				}
				if state.Mode == "syncing" && state.HighestBlock < state.CurrentBlock {
					return fmt.Sprintf("highestBlock %d below currentBlock %d", state.HighestBlock, state.CurrentBlock)
				}
				return ""
			},
			"sync": func(obs convergenceObservation) string {
				state, ok := obs.Value.(normalizedSyncingState)
				if !ok {
					return fmt.Sprintf("expected normalized syncing state, got %T", obs.Value)
				}
				if state.Mode != "idle" && state.Mode != "syncing" {
					return fmt.Sprintf("unexpected sync syncing mode %q", state.Mode)
				}
				if state.Mode == "syncing" && state.HighestBlock < state.CurrentBlock {
					return fmt.Sprintf("highestBlock %d below currentBlock %d", state.HighestBlock, state.CurrentBlock)
				}
				return ""
			},
		},
	}
	return expectation
}

func netPeerCountExpectation(topology RPCTopology) roleAwareExpectation {
	validatorCount := len(topology.Validators)
	syncCount := len(topology.Sync)
	minimumForAll := len(topology.All) - 1
	return roleAwareExpectation{
		Method:     "net_peerCount",
		Settings:   defaultConvergenceSettings("cluster peer-count convergence"),
		Comparator: canonicalEquality,
		ExpectationsByRole: map[string]func(convergenceObservation) string{
			"validator": func(obs convergenceObservation) string {
				count, ok := obs.Value.(uint64)
				if !ok {
					return fmt.Sprintf("expected uint64 peer count, got %T", obs.Value)
				}
				if count == 0 {
					return "validator peer count must be non-zero"
				}
				if minimumForAll > 0 && count < uint64(minimumForAll) {
					return fmt.Sprintf("validator peer count %d below topology minimum %d", count, minimumForAll)
				}
				_ = validatorCount
				_ = syncCount
				return ""
			},
			"sync": func(obs convergenceObservation) string {
				count, ok := obs.Value.(uint64)
				if !ok {
					return fmt.Sprintf("expected uint64 peer count, got %T", obs.Value)
				}
				if count == 0 {
					return "sync peer count must be non-zero"
				}
				if minimumForAll > 0 && count < uint64(minimumForAll) {
					return fmt.Sprintf("sync peer count %d below topology minimum %d", count, minimumForAll)
				}
				return ""
			},
		},
	}
}

func ethCoinbaseExpectation() roleAwareExpectation {
	return roleAwareExpectation{
		Method:     "eth_coinbase",
		Settings:   defaultConvergenceSettings("role-aware coinbase expectations"),
		Comparator: canonicalNodeValues,
		ExpectationsByRole: map[string]func(convergenceObservation) string{
			"validator": func(obs convergenceObservation) string {
				if obs.Err != nil {
					if isMethodUnavailableError(obs.Err) {
						return ""
					}
					return fmt.Sprintf("unexpected validator eth_coinbase error %q", obs.Err.Error())
				}
				value, ok := obs.Value.(coinbaseObservation)
				if !ok {
					return fmt.Sprintf("expected coinbase observation, got %T", obs.Value)
				}
				if value.Mode != "address" {
					return fmt.Sprintf("validator expected address result, got %s", describeObservation(obs))
				}
				expected := obs.Node.ExpectedCoinbase()
				if expected != "" && value.Address != expected {
					return fmt.Sprintf("validator expected coinbase %s, got %s", expected, value.Address)
				}
				return ""
			},
			"sync": func(obs convergenceObservation) string {
				if obs.Err != nil {
					message := strings.ToLower(obs.Err.Error())
					if strings.Contains(message, "etherbase must be explicitly specified") || isMethodUnavailableError(obs.Err) {
						return ""
					}
					return fmt.Sprintf("unexpected sync eth_coinbase error %q", obs.Err.Error())
				}
				value, ok := obs.Value.(coinbaseObservation)
				if !ok {
					return fmt.Sprintf("expected coinbase observation, got %T", obs.Value)
				}
				if value.Mode == "address" {
					if value.Address == "" || !common.IsHexAddress(value.Address) {
						return fmt.Sprintf("sync node returned invalid coinbase %q", value.Address)
					}
					return ""
				}
				return fmt.Sprintf("unexpected sync eth_coinbase result %s", describeObservation(obs))
			},
		},
	}
}

func assertFixtureBackedEquality(
	t *testing.T,
	nodes []RPCNode,
	method string,
	args []interface{},
	expected any,
	semantic string,
) []convergenceObservation {
	t.Helper()
	expectation := fixtureEqualityExpectation(method, semantic)
	return collectStableRoleAwareObservations(
		t,
		nodes,
		method,
		args,
		expectation.Settings,
		func(client *rpc.Client, _ RPCNode) (any, error) {
			var result any
			err := client.CallContext(context.Background(), &result, method, args...)
			return result, err
		},
		func(node RPCNode, value any, err error) convergenceObservation {
			obs := convergenceObservation{Node: node, Value: value, Err: err}
			if err != nil {
				obs.Canonical = fmt.Sprintf("error=%q", err.Error())
				return obs
			}
			if reflect.DeepEqual(expected, value) {
				obs.Canonical = fmt.Sprintf("match=%#v", value)
				return obs
			}
			obs.Canonical = fmt.Sprintf("mismatch expected=%#v actual=%#v", expected, value)
			return obs
		},
		func(observations []convergenceObservation) string {
			for _, obs := range observations {
				if obs.Err != nil {
					return ""
				}
				if !strings.HasPrefix(obs.Canonical, "match=") {
					return ""
				}
			}
			return canonicalEquality(observations)
		},
	)
}
