package context

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"

	"juchain.org/chain/tools/ci/contracts"
	"juchain.org/chain/tools/ci/internal/config"
)

// Addresses of system contracts
var (
	ValidatorsAddr = common.HexToAddress("0x000000000000000000000000000000000000f010")
	PunishAddr     = common.HexToAddress("0x000000000000000000000000000000000000F011")
	ProposalAddr   = common.HexToAddress("0x000000000000000000000000000000000000F012")
	StakingAddr    = common.HexToAddress("0x000000000000000000000000000000000000F013")
)

var deterministicKeyBase = new(big.Int).Lsh(big.NewInt(1), 252)

func debugEnabled() bool {
	v := strings.ToLower(os.Getenv("JUCHAIN_TEST_DEBUG"))
	return v == "1" || v == "true" || v == "yes"
}

func (c *CIContext) configuredEpoch() uint64 {
	if c != nil && c.Config != nil && c.Config.Network.Epoch > 0 {
		return c.Config.Network.Epoch
	}
	return 30
}

const (
	defaultProfileName       = "fast"
	defaultRetryPollInterval = 100 * time.Millisecond
	defaultBlockPollInterval = 100 * time.Millisecond
	defaultEpochWaitTimeout  = 45 * time.Second
	defaultEpochStallTimeout = 15 * time.Second
	minPollInterval          = 20 * time.Millisecond
	maxPollInterval          = 2 * time.Second
)

type testParams struct {
	ProposalCooldown   int64
	UnbondingPeriod    int64
	ValidatorUnjail    int64
	WithdrawProfit     int64
	CommissionCooldown int64
	ProposalLasting    int64
}

func profileDefaults(profile string) testParams {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "", "fast":
		return testParams{
			ProposalCooldown:   1,
			UnbondingPeriod:    3,
			ValidatorUnjail:    3,
			WithdrawProfit:     2,
			CommissionCooldown: 1,
			ProposalLasting:    30,
		}
	case "default":
		return testParams{
			ProposalCooldown:   1,
			UnbondingPeriod:    10,
			ValidatorUnjail:    10,
			WithdrawProfit:     5,
			CommissionCooldown: 5,
			ProposalLasting:    100,
		}
	case "edge":
		return testParams{
			ProposalCooldown:   60,
			UnbondingPeriod:    60,
			ValidatorUnjail:    60,
			WithdrawProfit:     30,
			CommissionCooldown: 30,
			ProposalLasting:    300,
		}
	default:
		return profileDefaults(defaultProfileName)
	}
}

func (c *CIContext) configuredTestParams() testParams {
	if c == nil || c.Config == nil {
		return profileDefaults(defaultProfileName)
	}

	params := profileDefaults(c.Config.Test.Profile)
	overrides := c.Config.Test.Params
	if overrides.ProposalCooldown > 0 {
		params.ProposalCooldown = overrides.ProposalCooldown
	}
	if overrides.UnbondingPeriod > 0 {
		params.UnbondingPeriod = overrides.UnbondingPeriod
	}
	if overrides.ValidatorUnjail > 0 {
		params.ValidatorUnjail = overrides.ValidatorUnjail
	}
	if overrides.WithdrawProfit > 0 {
		params.WithdrawProfit = overrides.WithdrawProfit
	}
	if overrides.CommissionCooldown > 0 {
		params.CommissionCooldown = overrides.CommissionCooldown
	}
	if overrides.ProposalLasting > 0 {
		params.ProposalLasting = overrides.ProposalLasting
	}
	return params
}

func normalizePollInterval(ms int64, fallback time.Duration) time.Duration {
	interval := fallback
	if ms > 0 {
		interval = time.Duration(ms) * time.Millisecond
	}
	if interval < minPollInterval {
		return minPollInterval
	}
	if interval > maxPollInterval {
		return maxPollInterval
	}
	return interval
}

func (c *CIContext) RetryPollInterval() time.Duration {
	if c == nil || c.Config == nil {
		return defaultRetryPollInterval
	}
	return normalizePollInterval(c.Config.Test.Timing.RetryPollMS, defaultRetryPollInterval)
}

func (c *CIContext) BlockPollInterval() time.Duration {
	if c == nil || c.Config == nil {
		return defaultBlockPollInterval
	}
	timing := c.Config.Test.Timing
	if timing.BlockPollMS > 0 {
		return normalizePollInterval(timing.BlockPollMS, defaultBlockPollInterval)
	}
	return normalizePollInterval(timing.RetryPollMS, defaultBlockPollInterval)
}

type CIContext struct {
	Config  *config.Config
	Clients []*ethclient.Client
	ChainID *big.Int

	// System Contracts
	Validators        *contracts.Validators
	Punish            *contracts.Punish
	Proposal          *contracts.Proposal
	Staking           *contracts.Staking
	ProposalAddr      common.Address
	FunderKey         *ecdsa.PrivateKey
	GenesisValidators []*ecdsa.PrivateKey
	GenesisSigners    []*ecdsa.PrivateKey

	validatorKeysByAddress map[common.Address]*ecdsa.PrivateKey
	signerKeysByAddress    map[common.Address]*ecdsa.PrivateKey
	validatorRPCByAddress  map[common.Address]string

	mu            sync.Mutex
	nonces        map[common.Address]uint64
	clientIndex   int
	proposerIndex int
	accountIndex  uint64
}

