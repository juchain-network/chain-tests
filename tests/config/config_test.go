package tests

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/internal/utils"
)

// Config IDs from Proposal.sol
const (
	ConfigID_ProposalLastingPeriod    = 0
	ConfigID_PunishThreshold          = 1
	ConfigID_RemoveThreshold          = 2
	ConfigID_DecreaseRate             = 3
	ConfigID_WithdrawProfitPeriod     = 4
	ConfigID_BlockReward              = 5
	ConfigID_UnbondingPeriod          = 6
	ConfigID_ValidatorUnjailPeriod    = 7
	ConfigID_MinValidatorStake        = 8
	ConfigID_MaxValidators            = 9
	ConfigID_MinDelegation            = 10
	ConfigID_MinUndelegation          = 11
	ConfigID_DoubleSignSlashAmount    = 12
	ConfigID_DoubleSignRewardAmount   = 13
	ConfigID_BurnAddress              = 14
	ConfigID_DoubleSignWindow         = 15
	ConfigID_CommissionUpdateCooldown = 16
	ConfigID_BaseRewardRatio          = 17
	ConfigID_MaxCommissionRate        = 18
	ConfigID_ProposalCooldown         = 19
)

func TestA_SystemConfigSetup(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized or no validators")
	}

	// Define target parameters for testing environment
	targets := []struct {
		name string
		cid  int64
		val  *big.Int
	}{
		{"ProposalCooldown", ConfigID_ProposalCooldown, big.NewInt(1)},
		{"UnbondingPeriod", ConfigID_UnbondingPeriod, big.NewInt(10)},
		{"ValidatorUnjailPeriod", ConfigID_ValidatorUnjailPeriod, big.NewInt(10)},
		{"WithdrawProfitPeriod", ConfigID_WithdrawProfitPeriod, big.NewInt(5)},
		{"MinValidatorStake", ConfigID_MinValidatorStake, utils.ToWei(1)},
		{"MinDelegation", ConfigID_MinDelegation, utils.ToWei(1)},
		{"CommissionUpdateCooldown", ConfigID_CommissionUpdateCooldown, big.NewInt(5)},
		{"ProposalLastingPeriod", ConfigID_ProposalLastingPeriod, big.NewInt(100)},
	}

	for _, target := range targets {
		t.Logf("Verifying %s is %v...", target.name, target.val)
		current, err := ctx.GetConfigValue(target.cid)
		utils.AssertNoError(t, err, fmt.Sprintf("failed to read %s", target.name))
		if current.Cmp(target.val) != 0 {
			t.Logf("%s mismatch (expected %v, got %v). Attempting to update...", target.name, target.val, current)
			if err := ctx.EnsureConfig(target.cid, target.val, current); err != nil {
				t.Fatalf("%s update failed: %v", target.name, err)
			}
			// Allow state to settle after proposal pass.
			waitBlocks(t, 1)
			current, err = ctx.GetConfigValue(target.cid)
			utils.AssertNoError(t, err, fmt.Sprintf("failed to re-read %s", target.name))
			if current.Cmp(target.val) != 0 {
				t.Fatalf("%s mismatch after update: expected %v, got %v", target.name, target.val, current)
			}
		}
	}
}

func TestB_ConfigBoundaryChecks(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	// Wait for any previous cooldown to expire
	t.Log("Waiting for potential proposal cooldown...")
	waitBlocks(t, 1)

	proposerCounter := 0
	runRevertTest := func(name string, cid uint256, val *big.Int, expectedErr string) {
		t.Run(name, func(t *testing.T) {
			// Find an active proposer
			var proposerKey *ecdsa.PrivateKey
			for i := 0; i < len(ctx.GenesisValidators); i++ {
				k := ctx.GenesisValidators[proposerCounter%len(ctx.GenesisValidators)]
				proposerCounter++
				addr := crypto.PubkeyToAddress(k.PublicKey)
				if active, _ := ctx.Validators.IsValidatorActive(nil, addr); active {
					proposerKey = k
					break
				}
			}
			if proposerKey == nil {
				t.Fatal("no active proposers")
			}

			opts, _ := ctx.GetTransactor(proposerKey)
			_, err := ctx.Proposal.CreateUpdateConfigProposal(opts, big.NewInt(int64(cid)), val)

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", expectedErr)
			}
			if !strings.Contains(err.Error(), expectedErr) {
				t.Logf("Got error: %v", err)
				t.Errorf("expected error %q, got %q", expectedErr, err.Error())
			}

			waitBlocks(t, 1)
		})
	}

	// [C-02] General Validation
	runRevertTest("Invalid Config ID", 20, big.NewInt(100), "Invalid config ID")
	runRevertTest("Zero Value", ConfigID_ProposalCooldown, big.NewInt(0), "Config value must be positive")

	// [C-03] Threshold Logic
	// Assuming current values: Punish=24, Remove=48, Decrease=24
	runRevertTest("Punish >= Remove", ConfigID_PunishThreshold, big.NewInt(48), "punishThreshold must be < removeThreshold")
	runRevertTest("Remove <= Punish", ConfigID_RemoveThreshold, big.NewInt(24), "removeThreshold must be > punishThreshold")
	runRevertTest("Decrease > Remove", ConfigID_DecreaseRate, big.NewInt(49), "decreaseRate must be <= removeThreshold")

	// [C-04] Consensus & Safety
	runRevertTest("Max Validators Overflow", ConfigID_MaxValidators, big.NewInt(22), "maxValidators exceeds consensus limit")
	// Generic positive check catches zero address first
	runRevertTest("Zero Burn Address", ConfigID_BurnAddress, big.NewInt(0), "Config value must be positive")
	// Burn address out of uint160 range
	burnTooLarge := new(big.Int).Lsh(big.NewInt(1), 160)
	runRevertTest("Burn Address Overflow", ConfigID_BurnAddress, burnTooLarge, "burnAddress invalid")

	// [C-05] Economic
	// Default DoubleSignSlash=50000 ether, Reward=10000 ether
	// Slash < Reward -> Set Slash to 1 wei
	runRevertTest("Slash < Reward", ConfigID_DoubleSignSlashAmount, big.NewInt(1), "doubleSignSlashAmount must be >= doubleSignRewardAmount")

	// Reward > Slash -> Set Reward to 60000 ether
	rewardTooHigh := utils.ToWei(60000)
	runRevertTest("Reward > Slash", ConfigID_DoubleSignRewardAmount, rewardTooHigh, "doubleSignRewardAmount must be <= doubleSignSlashAmount")

	// Invalid Base Ratio (> 10000)
	runRevertTest("Invalid Base Ratio", ConfigID_BaseRewardRatio, big.NewInt(10001), "baseRewardRatio must be <= 10000")

	// Invalid Max Commission (> 10000)
	runRevertTest("Invalid Max Commission", ConfigID_MaxCommissionRate, big.NewInt(10001), "maxCommissionRate must be <= 10000")
}

// Helper type for uint256 since I used it in struct definition
type uint256 = uint64
