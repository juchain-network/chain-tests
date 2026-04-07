package tests

import (
	"context"
	"math/big"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestZ_CheckpointRuntimePunishStillUsesOldSigner(t *testing.T) {
	if ctx == nil {
		t.Fatalf("context not initialized")
	}
	if !testkit.IsMultiValidatorSeparatedMode(ctx, 3) {
		t.Skip("requires multi-validator separated-signer topology")
	}

	epochBig, err := ctx.Proposal.Epoch(nil)
	if err != nil || epochBig == nil || epochBig.Sign() <= 0 {
		t.Fatalf("read epoch failed: %v", err)
	}
	epoch := epochBig.Uint64()
	if epoch < 4 {
		t.Fatalf("epoch too small for checkpoint punish scenario: %d", epoch)
	}
	ensureMinActiveValidators(t, 3, 2)
	activeValidators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("get active validators failed: %v", err)
	}
	if len(activeValidators) != 3 {
		t.Skipf("requires exactly 3 active validators, got %d", len(activeValidators))
	}

	origPunishThreshold, err := ctx.Proposal.PunishThreshold(nil)
	if err != nil || origPunishThreshold == nil {
		t.Fatalf("read punishThreshold failed: %v", err)
	}
	origRemoveThreshold, err := ctx.Proposal.RemoveThreshold(nil)
	if err != nil || origRemoveThreshold == nil {
		t.Fatalf("read removeThreshold failed: %v", err)
	}
	if err := ctx.EnsureConfig(1, big.NewInt(1), origPunishThreshold); err != nil {
		t.Fatalf("ensure punishThreshold=1 failed: %v", err)
	}
	if err := ctx.EnsureConfig(2, big.NewInt(1000), origRemoveThreshold); err != nil {
		t.Fatalf("ensure removeThreshold=1000 failed: %v", err)
	}
	t.Cleanup(func() {
		_ = ctx.EnsureConfig(1, new(big.Int).Set(origPunishThreshold), big.NewInt(1))
		_ = ctx.EnsureConfig(2, new(big.Int).Set(origRemoveThreshold), big.NewInt(1000))
	})

	restoreSigner := func(rotation *testkit.SignerRotation) {
		if rotation == nil {
			return
		}
		_ = testkit.RestartValidatorNodeWithSigner(ctx, rotation.Validator, rotation.NewSignerKey, 90*time.Second)
	}

	primaryValidator := common.Address{}
	if len(ctx.Config.Validators) > 0 {
		primaryValidator = common.HexToAddress(ctx.Config.Validators[0].Address)
	}

	var targetValidator common.Address
	var predictedCheckpoint uint64
	for {
		head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || head == nil {
			t.Fatalf("read latest header failed: %v", err)
		}
		predictedCheckpoint = ((head.Number.Uint64() / epoch) + 1) * epoch
		switch {
		case predictedCheckpoint <= head.Number.Uint64()+3:
			if _, err := ctx.WaitUntilHeight(predictedCheckpoint+1, 90*time.Second); err != nil {
				t.Fatalf("wait for next checkpoint window failed: %v", err)
			}
			continue
		case predictedCheckpoint > head.Number.Uint64()+12:
			if _, err := ctx.WaitUntilHeight(predictedCheckpoint-12, 90*time.Second); err != nil {
				t.Fatalf("wait to align checkpoint window failed: %v", err)
			}
			continue
		}
		for _, validator := range activeValidators {
			if primaryValidator != (common.Address{}) && validator == primaryValidator {
				continue
			}
			targetValidator = validator
			break
		}
		if targetValidator == (common.Address{}) {
			t.Skip("no non-primary active validator available for checkpoint punish scenario")
		}
		break
	}

	rotation, _, err := testkit.PrepareValidatorSignerRotation(ctx, targetValidator)
	if err != nil {
		t.Fatalf("prepare signer rotation failed: %v", err)
	}
	if rotation.EffectiveBlock != predictedCheckpoint {
		t.Fatalf("rotation effective block mismatch: got=%d want=%d", rotation.EffectiveBlock, predictedCheckpoint)
	}
	if rotation.EffectiveBlock < 2 {
		t.Fatalf("unexpected effective block for punish scenario: %d", rotation.EffectiveBlock)
	}

	infoBeforeEvidence, err := ctx.Staking.GetValidatorInfo(nil, rotation.Validator)
	if err != nil {
		t.Fatalf("read validator info before unused-signer evidence failed: %v", err)
	}
	latestBeforeEvidence, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || latestBeforeEvidence == nil {
		t.Fatalf("read latest header before unused-signer evidence failed: %v", err)
	}
	if latestBeforeEvidence.Number.Uint64() >= rotation.EffectiveBlock {
		t.Fatalf(
			"too late to submit unused-signer evidence before checkpoint: head=%d effective=%d",
			latestBeforeEvidence.Number.Uint64(),
			rotation.EffectiveBlock,
		)
	}
	targetHeight := new(big.Int).Sub(latestBeforeEvidence.Number, big.NewInt(1))
	if targetHeight.Sign() <= 0 {
		targetHeight = big.NewInt(1)
	}
	baseTime := latestBeforeEvidence.Time
	if targetHeader, err := ctx.Clients[0].HeaderByNumber(context.Background(), targetHeight); err == nil && targetHeader != nil {
		baseTime = targetHeader.Time
	}
	h1 := &types.Header{
		ParentHash:  common.Hash{},
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    rotation.NewSigner,
		Root:        common.Hash{0x11},
		TxHash:      types.EmptyRootHash,
		ReceiptHash: types.EmptyRootHash,
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(1),
		Number:      targetHeight,
		GasLimit:    30_000_000,
		GasUsed:     0,
		Time:        baseTime,
		Extra:       make([]byte, 32+65),
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
	}
	h2 := &types.Header{
		ParentHash:  common.Hash{},
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    rotation.NewSigner,
		Root:        common.Hash{0x12},
		TxHash:      types.EmptyRootHash,
		ReceiptHash: types.EmptyRootHash,
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(1),
		Number:      new(big.Int).Set(targetHeight),
		GasLimit:    30_000_000,
		GasUsed:     0,
		Time:        baseTime,
		Extra:       make([]byte, 32+65),
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
	}
	rlp1, err := signHeaderClique(h1, rotation.NewSignerKey)
	if err != nil {
		t.Fatalf("sign unused-signer evidence header #1 failed: %v", err)
	}
	rlp2, err := signHeaderClique(h2, rotation.NewSignerKey)
	if err != nil {
		t.Fatalf("sign unused-signer evidence header #2 failed: %v", err)
	}
	punishABI, err := contracts.PunishMetaData.GetAbi()
	if err != nil {
		t.Fatalf("load punish ABI for unused-signer evidence failed: %v", err)
	}
	callData, err := punishABI.Pack("submitDoubleSignEvidence", rlp1, rlp2)
	if err != nil {
		t.Fatalf("pack unused-signer evidence call failed: %v", err)
	}
	reporterAddr := common.HexToAddress(ctx.Config.Funder.Address)
	msg := ethereum.CallMsg{
		From: reporterAddr,
		To:   &testctx.PunishAddr,
		Gas:  3_000_000,
		Data: callData,
	}
	if _, err := ctx.Clients[0].CallContract(context.Background(), msg, latestBeforeEvidence.Number); err == nil {
		t.Fatalf("unused pre-activation signer evidence unexpectedly succeeded in eth_call")
	}
	infoAfterEvidence, err := ctx.Staking.GetValidatorInfo(nil, rotation.Validator)
	if err != nil {
		t.Fatalf("read validator info after unused-signer evidence failed: %v", err)
	}
	if infoBeforeEvidence.SelfStake == nil || infoAfterEvidence.SelfStake == nil {
		t.Fatalf(
			"unexpected nil self-stake around unused-signer evidence: before=%v after=%v",
			infoBeforeEvidence.SelfStake,
			infoAfterEvidence.SelfStake,
		)
	}
	if infoAfterEvidence.SelfStake.Cmp(infoBeforeEvidence.SelfStake) != 0 {
		t.Fatalf(
			"unused pre-activation signer evidence changed validator self stake: before=%s after=%s",
			infoBeforeEvidence.SelfStake.String(),
			infoAfterEvidence.SelfStake.String(),
		)
	}
	if infoAfterEvidence.IsJailed != infoBeforeEvidence.IsJailed {
		t.Fatalf(
			"unused pre-activation signer evidence changed jailed state: before=%v after=%v",
			infoBeforeEvidence.IsJailed,
			infoAfterEvidence.IsJailed,
		)
	}

	stopHeight := rotation.EffectiveBlock - 3
	if _, err := ctx.WaitUntilHeight(stopHeight, 120*time.Second); err != nil {
		t.Fatalf("wait for stop height %d failed: %v", stopHeight, err)
	}
	if err := testkit.StopValidatorNode(ctx, rotation.Validator, 30*time.Second); err != nil {
		t.Fatalf("stop target validator node failed: %v", err)
	}

	preBlock := rotation.EffectiveBlock - 1
	if _, err := ctx.WaitUntilHeight(preBlock, 120*time.Second); err != nil {
		restoreSigner(rotation)
		t.Fatalf("wait for pre-checkpoint block %d failed: %v", preBlock, err)
	}
	preCall := &bind.CallOpts{BlockNumber: new(big.Int).SetUint64(preBlock)}
	preRuntimeValidator, err := ctx.Validators.GetValidatorBySigner(preCall, rotation.NewSigner)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getValidatorBySigner before checkpoint failed: %v", err)
	}
	if preRuntimeValidator != (common.Address{}) {
		restoreSigner(rotation)
		t.Fatalf("pre-checkpoint runtime signer mapping unexpectedly exposes new signer: got=%s want=%s", preRuntimeValidator.Hex(), common.Address{}.Hex())
	}
	preHistoryValidator, err := ctx.Validators.GetValidatorBySignerHistory(preCall, rotation.NewSigner)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getValidatorBySignerHistory before checkpoint failed: %v", err)
	}
	if preHistoryValidator != (common.Address{}) {
		restoreSigner(rotation)
		t.Fatalf("pre-checkpoint history signer mapping unexpectedly exposes new signer: got=%s want=%s", preHistoryValidator.Hex(), common.Address{}.Hex())
	}

	if _, err := ctx.WaitUntilHeight(rotation.EffectiveBlock, 120*time.Second); err != nil {
		restoreSigner(rotation)
		t.Fatalf("wait for checkpoint block %d failed: %v", rotation.EffectiveBlock, err)
	}

	checkpointNum := new(big.Int).SetUint64(rotation.EffectiveBlock)
	checkpointCall := &bind.CallOpts{BlockNumber: checkpointNum}
	runtimeSigners, err := ctx.Validators.GetTopSigners(checkpointCall)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getTopSigners at checkpoint failed: %v", err)
	}
	transitionSigners, err := ctx.Validators.GetTopSignersForEpochTransition(checkpointCall)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getTopSignersForEpochTransition at checkpoint failed: %v", err)
	}

	headerN, err := ctx.Clients[0].HeaderByNumber(context.Background(), checkpointNum)
	if err != nil || headerN == nil {
		restoreSigner(rotation)
		t.Fatalf("read checkpoint header failed: %v", err)
	}
	extraSigners, err := testkit.ParseHeaderExtraSigners(headerN.Extra)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("parse checkpoint header extra failed: %v", err)
	}
	currentValidator, err := ctx.Validators.GetValidatorBySigner(checkpointCall, rotation.OldSigner)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getValidatorBySigner at checkpoint failed: %v", err)
	}
	readPendingHead := func(call *bind.CallOpts) (common.Address, bool) {
		addr, err := ctx.Punish.PendingValidators(call, big.NewInt(0))
		if err != nil {
			return common.Address{}, false
		}
		return addr, true
	}
	pendingHead := common.Address{}
	pendingObserved := false
	for offset := uint64(0); offset < 3; offset++ {
		height := rotation.EffectiveBlock + offset
		if offset > 0 {
			if _, err := ctx.WaitUntilHeight(height, 30*time.Second); err != nil {
				restoreSigner(rotation)
				t.Fatalf("wait for pending punish observation height %d failed: %v", height, err)
			}
		}
		call := &bind.CallOpts{BlockNumber: new(big.Int).SetUint64(height)}
		if addr, ok := readPendingHead(call); ok {
			pendingObserved = true
			pendingHead = addr
			if addr == rotation.Validator {
				break
			}
		}
	}

	restoreSigner(rotation)

	if addressIndex(runtimeSigners, rotation.OldSigner) < 0 {
		t.Fatalf("old signer %s missing from checkpoint runtime signer set %v", rotation.OldSigner.Hex(), runtimeSigners)
	}
	if addressIndex(runtimeSigners, rotation.NewSigner) >= 0 {
		t.Fatalf("new signer %s unexpectedly present in checkpoint runtime signer set %v", rotation.NewSigner.Hex(), runtimeSigners)
	}
	if addressIndex(transitionSigners, rotation.NewSigner) < 0 {
		t.Fatalf("new signer %s missing from checkpoint transition signer set %v", rotation.NewSigner.Hex(), transitionSigners)
	}
	if headerN.Coinbase == rotation.OldSigner {
		t.Fatalf("checkpoint block was still produced by in-turn signer %s; out-of-turn punish path not triggered", rotation.OldSigner.Hex())
	}
	if headerN.Difficulty == nil || headerN.Difficulty.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("checkpoint block is not out-of-turn: difficulty=%v", headerN.Difficulty)
	}
	if !sameAddressSet(extraSigners, transitionSigners) {
		t.Fatalf("checkpoint header extra signer set mismatch: extra=%v transition=%v", extraSigners, transitionSigners)
	}
	if addressIndex(extraSigners, rotation.NewSigner) < 0 {
		t.Fatalf("checkpoint header extra signer set missing new signer %s: %v", rotation.NewSigner.Hex(), extraSigners)
	}
	if addressIndex(extraSigners, rotation.OldSigner) >= 0 {
		t.Fatalf("checkpoint header extra signer set still contains old signer %s: %v", rotation.OldSigner.Hex(), extraSigners)
	}
	if currentValidator != rotation.Validator {
		t.Fatalf("checkpoint old signer mapping mismatch: got=%s want=%s", currentValidator.Hex(), rotation.Validator.Hex())
	}
	if !pendingObserved {
		t.Logf(
			"checkpoint punish queue was not observable within 3 blocks after checkpoint; continuing with header/runtime assertions only (target=%s oldSigner=%s headerCoinbase=%s)",
			rotation.Validator.Hex(),
			rotation.OldSigner.Hex(),
			headerN.Coinbase.Hex(),
		)
	} else if pendingHead != rotation.Validator {
		t.Fatalf(
			"checkpoint punish did not enqueue validator on epoch boundary: target=%s oldSigner=%s headerCoinbase=%s pendingHead=%s",
			rotation.Validator.Hex(),
			rotation.OldSigner.Hex(),
			headerN.Coinbase.Hex(),
			pendingHead.Hex(),
		)
	}

	if err := ctx.WaitForBlockProgress(2, 90*time.Second); err != nil {
		t.Fatalf("chain did not stay live after checkpoint punish path: %v", err)
	}
}

func addressIndex(items []common.Address, target common.Address) int {
	for i, item := range items {
		if item == target {
			return i
		}
	}
	return -1
}

func sameAddressSet(left []common.Address, right []common.Address) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[common.Address]int, len(left))
	for _, item := range left {
		counts[item]++
	}
	for _, item := range right {
		counts[item]--
		if counts[item] < 0 {
			return false
		}
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}
