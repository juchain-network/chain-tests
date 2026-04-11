package testkit

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	testctx "juchain.org/chain/tools/ci/internal/context"
	"juchain.org/chain/tools/ci/internal/utils"
)

type SignerRotation struct {
	Validator      common.Address
	OldSigner      common.Address
	NewSigner      common.Address
	NewSignerKey   *ecdsa.PrivateKey
	EffectiveBlock uint64
}

func IsSingleValidatorSeparatedMode(c *testctx.CIContext) bool {
	if c == nil || c.Config == nil || len(c.Config.Validators) != 1 {
		return false
	}
	validator := strings.TrimSpace(c.Config.Validators[0].Address)
	signer := strings.TrimSpace(c.Config.Validators[0].SignerAddress)
	return validator != "" && signer != "" && !strings.EqualFold(validator, signer)
}

func IsMultiValidatorSeparatedMode(c *testctx.CIContext, minValidators int) bool {
	if c == nil || c.Config == nil || len(c.Config.Validators) < minValidators {
		return false
	}
	for _, validator := range c.Config.Validators {
		if strings.TrimSpace(validator.Address) == "" || strings.TrimSpace(validator.SignerAddress) == "" {
			return false
		}
		if strings.EqualFold(strings.TrimSpace(validator.Address), strings.TrimSpace(validator.SignerAddress)) {
			return false
		}
	}
	return true
}

func ParseHeaderExtraSigners(extra []byte) ([]common.Address, error) {
	if len(extra) < 32+65 {
		return nil, fmt.Errorf("header extra too short: %d", len(extra))
	}
	payload := extra[32 : len(extra)-65]
	if len(payload)%common.AddressLength != 0 {
		return nil, fmt.Errorf("invalid signer payload length: %d", len(payload))
	}
	signers := make([]common.Address, 0, len(payload)/common.AddressLength)
	for i := 0; i < len(payload); i += common.AddressLength {
		signers = append(signers, common.BytesToAddress(payload[i:i+common.AddressLength]))
	}
	return signers, nil
}

func PrepareValidatorSignerRotation(c *testctx.CIContext, validator common.Address) (*SignerRotation, *types.Receipt, error) {
	if c == nil || c.Config == nil {
		return nil, nil, fmt.Errorf("context not initialized")
	}
	validatorKey := c.ValidatorKeyByAddress(validator)
	if validatorKey == nil {
		return nil, nil, fmt.Errorf("validator key not found for %s", validator.Hex())
	}
	oldSigner, err := c.Validators.GetValidatorSigner(nil, validator)
	if err != nil {
		return nil, nil, fmt.Errorf("read current validator signer failed: %w", err)
	}
	feeAddr, err := c.FeeAddressByValidator(validator)
	if err != nil {
		return nil, nil, fmt.Errorf("read fee address failed: %w", err)
	}

	newSignerKey, newSigner, err := c.CreateAndFundAccount(utils.ToWei(10))
	if err != nil {
		return nil, nil, fmt.Errorf("create rotation signer failed: %w", err)
	}

	opts, err := c.GetTransactor(validatorKey)
	if err != nil {
		return nil, nil, fmt.Errorf("get validator transactor failed: %w", err)
	}
	opts.GasLimit = 1_500_000
	tx, err := c.Validators.CreateOrEditValidator(opts, feeAddr, newSigner, "checkpoint-rotation", "", "", "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("schedule signer rotation failed: %w", err)
	}
	if err := c.WaitMined(tx.Hash()); err != nil {
		return nil, nil, fmt.Errorf("rotation tx failed: %w", err)
	}
	receipt, err := c.Clients[0].TransactionReceipt(context.Background(), tx.Hash())
	if err != nil || receipt == nil {
		return nil, nil, fmt.Errorf("read rotation receipt failed: %w", err)
	}

	var effectiveBlock uint64
	for _, lg := range receipt.Logs {
		ev, parseErr := c.Validators.ParseLogScheduleValidatorSigner(*lg)
		if parseErr != nil {
			continue
		}
		if ev.Validator == validator && ev.Signer == newSigner && ev.EffectiveBlock != nil {
			effectiveBlock = ev.EffectiveBlock.Uint64()
			break
		}
	}
	if effectiveBlock == 0 {
		epochVal, epochErr := c.Validators.Epoch(nil)
		if epochErr != nil || epochVal == nil || epochVal.Sign() <= 0 {
			return nil, nil, fmt.Errorf("rotation effective block not found in logs")
		}
		current, err := c.Clients[0].BlockNumber(context.Background())
		if err != nil {
			return nil, nil, fmt.Errorf("read current block after rotation failed: %w", err)
		}
		epoch := epochVal.Uint64()
		effectiveBlock = ((current / epoch) + 1) * epoch
	}

	return &SignerRotation{
		Validator:      validator,
		OldSigner:      oldSigner,
		NewSigner:      newSigner,
		NewSignerKey:   newSignerKey,
		EffectiveBlock: effectiveBlock,
	}, receipt, nil
}