func NewCIContext(cfg *config.Config) (*CIContext, error) {
	if len(cfg.RPCs) == 0 {
		return nil, fmt.Errorf("no rpcs provided")
	}

	var clients []*ethclient.Client
	for _, url := range cfg.RPCs {
		client, err := ethclient.Dial(url)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %w", url, err)
		}
		clients = append(clients, client)
	}

	primaryClient := clients[0]
	chainID, err := primaryClient.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain id: %w", err)
	}

	val, err := contracts.NewValidators(ValidatorsAddr, primaryClient)
	if err != nil {
		return nil, err
	}
	pun, err := contracts.NewPunish(PunishAddr, primaryClient)
	if err != nil {
		return nil, err
	}
	prop, err := contracts.NewProposal(ProposalAddr, primaryClient)
	if err != nil {
		return nil, err
	}

	stk, err := contracts.NewStaking(StakingAddr, primaryClient)
	if err != nil {
		return nil, err
	}

	funderKey, err := crypto.HexToECDSA(cfg.Funder.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid funder private key: %w", err)
	}

	var genesisValidators []*ecdsa.PrivateKey
	var genesisSigners []*ecdsa.PrivateKey
	validatorKeysByAddress := make(map[common.Address]*ecdsa.PrivateKey)
	signerKeysByAddress := make(map[common.Address]*ecdsa.PrivateKey)
	validatorRPCByAddress := make(map[common.Address]string)
	for i, v := range cfg.Validators {
		key, err := crypto.HexToECDSA(v.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid validator private key at index %d: %w", i, err)
		}
		addr := crypto.PubkeyToAddress(key.PublicKey)
		validatorKeysByAddress[addr] = key
		genesisValidators = append(genesisValidators, key)

		signerKey := key
		if strings.TrimSpace(v.SignerPrivateKey) != "" {
			signerKey, err = crypto.HexToECDSA(v.SignerPrivateKey)
			if err != nil {
				return nil, fmt.Errorf("invalid signer private key at index %d: %w", i, err)
			}
		}
		genesisSigners = append(genesisSigners, signerKey)
		signerAddr := crypto.PubkeyToAddress(signerKey.PublicKey)
		signerKeysByAddress[signerAddr] = signerKey
		if i < len(cfg.ValidatorRPCs) && strings.TrimSpace(cfg.ValidatorRPCs[i]) != "" {
			validatorRPCByAddress[addr] = strings.TrimSpace(cfg.ValidatorRPCs[i])
		}
	}

	c := &CIContext{
		Config:                 cfg,
		Clients:                clients,
		ChainID:                chainID,
		Validators:             val,
		Punish:                 pun,
		Proposal:               prop,
		Staking:                stk,
		ProposalAddr:           ProposalAddr,
		FunderKey:              funderKey,
		GenesisValidators:      genesisValidators,
		GenesisSigners:         genesisSigners,
		validatorKeysByAddress: validatorKeysByAddress,
		signerKeysByAddress:    signerKeysByAddress,
		validatorRPCByAddress:  validatorRPCByAddress,
		nonces:                 make(map[common.Address]uint64),
	}

	// Prime the network with a dummy tx and use a lightweight readiness probe.
	// A single confirmed height increment is enough to prove the chain is alive.
	c.sendDummyTx()
	if err := c.WaitForBlockProgress(1, 90*time.Second); err != nil {
		// One short retry avoids transient native startup stalls.
		c.sendDummyTx()
		if errRetry := c.WaitForBlockProgress(1, 45*time.Second); errRetry != nil {
			return nil, fmt.Errorf("chain not producing blocks: %w", err)
		}
	}

	// Auto-Initialize only when the selected fork profile expects POSA system contracts.
	if c.shouldAutoInitialize() {
		if err := c.autoInitialize(); err != nil {
			return nil, fmt.Errorf("autoInitialize failed: %w", err)
		}
	} else {
		fmt.Printf("ℹ️ skip auto-initialization for fork mode=%s target=%s\n", cfg.Fork.Mode, cfg.Fork.Target)
	}
	// Keep generated test accounts unique across sequential runs that share one chain.
	c.seedAccountIndexFromFunderNonce()

	return c, nil
}

func (c *CIContext) shouldAutoInitialize() bool {
	if c == nil || c.Config == nil {
		return true
	}

	mode := strings.ToLower(strings.TrimSpace(c.Config.Fork.Mode))
	target := strings.ToLower(strings.TrimSpace(c.Config.Fork.Target))

	switch mode {
	case "poa", "upgrade":
		// POA address space does not expose POSA proposal config ids used by autoInitialize().
		return false
	case "smoke":
		// Static smoke matrix: only *_posa profile should use POSA auto-init path.
		return strings.Contains(target, "posa")
	case "posa":
		return true
	default:
		// Keep backward compatibility for legacy configs that omit fork.mode.
		return true
	}
}

func (c *CIContext) WaitForBlockProgress(minIncrements int, timeout time.Duration) error {
	if minIncrements <= 0 {
		return nil
	}
	start := time.Now()
	var last uint64
	increments := 0
	lastPoke := time.Now()

	for time.Since(start) < timeout {
		cur, err := c.Clients[0].BlockNumber(context.Background())
		if err == nil {
			if last != 0 && cur > last {
				increments++
				if increments >= minIncrements {
					return nil
				}
			}
			last = cur
		}
		if time.Since(lastPoke) > 5*time.Second {
			c.sendDummyTx()
			lastPoke = time.Now()
		}
		time.Sleep(c.BlockPollInterval())
	}
	return fmt.Errorf("block height did not progress (min increments=%d)", minIncrements)
}

func (c *CIContext) sendDummyTx() {
	if c.FunderKey == nil {
		return
	}
	addr := crypto.PubkeyToAddress(c.FunderKey.PublicKey)
	nonce, err := c.Clients[0].PendingNonceAt(context.Background(), addr)
	if err != nil {
		return
	}
	gasPrice, err := c.Clients[0].SuggestGasPrice(context.Background())
	if err != nil {
		gasPrice = big.NewInt(1000000000)
	}
	tx := types.NewTransaction(nonce, addr, big.NewInt(0), 21000, gasPrice, nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(c.ChainID), c.FunderKey)
	if err != nil {
		return
	}
	if err := c.Clients[0].SendTransaction(context.Background(), signedTx); err != nil {
		return
	}
	c.mu.Lock()
	if cur, ok := c.nonces[addr]; !ok || nonce+1 > cur {
		c.nonces[addr] = nonce + 1
	}
	c.mu.Unlock()
}

