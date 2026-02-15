const fs = require('fs');
const path = require('path');

// Contract address mappings (Must match init_genesis.js)
const CONTRACT_ADDRESSES = {
  Validators: "0x000000000000000000000000000000000000f010",
  Punish: "0x000000000000000000000000000000000000f011",
  Proposal: "0x000000000000000000000000000000000000f012",
  Staking: "0x000000000000000000000000000000000000f013",
};

// Resolve chain-contract root.
// Priority:
// 1) CHAIN_CONTRACT_OUT env (compiled output only)
// 2) CHAIN_CONTRACT_ROOT env
// 2) ../chain-contract (sibling of chain-tests)
// 3) local root fallback
const CHAIN_CONTRACT_OUT = process.env.CHAIN_CONTRACT_OUT;
const DEFAULT_CHAIN_CONTRACT_ROOT = path.resolve(__dirname, '../../chain-contract');
const LOCAL_ROOT_FALLBACK = path.resolve(__dirname, '..');
const CHAIN_CONTRACT_ROOT = process.env.CHAIN_CONTRACT_ROOT || (
  fs.existsSync(DEFAULT_CHAIN_CONTRACT_ROOT) ? DEFAULT_CHAIN_CONTRACT_ROOT : LOCAL_ROOT_FALLBACK
);
const OUT_DIR = CHAIN_CONTRACT_OUT || path.join(CHAIN_CONTRACT_ROOT, 'out');

function getContractBytecode(contractName) {
  const artifactPath = path.join(OUT_DIR, `${contractName}.sol`, `${contractName}.json`);
  try {
    if (!fs.existsSync(artifactPath)) {
      console.error(`Artifact not found: ${artifactPath}`);
      return null;
    }
    const artifact = JSON.parse(fs.readFileSync(artifactPath, 'utf8'));
    // Use deployedBytecode (runtime bytecode) for genesis alloc
    return artifact.deployedBytecode?.object || artifact.deployedBytecode;
  } catch (error) {
    console.error(`Failed to read bytecode for ${contractName}: ${error.message}`);
    return null;
  }
}

function main() {
  const alloc = {};

  for (const [contractName, address] of Object.entries(CONTRACT_ADDRESSES)) {
    let bytecode = getContractBytecode(contractName);
    
    if (!bytecode) {
      console.error(`Error: Could not find bytecode for ${contractName}. Ensure compiled artifacts exist in ${OUT_DIR}.`);
      process.exit(1);
    }

    // Ensure hex prefix
    if (!bytecode.startsWith('0x')) {
      bytecode = '0x' + bytecode;
    }

    alloc[address] = {
      balance: "0x0",
      code: bytecode
    };
  }

  // Output JSON string to stdout
  console.log(JSON.stringify(alloc, null, 2));
}

main();
