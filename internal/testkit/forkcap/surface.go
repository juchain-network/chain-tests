package forkcap

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"

	"juchain.org/chain/tools/ci/contracts"
	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/config"
	basekit "juchain.org/chain/tools/ci/internal/testkit"
)

const (
	BPO1BlobTarget                = uint64(10)
	BPO1BlobMax                   = uint64(15)
	BPO1BlobBaseFeeUpdateFraction = uint64(8346193)
	BPO2BlobTarget                = uint64(14)
	BPO2BlobMax                   = uint64(21)
	BPO2BlobBaseFeeUpdateFraction = uint64(11684671)
)

type rpcBlobConfig struct {
	Target         uint64 `json:"target"`
	Max            uint64 `json:"max"`
	UpdateFraction uint64 `json:"baseFeeUpdateFraction"`
}

type rpcConfig struct {
	ActivationTime  uint64                    `json:"activationTime"`
	BlobSchedule    *rpcBlobConfig            `json:"blobSchedule"`
	Precompiles     map[string]common.Address `json:"precompiles"`
	SystemContracts map[string]common.Address `json:"systemContracts"`
}

type rpcConfigResponse struct {
	Current *rpcConfig `json:"current"`
	Next    *rpcConfig `json:"next"`
	Last    *rpcConfig `json:"last"`
}

func RuntimeImpl(cfg *config.Config) string {
	return basekit.RuntimeImplForNode(cfg, 0)
}

func CheckForkRPCSurface(cfg *config.Config, rpcURL string) error {
	return basekit.VerifyForkRPCSurface(cfg, rpcURL, RuntimeImpl(cfg))
}

func CheckFixHeaderSurface(cfg *config.Config, rpcURL string) error {
	if err := CheckForkRPCSurface(cfg, rpcURL); err != nil {
		return err
	}
	return verifyLatestBlockBaseFee(cfg, rpcURL)
}

func CheckBPOBlobSchedule(cfg *config.Config, rpcURL string, fork string) error {
	block, err := LatestBlockFieldMap(rpcURL)
	if err != nil {
		return fmt.Errorf("read latest block for %s blob schedule: %w", fork, err)
	}
	if !FieldPresent(block, "blobGasUsed") || !FieldPresent(block, "excessBlobGas") {
		return fmt.Errorf("missing Cancun+ blob fields while checking %s blob schedule", fork)
	}
	switch NormalizeFork(fork) {
	case "bpo1":
		return basekit.VerifyBlobScheduleForDebug(rpcURL, BPO1BlobTarget, BPO1BlobMax, BPO1BlobBaseFeeUpdateFraction)
	case "bpo2":
		return basekit.VerifyBlobScheduleForDebug(rpcURL, BPO2BlobTarget, BPO2BlobMax, BPO2BlobBaseFeeUpdateFraction)
	default:
		return fmt.Errorf("unsupported bpo blob schedule fork %q", fork)
	}
}

