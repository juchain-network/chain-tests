package tests

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
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
	"juchain.org/chain/tools/ci/internal/utils"
)

var (
	ctx             *testctx.CIContext
	configPath      = flag.String("config", "../../data/test_config.yaml", "Path to generated test configuration file")
	proposerCounter int
)

type uint256 = uint64

func TestMain(m *testing.M) {
	// Parse flags
	flag.Parse()

	// Initialize logger
	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true)))

	// Load config
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Error("Failed to load config", "err", err)
		os.Exit(1)
	}
	if len(cfg.RPCs) == 0 {
		log.Error("No RPCs configured in test config", "config", *configPath)
		os.Exit(1)
	}
	if cfg.Funder.PrivateKey == "" || cfg.Funder.Address == "" {
		log.Error("Funder config missing address or private_key", "config", *configPath)
		os.Exit(1)
	}
	if funderKey, err := crypto.HexToECDSA(cfg.Funder.PrivateKey); err == nil {
		derived := crypto.PubkeyToAddress(funderKey.PublicKey).Hex()
		if !strings.EqualFold(derived, cfg.Funder.Address) {
			log.Error("Funder address does not match private_key", "derived", derived, "config", cfg.Funder.Address)
			os.Exit(1)
		}
	} else {
		log.Error("Invalid funder private_key", "err", err)
		os.Exit(1)
	}
	if len(cfg.Validators) == 0 {
		log.Error("No genesis validators configured in test config", "config", *configPath)
		os.Exit(1)
	}
	for i, v := range cfg.Validators {
		if v.Address == "" || v.PrivateKey == "" {
			log.Error("Validator config missing address or private_key", "index", i, "config", *configPath)
			os.Exit(1)
		}
		if key, err := crypto.HexToECDSA(v.PrivateKey); err == nil {
			derived := crypto.PubkeyToAddress(key.PublicKey).Hex()
			if !strings.EqualFold(derived, v.Address) {
				log.Error("Validator address does not match private_key", "index", i, "derived", derived, "config", v.Address)
				os.Exit(1)
			}
		} else {
			log.Error("Invalid validator private_key", "index", i, "err", err)
			os.Exit(1)
		}
	}
	if len(cfg.ValidatorRPCs) > 0 && len(cfg.ValidatorRPCs) < len(cfg.Validators) {
		log.Error("validator_rpcs length must cover validators list", "validator_rpcs", len(cfg.ValidatorRPCs), "validators", len(cfg.Validators))
		os.Exit(1)
	}

	// Init context
	c, err := testctx.NewCIContext(cfg)
	if err != nil {
		log.Error("Failed to init context", "err", err)
		os.Exit(1)
	}
	ctx = c

	os.Exit(m.Run())
}

func debugEnabled() bool {
	v := strings.ToLower(os.Getenv("JUCHAIN_TEST_DEBUG"))
	return v == "1" || v == "true" || v == "yes"
}

func retrySleep() time.Duration {
	if ctx != nil {
		return ctx.RetryPollInterval()
	}
	return 100 * time.Millisecond
}

func blockPollInterval() time.Duration {
	if ctx != nil {
		return ctx.BlockPollInterval()
	}
	return 100 * time.Millisecond
}

// Helpers

func waitBlocks(t *testing.T, n int) {
	if n <= 0 {
		return
	}
	start, _ := ctx.Clients[0].BlockNumber(context.Background())
	target := start + uint64(n)
	if debugEnabled() {
		fmt.Printf("DEBUG: Waiting for %d blocks (from %d to %d)...\n", n, start, target)
	}

	// Send dummy transactions to force block production if needed
	// (Some PoA networks only seal blocks when there are transactions)
	ticker := time.NewTicker(blockPollInterval())
	defer ticker.Stop()

	for {
		current, _ := ctx.Clients[0].BlockNumber(context.Background())
		if current >= target {
			break
		}

		// Optional: Send a small transfer from funder to itself to trigger block sealing.
		// Use pending nonce directly to avoid epoch-block waits.
		if ctx.FunderKey != nil {
			addr := crypto.PubkeyToAddress(ctx.FunderKey.PublicKey)
			nonce, err := ctx.Clients[0].PendingNonceAt(context.Background(), addr)
			if err == nil {
				gasPrice, err := ctx.Clients[0].SuggestGasPrice(context.Background())
				if err != nil {
					gasPrice = big.NewInt(1000000000) // 1 Gwei fallback
				}
				tx := types.NewTransaction(nonce, addr, big.NewInt(0), 21000, gasPrice, nil)
				if signedTx, err := types.SignTx(tx, types.NewEIP155Signer(ctx.ChainID), ctx.FunderKey); err == nil {
					_ = ctx.Clients[0].SendTransaction(context.Background(), signedTx)
				}
			}
		}

		select {
		case <-ticker.C:
			continue
		}
	}
}

