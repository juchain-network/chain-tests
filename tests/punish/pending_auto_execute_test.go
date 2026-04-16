package tests

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"math/big"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/testkit"
)

func sortedRuntimeSigners(t *testing.T) []common.Address {
	t.Helper()
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	consensusSigners, err := ctx.TopRuntimeSigners()
	if err != nil {
		t.Fatalf("read runtime signers failed: %v", err)
	}
	sort.Slice(consensusSigners, func(i, j int) bool {
		return bytes.Compare(consensusSigners[i].Bytes(), consensusSigners[j].Bytes()) < 0
	})
	return consensusSigners
}

func resolveValidatorSignerIdentity(t *testing.T, validator common.Address) (common.Address, *ecdsa.PrivateKey) {
	t.Helper()
	signerAddr, signerKey := signerIdentityForValidator(validator, keyForAddress(validator))
	if signerAddr == (common.Address{}) {
		t.Fatalf("missing signer address for validator %s", validator.Hex())
	}
	if signerKey == nil {
		t.Fatalf("missing signer key for validator %s signer=%s", validator.Hex(), signerAddr.Hex())
	}
	return signerAddr, signerKey
}

func switchObserverIfStoppingPrimaryValidator(t *testing.T, targetVal common.Address) {
	t.Helper()
	primaryValidator := common.Address{}
	if ctx.Config != nil && len(ctx.Config.Validators) > 0 {
		primaryValidator = common.HexToAddress(ctx.Config.Validators[0].Address)
	}
	if targetVal != primaryValidator {
		return
	}

	observerURL := ""
	if ctx.Config != nil {
		targetRPC := strings.TrimSpace(ctx.ValidatorRPCByValidator(targetVal))
		for _, rpcURL := range ctx.Config.ValidatorRPCs {
			rpcURL = strings.TrimSpace(rpcURL)
			if rpcURL != "" && rpcURL != targetRPC {
				observerURL = rpcURL
				break
			}
		}
		if observerURL == "" {
			observerURL = strings.TrimSpace(ctx.Config.SyncRPC)
		}
	}
	if observerURL == "" {
		t.Fatalf("need alternate observer rpc when stopping primary validator %s", targetVal.Hex())
	}

	observerClient, err := ethclient.Dial(observerURL)
	if err != nil {
		t.Fatalf("dial observer rpc %s failed: %v", observerURL, err)
	}
	if _, err := observerClient.BlockNumber(context.Background()); err != nil {
		observerClient.Close()
		t.Fatalf("observer rpc %s not ready: %v", observerURL, err)
	}

	restoreClient := ctx.Clients[0]
	restoreValidators := ctx.Validators
	restorePunish := ctx.Punish
	restoreProposal := ctx.Proposal
	restoreStaking := ctx.Staking

	observerValidators, err := contracts.NewValidators(testctx.ValidatorsAddr, observerClient)
	if err != nil {
		t.Fatalf("rebind validators observer client failed: %v", err)
	}
	observerPunish, err := contracts.NewPunish(testctx.PunishAddr, observerClient)
	if err != nil {
		t.Fatalf("rebind punish observer client failed: %v", err)
	}
	observerProposal, err := contracts.NewProposal(testctx.ProposalAddr, observerClient)
	if err != nil {
		t.Fatalf("rebind proposal observer client failed: %v", err)
	}
	observerStaking, err := contracts.NewStaking(testctx.StakingAddr, observerClient)
	if err != nil {
		t.Fatalf("rebind staking observer client failed: %v", err)
	}

	ctx.Clients[0] = observerClient
	ctx.Validators = observerValidators
	ctx.Punish = observerPunish
	ctx.Proposal = observerProposal
	ctx.Staking = observerStaking
	t.Logf("switched observer client to %s while primary validator is stopped", observerURL)

	t.Cleanup(func() {
		observerClient.Close()
		ctx.Clients[0] = restoreClient
		ctx.Validators = restoreValidators
		ctx.Punish = restorePunish
		ctx.Proposal = restoreProposal
		ctx.Staking = restoreStaking
	})
}

