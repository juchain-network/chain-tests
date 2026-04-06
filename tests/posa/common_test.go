package tests

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"juchain.org/chain/tools/ci/internal/config"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/testkit"
)

var (
	ctx        *testctx.CIContext
	cfg        *config.Config
	configPath = flag.String("config", "../../data/test_config.yaml", "Path to generated test configuration file")
)

func TestMain(m *testing.M) {
	flag.Parse()
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true)))

	loadedCfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Error("Failed to load config", "err", err)
		os.Exit(1)
	}
	if len(loadedCfg.RPCs) == 0 {
		log.Error("No RPCs configured in test config", "config", *configPath)
		os.Exit(1)
	}
	if loadedCfg.Funder.PrivateKey == "" || loadedCfg.Funder.Address == "" {
		log.Error("Funder config missing address or private_key", "config", *configPath)
		os.Exit(1)
	}
	c, err := testctx.NewCIContext(loadedCfg)
	if err != nil {
		log.Error("Failed to init context", "err", err)
		os.Exit(1)
	}
	cfg = loadedCfg
	ctx = c
	os.Exit(m.Run())
}

func retrySleep() time.Duration {
	if ctx != nil {
		return ctx.RetryPollInterval()
	}
	return 100 * time.Millisecond
}

func waitBlocks(t *testing.T, n int) {
	t.Helper()
	if n <= 0 {
		return
	}
	if err := ctx.WaitForBlockProgress(n, 120*time.Second); err != nil {
		t.Fatalf("wait for %d blocks failed: %v", n, err)
	}
}

func getProposalID(tx *types.Transaction) [32]byte {
	var receipt *types.Receipt
	for i := 0; i < 15; i++ {
		r, err := ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
		if err == nil && r != nil {
			receipt = r
			break
		}
		time.Sleep(retrySleep())
	}
	if receipt == nil {
		return [32]byte{}
	}
	for _, lg := range receipt.Logs {
		if ev, err := ctx.Proposal.ParseLogCreateProposal(*lg); err == nil {
			return ev.Id
		}
	}
	return [32]byte{}
}

func robustVote(t *testing.T, voter *ecdsa.PrivateKey, proposalID [32]byte) {
	t.Helper()
	voterAddr := crypto.PubkeyToAddress(voter.PublicKey)
	for retry := 0; retry < 12; retry++ {
		ctx.WaitIfEpochBlock()
		opts, err := ctx.GetTransactor(voter)
		if err != nil {
			time.Sleep(retrySleep())
			continue
		}
		tx, err := ctx.Proposal.VoteProposal(opts, proposalID, true)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "epoch block forbidden") {
				time.Sleep(retrySleep())
				continue
			}
			t.Fatalf("vote proposal failed: %v", err)
		}
		if err := ctx.WaitMined(tx.Hash()); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "epoch block forbidden") {
				time.Sleep(retrySleep())
				continue
			}
			if shouldRetryVoteRevert(t, voterAddr, proposalID, tx.Hash()) {
				time.Sleep(retrySleep())
				continue
			}
			t.Fatalf("vote proposal tx failed: %v", err)
		}
		return
	}
	t.Fatalf("vote proposal retries exhausted")
}

func shouldRetryVoteRevert(t *testing.T, voterAddr common.Address, proposalID [32]byte, txHash common.Hash) bool {
	t.Helper()
	if ctx == nil {
		return false
	}
	if txHash != (common.Hash{}) {
		if receipt, err := ctx.Clients[0].TransactionReceipt(context.Background(), txHash); err == nil && receipt != nil && receipt.BlockNumber != nil {
			epoch := uint64(currentEpochLength())
			if epochVal, err := ctx.Validators.Epoch(nil); err == nil && epochVal != nil && epochVal.Sign() > 0 {
				epoch = epochVal.Uint64()
			}
			if epoch > 0 && receipt.BlockNumber.Uint64()%epoch == 0 {
				return true
			}
		}
	}

	if active, err := ctx.Validators.IsValidatorActive(nil, voterAddr); err == nil && !active {
		return false
	}
	if vote, err := ctx.Proposal.Votes(nil, voterAddr, proposalID); err == nil && vote.VoteTime != nil && vote.VoteTime.Sign() > 0 {
		return false
	}
	if result, err := ctx.Proposal.Results(nil, proposalID); err == nil && result.ResultExist {
		return false
	}
	proposal, err := ctx.Proposal.Proposals(nil, proposalID)
	if err != nil || proposal.CreateTime == nil || proposal.CreateTime.Sign() == 0 {
		return false
	}

	if proposal.CreateBlock != nil && proposal.CreateBlock.Sign() > 0 {
		if lasting, err := ctx.Proposal.ProposalLastingPeriod(nil); err == nil && lasting != nil && lasting.Sign() > 0 {
			if head, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil); err == nil && head != nil {
				expireHeight := new(big.Int).Add(proposal.CreateBlock, lasting)
				if head.Number != nil && head.Number.Cmp(expireHeight) >= 0 {
					return false
				}
			}
		}
	}
	return true
}

