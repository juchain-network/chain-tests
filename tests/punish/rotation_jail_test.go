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

func TestZ_SignerRotationMissingNewSignerTriggersPunishAndJail(t *testing.T) {
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
	removeThreshold, err := ctx.Proposal.RemoveThreshold(nil)
	if err != nil || removeThreshold == nil || removeThreshold.Sign() <= 0 {
		t.Fatalf("read removeThreshold failed: %v", err)
	}

	epoch := epochBig.Uint64()
	punishBlocks := punishThreshold.Uint64()*3 + 3
	jailBlocks := removeThreshold.Uint64()*3 + 3
	if epoch <= jailBlocks {
		t.Skipf("requires epoch > %d for long-window rotation punish assertion, got %d", jailBlocks, epoch)
	}

	targetValidator := activeValidators[0]
	baselineIncoming := waitForIncomingProfits(t, targetValidator, 18)
	_, _, _, totalJailedBefore, _, err := ctx.Validators.GetValidatorInfo(nil, targetValidator)
	if err != nil {
		t.Fatalf("read validator info before rotation failed: %v", err)
	}

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
	if len(otherSigners) != 2 {
		t.Fatalf("expected 2 non-target signers, got %d", len(otherSigners))
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
			waitUntilHeightOrStall(t, "checkpoint-window-skip", nextCheckpoint+1, 15*time.Second, longWindowTimeout(4))
			continue
		case nextCheckpoint > head.Number.Uint64()+12:
			waitUntilHeightOrStall(t, "checkpoint-window-align", nextCheckpoint-12, 15*time.Second, longWindowTimeout(nextCheckpoint-head.Number.Uint64()))
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
	if rotation.EffectiveBlock < 2 {
		t.Fatalf("unexpected effective block: %d", rotation.EffectiveBlock)
	}
	if expectedCheckpoint != 0 && rotation.EffectiveBlock != expectedCheckpoint {
		t.Fatalf("rotation effective block mismatch: got=%d want=%d", rotation.EffectiveBlock, expectedCheckpoint)
	}

	checkpointHeight := rotation.EffectiveBlock
	thresholdHeight := checkpointHeight + punishBlocks
	jailDeadline := checkpointHeight + jailBlocks
	if thresholdHeight%epoch == 0 {
		thresholdHeight++
	}
	if jailDeadline%epoch == 0 {
		jailDeadline++
	}
	if jailDeadline >= checkpointHeight+epoch {
		t.Fatalf(
			"jail observation crossed next epoch boundary: checkpoint=%d deadline=%d epoch=%d",
			checkpointHeight,
			jailDeadline,
			epoch,
		)
	}

	waitUntilHeightOrStall(t, "checkpoint-height", checkpointHeight, 15*time.Second, longWindowTimeout(checkpointHeight))

	checkpointCall := callAt(checkpointHeight)
	runtimeSigners, err := ctx.Validators.GetTopSigners(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSigners at checkpoint failed: %v", err)
	}
	transitionSigners, err := ctx.Validators.GetTopSignersForEpochTransition(checkpointCall)
	if err != nil {
		t.Fatalf("getTopSignersForEpochTransition at checkpoint failed: %v", err)
	}
	if len(runtimeSigners) != 3 {
		t.Fatalf("checkpoint runtime signer count mismatch: got=%d want=3", len(runtimeSigners))
	}
	if len(transitionSigners) != 3 {
		t.Fatalf("checkpoint transition signer count mismatch: got=%d want=3", len(transitionSigners))
	}
	if addressIndex(runtimeSigners, rotation.OldSigner) < 0 {
		t.Fatalf("checkpoint runtime signers missing old signer %s: %v", rotation.OldSigner.Hex(), runtimeSigners)
	}
	if addressIndex(runtimeSigners, rotation.NewSigner) >= 0 {
		t.Fatalf("checkpoint runtime signers unexpectedly expose new signer %s: %v", rotation.NewSigner.Hex(), runtimeSigners)
	}
	if addressIndex(transitionSigners, rotation.NewSigner) < 0 {
		t.Fatalf("checkpoint transition signers missing new signer %s: %v", rotation.NewSigner.Hex(), transitionSigners)
	}
	if addressIndex(transitionSigners, rotation.OldSigner) >= 0 {
		t.Fatalf("checkpoint transition signers unexpectedly retain old signer %s: %v", rotation.OldSigner.Hex(), transitionSigners)
	}

	headerN, err := ctx.Clients[0].HeaderByNumber(context.Background(), new(big.Int).SetUint64(checkpointHeight))
	if err != nil || headerN == nil {
		t.Fatalf("read checkpoint header failed: %v", err)
	}
	extraSigners, err := testkit.ParseHeaderExtraSigners(headerN.Extra)
	if err != nil {
		t.Fatalf("parse checkpoint header extra failed: %v", err)
	}
	if len(extraSigners) != 3 {
		t.Fatalf("checkpoint header extra signer count mismatch: got=%d want=3", len(extraSigners))
	}
	if addressIndex(extraSigners, rotation.NewSigner) < 0 {
		t.Fatalf("checkpoint header extra missing new signer %s: %v", rotation.NewSigner.Hex(), extraSigners)
	}
	if addressIndex(extraSigners, rotation.OldSigner) >= 0 {
		t.Fatalf("checkpoint header extra unexpectedly retains old signer %s: %v", rotation.OldSigner.Hex(), extraSigners)
	}

	waitUntilHeightOrStall(t, "punish-threshold", thresholdHeight, 15*time.Second, longWindowTimeout(thresholdHeight-checkpointHeight))
	thresholdCall := callAt(thresholdHeight)
	thresholdRecord, err := ctx.Punish.GetPunishRecord(thresholdCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read punish record at threshold height failed: %v", err)
	}
	if thresholdRecord.Cmp(punishThreshold) < 0 {
		t.Fatalf(
			"punish record below punishThreshold at height %d: got=%s want>=%s",
			thresholdHeight,
			thresholdRecord.String(),
			punishThreshold.String(),
		)
	}
	if thresholdRecord.Cmp(removeThreshold) >= 0 {
		t.Fatalf(
			"punish record already reached removeThreshold too early at height %d: got=%s remove=%s",
			thresholdHeight,
			thresholdRecord.String(),
			removeThreshold.String(),
		)
	}

	thresholdInfo, err := ctx.Staking.GetValidatorInfo(thresholdCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read staking info at punish threshold failed: %v", err)
	}
	if thresholdInfo.IsJailed {
		t.Fatalf("validator unexpectedly jailed before removeThreshold at height %d", thresholdHeight)
	}
	_, _, thresholdIncoming, thresholdTotalJailed, _, err := ctx.Validators.GetValidatorInfo(thresholdCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read validator fee info at punish threshold failed: %v", err)
	}
	if thresholdIncoming == nil || thresholdIncoming.Sign() != 0 {
		t.Fatalf(
			"validator incoming profits were not cleared at punish threshold: height=%d incoming=%v",
			thresholdHeight,
			thresholdIncoming,
		)
	}
	if baselineIncoming.Sign() > 0 && thresholdTotalJailed.Cmp(totalJailedBefore) <= 0 {
		t.Fatalf(
			"validator totalJailedHb did not increase after reward removal: before=%s after=%s baselineIncoming=%s",
			totalJailedBefore.String(),
			thresholdTotalJailed.String(),
			baselineIncoming.String(),
		)
	}
	thresholdRuntime, err := ctx.Validators.GetTopSigners(thresholdCall)
	if err != nil {
		t.Fatalf("getTopSigners after checkpoint failed: %v", err)
	}
	if addressIndex(thresholdRuntime, rotation.NewSigner) < 0 {
		t.Fatalf("post-checkpoint runtime signers missing rotated signer %s: %v", rotation.NewSigner.Hex(), thresholdRuntime)
	}
	if addressIndex(thresholdRuntime, rotation.OldSigner) >= 0 {
		t.Fatalf("post-checkpoint runtime signers still contain old signer %s: %v", rotation.OldSigner.Hex(), thresholdRuntime)
	}
	thresholdRuntimeValidator, err := ctx.Validators.GetValidatorBySigner(thresholdCall, rotation.NewSigner)
	if err != nil {
		t.Fatalf("getValidatorBySigner for rotated signer failed: %v", err)
	}
	if thresholdRuntimeValidator != rotation.Validator {
		t.Fatalf(
			"rotated signer runtime mapping mismatch at height %d: got=%s want=%s",
			thresholdHeight,
			thresholdRuntimeValidator.Hex(),
			rotation.Validator.Hex(),
		)
	}

	jailHeight := waitForJailTransitionOrStall(t, rotation.Validator, thresholdHeight+1, jailDeadline, 15*time.Second)
	jailCall := callAt(jailHeight)
	jailRecord, err := ctx.Punish.GetPunishRecord(jailCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read punish record at jail height failed: %v", err)
	}
	if jailRecord.Sign() != 0 {
		t.Fatalf("punish record should reset after jail: height=%d record=%s", jailHeight, jailRecord.String())
	}
	jailInfo, err := ctx.Staking.GetValidatorInfo(jailCall, rotation.Validator)
	if err != nil {
		t.Fatalf("read staking info at jail height failed: %v", err)
	}
	if !jailInfo.IsJailed {
		t.Fatalf("validator was not jailed by height %d", jailHeight)
	}
	activeAtJail, err := ctx.Validators.IsValidatorActive(jailCall, rotation.Validator)
	if err != nil {
		t.Fatalf("check validator active state at jail height failed: %v", err)
	}
	if activeAtJail {
		t.Fatalf("validator remained active after jail at height %d", jailHeight)
	}
	eligibleValidators, err := ctx.Validators.GetRewardEligibleValidatorsWithStakes(jailCall)
	if err != nil {
		t.Fatalf("read reward-eligible validators at jail height failed: %v", err)
	}
	if addressIndex(eligibleValidators.Validators, rotation.Validator) >= 0 {
		t.Fatalf("jailed validator still present in reward-eligible validator set at height %d", jailHeight)
	}
	eligibleSigners, err := ctx.Validators.GetRewardEligibleSignersWithStakes(jailCall)
	if err != nil {
		t.Fatalf("read reward-eligible signers at jail height failed: %v", err)
	}
	if addressIndex(eligibleSigners.Signers, rotation.NewSigner) >= 0 {
		t.Fatalf("jailed validator rotated signer still present in reward-eligible signer set at height %d", jailHeight)
	}
	if addressIndex(eligibleSigners.Signers, rotation.OldSigner) >= 0 {
		t.Fatalf("jailed validator old signer still present in reward-eligible signer set at height %d", jailHeight)
	}

	observedDuringRotation := collectCoinbaseSet(t, checkpointHeight+1, jailHeight)
	for _, signer := range otherSigners {
		if !observedDuringRotation[signer] {
			t.Fatalf(
				"non-target validator signer %s never produced blocks after rotation; observed=%v",
				signer.Hex(),
				coinbaseSetKeys(observedDuringRotation),
			)
		}
	}
	if observedDuringRotation[rotation.NewSigner] {
		t.Fatalf("rotated signer %s unexpectedly produced blocks without restart", rotation.NewSigner.Hex())
	}
	if observedDuringRotation[rotation.OldSigner] {
		t.Fatalf("old signer %s unexpectedly kept producing blocks after checkpoint", rotation.OldSigner.Hex())
	}

	postJailHeight := jailHeight + 3
	waitUntilHeightOrStall(t, "post-jail-liveness", postJailHeight, 15*time.Second, longWindowTimeout(3))
	observedPostJail := collectCoinbaseSet(t, jailHeight+1, postJailHeight)
	for _, signer := range otherSigners {
		if !observedPostJail[signer] {
			t.Fatalf(
				"chain did not stay live on remaining validators after jail; missing signer=%s observed=%v",
				signer.Hex(),
				coinbaseSetKeys(observedPostJail),
			)
		}
	}
}

func callAt(height uint64) *bind.CallOpts {
	return &bind.CallOpts{BlockNumber: new(big.Int).SetUint64(height)}
}

func longWindowTimeout(blocks uint64) time.Duration {
	if blocks < 1 {
		blocks = 1
	}
	return time.Duration(blocks)*6*time.Second + 2*time.Minute
}

func collectCoinbaseSet(t *testing.T, startHeight, endHeight uint64) map[common.Address]bool {
	t.Helper()
	if endHeight < startHeight {
		return map[common.Address]bool{}
	}
	observed := make(map[common.Address]bool)
	for height := startHeight; height <= endHeight; height++ {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), new(big.Int).SetUint64(height))
		if err != nil || header == nil {
			t.Fatalf("read header at height %d failed: %v", height, err)
		}
		observed[header.Coinbase] = true
	}
	return observed
}

