package tests

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"juchain.org/chain/tools/ci/internal/utils"
)

func TestI_ValidatorExtras(t *testing.T) {
	if ctx == nil || len(ctx.GenesisValidators) == 0 {
		t.Fatalf("Context not initialized")
	}

	valKey := ctx.GenesisValidators[0]
	valAddr := common.HexToAddress(ctx.Config.Validators[0].Address)
	pass, _ := ctx.Proposal.Pass(nil, valAddr)
	if !pass {
		if err := passProposalFor(t, valAddr, "V-02b Auth"); err != nil {
			t.Fatalf("validator not authorized for edit tests: %v", err)
		}
	}

	// Description boundary checks (identity, website, email, details)
	t.Run("V-02b_DescriptionBoundaryFields", func(t *testing.T) {
		opts, _ := ctx.GetTransactor(valKey)

		tooLong := func(n int) string {
			b := make([]byte, n)
			for i := range b {
				b[i] = 'a'
			}
			return string(b)
		}

		// identity > 3000
		_, err := ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", tooLong(3001), "", "", "")
		if err == nil {
			t.Fatal("identity > 3000 should fail")
		}
		// website > 140
		_, err = ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", "", tooLong(141), "", "")
		if err == nil {
			t.Fatal("website > 140 should fail")
		}
		// email > 140
		_, err = ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", "", "", tooLong(141), "")
		if err == nil {
			t.Fatal("email > 140 should fail")
		}
		// details > 280
		_, err = ctx.Validators.CreateOrEditValidator(opts, valAddr, "ok", "", "", "", tooLong(281))
		if err == nil {
			t.Fatal("details > 280 should fail")
		}
	})

	// Withdraw profits exceptions
	t.Run("V-04_WithdrawProfitsExceptions", func(t *testing.T) {
		feeAddr, _, incoming, _, _, _ := ctx.Validators.GetValidatorInfo(nil, valAddr)

		// Non-fee address should fail
		userKey, _, _ := ctx.CreateAndFundAccount(utils.ToWei(1))
		userOpts, _ := ctx.GetTransactor(userKey)
		_, err := ctx.Validators.WithdrawProfits(userOpts, valAddr)
		if err == nil || !strings.Contains(err.Error(), "fee receiver") {
			t.Fatalf("expected non-fee withdrawal to fail, got: %v", err)
		}

		// Ensure fee address is a known key (validator address).
		opts, _ := ctx.GetTransactor(valKey)
		if tx, err := ctx.Validators.CreateOrEditValidator(opts, valAddr, "Genesis", "", "", "", ""); err == nil {
			ctx.WaitMined(tx.Hash())
		} else {
			t.Fatalf("failed to set fee address for zero-profit check: %v", err)
		}
		feeAddr, _, incoming, _, _, _ = ctx.Validators.GetValidatorInfo(nil, valAddr)
		feeKey := keyForAddress(feeAddr)
		if feeKey == nil {
			t.Fatalf("fee address key not available for zero-profit check")
		}
		if incoming.Cmp(big.NewInt(0)) > 0 {
			// Try a single withdraw to clear profits if cooldown allows.
			feeOpts, _ := ctx.GetTransactor(feeKey)
			tx, err := ctx.Validators.WithdrawProfits(feeOpts, valAddr)
			if err == nil {
				ctx.WaitMined(tx.Hash())
				// Immediate second withdraw should fail due to cooldown or no profits.
				feeOpts, _ = ctx.GetTransactor(feeKey)
				_, err = ctx.Validators.WithdrawProfits(feeOpts, valAddr)
				if err == nil {
					t.Fatal("expected withdraw exception after immediate retry, got success")
				}
				if !strings.Contains(err.Error(), "You don't have any profits") && !strings.Contains(err.Error(), "wait enough blocks") {
					t.Fatalf("unexpected withdraw error: %v", err)
				}
				return
			}
			if strings.Contains(err.Error(), "wait enough blocks") {
				// Cooldown not satisfied; acceptable exception path.
				return
			}
			t.Fatalf("cannot withdraw to clear profits: %v", err)
		}

		// No incoming profits; expect an exception (cooldown or zero profits).
		feeOpts, _ := ctx.GetTransactor(feeKey)
		_, err = ctx.Validators.WithdrawProfits(feeOpts, valAddr)
		if err == nil {
			t.Fatal("expected withdraw exception, got success")
		}
		if !strings.Contains(err.Error(), "You don't have any profits") && !strings.Contains(err.Error(), "wait enough blocks") {
			t.Fatalf("unexpected withdraw error: %v", err)
		}
	})
}
