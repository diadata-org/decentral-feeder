// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "../IPriceMethodology.sol";
import "../IDIAOracleV3.sol";

/**
 * @title AveragePriceMethodology
 * @dev Calculates price by averaging all historical values from each oracle,
 *      then taking the median of those averages. Returns the timestamp of the median value.
 */
contract AveragePriceMethodology is IPriceMethodology {
    error ThresholdNotMet(uint256 validValues, uint256 threshold);

    struct ValueWithTimestamp {
        uint128 value;
        uint128 timestamp;
    }

    /**
     * @notice Calculates price using average methodology
     * @dev For each oracle:
     *      1. Gets historical values using getValueHistory()
     *      2. Takes up to windowSize most recent non-expired values
     *      3. Calculates average of those values with max timestamp
     *      4. Takes median of all oracle averages and returns its timestamp
     * @param key The asset identifier
     * @param oracles Array of oracle addresses
     * @param timeoutSeconds Timeout period for valid values
     * @param threshold Minimum number of valid oracle values required
     * @param windowSize Maximum number of recent historical values to consider per oracle
     * @return value The calculated median of averages
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

        return _aggregateAverageMedian(oracleContracts, key, timeoutSeconds, threshold, windowSize);
    }

    /**
     * @notice Aggregates oracle values using average-then-median methodology.
     * @dev For each oracle: takes up to windowSize most recent valid values, averages them,
     *      then takes the median of those averages and returns the timestamp of that median.
     * @param oracles Array of oracle contracts to aggregate from.
     * @param key The asset identifier.
     * @param timeoutSeconds Timeout period for valid values.
     * @param threshold Minimum number of valid oracle values required.
     * @param windowSize Maximum number of recent historical values to consider per oracle.
     * @return value The aggregated value (median of averages).
     * @return timestamp The timestamp of the median value.
     */
    function _aggregateAverageMedian(
        IDIAOracleV3[] memory oracles,
        string memory key,
        uint256 timeoutSeconds,
        uint256 threshold,
        uint256 windowSize
    ) internal view returns (uint128 value, uint128 timestamp) {
        uint256 numOracles = oracles.length;

        ValueWithTimestamp[] memory oracleResults = new ValueWithTimestamp[](numOracles);
        uint256 validValues = 0;

        for (uint256 i = 0; i < numOracles; i++) {
            ValueWithTimestamp memory result = _calculateOracleAverage(
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

        // Sort oracle averages by value using bubble sort
        for (uint256 i = 0; i < validValues - 1; i++) {
            for (uint256 j = i + 1; j < validValues; j++) {
                if (oracleResults[i].value > oracleResults[j].value) {
                    ValueWithTimestamp memory temp = oracleResults[i];
                    oracleResults[i] = oracleResults[j];
                    oracleResults[j] = temp;
                }
            }
        }

        uint256 medianIndex = validValues / 2;
        uint128 medianValue;
        uint128 medianTimestamp;

        if (validValues % 2 == 0) {
            // Even: average of two middle values, use max timestamp of the two
            uint256 lowerIndex = medianIndex - 1;
            medianValue = uint128(
                (uint256(oracleResults[lowerIndex].value) + uint256(oracleResults[medianIndex].value)) / 2
            );
            medianTimestamp = oracleResults[lowerIndex].timestamp > oracleResults[medianIndex].timestamp
                ? oracleResults[lowerIndex].timestamp
                : oracleResults[medianIndex].timestamp;
        } else {
            // Odd: use middle value and its timestamp
            medianValue = oracleResults[medianIndex].value;
            medianTimestamp = oracleResults[medianIndex].timestamp;
        }

        return (medianValue, medianTimestamp);
    }

    /**
     * @notice Calculates average value and its max timestamp 
     * @dev Returns the average value and the maximum timestamp of values used in the average
     * @param history Array of value entries from the oracle
     * @param timeoutSeconds Timeout period for valid values
     * @param windowSize Maximum number of recent values to consider
     * @return result The average value and its max timestamp
     */
    function _calculateOracleAverage(
        IDIAOracleV3.ValueEntry[] memory history,
        uint256 timeoutSeconds,
        uint256 windowSize
    ) internal view returns (ValueWithTimestamp memory result) {
        if (history.length == 0) {
            return ValueWithTimestamp(0, 0);
        }

        uint256 sum = 0;
        uint256 validCount = 0;
        uint128 maxTs = 0;
        uint256 maxIndex = windowSize < history.length ? windowSize : history.length;

        for (uint256 j = 0; j < maxIndex; j++) {
            uint128 entryTimestamp = history[j].timestamp;
            if ((entryTimestamp + timeoutSeconds) < block.timestamp) {
                continue;
            }
            sum += history[j].value;
            validCount += 1;
            if (entryTimestamp > maxTs) {
                maxTs = entryTimestamp;
            }
        }

        if (validCount == 0) {
            return ValueWithTimestamp(0, 0);
        }

        return ValueWithTimestamp(uint128(sum / validCount), maxTs);
    }
}
