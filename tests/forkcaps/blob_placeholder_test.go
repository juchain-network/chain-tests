package tests

import (
	"testing"

	fc "juchain.org/chain/tools/ci/internal/testkit/forkcap"
)

func TestK_ForkcapCapability_BlobTxDeferred(t *testing.T) {
	requireForkcapSelection(t, "cancun")
	deferred := deferredCapabilityMap()
	cap, ok := deferred["blob_tx_submission"]
	if !ok {
		t.Fatal("expected deferred blob_tx_submission capability in registry")
	}
	t.Skip(fc.DeferredReason(cap))
}
