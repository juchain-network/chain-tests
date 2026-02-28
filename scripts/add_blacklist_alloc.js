const fs = require('fs');
const path = require('path');

function usage() {
  console.error('Usage: node scripts/add_blacklist_alloc.js <alloc.json> <contract-address> <runtime-code-file>');
}

function readRuntimeCode(filePath) {
  const raw = fs.readFileSync(filePath, 'utf8').trim();
  if (!raw) {
    throw new Error(`empty runtime code file: ${filePath}`);
  }
  return raw.startsWith('0x') ? raw : `0x${raw}`;
}

function main() {
  const allocPath = process.argv[2];
  const contractAddress = process.argv[3];
  const runtimeCodeFile = process.argv[4];

  if (!allocPath || !contractAddress || !runtimeCodeFile) {
    usage();
    process.exit(1);
  }

  const allocAbs = path.resolve(allocPath);
  const codeAbs = path.resolve(runtimeCodeFile);

  if (!fs.existsSync(allocAbs)) {
    throw new Error(`alloc file not found: ${allocAbs}`);
  }
  if (!fs.existsSync(codeAbs)) {
    throw new Error(`runtime code file not found: ${codeAbs}`);
  }

  let alloc = JSON.parse(fs.readFileSync(allocAbs, 'utf8'));
  if (typeof alloc !== 'object' || alloc === null || Array.isArray(alloc)) {
    throw new Error(`invalid alloc JSON object: ${allocAbs}`);
  }

  const normalized = contractAddress.toLowerCase().replace(/^0x/, '');
  if (!/^[0-9a-f]{40}$/.test(normalized)) {
    throw new Error(`invalid contract address: ${contractAddress}`);
  }

  const key = `0x${normalized}`;
  const runtimeCode = readRuntimeCode(codeAbs);

  alloc[key] = {
    balance: alloc[key]?.balance || '0x0',
    code: runtimeCode,
  };

  process.stdout.write(JSON.stringify(alloc, null, 2));
}

main();