func (c *CIContext) GetConfigValue(cid int64) (*big.Int, error) {
	switch cid {
	case 0:
		return c.Proposal.ProposalLastingPeriod(nil)
	case 1:
		return c.Proposal.PunishThreshold(nil)
	case 2:
		return c.Proposal.RemoveThreshold(nil)
	case 3:
		return c.Proposal.DecreaseRate(nil)
	case 4:
		return c.Proposal.WithdrawProfitPeriod(nil)
	case 5:
		return c.Proposal.BlockReward(nil)
	case 6:
		return c.Proposal.UnbondingPeriod(nil)
	case 7:
		return c.Proposal.ValidatorUnjailPeriod(nil)
	case 8:
		return c.Proposal.MinValidatorStake(nil)
	case 9:
		return c.Proposal.MaxValidators(nil)
	case 10:
		return c.Proposal.MinDelegation(nil)
	case 11:
		return c.Proposal.MinUndelegation(nil)
	case 12:
		return c.Proposal.DoubleSignSlashAmount(nil)
	case 13:
		return c.Proposal.DoubleSignRewardAmount(nil)
	case 14:
		addr, err := c.Proposal.BurnAddress(nil)
		if err != nil {
			return nil, err
		}
		return new(big.Int).SetBytes(addr.Bytes()), nil
	case 15:
		return c.Proposal.DoubleSignWindow(nil)
	case 16:
		return c.Proposal.CommissionUpdateCooldown(nil)
	case 17:
		return c.Proposal.BaseRewardRatio(nil)
	case 18:
		return c.Proposal.MaxCommissionRate(nil)
	case 19:
		return c.Proposal.ProposalCooldown(nil)
	default:
		return nil, fmt.Errorf("unknown config id %d", cid)
	}
}

func (c *CIContext) autoInitialize() error {
	if len(c.GenesisValidators) == 0 {
		return fmt.Errorf("no genesis validators configured")
	}

	pickValidator := func(idx int) *ecdsa.PrivateKey {
		if idx < len(c.GenesisValidators) {
			return c.GenesisValidators[idx]
		}
		return c.GenesisValidators[len(c.GenesisValidators)-1]
	}

	// Robust check: if MinValidatorStake is default (100k JU), we need setup.
	minStake, err := c.Proposal.MinValidatorStake(nil)
	if err == nil && minStake.Cmp(big.NewInt(1000000000000000000)) == 0 {
		fmt.Printf("ℹ️  System already configured (MinValidatorStake = 1 JU).\n")
	}

	// Check if we need to call initialize() at all
	initialized, _ := c.Proposal.Initialized(nil)
	if !initialized {
		fmt.Printf("🔧 System unconfigured, performing auto-initialization...\n")
		var valAddrs []common.Address
		var signerAddrs []common.Address
		for _, vk := range c.GenesisValidators {
			valAddrs = append(valAddrs, crypto.PubkeyToAddress(vk.PublicKey))
		}
		for _, sk := range c.GenesisSigners {
			signerAddrs = append(signerAddrs, crypto.PubkeyToAddress(sk.PublicKey))
		}
		if len(signerAddrs) != len(valAddrs) {
			return fmt.Errorf("genesis signer count mismatch: validators=%d signers=%d", len(valAddrs), len(signerAddrs))
		}

		// 1. Initialize Proposal
		opts, _ := c.GetTransactor(pickValidator(0))
		opts.GasLimit = 1000000
		fmt.Printf("  > Initializing Proposal...\n")
		tx, err := c.Proposal.Initialize(opts, valAddrs, ValidatorsAddr, new(big.Int).SetUint64(c.configuredEpoch()))
		if err == nil {
			c.WaitMined(tx.Hash())
		}

		// 2. Initialize Staking with Validators
		opts, _ = c.GetTransactor(pickValidator(1))
		opts.GasLimit = 2000000
		fmt.Printf("  > Initializing Staking with Validators...\n")
		tx, err = c.Staking.InitializeWithValidators(opts, ValidatorsAddr, ProposalAddr, PunishAddr, valAddrs, big.NewInt(1000))
		if err == nil {
			c.WaitMined(tx.Hash())
		}

		// 3. Initialize Validators
		opts, _ = c.GetTransactor(pickValidator(2))
		opts.GasLimit = 1000000
		fmt.Printf("  > Initializing Validators...\n")
		tx, err = c.Validators.Initialize(opts, valAddrs, signerAddrs, ProposalAddr, PunishAddr, StakingAddr)
		if err == nil {
			c.WaitMined(tx.Hash())
		}
	}

	// 4. Always ensure test-friendly parameters if they are not set
	fmt.Printf("  > Configuring system parameters...\n")
	params := c.configuredTestParams()

	type configTarget struct {
		cid    int64
		name   string
		target *big.Int
	}
	targets := []configTarget{
		{cid: 19, name: "ProposalCooldown", target: big.NewInt(params.ProposalCooldown)},
		{cid: 6, name: "UnbondingPeriod", target: big.NewInt(params.UnbondingPeriod)},
		{cid: 7, name: "ValidatorUnjailPeriod", target: big.NewInt(params.ValidatorUnjail)},
		{cid: 4, name: "WithdrawProfitPeriod", target: big.NewInt(params.WithdrawProfit)},
		{cid: 8, name: "MinValidatorStake", target: big.NewInt(1000000000000000000)},
		{cid: 10, name: "MinDelegation", target: big.NewInt(1000000000000000000)},
		{cid: 16, name: "CommissionUpdateCooldown", target: big.NewInt(params.CommissionCooldown)},
		{cid: 0, name: "ProposalLastingPeriod", target: big.NewInt(params.ProposalLasting)},
	}

	// First ensure pass.
	for _, item := range targets {
		current, err := c.GetConfigValue(item.cid)
		if err != nil {
			log.Warn("Read config failed before ensure", "cid", item.cid, "name", item.name, "err", err)
			continue
		}
		if err := c.EnsureConfig(item.cid, item.target, current); err != nil {
			log.Warn("Ensure config failed", "cid", item.cid, "name", item.name, "target", item.target, "err", err)
		}
	}

	// Reconcile pass: occasionally one proposal path can race an epoch boundary and
	// the value remains stale even though the call path looked successful.
	for _, item := range targets {
		current, err := c.GetConfigValue(item.cid)
		if err != nil || current == nil {
			log.Warn("Read config failed during reconcile", "cid", item.cid, "name", item.name, "err", err)
			continue
		}
		if current.Cmp(item.target) != 0 {
			if err := c.EnsureConfig(item.cid, item.target, current); err != nil {
				log.Warn("Reconcile config failed", "cid", item.cid, "name", item.name, "target", item.target, "current", current, "err", err)
			}
		}
	}

	var unresolved []string
	for _, item := range targets {
		current, err := c.GetConfigValue(item.cid)
		if err != nil || current == nil {
			unresolved = append(unresolved, fmt.Sprintf("%s(cid=%d):read_error", item.name, item.cid))
			continue
		}
		if current.Cmp(item.target) != 0 {
			unresolved = append(unresolved, fmt.Sprintf("%s(cid=%d):want=%s got=%s", item.name, item.cid, item.target.String(), current.String()))
		}
	}

	fmt.Printf("ℹ️  Auto-initialization complete.\n")
	if len(unresolved) > 0 {
		return fmt.Errorf("auto-initialize unresolved configs: %s", strings.Join(unresolved, "; "))
	}
	return nil
}

