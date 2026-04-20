package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	defaultSenderFundingWei = "10000000000000000000" // 10 ETH
	defaultDispatchWindow   = 100 * time.Millisecond
	defaultQueueDepth       = 256
	defaultDrainTimeout     = 30 * time.Second
)

var perfDeterministicKeyBase = new(big.Int).Lsh(big.NewInt(1), 251)

type senderAccount struct {
	key             *ecdsa.PrivateKey
	address         common.Address
	queue           chan struct{}
	nextNonce       uint64
	startNonce      uint64
	startConfirmed  uint64
	gasPrice        *big.Int
	lastError       atomic.Value
	confirmedLatest atomic.Uint64
}

type loadGenerator struct {
	client           *ethclient.Client
	chainID          *big.Int
	accounts         []*senderAccount
	targetTPS        int
	dispatchWindow   time.Duration
	stopCh           chan struct{}
	doneCh           chan struct{}
	dispatched       atomic.Int64
	sent             atomic.Int64
	accepted         atomic.Int64
	failed           atomic.Int64
	backpressureDrop atomic.Int64
	mu               sync.Mutex
	stopped          bool
}

type loadGeneratorSnapshot struct {
	Dispatched       int64
	Sent             int64
	Accepted         int64
	Failed           int64
	BackpressureDrop int64
	Confirmed        int64
	PendingBacklog   int64
}

func buildMaxTPSSteps(base, step, target int) ([]int, error) {
	if base <= 0 {
		return nil, fmt.Errorf("base tps must be > 0")
	}
	if step <= 0 {
		return nil, fmt.Errorf("step tps must be > 0")
	}
	if target < base {
		return nil, fmt.Errorf("target tps %d must be >= base tps %d", target, base)
	}
	steps := make([]int, 0, ((target-base)/step)+1)
	for cur := base; cur <= target; cur += step {
		steps = append(steps, cur)
	}
	if len(steps) == 0 || steps[len(steps)-1] != target {
		steps = append(steps, target)
	}
	return steps, nil
}

func recommendedSenderAccountCount(maxTPS int) int {
	if maxTPS <= 0 {
		return 1
	}
	count := int(math.Ceil(float64(maxTPS) / 100.0))
	if count < 4 {
		count = 4
	}
	if count > 128 {
		count = 128
	}
	return count
}

func deterministicPerfKey(index int) (*ecdsa.PrivateKey, error) {
	if index < 0 {
		return nil, fmt.Errorf("sender index must be non-negative")
	}
	keyInt := new(big.Int).Add(perfDeterministicKeyBase, big.NewInt(int64(index+1)))
	keyBytes := keyInt.FillBytes(make([]byte, 32))
	return crypto.ToECDSA(keyBytes)
}

func fundingAmountWei() *big.Int {
	amt, ok := new(big.Int).SetString(defaultSenderFundingWei, 10)
	if !ok {
		return big.NewInt(0)
	}
	return amt
}