func getPropID(tx *types.Transaction) [32]byte {
	var receipt *types.Receipt
	var err error
	for i := 0; i < 10; i++ {
		receipt, err = ctx.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
		if err == nil && receipt != nil {
			break
		}
		time.Sleep(retrySleep())
	}
	if receipt == nil {
		return [32]byte{}
	}

	for _, l := range receipt.Logs {
		if ev, err := ctx.Proposal.ParseLogCreateProposal(*l); err == nil {
			return ev.Id
		}
		if ev, err := ctx.Proposal.ParseLogCreateConfigProposal(*l); err == nil {
			return ev.Id
		}
	}
	return [32]byte{}
}

func robustVote(t *testing.T, voterKey *ecdsa.PrivateKey, propID [32]byte, auth bool) {
	var err error
	voterAddr := crypto.PubkeyToAddress(voterKey.PublicKey)
	for retry := 0; retry < 10; retry++ {
		// Only active, non-jailed validators can vote
		active, _ := ctx.Validators.IsValidatorActive(nil, voterAddr)
		if !active {
			return
		}
		info, _ := ctx.Staking.GetValidatorInfo(nil, voterAddr)
		if info.IsJailed {
			return
		}

		// Avoid epoch blocks which are forbidden for voting
		ctx.WaitIfEpochBlock()

		opts, errG := ctx.GetTransactor(voterKey)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}

		var txVote *types.Transaction
		txVote, err = ctx.Proposal.VoteProposal(opts, propID, auth)
		if err == nil {
			if errW := ctx.WaitMined(txVote.Hash()); errW == nil {
				return
			} else {
				if strings.Contains(errW.Error(), "Epoch block forbidden") {
					time.Sleep(retrySleep())
					continue
				}
				if strings.Contains(errW.Error(), "Validator only") || strings.Contains(errW.Error(), "Validator is jailed") {
					return
				}
				if t != nil {
					t.Logf("vote tx failed: %v", errW)
				}
				return
			}
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") {
			time.Sleep(retrySleep())
			continue
		}
		if strings.Contains(err.Error(), "Validator only") || strings.Contains(err.Error(), "Validator is jailed") {
			return
		}
		if strings.Contains(err.Error(), "Proposal already passed") {
			return
		}
		break
	}
}

func passProposalFor(t *testing.T, target common.Address, name string) error {
	var tx *types.Transaction
	var err error
	mined := false
	for retry := 0; retry < 15; retry++ {
		proposerIndex := proposerCounter % len(ctx.GenesisValidators)
		proposerCounter++
		proposerKey := ctx.GenesisValidators[proposerIndex]

		// Ensure proposer is active and not jailed
		proposerAddr := crypto.PubkeyToAddress(proposerKey.PublicKey)
		active, _ := ctx.Validators.IsValidatorActive(nil, proposerAddr)
		if !active {
			continue
		}
		info, _ := ctx.Staking.GetValidatorInfo(nil, proposerAddr)
		if info.IsJailed {
			continue
		}

		opts, errG := ctx.GetTransactor(proposerKey)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}

		tx, err = ctx.Proposal.CreateProposal(opts, target, true, name)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW != nil {
				if strings.Contains(errW.Error(), "timeout waiting for tx") {
					waitBlocks(t, 1)
					continue
				}
				return errW
			}
			mined = true
			break
		}
		if strings.Contains(err.Error(), "Proposal creation too frequent") {
			waitProposalCooldownFor(t, proposerAddr)
			continue
		}
		if strings.Contains(err.Error(), "nonce too low") {
			ctx.RefreshNonce(proposerAddr)
			waitNextBlock()
			continue
		}
		return err
	}
	if !mined || tx == nil {
		return fmt.Errorf("failed to create proposal for %s", target.Hex())
	}
	propID := [32]byte{}
	err = testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 4,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		propID = getPropID(tx)
		return propID != ([32]byte{}), nil
	})
	if err != nil {
		return fmt.Errorf("could not find proposal ID in logs for tx %s: %w", tx.Hash().Hex(), err)
	}

	for attempt := 0; attempt < 5; attempt++ {
		for _, voterKey := range ctx.GenesisValidators {
			voterAddr := crypto.PubkeyToAddress(voterKey.PublicKey)
			active, _ := ctx.Validators.IsValidatorActive(nil, voterAddr)
			if !active {
				continue
			}
			info, _ := ctx.Staking.GetValidatorInfo(nil, voterAddr)
			if info.IsJailed {
				continue
			}

			if passed, errPass := ctx.Proposal.Pass(nil, target); errPass == nil && passed {
				return nil
			}

			robustVote(t, voterKey, propID, true)

			if passed, errPass := ctx.Proposal.Pass(nil, target); errPass == nil && passed {
				return nil
			}
		}
		err = testkit.WaitUntil(testkit.WaitUntilOptions{
			MaxAttempts: 2,
			Interval:    retrySleep(),
			OnRetry: func(int) {
				waitBlocks(t, 1)
			},
		}, func() (bool, error) {
			pass, err := ctx.Proposal.Pass(nil, target)
			if err != nil {
				return false, err
			}
			return pass, nil
		})
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("proposal did not pass for %s", target.Hex())
}

