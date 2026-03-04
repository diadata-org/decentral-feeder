// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "../IPriceMethodology.sol";
import "../IDIAOracleV3.sol";
import "../QuickSort.sol";

/**
 * @title VolumeWeightedAveragePriceMethodology
 * @notice Price methodology that calculates VWAP across multiple oracles, then takes the median
 * @dev This methodology implements a two-stage calculation combining VWAP and median:
 *
 *      Stage 1 - Per-Oracle VWAP:
 *      For each oracle, calculates the Volume Weighted Average Price
 *
 *      Only considers:
 *      - Non-expired values (within timeoutSeconds)
 *      - Values with non-zero volume
 *      - Up to windowSize most recent values
 *
 *      Stage 2 - Cross-Oracle Median:
 *      Takes the median of all per-oracle VWAPs to get the final price.
 *      This protects against outlier VWAP values from compromised oracles.
 *
 *      Algorithm:
 *      1. Query historical values from each oracle
 *      2. Filter out expired values and zero-volume entries
 *      3. Calculate VWAP for each oracle
 *      4. Calculate median of all oracle VWAPs (using QuickSort)
 *      5. Return median VWAP with maximum timestamp
 *
 *      Use Cases:
 *      - Markets where volume is a good quality indicator
 *      - Resistant to low-volume outlier prices
 *      - Combines volume weighting with outlier protection
 *
 *      Note: Oracles must provide volume data for this methodology to work correctly.
 *            Entries with zero volume are skipped.
 *
 */
contract VolumeWeightedAveragePriceMethodology is IPriceMethodology {
    error ThresholdNotMet(uint256 validValues, uint256 threshold);

    /// @notice Result struct for VWAP calculation
    /// @param vwap The calculated volume-weighted average price
    /// @param maxTimestamp The maximum (most recent) timestamp from valid entries
    /// @param valid True if the calculation produced a valid result (non-zero total volume)
    struct VWAPResult {
        uint128 vwap;
        uint128 maxTimestamp;
        bool valid;
    }

    /**
     * @notice Calculates price using VWAP methodology
     * @dev For each oracle:
     *      1. Gets historical values and filters by timeout
     *      2. Calculates VWAP from non-expired, non-zero volume entries
     *      3. Takes median of all oracle VWAPs
     *
     * @param key The asset identifier (e.g., "BTC/USD")
     * @param oracles Array of oracle addresses to query
     * @param timeoutSeconds Timeout period for valid values (values older than this are ignored)
     * @param threshold Minimum number of valid oracle values required to return a result
     * @param windowSize Maximum number of recent historical values to consider per oracle
     * @return value The calculated median of VWAPs across all oracles
     * @return timestamp The maximum timestamp from all valid VWAP calculations
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

        uint128[] memory vwaps = new uint128[](numOracles);
        uint256 validValues = 0;
        uint128 maxTimestamp = 0;

        for (uint256 i = 0; i < numOracles; i++) {
            VWAPResult memory result = _calculateOracleVWAP(IDIAOracleV3(oracles[i]), key, timeoutSeconds, windowSize);

            if (result.valid) {
                vwaps[validValues] = result.vwap;
                validValues++;
                if (result.maxTimestamp > maxTimestamp) {
                    maxTimestamp = result.maxTimestamp;
                }
            }
        }

        if (validValues < threshold) {
            revert ThresholdNotMet(validValues, threshold);
        }

        vwaps = QuickSort.sort(vwaps, 0, validValues - 1);

        uint256 medianIndex = validValues / 2;
        uint128 medianValue;
        if (validValues % 2 == 0) {
            uint256 lowerIndex = medianIndex - 1;
            medianValue = uint128((uint256(vwaps[lowerIndex]) + uint256(vwaps[medianIndex])) / 2);
        } else {
            medianValue = vwaps[medianIndex];
        }
        return (medianValue, maxTimestamp);
    }

    /**
     * @notice Calculates VWAP for a single oracle
     * @dev Computes the Volume Weighted Average Price from historical data.
     *
     *      Formula: VWAP = Σ(price_i × volume_i) / Σ(volume_i)
     *
     *      Filtering:
     *      - Only considers up to windowSize most recent entries
     *      - Skips entries where timestamp + timeoutSeconds < block.timestamp (expired)
     *      - Skips entries with zero volume (would cause division by zero)
     *
     *      Returns invalid result if:
     *      - No historical data exists
     *      - All entries are expired
     *      - Total volume is zero
     *
     * @param oracle The oracle contract to query
     * @param key The asset identifier (e.g., "BTC/USD")
     * @param timeoutSeconds Timeout period - entries older than this are ignored
     * @param windowSize Maximum number of recent entries to consider
     * @return result VWAPResult containing vwap, maxTimestamp, and valid flag
     */
    function _calculateOracleVWAP(
        IDIAOracleV3 oracle,
        string memory key,
        uint256 timeoutSeconds,
        uint256 windowSize
    ) internal view returns (VWAPResult memory) {
        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);

        if (history.length == 0) {
            return VWAPResult(0, 0, false);
        }

        uint256 sumPriceVolume = 0;
        uint256 sumVolume = 0;
        uint128 maxTs = 0;
        uint256 maxIndex = windowSize < history.length ? windowSize : history.length;

        for (uint256 j = 0; j < maxIndex; j++) {
            IDIAOracleV3.ValueEntry memory entry = history[j];

            if ((entry.timestamp + timeoutSeconds) < block.timestamp) {
                continue;
            }

            if (entry.volume == 0) {
                continue;
            }

            sumPriceVolume += uint256(entry.value) * uint256(entry.volume);
            sumVolume += entry.volume;

            if (entry.timestamp > maxTs) {
                maxTs = entry.timestamp;
            }
        }

        if (sumVolume == 0) {
            return VWAPResult(0, 0, false);
        }

        return VWAPResult(uint128(sumPriceVolume / sumVolume), maxTs, true);
    }
}
