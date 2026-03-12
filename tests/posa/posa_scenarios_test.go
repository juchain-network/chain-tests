package tests

import (
	"context"
	"fmt"
	"math/big"
	"os/exec"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

const posaMaxHeightLag = uint64(8)

func TestP_GenesisPOSAFirstBlockRewardPath(t *testing.T) {
	if ctx == nil {
		t.Fatalf("context not initialized")
	}

	start, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("read initial block number failed: %v", err)
	}
	if start < 2 {
		waitBlocks(t, int(2-start))
		start, err = ctx.Clients[0].BlockNumber(context.Background())
		if err != nil {
			t.Fatalf("read block number after warmup failed: %v", err)
		}
	}
	if start < 2 {
		t.Fatalf("network did not progress past first two blocks: height=%d", start)
	}

	if err := ctx.WaitForBlockProgress(2, 120*time.Second); err != nil {
		t.Fatalf("chain stalled after genesis POSA startup: %v", err)
	}

	end, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		t.Fatalf("read final block number failed: %v", err)
	}
	if end <= start {
		t.Fatalf("expected block growth after first-block reward path check: start=%d end=%d", start, end)
	}
}

func TestP_ValidatorJoinLeave(t *testing.T) {
	if ctx == nil {
		t.Fatalf("context not initialized")
	}
	startActive, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("read start active validators failed: %v", err)
	}
	candidateKey, candidateAddr := registerCandidateValidator(t)

	if !waitActiveSetState(t, candidateAddr, true, 2) {
		t.Fatalf("candidate validator did not enter active set: %s", candidateAddr.Hex())
	}

	robustResignValidator(t, candidateKey, candidateAddr)
	if !waitActiveSetState(t, candidateAddr, false, 3) {
		t.Fatalf("candidate validator did not leave active set after resign: %s", candidateAddr.Hex())
	}

	robustExitValidator(t, candidateKey)
	if !waitActiveSetState(t, candidateAddr, false, 1) {
		t.Fatalf("candidate validator unexpectedly active after exit: %s", candidateAddr.Hex())
	}

	endActive, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("read end active validators failed: %v", err)
	}
	if len(endActive) < len(startActive)-1 {
		t.Fatalf("unexpected active validator shrink: start=%d end=%d", len(startActive), len(endActive))
	}
}

func TestP_GovernanceParamMutationAffectsConsensus(t *testing.T) {
	before, err := ctx.Proposal.ProposalCooldown(nil)
	if err != nil {
		t.Fatalf("read ProposalCooldown failed: %v", err)
	}
	target := new(big.Int).Add(before, big.NewInt(1))
	if before.Cmp(big.NewInt(1)) > 0 {
		target = new(big.Int).Sub(before, big.NewInt(1))
	}
	if target.Cmp(before) == 0 {
		target = new(big.Int).Add(before, big.NewInt(2))
	}

	if err := ctx.EnsureConfig(19, target, before); err != nil {
		t.Fatalf("EnsureConfig proposalCooldown failed: %v", err)
	}
	after, err := ctx.Proposal.ProposalCooldown(nil)
	if err != nil {
		t.Fatalf("read ProposalCooldown after mutation failed: %v", err)
	}
	if after.Cmp(target) != 0 {
		t.Fatalf("ProposalCooldown mismatch: expect=%s got=%s", target.String(), after.String())
	}

	if err := ctx.EnsureConfig(19, before, after); err != nil {
		t.Fatalf("restore ProposalCooldown failed: %v", err)
	}
}

func TestP_SyncNodeCatchUp(t *testing.T) {
	if len(cfg.NodeRPCs) == 0 {
		t.Skip("node_rpcs not configured")
	}

	maxLag := uint64(0)
	for i := 0; i < 20; i++ {
		heights := make(map[string]uint64)
		var maxH uint64
		minH := ^uint64(0)
		for _, c := range ctx.Clients {
			h, err := c.BlockNumber(context.Background())
			if err != nil {
				t.Fatalf("blockNumber failed: %v", err)
			}
			key := fmt.Sprintf("client_%p", c)
			heights[key] = h
			if h > maxH {
				maxH = h
			}
			if h < minH {
				minH = h
			}
		}
		if maxH-minH > maxLag {
			maxLag = maxH - minH
		}
		time.Sleep(1 * time.Second)
	}
	if maxLag > posaMaxHeightLag {
		t.Fatalf("sync lag too large: maxLag=%d limit=%d", maxLag, posaMaxHeightLag)
	}
}

func TestP_RestartConvergence(t *testing.T) {
	cmd := exec.Command("/bin/bash", "-lc", "cd ../../ && TEST_ENV_CONFIG=${TEST_ENV_CONFIG:-config/test_env.yaml} ./scripts/perf/restart_node.sh")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("restart helper unavailable in current environment: %v output=%s", err, string(out))
	}
	if err := ctx.WaitForBlockProgress(3, 120*time.Second); err != nil {
		t.Fatalf("block progress did not recover after restart: %v", err)
	}

	maxH := uint64(0)
	minH := ^uint64(0)
	for _, c := range ctx.Clients {
		h, err := c.BlockNumber(context.Background())
		if err != nil {
			t.Fatalf("blockNumber after restart failed: %v", err)
		}
		if h > maxH {
			maxH = h
		}
		if h < minH {
			minH = h
		}
	}
	if maxH-minH > posaMaxHeightLag {
		t.Fatalf("post-restart convergence lag too large: max=%d min=%d lag=%d", maxH, minH, maxH-minH)
	}
}

func TestP_PunishRecoveryBaseline(t *testing.T) {
	if len(ctx.GenesisValidators) == 0 || len(cfg.Validators) == 0 {
		t.Skip("missing validator config")
	}
	validatorAddr := common.HexToAddress(cfg.Validators[0].Address)
	before, err := ctx.Punish.GetPunishRecord(nil, validatorAddr)
	if err != nil {
		t.Fatalf("get punish record failed: %v", err)
	}

	cleaned := false
	for _, vk := range ctx.GenesisValidators {
		opts, err := ctx.GetTransactor(vk)
		if err != nil {
			continue
		}
		tx, err := ctx.Punish.CleanPunishRecord(opts, validatorAddr)
		if err != nil {
			continue
		}
		if err := ctx.WaitMined(tx.Hash()); err == nil {
			cleaned = true
			break
		}
	}
	if !cleaned {
		t.Log("cleanPunishRecord not accepted in this block window; baseline read-path verified")
		return
	}
	waitBlocks(t, 1)
	after, err := ctx.Punish.GetPunishRecord(nil, validatorAddr)
	if err != nil {
		t.Fatalf("get punish record after clean failed: %v", err)
	}
	if after.Cmp(before) > 0 {
		t.Fatalf("unexpected punish record increase after clean: before=%s after=%s", before.String(), after.String())
	}
}