func createAndRegisterValidator(t *testing.T, name string) (*ecdsa.PrivateKey, common.Address, error) {
	key, addr, err := ctx.CreateAndFundAccount(utils.ToWei(500005))
	if err != nil {
		return nil, addr, err
	}

	isRegistered := func() bool {
		info, errInfo := ctx.Staking.GetValidatorInfo(nil, addr)
		return errInfo == nil && info.IsRegistered
	}
	ensureProposalPassed := func() error {
		var lastProposalErr error
		for attempt := 0; attempt < 3; attempt++ {
			pass, errPass := ctx.Proposal.Pass(nil, addr)
			if errPass == nil && pass {
				return nil
			}
			errProp := passProposalFor(t, addr, name)
			if errProp == nil {
				return nil
			}
			lastProposalErr = errProp
			if strings.Contains(errProp.Error(), "Proposal expired") ||
				strings.Contains(errProp.Error(), "proposal did not pass") ||
				strings.Contains(errProp.Error(), "condition not met") {
				waitBlocks(t, 1)
				continue
			}
			return errProp
		}
		if lastProposalErr != nil {
			return lastProposalErr
		}
		return fmt.Errorf("failed to pass proposal for %s", addr.Hex())
	}

	if err = ensureProposalPassed(); err != nil {
		return nil, addr, err
	}

	var lastErr error
	for retry := 0; retry < 15; retry++ {
		if isRegistered() {
			return key, addr, nil
		}

		opts, errG := ctx.GetTransactor(key)
		if errG != nil {
			lastErr = errG
			time.Sleep(retrySleep())
			continue
		}
		opts.Value = utils.ToWei(100000)

		txReg, err := ctx.Staking.RegisterValidator(opts, big.NewInt(1000))
		if err == nil {
			errW := ctx.WaitMined(txReg.Hash())
			if errW == nil {
				return key, addr, nil
			}
			lastErr = errW
			// Register tx may revert transiently around epoch/set-update windows.
			if strings.Contains(errW.Error(), "Proposal expired") {
				if errP := ensureProposalPassed(); errP != nil {
					lastErr = errP
				}
				waitBlocks(t, 1)
				continue
			}
			if strings.Contains(errW.Error(), "Epoch block forbidden") ||
				strings.Contains(errW.Error(), "Too many new validators") ||
				strings.Contains(strings.ToLower(errW.Error()), "reverted") {
				if isRegistered() {
					return key, addr, nil
				}
				if errP := ensureProposalPassed(); errP != nil {
					lastErr = errP
				}
				waitForNextEpochBlock(t)
				continue
			}
			return nil, addr, errW
		}

		lastErr = err
		if strings.Contains(err.Error(), "Proposal expired") {
			if errP := ensureProposalPassed(); errP != nil {
				lastErr = errP
			}
			waitBlocks(t, 1)
			continue
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") ||
			strings.Contains(err.Error(), "Too many new validators") ||
			strings.Contains(err.Error(), "Must pass proposal first") {
			if errP := ensureProposalPassed(); errP != nil {
				lastErr = errP
			}
			waitForNextEpochBlock(t)
			continue
		}
		break
	}

	if isRegistered() {
		return key, addr, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to register validator %s", addr.Hex())
	}
	return nil, addr, lastErr
}

// Robust Staking Helpers

func robustDelegate(t *testing.T, key *ecdsa.PrivateKey, val common.Address, amount *big.Int) {
	testkit.RobustDelegate(t, key, val, amount, testkit.DelegateOps{
		GetTransactor: ctx.GetTransactor,
		Delegate:      ctx.Staking.Delegate,
		WaitMined:     ctx.WaitMined,
	})
}

func robustUndelegate(t *testing.T, key *ecdsa.PrivateKey, val common.Address, amount *big.Int) {
	var lastErr error
	for retry := 0; retry < 10; retry++ {
		opts, errG := ctx.GetTransactor(key)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}
		tx, err := ctx.Staking.Undelegate(opts, val, amount)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				if strings.Contains(errW.Error(), "Epoch block forbidden") {
					time.Sleep(retrySleep())
					continue
				}
				lastErr = errW
				time.Sleep(retrySleep())
				continue
			}
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") {
			time.Sleep(retrySleep())
			continue
		}
		if t != nil {
			t.Fatalf("undelegate call failed: %v", err)
		} else {
			return
		}
	}
	if t != nil && lastErr != nil {
		t.Fatalf("undelegate tx failed: %v", lastErr)
	}
}

