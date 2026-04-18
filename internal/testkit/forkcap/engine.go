package forkcap

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"juchain.org/chain/tools/ci/internal/config"
)

const (
	defaultEnginePort = "18550"
)

type engineRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type engineRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *engineRPCError `json:"error"`
}

func CheckOsakaEngineBlobAPITransition(cfg *config.Config, shouldFail bool) error {
	endpoint, secret, err := EngineAuthRPC(cfg)
	if err != nil {
		return err
	}
	v2Result, v2Err, err := callEngineRPC(endpoint, secret, "engine_getBlobsV2", []any{[]common.Hash{common.HexToHash("0x01")}})
	if err != nil {
		return err
	}
	v3Result, v3Err, err := callEngineRPC(endpoint, secret, "engine_getBlobsV3", []any{[]common.Hash{common.HexToHash("0x01")}})
	if err != nil {
		return err
	}
	if v2Err != nil {
		return fmt.Errorf("unexpected engine_getBlobsV2 rpc error: code=%d message=%q", v2Err.Code, v2Err.Message)
	}
	if v3Err != nil {
		return fmt.Errorf("unexpected engine_getBlobsV3 rpc error: code=%d message=%q", v3Err.Code, v3Err.Message)
	}
	if shouldFail {
		if !isNullJSON(v2Result) {
			return fmt.Errorf("expected pre-Osaka engine_getBlobsV2 to return null, got %s", strings.TrimSpace(string(v2Result)))
		}
		if !isNullJSON(v3Result) {
			return fmt.Errorf("expected pre-Osaka engine_getBlobsV3 to return null, got %s", strings.TrimSpace(string(v3Result)))
		}
		return nil
	}
	if !isNullJSON(v2Result) {
		return fmt.Errorf("expected post-Osaka engine_getBlobsV2 to return null for missing blob, got %s", strings.TrimSpace(string(v2Result)))
	}
	trimmedV3 := bytes.TrimSpace(v3Result)
	if bytes.Equal(trimmedV3, []byte("null")) || len(trimmedV3) == 0 {
		return fmt.Errorf("expected post-Osaka engine_getBlobsV3 partial response, got %s", strings.TrimSpace(string(v3Result)))
	}
	var decoded []*engine.BlobAndProofV2
	if err := json.Unmarshal(trimmedV3, &decoded); err != nil {
		return fmt.Errorf("decode post-Osaka engine_getBlobsV3 result: %w", err)
	}
	if len(decoded) != 1 {
		return fmt.Errorf("expected post-Osaka engine_getBlobsV3 result len=1, got %d", len(decoded))
	}
	if decoded[0] != nil {
		return fmt.Errorf("expected post-Osaka engine_getBlobsV3 missing blob entry to be null, got %#v", decoded[0])
	}
	return nil
}

func CheckOsakaEngineGetPayloadTransition(cfg *config.Config, shouldFail bool) error {
	endpoint, secret, err := EngineAuthRPC(cfg)
	if err != nil {
		return err
	}
	payloadID, err := buildEnginePayloadV3(cfg, endpoint, secret)
	if err != nil {
		return err
	}
	v4Result, v4Err, err := callEngineRPC(endpoint, secret, "engine_getPayloadV4", []any{payloadID})
	if err != nil {
		return err
	}
	v5Result, v5Err, err := callEngineRPC(endpoint, secret, "engine_getPayloadV5", []any{payloadID})
	if err != nil {
		return err
	}
	if shouldFail {
		if v4Err != nil {
			return fmt.Errorf("expected pre-Osaka engine_getPayloadV4 success, got code=%d message=%q", v4Err.Code, v4Err.Message)
		}
		if isNullJSON(v4Result) {
			return fmt.Errorf("expected pre-Osaka engine_getPayloadV4 payload result, got %s", strings.TrimSpace(string(v4Result)))
		}
		if err := requireUnsupportedFork("engine_getPayloadV5", v5Err); err != nil {
			return err
		}
		return nil
	}
	if err := requireUnsupportedFork("engine_getPayloadV4", v4Err); err != nil {
		return err
	}
	if v5Err != nil {
		return fmt.Errorf("expected post-Osaka engine_getPayloadV5 success, got code=%d message=%q", v5Err.Code, v5Err.Message)
	}
	if isNullJSON(v5Result) {
		return fmt.Errorf("expected post-Osaka engine_getPayloadV5 payload result, got %s", strings.TrimSpace(string(v5Result)))
	}
	return nil
}

