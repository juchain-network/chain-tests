package tests

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/testkit"
	"juchain.org/chain/tools/ci/internal/utils"
)

type explicitSignerCandidate struct {
	ValidatorKey *ecdsa.PrivateKey
	Validator    common.Address
	SignerKey    *ecdsa.PrivateKey
	Signer       common.Address
}

func createAndRegisterValidatorWithExplicitSigner(t *testing.T, name string) (*explicitSignerCandidate, error) {
	t.Helper()

	validatorKey, validatorAddr, err := ctx.CreateAndFundAccount(utils.ToWei(500005))
	if err != nil {
		return nil, err
	}
	signerKey, signerAddr, err := ctx.CreateAndFundAccount(utils.ToWei(10))
	if err != nil {
		return nil, err
	}

	ensureProposalPassed := func() error {
		var lastProposalErr error
		for attempt := 0; attempt < 3; attempt++ {
			pass, errPass := ctx.Proposal.Pass(nil, validatorAddr)
			if errPass == nil && pass {
				return nil
			}
			errProp := passProposalFor(t, validatorAddr, name)
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
		return fmt.Errorf("failed to pass proposal for %s", validatorAddr.Hex())
	}

	if err := ensureProposalPassed(); err != nil {
		return nil, err
	}

	var lastErr error
	for retry := 0; retry < 10; retry++ {
		opts, errG := ctx.GetTransactor(validatorKey)
		if errG != nil {
			lastErr = errG
			time.Sleep(retrySleep())
			continue
		}
		opts.GasLimit = 1_500_000
		tx, err := ctx.Validators.CreateOrEditValidator(opts, validatorAddr, signerAddr, name, "", "", "", "")
		if err == nil {
			if errW := ctx.WaitMined(tx.Hash()); errW == nil {
				lastErr = nil
				break
			} else {
				lastErr = errW
				if strings.Contains(errW.Error(), "nonce too low") {
					ctx.RefreshNonce(validatorAddr)
					waitBlocks(t, 1)
					continue
				}
				if strings.Contains(errW.Error(), "Epoch block forbidden") ||
					strings.Contains(strings.ToLower(errW.Error()), "reverted") {
					waitBlocks(t, 1)
					continue
				}
				return nil, errW
			}
		}

		lastErr = err
		if strings.Contains(err.Error(), "nonce too low") {
			ctx.RefreshNonce(validatorAddr)
			waitBlocks(t, 1)
			continue
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") {
			waitBlocks(t, 1)
			continue
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, lastErr
	}

	minStake := testkit.RequireMinValidatorStake(t, func() (*big.Int, error) { return ctx.Proposal.MinValidatorStake(nil) })
	var registerErr error
	for retry := 0; retry < 15; retry++ {
		info, errInfo := ctx.Staking.GetValidatorInfo(nil, validatorAddr)
		if errInfo == nil && info.IsRegistered {
			return &explicitSignerCandidate{
				ValidatorKey: validatorKey,
				Validator:    validatorAddr,
				SignerKey:    signerKey,
				Signer:       signerAddr,
			}, nil
		}

		opts, errG := ctx.GetTransactor(validatorKey)
		if errG != nil {
			registerErr = errG
			time.Sleep(retrySleep())
			continue
		}
		opts.Value = new(big.Int).Set(minStake)

		txReg, err := ctx.Staking.RegisterValidator(opts, big.NewInt(1000))
		if err == nil {
			errW := ctx.WaitMined(txReg.Hash())
			if errW == nil {
				return &explicitSignerCandidate{
					ValidatorKey: validatorKey,
					Validator:    validatorAddr,
					SignerKey:    signerKey,
					Signer:       signerAddr,
				}, nil
			}
			registerErr = errW
			if strings.Contains(errW.Error(), "Proposal expired") {
				if errP := ensureProposalPassed(); errP != nil {
					registerErr = errP
				}
				waitBlocks(t, 1)
				continue
			}
			if strings.Contains(errW.Error(), "Epoch block forbidden") ||
				strings.Contains(errW.Error(), "Too many new validators") ||
				strings.Contains(strings.ToLower(errW.Error()), "reverted") {
				if errP := ensureProposalPassed(); errP != nil {
					registerErr = errP
				}
				waitBlocks(t, 1)
				continue
			}
			return nil, errW
		}

		registerErr = err
		if strings.Contains(err.Error(), "Proposal expired") {
			if errP := ensureProposalPassed(); errP != nil {
				registerErr = errP
			}
			waitBlocks(t, 1)
			continue
		}
		if strings.Contains(err.Error(), "Epoch block forbidden") ||
			strings.Contains(err.Error(), "Too many new validators") ||
			strings.Contains(err.Error(), "Must pass proposal first") {
			if errP := ensureProposalPassed(); errP != nil {
				registerErr = errP
			}
			waitBlocks(t, 1)
			continue
		}
		break
	}

	if registerErr == nil {
		registerErr = fmt.Errorf("failed to register validator %s with explicit signer", validatorAddr.Hex())
	}
	return nil, registerErr
}
