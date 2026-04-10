const fs = require('fs');
const path = require('path');

const envFile = process.env.NATIVE_ENV_FILE || path.resolve(__dirname, '../data/native/.env');

function loadEnvFile(file) {
  if (!fs.existsSync(file)) return;
  const lines = fs.readFileSync(file, 'utf8').split(/\r?\n/);
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const idx = trimmed.indexOf('=');
    if (idx <= 0) continue;
    const key = trimmed.slice(0, idx).trim();
    const val = trimmed.slice(idx + 1).trim();
    if (!Object.prototype.hasOwnProperty.call(process.env, key)) {
      process.env[key] = val;
    }
  }
}

loadEnvFile(envFile);

const ns = process.env.PM2_NAMESPACE || 'ju-chain';
const gethBinary = process.env.GETH_BINARY || 'geth';
const rethBinary = process.env.RETH_BINARY || path.resolve(__dirname, '../../rchain/target/release/congress-node');
const logDir = process.env.NATIVE_LOG_DIR || path.resolve(__dirname, '../data/native-logs');
const networkId = process.env.NETWORK_ID || '666666';
const bootnodes = process.env.BOOTNODES || '';
const stateScheme = process.env.STATE_SCHEME || '';
const historyState = process.env.HISTORY_STATE || '';
const validatorAuthMode = (process.env.VALIDATOR_AUTH_MODE || 'auto').toLowerCase();
const defaultImpl = (process.env.DEFAULT_RUNTIME_IMPL || 'geth').toLowerCase();
const genesisFile = process.env.GENESIS_FILE;
const rethTrustedOnly = (process.env.RETH_TRUSTED_ONLY || 'true').toLowerCase();
const upgradeOverridePosaTime = (process.env.UPGRADE_OVERRIDE_POSA_TIME || '').trim();
const upgradeOverridePosaValidators = (process.env.UPGRADE_OVERRIDE_POSA_VALIDATORS || '').trim();
const upgradeOverridePosaSigners = (process.env.UPGRADE_OVERRIDE_POSA_SIGNERS || '').trim();

function nameOf(suffix) {
  return `${ns}-${suffix}`;
}

function appConfig(name, script, args, logPrefix, extraEnv = {}) {
  return {
    name: nameOf(name),
    script,
    args,
    cwd: process.cwd(),
    instances: 1,
    autorestart: true,
    watch: false,
    max_memory_restart: '2G',
    env: Object.assign({}, process.env, {
      ...extraEnv,
      NODE_ENV: 'production'
    }),
    error_file: path.join(logDir, `${logPrefix}-error.log`),
    out_file: path.join(logDir, `${logPrefix}-out.log`),
    log_file: path.join(logDir, `${logPrefix}-combined.log`),
    time: true,
    merge_logs: true,
    kill_timeout: 30000,
    restart_delay: 3000,
    max_restarts: 10,
    min_uptime: '10s'
  };
}

function readTrimmed(filePath) {
  if (!filePath) return '';
  try {
    return fs.readFileSync(filePath, 'utf8').trim();
  } catch (_) {
    return '';
  }
}

function resolveNodeImpl(index) {
  const raw = (process.env[`NODE${index}_IMPL`] || defaultImpl || 'geth').toLowerCase();
  if (raw !== 'geth' && raw !== 'reth') {
    throw new Error(`unsupported NODE${index}_IMPL=${raw}`);
  }
  return raw;
}

function resolveNodeBinary(index) {
  const explicit = process.env[`NODE${index}_BINARY`];
  if (explicit) return explicit;
  return binaryForImpl(resolveNodeImpl(index));
}

function migrationOverrideArgs() {
  if ((upgradeOverridePosaValidators && !upgradeOverridePosaSigners) || (!upgradeOverridePosaValidators && upgradeOverridePosaSigners)) {
    throw new Error('UPGRADE_OVERRIDE_POSA_VALIDATORS and UPGRADE_OVERRIDE_POSA_SIGNERS must be provided together');
  }

  const args = [];
  if (upgradeOverridePosaTime) {
    args.push(`--override.posaTime=${upgradeOverridePosaTime}`);
  }
  if (upgradeOverridePosaValidators) {
    args.push(`--override.posaValidators=${upgradeOverridePosaValidators}`);
  }
  if (upgradeOverridePosaSigners) {
    args.push(`--override.posaSigners=${upgradeOverridePosaSigners}`);
  }
  return args;
}

