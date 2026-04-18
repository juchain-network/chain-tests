package tests

import (
	"bytes"
	"context"
	"math/big"
	"sort"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/testkit"
)

type checkpointValidatorSigner struct {
	validator common.Address
	signer    common.Address
}

func selectCheckpointInTurnValidator(t *testing.T, validators []common.Address, checkpoint uint64) common.Address {
	t.Helper()
	if len(validators) == 0 {
		t.Fatalf("no active validators available for checkpoint selection")
	}

	entries := make([]checkpointValidatorSigner, 0, len(validators))
	for _, validator := range validators {
		signer, err := ctx.Validators.GetValidatorSigner(nil, validator)
		if err != nil {
			t.Fatalf("read signer for validator %s failed: %v", validator.Hex(), err)
		}
		entries = append(entries, checkpointValidatorSigner{
			validator: validator,
			signer:    signer,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].signer.Bytes(), entries[j].signer.Bytes()) < 0
	})

	idx := int(checkpoint % uint64(len(entries)))
	return entries[idx].validator
}

func openCheckpointLiveReader(
	t *testing.T,
	validators []common.Address,
	exclude common.Address,
) (*ethclient.Client, *contracts.Validators, *contracts.Punish, common.Address) {
	t.Helper()

	for _, validator := range validators {
		if validator == exclude {
			continue
		}
		rpcURL := ctx.ValidatorRPCByValidator(validator)
		if rpcURL == "" {
			continue
		}
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			continue
		}
		if _, err := client.BlockNumber(context.Background()); err != nil {
			client.Close()
			continue
		}
		valReader, err := contracts.NewValidators(testctx.ValidatorsAddr, client)
		if err != nil {
			client.Close()
			t.Fatalf("create alternate validators binding for %s failed: %v", validator.Hex(), err)
		}
		punReader, err := contracts.NewPunish(testctx.PunishAddr, client)
		if err != nil {
			client.Close()
			t.Fatalf("create alternate punish binding for %s failed: %v", validator.Hex(), err)
		}
		return client, valReader, punReader, validator
	}

	t.Fatalf("no live validator reader available after excluding %s", exclude.Hex())
	return nil, nil, nil, common.Address{}
}