func isActiveValidator(t *testing.T, addr common.Address) bool {
	t.Helper()
	active, err := ctx.Validators.IsValidatorActive(nil, addr)
	if err != nil {
		t.Fatalf("query validator active failed: %v", err)
	}
	return active
}

func currentEpochLength() int {
	if ctx != nil && ctx.Config != nil && ctx.Config.Network.Epoch > 0 {
		return int(ctx.Config.Network.Epoch)
	}
	return 30
}

func waitValidatorActiveState(t *testing.T, addr common.Address, target bool, maxEpochs int) bool {
	t.Helper()
	if maxEpochs <= 0 {
		maxEpochs = 1
	}
	epoch := currentEpochLength()
	maxBlocks := maxEpochs*epoch + 4
	for i := 0; i < maxBlocks; i++ {
		active, err := ctx.Validators.IsValidatorActive(nil, addr)
		if err != nil {
			time.Sleep(retrySleep())
			continue
		}
		if active == target {
			return true
		}
		ctx.WaitIfEpochBlock()
		if err := ctx.WaitForBlockProgress(1, 45*time.Second); err != nil {
			time.Sleep(retrySleep())
		}
	}
	return false
}

func inActiveSet(addr common.Address) (bool, error) {
	vals, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		return false, err
	}
	for _, v := range vals {
		if v == addr {
			return true, nil
		}
	}
	return false, nil
}

func waitActiveSetState(t *testing.T, addr common.Address, target bool, maxEpochs int) bool {
	t.Helper()
	if maxEpochs <= 0 {
		maxEpochs = 1
	}
	epoch := currentEpochLength()
	maxBlocks := maxEpochs*epoch + 4
	for i := 0; i < maxBlocks; i++ {
		inSet, err := inActiveSet(addr)
		if err == nil && inSet == target {
			return true
		}
		if err != nil {
			time.Sleep(retrySleep())
			continue
		}
		ctx.WaitIfEpochBlock()
		if err := ctx.WaitForBlockProgress(1, 45*time.Second); err != nil {
			time.Sleep(retrySleep())
		}
	}
	return false
}

func robustExitValidator(t *testing.T, key *ecdsa.PrivateKey) {
	t.Helper()
	var lastErr error
	for retry := 0; retry < 16; retry++ {
		ctx.WaitIfEpochBlock()
		opts, err := ctx.GetTransactor(key)
		if err != nil {
			lastErr = err
			time.Sleep(retrySleep())
			continue
		}

		tx, err := ctx.Staking.ExitValidator(opts)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				lastErr = errW
				msg := strings.ToLower(errW.Error())
				if strings.Contains(msg, "epoch block forbidden") ||
					strings.Contains(msg, "active set") ||
					strings.Contains(msg, "wait until next epoch") {
					waitBlocks(t, 1)
					continue
				}
				t.Fatalf("exit validator tx failed: %v", errW)
			}
		}

		lastErr = err
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "epoch block forbidden") ||
			strings.Contains(msg, "active set") ||
			strings.Contains(msg, "wait until next epoch") {
			waitBlocks(t, 1)
			continue
		}
		t.Fatalf("exit validator call failed: %v", err)
	}

	t.Fatalf("exit validator retries exhausted: %v", lastErr)
}