func PrepareSingleValidatorSignerRotation(c *testctx.CIContext) (*SignerRotation, *types.Receipt, error) {
	if c == nil || c.Config == nil {
		return nil, nil, fmt.Errorf("context not initialized")
	}
	if len(c.Config.Validators) != 1 {
		return nil, nil, fmt.Errorf("single-validator topology required")
	}
	validator := common.HexToAddress(c.Config.Validators[0].Address)
	return PrepareValidatorSignerRotation(c, validator)
}

func ImportUnlockAndSetEtherbase(client *ethclient.Client, key *ecdsa.PrivateKey, password string) error {
	if client == nil {
		return fmt.Errorf("nil client")
	}
	if key == nil {
		return fmt.Errorf("nil key")
	}
	if strings.TrimSpace(password) == "" {
		password = "123456"
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)
	raw := hex.EncodeToString(crypto.FromECDSA(key))
	var imported string
	errRaw := client.Client().Call(&imported, "personal_importRawKey", raw, password)
	if errRaw != nil {
		lowerRaw := strings.ToLower(errRaw.Error())
		if !strings.Contains(lowerRaw, "already exists") && !strings.Contains(lowerRaw, "account exists") {
			errPrefixed := client.Client().Call(&imported, "personal_importRawKey", "0x"+raw, password)
			if errPrefixed != nil {
				lowerPrefixed := strings.ToLower(errPrefixed.Error())
				if !strings.Contains(lowerPrefixed, "already exists") && !strings.Contains(lowerPrefixed, "account exists") {
					return fmt.Errorf("import raw key failed: raw=%v prefixed=%v", errRaw, errPrefixed)
				}
			}
		}
	}

	var unlocked bool
	if err := client.Client().Call(&unlocked, "personal_unlockAccount", addr, password, 0); err != nil {
		return fmt.Errorf("unlock account failed: %w", err)
	}
	if !unlocked {
		return fmt.Errorf("unlock account returned false for %s", addr.Hex())
	}

	var ok bool
	if err := client.Client().Call(&ok, "miner_setEtherbase", addr); err != nil {
		return fmt.Errorf("set etherbase failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("set etherbase returned false for %s", addr.Hex())
	}
	return nil
}

func MinerStop(client *ethclient.Client) error {
	if client == nil {
		return fmt.Errorf("nil client")
	}
	var ok bool
	if err := client.Client().Call(&ok, "miner_stop"); err != nil {
		return fmt.Errorf("miner_stop failed: %w", err)
	}
	return nil
}

func MinerStart(client *ethclient.Client) error {
	if client == nil {
		return fmt.Errorf("nil client")
	}
	var ok bool
	if err := client.Client().Call(&ok, "miner_start"); err != nil {
		if err2 := client.Client().Call(&ok, "miner_start", 1); err2 != nil {
			return fmt.Errorf("miner_start failed: %w", err)
		}
	}
	return nil
}

func restartSingleNodeRuntime(c *testctx.CIContext, timeout time.Duration) error {
	if c == nil || c.Config == nil {
		return fmt.Errorf("context not initialized")
	}
	if strings.TrimSpace(c.Config.SourcePath) == "" {
		return fmt.Errorf("config source path is empty")
	}

	repoRoot := filepath.Dir(filepath.Dir(c.Config.SourcePath))
	script := filepath.Join(repoRoot, "scripts", "network", "native_single.sh")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("native single script not found: %w", err)
	}

	configPath := c.Config.SourcePath
	sessionPath := filepath.Join(repoRoot, "data", "runtime_session.yaml")
	if stat, err := os.Stat(sessionPath); err == nil && !stat.IsDir() {
		configPath = sessionPath
	}

	timeoutSecs := int(timeout / time.Second)
	if timeoutSecs <= 0 {
		timeoutSecs = 90
	}

	run := func(action string) error {
		cmd := exec.Command("/bin/bash", script, action, configPath)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), fmt.Sprintf("WAIT_TIMEOUT=%d", timeoutSecs))
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s native single node failed: %w output=%s", action, err, strings.TrimSpace(string(out)))
		}
		return nil
	}

	if err := run("down"); err != nil {
		return err
	}
	if err := run("up"); err != nil {
		return err
	}
	if err := run("ready"); err != nil {
		return err
	}
	return nil
}

