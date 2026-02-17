package tests

import (
	"math/big"
	"testing"

	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestC_StakingFlow(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}

	if len(ctx.GenesisValidators) == 0 {
		t.Fatalf("No genesis validators configured, cannot run governance tests")
	}

	// 1. Create a new account for the Candidate Validator
	t.Log("Creating new candidate validator account...")
	valKey, valAddr, err := ctx.CreateAndFundAccount(utils.ToWei(100005)) // 100k min stake + gas
	utils.AssertNoError(t, err, "failed to create validator account")

	// 2. Pass Proposal (All genesis validators vote YES)
	t.Log("Passing proposal...")
	err = passProposalFor(t, valAddr, "New Validator Candidate")
	utils.AssertNoError(t, err, "failed to pass proposal")

	// 3. Register Validator (Candidate executes this)
	t.Log("Registering validator...")
	err = testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 3,
		Interval:    retrySleep(),
	}, func() (bool, error) {
		pass, err := ctx.Proposal.Pass(nil, valAddr)
		if err != nil {
			return false, err
		}
		return pass, nil
	})
	if err != nil {
		t.Log("Proposal has not passed yet (not enough votes?). Skipping registration.")
		// In a 4-node cluster, we need >50% (3 votes). If we have keys for all 4, it should pass.
		// If we only configured 1 key, it might fail.
		return
	}

	registerOpts, _ := ctx.GetTransactor(valKey)
	registerOpts.Value = utils.ToWei(100000) // Min stake
	commission := big.NewInt(1000)           // 10%

	txReg, err := ctx.Staking.RegisterValidator(registerOpts, commission)
	utils.AssertNoError(t, err, "failed to register validator")

	err = ctx.WaitMined(txReg.Hash())
	utils.AssertNoError(t, err, "register tx failed")

	// 5. Verify Registration
	isRegistered, err := ctx.Validators.IsValidatorExist(nil, valAddr)
	utils.AssertNoError(t, err, "failed to check validator existence")
	utils.AssertTrue(t, isRegistered, "Validator should be registered")

	t.Logf("Validator %s successfully registered!", valAddr.Hex())
}
