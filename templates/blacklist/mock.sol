// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract BlacklistManagerMock {
    mapping(address => bool) private blacklisted;
    address[] private entries;

    event AddedToBlacklist(address indexed addr, uint256 timestamp);
    event BatchAddedToBlacklist(address[] addresses, uint256 timestamp);
    event RemovedFromBlacklist(address indexed addr, uint256 timestamp);
    event BatchRemovedFromBlacklist(address[] addresses, uint256 timestamp);

    function getAllBlacklistedAddresses() external view returns (address[] memory) {
        return entries;
    }

    function addToBlacklist(address addr) external {
        if (!blacklisted[addr]) {
            blacklisted[addr] = true;
            entries.push(addr);
        }
        emit AddedToBlacklist(addr, block.timestamp);
    }

    function addBatchToBlacklist(address[] calldata addresses) external {
        for (uint256 i = 0; i < addresses.length; i++) {
            address addr = addresses[i];
            if (!blacklisted[addr]) {
                blacklisted[addr] = true;
                entries.push(addr);
            }
        }
        emit BatchAddedToBlacklist(addresses, block.timestamp);
    }

    function removeFromBlacklist(address addr) external {
        if (blacklisted[addr]) {
            blacklisted[addr] = false;
            _remove(addr);
        }
        emit RemovedFromBlacklist(addr, block.timestamp);
    }

    function removeBatchFromBlacklist(address[] calldata addresses) external {
        for (uint256 i = 0; i < addresses.length; i++) {
            address addr = addresses[i];
            if (blacklisted[addr]) {
                blacklisted[addr] = false;
                _remove(addr);
            }
        }
        emit BatchRemovedFromBlacklist(addresses, block.timestamp);
    }

    function _remove(address addr) internal {
        uint256 len = entries.length;
        for (uint256 i = 0; i < len; i++) {
            if (entries[i] == addr) {
                entries[i] = entries[len - 1];
                entries.pop();
                break;
            }
        }
    }
}
