package tests

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"juchain.org/chain/tools/ci/internal/testkit"
)

func keyForAddress(addr common.Address) *ecdsa.PrivateKey {
	if ctx == nil {
		return nil
	}
	return ctx.ValidatorKeyByAddress(addr)
}

func currentEpochLength() int {
	if ctx != nil && ctx.Config != nil && ctx.Config.Network.Epoch > 0 {
		return int(ctx.Config.Network.Epoch)
	}
	return 30
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

	epoch := uint64(0)
	if ctx.Config != nil && ctx.Config.Network.Epoch > 0 {
		epoch = ctx.Config.Network.Epoch
	}
	if epoch == 0 {
		if epochBig, err := ctx.Proposal.Epoch(nil); err == nil && epochBig != nil && epochBig.Sign() > 0 {
			epoch = epochBig.Uint64()
		}
	}
	if epoch == 0 {
		epoch = 30
	}

	maxBlocks := int(epoch*uint64(maxEpochs) + 2)
	if maxBlocks < 4 {
		maxBlocks = 4
	}
	for i := 0; i < maxBlocks; i++ {
		owner, err := ctx.Validators.GetValidatorBySignerHistory(nil, signer)
		if err == nil && owner == validator {
			return true
		}
		waitBlocks(t, 1)
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
	epoch := currentEpochLength()
	maxBlocks := maxEpochs*epoch + 4
	for i := 0; i < maxBlocks; i++ {
		active, _ := ctx.Validators.IsValidatorActive(nil, addr)
		if active {
			return true
		}
		ctx.WaitIfEpochBlock()
		if err := ctx.WaitForBlockProgress(1, 45*time.Second); err != nil {
			return false
		}
	}
	return false
}

func waitForValidatorJailed(t *testing.T, addr common.Address, maxBlocks int) bool {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if maxBlocks < 1 {
		maxBlocks = 1
	}
	for i := 0; i < maxBlocks; i++ {
		info, err := ctx.Staking.GetValidatorInfo(nil, addr)
		if err == nil && info.IsJailed {
			return true
		}
		waitBlocks(t, 1)
	}
	return false
}

func waitForIncomingProfits(t *testing.T, valAddr common.Address, maxBlocks int) *big.Int {
	t.Helper()
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if maxBlocks < 1 {
		maxBlocks = 1
	}

	var incoming *big.Int
	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: maxBlocks + 1,
		Interval:    retrySleep(),
		OnRetry: func(int) {
			waitBlocks(t, 1)
		},
	}, func() (bool, error) {
		_, _, latestIncoming, _, _, err := ctx.Validators.GetValidatorInfo(nil, valAddr)
		if err != nil {
			return false, err
		}
		incoming = latestIncoming
		return latestIncoming != nil && latestIncoming.Sign() > 0, nil
	})
	if err != nil {
		t.Fatalf("validator %s did not accrue incoming profits in time: %v", valAddr.Hex(), err)
	}
	return new(big.Int).Set(incoming)
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

func requireChainProgressOrSkip(t *testing.T, minIncrements int, timeout time.Duration, reason string) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if err := ctx.WaitForBlockProgress(minIncrements, timeout); err != nil {
		height, readErr := ctx.Clients[0].BlockNumber(context.Background())
		if readErr != nil {
			t.Skipf("%s: chain stalled and current height is unavailable: %v (height err: %v)", reason, err, readErr)
		}
		t.Skipf("%s: chain stalled at height %d: %v", reason, height, err)
	}
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

func pickInTurnValidatorForNextBlock(t *testing.T) (*ecdsa.PrivateKey, common.Address, common.Address) {
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
		signerAddr, signerKey := signerIdentityForValidator(addr, keyForAddress(addr))
		if signerKey != nil {
			return signerKey, addr, signerAddr
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
	t.Fatalf("no signer key for active validator set; coinbaseSigner=%s currentValidator=%s active=%s known=%s", header.Coinbase.Hex(), currentValidator.Hex(), strings.Join(active, ","), strings.Join(known, ","))
	return nil, common.Address{}, common.Address{} // unreachable
}

func waitNextBlock() {
	waitBlocks(nil, 1)
}

func waitProposalCooldownFor(t *testing.T, proposer common.Address) {
	if ctx == nil {
		if t != nil {
			t.Fatalf("Context not initialized")
		}
		return
	}
	cooldown, err := ctx.Proposal.ProposalCooldown(nil)
	if err != nil || cooldown == nil || cooldown.Sign() <= 0 {
		waitBlocks(t, 1)
		return
	}
	lastBlock, err := ctx.Proposal.LastProposalBlock(nil, proposer)
	if err != nil || lastBlock == nil || lastBlock.Sign() == 0 {
		waitBlocks(t, 1)
		return
	}
	curHeight, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		waitBlocks(t, 1)
		return
	}
	target := new(big.Int).Add(lastBlock, cooldown)
	if !target.IsUint64() {
		waitBlocks(t, 1)
		return
	}
	targetHeight := target.Uint64()
	if curHeight < targetHeight {
		waitBlocks(t, int(targetHeight-curHeight))
	}
}

func waitWithdrawProfitCooldownFor(t *testing.T, validator common.Address) {
	if ctx == nil {
		if t != nil {
			t.Fatalf("Context not initialized")
		}
		return
	}
	period, err := ctx.Proposal.WithdrawProfitPeriod(nil)
	if err != nil || period == nil || period.Sign() <= 0 {
		waitBlocks(t, 1)
		return
	}
	_, _, _, _, lastWithdrawBlock, err := ctx.Validators.GetValidatorInfo(nil, validator)
	if err != nil || lastWithdrawBlock == nil || lastWithdrawBlock.Sign() == 0 {
		waitBlocks(t, 1)
		return
	}
	curHeight, err := ctx.Clients[0].BlockNumber(context.Background())
	if err != nil {
		waitBlocks(t, 1)
		return
	}
	target := new(big.Int).Add(lastWithdrawBlock, period)
	if !target.IsUint64() {
		waitBlocks(t, 1)
		return
	}
	targetHeight := target.Uint64()
	if curHeight < targetHeight {
		waitBlocks(t, int(targetHeight-curHeight))
	}
}
