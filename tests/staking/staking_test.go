package tests

import (
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
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
	t.Log("Creating and registering new candidate validator account...")
	var valAddr = common.Address{}
	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 3,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		_, addr, errCreate := createAndRegisterValidator(t, "New Validator Candidate")
		if errCreate == nil {
			valAddr = addr
			return true, nil
		}
		msg := strings.ToLower(errCreate.Error())
		if strings.Contains(msg, "proposal expired") ||
			strings.Contains(msg, "must repropose") ||
			strings.Contains(msg, "condition not met") ||
			strings.Contains(msg, "failed to create proposal") ||
			strings.Contains(msg, "failed to pass proposal") {
			return false, nil
		}
		return false, errCreate
	})
	utils.AssertNoError(t, err, "failed to create/register validator")

	// 5. Verify Registration
	isRegistered := false
	err = testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 6,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		registered, errCheck := ctx.Validators.IsValidatorExist(nil, valAddr)
		if errCheck != nil {
			return false, errCheck
		}
		isRegistered = registered
		if registered {
			return true, nil
		}
		info, errInfo := ctx.Staking.GetValidatorInfo(nil, valAddr)
		if errInfo == nil && info.IsRegistered {
			isRegistered = true
			return true, nil
		}
		return false, nil
	})
	utils.AssertNoError(t, err, "failed to check validator registration state")
	utils.AssertTrue(t, isRegistered, "Validator should be registered")

	t.Logf("Validator %s successfully registered!", valAddr.Hex())
}