func robustResignValidator(t *testing.T, key *ecdsa.PrivateKey, addr common.Address) {
	t.Helper()
	var lastErr error
	for retry := 0; retry < 16; retry++ {
		ctx.WaitIfEpochBlock()
		opts, err := ctx.GetTransactor(key)
		if err != nil {
			lastErr = err
			time.Sleep(retrySleep())
			continue
		}

		tx, err := ctx.Staking.ResignValidator(opts)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				lastErr = errW
				msg := strings.ToLower(errW.Error())
				if strings.Contains(msg, "epoch block forbidden") ||
					strings.Contains(msg, "validator not registered") {
					waitBlocks(t, 1)
					continue
				}
				if strings.Contains(msg, "already resigned") {
					return
				}
				t.Fatalf("resign validator tx failed: %v", errW)
			}
		}

		lastErr = err
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "epoch block forbidden") {
			waitBlocks(t, 1)
			continue
		}
		if strings.Contains(msg, "validator not registered") {
			if addr != (common.Address{}) {
				if exists, errV := ctx.Validators.IsValidatorExist(nil, addr); errV == nil && !exists {
					waitBlocks(t, 1)
					continue
				}
			}
			waitBlocks(t, 1)
			continue
		}
		if strings.Contains(msg, "already resigned") {
			return
		}
		t.Fatalf("resign validator call failed: %v", err)
	}

	t.Fatalf("resign validator retries exhausted: %v", lastErr)
}

func currentActiveValidatorKeys(t *testing.T) ([]*ecdsa.PrivateKey, []common.Address) {
	t.Helper()
	activeSet, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("read active validators failed: %v", err)
	}
	keys := make([]*ecdsa.PrivateKey, 0, len(activeSet))
	addrs := make([]common.Address, 0, len(activeSet))
	for _, addr := range activeSet {
		if key := ctx.ValidatorKeyByAddress(addr); key != nil {
			keys = append(keys, key)
			addrs = append(addrs, addr)
		}
	}
	if len(keys) == 0 {
		t.Fatalf("no local validator keys matched current active set: %v", activeSet)
	}
	return keys, addrs
}

func registerCandidateValidator(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	t.Helper()
	fundAmount := new(big.Int)
	fundAmount.SetString("2000000000000000000000", 10)
	key, addr, err := ctx.CreateAndFundAccount(fundAmount)
	if err != nil {
		t.Fatalf("create candidate account failed: %v", err)
	}

	activeKeys, activeAddrs := currentActiveValidatorKeys(t)
	proposer := activeKeys[0]
	ctx.WaitIfEpochBlock()
	opts, err := ctx.GetTransactor(proposer)
	if err != nil {
		t.Fatalf("get proposer transactor failed: %v", err)
	}
	proposalTx, err := ctx.Proposal.CreateProposal(opts, addr, true, "P-Join")
	if err != nil {
		t.Fatalf("propose validator failed: %v", err)
	}
	if err := ctx.WaitMined(proposalTx.Hash()); err != nil {
		t.Fatalf("proposal tx failed: %v", err)
	}
	pid := getProposalID(proposalTx)
	if pid == ([32]byte{}) {
		t.Fatalf("missing proposal id from logs")
	}
	voters := activeKeys
	if len(voters) > 1 {
		// proposer already participates in proposal lifecycle; avoid duplicate-vote reverts
		voters = voters[1:]
		activeAddrs = activeAddrs[1:]
	}
	if len(voters) == 0 {
		t.Fatalf("proposal %x has no remaining active validators to vote", pid)
	}
	for _, vk := range voters {
		robustVote(t, vk, pid)
	}

	registered := false
	for retry := 0; retry < 12; retry++ {
		ctx.WaitIfEpochBlock()
		regOpts, err := ctx.GetTransactor(key)
		if err != nil {
			time.Sleep(retrySleep())
			continue
		}
		regOpts.Value = testkit.RequireMinValidatorStake(t, func() (*big.Int, error) { return ctx.Proposal.MinValidatorStake(nil) })
		tx, err := ctx.Staking.RegisterValidator(regOpts, big.NewInt(1000))
		if err == nil {
			if err := ctx.WaitMined(tx.Hash()); err == nil {
				registered = true
				break
			}
		}
		time.Sleep(retrySleep())
	}
	if !registered {
		t.Fatalf("register candidate validator failed")
	}
	return key, addr
}

func requireBaselinePOSATopology(t *testing.T) []common.Address {
	t.Helper()
	activeSet, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil {
		t.Fatalf("read active validators failed: %v", err)
	}
	expected := len(ctx.GenesisValidators)
	if expected == 0 {
		t.Fatalf("missing genesis validators")
	}
	if len(activeSet) != expected {
		t.Skipf("requires baseline POSA topology with %d active validators, got %d", expected, len(activeSet))
	}
	return activeSet
}
