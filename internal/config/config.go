package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SourcePath string `yaml:"-"`

	RPCs []string `yaml:"rpcs"` // List of RPC endpoints (e.g., node 1, node 2...)
	// Optional: Per-validator RPCs aligned with Validators order.
	ValidatorRPCs []string `yaml:"validator_rpcs"`
	// Optional: Dedicated sync-node RPC endpoint.
	SyncRPC string `yaml:"sync_rpc"`
	// Optional: Explicit per-node RPC endpoints for multi-node health checks.
	NodeRPCs []NodeRPC `yaml:"node_rpcs"`

	Network struct {
		Epoch uint64 `yaml:"epoch"`
	} `yaml:"network"`

	Fork struct {
		Mode          string `yaml:"mode"`
		Target        string `yaml:"target"`
		ScheduledTime int64  `yaml:"scheduled_time"`
		DelaySeconds  int64  `yaml:"delay_seconds"`
		Schedule      struct {
			ShanghaiTime  int64 `yaml:"shanghai_time"`
			CancunTime    int64 `yaml:"cancun_time"`
			FixHeaderTime int64 `yaml:"fix_header_time"`
			PosaTime      int64 `yaml:"posa_time"`
		} `yaml:"schedule"`
		Override struct {
			PosaTime       int64    `yaml:"posa_time"`
			PosaValidators []string `yaml:"posa_validators"`
			PosaSigners    []string `yaml:"posa_signers"`
		} `yaml:"override"`
	} `yaml:"fork"`

	Runtime struct {
		Backend  string `yaml:"backend"`
		ImplMode string `yaml:"impl_mode"`
		Impl     string `yaml:"impl"`
	} `yaml:"runtime"`

	ValidatorAuth struct {
		Mode string `yaml:"mode"`
	} `yaml:"validator_auth"`

	RuntimeNodes []RuntimeNode `yaml:"runtime_nodes"`

	Blacklist struct {
		Enabled         bool   `yaml:"enabled"`
		Mode            string `yaml:"mode"`
		ContractAddress string `yaml:"contract_address"`
		AlertFailOpen   bool   `yaml:"alert_fail_open"`
		Mock            struct {
			Predeploy bool   `yaml:"predeploy"`
			CodeFile  string `yaml:"code_file"`
			ABIFile   string `yaml:"abi_file"`
		} `yaml:"mock"`
	} `yaml:"blacklist"`

	// The rich account that funds test accounts
	Funder struct {
		PrivateKey string `yaml:"private_key"`
		Address    string `yaml:"address"`
	} `yaml:"funder"`

	// Optional: Pre-existing validators keys to test proposal voting etc.
	Validators []Validator `yaml:"validators"`

	// Test settings
	Test struct {
		FundingAmount string `yaml:"funding_amount"` // Amount to fund each test account (in Wei)
		Profile       string `yaml:"profile"`        // Test profile name: fast/default/edge
		Timing        struct {
			RetryPollMS int64 `yaml:"retry_poll_ms"` // Poll interval for retry loops
			BlockPollMS int64 `yaml:"block_poll_ms"` // Poll interval for block progress checks
		} `yaml:"timing"`
		Smoke struct {
			ObserveSeconds int64 `yaml:"observe_seconds"` // Smoke observe window in seconds
		} `yaml:"smoke"`
		Params struct {
			ProposalCooldown   int64 `yaml:"proposal_cooldown"`
			UnbondingPeriod    int64 `yaml:"unbonding_period"`
			ValidatorUnjail    int64 `yaml:"validator_unjail_period"`
			WithdrawProfit     int64 `yaml:"withdraw_profit_period"`
			CommissionCooldown int64 `yaml:"commission_update_cooldown"`
			ProposalLasting    int64 `yaml:"proposal_lasting_period"`
		} `yaml:"params"`
	} `yaml:"test"`
}

type NodeRPC struct {
	Name string `yaml:"name"`
	Role string `yaml:"role"`
	URL  string `yaml:"url"`
}

type Validator struct {
	PrivateKey       string `yaml:"private_key"`
	Address          string `yaml:"address"`
	SignerPrivateKey string `yaml:"signer_private_key"`
	SignerAddress    string `yaml:"signer_address"`
	FeeAddress       string `yaml:"fee_address"`
}

type RuntimeNode struct {
	Name             string `yaml:"name"`
	Role             string `yaml:"role"`
	Impl             string `yaml:"impl"`
	Binary           string `yaml:"binary"`
	ValidatorKey     string `yaml:"validator_key"`
	ValidatorAddress string `yaml:"validator_address"`
	SignerKey        string `yaml:"signer_key"`
	SignerAddress    string `yaml:"signer_address"`
	FeeAddress       string `yaml:"fee_address"`
	KeystoreFile     string `yaml:"keystore_file"`
	KeystoreAddress  string `yaml:"keystore_address"`
	PasswordFile     string `yaml:"password_file"`
}

func LoadConfig(path string) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config path: %w", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	cfg.SourcePath = absPath

	return &cfg, nil
}
