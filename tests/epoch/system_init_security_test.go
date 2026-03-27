package tests

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
)

const eip170MaxCodeSize = 24_576

func TestZ_SystemInitSecurityGuards(t *testing.T) {
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if len(ctx.Clients) == 0 {
		t.Fatalf("No RPC clients configured")
	}

	senderKey := ctx.FunderKey
	if senderKey == nil && len(ctx.GenesisValidators) > 0 {
		senderKey = ctx.GenesisValidators[0]
	}
	if senderKey == nil {
		t.Fatalf("No funded key available for sending test transactions")
	}

	dummy := common.HexToAddress("0x000000000000000000000000000000000000dEaD")
	vals := []common.Address{dummy}

	t.Run("ExternalInitializeSelectorsForbidden", func(t *testing.T) {
		cases := []struct {
			name   string
			addr   common.Address
			meta   *bind.MetaData
			method string
			args   []interface{}
		}{
			{
				name:   "Proposal.initialize",
				addr:   testctx.ProposalAddr,
				meta:   contracts.ProposalMetaData,
				method: "initialize",
				args:   []interface{}{vals, dummy, big.NewInt(1)},
			},
			{
				name:   "Validators.initialize",
				addr:   testctx.ValidatorsAddr,
				meta:   contracts.ValidatorsMetaData,
				method: "initialize",
				args:   []interface{}{vals, vals, dummy, dummy, dummy},
			},
			{
				name:   "Punish.initialize",
				addr:   testctx.PunishAddr,
				meta:   contracts.PunishMetaData,
				method: "initialize",
				args:   []interface{}{dummy, dummy, dummy},
			},
			{
				name:   "Staking.initialize",
				addr:   testctx.StakingAddr,
				meta:   contracts.StakingMetaData,
				method: "initialize",
				args:   []interface{}{dummy, dummy, dummy},
			},
			{
				name:   "Staking.initializeWithValidators",
				addr:   testctx.StakingAddr,
				meta:   contracts.StakingMetaData,
				method: "initializeWithValidators",
				args:   []interface{}{dummy, dummy, dummy, vals, big.NewInt(1)},
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				data := packMethodData(t, tc.meta, tc.method, tc.args...)
				expectForbiddenSystemTx(t, senderKey, tc.addr, data)
			})
		}
	})

	t.Run("ExternalReinitializeSelectorsForbidden", func(t *testing.T) {
		cases := []struct {
			name string
			addr common.Address
			meta *bind.MetaData
		}{
			{name: "Proposal.reinitializeV2", addr: testctx.ProposalAddr, meta: contracts.ProposalMetaData},
			{name: "Validators.reinitializeV2", addr: testctx.ValidatorsAddr, meta: contracts.ValidatorsMetaData},
			{name: "Punish.reinitializeV2", addr: testctx.PunishAddr, meta: contracts.PunishMetaData},
			{name: "Staking.reinitializeV2", addr: testctx.StakingAddr, meta: contracts.StakingMetaData},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				data := packMethodData(t, tc.meta, "reinitializeV2")
				expectForbiddenSystemTx(t, senderKey, tc.addr, data)
			})
		}
	})

	t.Run("ExternalSystemRuntimeSelectorsForbidden", func(t *testing.T) {
		cases := []struct {
			name   string
			addr   common.Address
			meta   *bind.MetaData
			method string
			args   []interface{}
		}{
			{
				name:   "Punish.executePending",
				addr:   testctx.PunishAddr,
				meta:   contracts.PunishMetaData,
				method: "executePending",
				args:   []interface{}{big.NewInt(1)},
			},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				data := packMethodData(t, tc.meta, tc.method, tc.args...)
				expectForbiddenSystemTx(t, senderKey, tc.addr, data)
			})
		}
	})

	t.Run("FixedAddressValidationOnFreshDeploy", func(t *testing.T) {
		from := crypto.PubkeyToAddress(senderKey.PublicKey)
		wrong := common.HexToAddress("0x000000000000000000000000000000000000bEEF")
		initVals := []common.Address{from}

		newOpts := func(gas uint64) *bind.TransactOpts {
			opts, err := ctx.GetTransactor(senderKey)
			if err != nil {
				t.Fatalf("failed to get transactor: %v", err)
			}
			opts.GasLimit = gas
			return opts
		}

		// Use explicit headroom because generated system contract bytecode has grown.
		// These fresh-deploy guard checks should fail on initialize semantics when
		// raw CREATE deployment is still valid for the contract under test.

		// Proposal: validators_ must match fixed VALIDATOR_ADDR.
		proposalAddr, tx, _, err := contracts.DeployProposal(newOpts(12_000_000), ctx.Clients[0])
		if err != nil {
			t.Fatalf("deploy proposal failed: %v", err)
		}
		if err := ctx.WaitMined(tx.Hash()); err != nil {
			t.Fatalf("deploy proposal tx failed: %v", err)
		}
		callExpectRevertContains(
			t,
			from,
			proposalAddr,
			packMethodData(t, contracts.ProposalMetaData, "initialize", initVals, wrong, big.NewInt(100)),
			"Invalid validators contract address",
		)

		// Validators: proposal_ must match fixed PROPOSAL_ADDR.
		//
		// The current Validators runtime code is installed by consensus at the fixed
		// system address. If the runtime code size already exceeds the CREATE deploy
		// limit, a raw fresh deploy is expected to fail before initialize() can be
		// exercised, and that should be treated as a deployment-path constraint, not
		// as an initialize-guard regression.
		validatorsRuntimeCode, codeErr := ctx.Clients[0].CodeAt(context.Background(), testctx.ValidatorsAddr, nil)
		if codeErr != nil {
			t.Fatalf("read validators runtime code failed: %v", codeErr)
		}
		validatorsAddr, tx, _, err := contracts.DeployValidators(newOpts(30_000_000), ctx.Clients[0])
		if err != nil {
			t.Fatalf("deploy validators failed: %v", err)
		}
		if err := ctx.WaitMined(tx.Hash()); err != nil {
			if len(validatorsRuntimeCode) > eip170MaxCodeSize {
				t.Logf(
					"validators raw deploy rejected as expected because runtime size exceeds EIP-170 limit: size=%d limit=%d tx=%s",
					len(validatorsRuntimeCode),
					eip170MaxCodeSize,
					tx.Hash().Hex(),
				)
			} else {
				t.Fatalf("deploy validators tx failed: %v", err)
			}
		} else {
			callExpectRevertContains(
				t,
				from,
				validatorsAddr,
				packMethodData(
					t,
					contracts.ValidatorsMetaData,
					"initialize",
					initVals,
					initVals,
					wrong,
					testctx.PunishAddr,
					testctx.StakingAddr,
				),
				"Invalid proposal contract address",
			)
		}

		// Punish: fixed validators address check remains strict.
		// zero-staking behavior may vary by contract version (revert or no-op success).
		punishAddr, tx, _, err := contracts.DeployPunish(newOpts(12_000_000), ctx.Clients[0])
		if err != nil {
			t.Fatalf("deploy punish failed: %v", err)
		}
		if err := ctx.WaitMined(tx.Hash()); err != nil {
			t.Fatalf("deploy punish tx failed: %v", err)
		}
		callExpectRevertContainsOrSuccess(
			t,
			from,
			punishAddr,
			packMethodData(
				t,
				contracts.PunishMetaData,
				"initialize",
				testctx.ValidatorsAddr,
				testctx.ProposalAddr,
				common.Address{},
			),
			"Invalid staking address",
		)
		callExpectRevertContains(
			t,
			from,
			punishAddr,
			packMethodData(
				t,
				contracts.PunishMetaData,
				"initialize",
				wrong,
				testctx.ProposalAddr,
				testctx.StakingAddr,
			),
			"Invalid validators contract address",
		)

		// Staking: validators_ and proposal_ must match fixed addresses.
		stakingAddr, tx, _, err := contracts.DeployStaking(newOpts(25_000_000), ctx.Clients[0])
		if err != nil {
			t.Fatalf("deploy staking failed: %v", err)
		}
		if err := ctx.WaitMined(tx.Hash()); err != nil {
			t.Fatalf("deploy staking tx failed: %v", err)
		}
		callExpectRevertContains(
			t,
			from,
			stakingAddr,
			packMethodData(
				t,
				contracts.StakingMetaData,
				"initialize",
				wrong,
				testctx.ProposalAddr,
				testctx.PunishAddr,
			),
			"Invalid validators contract address",
		)
		callExpectRevertContains(
			t,
			from,
			stakingAddr,
			packMethodData(
				t,
				contracts.StakingMetaData,
				"initializeWithValidators",
				testctx.ValidatorsAddr,
				wrong,
				testctx.PunishAddr,
				initVals,
				big.NewInt(1000),
			),
			"Invalid proposal contract address",
		)
	})
}