func (c *CIContext) ValidatorKeyByAddress(addr common.Address) *ecdsa.PrivateKey {
	if c == nil {
		return nil
	}
	if key := c.validatorKeysByAddress[addr]; key != nil {
		return key
	}
	return c.signerKeysByAddress[addr]
}

func (c *CIContext) SignerKeyByAddress(addr common.Address) *ecdsa.PrivateKey {
	if c == nil {
		return nil
	}
	if key := c.signerKeysByAddress[addr]; key != nil {
		return key
	}
	return c.validatorKeysByAddress[addr]
}

func (c *CIContext) SignerAddressByValidator(addr common.Address) (common.Address, error) {
	if c == nil {
		return common.Address{}, fmt.Errorf("nil context")
	}
	if c.Validators != nil {
		signer, err := c.Validators.GetValidatorSigner(nil, addr)
		if err == nil && signer != (common.Address{}) {
			return signer, nil
		}
	}
	for _, validator := range c.Config.Validators {
		if !strings.EqualFold(validator.Address, addr.Hex()) {
			continue
		}
		if strings.TrimSpace(validator.SignerAddress) != "" {
			return common.HexToAddress(validator.SignerAddress), nil
		}
		break
	}
	return addr, nil
}

func (c *CIContext) ValidatorAddressBySigner(addr common.Address) (common.Address, error) {
	if c == nil {
		return common.Address{}, fmt.Errorf("nil context")
	}
	if c.Validators != nil {
		validator, err := c.Validators.GetValidatorBySigner(nil, addr)
		if err == nil && validator != (common.Address{}) {
			return validator, nil
		}
		historical, err := c.Validators.GetValidatorBySignerHistory(nil, addr)
		if err == nil && historical != (common.Address{}) {
			return historical, nil
		}
	}
	for _, validator := range c.Config.Validators {
		if strings.EqualFold(validator.Address, addr.Hex()) {
			return common.HexToAddress(validator.Address), nil
		}
		if strings.TrimSpace(validator.SignerAddress) != "" && strings.EqualFold(validator.SignerAddress, addr.Hex()) {
			return common.HexToAddress(validator.Address), nil
		}
	}
	return addr, nil
}

func (c *CIContext) FeeAddressByValidator(addr common.Address) (common.Address, error) {
	if c == nil {
		return common.Address{}, fmt.Errorf("nil context")
	}
	if c.Validators != nil {
		feeAddr, _, _, _, _, err := c.Validators.GetValidatorInfo(nil, addr)
		if err == nil && feeAddr != (common.Address{}) {
			return feeAddr, nil
		}
	}
	for _, validator := range c.Config.Validators {
		if strings.EqualFold(validator.Address, addr.Hex()) {
			if strings.TrimSpace(validator.FeeAddress) != "" {
				return common.HexToAddress(validator.FeeAddress), nil
			}
			break
		}
	}
	return addr, nil
}

func (c *CIContext) CurrentCoinbaseSigner() (common.Address, error) {
	if c == nil || len(c.Clients) == 0 {
		return common.Address{}, fmt.Errorf("no rpc clients configured")
	}
	header, err := c.Clients[0].HeaderByNumber(context.Background(), nil)
	if err != nil {
		return common.Address{}, err
	}
	return header.Coinbase, nil
}

func (c *CIContext) CurrentCoinbaseValidator() (common.Address, error) {
	signer, err := c.CurrentCoinbaseSigner()
	if err != nil {
		return common.Address{}, err
	}
	return c.ValidatorAddressBySigner(signer)
}

func (c *CIContext) TopRuntimeSigners() ([]common.Address, error) {
	if c == nil || c.Validators == nil {
		return nil, fmt.Errorf("validators contract not initialized")
	}
	return c.Validators.GetTopSigners(nil)
}

func (c *CIContext) TopTransitionSigners() ([]common.Address, error) {
	if c == nil || c.Validators == nil {
		return nil, fmt.Errorf("validators contract not initialized")
	}
	signers, err := c.Validators.GetTopSignersForEpochTransition(nil)
	if err == nil {
		return signers, nil
	}
	return c.Validators.GetTopSigners(nil)
}