func coinbaseSetKeys(items map[common.Address]bool) []common.Address {
	keys := make([]common.Address, 0, len(items))
	for addr := range items {
		keys = append(keys, addr)
	}
	return keys
}

func waitUntilHeightOrStall(t *testing.T, label string, target uint64, stallTimeout, overallTimeout time.Duration) uint64 {
	t.Helper()
	if ctx == nil || len(ctx.Clients) == 0 {
		t.Fatalf("context not initialized")
	}
	if stallTimeout <= 0 {
		stallTimeout = 15 * time.Second
	}
	if overallTimeout <= 0 {
		overallTimeout = 2 * time.Minute
	}

	start := time.Now()
	lastProgress := time.Now()
	lastHeight, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("%s: read initial block height failed: %v", label, err)
	}
	if lastHeight >= target {
		return lastHeight
	}

	for time.Since(start) < overallTimeout {
		current, err := ctx.Clients[0].BlockNumber(context.Background())
		if err == nil {
			if current >= target {
				return current
			}
			if current > lastHeight {
				lastHeight = current
				lastProgress = time.Now()
			}
		}

		if time.Since(lastProgress) >= stallTimeout {
			t.Fatalf(
				"%s: chain stalled before target height: target=%d current=%d stalled_for=%s recent_coinbases=%v",
				label,
				target,
				lastHeight,
				time.Since(lastProgress).Round(time.Second),
				recentCoinbases(12),
			)
		}

		time.Sleep(blockPollInterval())
	}

	t.Fatalf(
		"%s: timeout waiting for target height: target=%d current=%d waited=%s recent_coinbases=%v",
		label,
		target,
		lastHeight,
		overallTimeout.Round(time.Second),
		recentCoinbases(12),
	)
	return 0
}