func runtimeSingleSignerFiles(c *testctx.CIContext) (string, string, error) {
	if c == nil || c.Config == nil {
		return "", "", fmt.Errorf("context not initialized")
	}
	if len(c.Config.RuntimeNodes) == 0 {
		return "", "", fmt.Errorf("runtime_nodes not configured")
	}

	keyFile := strings.TrimSpace(c.Config.RuntimeNodes[0].SignerKey)
	if keyFile == "" {
		keyFile = filepath.Join(filepath.Dir(c.Config.SourcePath), "node0", "signer.key")
	}
	if !filepath.IsAbs(keyFile) {
		keyFile = filepath.Clean(filepath.Join(filepath.Dir(c.Config.SourcePath), keyFile))
	}

	addrFile := filepath.Join(filepath.Dir(keyFile), "signer.addr")
	return keyFile, addrFile, nil
}

func runtimeNodeImpl(c *testctx.CIContext, idx int) string {
	if c == nil || c.Config == nil {
		return ""
	}
	if idx >= 0 && idx < len(c.Config.RuntimeNodes) {
		if impl := strings.ToLower(strings.TrimSpace(c.Config.RuntimeNodes[idx].Impl)); impl != "" {
			return impl
		}
	}
	return strings.ToLower(strings.TrimSpace(c.Config.Runtime.Impl))
}

func runtimeNodeDir(c *testctx.CIContext, idx int) (string, error) {
	if c == nil || c.Config == nil {
		return "", fmt.Errorf("context not initialized")
	}
	if strings.TrimSpace(c.Config.SourcePath) == "" {
		return "", fmt.Errorf("config source path is empty")
	}
	return filepath.Join(filepath.Dir(c.Config.SourcePath), fmt.Sprintf("node%d", idx)), nil
}

func runtimeNodePasswordFile(c *testctx.CIContext, idx int, nodeDir string) string {
	if c != nil && c.Config != nil && idx >= 0 && idx < len(c.Config.RuntimeNodes) {
		if passFile := strings.TrimSpace(c.Config.RuntimeNodes[idx].PasswordFile); passFile != "" {
			if !filepath.IsAbs(passFile) {
				return filepath.Clean(filepath.Join(filepath.Dir(c.Config.SourcePath), passFile))
			}
			return passFile
		}
	}
	return filepath.Join(nodeDir, "password.txt")
}

