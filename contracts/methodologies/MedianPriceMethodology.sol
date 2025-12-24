// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "../IPriceMethodology.sol";
import "../IDIAOracleV3.sol";
import "../QuickSort.sol";

/**
 * @title MedianPriceMethodology
 * @dev Calculates price by taking the median of all historical values from each oracle,
 *      then taking the median of those medians
 */
contract MedianPriceMethodology is IPriceMethodology {
    error ThresholdNotMet(uint256 validValues, uint256 threshold);
    
    /**
     * @notice Calculates price using median methodology
     * @dev For each oracle:
     *      1. Gets historical values using getValueHistory()
     *      2. Takes up to windowSize most recent non-expired values
     *      3. Calculates median of those values
     *      4. Takes median of all oracle medians
     * @param key The asset identifier
     * @param oracles Array of oracle addresses
     * @param timeoutSeconds Timeout period for valid values
     * @param threshold Minimum number of valid oracle values required
     * @param windowSize Maximum number of recent historical values to consider per oracle
     * @return value The calculated median of medians
     * @return timestamp Current block timestamp
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
        
        return _aggregateMedianMedian(
            oracleContracts,
            key,
            timeoutSeconds,
            threshold,
            windowSize
        );
    }
    
    /**
     * @notice Aggregates oracle values using median-then-median methodology.
     * @dev For each oracle: takes up to windowSize most recent valid (non-expired) values,
     *      calculates median, then takes the median of those medians.
     * @param oracles Array of oracle contracts to aggregate from.
     * @param key The asset identifier.
     * @param timeoutSeconds Timeout period for valid values.
     * @param threshold Minimum number of valid oracle values required.
     * @param windowSize Maximum number of recent historical values to consider per oracle.
     * @return value The aggregated value (median of medians).
     * @return timestamp The timestamp associated with the result.
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

        uint128[] memory medians = new uint128[](numOracles);
        uint256 validValues = 0;
        uint128 maxTimestamp = 0;

        for (uint256 i = 0; i < numOracles; i++) {
            IDIAOracleV3.ValueEntry[] memory history = oracles[i].getValueHistory(key);
            
            if (history.length == 0) {
                continue; 
            }

            uint128[] memory validHistoryValues = new uint128[](windowSize);
            uint256 validCount = 0;
            uint256 maxIndex = windowSize < history.length ? windowSize : history.length;
            
             for (uint256 j = 0; j < maxIndex; j++) {
                uint128 entryTimestamp = history[j].timestamp;
                if ((entryTimestamp + timeoutSeconds) < block.timestamp) {
                    continue;
                }
                validHistoryValues[validCount] = history[j].value;
                validCount += 1;
                 if (entryTimestamp > maxTimestamp) {
                    maxTimestamp = entryTimestamp;
                }
            }

             if (validCount > 0) {
                 uint128[] memory sortedValues = new uint128[](validCount);
                for (uint256 k = 0; k < validCount; k++) {
                    sortedValues[k] = validHistoryValues[k];
                }
                
                 sortedValues = QuickSort.sort(sortedValues, 0, validCount - 1);
                
                 uint256 oracleMedianIndex = validCount / 2;
                medians[validValues] = sortedValues[oracleMedianIndex];
                validValues += 1;
            }
        }

        if (validValues < threshold) {
            revert ThresholdNotMet(validValues, threshold);
        }

         medians = QuickSort.sort(medians, 0, validValues - 1);

         uint256 finalMedianIndex = validValues / 2;
        return (medians[finalMedianIndex], maxTimestamp);
    }
}