func (c *CIContext) ValidatorRPCByValidator(addr common.Address) string {
	if c == nil {
		return ""
	}
	return c.validatorRPCByAddress[addr]
}

func (c *CIContext) GetTransactor(key *ecdsa.PrivateKey) (*bind.TransactOpts, error) {
	return c.GetTransactorEx(key, true)
}

func (c *CIContext) GetTransactorEx(key *ecdsa.PrivateKey, forceRefresh bool) (*bind.TransactOpts, error) {
	if key == nil {
		return nil, fmt.Errorf("nil private key")
	}

	// Wait if we are at an epoch block to avoid "Epoch block forbidden" errors
	if err := c.WaitIfEpochBlockWithTimeout(defaultEpochWaitTimeout); err != nil {
		return nil, err
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)

	c.mu.Lock()
	_, known := c.nonces[addr]
	c.mu.Unlock()

	if !known || forceRefresh {
		c.RefreshNonce(addr)
	}

	c.mu.Lock()
	nonce := c.nonces[addr]
	c.nonces[addr]++
	c.mu.Unlock()

	opts, err := bind.NewKeyedTransactorWithChainID(key, c.ChainID)
	if err != nil {
		return nil, err
	}

	opts.Nonce = big.NewInt(int64(nonce))
	gasPrice, err := c.Clients[0].SuggestGasPrice(context.Background())
	if err != nil {
		gasPrice = big.NewInt(50000000000) // Fallback to 50 Gwei
	}

	opts.GasPrice = gasPrice
	opts.GasLimit = 0 // Allow estimation

	return opts, nil
}

func (c *CIContext) GetTransactorNoEpochWait(key *ecdsa.PrivateKey, forceRefresh bool) (*bind.TransactOpts, error) {
	if key == nil {
		return nil, fmt.Errorf("nil private key")
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)

	c.mu.Lock()
	_, known := c.nonces[addr]
	c.mu.Unlock()

	if !known || forceRefresh {
		c.RefreshNonce(addr)
	}

	c.mu.Lock()
	nonce := c.nonces[addr]
	c.nonces[addr]++
	c.mu.Unlock()

	opts, err := bind.NewKeyedTransactorWithChainID(key, c.ChainID)
	if err != nil {
		return nil, err
	}

	opts.Nonce = big.NewInt(int64(nonce))
	gasPrice, err := c.Clients[0].SuggestGasPrice(context.Background())
	if err != nil {
		gasPrice = big.NewInt(50000000000) // Fallback to 50 Gwei
	}

	opts.GasPrice = gasPrice
	opts.GasLimit = 0 // Allow estimation

	return opts, nil
}

func (c *CIContext) RefreshNonce(addr common.Address) {
	if addr == (common.Address{}) {
		return
	}
	var maxNonce uint64
	for i, client := range c.Clients {
		n, err := client.PendingNonceAt(context.Background(), addr)
		if err == nil {
			if n > maxNonce {
				maxNonce = n
			}
		} else {
			log.Warn("Failed to get pending nonce from client", "node", i, "addr", addr.Hex(), "err", err)
		}
	}
	c.mu.Lock()
	c.nonces[addr] = maxNonce
	c.mu.Unlock()
	if debugEnabled() {
		fmt.Printf("DEBUG: Refreshed nonce for %s to %d\n", addr.Hex(), maxNonce)
	}
}

func (c *CIContext) SyncNonces() {
	c.mu.Lock()
	addrs := make([]common.Address, 0, len(c.nonces))
	for addr := range c.nonces {
		addrs = append(addrs, addr)
	}
	c.mu.Unlock()

	for _, addr := range addrs {
		c.RefreshNonce(addr)
	}
}

func (c *CIContext) nextDeterministicKey() (*ecdsa.PrivateKey, error) {
	c.mu.Lock()
	idx := c.accountIndex
	c.accountIndex++
	c.mu.Unlock()

	keyInt := new(big.Int).Add(deterministicKeyBase, new(big.Int).SetUint64(idx+1))
	keyBytes := keyInt.FillBytes(make([]byte, 32))
	return crypto.ToECDSA(keyBytes)
}

func (c *CIContext) seedAccountIndexFromFunderNonce() {
	if c == nil || c.FunderKey == nil || len(c.Clients) == 0 {
		return
	}
	addr := crypto.PubkeyToAddress(c.FunderKey.PublicKey)
	var maxNonce uint64
	for _, client := range c.Clients {
		nonce, err := client.PendingNonceAt(context.Background(), addr)
		if err != nil {
			continue
		}
		if nonce > maxNonce {
			maxNonce = nonce
		}
	}
	c.mu.Lock()
	if maxNonce > c.accountIndex {
		c.accountIndex = maxNonce
	}
	c.mu.Unlock()
}

func (c *CIContext) CreateAndFundAccount(amount *big.Int) (*ecdsa.PrivateKey, common.Address, error) {
	key, err := c.nextDeterministicKey()
	if err != nil {
		return nil, common.Address{}, err
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)

	funderAddr := crypto.PubkeyToAddress(c.FunderKey.PublicKey)
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		opts, err := c.GetTransactorEx(c.FunderKey, true)
		if err != nil {
			lastErr = err
			continue
		}

		gasPrice := opts.GasPrice
		if gasPrice == nil || gasPrice.Sign() <= 0 {
			gasPrice = big.NewInt(1000000000) // 1 gwei fallback
		}
		if attempt > 0 {
			// Bump price on retries to avoid replacement-underpriced races.
			gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(int64(attempt+1)))
		}

		if debugEnabled() {
			fmt.Printf("DEBUG: Funding account %s from %s using nonce %d (attempt=%d)\n", addr.Hex(), funderAddr.Hex(), opts.Nonce.Uint64(), attempt+1)
		}

		// For simple transfers, 21000 is enough.
		tx := types.NewTransaction(opts.Nonce.Uint64(), addr, amount, 21000, gasPrice, nil)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(c.ChainID), c.FunderKey)
		if err != nil {
			return nil, common.Address{}, err
		}

		// Only send to the first client to avoid pool conflicts/confusion.
		err = c.Clients[0].SendTransaction(context.Background(), signedTx)
		if err != nil {
			lastErr = err
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "replacement transaction underpriced") ||
				strings.Contains(msg, "nonce too low") ||
				strings.Contains(msg, "already known") {
				c.RefreshNonce(funderAddr)
				time.Sleep(c.RetryPollInterval())
				continue
			}
			return nil, common.Address{}, fmt.Errorf("failed to send funding tx: %w", err)
		}

		log.Info("Funded account", "address", addr.Hex(), "tx", signedTx.Hash().Hex())

		if err := c.WaitMined(signedTx.Hash()); err != nil {
			lastErr = err
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "timeout waiting for tx") || strings.Contains(msg, "replaced") {
				c.RefreshNonce(funderAddr)
				time.Sleep(c.RetryPollInterval())
				continue
			}
			return nil, common.Address{}, fmt.Errorf("funding tx failed: %w", err)
		}

		return key, addr, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown error")
	}
	return nil, common.Address{}, fmt.Errorf("failed to fund account after retries: %w", lastErr)
}

