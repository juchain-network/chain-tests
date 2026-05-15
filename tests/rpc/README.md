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

Run the suite using:
```bash
make test-rpc
```

This target is defined in the top-level `Makefile` and wraps the standard CI runner to execute all tests in `tests/rpc`.

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
