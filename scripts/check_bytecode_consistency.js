#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const CONTRACTS = [
  {
    name: 'Validators',
    gethVar: 'posaValidatorsBytecodeHex',
    rethConst: 'POSA_VALIDATORS_BYTECODE_HEX'
  },
  {
    name: 'Proposal',
    gethVar: 'posaProposalBytecodeHex',
    rethConst: 'POSA_PROPOSAL_BYTECODE_HEX'
  },
  {
    name: 'Punish',
    gethVar: 'posaPunishBytecodeHex',
    rethConst: 'POSA_PUNISH_BYTECODE_HEX'
  },
  {
    name: 'Staking',
    gethVar: 'posaStakingBytecodeHex',
    rethConst: 'POSA_STAKING_BYTECODE_HEX'
  }
];

function usage() {
  console.error(
    'Usage: node scripts/check_bytecode_consistency.js --out-dir <chain-contract/out> [--bytecode-go <chain/consensus/congress/bytecode.go>] [--bytecode-rs <rchain/crates/congress-core/src/bytecode.rs>] [--impl <geth|reth|mixed>]'
  );
}

function parseArgs(argv) {
  const args = {};
  for (let i = 0; i < argv.length; i++) {
    const k = argv[i];
    if (!k.startsWith('--')) continue;
    const key = k.slice(2);
    const val = argv[i + 1];
    if (!val || val.startsWith('--')) {
      throw new Error(`missing value for --${key}`);
    }
    args[key] = val;
    i++;
  }
  return args;
}

function normalizeHex(v) {
  if (typeof v !== 'string') {
    throw new Error('bytecode must be string');
  }
  let hex = v.trim();
  if (hex.startsWith('0x') || hex.startsWith('0X')) {
    hex = hex.slice(2);
  }
  hex = hex.toLowerCase();
  if (!/^[0-9a-f]*$/.test(hex)) {
    throw new Error('bytecode contains non-hex characters');
  }
  if (hex.length % 2 !== 0) {
    throw new Error('bytecode hex length must be even');
  }
  return hex;
}

function readArtifactBytecode(outDir, contractName) {
  const artifactPath = path.join(outDir, `${contractName}.sol`, `${contractName}.json`);
  if (!fs.existsSync(artifactPath)) {
    throw new Error(`artifact missing: ${artifactPath}`);
  }
  const artifact = JSON.parse(fs.readFileSync(artifactPath, 'utf8'));
  const bytecode =
    artifact?.deployedBytecode?.object ??
    artifact?.deployedBytecode ??
    artifact?.bytecode?.object ??
    artifact?.bytecode;
  if (!bytecode) {
    throw new Error(`no bytecode in artifact: ${artifactPath}`);
  }
  return normalizeHex(bytecode);
}

function parseBytecodeGo(bytecodeGoPath) {
  if (!fs.existsSync(bytecodeGoPath)) {
    throw new Error(`bytecode.go missing: ${bytecodeGoPath}`);
  }
  const src = fs.readFileSync(bytecodeGoPath, 'utf8');
  const out = {};

  const assignRe =
    /(?:^|\n)\s*(posa[A-Za-z]+BytecodeHex)\s*=\s*([\s\S]*?)(?=(?:\n\s*posa[A-Za-z]+BytecodeHex\s*=)|\n\))/g;
  for (const match of src.matchAll(assignRe)) {
    const varName = match[1];
    const body = match[2];
    const chunks = [];
    for (const m of body.matchAll(/"([0-9a-fA-F]*)"/g)) {
      chunks.push(m[1]);
    }
    if (chunks.length === 0) {
      continue;
    }
    out[varName] = normalizeHex(chunks.join(''));
  }
  return out;
}

function parseBytecodeRust(bytecodeRsPath) {
  if (!fs.existsSync(bytecodeRsPath)) {
    throw new Error(`bytecode.rs missing: ${bytecodeRsPath}`);
  }
  const src = fs.readFileSync(bytecodeRsPath, 'utf8');
  const out = {};
  const constRe = /pub\s+const\s+([A-Z0-9_]+)\s*:\s*&str\s*=\s*([\s\S]*?);/g;

  for (const match of src.matchAll(constRe)) {
    const constName = match[1];
    const body = match[2];
    const chunks = [];
    for (const m of body.matchAll(/"([0-9a-fA-F]*)"/g)) {
      chunks.push(m[1]);
    }
    if (chunks.length === 0) {
      continue;
    }
    out[constName] = normalizeHex(chunks.join(''));
  }

  return out;
}