func CheckBPOEthConfigTransitionSurface(rpcURL string, fork string, shouldFail bool) error {
	resp, err := ethConfig(rpcURL)
	if err != nil {
		return fmt.Errorf("eth_config for %s transition surface failed: %w", fork, err)
	}
	expectBlob := func(name string, got *rpcBlobConfig, target, max, updateFraction uint64) error {
		if got == nil {
			return fmt.Errorf("%s blobSchedule is missing", name)
		}
		if got.Target != target || got.Max != max || got.UpdateFraction != updateFraction {
			return fmt.Errorf("unexpected %s blobSchedule: got target=%d max=%d updateFraction=%d want target=%d max=%d updateFraction=%d", name, got.Target, got.Max, got.UpdateFraction, target, max, updateFraction)
		}
		return nil
	}
	switch NormalizeFork(fork) {
	case "bpo1":
		if shouldFail {
			if err := expectBlob("current", resp.Current.BlobSchedule, 6, 9, 5007716); err != nil {
				return err
			}
			if resp.Next != nil || resp.Last != nil {
				return fmt.Errorf("expected pre-BPO1 eth_config next/last to be nil in static smoke mode, got next=%v last=%v", resp.Next != nil, resp.Last != nil)
			}
			return nil
		}
		if err := expectBlob("current", resp.Current.BlobSchedule, BPO1BlobTarget, BPO1BlobMax, BPO1BlobBaseFeeUpdateFraction); err != nil {
			return err
		}
		if resp.Next != nil || resp.Last != nil {
			return fmt.Errorf("expected post-BPO1 eth_config next/last to be nil in static smoke mode, got next=%v last=%v", resp.Next != nil, resp.Last != nil)
		}
		return nil
	case "bpo2":
		if shouldFail {
			if err := expectBlob("current", resp.Current.BlobSchedule, BPO1BlobTarget, BPO1BlobMax, BPO1BlobBaseFeeUpdateFraction); err != nil {
				return err
			}
			if resp.Next != nil || resp.Last != nil {
				return fmt.Errorf("expected pre-BPO2 eth_config next/last to be nil in static smoke mode, got next=%v last=%v", resp.Next != nil, resp.Last != nil)
			}
			return nil
		}
		if err := expectBlob("current", resp.Current.BlobSchedule, BPO2BlobTarget, BPO2BlobMax, BPO2BlobBaseFeeUpdateFraction); err != nil {
			return err
		}
		if resp.Next != nil || resp.Last != nil {
			return fmt.Errorf("expected post-BPO2 eth_config next/last to be nil in static smoke mode, got next=%v last=%v", resp.Next != nil, resp.Last != nil)
		}
		return nil
	default:
		return fmt.Errorf("unsupported bpo transition surface fork %q", fork)
	}
}

