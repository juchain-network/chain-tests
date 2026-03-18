package tests

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"juchain.org/chain/tools/ci/internal/config"
	testctx "juchain.org/chain/tools/ci/internal/context"
)

var (
	ctx        *testctx.CIContext
	cfg        *config.Config
	configPath = flag.String("config", "../../data/test_config.yaml", "Path to generated test configuration file")
)

func TestMain(m *testing.M) {
	flag.Parse()

	log.SetDefault(log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, log.LevelInfo, true)))

	loadedCfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Error("Failed to load config", "err", err)
		os.Exit(1)
	}
	if len(loadedCfg.RPCs) == 0 {
		log.Error("No RPCs configured in test config", "config", *configPath)
		os.Exit(1)
	}
	if loadedCfg.Funder.PrivateKey == "" || loadedCfg.Funder.Address == "" {
		log.Error("Funder config missing address or private_key", "config", *configPath)
		os.Exit(1)
	}
	if funderKey, err := crypto.HexToECDSA(loadedCfg.Funder.PrivateKey); err == nil {
		derived := crypto.PubkeyToAddress(funderKey.PublicKey).Hex()
		if !strings.EqualFold(derived, loadedCfg.Funder.Address) {
			log.Error("Funder address does not match private_key", "derived", derived, "config", loadedCfg.Funder.Address)
			os.Exit(1)
		}
	} else {
		log.Error("Invalid funder private_key", "err", err)
		os.Exit(1)
	}

	c, err := testctx.NewCIContext(loadedCfg)
	if err != nil {
		log.Error("Failed to init context", "err", err)
		os.Exit(1)
	}

	cfg = loadedCfg
	ctx = c

	os.Exit(m.Run())
}
