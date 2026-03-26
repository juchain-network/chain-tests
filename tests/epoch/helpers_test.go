package tests

import (
	"context"
	"crypto/ecdsa"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func keyForAddress(addr common.Address) *ecdsa.PrivateKey {
	if ctx == nil {
		return nil
	}
	return ctx.ValidatorKeyByAddress(addr)
}

func signerIdentityForValidator(addr common.Address, fallbackKey *ecdsa.PrivateKey) (common.Address, *ecdsa.PrivateKey) {
	if ctx == nil {
		return common.Address{}, nil
	}

	signerAddr, err := ctx.SignerAddressByValidator(addr)
	if err != nil || signerAddr == (common.Address{}) {
		signerAddr = addr
	}

	if signerKey := ctx.SignerKeyByAddress(signerAddr); signerKey != nil {
		return signerAddr, signerKey
	}

	if fallbackKey != nil && crypto.PubkeyToAddress(fallbackKey.PublicKey) == signerAddr {
		return signerAddr, fallbackKey
	}

	return signerAddr, nil
}

func waitForSignerHistoricalOwner(t *testing.T, signer common.Address, validator common.Address, maxEpochs int) bool {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}
	for i := 0; i < maxEpochs; i++ {
		owner, err := ctx.Validators.GetValidatorBySignerHistory(nil, signer)
		if err == nil && owner == validator {
			return true
		}
		waitForNextEpochBlock(t)
	}
	return false
}

func minerKeyOrSkip(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	if ctx == nil || len(ctx.Clients) == 0 {
		t.Fatalf("Context not initialized")
	}

	var lastCoinbase common.Address
	var lastValidator common.Address
	for attempt := 0; attempt < 30; attempt++ {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil {
			t.Fatalf("failed to read header: %v", err)
		}
		coinbase := header.Coinbase
		lastCoinbase = coinbase
		if coinbase != (common.Address{}) {
			validator, err := ctx.ValidatorAddressBySigner(coinbase)
			if err == nil {
				lastValidator = validator
				if key := keyForAddress(validator); key != nil {
					return key, validator
				}
			}
		}
		waitBlocks(t, 1)
	}

	known := make([]string, 0, len(ctx.GenesisValidators))
	for _, k := range ctx.GenesisValidators {
		known = append(known, crypto.PubkeyToAddress(k.PublicKey).Hex())
	}
	t.Fatalf("no validator key matches current coinbase signer after 30 blocks; lastSigner=%s lastValidator=%s known=%s", lastCoinbase.Hex(), lastValidator.Hex(), strings.Join(known, ","))
	return nil, common.Address{} // unreachable
}

// waitForNextEpochBlock mines or waits until an epoch block has passed and state has settled.
// It also handles triggering updateActiveValidatorSet as the miner if we are at an epoch block.
func waitForNextEpochBlock(t *testing.T) uint64 {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	height, err := ctx.WaitForNextEpochTransition()
	if err != nil {
		t.Fatalf("wait for next epoch transition failed: %v", err)
	}
	return height
}

func waitForValidatorActive(t *testing.T, addr common.Address, maxEpochs int) bool {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}
	for i := 0; i < maxEpochs; i++ {
		active, _ := ctx.Validators.IsValidatorActive(nil, addr)
		if active {
			return true
		}
		waitForNextEpochBlock(t)
	}
	return false
}

func ensureMinActiveValidators(t *testing.T, min int, maxEpochs int) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if min < 1 {
		min = 1
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}
	for attempt := 0; attempt < maxEpochs; attempt++ {
		set, err := ctx.Validators.GetActiveValidators(nil)
		if err == nil && len(set) >= min {
			return
		}
		waitForNextEpochBlock(t)
	}
	set, _ := ctx.Validators.GetActiveValidators(nil)
	t.Fatalf("active validators < %d (got %d)", min, len(set))
}