func robustClaimRewards(t *testing.T, key *ecdsa.PrivateKey, val common.Address) {
	for retry := 0; retry < 10; retry++ {
		opts, errG := ctx.GetTransactor(key)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}
		tx, err := ctx.Staking.ClaimRewards(opts, val)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				if strings.Contains(errW.Error(), "Epoch block forbidden") {
					time.Sleep(retrySleep())
					continue
				}
				if t != nil {
					t.Fatalf("claimRewards tx failed: %v", errW)
				} else {
					return
				}
			}
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") {
			time.Sleep(retrySleep())
			continue
		}
		if t != nil {
			t.Fatalf("claimRewards call failed: %v", err)
		} else {
			return
		}
	}
}

func robustWithdrawUnbonded(t *testing.T, key *ecdsa.PrivateKey, val common.Address, maxEntries int64) {
	for retry := 0; retry < 10; retry++ {
		opts, errG := ctx.GetTransactor(key)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}
		tx, err := ctx.Staking.WithdrawUnbonded(opts, val, big.NewInt(maxEntries))
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				if strings.Contains(errW.Error(), "Epoch block forbidden") {
					time.Sleep(retrySleep())
					continue
				}
				if t != nil {
					t.Fatalf("withdrawUnbonded tx failed: %v", errW)
				} else {
					return
				}
			}
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") {
			time.Sleep(retrySleep())
			continue
		}
		if t != nil {
			t.Fatalf("withdrawUnbonded call failed: %v", err)
		} else {
			return
		}
	}
}

func robustExitValidator(t *testing.T, key *ecdsa.PrivateKey) {
	for retry := 0; retry < 10; retry++ {
		opts, errG := ctx.GetTransactor(key)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}
		tx, err := ctx.Staking.ExitValidator(opts)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				if strings.Contains(errW.Error(), "Epoch block forbidden") {
					time.Sleep(retrySleep())
					continue
				}
				if t != nil {
					t.Fatalf("exitValidator tx failed: %v", errW)
				} else {
					return
				}
			}
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") {
			time.Sleep(retrySleep())
			continue
		}
		if strings.Contains(err.Error(), "active set") || strings.Contains(err.Error(), "wait until next epoch") {
			waitForNextEpochBlock(t)
			continue
		}
		if t != nil {
			t.Fatalf("exitValidator call failed: %v", err)
		} else {
			return
		}
	}
}

func robustClaimValidatorRewards(t *testing.T, key *ecdsa.PrivateKey) {
	for retry := 0; retry < 10; retry++ {
		opts, errG := ctx.GetTransactor(key)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}
		tx, err := ctx.Staking.ClaimValidatorRewards(opts)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				if strings.Contains(errW.Error(), "Epoch block forbidden") {
					time.Sleep(retrySleep())
					continue
				}
				if t != nil {
					t.Fatalf("claimValidatorRewards tx failed: %v", errW)
				} else {
					return
				}
			}
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") {
			time.Sleep(retrySleep())
			continue
		}
		if t != nil {
			t.Fatalf("claimValidatorRewards call failed: %v", err)
		} else {
			return
		}
	}
}

func robustUnjailValidator(t *testing.T, key *ecdsa.PrivateKey, addr common.Address) {
	for retry := 0; retry < 10; retry++ {
		opts, errG := ctx.GetTransactor(key)
		if errG != nil {
			time.Sleep(retrySleep())
			continue
		}
		tx, err := ctx.Staking.UnjailValidator(opts, addr)
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				return
			} else {
				if strings.Contains(errW.Error(), "Epoch block forbidden") || strings.Contains(errW.Error(), "Too many new validators") {
					waitForNextEpochBlock(t)
					continue
				}
				if t != nil {
					t.Fatalf("unjail tx failed: %v", errW)
				}
				return
			}
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") || strings.Contains(err.Error(), "Too many new validators") {
			waitForNextEpochBlock(t)
			continue
		}
		if t != nil {
			t.Fatalf("unjail call failed: %v", err)
		} else {
			return
		}
	}
}
