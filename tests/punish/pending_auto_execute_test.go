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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/testkit"
)

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

	runtimeSigners, err := ctx.TopRuntimeSigners()
	if err != nil {
		t.Fatalf("read runtime signers failed: %v", err)
	}
	consensusSigners := append([]common.Address(nil), runtimeSigners...)
	sort.Slice(consensusSigners, func(i, j int) bool {
		return bytes.Compare(consensusSigners[i][:], consensusSigners[j][:]) < 0
	})
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

	if _, err := testkit.WaitUntilHeightOrStall(
		ctx,
		"P-24 isolated stop target validator",
		stopHeight,
		20*time.Second,
		testkit.LongWindowTimeout(stopHeight-currentHeight),
	); err != nil {
		t.Fatalf("%v", err)
	}

	primaryValidator := common.Address{}
	if ctx.Config != nil && len(ctx.Config.Validators) > 0 {
		primaryValidator = common.HexToAddress(ctx.Config.Validators[0].Address)
	}
	if targetVal == primaryValidator {
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
		t.Logf("P-24 isolated switched observer client to %s while primary validator is stopped", observerURL)

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
		if err := testkit.RestartValidatorNodeWithSigner(ctx, targetVal, targetSignerKey, 90*time.Second); err != nil {
			t.Errorf("restart validator %s after isolated P-24 failed: %v", targetVal.Hex(), err)
		}
	})
	if err := testkit.StopValidatorNode(ctx, targetVal, 30*time.Second); err != nil {
		t.Fatalf("stop target validator %s failed: %v", targetVal.Hex(), err)
	}

	if _, err := testkit.WaitUntilHeightOrStall(
		ctx,
		"P-24 isolated executePending auto-consume",
		checkpoint+1,
		20*time.Second,
		testkit.LongWindowTimeout(checkpoint+1-stopHeight),
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
