package forkcap

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func BuildCallMsg(key *ecdsa.PrivateKey, to common.Address, data []byte, gas uint64) ethereum.CallMsg {
	from := common.Address{}
	if key != nil {
		from = crypto.PubkeyToAddress(key.PublicKey)
	}
	if gas == 0 {
		gas = DefaultCallGas
	}
	return ethereum.CallMsg{From: from, To: &to, Gas: gas, Data: data}
}

func (h *Harness) DeployRawContract(bytecode []byte, gasLimit uint64) (common.Address, *types.Receipt, error) {
	if len(bytecode) == 0 {
		return common.Address{}, nil, fmt.Errorf("empty deployment bytecode")
	}
	key := h.FunderKey()
	if key == nil {
		return common.Address{}, nil, fmt.Errorf("missing funded signer key")
	}
	if gasLimit == 0 {
		gasLimit = 1_000_000
	}
	tx, err := h.NewLegacyTx(key, nil, nil, gasLimit, bytecode)
	if err != nil {
		return common.Address{}, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTxTimeout)
	defer cancel()
	if err := h.SendSignedTransaction(ctx, tx); err != nil {
		return common.Address{}, nil, err
	}
	if err := h.WaitMined(tx.Hash()); err != nil {
		return common.Address{}, nil, err
	}
	receipt, err := h.Receipt(ctx, tx.Hash())
	if err != nil {
		return common.Address{}, nil, err
	}
	if receipt == nil {
		return common.Address{}, nil, fmt.Errorf("nil receipt for deploy tx %s", tx.Hash().Hex())
	}
	if receipt.ContractAddress == (common.Address{}) {
		return common.Address{}, receipt, fmt.Errorf("missing contract address in deploy receipt %s", tx.Hash().Hex())
	}
	return receipt.ContractAddress, receipt, nil
}

func (h *Harness) Call(to common.Address, data []byte) ([]byte, error) {
	key := h.FunderKey()
	if key == nil {
		return nil, fmt.Errorf("missing funded signer key")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	msg := BuildCallMsg(key, to, data, DefaultCallGas)
	return h.CallContract(ctx, msg)
}
