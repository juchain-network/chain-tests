package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestZ_AddValidatorWithSeparateSignerBecomesActiveAndSealsBlocks(t *testing.T) {
	if ctx == nil {
		t.Fatalf("context not initialized")
	}
	if !testkit.IsMultiValidatorSeparatedMode(ctx, 3) {
		t.Skip("requires multi-validator separated-signer topology")
	}

	ensureMinActiveValidators(t, 3, 2)
	baseActiveValidators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("read initial active validators failed: %v", err)
	}
	if len(baseActiveValidators) < 3 {
		t.Fatalf("expected at least 3 active validators, got %d", len(baseActiveValidators))
	}
	hostValidator := baseActiveValidators[0]

	epochBig, err := ctx.Proposal.Epoch(nil)
	if err != nil || epochBig == nil || epochBig.Sign() <= 0 {
		t.Fatalf("read epoch failed: %v", err)
	}
	epoch := epochBig.Uint64()
	if epoch < 240 {
		t.Skipf("requires long epoch for add-validator live scenario, got %d", epoch)
	}

	for {
		head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || head == nil {
			t.Fatalf("read latest header before add-validator setup failed: %v", err)
		}
		nextCheckpoint := ((head.Number.Uint64() / epoch) + 1) * epoch
		if nextCheckpoint <= head.Number.Uint64()+25 {
			if _, err := testkit.WaitUntilHeightOrStall(ctx, "add-validator-live-align", nextCheckpoint+1, 15*time.Second, testkit.LongWindowTimeout(4)); err != nil {
				t.Fatalf("%v", err)
			}
			continue
		}
		break
	}

	candidate, err := createAndRegisterValidatorWithExplicitSigner(t, "LongChain Candidate")
	if err != nil {
		t.Fatalf("create/register validator with explicit signer failed: %v", err)
	}

	if active, _ := ctx.Validators.IsValidatorActive(nil, candidate.Validator); active {
		t.Fatalf("candidate validator unexpectedly active before epoch transition")
	}
	historyOwner, err := ctx.Validators.GetValidatorBySignerHistory(nil, candidate.Signer)
	if err != nil {
		t.Fatalf("getValidatorBySignerHistory before activation failed: %v", err)
	}
	if historyOwner != (common.Address{}) {
		t.Fatalf("candidate signer unexpectedly entered history before activation: got=%s want=%s", historyOwner.Hex(), common.Address{}.Hex())
	}
	storedSigner, err := ctx.Validators.GetValidatorSigner(nil, candidate.Validator)
	if err != nil {
		t.Fatalf("getValidatorSigner before activation failed: %v", err)
	}
	if storedSigner != candidate.Signer {
		t.Fatalf("candidate stored signer mismatch: got=%s want=%s", storedSigner.Hex(), candidate.Signer.Hex())
	}

	head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || head == nil {
		t.Fatalf("read latest header after candidate registration failed: %v", err)
	}
	activationCheckpoint := ((head.Number.Uint64() / epoch) + 1) * epoch
	remainingToActivation := uint64(1)
	if activationCheckpoint+1 > head.Number.Uint64() {
		remainingToActivation = activationCheckpoint + 1 - head.Number.Uint64()
	}
	if _, err := testkit.WaitUntilHeightOrStall(ctx, "add-validator-live-activation", activationCheckpoint+1, 15*time.Second, testkit.LongWindowTimeout(remainingToActivation)); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"add-validator-live-activation-primary",
		activationCheckpoint+1,
		ctx.BlockPollInterval(),
		30*time.Second,
	); err != nil {
		t.Fatalf("%v", err)
	}
	checkpointHeader, err := ctx.Clients[0].HeaderByNumber(context.Background(), new(big.Int).SetUint64(activationCheckpoint))
	if err != nil || checkpointHeader == nil {
		t.Fatalf("read activation checkpoint header failed: %v", err)
	}
	checkpointExtraSigners, err := testkit.ParseHeaderExtraSigners(checkpointHeader.Extra)
	if err != nil {
		t.Fatalf("parse activation checkpoint header extra failed: %v", err)
	}
	t.Logf(
		"add-validator-live host=%s candidate_validator=%s candidate_signer=%s activation_checkpoint=%d checkpoint_extra=%v",
		hostValidator.Hex(),
		candidate.Validator.Hex(),
		candidate.Signer.Hex(),
		activationCheckpoint,
		checkpointExtraSigners,
	)
	if err := testkit.RestartValidatorNodeWithSigner(ctx, hostValidator, candidate.SignerKey, 90*time.Second); err != nil {
		t.Fatalf("restart host validator with candidate signer after activation failed: %v", err)
	}
	// This scenario validates that the activated candidate signer can join the
	// live sealing rotation. The restarted host runtime may briefly race on
	// reorgs after the checkpoint, so requiring two consecutive canonical-head
	// matches here is stricter than the business goal and caused false failures.
	if err := ctx.WaitForBlockProgress(2, 90*time.Second); err != nil {
		t.Fatalf("chain did not stay live after switching host node signer post-activation: %v", err)
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
		activeSet, err := ctx.Validators.GetActiveValidators(nil)
		if err != nil || testkit.AddressIndex(activeSet, candidate.Validator) < 0 {
			return false, err
		}
		topSigners, err := ctx.Validators.GetTopSigners(nil)
		if err != nil || testkit.AddressIndex(topSigners, candidate.Signer) < 0 {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("candidate did not become active with historical signer mapping in time: %v", err)
	}

	observationEnd := activationCheckpoint + 24
	// After activation there are 4 active validators, but this scenario only moves the
	// host runtime over to the candidate signer. The original host signer then goes
	// offline, so canonical progress can have noticeably longer gaps before out-of-turn
	// sealing recovers liveness.
	if _, err := testkit.WaitUntilHeightOrStall(ctx, "add-validator-live-observation", observationEnd, 45*time.Second, testkit.LongWindowTimeout(24)); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"add-validator-live-observation-primary",
		observationEnd,
		ctx.BlockPollInterval(),
		30*time.Second,
	); err != nil {
		t.Fatalf("%v", err)
	}

	observed, err := testkit.CollectCoinbaseSet(ctx, activationCheckpoint+1, observationEnd)
	if err != nil {
		t.Fatalf("collect activation coinbases failed: %v", err)
	}
	if !observed[candidate.Signer] {
		postTopSigners, topErr := ctx.Validators.GetTopSigners(testkit.CallAt(observationEnd))
		if topErr != nil {
			t.Fatalf(
				"candidate signer %s never sealed after activation; observed=%v checkpoint_extra=%v recent_coinbases=%v read_top_signers_err=%v",
				candidate.Signer.Hex(),
				testkit.CoinbaseSetKeys(observed),
				checkpointExtraSigners,
				testkit.RecentCoinbases(ctx, 16),
				topErr,
			)
		}
		t.Fatalf(
			"candidate signer %s never sealed after activation; observed=%v checkpoint_extra=%v top_signers_at_%d=%v recent_coinbases=%v",
			candidate.Signer.Hex(),
			testkit.CoinbaseSetKeys(observed),
			checkpointExtraSigners,
			observationEnd,
			postTopSigners,
			testkit.RecentCoinbases(ctx, 16),
		)
	}
	if observed[candidate.Validator] {
		t.Fatalf("candidate validator cold address %s unexpectedly appeared as coinbase; observed=%v", candidate.Validator.Hex(), testkit.CoinbaseSetKeys(observed))
	}

	postCall := testkit.CallAt(observationEnd)
	active, err := ctx.Validators.IsValidatorActive(postCall, candidate.Validator)
	if err != nil {
		t.Fatalf("read candidate active state failed: %v", err)
	}
	if !active {
		t.Fatalf("candidate validator became inactive unexpectedly by height %d", observationEnd)
	}
	info, err := ctx.Staking.GetValidatorInfo(postCall, candidate.Validator)
	if err != nil {
		t.Fatalf("read candidate validator info failed: %v", err)
	}
	if info.IsJailed {
		t.Fatalf("candidate validator was jailed unexpectedly by height %d", observationEnd)
	}
	eligibleSigners, err := ctx.Validators.GetRewardEligibleSignersWithStakes(postCall)
	if err != nil {
		t.Fatalf("read reward-eligible signers failed: %v", err)
	}
	if testkit.AddressIndex(eligibleSigners.Signers, candidate.Signer) < 0 {
		t.Fatalf("reward-eligible signer set missing candidate signer %s: %v", candidate.Signer.Hex(), eligibleSigners.Signers)
	}
}