func (c *CIContext) WaitMined(txHash common.Hash) error {
	if txHash == (common.Hash{}) {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	loggedPool := false
	lastKeepAlive := time.Time{}
	lastHeadProbe := time.Time{}
	lastPoke := time.Time{}
	lastHeight := uint64(0)
	stuckSince := time.Time{}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for tx %s after %v", txHash.Hex(), time.Since(startTime))
		case <-ticker.C:
			if lastKeepAlive.IsZero() || time.Since(lastKeepAlive) >= 2*time.Second {
				fmt.Print(".") // Keep-alive output
				lastKeepAlive = time.Now()
			}

			// Recover quickly from epoch-boundary stalls while waiting for receipts.
			if lastHeadProbe.IsZero() || time.Since(lastHeadProbe) >= 500*time.Millisecond {
				height, errHead := c.Clients[0].BlockNumber(context.Background())
				if errHead == nil {
					if height != lastHeight {
						lastHeight = height
						stuckSince = time.Time{}
					} else if stuckSince.IsZero() {
						stuckSince = time.Now()
					}

					if !stuckSince.IsZero() && time.Since(stuckSince) > 3*time.Second {
						// Poke chain progression on generic stalls.
						if lastPoke.IsZero() || time.Since(lastPoke) > 2*time.Second {
							c.sendDummyTx()
							lastPoke = time.Now()
						}
					}
				}
				lastHeadProbe = time.Now()
			}

			receipt, err := c.Clients[0].TransactionReceipt(context.Background(), txHash)
			if err == nil {
				if receipt.Status == 0 {
					return fmt.Errorf("transaction %s reverted", txHash.Hex())
				}
				return nil
			}

			if time.Since(startTime) > 60*time.Second && !loggedPool {
				loggedPool = true
				var result interface{}
				err := c.Clients[0].Client().Call(&result, "txpool_content")
				if err == nil {
					log.Info("TX Pool Content on timeout", "content", result)
				}

				var sync interface{}
				err = c.Clients[0].Client().Call(&sync, "eth_syncing")
				if err == nil {
					log.Info("Sync status on timeout", "status", sync)
				}

				header, err := c.Clients[0].HeaderByNumber(context.Background(), nil)
				if err == nil {
					log.Info("Current block on timeout", "number", header.Number.String(), "hash", header.Hash().Hex())
				}
			}
		}
	}
}