func logConsensusLivenessFailure(t *testing.T, scenario string, stoppedValidator common.Address, beforeProgress, afterProgress uint64, beforeSamples, afterSamples []testkit.HeightSample, err error, businessAssertionsStarted bool) {
	t.Helper()
	t.Logf(
		"failure_type=ConsensusLivenessFailure scenario=%s network=3-validators stopped_validator=%s last_height_before_stall=%d competing_block_observed=unknown recovered_after_restart=false business_assertions_started=%t err=%v before_samples=%v after_samples=%v recent_coinbases=%v runtime_signers=%v",
		scenario,
		stoppedValidator.Hex(),
		beforeProgress,
		businessAssertionsStarted,
		err,
		beforeSamples,
		afterSamples,
		testkit.RecentCoinbases(ctx, 12),
		sortedRuntimeSigners(t),
	)
}

func TestZ_Liveness_StopOneValidator_RemainingTwoStillSeal(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	ensureMinActiveValidators(t, 3, 1)

	consensusSigners := sortedRuntimeSigners(t)
	if len(consensusSigners) != 3 {
		t.Fatalf("expected clean 3-validator scenario, got %d signers: %v", len(consensusSigners), consensusSigners)
	}

	targetSigner := consensusSigners[len(consensusSigners)-1]
	targetVal, err := ctx.ValidatorAddressBySigner(targetSigner)
	if err != nil {
		t.Fatalf("resolve validator for signer %s failed: %v", targetSigner.Hex(), err)
	}
	if targetVal == (common.Address{}) {
		t.Fatalf("runtime signer %s has no validator mapping", targetSigner.Hex())
	}
	targetSignerAddr, targetSignerKey := resolveValidatorSignerIdentity(t, targetVal)
	if targetSignerAddr != targetSigner {
		t.Fatalf("runtime signer drifted: validator=%s runtime=%s expected=%s", targetVal.Hex(), targetSignerAddr.Hex(), targetSigner.Hex())
	}
	switchObserverIfStoppingPrimaryValidator(t, targetVal)

	t.Cleanup(func() {
		if err := testkit.RestartValidatorNodeWithSigner(ctx, targetVal, targetSignerKey, 90*time.Second); err != nil {
			t.Errorf("restart validator %s after liveness stop failed: %v", targetVal.Hex(), err)
		}
	})

	head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || head == nil {
		t.Fatalf("read latest header for liveness probe failed: %v", err)
	}
	startHeight := head.Number.Uint64()
	if _, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"single-validator failure liveness pre-stop alignment",
		startHeight+1,
		ctx.BlockPollInterval(),
		testkit.LongWindowTimeout(1),
	); err != nil {
		t.Fatalf("align pre-stop liveness window failed: %v", err)
	}

	if err := testkit.StopValidatorNode(ctx, targetVal, 30*time.Second); err != nil {
		t.Fatalf("stop validator %s for liveness probe failed: %v", targetVal.Hex(), err)
	}
	beforeProgress, beforeSamples := testkit.ObserveHeights(ctx)
	targetHeight := beforeProgress + 3
	if _, err := testkit.WaitUntilHeightOrStall(
		ctx,
		"single-validator failure chain liveness",
		targetHeight,
		15*time.Second,
		testkit.LongWindowTimeout(3),
	); err != nil {
		afterProgress, afterSamples := testkit.ObserveHeights(ctx)
		logConsensusLivenessFailure(t, "stop-one-validator", targetVal, beforeProgress, afterProgress, beforeSamples, afterSamples, err, false)
		t.Fatalf("chain liveness failed after stopping one validator in a 3-validator network: stopped=%s before=%d after=%d target=%d err=%v", targetVal.Hex(), beforeProgress, afterProgress, targetHeight, err)
	}
}

