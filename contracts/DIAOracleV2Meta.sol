// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import "./IDIAOracleV2.sol";
import "./QuickSort.sol";

/**
 * @title DIAOracleV2Meta
 */
contract DIAOracleV2Meta is Ownable(msg.sender) {
    /// @notice Mapping of registered oracle addresses.
    mapping(uint256 => address) public oracles;

    /// @notice Number of registered oracles.
    uint256 private numOracles;

    /// @notice Minimum number of valid values required to return a result.
    uint256 private threshold;

    /// @notice The timeout period in seconds for oracle values.
    uint256 private timeoutSeconds;

    event OracleAdded(address newOracleAddress);
    event OracleRemoved(address removedOracleAddress);

    error OracleNotFound();
    error ZeroAddress();
    error InvalidThreshold(uint256 value);
    error InvalidTimeOut(uint256 value);
    error TimeoutExceedsLimit(uint256 value);
    error OracleExists();
    error ThresholdNotMet(uint256 validValues, uint256 threshold);


    modifier validateAddress(address _address) {
        if (_address == address(0)) revert ZeroAddress();
        _;
    }

    /**
     * @notice Adds a new oracle to the registry.
     * @dev Only the administrator can call this function.
     * @param newOracleAddress The address of the oracle contract to add.
     */
    function addOracle(
        address newOracleAddress
    ) public onlyOwner validateAddress(newOracleAddress) {
        for (uint256 i = 0; i < numOracles; i++) {
            if (oracles[i] == newOracleAddress) {
                revert OracleExists();
            }
        }
        oracles[numOracles] = newOracleAddress;
        numOracles += 1;
        emit OracleAdded(newOracleAddress);
    }

    /**
     * @notice Removes an oracle from the registry.
     * @dev Only the administrator can call this function.
     * @param oracleToRemove The address of the oracle contract to remove.
     */

    function removeOracle(address oracleToRemove) public onlyOwner {
        for (uint256 i = 0; i < numOracles; i++) {
            if (oracles[i] == oracleToRemove) {
                oracles[i] = oracles[numOracles - 1];
                oracles[numOracles - 1] = address(0);
                numOracles--;
                emit OracleRemoved(oracleToRemove);
                return;
            }
        }
        revert OracleNotFound();
    }

    /**
     * @notice Sets the required threshold of valid oracle values.
     * @dev Only the administrator can call this function.
     * @param newThreshold The new threshold value.
     */

    function setThreshold(uint256 newThreshold) public onlyOwner {
        if (newThreshold == 0) {
            revert InvalidThreshold(newThreshold);
        }
        threshold = newThreshold;
    }

    /**
     * @notice Sets the timeout period for oracle values.
     * @dev Only the administrator can call this function.
     * @param newTimeoutSeconds The new timeout period in seconds.
     */
    function setTimeoutSeconds(uint256 newTimeoutSeconds) public onlyOwner {
        if (newTimeoutSeconds == 0) {
            revert InvalidTimeOut(newTimeoutSeconds);
        }
        if (newTimeoutSeconds > 86400) {
            revert TimeoutExceedsLimit(newTimeoutSeconds);
        }

        // Timeout should be at most one day
        timeoutSeconds = newTimeoutSeconds;
    }

    /**
     * @notice Retrieves the median price value for a given asset key from registered oracles.
     * @dev Only returns values that are not older than the timeout period.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return value The median price from available oracles.
     * @return timestamp The current block timestamp.
     */

    function getValue(string memory key) external returns (uint128, uint128) { 
        if (timeoutSeconds == 0) {
            revert InvalidTimeOut(timeoutSeconds);
        }
        if (threshold == 0) {
            revert InvalidThreshold(threshold);
        }

        uint128[] memory values = new uint128[](numOracles);

        uint256 validValues = 0;

        for (uint256 i = 0; i < numOracles; i++) {
            address currAddress = oracles[i];
            uint128 currValue;
            uint128 currTimestamp;
            IDIAOracleV2 currOracle = IDIAOracleV2(currAddress);

            (currValue, currTimestamp) = currOracle.getValue(key);

            // Discard values older than threshold
            if ((currTimestamp + timeoutSeconds) < block.timestamp) {
                continue;
            }
            values[validValues] = currValue;

            validValues += 1;
        }

        if (validValues < threshold) {
            revert ThresholdNotMet(validValues, threshold);
        }

        // Sort by value to retrieve the median
        values = QuickSort.sort(values, 0, validValues - 1);

        // Get median value and timestamp
        uint256 medianIndex = validValues / 2;

        return (values[medianIndex], uint128(block.timestamp));
    }

    function getNumOracles() external view returns (uint256) {
        return numOracles;
    }

    function getThreshold() external view returns (uint256) {
        return threshold;
    }

    function getTimeoutSeconds() external view returns (uint256) {
        return timeoutSeconds;
    }
}

