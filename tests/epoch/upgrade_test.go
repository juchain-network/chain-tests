package tests

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
)

func TestZ_UpgradesAndInitGuards(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if len(ctx.GenesisValidators) == 0 {
		t.Fatalf("No genesis validators configured")
	}

	t.Run("InitGuards", func(t *testing.T) {
		dummy := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
		opts, _ := ctx.GetTransactor(ctx.GenesisValidators[0])

		checkReinit := func(name string, call func() (*types.Transaction, error)) {
			tx, err := call()
			if err == nil {
				// If simulation didn't catch it, WaitMined must
				err = ctx.WaitMined(tx.Hash())
			}

			if err == nil {
				t.Errorf("%s.initialize should have failed", name)
			} else {
				t.Logf("%s.initialize failed as expected: %v", name, err)
			}
		}

		// Proposal.initialize
		checkReinit("Proposal", func() (*types.Transaction, error) {
			return ctx.Proposal.Initialize(opts, []common.Address{dummy}, dummy, big.NewInt(1))
		})

		// Validators.initialize
		checkReinit("Validators", func() (*types.Transaction, error) {
			return ctx.Validators.Initialize(opts, []common.Address{dummy}, []common.Address{dummy}, dummy, dummy, dummy)
		})

		// Punish.initialize
		checkReinit("Punish", func() (*types.Transaction, error) {
			return ctx.Punish.Initialize(opts, dummy, dummy, dummy)
		})

		// Staking.initialize
		checkReinit("Staking", func() (*types.Transaction, error) {
			return ctx.Staking.Initialize(opts, dummy, dummy, dummy)
		})

		// Staking.initializeWithValidators
		checkReinit("StakingValidators", func() (*types.Transaction, error) {
			return ctx.Staking.InitializeWithValidators(opts, dummy, dummy, dummy, []common.Address{dummy}, big.NewInt(1))
		})
	})

	t.Run("ReinitializeV2", func(t *testing.T) {
		if ctx.FunderKey == nil {
			t.Fatalf("Funder key not initialized")
		}
		cases := []struct {
			name string
			addr common.Address
			meta *bind.MetaData
		}{
			{name: "Proposal", addr: testctx.ProposalAddr, meta: contracts.ProposalMetaData},
			{name: "Validators", addr: testctx.ValidatorsAddr, meta: contracts.ValidatorsMetaData},
			{name: "Staking", addr: testctx.StakingAddr, meta: contracts.StakingMetaData},
			{name: "Punish", addr: testctx.PunishAddr, meta: contracts.PunishMetaData},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				data := packMethodData(t, tc.meta, "reinitializeV2")
				expectForbiddenSystemTx(t, ctx.FunderKey, tc.addr, data)
			})
		}
	})
}

func pickInTurnValidator(t *testing.T) (*ecdsa.PrivateKey, common.Address, *ethclient.Client) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	validators, err := ctx.Validators.GetActiveValidators(nil)
	if err != nil || len(validators) == 0 {
		t.Fatalf("no active validators available")
	}
	sort.Slice(validators, func(i, j int) bool {
		return bytes.Compare(validators[i][:], validators[j][:]) < 0
	})
	for attempt := 0; attempt < 12; attempt++ {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil || header == nil {
			waitNextBlock()
			continue
		}
		currentValidator, err := ctx.ValidatorAddressBySigner(header.Coinbase)
		if err != nil {
			t.Fatalf("map coinbase signer to validator failed: %v", err)
		}
		start := 0
		for i, v := range validators {
			if v == currentValidator {
				start = (i + 1) % len(validators)
				break
			}
		}
		addr := validators[start]
		key := keyForAddress(addr)
		if key == nil {
			known := make([]string, 0, len(ctx.GenesisValidators))
			for _, k := range ctx.GenesisValidators {
				known = append(known, crypto.PubkeyToAddress(k.PublicKey).Hex())
			}
			t.Fatalf("no key for in-turn validator %s (known=%s)", addr.Hex(), strings.Join(known, ","))
		}
		client := clientForValidator(t, addr)
		if client != nil {
			h2, err := client.HeaderByNumber(context.Background(), nil)
			if err == nil && h2 != nil {
				currentValidator2, err := ctx.ValidatorAddressBySigner(h2.Coinbase)
				if err == nil && currentValidator2 == addr {
					return key, addr, client
				}
			}
		}
		waitNextBlock()
	}
	t.Fatalf("no in-turn validator matched across clients")
	return nil, common.Address{}, nil
}

