package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RPCs []string `yaml:"rpcs"` // List of RPC endpoints (e.g., node 1, node 2...)
	// Optional: Per-validator RPCs aligned with Validators order.
	ValidatorRPCs []string `yaml:"validator_rpcs"`

	Network struct {
		Epoch uint64 `yaml:"epoch"`
	} `yaml:"network"`

	// The rich account that funds test accounts
	Funder struct {
		PrivateKey string `yaml:"private_key"`
		Address    string `yaml:"address"`
	} `yaml:"funder"`

	// Optional: Pre-existing validators keys to test proposal voting etc.
	Validators []struct {
		PrivateKey string `yaml:"private_key"`
		Address    string `yaml:"address"`
	} `yaml:"validators"`

	// Test settings
	Test struct {
		FundingAmount string `yaml:"funding_amount"` // Amount to fund each test account (in Wei)
	} `yaml:"test"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
