// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "../IPriceMethodology.sol";
import "../IDIAOracleV3.sol";

/**
 * @title VolumeWeightedOracleMethodology
 * @dev Calculates price by weighting each oracle's contribution by its total volume.
 *      Oracles with higher volume have more influence on the final price.
 *      This methodology assumes higher volume indicates more reliable/liquid sources.
 */
contract VolumeWeightedOracleMethodology is IPriceMethodology {
    error ThresholdNotMet(uint256 validValues, uint256 threshold);
    error NoVolumeData();

    struct OracleResult {
        uint256 avgPrice;
        uint256 totalVolume;
        uint128 maxTimestamp;
        bool valid;
    }

    /**
     * @notice Calculates price using volume-weighted oracle methodology
     * @param key The asset identifier
     * @param oracles Array of oracle addresses
     * @param timeoutSeconds Timeout period for valid values
     * @param threshold Minimum number of valid oracle values required
     * @param windowSize Maximum number of recent historical values to consider
     * @return value The volume-weighted average price across oracles
     * @return timestamp Most recent timestamp from valid oracles
     */
    function calculateValue(
        string memory key,
        address[] memory oracles,
        uint256 timeoutSeconds,
        uint256 threshold,
        uint256 windowSize
    ) external view override returns (uint128 value, uint128 timestamp) {
        uint256 numOracles = oracles.length;
        if (numOracles == 0) {
            return (0, uint128(block.timestamp));
        }

        uint256 totalPriceVolume = 0;
        uint256 totalVolume = 0;
        uint256 validOracleCount = 0;
        uint128 maxTimestamp = 0;

        for (uint256 i = 0; i < numOracles; i++) {
            OracleResult memory result = _calculateOracleData(IDIAOracleV3(oracles[i]), key, timeoutSeconds, windowSize);

            if (result.valid) {
                totalPriceVolume += result.avgPrice * result.totalVolume;
                totalVolume += result.totalVolume;
                validOracleCount++;
                if (result.maxTimestamp > maxTimestamp) {
                    maxTimestamp = result.maxTimestamp;
                }
            }
        }

        if (validOracleCount < threshold) {
            revert ThresholdNotMet(validOracleCount, threshold);
        }

        if (totalVolume == 0) {
            revert NoVolumeData();
        }

        value = uint128(totalPriceVolume / totalVolume);
        return (value, maxTimestamp);
    }

    /**
     * @notice Calculates average price and total volume for a single oracle
     */
    function _calculateOracleData(IDIAOracleV3 oracle, string memory key, uint256 timeoutSeconds, uint256 windowSize)
        internal
        view
        returns (OracleResult memory)
    {
        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);

        if (history.length == 0) {
            return OracleResult(0, 0, 0, false);
        }

        uint256 priceSum = 0;
        uint256 volumeSum = 0;
        uint256 validCount = 0;
        uint128 maxTs = 0;
        uint256 maxIndex = windowSize < history.length ? windowSize : history.length;

        for (uint256 j = 0; j < maxIndex; j++) {
            IDIAOracleV3.ValueEntry memory entry = history[j];

            if ((entry.timestamp + timeoutSeconds) < block.timestamp) {
                continue;
            }

            priceSum += entry.value;
            volumeSum += entry.volume;
            validCount++;

            if (entry.timestamp > maxTs) {
                maxTs = entry.timestamp;
            }
        }

        if (validCount == 0 || volumeSum == 0) {
            return OracleResult(0, 0, 0, false);
        }

        return OracleResult(priceSum / validCount, volumeSum, maxTs, true);
    }
}