func clientForValidator(t *testing.T, addr common.Address) *ethclient.Client {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if len(ctx.Clients) > 1 {
		for _, c := range ctx.Clients {
			var cb common.Address
			if err := c.Client().Call(&cb, "eth_coinbase"); err == nil {
				validator, mapErr := ctx.ValidatorAddressBySigner(cb)
				if mapErr == nil && validator == addr {
					return c
				}
			}
		}
	}
	if rpcURL := ctx.ValidatorRPCByValidator(addr); rpcURL != "" {
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			t.Fatalf("failed to dial validator RPC %s: %v", rpcURL, err)
		}
		return client
	}
	if len(ctx.Config.Validators) == 0 {
		t.Fatalf("no validator config available; update test_config.yaml validators list")
	}
	ip := ""
	rpcURL := ""
	for i, v := range ctx.Config.Validators {
		if common.HexToAddress(v.Address) == addr {
			if i < len(ctx.Config.ValidatorRPCs) && ctx.Config.ValidatorRPCs[i] != "" {
				rpcURL = ctx.Config.ValidatorRPCs[i]
			}
			switch i {
			case 0:
				ip = "172.28.0.10"
			case 1:
				ip = "172.28.0.11"
			case 2:
				ip = "172.28.0.12"
			}
		}
	}
	if rpcURL == "" && ip != "" {
		rpcURL = "http://" + ip + ":8545"
	}
	if rpcURL == "" {
		t.Fatalf("no RPC mapping for validator %s; ensure validator_rpcs or validators[0..2] match node0-2", addr.Hex())
	}
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		t.Fatalf("failed to dial validator RPC %s: %v", rpcURL, err)
	}
	return client
}

func validateOnlyMinerCall(t *testing.T, contractAddr common.Address, meta *bind.MetaData) bool {
	if ctx == nil {
		return false
	}
	abi, err := meta.GetAbi()
	if err != nil {
		t.Logf("failed to load ABI: %v", err)
		return false
	}
	data, err := abi.Pack("reinitializeV2")
	if err != nil {
		t.Logf("failed to pack call data: %v", err)
		return false
	}

	minerAddr := common.HexToAddress(ctx.Config.Validators[0].Address)
	blockNum, err := findRecentBlockByCoinbase(ctx.Clients[0], minerAddr, 200)
	if err != nil {
		t.Logf("no recent block for miner %s: %v", minerAddr.Hex(), err)
		return false
	}

	msg := ethereum.CallMsg{
		From: minerAddr,
		To:   &contractAddr,
		Gas:  300000,
		Data: data,
	}
	if _, err := ctx.Clients[0].CallContract(context.Background(), msg, blockNum); err != nil {
		t.Logf("miner call failed: %v", err)
		return false
	}

	nonMiner := common.HexToAddress(ctx.Config.Funder.Address)
	msg.From = nonMiner
	if _, err := ctx.Clients[0].CallContract(context.Background(), msg, blockNum); err == nil {
		t.Log("non-miner call unexpectedly succeeded")
		return false
	}
	return true
}

func findRecentBlockByCoinbase(client *ethclient.Client, coinbase common.Address, lookback uint64) (*big.Int, error) {
	if client == nil {
		return nil, fmt.Errorf("nil client")
	}
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil || header == nil {
		return nil, fmt.Errorf("failed to read header: %v", err)
	}
	start := header.Number.Uint64()
	for i := uint64(0); i <= lookback && start >= i; i++ {
		num := new(big.Int).SetUint64(start - i)
		h, err := client.HeaderByNumber(context.Background(), num)
		if err == nil && h != nil && h.Coinbase == coinbase {
			return num, nil
		}
	}
	return nil, fmt.Errorf("no block found in last %d", lookback)
}
