package tests

import (
	"crypto/ecdsa"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"

	"juchain.org/chain/tools/ci/internal/config"
)

var (
	cfg        *config.Config
	funderKey  *ecdsa.PrivateKey
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

	key, err := crypto.HexToECDSA(loadedCfg.Funder.PrivateKey)
	if err != nil {
		log.Error("Invalid funder private_key", "err", err)
		os.Exit(1)
	}
	derived := crypto.PubkeyToAddress(key.PublicKey).Hex()
	if !strings.EqualFold(derived, loadedCfg.Funder.Address) {
		log.Error("Funder address does not match private_key", "derived", derived, "config", loadedCfg.Funder.Address)
		os.Exit(1)
	}

	cfg = loadedCfg
	funderKey = key
	os.Exit(m.Run())
}