func CheckPosaContractSurface(h *Harness, shouldFail bool) error {
	if h == nil || h.Client == nil {
		return fmt.Errorf("nil harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addresses := map[string]common.Address{
		"validators": testctx.ValidatorsAddr,
		"punish":     testctx.PunishAddr,
		"proposal":   testctx.ProposalAddr,
		"staking":    testctx.StakingAddr,
	}
	for name, addr := range addresses {
		code, err := h.Client.CodeAt(ctx, addr, nil)
		if err != nil {
			return fmt.Errorf("read %s code at %s: %w", name, addr.Hex(), err)
		}
		if shouldFail {
			if len(code) != 0 {
				return fmt.Errorf("expected pre-PoSA %s contract at %s to be absent, got code len=%d", name, addr.Hex(), len(code))
			}
			continue
		}
		if len(code) == 0 {
			return fmt.Errorf("expected post-PoSA %s contract at %s to be deployed", name, addr.Hex())
		}
	}
	return nil
}

func CheckPosaProposalWiringSurface(h *Harness, shouldFail bool) error {
	if h == nil || h.Client == nil {
		return fmt.Errorf("nil harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	code, err := h.Client.CodeAt(ctx, testctx.ProposalAddr, nil)
	if err != nil {
		return fmt.Errorf("read proposal code at %s: %w", testctx.ProposalAddr.Hex(), err)
	}
	if shouldFail {
		if len(code) != 0 {
			return fmt.Errorf("expected pre-PoSA proposal contract at %s to be absent, got code len=%d", testctx.ProposalAddr.Hex(), len(code))
		}
		return nil
	}
	if len(code) == 0 {
		return fmt.Errorf("expected post-PoSA proposal contract at %s to be deployed", testctx.ProposalAddr.Hex())
	}
	proposal, err := contracts.NewProposal(testctx.ProposalAddr, h.Client)
	if err != nil {
		return fmt.Errorf("bind proposal contract: %w", err)
	}
	initialized, err := proposal.Initialized(nil)
	if err != nil {
		return fmt.Errorf("read proposal initialized: %w", err)
	}
	if !initialized {
		return fmt.Errorf("expected post-PoSA proposal contract to be initialized")
	}
	validatorAddr, err := proposal.VALIDATORADDR(nil)
	if err != nil {
		return fmt.Errorf("read proposal validator addr: %w", err)
	}
	if validatorAddr != testctx.ValidatorsAddr {
		return fmt.Errorf("unexpected proposal validator addr: got %s want %s", validatorAddr.Hex(), testctx.ValidatorsAddr.Hex())
	}
	punishAddr, err := proposal.PUNISHADDR(nil)
	if err != nil {
		return fmt.Errorf("read proposal punish addr: %w", err)
	}
	if punishAddr != testctx.PunishAddr {
		return fmt.Errorf("unexpected proposal punish addr: got %s want %s", punishAddr.Hex(), testctx.PunishAddr.Hex())
	}
	stakingAddr, err := proposal.STAKINGADDR(nil)
	if err != nil {
		return fmt.Errorf("read proposal staking addr: %w", err)
	}
	if stakingAddr != testctx.StakingAddr {
		return fmt.Errorf("unexpected proposal staking addr: got %s want %s", stakingAddr.Hex(), testctx.StakingAddr.Hex())
	}
	return nil
}

func CheckPosaProposalParamsSurface(h *Harness, cfg *config.Config, shouldFail bool) error {
	if h == nil || h.Client == nil {
		return fmt.Errorf("nil harness")
	}
	if cfg == nil {
		return fmt.Errorf("missing config")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	code, err := h.Client.CodeAt(ctx, testctx.ProposalAddr, nil)
	if err != nil {
		return fmt.Errorf("read proposal code at %s: %w", testctx.ProposalAddr.Hex(), err)
	}
	if shouldFail {
		if len(code) != 0 {
			return fmt.Errorf("expected pre-PoSA proposal contract at %s to be absent, got code len=%d", testctx.ProposalAddr.Hex(), len(code))
		}
		return nil
	}
	if len(code) == 0 {
		return fmt.Errorf("expected post-PoSA proposal contract at %s to be deployed", testctx.ProposalAddr.Hex())
	}
	proposal, err := contracts.NewProposal(testctx.ProposalAddr, h.Client)
	if err != nil {
		return fmt.Errorf("bind proposal contract: %w", err)
	}
	expect := func(name string, got *big.Int, want int64) error {
		if got == nil {
			return fmt.Errorf("%s is nil", name)
		}
		wantBig := big.NewInt(want)
		if got.Cmp(wantBig) != 0 {
			return fmt.Errorf("unexpected %s: got %s want %s", name, got.String(), wantBig.String())
		}
		return nil
	}
	expectBig := func(name string, got *big.Int, want *big.Int) error {
		if got == nil {
			return fmt.Errorf("%s is nil", name)
		}
		if want == nil {
			return fmt.Errorf("%s expected value is nil", name)
		}
		if got.Cmp(want) != 0 {
			return fmt.Errorf("unexpected %s: got %s want %s", name, got.String(), want.String())
		}
		return nil
	}
	epoch, err := proposal.Epoch(nil)
	if err != nil {
		return fmt.Errorf("read proposal epoch: %w", err)
	}
	if err := expectBig("epoch", epoch, new(big.Int).SetUint64(cfg.Network.Epoch)); err != nil {
		return err
	}
	proposalCooldown, err := proposal.ProposalCooldown(nil)
	if err != nil {
		return fmt.Errorf("read proposalCooldown: %w", err)
	}
	if err := expect("proposalCooldown", proposalCooldown, cfg.Test.Params.ProposalCooldown); err != nil {
		return err
	}
	proposalLasting, err := proposal.ProposalLastingPeriod(nil)
	if err != nil {
		return fmt.Errorf("read proposalLastingPeriod: %w", err)
	}
	if err := expect("proposalLastingPeriod", proposalLasting, cfg.Test.Params.ProposalLasting); err != nil {
		return err
	}
	unbondingPeriod, err := proposal.UnbondingPeriod(nil)
	if err != nil {
		return fmt.Errorf("read unbondingPeriod: %w", err)
	}
	if err := expect("unbondingPeriod", unbondingPeriod, cfg.Test.Params.UnbondingPeriod); err != nil {
		return err
	}
	validatorUnjailPeriod, err := proposal.ValidatorUnjailPeriod(nil)
	if err != nil {
		return fmt.Errorf("read validatorUnjailPeriod: %w", err)
	}
	if err := expect("validatorUnjailPeriod", validatorUnjailPeriod, cfg.Test.Params.ValidatorUnjail); err != nil {
		return err
	}
	withdrawProfitPeriod, err := proposal.WithdrawProfitPeriod(nil)
	if err != nil {
		return fmt.Errorf("read withdrawProfitPeriod: %w", err)
	}
	if err := expect("withdrawProfitPeriod", withdrawProfitPeriod, cfg.Test.Params.WithdrawProfit); err != nil {
		return err
	}
	commissionUpdateCooldown, err := proposal.CommissionUpdateCooldown(nil)
	if err != nil {
		return fmt.Errorf("read commissionUpdateCooldown: %w", err)
	}
	if err := expect("commissionUpdateCooldown", commissionUpdateCooldown, cfg.Test.Params.CommissionCooldown); err != nil {
		return err
	}
	minValidatorStake, err := proposal.MinValidatorStake(nil)
	if err != nil {
		return fmt.Errorf("read minValidatorStake: %w", err)
	}
	if err := expectBig("minValidatorStake", minValidatorStake, big.NewInt(1_000_000_000_000_000_000)); err != nil {
		return err
	}
	minDelegation, err := proposal.MinDelegation(nil)
	if err != nil {
		return fmt.Errorf("read minDelegation: %w", err)
	}
	if err := expectBig("minDelegation", minDelegation, big.NewInt(1_000_000_000_000_000_000)); err != nil {
		return err
	}
	return nil
}

func CheckPragueEthConfigPrecompileSurface(rpcURL string, shouldFail bool) error {
	resp, err := ethConfig(rpcURL)
	if err != nil {
		return fmt.Errorf("eth_config for Prague precompiles failed: %w", err)
	}
	if resp.Current == nil {
		return fmt.Errorf("eth_config current config is missing")
	}
	precompiles := resp.Current.Precompiles
	expected := map[string]common.Address{
		"BLS12_G1ADD":         common.BytesToAddress([]byte{0x0b}),
		"BLS12_G1MSM":         common.BytesToAddress([]byte{0x0c}),
		"BLS12_G2ADD":         common.BytesToAddress([]byte{0x0d}),
		"BLS12_G2MSM":         common.BytesToAddress([]byte{0x0e}),
		"BLS12_PAIRING_CHECK": common.BytesToAddress([]byte{0x0f}),
		"BLS12_MAP_FP_TO_G1":  common.BytesToAddress([]byte{0x10}),
		"BLS12_MAP_FP2_TO_G2": common.BytesToAddress([]byte{0x11}),
	}
	for name, addr := range expected {
		got, ok := precompiles[name]
		if shouldFail {
			if ok {
				return fmt.Errorf("expected pre-Prague eth_config.current.precompiles to omit %s, got %s", name, got.Hex())
			}
			continue
		}
		if !ok {
			return fmt.Errorf("expected post-Prague eth_config.current.precompiles to include %s", name)
		}
		if got != addr {
			return fmt.Errorf("unexpected post-Prague precompile %s: got %s want %s", name, got.Hex(), addr.Hex())
		}
	}
	return nil
}

func CheckOsakaEthConfigPrecompileSurface(rpcURL string, shouldFail bool) error {
	resp, err := ethConfig(rpcURL)
	if err != nil {
		return fmt.Errorf("eth_config for Osaka precompiles failed: %w", err)
	}
	if resp.Current == nil {
		return fmt.Errorf("eth_config current config is missing")
	}
	precompiles := resp.Current.Precompiles
	addr := P256VerifyPrecompileAddress()
	got, ok := precompiles["P256VERIFY"]
	if shouldFail {
		if ok {
			return fmt.Errorf("expected pre-Osaka eth_config.current.precompiles to omit P256VERIFY, got %s", got.Hex())
		}
		return nil
	}
	if !ok {
		return fmt.Errorf("expected post-Osaka eth_config.current.precompiles to include P256VERIFY")
	}
	if got != addr {
		return fmt.Errorf("unexpected post-Osaka P256VERIFY precompile address: got %s want %s", got.Hex(), addr.Hex())
	}
	return nil
}

func CheckOsakaModexpGasSemantics(h *Harness, shouldFail bool) error {
	if h == nil || h.Client == nil {
		return fmt.Errorf("nil harness")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	addr := common.BytesToAddress([]byte{0x05})
	lowGasMsg := ethereum.CallMsg{To: &addr, Gas: 21_300, Data: []byte{}}
	lowOut, lowErr := h.CallContract(ctx, lowGasMsg)
	if shouldFail {
		if lowErr != nil {
			return fmt.Errorf("expected pre-Osaka MODEXP call with 21300 gas to succeed, got %w", lowErr)
		}
		if len(lowOut) != 0 {
			return fmt.Errorf("expected pre-Osaka MODEXP empty-input output, got %x", lowOut)
		}
		return nil
	}
	if lowErr == nil {
		return fmt.Errorf("expected post-Osaka MODEXP call with 21300 gas to fail due to higher gas requirement")
	}
	lowMsg := strings.ToLower(lowErr.Error())
	if !strings.Contains(lowMsg, "out of gas") && !strings.Contains(lowMsg, "intrinsic gas too low") && !strings.Contains(lowMsg, "gas") {
		return fmt.Errorf("expected post-Osaka MODEXP low-gas failure, got %v", lowErr)
	}
	highGasMsg := ethereum.CallMsg{To: &addr, Gas: 21_600, Data: []byte{}}
	highOut, highErr := h.CallContract(ctx, highGasMsg)
	if highErr != nil {
		return fmt.Errorf("expected post-Osaka MODEXP call with 21600 gas to succeed, got %w", highErr)
	}
	if len(highOut) != 0 {
		return fmt.Errorf("expected post-Osaka MODEXP empty-input output, got %x", highOut)
	}
	return nil
}

func CheckOsakaTxGasCap(h *Harness) error {
	if h == nil {
		return fmt.Errorf("nil harness")
	}
	return basekit.VerifyOsakaTxGasCap(h.Client, h.FunderKey(), h.ChainID())
}

func CheckOsakaTxGasCapInactive(h *Harness) error {
	if h == nil {
		return fmt.Errorf("nil harness")
	}
	key := h.FunderKey()
	if key == nil {
		return fmt.Errorf("missing funded signer key")
	}
	from := crypto.PubkeyToAddress(key.PublicKey)
	tx, err := h.NewLegacyTx(key, &from, nil, basekit.OsakaMaxTxGas+1, nil)
	if err != nil {
		return fmt.Errorf("build pre-Osaka oversized tx: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	if err := h.SendSignedTransaction(ctx, tx); err != nil {
		return fmt.Errorf("expected oversized tx to remain valid pre-Osaka, got %w", err)
	}
	if err := h.WaitMined(tx.Hash()); err != nil {
		return fmt.Errorf("wait pre-Osaka oversized tx mined: %w", err)
	}
	receipt, err := h.Receipt(ctx, tx.Hash())
	if err != nil {
		return fmt.Errorf("read pre-Osaka oversized tx receipt: %w", err)
	}
	if receipt == nil {
		return fmt.Errorf("nil receipt for pre-Osaka oversized tx %s", tx.Hash().Hex())
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("pre-Osaka oversized tx failed: hash=%s status=%d to=%s", tx.Hash().Hex(), receipt.Status, from.Hex())
	}
	return nil
}

func LatestBlockFieldMap(rpcURL string) (map[string]any, error) {
	rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
	if err != nil {
		return nil, err
	}
	defer rpcClient.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var block map[string]any
	if err := rpcClient.CallContext(ctx, &block, "eth_getBlockByNumber", "latest", false); err != nil {
		return nil, err
	}
	if block == nil {
		return nil, fmt.Errorf("latest block response is nil")
	}
	return block, nil
}

func FieldPresent(block map[string]any, field string) bool {
	value, exists := block[field]
	if !exists || value == nil {
		return false
	}
	if raw, ok := value.(string); ok {
		trimmed := strings.TrimSpace(raw)
		return trimmed != "" && !strings.EqualFold(trimmed, "null")
	}
	return true
}

func ethConfig(rpcURL string) (*rpcConfigResponse, error) {
	rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
	if err != nil {
		return nil, err
	}
	defer rpcClient.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var resp rpcConfigResponse
	if err := rpcClient.CallContext(ctx, &resp, "eth_config"); err != nil {
		return nil, err
	}
	return &resp, nil
}

func verifyLatestBlockBaseFee(cfg *config.Config, rpcURL string) error {
	if cfg == nil {
		return fmt.Errorf("missing config for fixheader basefee verification")
	}
	rpcClient, err := rpc.DialContext(context.Background(), rpcURL)
	if err != nil {
		return err
	}
	defer rpcClient.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var latest rpcBlockHeader
	if err := rpcClient.CallContext(ctx, &latest, "eth_getBlockByNumber", "latest", false); err != nil {
		return fmt.Errorf("read latest block for basefee verification: %w", err)
	}
	if latest.Number == nil || latest.BaseFeePerGas == nil {
		return fmt.Errorf("latest block missing number or baseFeePerGas")
	}
	if latest.Number.ToInt().Sign() == 0 {
		return fmt.Errorf("cannot verify fixheader basefee on genesis block")
	}
	parentNumber := new(big.Int).Sub(latest.Number.ToInt(), big.NewInt(1))
	var parent rpcBlockHeader
	if err := rpcClient.CallContext(ctx, &parent, "eth_getBlockByNumber", hexutil.EncodeBig(parentNumber), false); err != nil {
		return fmt.Errorf("read parent block for basefee verification: %w", err)
	}
	if parent.Number == nil {
		return fmt.Errorf("parent block missing number")
	}
	if parent.BaseFeePerGas == nil {
		return fmt.Errorf("parent block missing baseFeePerGas")
	}
	parentHeader := &types.Header{
		Number:   parent.Number.ToInt(),
		GasLimit: uint64(parent.GasLimit),
		GasUsed:  uint64(parent.GasUsed),
		BaseFee:  parent.BaseFeePerGas.ToInt(),
	}
	chainCfg := &params.ChainConfig{ChainID: big.NewInt(1), LondonBlock: big.NewInt(0)}
	want := eip1559.CalcBaseFee(chainCfg, parentHeader)
	have := latest.BaseFeePerGas.ToInt()
	if have.Cmp(want) != 0 {
		return fmt.Errorf("invalid post-FixHeader baseFeePerGas: have %s want %s", have, want)
	}
	return nil
}

type rpcBlockHeader struct {
	Number        *hexutil.Big   `json:"number"`
	GasLimit      hexutil.Uint64 `json:"gasLimit"`
	GasUsed       hexutil.Uint64 `json:"gasUsed"`
	BaseFeePerGas *hexutil.Big   `json:"baseFeePerGas"`
}
