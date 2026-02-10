// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import {IERC165} from "@openzeppelin/contracts/utils/introspection/IERC165.sol";

interface IDIAOracleV3 is IERC165 {
    struct ValueEntry {
        uint128 value;
        uint128 timestamp;
        uint128 volume;
    }

    function setValue(string memory key, uint128 value, uint128 timestamp) external;
    function getValue(string memory key) external view returns (uint128, uint128);
    function setMultipleValues(string[] memory keys, uint256[] memory compressedValues) external;
    function setRawValue(bytes calldata data) external;
    function setMultipleRawValues(bytes[] calldata dataArray) external;
    function getRawData(string memory key) external view returns (bytes memory);

    // Historical value functions
    function getValueAt(string memory key, uint256 index)
        external
        view
        returns (uint128 value, uint128 timestamp, uint128 volume);
    function getValueHistory(string memory key) external view returns (ValueEntry[] memory);
    function getValueCount(string memory key) external view returns (uint256);
    function setMaxHistorySize(uint256 newMaxSize) external;
    function getMaxHistorySize() external view returns (uint256);
}
