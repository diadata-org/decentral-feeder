// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "./IDIAOracleV3.sol";

/**
 * @title IPriceMethodology
 * @dev Interface for price calculation methodologies
 *      Allows different calculation strategies 
 */
interface IPriceMethodology {
    /**
     * @notice Calculates a price value from multiple oracle sources
     * @param key The asset identifier (e.g., "BTC/USD")
     * @param oracles Array of oracle contract addresses to query
     * @param timeoutSeconds Timeout period in seconds for valid values
     * @param threshold Minimum number of valid oracle values required
     * @param windowSize Maximum number of recent historical values to consider per oracle
     * @return value The calculated price value
     * @return timestamp The timestamp associated with the calculated value
     */
    function calculateValue(
        string memory key,
        address[] memory oracles,
        uint256 timeoutSeconds,
        uint256 threshold,
        uint256 windowSize
    ) external view returns (uint128 value, uint128 timestamp);
}
