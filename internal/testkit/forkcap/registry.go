package forkcap

import (
	"fmt"
	"sort"
	"strings"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDeferred Status = "deferred"
)

var forkOrder = []string{"shanghai", "cancun", "fixheader", "posa", "prague", "osaka", "bpo1", "bpo2"}

func NormalizeFork(fork string) string {
	return strings.TrimSpace(strings.ToLower(fork))
}

func ForkOrderIndex(fork string) (int, bool) {
	fork = NormalizeFork(fork)
	for i, candidate := range forkOrder {
		if candidate == fork {
			return i, true
		}
	}
	return -1, false
}

type Capability struct {
	Name        string
	MinimumFork string
	Status      Status
	Reason      string
	Tags        []string
	Deferred    bool
	Description string
}

func DefaultRegistry() []Capability {
	caps := []Capability{
		{
			Name:        "push0_execution",
			MinimumFork: "shanghai",
			Status:      StatusActive,
			Description: "Validate PUSH0 fork gating and post-fork execution semantics.",
			Tags:        []string{"shanghai", "opcode", "gate"},
		},
		{
			Name:        "mcopy_execution",
			MinimumFork: "cancun",
			Status:      StatusActive,
			Description: "Validate MCOPY fork gating and post-fork execution semantics.",
			Tags:        []string{"cancun", "opcode", "gate"},
		},
		{
			Name:        "transient_storage_lifecycle",
			MinimumFork: "cancun",
			Status:      StatusActive,
			Description: "Validate TSTORE/TLOAD fork gating and transient lifetime semantics.",
			Tags:        []string{"cancun", "opcode", "semantics"},
		},
		{
			Name:        "cancun_header_surface",
			MinimumFork: "cancun",
			Status:      StatusActive,
			Description: "Validate Cancun-era block/RPC header fields on the running chain.",
			Tags:        []string{"cancun", "rpc", "header"},
		},
		{
			Name:        "blob_tx_submission",
			MinimumFork: "cancun",
			Status:      StatusDeferred,
			Deferred:    true,
			Reason:      "Deferred: blob transactions are temporarily disabled at the txpool layer on this chain.",
			Description: "Reserved Cancun capability slot for future blob tx submission/inclusion coverage.",
			Tags:        []string{"cancun", "blob", "deferred"},
		},
		{
			Name:        "fixheader_rpc_surface",
			MinimumFork: "fixheader",
			Status:      StatusActive,
			Description: "Validate FixHeader-era block/RPC header fields such as parentBeaconBlockRoot and post-fork baseFee handling on the running chain.",
			Tags:        []string{"fixheader", "rpc", "header"},
		},
		{
			Name:        "posa_contract_surface",
			MinimumFork: "posa",
			Status:      StatusActive,
			Description: "Validate PoSA system-contract surface: pre-PoSA contract set absent, post-PoSA validators/proposal/punish/staking contracts deployed at the canonical addresses.",
			Tags:        []string{"posa", "contracts", "surface"},
		},
		{
			Name:        "posa_proposal_wiring_surface",
			MinimumFork: "posa",
			Status:      StatusActive,
			Description: "Validate PoSA proposal-contract initialization and canonical validator/punish/staking wiring on the running chain.",
			Tags:        []string{"posa", "contracts", "wiring"},
		},
		{
			Name:        "posa_proposal_params_surface",
			MinimumFork: "posa",
			Status:      StatusActive,
			Description: "Validate PoSA proposal-contract parameter surface against the harness-configured epoch and test-friendly governance settings.",
			Tags:        []string{"posa", "contracts", "params"},
		},
		{
			Name:        "prague_rpc_surface",
			MinimumFork: "prague",
			Status:      StatusActive,
			Description: "Validate Prague-era RPC surface fields such as requestsHash on the running chain.",
			Tags:        []string{"prague", "rpc", "surface"},
		},
		{
			Name:        "prague_eth_config_precompile_surface",
			MinimumFork: "prague",
			Status:      StatusActive,
			Description: "Validate Prague eth_config precompile exposure for the BLS12-381 precompile set on the running chain.",
			Tags:        []string{"prague", "rpc", "precompile"},
		},
		{
			Name:        "prague_setcode_tx",
			MinimumFork: "prague",
			Status:      StatusActive,
			Description: "Validate Prague SetCodeTx fork gating and successful delegation-code installation on the running chain.",
			Tags:        []string{"prague", "tx", "semantics"},
		},
		{
			Name:        "prague_capability_matrix",
			MinimumFork: "prague",
			Status:      StatusDeferred,
			Deferred:    true,
			Reason:      "Deferred: Prague semantic capability set is not fully populated in this repository yet; keep suite inheritance ready and add concrete cases incrementally.",
			Description: "Reserved Prague suite slot for future semantic protocol capability coverage.",
			Tags:        []string{"prague", "deferred", "roadmap"},
		},
		{
			Name:        "bpo1_blob_schedule",
			MinimumFork: "bpo1",
			Status:      StatusActive,
			Description: "Validate BPO1 blob schedule activation through eth_config on the running chain.",
			Tags:        []string{"bpo1", "blob", "rpc"},
		},
		{
			Name:        "bpo2_blob_schedule",
			MinimumFork: "bpo2",
			Status:      StatusActive,
			Description: "Validate BPO2 blob schedule activation through eth_config on the running chain.",
			Tags:        []string{"bpo2", "blob", "rpc"},
		},
		{
			Name:        "osaka_engine_blob_api_transition",
			MinimumFork: "osaka",
			Status:      StatusActive,
			Description: "Validate Osaka authrpc blob API transition: pre-Osaka GetBlobsV2/V3 return null for a missing blob, while post-Osaka GetBlobsV3 exposes partial-response semantics as [null].",
			Tags:        []string{"osaka", "engine-api", "blob"},
		},
		{
			Name:        "osaka_engine_getpayload_transition",
			MinimumFork: "osaka",
			Status:      StatusActive,
			Description: "Validate Osaka authrpc getPayload transition: GetPayloadV4 works pre-Osaka while GetPayloadV5 works only after Osaka.",
			Tags:        []string{"osaka", "engine-api", "gate"},
		},
		{
			Name:        "osaka_eth_config_precompile_surface",
			MinimumFork: "osaka",
			Status:      StatusActive,
			Description: "Validate Osaka eth_config precompile exposure for the Osaka-only P256VERIFY precompile on the running chain.",
			Tags:        []string{"osaka", "rpc", "precompile"},
		},
		{
			Name:        "osaka_modexp_gas_semantics",
			MinimumFork: "osaka",
			Status:      StatusActive,
			Description: "Validate Osaka MODEXP gas-threshold behavior: an empty-input call succeeds with 21300 gas pre-Osaka but requires at least 21600 gas after Osaka.",
			Tags:        []string{"osaka", "precompile", "gas"},
		},
		{
			Name:        "osaka_p256verify_precompile",
			MinimumFork: "osaka",
			Status:      StatusActive,
			Description: "Validate Osaka P256VERIFY precompile activation and valid-signature execution semantics.",
			Tags:        []string{"osaka", "precompile", "semantics"},
		},
		{
			Name:        "osaka_tx_gas_cap",
			MinimumFork: "osaka",
			Status:      StatusActive,
			Description: "Validate Osaka max transaction gas-cap enforcement on the running chain.",
			Tags:        []string{"osaka", "tx", "policy"},
		},
		{
			Name:        "osaka_capability_matrix",
			MinimumFork: "osaka",
			Status:      StatusDeferred,
			Deferred:    true,
			Reason:      "Deferred: Osaka semantic capability set is not fully populated in this repository yet; keep suite inheritance ready and add concrete cases incrementally.",
			Description: "Reserved Osaka suite slot for future semantic protocol capability coverage.",
			Tags:        []string{"osaka", "deferred", "roadmap"},
		},
	}
	sort.Slice(caps, func(i, j int) bool {
		return caps[i].Name < caps[j].Name
	})
	return caps
}

func FilterByFork(caps []Capability, fork string) []Capability {
	fork = NormalizeFork(fork)
	if fork == "" || fork == "all" {
		out := make([]Capability, len(caps))
		copy(out, caps)
		return out
	}
	maxIndex, ok := ForkOrderIndex(fork)
	if !ok {
		return nil
	}
	out := make([]Capability, 0, len(caps))
	for _, cap := range caps {
		idx, capKnown := ForkOrderIndex(cap.MinimumFork)
		if !capKnown {
			continue
		}
		if idx <= maxIndex {
			out = append(out, cap)
		}
	}
	return out
}

func ValidateForkSelection(fork string) error {
	fork = NormalizeFork(fork)
	if fork == "" || fork == "all" {
		return nil
	}
	if _, ok := ForkOrderIndex(fork); ok {
		return nil
	}
	return fmt.Errorf("unsupported fork capability suite %q", fork)
}