function gethCommonArgs(opts) {
  const args = [
    '--networkid', networkId,
    '--txpool.nolocals',
    '--txpool.globalslots', process.env.TXPOOL_GLOBAL_SLOTS || '12800',
    '--txpool.globalqueue', process.env.TXPOOL_GLOBAL_QUEUE || '5120',
    '--txpool.lifetime', process.env.TXPOOL_LIFETIME || '10m0s',
    '--txpool.pricelimit', process.env.TXPOOL_PRICE_LIMIT || '1000000000',
    '--syncmode=full',
    '--gcmode=full',
    '--verbosity=' + (process.env.VERBOSITY || '3'),
    '--datadir', opts.datadir,
    '--nodekey', opts.nodekey,
    '--port=' + opts.p2pPort,
    '--bootnodes=' + bootnodes,
    '--http',
    '--http.addr=0.0.0.0',
    '--http.port=' + opts.httpPort,
    '--http.corsdomain=*',
    '--http.vhosts=*',
    '--http.api=web3,debug,eth,txpool,net,personal,admin,miner,congress',
    '--ws',
    '--ws.addr=0.0.0.0',
    '--ws.port=' + opts.wsPort,
    '--ws.origins=*',
    '--ws.api=debug,eth,txpool,net,engine,personal,admin,miner,congress',
    '--authrpc.port=' + opts.enginePort,
    '--cache', opts.cache || '1024'
  ];

  if (stateScheme) {
    args.push('--state.scheme=' + stateScheme);
  }
  if (historyState) {
    args.push('--history.state=' + historyState);
  }
  args.push(...migrationOverrideArgs());

  if (opts.allowInsecureUnlock || opts.mine) {
    args.push('--allow-insecure-unlock');
  }

  if (opts.mine) {
    args.push(
      '--mine',
      '--miner.etherbase', opts.address,
      '--miner.gasprice', '0',
      '--unlock', opts.address,
      '--password', opts.passwordFile
    );
  }

  return args;
}

function resolveRethValidatorAuthArgs(index) {
  if (!index) return [];

  const keystorePath = process.env[`VALIDATOR${index}_KEYSTORE_PATH`];
  const passFile = process.env[`VALIDATOR${index}_PASSWORD`];
  const passEnvName = process.env.KEYSTORE_PASSWORD_ENV_NAME || '';
  const hasKeystore = Boolean(keystorePath && fs.existsSync(keystorePath));

  if (validatorAuthMode !== 'auto' && validatorAuthMode !== 'keystore') {
    throw new Error(`validator auth mode ${validatorAuthMode} is not supported for reth; use auto or keystore`);
  }
  if (!hasKeystore) {
    throw new Error(`missing validator keystore for validator${index}`);
  }

  const args = ['--validator-keystore', keystorePath];
  if (passFile && fs.existsSync(passFile)) {
    args.push('--validator-password-file', passFile);
    return args;
  }
  if (passEnvName && process.env[passEnvName]) {
    args.push('--password', `env:${passEnvName}`);
    return args;
  }
  throw new Error(`missing validator password source for validator${index}`);
}

function rethCommonArgs(opts) {
  if (!genesisFile) {
    throw new Error('GENESIS_FILE is required for reth runtime');
  }
  if (upgradeOverridePosaTime || upgradeOverridePosaValidators || upgradeOverridePosaSigners) {
    throw new Error('upgrade override currently supports geth runtime only');
  }
  const args = [
    'node',
    '--chain', genesisFile,
    '--datadir', opts.datadir,
    '--http',
    '--http.addr', '0.0.0.0',
    '--http.port', String(opts.httpPort),
    '--http.api', 'all',
    '--ws',
    '--ws.addr', '0.0.0.0',
    '--ws.port', String(opts.wsPort),
    '--ws.api', 'all',
    '--authrpc.port', String(opts.enginePort),
    '--port', String(opts.p2pPort),
    '--discovery.port', String(opts.p2pPort),
    '--p2p-secret-key', opts.nodekey,
    '--log.file.directory', logDir
  ];

  if (bootnodes) {
    args.push('--bootnodes', bootnodes, '--trusted-peers', bootnodes);
  }
  if (rethTrustedOnly === '1' || rethTrustedOnly === 'true' || rethTrustedOnly === 'yes' || rethTrustedOnly === 'on') {
    args.push('--trusted-only');
  }

  if (opts.validatorIndex) {
    args.push(...resolveRethValidatorAuthArgs(opts.validatorIndex));
  }

  return args;
}

