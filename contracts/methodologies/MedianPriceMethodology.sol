// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "../IPriceMethodology.sol";
import "../IDIAOracleV3.sol";

/**
 * @title MedianPriceMethodology
 * @dev Calculates price by taking the median of all historical values from each oracle,
 *      then taking the median of those medians. Returns the timestamp of the median value.
 */
contract MedianPriceMethodology is IPriceMethodology {
    error ThresholdNotMet(uint256 validValues, uint256 threshold);

    struct ValueWithTimestamp {
        uint128 value;
        uint128 timestamp;
    }

    /**
     * @notice Calculates price using median methodology
     * @dev For each oracle:
     *      1. Gets historical values using getValueHistory()
     *      2. Takes up to windowSize most recent non-expired values
     *      3. Calculates median of those values with its timestamp
     *      4. Takes median of all oracle medians and returns its timestamp
     * @param key The asset identifier
     * @param oracles Array of oracle addresses
     * @param timeoutSeconds Timeout period for valid values
     * @param threshold Minimum number of valid oracle values required
     * @param windowSize Maximum number of recent historical values to consider per oracle
     * @return value The calculated median of medians
     * @return timestamp The timestamp of the median value
     */
    function calculateValue(
        string memory key,
        address[] memory oracles,
        uint256 timeoutSeconds,
        uint256 threshold,
        uint256 windowSize
    ) external view override returns (uint128 value, uint128 timestamp) {
        IDIAOracleV3[] memory oracleContracts = new IDIAOracleV3[](oracles.length);
        for (uint256 i = 0; i < oracles.length; i++) {
            oracleContracts[i] = IDIAOracleV3(oracles[i]);
        }

        return _aggregateMedianMedian(oracleContracts, key, timeoutSeconds, threshold, windowSize);
    }

    /**
     * @notice Aggregates oracle values using median-then-median methodology.
     * @dev For each oracle: takes up to windowSize most recent valid (non-expired) values,
     *      calculates median with its timestamp, then takes the median of those medians
     *      and returns the timestamp of that median value.
     * @param oracles Array of oracle contracts to aggregate from.
     * @param key The asset identifier.
     * @param timeoutSeconds Timeout period for valid values.
     * @param threshold Minimum number of valid oracle values required.
     * @param windowSize Maximum number of recent historical values to consider per oracle.
     * @return value The aggregated value (median of medians).
     * @return timestamp The timestamp of the median value.
     */
    function _aggregateMedianMedian(
        IDIAOracleV3[] memory oracles,
        string memory key,
        uint256 timeoutSeconds,
        uint256 threshold,
        uint256 windowSize
    ) internal view returns (uint128 value, uint128 timestamp) {
        uint256 numOracles = oracles.length;
        if (numOracles == 0) {
            return (0, uint128(block.timestamp));
        }

        ValueWithTimestamp[] memory oracleResults = new ValueWithTimestamp[](numOracles);
        uint256 validValues = 0;

        for (uint256 i = 0; i < numOracles; i++) {
            ValueWithTimestamp memory result = _calculateOracleMedian(
                oracles[i].getValueHistory(key),
                timeoutSeconds,
                windowSize
            );

            if (result.timestamp != 0) {
                oracleResults[validValues] = result;
                validValues++;
            }
        }

        if (validValues < threshold) {
            revert ThresholdNotMet(validValues, threshold);
        }

        // Sort oracle medians by value using bubble sort
        for (uint256 i = 0; i < validValues - 1; i++) {
            for (uint256 j = i + 1; j < validValues; j++) {
                if (oracleResults[i].value > oracleResults[j].value) {
                    ValueWithTimestamp memory temp = oracleResults[i];
                    oracleResults[i] = oracleResults[j];
                    oracleResults[j] = temp;
                }
            }
        }

        uint256 finalMedianIndex = validValues / 2;
        uint128 finalMedian;
        uint128 medianTimestamp;

        if (validValues % 2 == 0) {
            // Even: average of two middle values, use max timestamp of the two
            uint256 lowerIndex = finalMedianIndex - 1;
            finalMedian = uint128(
                (uint256(oracleResults[lowerIndex].value) + uint256(oracleResults[finalMedianIndex].value)) / 2
            );
            medianTimestamp = oracleResults[lowerIndex].timestamp > oracleResults[finalMedianIndex].timestamp
                ? oracleResults[lowerIndex].timestamp
                : oracleResults[finalMedianIndex].timestamp;
        } else {
            // Odd: use middle value and its timestamp
            finalMedian = oracleResults[finalMedianIndex].value;
            medianTimestamp = oracleResults[finalMedianIndex].timestamp;
        }

        return (finalMedian, medianTimestamp);
    }

    /**
     * @notice Calculates median value and its timestamp for a single oracle
     * @dev Returns the median value and the timestamp of that median value
     * @param history Array of value entries from the oracle
     * @param timeoutSeconds Timeout period for valid values
     * @param windowSize Maximum number of recent values to consider
     * @return result The median value and its timestamp
     */
    function _calculateOracleMedian(
        IDIAOracleV3.ValueEntry[] memory history,
        uint256 timeoutSeconds,
        uint256 windowSize
    ) internal view returns (ValueWithTimestamp memory result) {
        if (history.length == 0) {
            return ValueWithTimestamp(0, 0);
        }

        ValueWithTimestamp[] memory validEntries = new ValueWithTimestamp[](windowSize);
        uint256 validCount = 0;
        uint256 maxIndex = windowSize < history.length ? windowSize : history.length;

        for (uint256 j = 0; j < maxIndex; j++) {
            uint128 entryTimestamp = history[j].timestamp;
            if ((entryTimestamp + timeoutSeconds) < block.timestamp) {
                continue;
            }
            validEntries[validCount] = ValueWithTimestamp(history[j].value, entryTimestamp);
            validCount++;
        }

        if (validCount == 0) {
            return ValueWithTimestamp(0, 0);
        }

        // Sort by value using bubble sort
        for (uint256 i = 0; i < validCount - 1; i++) {
            for (uint256 j = i + 1; j < validCount; j++) {
                if (validEntries[i].value > validEntries[j].value) {
                    ValueWithTimestamp memory temp = validEntries[i];
                    validEntries[i] = validEntries[j];
                    validEntries[j] = temp;
                }
            }
        }

        uint256 medianIndex = validCount / 2;
        uint128 medianValue;
        uint128 medianTimestamp;

        if (validCount % 2 == 0) {
            // Even: average of two middle values
            uint256 lowerIndex = medianIndex - 1;
            medianValue = uint128(
                (uint256(validEntries[lowerIndex].value) + uint256(validEntries[medianIndex].value)) / 2
            );
            // Use max timestamp of the two median values
            medianTimestamp = validEntries[lowerIndex].timestamp > validEntries[medianIndex].timestamp
                ? validEntries[lowerIndex].timestamp
                : validEntries[medianIndex].timestamp;
        } else {
            // Odd: use middle value
            medianValue = validEntries[medianIndex].value;
            medianTimestamp = validEntries[medianIndex].timestamp;
        }

        return ValueWithTimestamp(medianValue, medianTimestamp);
    }
}
