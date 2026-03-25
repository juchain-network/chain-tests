package tests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"juchain.org/chain/tools/ci/internal/testkit"
)

func TestZ_UpgradeOverrideBootstrapMapping(t *testing.T) {
	if ctx == nil || ctx.Config == nil {
		t.Fatalf("context not initialized")
	}
	if strings.ToLower(strings.TrimSpace(ctx.Config.Fork.Mode)) != "upgrade" {
		t.Skip("fork.mode is not upgrade")
	}

	overrideVals := ctx.Config.Fork.Override.PosaValidators
	overrideSigners := ctx.Config.Fork.Override.PosaSigners
	if len(overrideVals) == 0 && len(overrideSigners) == 0 {
		t.Skip("fork.override.posa_validators/signers not configured")
	}
	if len(overrideVals) != len(overrideSigners) {
		t.Fatalf("override validator/signer length mismatch: %d != %d", len(overrideVals), len(overrideSigners))
	}

	t.Logf("waiting for PoSA migration boundary: scheduled_time=%d override_posa_time=%d pairs=%d", ctx.Config.Fork.ScheduledTime, ctx.Config.Fork.Override.PosaTime, len(overrideVals))

	interval := retrySleep()
	if interval < time.Second {
		interval = time.Second
	}

	err := testkit.WaitUntil(testkit.WaitUntilOptions{
		MaxAttempts: 180,
		Interval:    interval,
		OnRetry: func(attempt int) {
			_ = ctx.WaitForBlockProgress(1, 5*time.Second)
		},
	}, func() (bool, error) {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err != nil {
			return false, err
		}
		if header == nil {
			return false, fmt.Errorf("latest header is nil")
		}
		if ctx.Config.Fork.ScheduledTime > 0 && int64(header.Time) < ctx.Config.Fork.ScheduledTime {
			return false, nil
		}

		for i := range overrideVals {
			validator := common.HexToAddress(overrideVals[i])
			signer := common.HexToAddress(overrideSigners[i])

			gotValidator, err := ctx.Validators.GetValidatorBySigner(nil, signer)
			if err != nil {
				return false, err
			}
			if gotValidator == (common.Address{}) {
				return false, fmt.Errorf("signer %s not mapped yet", signer.Hex())
			}
			if gotValidator != validator {
				return false, fmt.Errorf("signer %s mapped to %s want %s", signer.Hex(), gotValidator.Hex(), validator.Hex())
			}

			gotSigner, err := ctx.Validators.GetValidatorSigner(nil, validator)
			if err != nil {
				return false, err
			}
			if gotSigner != signer {
				return false, fmt.Errorf("validator %s signer %s want %s", validator.Hex(), gotSigner.Hex(), signer.Hex())
			}

			info, err := ctx.Staking.GetValidatorInfo(nil, validator)
			if err != nil {
				return false, err
			}
			if info.SelfStake == nil || info.SelfStake.Sign() <= 0 {
				return false, fmt.Errorf("validator %s self stake not initialized yet", validator.Hex())
			}
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("upgrade override bootstrap mapping not observed: %v", err)
	}

	if err := ctx.WaitForBlockProgress(2, 60*time.Second); err != nil {
		t.Fatalf("chain stalled after upgrade override mapping became active: %v", err)
	}
}

func TestZ_UnderfundedUpgradeDefersMigration(t *testing.T) {
	if os.Getenv("EXPECT_UPGRADE_DEFER") != "1" {
		t.Skip("set EXPECT_UPGRADE_DEFER=1 to run underfunded upgrade defer check")
	}
	if ctx == nil || ctx.Config == nil {
		t.Fatalf("context not initialized")
	}
	if strings.ToLower(strings.TrimSpace(ctx.Config.Fork.Mode)) != "upgrade" {
		t.Skip("fork.mode is not upgrade")
	}
	overrideSigners := ctx.Config.Fork.Override.PosaSigners
	overrideValidators := ctx.Config.Fork.Override.PosaValidators
	if len(overrideSigners) != 1 || len(overrideValidators) != 1 {
		t.Fatalf("expected exactly one override validator/signer pair, got validators=%d signers=%d", len(overrideValidators), len(overrideSigners))
	}
	if ctx.Config.Fork.ScheduledTime <= 0 {
		t.Fatalf("fork scheduled_time must be set for upgrade defer test")
	}

	waitUntil := time.Now().Add(90 * time.Second)
	for {
		header, err := ctx.Clients[0].HeaderByNumber(context.Background(), nil)
		if err == nil && header != nil && int64(header.Time) >= ctx.Config.Fork.ScheduledTime+5 {
			break
		}
		if time.Now().After(waitUntil) {
			t.Fatalf("timed out waiting past scheduled upgrade time=%d", ctx.Config.Fork.ScheduledTime)
		}
		_ = ctx.WaitForBlockProgress(1, 5*time.Second)
	}

	if err := ctx.WaitForBlockProgress(3, 45*time.Second); err != nil {
		t.Fatalf("chain did not stay live after underfunded upgrade boundary: %v", err)
	}

	overrideSigner := common.HexToAddress(overrideSigners[0])
	overrideValidator := common.HexToAddress(overrideValidators[0])
	gotValidator, err := ctx.Validators.GetValidatorBySigner(nil, overrideSigner)
	if err == nil && gotValidator == overrideValidator {
		t.Fatalf("underfunded upgrade unexpectedly activated override mapping: signer=%s validator=%s", overrideSigner.Hex(), gotValidator.Hex())
	}
	t.Logf("underfunded upgrade remained deferred as expected: signer=%s err=%v mapped=%s", overrideSigner.Hex(), err, gotValidator.Hex())
}

func TestZ_OverrideDriftRestartKeepsStoredMapping(t *testing.T) {
	if os.Getenv("EXPECT_OVERRIDE_DRIFT_REJECT") != "1" {
		t.Skip("set EXPECT_OVERRIDE_DRIFT_REJECT=1 to run override drift restart check")
	}
	if ctx == nil || ctx.Config == nil {
		t.Fatalf("context not initialized")
	}
	if strings.ToLower(strings.TrimSpace(ctx.Config.Fork.Mode)) != "upgrade" {
		t.Skip("fork.mode is not upgrade")
	}

	overrideVals := ctx.Config.Fork.Override.PosaValidators
	overrideSigners := ctx.Config.Fork.Override.PosaSigners
	if len(overrideVals) != 1 || len(overrideSigners) != 1 {
		t.Fatalf("expected exactly one stored override validator/signer pair, got validators=%d signers=%d", len(overrideVals), len(overrideSigners))
	}

	driftValidatorHex := strings.TrimSpace(os.Getenv("DRIFT_OVERRIDE_VALIDATOR"))
	driftSignerHex := strings.TrimSpace(os.Getenv("DRIFT_OVERRIDE_SIGNER"))
	if driftValidatorHex == "" || driftSignerHex == "" {
		t.Fatalf("DRIFT_OVERRIDE_VALIDATOR and DRIFT_OVERRIDE_SIGNER must be set")
	}

	if err := ctx.WaitForBlockProgress(1, 60*time.Second); err != nil {
		t.Fatalf("chain did not stay live after drifted restart: %v", err)
	}

	storedValidator := common.HexToAddress(overrideVals[0])
	storedSigner := common.HexToAddress(overrideSigners[0])
	gotValidator, err := ctx.Validators.GetValidatorBySigner(nil, storedSigner)
	if err != nil {
		t.Fatalf("read stored signer mapping failed: %v", err)
	}
	if gotValidator != storedValidator {
		t.Fatalf("stored signer mapping drifted after restart: got=%s want=%s", gotValidator.Hex(), storedValidator.Hex())
	}
	gotSigner, err := ctx.Validators.GetValidatorSigner(nil, storedValidator)
	if err != nil {
		t.Fatalf("read stored validator signer failed: %v", err)
	}
	if gotSigner != storedSigner {
		t.Fatalf("stored validator signer drifted after restart: got=%s want=%s", gotSigner.Hex(), storedSigner.Hex())
	}

	driftValidator := common.HexToAddress(driftValidatorHex)
	driftSigner := common.HexToAddress(driftSignerHex)
	driftMappedValidator, err := ctx.Validators.GetValidatorBySigner(nil, driftSigner)
	if err == nil && driftMappedValidator == driftValidator {
		t.Fatalf("drifted restart unexpectedly activated drift override mapping: signer=%s validator=%s", driftSigner.Hex(), driftMappedValidator.Hex())
	}
}