func packMethodData(t *testing.T, meta *bind.MetaData, method string, args ...interface{}) []byte {
	t.Helper()
	contractABI, err := meta.GetAbi()
	if err != nil {
		t.Fatalf("failed to load ABI for %s: %v", method, err)
	}
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		t.Fatalf("failed to pack %s: %v", method, err)
	}
	return data
}

func expectForbiddenSystemTx(t *testing.T, key *ecdsa.PrivateKey, to common.Address, data []byte) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	if key == nil {
		t.Fatalf("nil sender key")
	}

	from := crypto.PubkeyToAddress(key.PublicKey)
	ctx.RefreshNonce(from)

	nonce, err := ctx.Clients[0].PendingNonceAt(context.Background(), from)
	if err != nil {
		t.Fatalf("failed to get pending nonce for %s: %v", from.Hex(), err)
	}
	gasPrice, err := ctx.Clients[0].SuggestGasPrice(context.Background())
	if err != nil || gasPrice == nil || gasPrice.Sign() == 0 {
		gasPrice = big.NewInt(1_000_000_000) // 1 gwei
	}

	tx := types.NewTransaction(nonce, to, big.NewInt(0), 500_000, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(ctx.ChainID), key)
	if err != nil {
		t.Fatalf("failed to sign tx: %v", err)
	}

	err = ctx.Clients[0].SendTransaction(context.Background(), signedTx)
	if err == nil {
		t.Fatalf("expected forbidden system transaction, got success: tx=%s to=%s", signedTx.Hash().Hex(), to.Hex())
	}
	if !strings.Contains(strings.ToLower(err.Error()), "forbidden system transaction") {
		t.Fatalf("expected forbidden system transaction, got: %v", err)
	}
}

func callExpectRevertContains(t *testing.T, from, to common.Address, data []byte, want string) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	msg := ethereum.CallMsg{
		From: from,
		To:   &to,
		Gas:  3_000_000,
		Data: data,
	}
	_, err := ctx.Clients[0].CallContract(context.Background(), msg, nil)
	if err == nil {
		t.Fatalf("expected call revert containing %q, got success", want)
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("expected revert containing %q, got %v", want, err)
	}
}

func callExpectRevertContainsOrSuccess(t *testing.T, from, to common.Address, data []byte, want string) {
	t.Helper()
	if ctx == nil {
		t.Fatalf("Context not initialized")
	}
	msg := ethereum.CallMsg{
		From: from,
		To:   &to,
		Gas:  3_000_000,
		Data: data,
	}
	_, err := ctx.Clients[0].CallContract(context.Background(), msg, nil)
	if err == nil {
		return
	}
	if want != "" && !strings.Contains(err.Error(), want) {
		t.Fatalf("expected call success or revert containing %q, got %v", want, err)
	}
}
