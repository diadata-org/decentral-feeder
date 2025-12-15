// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "../IPriceMethodology.sol";
import "../IDIAOracleV3.sol";
import "../QuickSort.sol";

/**
 * @title AveragePriceMethodology
 * @dev Calculates price by averaging all historical values from each oracle,
 *      then taking the median of those averages
 */
contract AveragePriceMethodology is IPriceMethodology {
    error ThresholdNotMet(uint256 validValues, uint256 threshold);
    
    /**
     * @notice Calculates price using average methodology
     * @dev For each oracle:
     *      1. Gets all historical values using getValueHistory()
     *      2. Calculates average of all non-expired historical values
     *      3. Takes median of all oracle averages
     * @param key The asset identifier
     * @param oracles Array of oracle addresses
     * @param timeoutSeconds Timeout period for valid values
     * @param threshold Minimum number of valid oracle values required
     * @return value The calculated median of averages
     * @return timestamp Current block timestamp
     */
    function calculateValue(
        string memory key,
        address[] memory oracles,
        uint256 timeoutSeconds,
        uint256 threshold
    ) external view override returns (uint128 value, uint128 timestamp) {
        // Convert address array to IDIAOracleV3 array
        IDIAOracleV3[] memory oracleContracts = new IDIAOracleV3[](oracles.length);
        for (uint256 i = 0; i < oracles.length; i++) {
            oracleContracts[i] = IDIAOracleV3(oracles[i]);
        }
        
        return _aggregateAverageMedian(
            oracleContracts,
            key,
            timeoutSeconds,
            threshold
        );
    }
    
    /**
     * @notice Aggregates oracle values using average-then-median methodology.
     * @dev For each oracle: averages all valid (non-expired) historical values,
     *      then takes the median of those averages.
     * @param oracles Array of oracle contracts to aggregate from.
     * @param key The asset identifier.
     * @param timeoutSeconds Timeout period for valid values.
     * @param threshold Minimum number of valid oracle values required.
     * @return value The aggregated value (median of averages).
     * @return timestamp The timestamp associated with the result.
     */
    function _aggregateAverageMedian(
        IDIAOracleV3[] memory oracles,
        string memory key,
        uint256 timeoutSeconds,
        uint256 threshold
    ) internal view returns (uint128 value, uint128 timestamp) {
        uint256 numOracles = oracles.length;
        if (numOracles == 0) {
            return (0, uint128(block.timestamp));
        }

        uint128[] memory averages = new uint128[](numOracles);
        uint256 validValues = 0;

        for (uint256 i = 0; i < numOracles; i++) {
            IDIAOracleV3 currOracle = oracles[i];

            // Get all historical values from this oracle
            IDIAOracleV3.ValueEntry[] memory history = currOracle.getValueHistory(key);
            
            if (history.length == 0) {
                continue; // Skip oracles with no history
            }

            // Calculate average of all historical values,only count non-expired ones
            uint256 sum = 0;
            uint256 validCount = 0;
            
            for (uint256 j = 0; j < history.length; j++) {
                 if ((history[j].timestamp + timeoutSeconds) < block.timestamp) {
                    continue;
                }
                sum += history[j].value;
                validCount += 1;
            }

             if (validCount > 0) {
                averages[validValues] = uint128(sum / validCount);
                validValues += 1;
            }
        }

        if (validValues < threshold) {
            revert ThresholdNotMet(validValues, threshold);
        }

         averages = QuickSort.sort(averages, 0, validValues - 1);

         uint256 medianIndex = validValues / 2;
        return (averages[medianIndex], uint128(block.timestamp));
    }
}
