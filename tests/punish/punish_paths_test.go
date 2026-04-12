package tests

import (
	"bytes"
	"context"
	"math/big"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestG_PunishPaths(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}

	// [P-23] Punish Normal (Must be called by Miner)
	t.Run("P-23_PunishNormal", func(t *testing.T) {
		// Only the current block proposer can call punish
		// This is hard to test from outside unless we are the miner.
		// In integration tests, we can try calling it from Validator 0
		// who is likely to be a miner occasionally.

		valKey := ctx.GenesisValidators[0]
		targetVal := common.HexToAddress(ctx.Config.Validators[1].Address)

		opts, err := ctx.GetTransactor(valKey)
		if err != nil {
			t.Fatal(err)
		}

		_, err = ctx.Punish.Punish(opts, targetVal)
		if err != nil {
			t.Logf("punish call failed (as expected if not miner): %v", err)
		} else {
			t.Log("punish call succeeded (Validator 0 is current miner)")
		}
	})

	// [P-24] ExecutePending must be blocked for external tx.
	t.Run("P-24_ExecutePendingForbiddenExternalTx", func(t *testing.T) {
		opts, err := ctx.GetTransactor(ctx.GenesisValidators[0])
		if err != nil {
			t.Fatal(err)
		}
		_, err = ctx.Punish.ExecutePending(opts, big.NewInt(1))
		if err == nil {
			t.Fatalf("expected executePending external tx to be rejected")
		}
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "forbidden system transaction") && !strings.Contains(msg, "miner only") {
			t.Fatalf("expected forbidden system tx or miner-only rejection, got: %v", err)
		}
	})

	// [P-24] ExecutePending should be auto-executed by consensus without external tx.
	t.Run("P-24_ExecutePendingAutoByConsensus", func(t *testing.T) {
		const (
			configIDPunishThreshold int64 = 1
			configIDRemoveThreshold int64 = 2
		)

		ensureMinActiveValidators(t, 3, 2)

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

		epochBig, err := ctx.Proposal.Epoch(nil)
		if err != nil || epochBig == nil || epochBig.Sign() == 0 {
			t.Fatalf("epoch not available: %v", err)
		}
		epoch := epochBig.Uint64()
		if epoch > 600 {
			t.Skipf("epoch=%d too large for bounded local integration assertion", epoch)
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

		// Stabilize to an empty queue before creating a deterministic pending entry.
		for i := 0; i < 20; i++ {
			if _, has, err := pendingHeadAt(0); err == nil && !has {
				break
			}
			waitBlocks(t, 1)
		}
		if _, has, err := pendingHeadAt(0); err != nil {
			t.Fatalf("read pending queue before P-24 failed: %v", err)
		} else if has {
			t.Fatalf("pending queue should be empty before P-24 auto-execution assertion")
		}

		runtimeSigners, err := ctx.TopRuntimeSigners()
		if err != nil {
			t.Fatalf("read runtime signers failed: %v", err)
		}
		consensusSigners := append([]common.Address(nil), runtimeSigners...)
		sort.Slice(consensusSigners, func(i, j int) bool {
			return bytes.Compare(consensusSigners[i][:], consensusSigners[j][:]) < 0
		})
		if len(consensusSigners) < 3 {
			t.Skipf("need at least 3 consensus signers to keep chain live while one signer is stopped, got %d", len(consensusSigners))
		}

		signerToValidator := make(map[common.Address]common.Address, len(consensusSigners))
		for _, signer := range consensusSigners {
			validator, err := ctx.ValidatorAddressBySigner(signer)
			if err != nil {
				t.Fatalf("resolve validator for signer %s failed: %v", signer.Hex(), err)
			}
			if validator == (common.Address{}) {
				t.Fatalf("runtime signer %s has no validator mapping", signer.Hex())
			}
			signerToValidator[signer] = validator
		}

		scheduleSignerAt := func(height uint64) common.Address {
			return consensusSigners[int(height%uint64(len(consensusSigners)))]
		}
		countScheduledMisses := func(fromExclusive, toInclusive uint64, signer common.Address) uint64 {
			if toInclusive <= fromExclusive {
				return 0
			}
			var misses uint64
			for height := fromExclusive + 1; height <= toInclusive; height++ {
				if scheduleSignerAt(height) == signer {
					misses++
				}
			}
			return misses
		}

		activeVals, err := ctx.Validators.GetActiveValidators(nil)
		if err != nil {
			t.Fatalf("read active validators failed: %v", err)
		}
		head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || head == nil {
			t.Fatalf("read latest header before P-24 alignment failed: %v", err)
		}
		currentHeight := head.Number.Uint64()

		seenCoinbases := testkit.RecentCoinbases(ctx, len(consensusSigners)*4)
		seenSigners := make(map[common.Address]bool, len(seenCoinbases))
		for _, signer := range seenCoinbases {
			seenSigners[signer] = true
		}

		var checkpoint uint64
		var stopHeight uint64
		var targetVal common.Address
		var targetScheduleSigner common.Address
		var targetPunishThreshold *big.Int
		useDormantTarget := false

		nextCheckpoint := ((currentHeight / epoch) + 1) * epoch
		checkpointSlotSigner := scheduleSignerAt(nextCheckpoint)
		var dormantCandidates []common.Address
		for _, signer := range consensusSigners {
			if seenSigners[signer] {
				continue
			}
			dormantCandidates = append(dormantCandidates, signer)
		}
		if len(dormantCandidates) > 0 {
			if !seenSigners[checkpointSlotSigner] {
				targetScheduleSigner = checkpointSlotSigner
				targetVal = signerToValidator[targetScheduleSigner]
				useDormantTarget = true
			} else {
				dormantHex := make([]string, 0, len(dormantCandidates))
				for _, signer := range dormantCandidates {
					dormantHex = append(dormantHex, signer.Hex())
				}
				t.Skipf(
					"P-24 shared state already has dormant validator signers on non-checkpoint slots; stopping another live signer would risk chain liveness (checkpoint_signer=%s dormant=%s signers=%v)",
					checkpointSlotSigner.Hex(),
					strings.Join(dormantHex, ","),
					consensusSigners,
				)
			}
		}

		if useDormantTarget {
			var missCount uint64
			var recordNow *big.Int
			for attempt := 0; attempt < 3; attempt++ {
				head, err = ctx.Clients[0].HeaderByNumber(context.Background(), nil)
				if err != nil || head == nil {
					t.Fatalf("read latest header for dormant-target alignment failed: %v", err)
				}
				currentHeight = head.Number.Uint64()
				if nextCheckpoint <= currentHeight+3 {
					if _, err := testkit.WaitUntilHeightOrStall(
						ctx,
						"P-24 dormant-target next checkpoint",
						nextCheckpoint+1,
						20*time.Second,
						testkit.LongWindowTimeout(nextCheckpoint+1-currentHeight),
					); err != nil {
						t.Fatalf("%v", err)
					}
					continue
				}
				missCount = countScheduledMisses(currentHeight, nextCheckpoint, targetScheduleSigner)
				if missCount == 0 {
					if _, err := testkit.WaitUntilHeightOrStall(
						ctx,
						"P-24 dormant-target align next epoch",
						nextCheckpoint+1,
						20*time.Second,
						testkit.LongWindowTimeout(nextCheckpoint+1-currentHeight),
					); err != nil {
						t.Fatalf("%v", err)
					}
					continue
				}

				recordNow, err = ctx.Punish.GetPunishRecord(callAt(currentHeight), targetVal)
				if err != nil {
					t.Fatalf("read punish record for dormant target failed: %v", err)
				}
				checkpoint = nextCheckpoint
				break
			}
			if checkpoint == 0 {
				t.Fatalf("failed to align dormant target %s to a checkpoint window", targetVal.Hex())
			}

			targetPunishThreshold = new(big.Int).Add(recordNow, new(big.Int).SetUint64(missCount))
			t.Logf(
				"P-24 using dormant active validator=%s signer=%s current_record=%s miss_count=%d checkpoint=%d recent_coinbases=%v consensus_signers=%v",
				targetVal.Hex(),
				targetScheduleSigner.Hex(),
				recordNow.String(),
				missCount,
				checkpoint,
				seenCoinbases,
				consensusSigners,
			)
		} else {
			// Use a two-cycle miss window so the first two misses happen on non-epoch
			// blocks, and the third miss lands on the checkpoint block to enqueue the
			// pending removal entry that consensus should auto-consume on the next block.
			targetPunishThreshold = big.NewInt(3)
			missWindow := uint64(len(consensusSigners)) * uint64(targetPunishThreshold.Int64()-1)
			if epoch <= missWindow+1 {
				t.Skipf("epoch=%d too small for deterministic P-24 stop window (need > %d)", epoch, missWindow+1)
			}

			nextCheckpoint := ((currentHeight / epoch) + 1) * epoch
			for attempt := 0; attempt < len(consensusSigners)*4; attempt++ {
				candidateCheckpoint := nextCheckpoint + uint64(attempt)*epoch
				if candidateCheckpoint <= currentHeight {
					continue
				}
				candidateStop := candidateCheckpoint - missWindow - 1
				if candidateStop <= currentHeight+3 {
					continue
				}
				candidateSigner := scheduleSignerAt(candidateCheckpoint)
				candidateTarget, ok := signerToValidator[candidateSigner]
				if !ok || candidateTarget == (common.Address{}) {
					continue
				}
				checkpoint = candidateCheckpoint
				stopHeight = candidateStop
				targetScheduleSigner = candidateSigner
				targetVal = candidateTarget
				break
			}
			if checkpoint == 0 || targetVal == (common.Address{}) {
				t.Skipf(
					"no checkpoint window found for P-24 auto-consume (current_height=%d signers=%v active=%v)",
					currentHeight,
					consensusSigners,
					activeVals,
				)
			}

			if _, err := testkit.WaitUntilHeightOrStall(
				ctx,
				"P-24 stop target validator",
				stopHeight,
				20*time.Second,
				testkit.LongWindowTimeout(stopHeight-currentHeight),
			); err != nil {
				t.Fatalf("%v", err)
			}

			head, err = ctx.Clients[0].HeaderByNumber(context.Background(), nil)
			if err != nil || head == nil {
				t.Fatalf("read stop-height header failed: %v", err)
			}
			if head.Number.Uint64() != stopHeight {
				t.Fatalf("missed deterministic stop window: got height=%d want=%d", head.Number.Uint64(), stopHeight)
			}
			if gotMisses := countScheduledMisses(stopHeight, checkpoint, targetScheduleSigner); gotMisses != uint64(targetPunishThreshold.Int64()) {
				t.Fatalf(
					"predicted miss window drifted: signer=%s stop=%d checkpoint=%d got_misses=%d want=%d signers=%v",
					targetScheduleSigner.Hex(),
					stopHeight,
					checkpoint,
					gotMisses,
					targetPunishThreshold.Int64(),
					consensusSigners,
				)
			}
			if scheduleSignerAt(checkpoint) != targetScheduleSigner {
				t.Fatalf(
					"predicted checkpoint signer drifted: want=%s got=%s checkpoint=%d signers=%v",
					targetScheduleSigner.Hex(),
					scheduleSignerAt(checkpoint).Hex(),
					checkpoint,
					consensusSigners,
				)
			}
		}

		targetSigner, targetSignerKey := signerIdentityForValidator(targetVal, keyForAddress(targetVal))
		if targetScheduleSigner != (common.Address{}) && targetSigner != targetScheduleSigner {
			t.Fatalf(
				"P-24 target signer drifted from consensus schedule: validator=%s runtime=%s schedule=%s",
				targetVal.Hex(),
				targetSigner.Hex(),
				targetScheduleSigner.Hex(),
			)
		}
		if !useDormantTarget && targetSignerKey == nil {
			t.Skipf("no signer key available for P-24 target validator %s signer=%s", targetVal.Hex(), targetSigner.Hex())
		}
		t.Logf(
			"P-24 target validator=%s signer=%s stop_height=%d checkpoint=%d active_count=%d dormant=%t punish_threshold=%s consensus_signers=%v",
			targetVal.Hex(),
			targetSigner.Hex(),
			stopHeight,
			checkpoint,
			len(consensusSigners),
			useDormantTarget,
			targetPunishThreshold.String(),
			consensusSigners,
		)

		waitStartHeight := currentHeight
		if !useDormantTarget {
			waitStartHeight = stopHeight
		}

		primaryValidator := common.Address{}
		if ctx.Config != nil && len(ctx.Config.Validators) > 0 {
			primaryValidator = common.HexToAddress(ctx.Config.Validators[0].Address)
		}
		if !useDormantTarget && primaryValidator != (common.Address{}) && targetVal == primaryValidator {
			observerURL := ""
			if ctx.Config != nil {
				observerURL = strings.TrimSpace(ctx.Config.SyncRPC)
				if observerURL == "" {
					targetRPC := strings.TrimSpace(ctx.ValidatorRPCByValidator(targetVal))
					for _, rpcURL := range ctx.Config.ValidatorRPCs {
						rpcURL = strings.TrimSpace(rpcURL)
						if rpcURL != "" && rpcURL != targetRPC {
							observerURL = rpcURL
							break
						}
					}
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
			t.Logf("P-24 switched observer client to %s while primary validator is stopped", observerURL)

			t.Cleanup(func() {
				observerClient.Close()
				ctx.Clients[0] = restoreClient
				ctx.Validators = restoreValidators
				ctx.Punish = restorePunish
				ctx.Proposal = restoreProposal
				ctx.Staking = restoreStaking
			})
		}

		t.Cleanup(func() {
			if useDormantTarget {
				return
			}
			if err := testkit.RestartValidatorNodeWithSigner(ctx, targetVal, targetSignerKey, 90*time.Second); err != nil {
				t.Errorf("restart validator %s after P-24 failed: %v", targetVal.Hex(), err)
			}
		})
		if !useDormantTarget {
			if err := testkit.StopValidatorNode(ctx, targetVal, 30*time.Second); err != nil {
				t.Fatalf("stop target validator %s failed: %v", targetVal.Hex(), err)
			}
		}

		if _, err := testkit.WaitUntilHeightOrStall(
			ctx,
			"P-24 executePending auto-consume",
			checkpoint+1,
			20*time.Second,
			testkit.LongWindowTimeout(checkpoint+1-waitStartHeight),
		); err != nil {
			t.Fatalf("wait for post-epoch non-epoch block failed: %v", err)
		}

		recordBeforeCheckpoint, err := ctx.Punish.GetPunishRecord(callAt(checkpoint-1), targetVal)
		if err != nil {
			t.Fatalf("read punish record before checkpoint failed: %v", err)
		}
		expectedRecordBeforeCheckpoint := new(big.Int).Sub(targetPunishThreshold, big.NewInt(1))
		if recordBeforeCheckpoint == nil || recordBeforeCheckpoint.Cmp(expectedRecordBeforeCheckpoint) != 0 {
			t.Fatalf(
				"expected punish record=%s before checkpoint, got %v",
				expectedRecordBeforeCheckpoint.String(),
				recordBeforeCheckpoint,
			)
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
			t.Fatalf(
				"pending queue was not auto-consumed by consensus execution at block %d: head=%s",
				checkpoint+1,
				pendingAtPost.Hex(),
			)
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
	})

	// [P-25] DecreaseMissedBlocksCounter
	t.Run("P-25_DecreaseMissedBlocksCounter", func(t *testing.T) {
		// Only called on Epoch blocks. If called on non-epoch, should revert.
		opts, err := ctx.GetTransactor(ctx.GenesisValidators[0])
		if err != nil {
			t.Fatal(err)
		}

		_, err = ctx.Punish.DecreaseMissedBlocksCounter(opts, big.NewInt(1))
		if err != nil {
			t.Logf("decreaseMissedBlocksCounter failed: %v", err)
		} else {
			t.Log("decreaseMissedBlocksCounter succeeded (lucky hit on epoch block)")
		}
	})
}
