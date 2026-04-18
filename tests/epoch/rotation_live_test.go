package tests

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestZ_SignerRotationNewSignerContinuesSealingAfterCheckpoint(t *testing.T) {
	if ctx == nil {
		t.Fatalf("context not initialized")
	}
	if !testkit.IsMultiValidatorSeparatedMode(ctx, 3) {
		t.Skip("requires multi-validator separated-signer topology")
	}

	ensureMinActiveValidators(t, 3, 2)
	activeValidators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("get active validators failed: %v", err)
	}
	if len(activeValidators) != 3 {
		t.Skipf("requires exactly 3 active validators, got %d", len(activeValidators))
	}

	epochBig, err := ctx.Proposal.Epoch(nil)
	if err != nil || epochBig == nil || epochBig.Sign() <= 0 {
		t.Fatalf("read epoch failed: %v", err)
	}
	punishThreshold, err := ctx.Proposal.PunishThreshold(nil)
	if err != nil || punishThreshold == nil || punishThreshold.Sign() <= 0 {
		t.Fatalf("read punishThreshold failed: %v", err)
	}
	epoch := epochBig.Uint64()

	targetValidator := activeValidators[0]
	otherSigners := make([]common.Address, 0, len(activeValidators)-1)
	for _, validator := range activeValidators {
		if validator == targetValidator {
			continue
		}
		signer, err := ctx.SignerAddressByValidator(validator)
		if err != nil {
			t.Fatalf("resolve signer for validator %s failed: %v", validator.Hex(), err)
		}
		otherSigners = append(otherSigners, signer)
	}

	var expectedCheckpoint uint64
	for {
		head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || head == nil {
			t.Fatalf("read latest header before rotation failed: %v", err)
		}
		nextCheckpoint := ((head.Number.Uint64() / epoch) + 1) * epoch
		switch {
		case nextCheckpoint <= head.Number.Uint64()+3:
			if _, err := testkit.WaitUntilHeightOrStall(ctx, "rotation-live-checkpoint-skip", nextCheckpoint+1, 15*time.Second, testkit.LongWindowTimeout(4)); err != nil {
				t.Fatalf("%v", err)
			}
			continue
		case nextCheckpoint > head.Number.Uint64()+12:
			if _, err := testkit.WaitUntilHeightOrStall(ctx, "rotation-live-checkpoint-align", nextCheckpoint-12, 15*time.Second, testkit.LongWindowTimeout(nextCheckpoint-head.Number.Uint64())); err != nil {
				t.Fatalf("%v", err)
			}
			continue
		default:
			expectedCheckpoint = nextCheckpoint
		}
		break
	}

	rotation, _, err := testkit.PrepareValidatorSignerRotation(ctx, targetValidator)
	if err != nil {
		t.Fatalf("prepare signer rotation failed: %v", err)
	}
	if rotation.EffectiveBlock != expectedCheckpoint {
		t.Fatalf("rotation effective block mismatch: got=%d want=%d", rotation.EffectiveBlock, expectedCheckpoint)
	}

	testkit.MarkScenarioStage("checkpoint-wait")
	if _, err := testkit.WaitUntilHeightOrStall(ctx, "rotation-live-checkpoint", rotation.EffectiveBlock, 15*time.Second, testkit.LongWindowTimeout(rotation.EffectiveBlock)); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"rotation-live-checkpoint-primary",
		rotation.EffectiveBlock,
		ctx.BlockPollInterval(),
		30*time.Second,
	); err != nil {
		t.Fatalf("%v", err)
	}

	checkpointCall := testkit.CallAt(rotation.EffectiveBlock)
	runtimeSigners, err := ctx.Validators.GetTopSigners(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSigners at checkpoint failed: %v", err)
	}
	transitionSigners, err := ctx.Validators.GetTopSignersForEpochTransition(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSignersForEpochTransition at checkpoint failed: %v", err)
	}
	if testkit.AddressIndex(runtimeSigners, rotation.OldSigner) < 0 || testkit.AddressIndex(runtimeSigners, rotation.NewSigner) >= 0 {
		t.Fatalf("checkpoint runtime signer split mismatch: runtime=%v old=%s new=%s", runtimeSigners, rotation.OldSigner.Hex(), rotation.NewSigner.Hex())
	}
	if testkit.AddressIndex(transitionSigners, rotation.NewSigner) < 0 || testkit.AddressIndex(transitionSigners, rotation.OldSigner) >= 0 {
		t.Fatalf("checkpoint transition signer split mismatch: transition=%v old=%s new=%s", transitionSigners, rotation.OldSigner.Hex(), rotation.NewSigner.Hex())
	}

	headerN, err := ctx.Clients[0].HeaderByNumber(context.Background(), checkpointCall.BlockNumber)
	if err != nil || headerN == nil {
		t.Fatalf("read checkpoint header failed: %v", err)
	}
	extraSigners, err := testkit.ParseHeaderExtraSigners(headerN.Extra)
	if err != nil {
		t.Fatalf("parse checkpoint header extra failed: %v", err)
	}
	if testkit.AddressIndex(extraSigners, rotation.NewSigner) < 0 || testkit.AddressIndex(extraSigners, rotation.OldSigner) >= 0 {
		t.Fatalf("checkpoint header extra signer split mismatch: extra=%v old=%s new=%s", extraSigners, rotation.OldSigner.Hex(), rotation.NewSigner.Hex())
	}
	t.Logf("rotation-live target validator=%s old=%s new=%s", rotation.Validator.Hex(), rotation.OldSigner.Hex(), rotation.NewSigner.Hex())

	if err := testkit.RestartValidatorNodeWithSigner(ctx, rotation.Validator, rotation.NewSignerKey, 90*time.Second); err != nil {
		t.Fatalf("restart validator with rotated signer failed: %v", err)
	}
	// Match the separated-signer live-switch semantics used by add-validator-live:
	// the business requirement here is that the chain stays live and the rotated
	// signer eventually participates in canonical sealing during the observation
	// window, not that the restarted validator immediately matches canonical head
	// for two consecutive blocks after restart.
	if err := ctx.WaitForBlockProgress(2, 90*time.Second); err != nil {
		t.Fatalf("chain did not stay live after restarting rotated validator: %v", err)
	}

	testkit.MarkScenarioStage("post-checkpoint-observation")
	observationEnd := rotation.EffectiveBlock + 12
	if _, err := testkit.WaitUntilHeightOrStall(ctx, "rotation-live-observation", observationEnd, 15*time.Second, testkit.LongWindowTimeout(12)); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"rotation-live-observation-primary",
		observationEnd,
		ctx.BlockPollInterval(),
		30*time.Second,
	); err != nil {
		t.Fatalf("%v", err)
	}

	observed, err := testkit.CollectCoinbaseSet(ctx, rotation.EffectiveBlock+1, observationEnd)
	if err != nil {
		t.Fatalf("collect post-checkpoint coinbases failed: %v", err)
	}
	if !observed[rotation.NewSigner] {
		t.Fatalf("rotated signer %s never produced blocks after restart; observed=%v", rotation.NewSigner.Hex(), testkit.CoinbaseSetKeys(observed))
	}
	if observed[rotation.OldSigner] {
		t.Fatalf("old signer %s unexpectedly produced post-checkpoint blocks; observed=%v", rotation.OldSigner.Hex(), testkit.CoinbaseSetKeys(observed))
	}
	for _, signer := range otherSigners {
		if !observed[signer] {
			t.Fatalf("other validator signer %s never produced blocks post-checkpoint; observed=%v", signer.Hex(), testkit.CoinbaseSetKeys(observed))
		}
	}

	postCall := testkit.CallAt(observationEnd)
	active, err := ctx.Validators.IsValidatorActive(postCall, rotation.Validator)
	if err != nil {
		t.Fatalf("check validator active state failed: %v", err)
	}
	if !active {
		t.Fatalf("rotated validator became inactive unexpectedly by height %d", observationEnd)
	}
	postRecord, err := ctx.Punish.GetPunishRecord(postCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read punish record after rotation failed: %v", err)
	}
	if postRecord.Cmp(punishThreshold) >= 0 {
		t.Fatalf("rotated validator punish record reached threshold unexpectedly: height=%d record=%s threshold=%s", observationEnd, postRecord.String(), punishThreshold.String())
	}
	eligibleSigners, err := ctx.Validators.GetRewardEligibleSignersWithStakes(postCall)
	if err != nil {
		t.Fatalf("read reward-eligible signers failed: %v", err)
	}
	if testkit.AddressIndex(eligibleSigners.Signers, rotation.NewSigner) < 0 {
		t.Fatalf("reward-eligible signer set missing rotated signer %s: %v", rotation.NewSigner.Hex(), eligibleSigners.Signers)
	}
}