func TestZ_ExecutePendingAutoByConsensus(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}

	const (
		configIDPunishThreshold int64 = 1
		configIDRemoveThreshold int64 = 2
	)

	ensureMinActiveValidators(t, 3, 1)

	punishThreshold, err := ctx.Proposal.PunishThreshold(nil)
	if err != nil || punishThreshold == nil {
		t.Fatalf("read punishThreshold failed: %v", err)
	}
	removeThreshold, err := ctx.Proposal.RemoveThreshold(nil)
	if err != nil || removeThreshold == nil {
		t.Fatalf("read removeThreshold failed: %v", err)
	}
	origPunishThreshold := new(big.Int).Set(punishThreshold)
	origRemoveThreshold := new(big.Int).Set(removeThreshold)
	t.Cleanup(func() {
		if err := ctx.EnsureConfig(configIDPunishThreshold, origPunishThreshold, big.NewInt(1)); err != nil {
			t.Errorf("restore punishThreshold failed: %v", err)
		}
		if err := ctx.EnsureConfig(configIDRemoveThreshold, origRemoveThreshold, big.NewInt(1000)); err != nil {
			t.Errorf("restore removeThreshold failed: %v", err)
		}
	})

	if err := ctx.EnsureConfig(configIDPunishThreshold, big.NewInt(1), punishThreshold); err != nil {
		t.Fatalf("set punishThreshold=1 failed: %v", err)
	}
	if err := ctx.EnsureConfig(configIDRemoveThreshold, big.NewInt(1000), removeThreshold); err != nil {
		t.Fatalf("set removeThreshold=1000 failed: %v", err)
	}

	epochBig, err := ctx.Proposal.Epoch(nil)
	if err != nil || epochBig == nil || epochBig.Sign() == 0 {
		t.Fatalf("epoch not available: %v", err)
	}
	epoch := epochBig.Uint64()

	consensusSigners := sortedRuntimeSigners(t)
	if len(consensusSigners) != 3 {
		t.Fatalf("expected clean 3-validator scenario, got %d signers: %v", len(consensusSigners), consensusSigners)
	}

	head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || head == nil {
		t.Fatalf("read latest header failed: %v", err)
	}
	currentHeight := head.Number.Uint64()
	checkpoint := ((currentHeight / epoch) + 1) * epoch
	if checkpoint <= currentHeight+3 {
		checkpoint += epoch
	}
	stopHeight := checkpoint - 2
	if stopHeight <= currentHeight {
		t.Fatalf("invalid stop window: current=%d stop=%d checkpoint=%d", currentHeight, stopHeight, checkpoint)
	}

	targetSigner := consensusSigners[int(checkpoint%uint64(len(consensusSigners)))]
	targetVal, err := ctx.ValidatorAddressBySigner(targetSigner)
	if err != nil {
		t.Fatalf("resolve validator for checkpoint signer %s failed: %v", targetSigner.Hex(), err)
	}
	if targetVal == (common.Address{}) {
		t.Fatalf("checkpoint signer %s has no validator mapping", targetSigner.Hex())
	}
	targetSignerAddr, targetSignerKey := signerIdentityForValidator(targetVal, keyForAddress(targetVal))
	if targetSignerAddr != targetSigner {
		t.Fatalf("runtime signer drifted: validator=%s runtime=%s schedule=%s", targetVal.Hex(), targetSignerAddr.Hex(), targetSigner.Hex())
	}
	if targetSignerKey == nil {
		t.Fatalf("missing signer key for validator %s signer=%s", targetVal.Hex(), targetSigner.Hex())
	}

	pendingHeadAt := func(height uint64) (common.Address, bool, error) {
		addr, err := ctx.Punish.PendingValidators(callAt(height), big.NewInt(0))
		if err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "execution reverted") || strings.Contains(lower, "index out of bounds") {
				return common.Address{}, false, nil
			}
			return common.Address{}, false, err
		}
		return addr, true, nil
	}
	if _, has, err := pendingHeadAt(0); err != nil {
		t.Fatalf("read pending queue before isolated P-24 failed: %v", err)
	} else if has {
		t.Fatalf("pending queue should be empty before isolated P-24 assertion")
	}

	t.Logf(
		"P-24 isolated target validator=%s signer=%s stop_height=%d checkpoint=%d signers=%v",
		targetVal.Hex(),
		targetSigner.Hex(),
		stopHeight,
		checkpoint,
		consensusSigners,
	)

	switchObserverIfStoppingPrimaryValidator(t, targetVal)

	t.Cleanup(func() {
		if err := testkit.RestartValidatorNodeWithSigner(ctx, targetVal, targetSignerKey, 90*time.Second); err != nil {
			t.Errorf("restart validator %s after isolated P-24 failed: %v", targetVal.Hex(), err)
		}
	})
	heightBeforeStop, err := testkit.WaitUntilClientHeight(
		ctx.Clients[0],
		"P-24 isolated wait until stop window",
		stopHeight,
		ctx.BlockPollInterval(),
		testkit.LongWindowTimeout(stopHeight),
	)
	if err != nil {
		t.Fatalf("wait for stop window failed: %v", err)
	}
	if heightBeforeStop < stopHeight {
		observedMax, observedSamples := testkit.ObserveHeights(ctx)
		t.Fatalf("stop window not reached: want>=%d local=%d observed_max=%d samples=%v", stopHeight, heightBeforeStop, observedMax, observedSamples)
	}
	if heightBeforeStop > stopHeight+1 {
		t.Skipf("missed precise stop window: stop_height=%d current=%d", stopHeight, heightBeforeStop)
	}
	if err := testkit.StopValidatorNode(ctx, targetVal, 30*time.Second); err != nil {
		t.Fatalf("stop target validator %s failed: %v", targetVal.Hex(), err)
	}
	beforeProgress, beforeSamples := testkit.ObserveHeights(ctx)
	if _, err := testkit.WaitUntilHeightOrStall(
		ctx,
		"P-24 isolated post-stop chain recovery",
		beforeProgress+1,
		15*time.Second,
		30*time.Second,
	); err != nil {
		afterProgress, afterSamples := testkit.ObserveHeights(ctx)
		logConsensusLivenessFailure(t, "pending-auto-execute", targetVal, beforeProgress, afterProgress, beforeSamples, afterSamples, err, false)
		t.Fatalf("chain liveness failed after stopping one validator before pending auto execute could be verified: stopped=%s before=%d after=%d err=%v", targetVal.Hex(), beforeProgress, afterProgress, err)
	}

	if _, err := testkit.WaitUntilHeightOrStall(
		ctx,
		"P-24 isolated reach checkpoint",
		checkpoint,
		20*time.Second,
		testkit.LongWindowTimeout(checkpoint-stopHeight),
	); err != nil {
		t.Fatalf("wait for checkpoint failed: %v", err)
	}

	if _, err := testkit.WaitUntilHeightOrStall(
		ctx,
		"P-24 isolated executePending auto-consume",
		checkpoint+1,
		20*time.Second,
		testkit.LongWindowTimeout(1),
	); err != nil {
		t.Fatalf("wait for post-epoch non-epoch block failed: %v", err)
	}

	recordBeforeCheckpoint, err := ctx.Punish.GetPunishRecord(callAt(checkpoint-1), targetVal)
	if err != nil {
		t.Fatalf("read punish record before checkpoint failed: %v", err)
	}
	if recordBeforeCheckpoint == nil || recordBeforeCheckpoint.Sign() != 0 {
		t.Fatalf("expected zero punish record before checkpoint, got %v", recordBeforeCheckpoint)
	}

	pendingAtCheckpoint, hasPendingAtCheckpoint, err := pendingHeadAt(checkpoint)
	if err != nil {
		t.Fatalf("read pending queue at checkpoint failed: %v", err)
	}
	if !hasPendingAtCheckpoint {
		t.Fatalf("pending queue entry not created at checkpoint block %d", checkpoint)
	}
	if pendingAtCheckpoint != targetVal {
		t.Fatalf("pending queue head mismatch at checkpoint: want=%s got=%s", targetVal.Hex(), pendingAtCheckpoint.Hex())
	}

	if pendingAtPost, hasPendingAtPost, err := pendingHeadAt(checkpoint + 1); err != nil {
		t.Fatalf("read pending queue after checkpoint failed: %v", err)
	} else if hasPendingAtPost {
		t.Fatalf("pending queue was not auto-consumed by consensus execution at block %d: head=%s", checkpoint+1, pendingAtPost.Hex())
	}

	info, err := ctx.Staking.GetValidatorInfo(callAt(checkpoint+1), targetVal)
	if err != nil {
		t.Fatalf("read validator info after auto-consume failed: %v", err)
	}
	if info.IsJailed {
		t.Fatalf("validator %s should not be jailed by pending incoming auto-consume", targetVal.Hex())
	}
	activeAfterConsume, err := ctx.Validators.IsValidatorActive(callAt(checkpoint+1), targetVal)
	if err != nil {
		t.Fatalf("read active state after auto-consume failed: %v", err)
	}
	if !activeAfterConsume {
		t.Fatalf("validator %s should remain active after pending incoming auto-consume", targetVal.Hex())
	}

	executePendingSelector := crypto.Keccak256([]byte("executePending(uint256)"))[:4]
	for _, height := range []uint64{checkpoint, checkpoint + 1} {
		block, err := ctx.Clients[0].BlockByNumber(context.Background(), new(big.Int).SetUint64(height))
		if err != nil || block == nil {
			t.Fatalf("read block %d for executePending tx scan failed: %v", height, err)
		}
		for _, tx := range block.Transactions() {
			to := tx.To()
			if to == nil || *to != testctx.PunishAddr {
				continue
			}
			data := tx.Data()
			if len(data) >= 4 && bytes.Equal(data[:4], executePendingSelector) {
				t.Fatalf("found unexpected external executePending tx in block %d: %s", height, tx.Hash().Hex())
			}
		}
	}
}