function sha256Hex(hex) {
  return crypto.createHash('sha256').update(Buffer.from(hex, 'hex')).digest('hex');
}

function firstDiffOffset(a, b) {
  const n = Math.min(a.length, b.length);
  for (let i = 0; i < n; i++) {
    if (a[i] !== b[i]) {
      return i;
    }
  }
  if (a.length !== b.length) {
    return n;
  }
  return -1;
}

function normalizeImplMode(raw) {
  const mode = (raw || '').toLowerCase();
  switch (mode) {
    case 'geth':
      return 'geth';
    case 'reth':
      return 'reth';
    case 'mixed':
    case 'both':
      return 'mixed';
    case '':
    case 'auto':
      return 'auto';
    default:
      throw new Error(`unsupported --impl: ${raw}`);
  }
}

function compareSource(kind, outDir, embedded, fieldName) {
  const mismatches = [];

  for (const c of CONTRACTS) {
    const artifactHex = readArtifactBytecode(outDir, c.name);
    const sourceField = c[fieldName];
    const embeddedHex = embedded[sourceField];
    if (!embeddedHex) {
      throw new Error(`${kind} symbol ${sourceField} not found`);
    }
    if (artifactHex !== embeddedHex) {
      const offset = firstDiffOffset(artifactHex, embeddedHex);
      mismatches.push({
        source: kind,
        contract: c.name,
        symbol: sourceField,
        compiledLen: artifactHex.length / 2,
        embeddedLen: embeddedHex.length / 2,
        compiledHash: sha256Hex(artifactHex),
        embeddedHash: sha256Hex(embeddedHex),
        firstDiffByte: offset < 0 ? -1 : Math.floor(offset / 2)
      });
    }
  }

  return mismatches;
}

function printMismatches(mismatches) {
  console.error('[runtime-precheck] ERROR: POSA bytecode mismatch detected.');
  for (const m of mismatches) {
    console.error(
      `[runtime-precheck]   [${m.source}] ${m.contract}: compiled(len=${m.compiledLen}, sha256=${m.compiledHash.slice(
        0,
        12
      )}) != embedded(len=${m.embeddedLen}, sha256=${m.embeddedHash.slice(0, 12)}), firstDiffByte=${m.firstDiffByte}`
    );
  }
  console.error('[runtime-precheck] Suggested fix:');
  console.error('[runtime-precheck]   1) rebuild chain-contract artifacts');
  console.error('[runtime-precheck]   2) rebuild geth and/or reth consensus bytecode embedding');
  console.error('[runtime-precheck]   3) rerun: make clean && make init && make run');
}

function main() {
  let args;
  try {
    args = parseArgs(process.argv.slice(2));
  } catch (err) {
    usage();
    throw err;
  }

  const outDir = args['out-dir'];
  const bytecodeGo = args['bytecode-go'];
  const bytecodeRs = args['bytecode-rs'];
  const impl = normalizeImplMode(args.impl);

  if (!outDir) {
    usage();
    process.exit(2);
  }

  let checkGeth = false;
  let checkReth = false;

  if (impl === 'geth') {
    checkGeth = true;
  } else if (impl === 'reth') {
    checkReth = true;
  } else if (impl === 'mixed') {
    checkGeth = true;
    checkReth = true;
  } else {
    checkGeth = Boolean(bytecodeGo);
    checkReth = Boolean(bytecodeRs);
  }

  if (!checkGeth && !checkReth) {
    throw new Error('no bytecode source selected: provide --impl or source paths');
  }
  if (checkGeth && !bytecodeGo) {
    throw new Error('geth bytecode selected but --bytecode-go is missing');
  }
  if (checkReth && !bytecodeRs) {
    throw new Error('reth bytecode selected but --bytecode-rs is missing');
  }

  const mismatches = [];
  if (checkGeth) {
    const embeddedGeth = parseBytecodeGo(bytecodeGo);
    mismatches.push(...compareSource('geth', outDir, embeddedGeth, 'gethVar'));
  }
  if (checkReth) {
    const embeddedReth = parseBytecodeRust(bytecodeRs);
    mismatches.push(...compareSource('reth', outDir, embeddedReth, 'rethConst'));
  }

  if (mismatches.length > 0) {
    printMismatches(mismatches);
    process.exit(1);
  }

  const checked = [checkGeth ? 'geth' : '', checkReth ? 'reth' : '']
    .filter(Boolean)
    .join(',');
  console.log(`[runtime-precheck] Bytecode consistency OK (checked: ${checked}).`);
}

try {
  main();
} catch (err) {
  console.error(`[runtime-precheck] ERROR: ${err.message}`);
  process.exit(1);
}