func getActiveProposerOrSkip(t *testing.T, maxEpochs int) *ecdsa.PrivateKey {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}
	if maxEpochs < 1 {
		maxEpochs = 1
	}
	for attempt := 0; attempt < maxEpochs; attempt++ {
		for _, k := range ctx.GenesisValidators {
			addr := crypto.PubkeyToAddress(k.PublicKey)
			active, _ := ctx.Validators.IsValidatorActive(nil, addr)
			if !active {
				continue
			}
			info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
			if !info.IsJailed {
				return k
			}
		}
		waitForNextEpochBlock(t)
	}
	t.Fatalf("no active proposer available")
	return nil
}

func pickInTurnValidatorForNextBlock(t *testing.T) (*ecdsa.PrivateKey, common.Address) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	validators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil || len(validators) == 0 {
		t.Fatalf("no active validators available")
	}
	header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil || header == nil {
		t.Fatalf("failed to read header: %v", err)
	}
	currentValidator, err := ctx.ValidatorAddressBySigner(header.Coinbase)
	if err != nil {
		t.Fatalf("failed to map coinbase signer %s to validator: %v", header.Coinbase.Hex(), err)
	}
	start := 0
	for i, v := range validators {
		if v == currentValidator {
			start = (i + 1) % len(validators)
			break
		}
	}

	for offset := 0; offset < len(validators); offset++ {
		idx := (start + offset) % len(validators)
		addr := validators[idx]
		if key := keyForAddress(addr); key != nil {
			return key, addr
		}
	}

	active := make([]string, 0, len(validators))
	for _, v := range validators {
		active = append(active, v.Hex())
	}
	known := make([]string, 0, len(ctx.GenesisValidators))
	for _, k := range ctx.GenesisValidators {
		known = append(known, crypto.PubkeyToAddress(k.PublicKey).Hex())
	}
	t.Fatalf("no key for active validator set; coinbaseSigner=%s currentValidator=%s active=%s known=%s", header.Coinbase.Hex(), currentValidator.Hex(), strings.Join(active, ","), strings.Join(known, ","))
	return nil, common.Address{} // unreachable
}

func waitNextBlock() {
	waitBlocks(nil, 1)
}

func getNextProposerOrSkip(t *testing.T, pIndex *int) *ecdsa.PrivateKey {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}
	if pIndex == nil {
		zero := 0
		pIndex = &zero
	}
	for attempt := 0; attempt < 3; attempt++ {
		start := *pIndex
		for i := 0; i < len(ctx.GenesisValidators); i++ {
			idx := (start + i) % len(ctx.GenesisValidators)
			k := ctx.GenesisValidators[idx]
			*pIndex = idx + 1

			addr := crypto.PubkeyToAddress(k.PublicKey)
			active, _ := ctx.Validators.IsValidatorActive(nil, addr)
			if !active {
				continue
			}
			info, _ := ctx.Staking.GetValidatorInfo(nil, addr)
			if !info.IsJailed {
				return k
			}
		}
		waitForNextEpochBlock(t)
	}
	t.Fatalf("no active proposer available")
	return nil
}

func voteProposalToPass(t *testing.T, propID [32]byte, name string) {
	for attempt := 0; attempt < 8; attempt++ {
		for _, vk := range ctx.GenesisValidators {
			voterAddr := crypto.PubkeyToAddress(vk.PublicKey)
			active, _ := ctx.Validators.IsValidatorActive(nil, voterAddr)
			if !active {
				continue
			}
			info, _ := ctx.Staking.GetValidatorInfo(nil, voterAddr)
			if info.IsJailed {
				continue
			}
			robustVote(t, vk, propID, true)
			if res, err := ctx.Proposal.Results(nil, propID); err == nil && res.ResultExist {
				return
			}
		}
		if res, err := ctx.Proposal.Results(nil, propID); err == nil && res.ResultExist {
			return
		}
		if err := ctx.WaitForBlockProgress(1, 15*time.Second); err != nil {
			t.Skipf("%s skipped: chain stalled while voting (%v)", name, err)
		}
	}
	t.Fatalf("%s failed: proposal did not finalize", name)
}