func replaceNodeKeystore(nodeDir string, passFile string, key *ecdsa.PrivateKey) (string, common.Address, error) {
	if key == nil {
		return "", common.Address{}, fmt.Errorf("nil signer key")
	}
	if strings.TrimSpace(passFile) == "" {
		passFile = filepath.Join(nodeDir, "password.txt")
	}

	passwordBytes, err := os.ReadFile(passFile)
	if err != nil {
		if os.IsNotExist(err) {
			passwordBytes = []byte("123456\n")
			if err := os.WriteFile(passFile, passwordBytes, 0o600); err != nil {
				return "", common.Address{}, fmt.Errorf("write validator password file failed: %w", err)
			}
		} else {
			return "", common.Address{}, fmt.Errorf("read validator password file failed: %w", err)
		}
	}
	password := strings.TrimRight(string(passwordBytes), "\r\n")
	if password == "" {
		password = "123456"
	}

	keystoreDir := filepath.Join(nodeDir, "keystore")
	if err := os.MkdirAll(keystoreDir, 0o755); err != nil {
		return "", common.Address{}, fmt.Errorf("create keystore dir failed: %w", err)
	}
	entries, err := os.ReadDir(keystoreDir)
	if err != nil {
		return "", common.Address{}, fmt.Errorf("read keystore dir failed: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(keystoreDir, entry.Name())); err != nil {
			return "", common.Address{}, fmt.Errorf("remove old keystore file failed: %w", err)
		}
	}

	ks := keystore.NewKeyStore(keystoreDir, keystore.StandardScryptN, keystore.StandardScryptP)
	acc, err := ks.ImportECDSA(key, password)
	if err != nil {
		return "", common.Address{}, fmt.Errorf("import rotated signer into keystore failed: %w", err)
	}

	addrFile := filepath.Join(nodeDir, "keystore.addr")
	if err := os.WriteFile(addrFile, []byte(acc.Address.Hex()+"\n"), 0o644); err != nil {
		return "", common.Address{}, fmt.Errorf("write keystore address file failed: %w", err)
	}
	return filepath.Clean(acc.URL.Path), acc.Address, nil
}

func writeRotatedSignerArtifacts(c *testctx.CIContext, idx int, keyFile string, addrFile string, key *ecdsa.PrivateKey) (common.Address, string, string, error) {
	if key == nil {
		return common.Address{}, "", "", fmt.Errorf("nil signer key")
	}
	keyHex := hex.EncodeToString(crypto.FromECDSA(key))
	addr := crypto.PubkeyToAddress(key.PublicKey)
	if err := os.WriteFile(keyFile, []byte(keyHex), 0o600); err != nil {
		return common.Address{}, "", "", fmt.Errorf("write runtime signer key failed: %w", err)
	}
	if err := os.WriteFile(addrFile, []byte(addr.Hex()+"\n"), 0o644); err != nil {
		return common.Address{}, "", "", fmt.Errorf("write runtime signer addr failed: %w", err)
	}

	if runtimeNodeImpl(c, idx) != "reth" {
		return addr, "", "", nil
	}

	nodeDir, err := runtimeNodeDir(c, idx)
	if err != nil {
		return common.Address{}, "", "", err
	}
	passFile := runtimeNodePasswordFile(c, idx, nodeDir)
	keystorePath, _, err := replaceNodeKeystore(nodeDir, passFile, key)
	if err != nil {
		return common.Address{}, "", "", err
	}
	return addr, keystorePath, passFile, nil
}

func runtimeNodeForValidator(c *testctx.CIContext, validator common.Address) (int, string, string, error) {
	if c == nil || c.Config == nil {
		return 0, "", "", fmt.Errorf("context not initialized")
	}
	for idx, node := range c.Config.RuntimeNodes {
		if strings.EqualFold(strings.TrimSpace(node.ValidatorAddress), validator.Hex()) {
			keyFile := strings.TrimSpace(node.SignerKey)
			if keyFile == "" {
				return 0, "", "", fmt.Errorf("runtime node signer_key missing for %s", validator.Hex())
			}
			if !filepath.IsAbs(keyFile) {
				keyFile = filepath.Clean(filepath.Join(filepath.Dir(c.Config.SourcePath), keyFile))
			}
			return idx, keyFile, filepath.Join(filepath.Dir(keyFile), "signer.addr"), nil
		}
	}
	return 0, "", "", fmt.Errorf("runtime node not found for validator %s", validator.Hex())
}

func ActivateRotatedSignerOnSingleNode(c *testctx.CIContext, rotation *SignerRotation, timeout time.Duration) error {
	if rotation == nil {
		return fmt.Errorf("rotation is nil")
	}

	keyFile, addrFile, err := runtimeSingleSignerFiles(c)
	if err != nil {
		return err
	}

	if _, _, _, err := writeRotatedSignerArtifacts(c, 0, keyFile, addrFile, rotation.NewSignerKey); err != nil {
		return err
	}
	if err := restartSingleNodeRuntime(c, timeout); err != nil {
		return err
	}
	return nil
}