function binaryForImpl(impl) {
  if (impl === 'geth') return gethBinary;
  if (impl === 'reth') return rethBinary;
  throw new Error(`unsupported impl: ${impl}`);
}

function argsForNode(node) {
  const impl = resolveNodeImpl(node.index);
  if (impl === 'geth') {
    return gethCommonArgs(node);
  }
  return rethCommonArgs(node);
}

function scriptForNode(node) {
  return resolveNodeBinary(node.index);
}

function coverageEnvForNode(node) {
  if ((process.env.CHAIN_COVERAGE_ENABLED || '') !== '1') {
    return {};
  }
  if (resolveNodeImpl(node.index) !== 'geth') {
    return {};
  }
  const dir = process.env[`NODE${node.index}_GOCOVERDIR`];
  if (!dir) {
    return {};
  }
  return { GOCOVERDIR: dir };
}

const nodeDefs = [
  {
    name: 'validator1',
    logPrefix: 'validator1',
    index: 0,
    validatorIndex: 1,
    datadir: process.env.NODE0_DATADIR,
    nodekey: process.env.NODE0_NODEKEY,
    httpPort: process.env.VALIDATOR1_HTTP_PORT || '18545',
    wsPort: process.env.VALIDATOR1_WS_PORT || '18546',
    enginePort: process.env.VALIDATOR1_ENGINE_PORT || '18550',
    p2pPort: process.env.VALIDATOR1_P2P_PORT || '40401',
    mine: true,
    address: process.env.VALIDATOR1_ADDRESS,
    passwordFile: process.env.VALIDATOR1_PASSWORD
  },
  {
    name: 'validator2',
    logPrefix: 'validator2',
    index: 1,
    validatorIndex: 2,
    datadir: process.env.NODE1_DATADIR,
    nodekey: process.env.NODE1_NODEKEY,
    httpPort: process.env.VALIDATOR2_HTTP_PORT || '18547',
    wsPort: process.env.VALIDATOR2_WS_PORT || '18548',
    enginePort: process.env.VALIDATOR2_ENGINE_PORT || '18552',
    p2pPort: process.env.VALIDATOR2_P2P_PORT || '40403',
    mine: true,
    address: process.env.VALIDATOR2_ADDRESS,
    passwordFile: process.env.VALIDATOR2_PASSWORD
  },
  {
    name: 'validator3',
    logPrefix: 'validator3',
    index: 2,
    validatorIndex: 3,
    datadir: process.env.NODE2_DATADIR,
    nodekey: process.env.NODE2_NODEKEY,
    httpPort: process.env.VALIDATOR3_HTTP_PORT || '18549',
    wsPort: process.env.VALIDATOR3_WS_PORT || '18553',
    enginePort: process.env.VALIDATOR3_ENGINE_PORT || '18554',
    p2pPort: process.env.VALIDATOR3_P2P_PORT || '40405',
    mine: true,
    address: process.env.VALIDATOR3_ADDRESS,
    passwordFile: process.env.VALIDATOR3_PASSWORD
  },
  {
    name: 'syncnode',
    logPrefix: 'syncnode',
    index: 3,
    validatorIndex: 0,
    datadir: process.env.NODE3_DATADIR,
    nodekey: process.env.NODE3_NODEKEY,
    httpPort: process.env.SYNCNODE_HTTP_PORT || '18551',
    wsPort: process.env.SYNCNODE_WS_PORT || '18555',
    enginePort: process.env.SYNCNODE_ENGINE_PORT || '18556',
    p2pPort: process.env.SYNCNODE_P2P_PORT || '40407',
    mine: false,
    allowInsecureUnlock: true,
    cache: '2048'
  }
];

module.exports = {
  apps: nodeDefs.map((node) => appConfig(node.name, scriptForNode(node), argsForNode(node), node.logPrefix, coverageEnvForNode(node)))
};
