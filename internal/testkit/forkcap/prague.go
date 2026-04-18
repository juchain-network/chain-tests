package forkcap

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

func CheckPragueSetCodeTx(h *Harness, shouldFail bool) error {
	if h == nil {
		return fmt.Errorf("nil harness")
	}
	funder := h.FunderKey()
	if funder == nil {
		return fmt.Errorf("missing funded signer key")
	}
	chainID := h.ChainID()
	if chainID == nil {
		return fmt.Errorf("missing chain id")
	}
	authKey, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate auth key: %w", err)
	}
	delegateKey, err := crypto.GenerateKey()
	if err != nil {
		return fmt.Errorf("generate delegate key: %w", err)
	}
	authority := crypto.PubkeyToAddress(authKey.PublicKey)
	delegate := crypto.PubkeyToAddress(delegateKey.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	nonce, err := h.PendingNonceAt(ctx, crypto.PubkeyToAddress(funder.PublicKey))
	if err != nil {
		return fmt.Errorf("read funder nonce: %w", err)
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
		return fmt.Errorf("sign setcode authorization: %w", err)
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
		return fmt.Errorf("sign setcode tx: %w", err)
	}
	err = h.SendSignedTransaction(ctx, signed)
	if shouldFail {
		if err == nil {
			return fmt.Errorf("expected pre-Prague SetCodeTx rejection, got tx accepted: %s", signed.Hash().Hex())
		}
		msg := strings.ToLower(err.Error())
		if !strings.Contains(msg, "prague") && !strings.Contains(msg, "type 4 rejected") && !strings.Contains(msg, "not supported") {
			return fmt.Errorf("expected pre-Prague SetCodeTx rejection, got %v", err)
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