func RestartValidatorNodeWithSigner(c *testctx.CIContext, validator common.Address, key *ecdsa.PrivateKey, timeout time.Duration) error {
	if c == nil || c.Config == nil {
		return fmt.Errorf("context not initialized")
	}
	if key == nil {
		return fmt.Errorf("nil signer key")
	}
	if strings.TrimSpace(c.Config.SourcePath) == "" {
		return fmt.Errorf("config source path is empty")
	}

	idx, keyFile, addrFile, err := runtimeNodeForValidator(c, validator)
	if err != nil {
		return err
	}
	addr, keystorePath, passFile, err := writeRotatedSignerArtifacts(c, idx, keyFile, addrFile, key)
	if err != nil {
		return err
	}

	repoRoot := filepath.Dir(filepath.Dir(c.Config.SourcePath))
	nativeEnvFile := filepath.Join(repoRoot, "data", "native", ".env")
	envUpdates := map[string]string{
		fmt.Sprintf("VALIDATOR%d_ADDRESS", idx+1):          addr.Hex(),
		fmt.Sprintf("VALIDATOR%d_KEYSTORE_ADDRESS", idx+1): addr.Hex(),
	}
	if keystorePath != "" {
		envUpdates[fmt.Sprintf("VALIDATOR%d_KEYSTORE_PATH", idx+1)] = keystorePath
	}
	if passFile != "" {
		envUpdates[fmt.Sprintf("VALIDATOR%d_PASSWORD", idx+1)] = passFile
	}
	if err := updateEnvFile(nativeEnvFile, envUpdates); err != nil {
		return fmt.Errorf("update native env for validator restart failed: %w", err)
	}

	nodeDir := filepath.Dir(keyFile)
	passwordFile := filepath.Join(nodeDir, "password.txt")
	if _, statErr := os.Stat(passwordFile); statErr != nil {
		if err := os.WriteFile(passwordFile, []byte("123456\n"), 0o600); err != nil {
			return fmt.Errorf("write validator password file failed: %w", err)
		}
	}

	gethBinary := envValueFromFile(nativeEnvFile, "GETH_BINARY")
	if strings.TrimSpace(gethBinary) != "" {
		cmd := exec.Command(gethBinary, "account", "import", "--datadir", nodeDir, "--password", passwordFile, keyFile)
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			lower := strings.ToLower(strings.TrimSpace(string(out)))
			if !strings.Contains(lower, "already exists") && !strings.Contains(lower, "account exists") {
				return fmt.Errorf("import rotated signer into keystore failed: %w output=%s", err, strings.TrimSpace(string(out)))
			}
		}
	}

	processName := fmt.Sprintf("%s-validator%d", defaultPM2Namespace(c), idx+1)
	envForPM2 := append(os.Environ(),
		fmt.Sprintf("PM2_NAMESPACE=%s", defaultPM2Namespace(c)),
		fmt.Sprintf("NATIVE_ENV_FILE=%s", nativeEnvFile),
		fmt.Sprintf("VALIDATOR%d_ADDRESS=%s", idx+1, addr.Hex()),
		fmt.Sprintf("VALIDATOR%d_KEYSTORE_ADDRESS=%s", idx+1, addr.Hex()),
	)
	if keystorePath != "" {
		envForPM2 = append(envForPM2, fmt.Sprintf("VALIDATOR%d_KEYSTORE_PATH=%s", idx+1, keystorePath))
	}
	if passFile != "" {
		envForPM2 = append(envForPM2, fmt.Sprintf("VALIDATOR%d_PASSWORD=%s", idx+1, passFile))
	}

	// `pm2 restart <name>` keeps the previously materialized argv, which may still
	// contain the old signer address. Delete + start (via ecosystem) forces argv
	// regeneration with the updated signer env.
	deleteCmd := exec.Command("pm2", "delete", processName)
	deleteCmd.Dir = repoRoot
	deleteCmd.Env = envForPM2
	if out, err := deleteCmd.CombinedOutput(); err != nil {
		lower := strings.ToLower(strings.TrimSpace(string(out)))
		if !strings.Contains(lower, "process or namespace") && !strings.Contains(lower, "not found") {
			return fmt.Errorf("pm2 delete failed for %s: %w output=%s", processName, err, strings.TrimSpace(string(out)))
		}
	}

	ecosystemFile := filepath.Join(repoRoot, "native", "ecosystem.config.js")
	startCmd := exec.Command("pm2", "start", ecosystemFile, "--only", processName, "--update-env")
	startCmd.Dir = repoRoot
	startCmd.Env = envForPM2
	if out, err := startCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pm2 start failed for %s: %w output=%s", processName, err, strings.TrimSpace(string(out)))
	}

	rpcURL := strings.TrimSpace(c.ValidatorRPCByValidator(validator))
	if rpcURL == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var client *ethclient.Client
	for time.Now().Before(deadline) {
		client, err = ethclient.Dial(rpcURL)
		if err == nil {
			_, blockErr := client.BlockNumber(context.Background())
			if blockErr == nil {
				break
			}
			client.Close()
			client = nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	if client == nil {
		return fmt.Errorf("validator rpc %s not ready after pm2 restart", rpcURL)
	}
	defer client.Close()
	return nil
}

func StopValidatorNode(c *testctx.CIContext, validator common.Address, timeout time.Duration) error {
	if c == nil || c.Config == nil {
		return fmt.Errorf("context not initialized")
	}
	if strings.TrimSpace(c.Config.SourcePath) == "" {
		return fmt.Errorf("config source path is empty")
	}

	idx, _, _, err := runtimeNodeForValidator(c, validator)
	if err != nil {
		return err
	}

	repoRoot := filepath.Dir(filepath.Dir(c.Config.SourcePath))
	processName := fmt.Sprintf("%s-validator%d", defaultPM2Namespace(c), idx+1)
	cmd := exec.Command("pm2", "stop", processName)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), fmt.Sprintf("PM2_NAMESPACE=%s", defaultPM2Namespace(c)))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pm2 stop failed for %s: %w output=%s", processName, err, strings.TrimSpace(string(out)))
	}

	rpcURL := strings.TrimSpace(c.ValidatorRPCByValidator(validator))
	if rpcURL == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		client, dialErr := ethclient.Dial(rpcURL)
		if dialErr != nil {
			return nil
		}
		_, blockErr := client.BlockNumber(context.Background())
		client.Close()
		if blockErr != nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("validator rpc %s still responding after pm2 stop", rpcURL)
}

