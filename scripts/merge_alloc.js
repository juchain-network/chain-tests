const fs = require('fs');

const sysContractsPath = process.argv[2];
const funderAddr = process.argv[3];

if (!sysContractsPath || !funderAddr) {
    console.error("Usage: node merge_alloc.js <sys_contracts.json> <funder_addr>");
    process.exit(1);
}

const DEFAULT_FUNDER_BALANCE = "1000000000000000000000000000"; // 1B tokens
const DEFAULT_VALIDATOR_BALANCE = DEFAULT_FUNDER_BALANCE;
const DEFAULT_SIGNER_BALANCE = "1000000000000000000"; // 1 token for gas
const DEFAULT_FEE_BALANCE = "1000000000000000000"; // 1 token for gas

function parseAddressList(envKey) {
    const raw = process.env[envKey];
    if (!raw) {
        return [];
    }
    try {
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) {
            throw new Error(`${envKey} must be a JSON array`);
        }
        return parsed.filter(Boolean);
    } catch (error) {
        console.error(`Failed to parse ${envKey}:`, error.message);
        process.exit(1);
    }
}

function parsePositiveBigInt(envKey, fallback) {
    const raw = (process.env[envKey] || fallback).trim();
    try {
        const value = BigInt(raw);
        if (value < 0n) {
            throw new Error('negative balance not allowed');
        }
        return value;
    } catch (error) {
        console.error(`Invalid ${envKey}: ${raw} (${error.message})`);
        process.exit(1);
    }
}

function normalizeAddress(addr) {
    if (typeof addr !== 'string') {
        return '';
    }
    const trimmed = addr.trim();
    if (!/^0x[0-9a-fA-F]{40}$/.test(trimmed)) {
        return '';
    }
    return trimmed;
}

function toQuantity(value) {
    return `0x${value.toString(16)}`;
}

function ensureBalance(alloc, addr, minimumBalance) {
    const normalized = normalizeAddress(addr);
    if (!normalized) {
        return;
    }

    const existing = alloc[normalized] || {};
    let currentBalance = 0n;
    if (existing.balance !== undefined) {
        try {
            currentBalance = BigInt(existing.balance);
        } catch (_) {
            currentBalance = 0n;
        }
    }
    if (existing.balance !== undefined && currentBalance >= minimumBalance) {
        alloc[normalized] = existing;
        return;
    }

    const targetBalance = currentBalance >= minimumBalance ? currentBalance : minimumBalance;
    alloc[normalized] = {
        ...existing,
        balance: toQuantity(targetBalance),
    };
}

try {
    const sysContracts = JSON.parse(fs.readFileSync(sysContractsPath, 'utf8'));
    const alloc = { ...sysContracts };

    const validators = parseAddressList('BOOTSTRAP_VALIDATORS_JSON');
    const signers = parseAddressList('BOOTSTRAP_SIGNERS_JSON');
    const feeAddresses = parseAddressList('BOOTSTRAP_FEE_ADDRESSES_JSON');
    const overrideValidators = parseAddressList('UPGRADE_OVERRIDE_POSA_VALIDATORS_JSON');
    const overrideSigners = parseAddressList('UPGRADE_OVERRIDE_POSA_SIGNERS_JSON');

    const funderBalance = parsePositiveBigInt('FUNDER_BALANCE_WEI', DEFAULT_FUNDER_BALANCE);
    const validatorBalance = parsePositiveBigInt('BOOTSTRAP_VALIDATOR_BALANCE_WEI', DEFAULT_VALIDATOR_BALANCE);
    const signerBalance = parsePositiveBigInt('BOOTSTRAP_SIGNER_BALANCE_WEI', DEFAULT_SIGNER_BALANCE);
    const feeBalance = parsePositiveBigInt('BOOTSTRAP_FEE_BALANCE_WEI', DEFAULT_FEE_BALANCE);

    ensureBalance(alloc, funderAddr, funderBalance);
    validators.forEach((addr) => ensureBalance(alloc, addr, validatorBalance));
    signers.forEach((addr) => ensureBalance(alloc, addr, signerBalance));
    feeAddresses.forEach((addr) => ensureBalance(alloc, addr, feeBalance));
    overrideValidators.forEach((addr) => ensureBalance(alloc, addr, validatorBalance));
    overrideSigners.forEach((addr) => ensureBalance(alloc, addr, signerBalance));

    console.log(JSON.stringify(alloc, null, 2));
} catch (error) {
    console.error("Error merging alloc:", error);
    process.exit(1);
}
