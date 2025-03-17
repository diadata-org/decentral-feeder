// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.26;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

interface IDIAOracleV2 {                                                           
    function setValue(string memory key, uint128 value, uint128 timestamp) external;
    function getValue(string memory key) external view returns (uint128, uint128);
    function setMultipleValues(string[] memory keys, uint256[] memory compressedValues) external;
}

/**
 * @title DIAOracleV2
 * @dev A simple oracle contract that allows an authorized updater to set and retrieve price values with timestamps.
 */
contract DIAOracleV2 is IDIAOracleV2, AccessControl {
    bytes32 public constant UPDATER_ROLE = keccak256("UPDATER_ROLE");
    /// @notice Mapping to store compressed values of assets (price and timestamp).
    /// @dev The stored value is a 256-bit integer where the upper 128 bits store the price and the lower 128 bits store the timestamp.
    mapping (string => uint256) public values;
    
    event OracleUpdate(string key, uint128 value, uint128 timestamp);
    event UpdaterAddressChange(address newUpdater);
    
    constructor() {
        _grantRole(UPDATER_ROLE, msg.sender);
    }
    
     /**
     * @notice Updates the price and timestamp for a given asset key.
     * @dev Only callable by the `oracleUpdater`.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @param value The price value to set.
     * @param timestamp The timestamp associated with the value.
     */
    function setValue(string memory key, uint128 value, uint128 timestamp) public {
        require(hasRole(UPDATER_ROLE, msg.sender), "Only the oracleUpdater role can update the oracle.");
        uint256 cValue = (((uint256)(value)) << 128) + timestamp;
        values[key] = cValue;
        emit OracleUpdate(key, value, timestamp);
    }

    /**
     * @notice Updates multiple asset values in a single transaction.
     * @dev Each entry in `compressedValues` should be a 256-bit integer where:
     *      - The upper 128 bits represent the price value.
     *      - The lower 128 bits represent the timestamp.
     * @param keys The array of asset identifiers.
     * @param compressedValues The array of compressed values (price and timestamp combined).
     */

    function setMultipleValues(string[] memory keys, uint256[] memory compressedValues) public {
        require(hasRole(UPDATER_ROLE, msg.sender), "Only the oracleUpdater role can update the oracle.");
        require(keys.length == compressedValues.length);
        
        for (uint128 i = 0; i < keys.length; i++) {
            string memory currentKey = keys[i];
            uint256 currentCvalue = compressedValues[i];
            uint128 value = (uint128)(currentCvalue >> 128);
            uint128 timestamp = (uint128)(currentCvalue % 2**128);

            values[currentKey] = currentCvalue;
            emit OracleUpdate(currentKey, value, timestamp);
        }
    }

    /**
     * @notice Retrieves the price and timestamp for a given asset key.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return value The stored price value.
     * @return timestamp The stored timestamp.
     */
    function getValue(string memory key) external view returns (uint128, uint128) {
        uint256 cValue = values[key];
        uint128 timestamp = (uint128)(cValue % 2**128);
        uint128 value = (uint128)(cValue >> 128);
        return (value, timestamp);
    }
}