func envValueFromFile(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := key + "="
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func updateEnvFile(path string, values map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	seen := make(map[string]bool, len(values))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for key, value := range values {
			prefix := key + "="
			if strings.HasPrefix(trimmed, prefix) {
				lines[i] = prefix + value
				seen[key] = true
			}
		}
	}
	for key, value := range values {
		if !seen[key] {
			lines = append(lines, key+"="+value)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func defaultPM2Namespace(c *testctx.CIContext) string {
	if value := strings.TrimSpace(os.Getenv("PM2_NAMESPACE")); value != "" {
		return value
	}
	return "ju-chain"
}

func WaitForCoinbaseSigner(c *testctx.CIContext, expected common.Address, timeout time.Duration) error {
	if c == nil || len(c.Clients) == 0 {
		return fmt.Errorf("context not initialized")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		header, err := c.Clients[0].HeaderByNumber(context.Background(), nil)
		if err == nil && header != nil && header.Coinbase == expected {
			return nil
		}
		_ = c.WaitForBlockProgress(1, 3*time.Second)
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("coinbase did not switch to %s within %s", expected.Hex(), timeout)
}

func WaitForBlockReceipt(c *testctx.CIContext, txHash common.Hash, timeout time.Duration) (*types.Receipt, error) {
	if c == nil || len(c.Clients) == 0 {
		return nil, fmt.Errorf("context not initialized")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		receipt, err := c.Clients[0].TransactionReceipt(context.Background(), txHash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("receipt not found for tx %s within %s", txHash.Hex(), timeout)
}

func SendValueSystemTx(c *testctx.CIContext, key *ecdsa.PrivateKey, value *big.Int, send func(*bind.TransactOpts) (*types.Transaction, error)) (*types.Transaction, error) {
	if c == nil {
		return nil, fmt.Errorf("context not initialized")
	}
	opts, err := c.GetTransactorNoEpochWait(key, true)
	if err != nil {
		return nil, err
	}
	opts.GasLimit = 1_500_000
	if value != nil {
		opts.Value = new(big.Int).Set(value)
	}
	return send(opts)
}
