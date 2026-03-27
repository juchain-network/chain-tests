package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestZ_CheckpointTransitionSignerSplit(t *testing.T) {
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

	preCall := &bind.CallOpts{BlockNumber: newBig(preBlock)}
	preRuntime, err := ctx.Validators.GetTopSigners(preCall)
	if err != nil {
		t.Fatalf("getTopSigners before checkpoint failed: %v", err)
	}
	preTransition, err := ctx.Validators.GetTopSignersForEpochTransition(preCall)
	if err != nil {
		t.Fatalf("getTopSignersForEpochTransition before checkpoint failed: %v", err)
	}
	if len(preRuntime) != 1 || preRuntime[0] != rotation.OldSigner {
		t.Fatalf("pre-checkpoint runtime signer mismatch: got=%v want=%s", preRuntime, rotation.OldSigner.Hex())
	}
	if len(preTransition) != 1 || preTransition[0] != rotation.OldSigner {
		t.Fatalf("pre-checkpoint transition signer mismatch: got=%v want=%s", preTransition, rotation.OldSigner.Hex())
	}
	preHistoryValidator, err := ctx.Validators.GetValidatorBySignerHistory(preCall, rotation.NewSigner)
	if err != nil {
		t.Fatalf("getValidatorBySignerHistory before checkpoint failed: %v", err)
	}
	if preHistoryValidator != (common.Address{}) {
		t.Fatalf("pre-checkpoint history unexpectedly exposes new signer: got=%s want=%s", preHistoryValidator.Hex(), common.Address{}.Hex())
	}

	if _, err := ctx.WaitUntilHeight(rotation.EffectiveBlock, 45*time.Second); err != nil {
		t.Fatalf("wait for checkpoint block %d failed: %v", rotation.EffectiveBlock, err)
	}
	checkpointNum := newBig(rotation.EffectiveBlock)
	checkpointCall := &bind.CallOpts{BlockNumber: checkpointNum}
	runtimeAtCheckpoint, err := ctx.Validators.GetTopSigners(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSigners at checkpoint failed: %v", err)
	}
	transitionAtCheckpoint, err := ctx.Validators.GetTopSignersForEpochTransition(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSignersForEpochTransition at checkpoint failed: %v", err)
	}
	if len(runtimeAtCheckpoint) != 1 || runtimeAtCheckpoint[0] != rotation.OldSigner {
		t.Fatalf("checkpoint runtime signer mismatch: got=%v want=%s", runtimeAtCheckpoint, rotation.OldSigner.Hex())
	}
	if len(transitionAtCheckpoint) != 1 || transitionAtCheckpoint[0] != rotation.NewSigner {
		t.Fatalf("checkpoint transition signer mismatch: got=%v want=%s", transitionAtCheckpoint, rotation.NewSigner.Hex())
	}

	headerN, err := ctx.Clients[0].HeaderByNumber(context.Background(), checkpointNum)
	if err != nil || headerN == nil {
		t.Fatalf("read checkpoint header failed: %v", err)
	}
	extraSigners, err := testkit.ParseHeaderExtraSigners(headerN.Extra)
	if err != nil {
		t.Fatalf("parse checkpoint header extra failed: %v", err)
	}
	if len(extraSigners) != 1 || extraSigners[0] != rotation.NewSigner {
		t.Fatalf("checkpoint header extra signer mismatch: got=%v want=%s", extraSigners, rotation.NewSigner.Hex())
	}

	historyValidator, err := ctx.Validators.GetValidatorBySignerHistory(checkpointCall, rotation.NewSigner)
	if err != nil {
		t.Fatalf("getValidatorBySignerHistory at checkpoint failed: %v", err)
	}
	if historyValidator != (common.Address{}) {
		t.Fatalf("checkpoint history signer mapping exposed transition signer too early: got=%s want=%s", historyValidator.Hex(), common.Address{}.Hex())
	}

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
		t.Fatalf("read post-transition head failed: %v", err)
	}
	if head.Number.Uint64() <= rotation.EffectiveBlock {
		t.Fatalf("expected head to advance past checkpoint: head=%d checkpoint=%d", head.Number.Uint64(), rotation.EffectiveBlock)
	}
	postRuntime, err := ctx.Validators.GetTopSigners(&bind.CallOpts{BlockNumber: head.Number})
	if err != nil {
		t.Fatalf("getTopSigners after checkpoint failed: %v", err)
	}
	postTransition, err := ctx.Validators.GetTopSignersForEpochTransition(&bind.CallOpts{BlockNumber: head.Number})
	if err != nil {
		t.Fatalf("getTopSignersForEpochTransition after checkpoint failed: %v", err)
	}
	if len(postRuntime) != 1 || postRuntime[0] != rotation.NewSigner {
		t.Fatalf("post-checkpoint runtime signer mismatch: got=%v want=%s", postRuntime, rotation.NewSigner.Hex())
	}
	if len(postTransition) != 1 || postTransition[0] != rotation.NewSigner {
		t.Fatalf("post-checkpoint transition signer mismatch: got=%v want=%s", postTransition, rotation.NewSigner.Hex())
	}
	if head.Coinbase != rotation.NewSigner {
		t.Fatalf("post-checkpoint head coinbase mismatch: got=%s want=%s", head.Coinbase.Hex(), rotation.NewSigner.Hex())
	}
	postHistoryValidator, err := ctx.Validators.GetValidatorBySignerHistory(&bind.CallOpts{BlockNumber: head.Number}, rotation.NewSigner)
	if err != nil {
		t.Fatalf("getValidatorBySignerHistory after checkpoint failed: %v", err)
	}
	if postHistoryValidator != rotation.Validator {
		t.Fatalf("post-checkpoint history signer mapping mismatch: got=%s want=%s", postHistoryValidator.Hex(), rotation.Validator.Hex())
	}
}

func newBig(v uint64) *big.Int {
	return new(big.Int).SetUint64(v)
}