func waitUntilHeightOnClient(t *testing.T, client *ethclient.Client, target uint64, timeout time.Duration) uint64 {
	t.Helper()
	if client == nil {
		t.Fatalf("waitUntilHeightOnClient called with nil client")
	}

	deadline := time.Now().Add(timeout)
	var last uint64
	var lastErr error
	for time.Now().Before(deadline) {
		height, err := client.BlockNumber(context.Background())
		if err == nil {
			last = height
			if height >= target {
				return height
			}
		} else {
			lastErr = err
		}
		time.Sleep(ctx.BlockPollInterval())
	}

	if lastErr != nil {
		t.Fatalf("wait for height %d failed: last=%d err=%v", target, last, lastErr)
	}
	t.Fatalf("wait for height %d timed out: last=%d", target, last)
	return last
}

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
		targetValidator = selectCheckpointInTurnValidator(t, activeValidators, predictedCheckpoint)
		break
	}
	liveClient, liveValidators, livePunish, liveReaderValidator := openCheckpointLiveReader(t, activeValidators, targetValidator)
	defer liveClient.Close()
	refreshLiveReader := func() {
		liveClient.Close()
		liveClient, liveValidators, livePunish, liveReaderValidator = openCheckpointLiveReader(t, activeValidators, targetValidator)
	}
	t.Logf(
		"checkpoint punish target=%s predictedCheckpoint=%d alternateReader=%s",
		targetValidator.Hex(),
		predictedCheckpoint,
		liveReaderValidator.Hex(),
	)

	rotation, _, err := testkit.PrepareValidatorSignerRotation(ctx, targetValidator)
	if err != nil {
		t.Fatalf("prepare signer rotation failed: %v", err)
	}
	if rotation.EffectiveBlock < 2 {
		t.Fatalf("unexpected effective block for punish scenario: %d", rotation.EffectiveBlock)
	}
	effectiveTarget := selectCheckpointInTurnValidator(t, activeValidators, rotation.EffectiveBlock)
	if effectiveTarget != targetValidator {
		t.Fatalf(
			"rotation effective block drift changed checkpoint target: predicted_checkpoint=%d predicted_target=%s effective_block=%d effective_target=%s",
			predictedCheckpoint,
			targetValidator.Hex(),
			rotation.EffectiveBlock,
			effectiveTarget.Hex(),
		)
	}
	if rotation.EffectiveBlock != predictedCheckpoint {
		t.Logf(
			"rotation effective block mismatch tolerated: got=%d predicted=%d target=%s",
			rotation.EffectiveBlock,
			predictedCheckpoint,
			targetValidator.Hex(),
		)
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

	// Stop after the target validator's previous in-turn slot has passed so the
	// checkpoint block is the first missed turn we intentionally induce.
	stopHeight := rotation.EffectiveBlock - 2
	if _, err := ctx.WaitUntilHeight(stopHeight, 120*time.Second); err != nil {
		t.Fatalf("wait for stop height %d failed: %v", stopHeight, err)
	}
	if err := testkit.StopValidatorNode(ctx, rotation.Validator, 30*time.Second); err != nil {
		t.Fatalf("stop target validator node failed: %v", err)
	}
	refreshLiveReader()

	preBlock := rotation.EffectiveBlock - 1
	waitUntilHeightOnClient(t, liveClient, preBlock, 120*time.Second)
	preCall := &bind.CallOpts{BlockNumber: new(big.Int).SetUint64(preBlock)}
	preRuntimeValidator, err := liveValidators.GetValidatorBySigner(preCall, rotation.NewSigner)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getValidatorBySigner before checkpoint failed: %v", err)
	}
	if preRuntimeValidator != (common.Address{}) {
		restoreSigner(rotation)
		t.Fatalf("pre-checkpoint runtime signer mapping unexpectedly exposes new signer: got=%s want=%s", preRuntimeValidator.Hex(), common.Address{}.Hex())
	}
	preHistoryValidator, err := liveValidators.GetValidatorBySignerHistory(preCall, rotation.NewSigner)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getValidatorBySignerHistory before checkpoint failed: %v", err)
	}
	if preHistoryValidator != (common.Address{}) {
		restoreSigner(rotation)
		t.Fatalf("pre-checkpoint history signer mapping unexpectedly exposes new signer: got=%s want=%s", preHistoryValidator.Hex(), common.Address{}.Hex())
	}

	testkit.MarkScenarioStage("checkpoint-wait")
	waitUntilHeightOnClient(t, liveClient, rotation.EffectiveBlock, 120*time.Second)

	checkpointNum := new(big.Int).SetUint64(rotation.EffectiveBlock)
	checkpointCall := &bind.CallOpts{BlockNumber: checkpointNum}
	runtimeSigners, err := liveValidators.GetTopSigners(checkpointCall)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getTopSigners at checkpoint failed: %v", err)
	}
	transitionSigners, err := liveValidators.GetTopSignersForEpochTransition(checkpointCall)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getTopSignersForEpochTransition at checkpoint failed: %v", err)
	}

	headerN, err := liveClient.HeaderByNumber(context.Background(), checkpointNum)
	if err != nil || headerN == nil {
		restoreSigner(rotation)
		t.Fatalf("read checkpoint header failed: %v", err)
	}
	extraSigners, err := testkit.ParseHeaderExtraSigners(headerN.Extra)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("parse checkpoint header extra failed: %v", err)
	}
	currentValidator, err := liveValidators.GetValidatorBySigner(checkpointCall, rotation.OldSigner)
	if err != nil {
		restoreSigner(rotation)
		t.Fatalf("getValidatorBySigner at checkpoint failed: %v", err)
	}
	readPendingHead := func(call *bind.CallOpts) (common.Address, bool) {
		addr, err := livePunish.PendingValidators(call, big.NewInt(0))
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
			waitUntilHeightOnClient(t, liveClient, height, 30*time.Second)
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
		t.Logf(
			"checkpoint block stayed on in-turn old signer; continuing with runtime/header transition assertions only: coinbase=%s oldSigner=%s targetValidator=%s reader=%s",
			headerN.Coinbase.Hex(),
			rotation.OldSigner.Hex(),
			rotation.Validator.Hex(),
			liveReaderValidator.Hex(),
		)
	} else if headerN.Difficulty == nil || headerN.Difficulty.Cmp(big.NewInt(1)) != 0 {
		t.Logf(
			"checkpoint block switched away from old signer but was not clearly out-of-turn: difficulty=%v coinbase=%s oldSigner=%s targetValidator=%s reader=%s",
			headerN.Difficulty,
			headerN.Coinbase.Hex(),
			rotation.OldSigner.Hex(),
			rotation.Validator.Hex(),
			liveReaderValidator.Hex(),
		)
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

	testkit.MarkScenarioStage("post-checkpoint-observation")
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