func prepareSenderAccounts(client *ethclient.Client, funderKey *ecdsa.PrivateKey, chainID *big.Int, count int) ([]*senderAccount, error) {
	if client == nil || funderKey == nil || chainID == nil {
		return nil, fmt.Errorf("sender account preparation requires client, funder key, and chain id")
	}
	if count <= 0 {
		return nil, fmt.Errorf("sender account count must be > 0")
	}

	accounts := make([]*senderAccount, 0, count)
	for i := 0; i < count; i++ {
		key, err := deterministicPerfKey(i)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, &senderAccount{
			key:     key,
			address: crypto.PubkeyToAddress(key.PublicKey),
			queue:   make(chan struct{}, defaultQueueDepth),
		})
	}

	if err := fundAccounts(client, funderKey, chainID, accounts, fundingAmountWei()); err != nil {
		return nil, err
	}
	if err := syncSenderAccountNonces(client, accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func fundAccounts(client *ethclient.Client, funderKey *ecdsa.PrivateKey, chainID *big.Int, accounts []*senderAccount, amount *big.Int) error {
	if len(accounts) == 0 {
		return nil
	}
	from := crypto.PubkeyToAddress(funderKey.PublicKey)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	baseNonce, err := client.PendingNonceAt(ctx, from)
	cancel()
	if err != nil {
		return fmt.Errorf("read funder nonce failed: %w", err)
	}
	gasPrice := big.NewInt(1_000_000_000)
	if gp, err := client.SuggestGasPrice(context.Background()); err == nil && gp != nil && gp.Sign() > 0 {
		gasPrice = gp
	}

	hashes := make([]common.Hash, 0, len(accounts))
	for i, account := range accounts {
		latestNonce, err := client.NonceAt(context.Background(), account.address, nil)
		if err == nil && latestNonce > 0 {
			continue
		}
		tx := types.NewTransaction(baseNonce+uint64(i), account.address, amount, 21000, gasPrice, nil)
		signed, err := types.SignTx(tx, types.NewEIP155Signer(chainID), funderKey)
		if err != nil {
			return fmt.Errorf("sign funding tx for %s failed: %w", account.address.Hex(), err)
		}
		if err := client.SendTransaction(context.Background(), signed); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "already known") || strings.Contains(msg, "nonce too low") {
				continue
			}
			return fmt.Errorf("send funding tx for %s failed: %w", account.address.Hex(), err)
		}
		hashes = append(hashes, signed.Hash())
	}

	deadline := time.Now().Add(5 * time.Minute)
	for _, hash := range hashes {
		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for funding tx %s", hash.Hex())
			}
			receipt, err := client.TransactionReceipt(context.Background(), hash)
			if err == nil && receipt != nil {
				if receipt.Status == 0 {
					return fmt.Errorf("funding tx %s reverted", hash.Hex())
				}
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func syncSenderAccountNonces(client *ethclient.Client, accounts []*senderAccount) error {
	for _, account := range accounts {
		pendingNonce, err := client.PendingNonceAt(context.Background(), account.address)
		if err != nil {
			return fmt.Errorf("read pending nonce for %s failed: %w", account.address.Hex(), err)
		}
		confirmedNonce, err := client.NonceAt(context.Background(), account.address, nil)
		if err != nil {
			return fmt.Errorf("read latest nonce for %s failed: %w", account.address.Hex(), err)
		}
		account.nextNonce = pendingNonce
		account.startNonce = pendingNonce
		account.startConfirmed = confirmedNonce
		account.confirmedLatest.Store(confirmedNonce)
		account.lastError.Store("")
		for len(account.queue) > 0 {
			<-account.queue
		}
	}
	return nil
}

func newLoadGenerator(client *ethclient.Client, chainID *big.Int, accounts []*senderAccount, targetTPS int) *loadGenerator {
	return &loadGenerator{
		client:         client,
		chainID:        chainID,
		accounts:       accounts,
		targetTPS:      targetTPS,
		dispatchWindow: defaultDispatchWindow,
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
}

func (g *loadGenerator) Start() {
	if g == nil || len(g.accounts) == 0 || g.targetTPS <= 0 {
		close(g.doneCh)
		return
	}
	gasPrice := big.NewInt(1_000_000_000)
	if gp, err := g.client.SuggestGasPrice(context.Background()); err == nil && gp != nil && gp.Sign() > 0 {
		gasPrice = gp
	}
	for _, account := range g.accounts {
		account.gasPrice = new(big.Int).Set(gasPrice)
		go g.runAccountWorker(account)
	}
	go g.runDispatcher()
}

func (g *loadGenerator) Stop() {
	if g == nil {
		return
	}
	g.mu.Lock()
	if g.stopped {
		g.mu.Unlock()
		return
	}
	g.stopped = true
	close(g.stopCh)
	g.mu.Unlock()
	<-g.doneCh
}

func (g *loadGenerator) runDispatcher() {
	defer close(g.doneCh)
	window := g.dispatchWindow
	if window <= 0 {
		window = defaultDispatchWindow
	}
	ticker := time.NewTicker(window)
	defer ticker.Stop()

	nextAccount := 0
	carry := 0.0
	for {
		select {
		case <-g.stopCh:
			return
		case <-ticker.C:
			burstFloat := (float64(g.targetTPS) * float64(window)) / float64(time.Second)
			burstFloat += carry
			burst := int(math.Floor(burstFloat))
			carry = burstFloat - float64(burst)
			if burst <= 0 && g.targetTPS > 0 {
				burst = 1
				carry = 0
			}
			for i := 0; i < burst; i++ {
				account := g.accounts[nextAccount%len(g.accounts)]
				nextAccount++
				g.dispatched.Add(1)
				select {
				case account.queue <- struct{}{}:
				default:
					g.failed.Add(1)
					g.backpressureDrop.Add(1)
					account.lastError.Store("sender queue backpressure")
				}
			}
		}
	}
}

func (g *loadGenerator) runAccountWorker(account *senderAccount) {
	for {
		select {
		case <-g.stopCh:
			return
		case <-account.queue:
			g.sent.Add(1)
			nonce := account.nextNonce
			tx := types.NewTransaction(nonce, account.address, big.NewInt(0), 21000, account.gasPrice, nil)
			signed, err := types.SignTx(tx, types.NewEIP155Signer(g.chainID), account.key)
			if err != nil {
				g.failed.Add(1)
				account.lastError.Store(err.Error())
				continue
			}
			if err := g.client.SendTransaction(context.Background(), signed); err != nil {
				g.failed.Add(1)
				account.lastError.Store(err.Error())
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "nonce too low") || strings.Contains(msg, "replacement transaction underpriced") || strings.Contains(msg, "already known") {
					if refreshed, refreshErr := g.client.PendingNonceAt(context.Background(), account.address); refreshErr == nil {
						account.nextNonce = refreshed
					}
				}
				continue
			}
			g.accepted.Add(1)
			account.nextNonce++
		}
	}
}

func (g *loadGenerator) Snapshot() loadGeneratorSnapshot {
	snap := loadGeneratorSnapshot{
		Dispatched:       g.dispatched.Load(),
		Sent:             g.sent.Load(),
		Accepted:         g.accepted.Load(),
		Failed:           g.failed.Load(),
		BackpressureDrop: g.backpressureDrop.Load(),
	}
	confirmed := g.refreshConfirmedCount()
	snap.Confirmed = confirmed
	snap.PendingBacklog = snap.Accepted - confirmed
	if snap.PendingBacklog < 0 {
		snap.PendingBacklog = 0
	}
	return snap
}

func (g *loadGenerator) refreshConfirmedCount() int64 {
	var total int64
	for _, account := range g.accounts {
		confirmedNonce, err := g.client.NonceAt(context.Background(), account.address, nil)
		if err != nil {
			account.lastError.Store(err.Error())
			confirmedNonce = account.confirmedLatest.Load()
		} else {
			account.confirmedLatest.Store(confirmedNonce)
		}
		confirmedDelta := int64(0)
		if confirmedNonce >= account.startConfirmed {
			confirmedDelta = int64(confirmedNonce - account.startConfirmed)
		}
		total += confirmedDelta
	}
	return total
}

func (g *loadGenerator) DrainConfirmations(timeout time.Duration) loadGeneratorSnapshot {
	if timeout <= 0 {
		timeout = defaultDrainTimeout
	}
	deadline := time.Now().Add(timeout)
	for {
		snap := g.Snapshot()
		if snap.PendingBacklog == 0 {
			return snap
		}
		if time.Now().After(deadline) {
			return snap
		}
		time.Sleep(1 * time.Second)
	}
}
