const fs = require('fs');

const sysContractsPath = process.argv[2];
const funderAddr = process.argv[3];
const validatorAddrs = process.argv[4].split(',');

if (!sysContractsPath || !funderAddr || !validatorAddrs) {
    console.error("Usage: node merge_alloc.js <sys_contracts.json> <funder_addr> <val_addrs_comma_sep>");
    process.exit(1);
}

try {
    const sysContracts = JSON.parse(fs.readFileSync(sysContractsPath, 'utf8'));
    const alloc = { ...sysContracts };

    const balance = "1000000000000000000000000000"; // 1B tokens

    // Add Funder
    alloc[funderAddr] = { balance: balance };

    // Add Validators
    for (const addr of validatorAddrs) {
        if (addr) {
            alloc[addr] = { balance: balance };
        }
    }

    console.log(JSON.stringify(alloc, null, 2));

} catch (error) {
    console.error("Error merging alloc:", error);
    process.exit(1);
}
