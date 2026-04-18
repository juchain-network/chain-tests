package tests

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestZ_AddValidatorMissingSignerTriggersPunishAndJail(t *testing.T) {
	if ctx == nil {
		t.Fatalf("context not initialized")
	}
	if !testkit.IsMultiValidatorSeparatedMode(ctx, 3) {
		t.Skip("requires multi-validator separated-signer topology")
	}

	ensureMinActiveValidators(t, 3, 2)
	epochBig, err := ctx.Proposal.Epoch(nil)
	if err != nil || epochBig == nil || epochBig.Sign() <= 0 {
		t.Fatalf("read epoch failed: %v", err)
	}
	punishThreshold, err := ctx.Proposal.PunishThreshold(nil)
	if err != nil || punishThreshold == nil || punishThreshold.Sign() <= 0 {
		t.Fatalf("read punishThreshold failed: %v", err)
	}
	removeThreshold, err := ctx.Proposal.RemoveThreshold(nil)
	if err != nil || removeThreshold == nil || removeThreshold.Sign() <= 0 {
		t.Fatalf("read removeThreshold failed: %v", err)
	}
	epoch := epochBig.Uint64()
	if epoch < 240 {
		t.Skipf("requires long epoch for add-validator punish scenario, got %d", epoch)
	}

	for {
		head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || head == nil {
			t.Fatalf("read latest header before add-validator setup failed: %v", err)
		}
		nextCheckpoint := ((head.Number.Uint64() / epoch) + 1) * epoch
		if nextCheckpoint <= head.Number.Uint64()+25 {
			if _, err := testkit.WaitUntilHeightOrStall(ctx, "add-validator-punish-align", nextCheckpoint+1, 15*time.Second, testkit.LongWindowTimeout(4)); err != nil {
				t.Fatalf("%v", err)
			}
			continue
		}
		break
	}

	candidate, err := createAndRegisterValidatorWithExplicitSigner(t, "Punish Candidate")
	if err != nil {
		t.Fatalf("create/register validator with explicit signer failed: %v", err)
	}
	if active, _ := ctx.Validators.IsValidatorActive(nil, candidate.Validator); active {
		t.Fatalf("candidate validator unexpectedly active before activation checkpoint")
	}
	preHistory, err := ctx.Validators.GetValidatorBySignerHistory(nil, candidate.Signer)
	if err != nil {
		t.Fatalf("getValidatorBySignerHistory before activation failed: %v", err)
	}
	if preHistory != (common.Address{}) {
		t.Fatalf("candidate signer unexpectedly entered history before activation: got=%s want=%s", preHistory.Hex(), common.Address{}.Hex())
	}

	head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || head == nil {
		t.Fatalf("read latest header after candidate registration failed: %v", err)
	}
	activationCheckpoint := ((head.Number.Uint64() / epoch) + 1) * epoch
	if _, err := testkit.WaitUntilHeightOrStall(ctx, "add-validator-punish-activation", activationCheckpoint+1, 15*time.Second, testkit.LongWindowTimeout(epoch)); err != nil {
		t.Fatalf("%v", err)
	}

	err = testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 30,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		active, err := ctx.Validators.IsValidatorActive(nil, candidate.Validator)
		if err != nil || !active {
			return false, err
		}
		historyOwner, err := ctx.Validators.GetValidatorBySignerHistory(nil, candidate.Signer)
		if err != nil || historyOwner != candidate.Validator {
			return false, err
		}
		topSigners, err := ctx.Validators.GetTopSigners(nil)
		if err != nil || testkit.AddressIndex(topSigners, candidate.Signer) < 0 {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("candidate did not enter active set with historical signer mapping in time: %v", err)
	}

	activeValidators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("get active validators after activation failed: %v", err)
	}
	if len(activeValidators) != 4 {
		t.Fatalf("expected 4 active validators after candidate activation, got %d", len(activeValidators))
	}

	otherSigners := make([]common.Address, 0, len(activeValidators)-1)
	for _, validator := range activeValidators {
		if validator == candidate.Validator {
			continue
		}
		signer, err := ctx.SignerAddressByValidator(validator)
		if err != nil {
			t.Fatalf("resolve signer for validator %s failed: %v", validator.Hex(), err)
		}
		otherSigners = append(otherSigners, signer)
	}

	punishBlocks := punishThreshold.Uint64()*uint64(len(activeValidators)) + uint64(len(activeValidators))
	jailBlocks := removeThreshold.Uint64()*uint64(len(activeValidators)) + uint64(len(activeValidators))
	thresholdHeight := activationCheckpoint + punishBlocks
	jailDeadline := activationCheckpoint + jailBlocks
	if thresholdHeight%epoch == 0 {
		thresholdHeight++
	}
	if jailDeadline%epoch == 0 {
		jailDeadline++
	}
	if jailDeadline >= activationCheckpoint+epoch {
		t.Fatalf("jail observation crossed next epoch boundary: activation=%d deadline=%d epoch=%d", activationCheckpoint, jailDeadline, epoch)
	}

	if _, err := testkit.WaitUntilHeightOrStall(ctx, "add-validator-punish-threshold", thresholdHeight, 15*time.Second, testkit.LongWindowTimeout(thresholdHeight-activationCheckpoint)); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"add-validator-punish-threshold-primary",
		thresholdHeight,
		ctx.BlockPollInterval(),
		30*time.Second,
	); err != nil {
		t.Fatalf("%v", err)
	}

	thresholdCall := testkit.CallAt(thresholdHeight)
	thresholdRecord, err := ctx.Punish.GetPunishRecord(thresholdCall, candidate.Validator)
	if err != nil {
		t.Fatalf("read candidate punish record at threshold failed: %v", err)
	}
	if thresholdRecord.Cmp(punishThreshold) < 0 {
		t.Fatalf("candidate punish record below threshold at height %d: got=%s want>=%s", thresholdHeight, thresholdRecord.String(), punishThreshold.String())
	}
	if thresholdRecord.Cmp(removeThreshold) >= 0 {
		t.Fatalf("candidate punish record reached remove threshold too early at height %d: got=%s remove=%s", thresholdHeight, thresholdRecord.String(), removeThreshold.String())
	}
	thresholdInfo, err := ctx.Staking.GetValidatorInfo(thresholdCall, candidate.Validator)
	if err != nil {
		t.Fatalf("read candidate staking info at threshold failed: %v", err)
	}
	if thresholdInfo.IsJailed {
		t.Fatalf("candidate validator unexpectedly jailed before removeThreshold at height %d", thresholdHeight)
	}
	thresholdRuntimeValidator, err := ctx.Validators.GetValidatorBySigner(thresholdCall, candidate.Signer)
	if err != nil {
		t.Fatalf("getValidatorBySigner for candidate signer at threshold failed: %v", err)
	}
	if thresholdRuntimeValidator != candidate.Validator {
		t.Fatalf("candidate runtime signer mapping mismatch at threshold: got=%s want=%s", thresholdRuntimeValidator.Hex(), candidate.Validator.Hex())
	}

	jailHeight, err := testkit.WaitForJailTransitionOrStall(ctx, candidate.Validator, thresholdHeight+1, jailDeadline, 15*time.Second)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"add-validator-punish-jail-primary",
		jailHeight,
		ctx.BlockPollInterval(),
		30*time.Second,
	); err != nil {
		t.Fatalf("%v", err)
	}

	jailCall := testkit.CallAt(jailHeight)
	jailRecord, err := ctx.Punish.GetPunishRecord(jailCall, candidate.Validator)
	if err != nil {
		t.Fatalf("read candidate punish record at jail height failed: %v", err)
	}
	if jailRecord.Sign() != 0 {
		t.Fatalf("candidate punish record should reset after jail: height=%d record=%s", jailHeight, jailRecord.String())
	}
	jailInfo, err := ctx.Staking.GetValidatorInfo(jailCall, candidate.Validator)
	if err != nil {
		t.Fatalf("read candidate staking info at jail height failed: %v", err)
	}
	if !jailInfo.IsJailed {
		t.Fatalf("candidate validator was not jailed by height %d", jailHeight)
	}
	activeAtJail, err := ctx.Validators.IsValidatorActive(jailCall, candidate.Validator)
	if err != nil {
		t.Fatalf("read candidate active state at jail height failed: %v", err)
	}
	if activeAtJail {
		t.Fatalf("candidate validator remained active after jail at height %d", jailHeight)
	}
	eligibleValidators, err := ctx.Validators.GetRewardEligibleValidatorsWithStakes(jailCall)
	if err != nil {
		t.Fatalf("read reward-eligible validators at jail height failed: %v", err)
	}
	if testkit.AddressIndex(eligibleValidators.Validators, candidate.Validator) >= 0 {
		t.Fatalf("candidate validator still present in reward-eligible validator set at height %d", jailHeight)
	}
	eligibleSigners, err := ctx.Validators.GetRewardEligibleSignersWithStakes(jailCall)
	if err != nil {
		t.Fatalf("read reward-eligible signers at jail height failed: %v", err)
	}
	if testkit.AddressIndex(eligibleSigners.Signers, candidate.Signer) >= 0 {
		t.Fatalf("candidate signer still present in reward-eligible signer set at height %d", jailHeight)
	}

	observedDuringWindow, err := testkit.CollectCoinbaseSet(ctx, activationCheckpoint+1, jailHeight)
	if err != nil {
		t.Fatalf("collect coinbases through jail window failed: %v", err)
	}
	for _, signer := range otherSigners {
		if !observedDuringWindow[signer] {
			t.Fatalf("non-candidate signer %s never produced blocks during punish window; observed=%v", signer.Hex(), testkit.CoinbaseSetKeys(observedDuringWindow))
		}
	}
	if observedDuringWindow[candidate.Signer] {
		t.Fatalf("candidate signer %s unexpectedly produced blocks without sync miner start; observed=%v", candidate.Signer.Hex(), testkit.CoinbaseSetKeys(observedDuringWindow))
	}
	if observedDuringWindow[candidate.Validator] {
		t.Fatalf("candidate validator cold address %s unexpectedly appeared as coinbase; observed=%v", candidate.Validator.Hex(), testkit.CoinbaseSetKeys(observedDuringWindow))
	}
}