func waitForJailTransitionOrStall(t *testing.T, validator common.Address, startHeight, deadline uint64, stallTimeout time.Duration) uint64 {
	t.Helper()
	if ctx == nil || len(ctx.Clients) == 0 {
		t.Fatalf("context not initialized")
	}
	if startHeight == 0 {
		startHeight = 1
	}
	if deadline < startHeight {
		deadline = startHeight
	}
	if stallTimeout <= 0 {
		stallTimeout = 15 * time.Second
	}

	start := time.Now()
	overallTimeout := longWindowTimeout(deadline - startHeight + 1)
	lastProgress := time.Now()
	lastHeight, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("read initial block height before jail wait failed: %v", err)
	}

	for time.Since(start) < overallTimeout {
		current, err := ctx.Clients[0].BlockNumber(context.Background())
		if err == nil {
			if current > lastHeight {
				lastHeight = current
				lastProgress = time.Now()
			}
			if current >= startHeight {
				call := callAt(current)
				record, recordErr := ctx.Punish.GetPunishRecord(call, validator)
				info, infoErr := ctx.Staking.GetValidatorInfo(call, validator)
				if recordErr == nil && infoErr == nil && info.IsJailed && record.Sign() == 0 {
					return current
				}
				if current >= deadline {
					t.Fatalf(
						"validator did not reach jailed+reset state within expected window: validator=%s start=%d deadline=%d current=%d punish_record=%v jailed=%v recent_coinbases=%v",
						validator.Hex(),
						startHeight,
						deadline,
						current,
						record,
						infoErr == nil && info.IsJailed,
						recentCoinbases(12),
					)
				}
			}
		}

		if time.Since(lastProgress) >= stallTimeout {
			t.Fatalf(
				"chain stalled while waiting for validator jail transition: validator=%s start=%d deadline=%d current=%d stalled_for=%s recent_coinbases=%v",
				validator.Hex(),
				startHeight,
				deadline,
				lastHeight,
				time.Since(lastProgress).Round(time.Second),
				recentCoinbases(12),
			)
		}

		time.Sleep(blockPollInterval())
	}

	t.Fatalf(
		"timeout waiting for validator jail transition: validator=%s start=%d deadline=%d current=%d waited=%s recent_coinbases=%v",
		validator.Hex(),
		startHeight,
		deadline,
		lastHeight,
		overallTimeout.Round(time.Second),
		recentCoinbases(12),
	)
	return 0
}

func recentCoinbases(limit int) []common.Address {
	if ctx == nil || len(ctx.Clients) == 0 || limit <= 0 {
		return nil
	}
	latest, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		return nil
	}
	start := uint64(1)
	if latest >= uint64(limit) {
		start = latest - uint64(limit) + 1
	}
	items := make([]common.Address, 0, latest-start+1)
	for height := start; height <= latest; height++ {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), new(big.Int).SetUint64(height))
		if err != nil || header == nil {
			continue
		}
		items = append(items, header.Coinbase)
	}
	return items
}
