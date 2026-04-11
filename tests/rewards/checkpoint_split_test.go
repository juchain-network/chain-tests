package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestZ_CheckpointRuntimeRewardsStillUseOldSigner(t *testing.T) {
	if ctx == nil {
		t.Fatalf("context not initialized")
	}
	if !testkit.IsSingleValidatorSeparatedMode(ctx) {
		t.Skip("requires single-validator separated-signer topology")
	}

	rotation, _, err := testkit.PrepareSingleValidatorSignerRotation(ctx)
	if err != nil {
		t.Fatalf("prepare signer rotation failed: %v", err)
	}
	if rotation.EffectiveBlock < 2 {
		t.Fatalf("unexpected effective block: %d", rotation.EffectiveBlock)
	}

	preBlock := rotation.EffectiveBlock - 1
	if _, err := ctx.WaitUntilHeight(preBlock, 90*time.Second); err != nil {
		t.Fatalf("wait for pre-checkpoint block %d failed: %v", preBlock, err)
	}

	preCall := &bind.CallOpts{BlockNumber: new(big.Int).SetUint64(preBlock)}
	preInfo, err := ctx.Staking.GetValidatorInfo(preCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read validator reward info before checkpoint failed: %v", err)
	}

	testkit.MarkScenarioStage("checkpoint-wait")
	if _, err := ctx.WaitUntilHeight(rotation.EffectiveBlock, 45*time.Second); err != nil {
		t.Fatalf("wait for checkpoint block %d failed: %v", rotation.EffectiveBlock, err)
	}

	checkpointNum := new(big.Int).SetUint64(rotation.EffectiveBlock)
	checkpointCall := &bind.CallOpts{BlockNumber: checkpointNum}
	runtimeSigners, err := ctx.Validators.GetTopSigners(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSigners at checkpoint failed: %v", err)
	}
	transitionSigners, err := ctx.Validators.GetTopSignersForEpochTransition(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSignersForEpochTransition at checkpoint failed: %v", err)
	}
	if len(runtimeSigners) != 1 || runtimeSigners[0] != rotation.OldSigner {
		t.Fatalf("checkpoint runtime signer mismatch: got=%v want=%s", runtimeSigners, rotation.OldSigner.Hex())
	}
	if len(transitionSigners) != 1 || transitionSigners[0] != rotation.NewSigner {
		t.Fatalf("checkpoint transition signer mismatch: got=%v want=%s", transitionSigners, rotation.NewSigner.Hex())
	}

	headerN, err := ctx.Clients[0].HeaderByNumber(context.Background(), checkpointNum)
	if err != nil || headerN == nil {
		t.Fatalf("read checkpoint header failed: %v", err)
	}
	if headerN.Coinbase != rotation.OldSigner {
		t.Fatalf("checkpoint header coinbase mismatch: got=%s want=%s", headerN.Coinbase.Hex(), rotation.OldSigner.Hex())
	}
	extraSigners, err := testkit.ParseHeaderExtraSigners(headerN.Extra)
	if err != nil {
		t.Fatalf("parse checkpoint header extra failed: %v", err)
	}
	if len(extraSigners) != 1 || extraSigners[0] != rotation.NewSigner {
		t.Fatalf("checkpoint header extra signer mismatch: got=%v want=%s", extraSigners, rotation.NewSigner.Hex())
	}

	checkpointInfo, err := ctx.Staking.GetValidatorInfo(checkpointCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read validator reward info at checkpoint failed: %v", err)
	}
	if checkpointInfo.AccumulatedRewards == nil || preInfo.AccumulatedRewards == nil || checkpointInfo.AccumulatedRewards.Cmp(preInfo.AccumulatedRewards) <= 0 {
		t.Fatalf(
			"checkpoint block did not increase validator rewards under old signer: before=%v after=%v",
			preInfo.AccumulatedRewards,
			checkpointInfo.AccumulatedRewards,
		)
	}

	testkit.MarkScenarioStage("post-checkpoint-observation")
	if err := testkit.ActivateRotatedSignerOnSingleNode(ctx, rotation, 90*time.Second); err != nil {
		t.Fatalf("restart node with rotated signer failed: %v", err)
	}
	if err := ctx.WaitForBlockProgress(1, 90*time.Second); err != nil {
		t.Fatalf("chain did not progress after restarting node with rotated signer: %v", err)
	}
	if err := testkit.WaitForCoinbaseSigner(ctx, rotation.NewSigner, 90*time.Second); err != nil {
		t.Fatalf("coinbase did not switch to new signer: %v", err)
	}

	head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || head == nil {
		t.Fatalf("read post-checkpoint head failed: %v", err)
	}
	if head.Number.Uint64() <= rotation.EffectiveBlock {
		t.Fatalf("expected head to progress beyond checkpoint: head=%d checkpoint=%d", head.Number.Uint64(), rotation.EffectiveBlock)
	}
	postCall := &bind.CallOpts{BlockNumber: head.Number}
	postRuntime, err := ctx.Validators.GetTopSigners(postCall)
	if err != nil {
		t.Fatalf("getTopSigners after checkpoint failed: %v", err)
	}
	if len(postRuntime) != 1 || postRuntime[0] != rotation.NewSigner {
		t.Fatalf("post-checkpoint runtime signer mismatch: got=%v want=%s", postRuntime, rotation.NewSigner.Hex())
	}
}
