package tests

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestI_StateRootCheckpoint(t *testing.T) {
	syncNode, validators := pickSyncAndValidators(interopNodes)
	if syncNode == nil {
		t.Skip("no sync node configured in node_rpcs")
	}
	if len(validators) == 0 {
		t.Skip("no validator rpc configured")
	}

	primary := validators[0]
	primaryEth := dialEth(t, primary.URL)
	defer primaryEth.Close()
	primaryRPC := dialRPC(t, primary.URL)
	defer primaryRPC.Close()

	syncEth := dialEth(t, syncNode.URL)
	defer syncEth.Close()
	syncRPC := dialRPC(t, syncNode.URL)
	defer syncRPC.Close()

	start := latestHeight(t, primaryEth)
	target := start + 6
	latest := waitForHeightAtLeast(t, primaryEth, target, 90*time.Second)
	syncLatest := waitForHeightAtLeast(t, syncEth, target, 90*time.Second)
	if syncLatest < target {
		t.Fatalf("sync node did not reach target height: target=%d got=%d", target, syncLatest)
	}
	if latest < target {
		t.Fatalf("validator did not reach target height: target=%d got=%d", target, latest)
	}

	checkpoints, err := parseCheckpointSpec(os.Getenv("INTEROP_CHECKPOINTS"), start, latest)
	if err != nil {
		t.Fatalf("parse checkpoints failed: %v", err)
	}
	if len(checkpoints) == 0 {
		t.Fatalf("no checkpoints resolved")
	}

	for _, cp := range checkpoints {
		vHash, vState := waitForStableHistoricalParity(t, primary.Name, primaryRPC, syncNode.Name, syncRPC, cp, 20*time.Second)
		t.Logf("checkpoint=%d hash=%s stateRoot=%s", cp, vHash, vState)
	}

	t.Logf("stateRoot parity passed: validator=%s sync=%s checkpoints=%s", primary.Name, syncNode.Name, fmt.Sprint(checkpoints))
}
