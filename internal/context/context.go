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
	return 60
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
	for i, v := range cfg.Validators {
		key, err := crypto.HexToECDSA(v.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid validator private key at index %d: %w", i, err)
		}
		genesisValidators = append(genesisValidators, key)
	}

	c := &CIContext{
		Config:            cfg,
		Clients:           clients,
		ChainID:           chainID,
		Validators:        val,
		Punish:            pun,
		Proposal:          prop,
		Staking:           stk,
		ProposalAddr:      ProposalAddr,
		FunderKey:         funderKey,
		GenesisValidators: genesisValidators,
		nonces:            make(map[common.Address]uint64),
	}

	if err := c.WaitForBlockProgress(2, 120*time.Second); err != nil {
		return nil, fmt.Errorf("chain not producing blocks: %w", err)
	}

	// Auto-Initialize if needed
	err = c.autoInitialize()
	if err != nil {
		fmt.Printf("⚠️ autoInitialize failed: %v\n", err)
	}

	return c, nil
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
		time.Sleep(500 * time.Millisecond)
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
	// Robust check: if MinValidatorStake is default (100k JU), we need setup.
	minStake, err := c.Proposal.MinValidatorStake(nil)
	if err == nil && minStake.Cmp(big.NewInt(1000000000000000000)) == 0 {
		fmt.Printf("✅ System already configured (MinValidatorStake = 1 JU).\n")
	}

	// Check if we need to call initialize() at all
	initialized, _ := c.Proposal.Initialized(nil)
	if !initialized {
		fmt.Printf("🔧 System unconfigured, performing auto-initialization...\n")
		var valAddrs []common.Address
		for _, vk := range c.GenesisValidators {
			valAddrs = append(valAddrs, crypto.PubkeyToAddress(vk.PublicKey))
		}

		// 1. Initialize Proposal
		opts, _ := c.GetTransactor(c.GenesisValidators[0])
		opts.GasLimit = 1000000
		fmt.Printf("  > Initializing Proposal...\n")
		tx, err := c.Proposal.Initialize(opts, valAddrs, ValidatorsAddr, new(big.Int).SetUint64(c.configuredEpoch()))
		if err == nil {
			c.WaitMined(tx.Hash())
		}

		// 2. Initialize Staking with Validators
		opts, _ = c.GetTransactor(c.GenesisValidators[1])
		opts.GasLimit = 2000000
		fmt.Printf("  > Initializing Staking with Validators...\n")
		tx, err = c.Staking.InitializeWithValidators(opts, ValidatorsAddr, ProposalAddr, PunishAddr, valAddrs, big.NewInt(1000))
		if err == nil {
			c.WaitMined(tx.Hash())
		}

		// 3. Initialize Validators
		opts, _ = c.GetTransactor(c.GenesisValidators[2])
		opts.GasLimit = 1000000
		fmt.Printf("  > Initializing Validators...\n")
		tx, err = c.Validators.Initialize(opts, valAddrs, ProposalAddr, PunishAddr, StakingAddr)
		if err == nil {
			c.WaitMined(tx.Hash())
		}
	}

	// 4. Always ensure test-friendly parameters if they are not set
	fmt.Printf("  > Configuring system parameters...\n")

	// Fetch current values to skip if already set
	pCool, _ := c.GetConfigValue(19)
	_ = c.EnsureConfig(19, big.NewInt(1), pCool)

	unbond, _ := c.GetConfigValue(6)
	_ = c.EnsureConfig(6, big.NewInt(10), unbond)

	unjail, _ := c.GetConfigValue(7)
	_ = c.EnsureConfig(7, big.NewInt(10), unjail)

	withdraw, _ := c.GetConfigValue(4)
	_ = c.EnsureConfig(4, big.NewInt(5), withdraw)

	minStakeVal, _ := c.GetConfigValue(8)
	_ = c.EnsureConfig(8, big.NewInt(1000000000000000000), minStakeVal)

	minDel, _ := c.GetConfigValue(10)
	_ = c.EnsureConfig(10, big.NewInt(1000000000000000000), minDel)

	commCool, _ := c.GetConfigValue(16)
	_ = c.EnsureConfig(16, big.NewInt(5), commCool)

	propLast, _ := c.GetConfigValue(0)
	_ = c.EnsureConfig(0, big.NewInt(100), propLast)

	fmt.Printf("✅ Auto-initialization complete.\n")
	return nil
}

func (c *CIContext) GetTransactor(key *ecdsa.PrivateKey) (*bind.TransactOpts, error) {
	return c.GetTransactorEx(key, true)
}

