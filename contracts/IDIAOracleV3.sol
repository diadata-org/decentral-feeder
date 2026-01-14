// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

interface IDIAOracleV3 {
    struct ValueEntry {
        uint128 value;
        uint128 timestamp;
    }

    function setValue(string memory key, uint128 value, uint128 timestamp) external;
    function getValue(string memory key) external view returns (uint128, uint128);
    function setMultipleValues(string[] memory keys, uint256[] memory compressedValues) external;
    
    // Historical value functions
    function getValueAt(string memory key, uint256 index) external view returns (uint128 value, uint128 timestamp);
    function getValueHistory(string memory key) external view returns (ValueEntry[] memory);
    function getValueCount(string memory key) external view returns (uint256);
    function setMaxHistorySize(uint256 newMaxSize) external;
    function getMaxHistorySize() external view returns (uint256);
}
