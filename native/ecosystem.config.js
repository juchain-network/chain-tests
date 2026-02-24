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
    if (!process.env[key]) {
      process.env[key] = val;
    }
  }
}
loadEnvFile(envFile);

const ns = process.env.PM2_NAMESPACE || 'ju-chain';
const geth = process.env.GETH_BINARY || path.resolve(__dirname, '../docker/juchain');
const logDir = process.env.NATIVE_LOG_DIR || path.resolve(__dirname, '../data/native-logs');
const networkId = process.env.NETWORK_ID || '666666';
const bootnodes = process.env.BOOTNODES || '';
const stateScheme = process.env.STATE_SCHEME || '';
const historyState = process.env.HISTORY_STATE || '';

function nameOf(suffix) {
  return `${ns}-${suffix}`;
}

function commonArgs(opts) {
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

  if (opts.mine) {
    args.push(
      '--mine',
      '--miner.etherbase', opts.address,
      '--miner.gasprice', '0',
      '--unlock', opts.address,
      '--password', opts.passwordFile,
      '--allow-insecure-unlock'
    );
  }

  return args;
}

function appConfig(appName, args, logPrefix) {
  return {
    name: nameOf(appName),
    script: geth,
    args,
    cwd: process.cwd(),
    instances: 1,
    autorestart: true,
    watch: false,
    max_memory_restart: '2G',
    env: {
      NODE_ENV: 'production'
    },
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

module.exports = {
  apps: [
    appConfig(
      'validator1',
      commonArgs({
        datadir: process.env.NODE0_DATADIR,
        nodekey: process.env.NODE0_NODEKEY,
        httpPort: process.env.VALIDATOR1_HTTP_PORT || '18545',
        wsPort: process.env.VALIDATOR1_WS_PORT || '18546',
        enginePort: process.env.VALIDATOR1_ENGINE_PORT || '18550',
        p2pPort: process.env.VALIDATOR1_P2P_PORT || '30301',
        mine: true,
        address: process.env.VALIDATOR1_ADDRESS,
        passwordFile: process.env.VALIDATOR1_PASSWORD
      }),
      'validator1'
    ),
    appConfig(
      'validator2',
      commonArgs({
        datadir: process.env.NODE1_DATADIR,
        nodekey: process.env.NODE1_NODEKEY,
        httpPort: process.env.VALIDATOR2_HTTP_PORT || '18547',
        wsPort: process.env.VALIDATOR2_WS_PORT || '18548',
        enginePort: process.env.VALIDATOR2_ENGINE_PORT || '18552',
        p2pPort: process.env.VALIDATOR2_P2P_PORT || '30303',
        mine: true,
        address: process.env.VALIDATOR2_ADDRESS,
        passwordFile: process.env.VALIDATOR2_PASSWORD
      }),
      'validator2'
    ),
    appConfig(
      'validator3',
      commonArgs({
        datadir: process.env.NODE2_DATADIR,
        nodekey: process.env.NODE2_NODEKEY,
        httpPort: process.env.VALIDATOR3_HTTP_PORT || '18549',
        wsPort: process.env.VALIDATOR3_WS_PORT || '18553',
        enginePort: process.env.VALIDATOR3_ENGINE_PORT || '18554',
        p2pPort: process.env.VALIDATOR3_P2P_PORT || '30305',
        mine: true,
        address: process.env.VALIDATOR3_ADDRESS,
        passwordFile: process.env.VALIDATOR3_PASSWORD
      }),
      'validator3'
    ),
    appConfig(
      'syncnode',
      commonArgs({
        datadir: process.env.NODE3_DATADIR,
        nodekey: process.env.NODE3_NODEKEY,
        httpPort: process.env.SYNCNODE_HTTP_PORT || '18551',
        wsPort: process.env.SYNCNODE_WS_PORT || '18555',
        enginePort: process.env.SYNCNODE_ENGINE_PORT || '18556',
        p2pPort: process.env.SYNCNODE_P2P_PORT || '30307',
        mine: false,
        cache: '2048'
      }),
      'syncnode'
    )
  ]
};
