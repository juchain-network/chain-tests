# RPC Testing Capability

This directory (`tests/rpc`) contains a dedicated functional test capability for validating public node RPC behavior against expected semantics on local runtime topologies.

## Scope and Inventory

First delivery focuses on supported **public node RPC** behavior that is stable and locally observable. 

### First-Scope Method Inventory

1. **Identity & Liveness Methods**
   - `web3_clientVersion`: Node version identifier
   - `eth_chainId`: The active chain ID
   - `eth_blockNumber`: Current block height
   - `eth_syncing`: Sync status (expected `false` or sync info)
   - `net_version`: Network ID
   - `net_peerCount`: Number of connected peers

2. **Block & Transaction Lookup Methods**
   - `eth_getBlockByNumber`: Fetch block details by block number
   - `eth_getBlockByHash`: Fetch block details by hash
   - `eth_getTransactionByHash`: Fetch tx by hash
   - `eth_getTransactionReceipt`: Fetch tx receipt

3. **Read-only Execution & Query Methods**
   - `eth_call`: Execute contract call without state change

4. **Role & Runtime-Dependent Methods**
   - `eth_coinbase`: Validator/miner fee recipient address

### Out of Scope (for First Delivery)
- Broad `debug_*` / `trace_*` namespaces
- Broad `admin_*` / `txpool_*` namespaces
- Authenticated Engine APIs (beyond existing forkcap checks)
- Universal parity claims for non-contractual namespaces

## Execution Model

RPC behavior validation is explicitly isolated from the generic test groups to keep it discoverable.

Canonical entrypoints:
```bash
make test-rpc
make test-rpc-readonly RPC_URL=http://localhost:18545
```

This target is defined in the top-level `Makefile` and wraps the shared CI runner to execute the RPC suite in `tests/rpc` against the local initialized network. The readonly target accepts `RPC_URL` for remote-friendly pure read-only validation.

By default `make test-rpc` runs the **full local suite**.
`make test-rpc-readonly` runs the **readonly subset** only.

Useful focused reruns:
```bash
make test-rpc RUN='TestRPC_NegativeMethods'
make test-rpc RUN='TestRPC_(NegativeMethods|ConditionalSurface)'
make test-rpc RUN='TestRPC_CrossNodeConsistency'

make test-rpc-readonly RPC_URL=http://localhost:18545 RUN='TestRPC_Readonly_BaselinePublicMethods'
```

Useful report redirection:
```bash
make test-rpc REPORT_DIR=reports/rpc_debug
make test-rpc RUN='TestRPC_CrossNodeConsistency' REPORT_DIR=reports/rpc_consistency
make test-rpc-readonly RPC_URL=http://localhost:18545 REPORT_DIR=reports/rpc_readonly_debug
```

What to expect from command output:
- the local target prints the resolved epoch and active run pattern
- the readonly target prints the supplied RPC URL and active run pattern
- successful runs print the shared CI runner artifact paths:
  - `Report: .../report.md`
  - `Summary: .../summary.json`
  - `Manifest: .../manifest.json`
- failure output should already identify the failing method, node, role, or sub-area through the RPC test helpers; after that, use the printed report paths and runtime logs for drill-down

## First-Delivery Diagnostics Contract

When `make test-rpc` fails, the intended maintainer workflow is:

1. Re-run the narrow failing area with `RUN='TestRPC_...'`
2. Preserve artifacts with `REPORT_DIR=reports/...`
3. Inspect the shared CI artifacts (`report.md`, `summary.json`, `manifest.json`)
4. Check runtime health with:
   - `make status`
   - `NODE=<node> make logs`

When `make test-rpc-readonly` fails, the intended maintainer workflow is:

1. Re-run the narrow failing area with `RUN='TestRPC_Readonly_...'`
2. Preserve artifacts with `REPORT_DIR=reports/...`
3. Confirm the provided RPC URL is reachable and exposes the expected read-only methods

Representative repro commands:
```bash
make test-rpc RUN='TestRPC_NegativeMethods' REPORT_DIR=reports/rpc_negative_debug
make test-rpc RUN='TestRPC_ConditionalSurface' REPORT_DIR=reports/rpc_conditional_debug
make test-rpc RUN='TestRPC_CrossNodeConsistency' REPORT_DIR=reports/rpc_consistency_debug
make test-rpc-readonly RPC_URL=http://localhost:18545 RUN='TestRPC_Readonly_BaselinePublicMethods' REPORT_DIR=reports/rpc_readonly_debug
```

## Scope Notes

First delivery intentionally stays thin and evidence-driven:
- local integration RPC: public happy-path identity, lookup, `eth_call`, cross-node, negative, and conditional behavior
- readonly RPC: public read-only identity / liveness / `eth_syncing` / `net_*` methods
- representative malformed/unsupported behavior
- representative protected/forbidden behavior
- representative conditional-surface behavior

Not claimed yet:
- exhaustive malformed request matrices
- broad `debug_*`, `trace_*`, `admin_*`, or `txpool_*` namespace coverage
- universal parity claims for non-contractual or runtime-private namespaces

## Node-Role & Runtime Assumptions

- **Topology**: The suite assumes a standard multi-node local network topology (e.g., 4 nodes) where nodes have assigned roles (e.g., `validator`, `rpc`).
- **Discovery**: Tests discover available endpoints by parsing the generated `test_config.yaml` to build an `RPCNode` catalog.
- **Cross-Node Agreement**: Where semantics dictate (e.g., block number parity after a short delay), the suite asserts consistency across differently-roled nodes.
- **Fork-Aware Constraints**: Some method fields (like `blobGasUsed`) are conditional on the active fork logic.

## Reusable Helper Seams

Instead of duplicating dialing and assertion logic, the new suite reuses these concepts:
- **`tests/interop/common_test.go`**: Existing dialers (`dialEth`, `dialRPC`), state root fetchers, and cross-node historical parity checks.
- **`internal/testkit/fork_surface.go`**: Fork-conditional validation of RPC field shapes (e.g., `verifyEthConfigBlobSchedule`, `latestBlockRPC`).
- **`internal/context/context.go`**: Topology helpers for nonce-safe transactional setup required by lookup method tests.
- **`tests/rpc/helpers_test.go`**: Standardized wrappers (like `assertRawCall`) that capture and print node role/endpoint diagnostics on failure.