func buildEnginePayloadV3(cfg *config.Config, endpoint string, secret []byte) (*engine.PayloadID, error) {
	if cfg == nil || len(cfg.RPCs) == 0 {
		return nil, fmt.Errorf("missing public rpc configuration for payload build")
	}
	client, err := ethclient.Dial(cfg.RPCs[0])
	if err != nil {
		return nil, fmt.Errorf("dial public rpc %s: %w", cfg.RPCs[0], err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	header, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("read latest header: %w", err)
	}
	beaconRoot := common.HexToHash("0x42")
	resp, rpcErr, err := callEngineForkchoiceUpdatedV3(endpoint, secret, engine.ForkchoiceStateV1{
		HeadBlockHash: header.Hash(),
	}, &engine.PayloadAttributes{
		Timestamp:             header.Time + 1,
		Random:                common.HexToHash("0x1234"),
		SuggestedFeeRecipient: preferredFeeRecipient(cfg),
		Withdrawals:           []*types.Withdrawal{},
		BeaconRoot:            &beaconRoot,
	})
	if err != nil {
		return nil, err
	}
	if rpcErr != nil {
		return nil, fmt.Errorf("engine_forkchoiceUpdatedV3 failed: code=%d message=%q", rpcErr.Code, rpcErr.Message)
	}
	if resp == nil {
		return nil, fmt.Errorf("engine_forkchoiceUpdatedV3 returned nil response")
	}
	if !strings.EqualFold(string(resp.PayloadStatus.Status), "VALID") {
		return nil, fmt.Errorf("engine_forkchoiceUpdatedV3 returned status=%s", resp.PayloadStatus.Status)
	}
	if resp.PayloadID == nil {
		return nil, fmt.Errorf("engine_forkchoiceUpdatedV3 returned nil payloadId")
	}
	return resp.PayloadID, nil
}

func callEngineForkchoiceUpdatedV3(endpoint string, secret []byte, state engine.ForkchoiceStateV1, attrs *engine.PayloadAttributes) (*engine.ForkChoiceResponse, *engineRPCError, error) {
	result, rpcErr, err := callEngineRPC(endpoint, secret, "engine_forkchoiceUpdatedV3", []any{state, attrs})
	if err != nil {
		return nil, nil, err
	}
	if rpcErr != nil {
		return nil, rpcErr, nil
	}
	var resp engine.ForkChoiceResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, nil, fmt.Errorf("decode engine_forkchoiceUpdatedV3 response: %w", err)
	}
	return &resp, nil, nil
}

func requireUnsupportedFork(method string, rpcErr *engineRPCError) error {
	if rpcErr == nil {
		return fmt.Errorf("expected %s unsupported-fork error, got success", method)
	}
	if rpcErr.Code != -38005 || !strings.EqualFold(strings.TrimSpace(rpcErr.Message), "Unsupported fork") {
		return fmt.Errorf("unexpected %s error: code=%d message=%q", method, rpcErr.Code, rpcErr.Message)
	}
	return nil
}

func isNullJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func preferredFeeRecipient(cfg *config.Config) common.Address {
	if cfg != nil {
		if len(cfg.RuntimeNodes) > 0 && strings.TrimSpace(cfg.RuntimeNodes[0].FeeAddress) != "" {
			return common.HexToAddress(cfg.RuntimeNodes[0].FeeAddress)
		}
		if len(cfg.Validators) > 0 && strings.TrimSpace(cfg.Validators[0].FeeAddress) != "" {
			return common.HexToAddress(cfg.Validators[0].FeeAddress)
		}
		if strings.TrimSpace(cfg.Funder.Address) != "" {
			return common.HexToAddress(cfg.Funder.Address)
		}
	}
	return common.Address{}
}

func EngineAuthRPC(cfg *config.Config) (string, []byte, error) {
	dataDir, err := forkcapDataDir(cfg)
	if err != nil {
		return "", nil, err
	}
	port, err := enginePortFromEnv(filepath.Join(dataDir, "native", ".env"))
	if err != nil {
		return "", nil, err
	}
	secretPath := filepath.Join(dataDir, "node0", "geth", "jwtsecret")
	secretRaw, err := os.ReadFile(secretPath)
	if err != nil {
		return "", nil, fmt.Errorf("read jwt secret %s: %w", secretPath, err)
	}
	secret, err := decodeJWTSecret(secretRaw)
	if err != nil {
		return "", nil, fmt.Errorf("decode jwt secret %s: %w", secretPath, err)
	}
	return fmt.Sprintf("http://127.0.0.1:%s", port), secret, nil
}

func forkcapDataDir(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.SourcePath != "" {
		return filepath.Dir(cfg.SourcePath), nil
	}
	return "", fmt.Errorf("missing config source path for forkcap data dir")
}

func enginePortFromEnv(envPath string) (string, error) {
	data, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultEnginePort, nil
		}
		return "", fmt.Errorf("read native env %s: %w", envPath, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "VALIDATOR1_ENGINE_PORT=") {
			continue
		}
		port := strings.TrimSpace(strings.TrimPrefix(trimmed, "VALIDATOR1_ENGINE_PORT="))
		if port == "" {
			break
		}
		return port, nil
	}
	return defaultEnginePort, nil
}

func decodeJWTSecret(raw []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, fmt.Errorf("empty jwt secret")
	}
	trimmed = strings.TrimPrefix(strings.ToLower(trimmed), "0x")
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	if len(decoded) == 0 {
		return nil, fmt.Errorf("decoded empty jwt secret")
	}
	return decoded, nil
}

func callEngineRPC(endpoint string, secret []byte, method string, params []any) (json.RawMessage, *engineRPCError, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal %s payload: %w", method, err)
	}
	token, err := authToken(secret, time.Now())
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s request failed: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("%s returned http %d", method, resp.StatusCode)
	}
	var decoded engineRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, nil, fmt.Errorf("decode %s response: %w", method, err)
	}
	return decoded.Result, decoded.Error, nil
}

func authToken(secret []byte, now time.Time) (string, error) {
	head, err := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}
	claims, err := json.Marshal(map[string]any{"iat": now.Unix()})
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}
	enc := base64.RawURLEncoding
	headerPart := enc.EncodeToString(head)
	claimsPart := enc.EncodeToString(claims)
	unsigned := headerPart + "." + claimsPart
	mac := hmac.New(sha256.New, secret)
	if _, err := mac.Write([]byte(unsigned)); err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	sig := enc.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}
