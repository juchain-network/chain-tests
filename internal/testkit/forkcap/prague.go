package forkcap

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/holiman/uint256"
)

type rpcPragueAuthorization struct {
	ChainID string         `json:"chainId"`
	Address common.Address `json:"address"`
	Nonce   string         `json:"nonce"`
	YParity string         `json:"yParity"`
	R       string         `json:"r"`
	S       string         `json:"s"`
}

type rpcPragueTransaction struct {
	Type              string                   `json:"type"`
	Hash              common.Hash              `json:"hash"`
	AuthorizationList []rpcPragueAuthorization `json:"authorizationList"`
}

func buildPragueSetCodeTx(h *Harness) (*types.Transaction, common.Address, common.Address, types.SetCodeAuthorization, error) {
	if h == nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("nil harness")
	}
	funder := h.FunderKey()
	if funder == nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("missing funded signer key")
	}
	chainID := h.ChainID()
	if chainID == nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("missing chain id")
	}
	authKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("generate auth key: %w", err)
	}
	delegateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("generate delegate key: %w", err)
	}
	authority := crypto.PubkeyToAddress(authKey.PublicKey)
	delegate := crypto.PubkeyToAddress(delegateKey.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	nonce, err := h.PendingNonceAt(ctx, crypto.PubkeyToAddress(funder.PublicKey))
	if err != nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("read funder nonce: %w", err)
	}
	gasPrice, err := h.SuggestGasPrice(ctx)
	if err != nil || gasPrice == nil || gasPrice.Sign() <= 0 {
		gasPrice = DefaultFallbackGasPrice()
	}
	auth, err := types.SignSetCode(authKey, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: delegate,
		Nonce:   0,
	})
	if err != nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("sign setcode authorization: %w", err)
	}
	to := common.HexToAddress("0x0000000000000000000000000000000000000022")
	tx := types.NewTx(&types.SetCodeTx{
		ChainID:   uint256.MustFromBig(chainID),
		Nonce:     nonce,
		GasTipCap: uint256.MustFromBig(gasPrice),
		GasFeeCap: uint256.MustFromBig(gasPrice),
		Gas:       80_000,
		To:        to,
		Value:     uint256.NewInt(0),
		AuthList:  []types.SetCodeAuthorization{auth},
	})
	signed, err := types.SignTx(tx, types.NewPragueSigner(chainID), funder)
	if err != nil {
		return nil, common.Address{}, common.Address{}, types.SetCodeAuthorization{}, fmt.Errorf("sign setcode tx: %w", err)
	}
	return signed, authority, delegate, auth, nil
}

func expectPragueSetCodeRejection(err error) error {
	if err == nil {
		return fmt.Errorf("expected pre-Prague SetCodeTx rejection, got success")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "prague") && !strings.Contains(msg, "type 4 rejected") && !strings.Contains(msg, "not supported") {
		return fmt.Errorf("expected pre-Prague SetCodeTx rejection, got %v", err)
	}
	return nil
}

func CheckPragueSetCodeTx(h *Harness, shouldFail bool) error {
	signed, authority, delegate, _, err := buildPragueSetCodeTx(h)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	err = h.SendSignedTransaction(ctx, signed)
	if shouldFail {
		if err := expectPragueSetCodeRejection(err); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("expected post-Prague SetCodeTx acceptance, got %w", err)
	}
	if err := h.WaitMined(signed.Hash()); err != nil {
		return fmt.Errorf("wait setcode tx mined: %w", err)
	}
	receipt, err := h.Receipt(ctx, signed.Hash())
	if err != nil {
		return fmt.Errorf("read setcode tx receipt: %w", err)
	}
	if receipt == nil {
		return fmt.Errorf("nil receipt for setcode tx %s", signed.Hash().Hex())
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("expected post-Prague SetCodeTx success receipt, got status=%d", receipt.Status)
	}
	code, err := h.Client.CodeAt(ctx, authority, nil)
	if err != nil {
		return fmt.Errorf("read delegated code at authority %s: %w", authority.Hex(), err)
	}
	want := types.AddressToDelegation(delegate)
	if string(code) != string(want) {
		return fmt.Errorf("unexpected delegated code at authority %s: got=%x want=%x", authority.Hex(), code, want)
	}
	return nil
}

func CheckPragueAuthorizationRPCSurface(h *Harness, rpcURL string, shouldFail bool) error {
	if h == nil {
		return fmt.Errorf("nil harness")
	}
	signed, _, delegate, auth, err := buildPragueSetCodeTx(h)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	err = h.SendSignedTransaction(ctx, signed)
	if shouldFail {
		if err := expectPragueSetCodeRejection(err); err != nil {
			return err
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("expected post-Prague SetCodeTx acceptance for RPC surface probe, got %w", err)
	}
	if err := h.WaitMined(signed.Hash()); err != nil {
		return fmt.Errorf("wait setcode tx mined for RPC surface probe: %w", err)
	}
	rpcClient, err := rpc.DialContext(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("dial rpc for Prague authorization surface: %w", err)
	}
	defer rpcClient.Close()
	var tx rpcPragueTransaction
	if err := rpcClient.CallContext(ctx, &tx, "eth_getTransactionByHash", signed.Hash()); err != nil {
		return fmt.Errorf("eth_getTransactionByHash for Prague authorization surface: %w", err)
	}
	if tx.Hash != signed.Hash() {
		return fmt.Errorf("unexpected transaction hash in Prague authorization RPC surface: got %s want %s", tx.Hash.Hex(), signed.Hash().Hex())
	}
	if tx.Type != "0x4" {
		return fmt.Errorf("unexpected Prague authorization tx type: got %s want 0x4", tx.Type)
	}
	if len(tx.AuthorizationList) != 1 {
		return fmt.Errorf("unexpected Prague authorizationList length: got %d want 1", len(tx.AuthorizationList))
	}
	item := tx.AuthorizationList[0]
	if item.Address != delegate {
		return fmt.Errorf("unexpected Prague authorization delegate address: got %s want %s", item.Address.Hex(), delegate.Hex())
	}
	if item.ChainID != fmt.Sprintf("0x%x", h.ChainID()) {
		return fmt.Errorf("unexpected Prague authorization chainId: got %s want 0x%x", item.ChainID, h.ChainID())
	}
	if item.Nonce != "0x0" {
		return fmt.Errorf("unexpected Prague authorization nonce: got %s want 0x0", item.Nonce)
	}
	if item.YParity != fmt.Sprintf("0x%x", auth.V) {
		return fmt.Errorf("unexpected Prague authorization yParity: got %s want 0x%x", item.YParity, auth.V)
	}
	if item.R == "" || item.R == "0x0" || item.S == "" || item.S == "0x0" {
		return fmt.Errorf("unexpected Prague authorization signature fields: r=%s s=%s", item.R, item.S)
	}
	return nil
}