func (c *CIContext) GetTransactorEx(key *ecdsa.PrivateKey, forceRefresh bool) (*bind.TransactOpts, error) {
	if key == nil {
		return nil, fmt.Errorf("nil private key")
	}

	// Wait if we are at an epoch block to avoid "Epoch block forbidden" errors
	c.WaitIfEpochBlock()

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

func (c *CIContext) CreateAndFundAccount(amount *big.Int) (*ecdsa.PrivateKey, common.Address, error) {
	key, err := c.nextDeterministicKey()
	if err != nil {
		return nil, common.Address{}, err
	}
	addr := crypto.PubkeyToAddress(key.PublicKey)

	opts, err := c.GetTransactorEx(c.FunderKey, true)
	if err != nil {
		return nil, common.Address{}, err
	}

	if debugEnabled() {
		fmt.Printf("DEBUG: Funding account %s from %s using nonce %d\n", addr.Hex(), crypto.PubkeyToAddress(c.FunderKey.PublicKey).Hex(), opts.Nonce.Uint64())
	}

	// For simple transfers, 21000 is enough
	tx := types.NewTransaction(opts.Nonce.Uint64(), addr, amount, 21000, opts.GasPrice, nil)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(c.ChainID), c.FunderKey)
	if err != nil {
		return nil, common.Address{}, err
	}

	// Only send to the first client to avoid pool conflicts/confusion
	err = c.Clients[0].SendTransaction(context.Background(), signedTx)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to send funding tx: %w", err)
	}

	log.Info("Funded account", "address", addr.Hex(), "tx", signedTx.Hash().Hex())

	if err := c.WaitMined(signedTx.Hash()); err != nil {
		return nil, common.Address{}, fmt.Errorf("funding tx failed: %w", err)
	}

	return key, addr, nil
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

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for tx %s after %v", txHash.Hex(), time.Since(startTime))
		case <-ticker.C:
			fmt.Print(".") // Keep-alive output
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
		// Double check fresh value from contract
		freshVal, err := c.GetConfigValue(cid)
		if err == nil && freshVal.Cmp(targetVal) == 0 {
			return nil
		}

		log.Info("Updating config", "cid", cid, "target", targetVal, "current", freshVal)

		lastErr = nil
		for i := 0; i < len(c.GenesisValidators); i++ {
			c.mu.Lock()
			proposerKey := c.GenesisValidators[c.proposerIndex%len(c.GenesisValidators)]
			c.proposerIndex++
			c.mu.Unlock()

			proposerAddr := crypto.PubkeyToAddress(proposerKey.PublicKey)
			actve, _ := c.Validators.IsValidatorActive(nil, proposerAddr)
			if !actve {
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
				if strings.Contains(errCall.Error(), "Proposal creation too frequent") || strings.Contains(errCall.Error(), "Epoch block forbidden") {
					continue
				}
				return fmt.Errorf("createUpdateConfigProposal failed: %w", errCall)
			}

			err = c.WaitMined(tx.Hash())
			if err != nil {
				lastErr = err
				continue
			}

			receipt, _ := c.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
			var propID [32]byte
			found := false
			for _, l := range receipt.Logs {
				if ev, errP := c.Proposal.ParseLogCreateConfigProposal(*l); errP == nil {
					propID = ev.Id
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("proposal log not found for tx %s", tx.Hash().Hex())
			}

			for _, vk := range c.GenesisValidators {
				voterAddr := crypto.PubkeyToAddress(vk.PublicKey)
				actve, _ := c.Validators.IsValidatorActive(nil, voterAddr)
				if !actve {
					continue
				}
				info, _ := c.Staking.GetValidatorInfo(nil, voterAddr)
				if info.IsJailed {
					continue
				}

				for retry := 0; retry < 3; retry++ {
					vo, _ := c.GetTransactor(vk)
					vo.GasLimit = 500000
					txV, errV := c.Proposal.VoteProposal(vo, propID, true)
					if errV == nil {
						errW := c.WaitMined(txV.Hash())
						if errW == nil {
							break
						}
						if strings.Contains(errW.Error(), "Epoch block forbidden") {
							continue
						}
						return errW
					}
					if strings.Contains(errV.Error(), "Epoch block forbidden") {
						continue
					}
					if strings.Contains(errV.Error(), "You can't vote for a proposal twice") {
						break
					}
					return errV
				}
			}

			// Re-check after votes
			updatedVal, err := c.GetConfigValue(cid)
			if err == nil && updatedVal.Cmp(targetVal) == 0 {
				return nil
			}
			lastErr = fmt.Errorf("config %d not updated after proposal", cid)
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

func (c *CIContext) waitBlocks(n uint64) {
	if n == 0 {
		return
	}
	start, err := c.Clients[0].BlockNumber(context.Background())
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		start, _ = c.Clients[0].BlockNumber(context.Background())
	}
	target := start + n
	for {
		cur, err := c.Clients[0].BlockNumber(context.Background())
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if cur >= target {
			return
		}
		c.sendDummyTx()
		time.Sleep(200 * time.Millisecond)
	}
}

func (c *CIContext) WaitIfEpochBlock() {
	epochVal, err := c.Validators.Epoch(nil)
	epoch := c.configuredEpoch()
	if err == nil && epochVal.Uint64() > 0 {
		epoch = epochVal.Uint64()
	}

	lastPoke := time.Now()
	lastLog := time.Time{}
	for {
		height, err := c.Clients[0].BlockNumber(context.Background())
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		mod := height % epoch
		if mod != 0 && mod != epoch-1 {
			return
		}
		if time.Since(lastLog) > 5*time.Second {
			fmt.Printf("⏳ At epoch block %d, waiting for next block...\n", height)
			lastLog = time.Now()
		}
		if time.Since(lastPoke) > 2*time.Second {
			c.sendDummyTx()
			lastPoke = time.Now()
		}
		time.Sleep(200 * time.Millisecond)
	}
}