func (c *CIContext) EnsureConfig(cid int64, targetVal *big.Int, currentVal *big.Int) error {
	if currentVal != nil && currentVal.Cmp(targetVal) == 0 {
		return nil
	}

	if len(c.GenesisValidators) == 0 {
		return fmt.Errorf("no genesis validators")
	}

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		// Double check fresh value from contract.
		freshVal, err := c.GetConfigValue(cid)
		if err == nil && freshVal != nil && freshVal.Cmp(targetVal) == 0 {
			return nil
		}
		if err != nil {
			lastErr = err
		}

		log.Info("Updating config", "cid", cid, "target", targetVal, "current", freshVal)

		lastErr = nil
		for i := 0; i < len(c.GenesisValidators); i++ {
			c.mu.Lock()
			proposerKey := c.GenesisValidators[c.proposerIndex%len(c.GenesisValidators)]
			c.proposerIndex++
			c.mu.Unlock()

			proposerAddr := crypto.PubkeyToAddress(proposerKey.PublicKey)
			active, _ := c.Validators.IsValidatorActive(nil, proposerAddr)
			if !active {
				continue
			}
			info, _ := c.Staking.GetValidatorInfo(nil, proposerAddr)
			if info.IsJailed {
				continue
			}

			opts, errG := c.GetTransactor(proposerKey)
			if errG != nil {
				lastErr = errG
				continue
			}

			opts.GasLimit = 1000000
			tx, errCall := c.Proposal.CreateUpdateConfigProposal(opts, big.NewInt(cid), targetVal)
			if errCall != nil {
				lastErr = errCall
				if strings.Contains(errCall.Error(), "Proposal creation too frequent") ||
					strings.Contains(errCall.Error(), "Epoch block forbidden") ||
					strings.Contains(errCall.Error(), "nonce too low") {
					c.waitBlocks(1)
					continue
				}
				if updatedVal, errCheck := c.GetConfigValue(cid); errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
					return nil
				}
				continue
			}

			if errMined := c.WaitMined(tx.Hash()); errMined != nil {
				lastErr = errMined
				if updatedVal, errCheck := c.GetConfigValue(cid); errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
					return nil
				}
				c.waitBlocks(1)
				continue
			}

			receipt, errReceipt := c.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
			if errReceipt != nil || receipt == nil {
				lastErr = fmt.Errorf("get config proposal receipt failed: %w", errReceipt)
				c.waitBlocks(1)
				continue
			}

			var propID [32]byte
			found := false
			for _, l := range receipt.Logs {
				if ev, errP := c.Proposal.ParseLogCreateConfigProposal(*l); errP == nil {
					propID = ev.Id
					found = true
					break
				}
				if ev, errP := c.Proposal.ParseLogCreateProposal(*l); errP == nil {
					propID = ev.Id
					found = true
					break
				}
			}
			if !found {
				if updatedVal, errCheck := c.GetConfigValue(cid); errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
					return nil
				}
				lastErr = fmt.Errorf("proposal log not found for tx %s", tx.Hash().Hex())
				c.waitBlocks(1)
				continue
			}

			for _, vk := range c.GenesisValidators {
				// Proposal may already be finalized by previous votes.
				if updatedVal, errCheck := c.GetConfigValue(cid); errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
					return nil
				}

				voterAddr := crypto.PubkeyToAddress(vk.PublicKey)
				active, _ := c.Validators.IsValidatorActive(nil, voterAddr)
				if !active {
					continue
				}
				info, _ := c.Staking.GetValidatorInfo(nil, voterAddr)
				if info.IsJailed {
					continue
				}

				for retry := 0; retry < 3; retry++ {
					vo, errVo := c.GetTransactor(vk)
					if errVo != nil {
						lastErr = errVo
						c.waitBlocks(1)
						continue
					}
					vo.GasLimit = 500000
					txV, errV := c.Proposal.VoteProposal(vo, propID, true)
					if errV != nil {
						lastErr = errV
						if strings.Contains(errV.Error(), "Epoch block forbidden") {
							c.waitBlocks(1)
							continue
						}
						if strings.Contains(errV.Error(), "You can't vote for a proposal twice") ||
							strings.Contains(errV.Error(), "Proposal already passed") ||
							strings.Contains(errV.Error(), "Proposal has expired") {
							break
						}
						if updatedVal, errCheck := c.GetConfigValue(cid); errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
							return nil
						}
						break
					}

					errW := c.WaitMined(txV.Hash())
					if errW == nil {
						// Stop early once target config is effective.
						if updatedVal, errCheck := c.GetConfigValue(cid); errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
							return nil
						}
						break
					}

					lastErr = errW
					if updatedVal, errCheck := c.GetConfigValue(cid); errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
						return nil
					}
					if strings.Contains(errW.Error(), "Epoch block forbidden") ||
						strings.Contains(errW.Error(), "reverted") ||
						strings.Contains(errW.Error(), "timeout waiting for tx") {
						c.waitBlocks(1)
						continue
					}
					break
				}
			}

			// Re-check after votes.
			updatedVal, errCheck := c.GetConfigValue(cid)
			if errCheck == nil && updatedVal != nil && updatedVal.Cmp(targetVal) == 0 {
				return nil
			}
			if errCheck != nil {
				lastErr = errCheck
			} else {
				lastErr = fmt.Errorf("config %d not updated after proposal", cid)
			}
			break
		}

		if lastErr == nil {
			lastErr = fmt.Errorf("no eligible proposer found")
		}

		c.waitBlocks(c.proposalCooldownBlocks())
	}

	return fmt.Errorf("failed to ensure config %d: %v", cid, lastErr)
}

func (c *CIContext) proposalCooldownBlocks() uint64 {
	val, err := c.Proposal.ProposalCooldown(nil)
	if err == nil && val.Uint64() > 0 {
		return val.Uint64()
	}
	return 1
}

func (c *CIContext) epochValue() uint64 {
	if c == nil {
		return 0
	}
	if c.Validators != nil {
		if epochVal, err := c.Validators.Epoch(nil); err == nil && epochVal != nil && epochVal.Sign() > 0 {
			return epochVal.Uint64()
		}
	}
	if c.Proposal != nil {
		if epochVal, err := c.Proposal.Epoch(nil); err == nil && epochVal != nil && epochVal.Sign() > 0 {
			return epochVal.Uint64()
		}
	}
	return c.configuredEpoch()
}

func sameAddressSlice(a []common.Address, b []common.Address) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	counts := make(map[common.Address]int, len(a))
	for _, addr := range a {
		counts[addr]++
	}
	for _, addr := range b {
		if counts[addr] == 0 {
			return false
		}
		counts[addr]--
	}
	for _, remaining := range counts {
		if remaining != 0 {
			return false
		}
	}
	return true
}

func (c *CIContext) waitUntilHeight(target uint64, timeout time.Duration) (uint64, error) {
	deadline := time.Now().Add(timeout)
	for {
		height, err := c.Clients[0].BlockNumber(context.Background())
		if err == nil {
			if height >= target {
				return height, nil
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return 0, fmt.Errorf("wait for block %d failed: %w", target, err)
			}
			return 0, fmt.Errorf("timeout waiting for block %d", target)
		}
		c.sendDummyTx()
		time.Sleep(c.BlockPollInterval())
	}
}

func (c *CIContext) WaitUntilHeight(target uint64, timeout time.Duration) (uint64, error) {
	return c.waitUntilHeight(target, timeout)
}

func (c *CIContext) waitBlocks(n uint64) {
	if n == 0 {
		return
	}
	start, err := c.Clients[0].BlockNumber(context.Background())
	if err != nil {
		time.Sleep(c.BlockPollInterval())
		start, _ = c.Clients[0].BlockNumber(context.Background())
	}
	target := start + n
	for {
		cur, err := c.Clients[0].BlockNumber(context.Background())
		if err != nil {
			time.Sleep(c.BlockPollInterval())
			continue
		}
		if cur >= target {
			return
		}
		c.sendDummyTx()
		time.Sleep(c.BlockPollInterval())
	}
}

func (c *CIContext) WaitForNextEpochTransition() (uint64, error) {
	if c == nil || len(c.Clients) == 0 {
		return 0, fmt.Errorf("context not initialized")
	}

	epoch := c.epochValue()
	if epoch == 0 {
		return 0, fmt.Errorf("epoch not available")
	}

	current, err := c.Clients[0].BlockNumber(context.Background())
	if err != nil {
		return 0, fmt.Errorf("read current block failed: %w", err)
	}

	nextEpochBlock := ((current / epoch) + 1) * epoch
	if current%epoch == 0 {
		nextEpochBlock = current + epoch
	}
	if nextEpochBlock == 0 {
		return 0, fmt.Errorf("invalid next epoch block computed from current=%d epoch=%d", current, epoch)
	}

	if _, err := c.waitUntilHeight(nextEpochBlock, 60*time.Second); err != nil {
		return 0, err
	}
	if err := c.WaitIfEpochBlockWithTimeout(defaultEpochWaitTimeout); err != nil {
		return 0, err
	}

	deadline := time.Now().Add(20 * time.Second)
	var lastErr error
	for {
		height, err := c.Clients[0].BlockNumber(context.Background())
		if err != nil {
			lastErr = fmt.Errorf("read stable height failed: %w", err)
		} else {
			highest, err := c.Validators.GetHighestValidators(nil)
			if err != nil {
				lastErr = fmt.Errorf("get highest validators failed: %w", err)
			} else {
				expectedSet, err := c.Staking.GetTopValidators(nil, highest)
				if err != nil {
					lastErr = fmt.Errorf("get top validators failed: %w", err)
				} else if len(expectedSet) == 0 {
					lastErr = fmt.Errorf("top validator set is empty after epoch block %d", nextEpochBlock)
				} else {
					activeSet, err := c.Validators.GetActiveValidators(nil)
					if err != nil {
						lastErr = fmt.Errorf("get active validators failed: %w", err)
					} else if sameAddressSlice(activeSet, expectedSet) {
						c.SyncNonces()
						return height, nil
					} else {
						lastErr = fmt.Errorf(
							"active validator set not settled after epoch block %d (height=%d expected=%d active=%d)",
							nextEpochBlock,
							height,
							len(expectedSet),
							len(activeSet),
						)
					}
				}
			}
		}

		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = fmt.Errorf("epoch transition did not settle after block %d", nextEpochBlock)
			}
			return 0, lastErr
		}

		c.sendDummyTx()
		time.Sleep(c.BlockPollInterval())
	}
}

func (c *CIContext) triggerEpochTransition(height uint64, epoch uint64) bool {
	// External system transactions are rejected by the chain. Keep this helper
	// as a no-op so older recovery sites do not try to submit forbidden calls.
	_ = height
	_ = epoch
	return true
}

func (c *CIContext) recentCoinbases(limit int) []common.Address {
	if c == nil || len(c.Clients) == 0 || limit <= 0 {
		return nil
	}

	latest, err := c.Clients[0].BlockNumber(context.Background())
	if err != nil {
		return nil
	}

	start := uint64(1)
	if latest >= uint64(limit) {
		start = latest - uint64(limit) + 1
	}

	items := make([]common.Address, 0, latest-start+1)
	for height := start; height <= latest; height++ {
		header, err := c.Clients[0].HeaderByNumber(context.Background(), new(big.Int).SetUint64(height))
		if err != nil || header == nil {
			continue
		}
		items = append(items, header.Coinbase)
	}
	return items
}

func (c *CIContext) WaitIfEpochBlockWithTimeout(timeout time.Duration) error {
	if c == nil || len(c.Clients) == 0 {
		return fmt.Errorf("context not initialized")
	}
	epoch := c.epochValue()
	if epoch == 0 {
		return nil
	}
	if timeout <= 0 {
		timeout = defaultEpochWaitTimeout
	}

	stallTimeout := defaultEpochStallTimeout
	if timeout < stallTimeout {
		stallTimeout = timeout
	}

	start := time.Now()
	lastPoke := time.Now()
	lastLog := time.Time{}
	lastHeight := uint64(0)
	lastProgress := time.Now()
	var lastErr error

	for time.Since(start) < timeout {
		height, err := c.Clients[0].BlockNumber(context.Background())
		if err != nil {
			lastErr = err
			if time.Since(lastPoke) > 2*time.Second {
				c.sendDummyTx()
				lastPoke = time.Now()
			}
			time.Sleep(c.BlockPollInterval())
			continue
		}

		lastErr = nil
		if height != lastHeight {
			lastHeight = height
			lastProgress = time.Now()
		}

		mod := height % epoch
		if mod != 0 && mod != epoch-1 {
			return nil
		}
		if time.Since(lastLog) > 5*time.Second {
			fmt.Printf("⏳ At epoch block %d, waiting for next block...\n", height)
			lastLog = time.Now()
		}
		if time.Since(lastProgress) >= stallTimeout {
			return fmt.Errorf(
				"epoch-boundary wait stalled: height=%d epoch=%d stalled_for=%s recent_coinbases=%v",
				height,
				epoch,
				time.Since(lastProgress).Round(time.Second),
				c.recentCoinbases(12),
			)
		}
		if time.Since(lastPoke) > 2*time.Second {
			c.sendDummyTx()
			lastPoke = time.Now()
		}
		time.Sleep(c.BlockPollInterval())
	}

	if lastErr != nil {
		return fmt.Errorf("epoch-boundary wait timed out after %s: %w", timeout.Round(time.Second), lastErr)
	}
	return fmt.Errorf(
		"epoch-boundary wait timed out after %s: height=%d epoch=%d recent_coinbases=%v",
		timeout.Round(time.Second),
		lastHeight,
		epoch,
		c.recentCoinbases(12),
	)
}

func (c *CIContext) WaitIfEpochBlock() {
	if err := c.WaitIfEpochBlockWithTimeout(defaultEpochWaitTimeout); err != nil {
		fmt.Printf("⚠️ epoch-boundary wait aborted: %v\n", err)
	}
}
