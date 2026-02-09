// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "../IPriceMethodology.sol";
import "../IDIAOracleV3.sol";
import "../QuickSort.sol";

/**
 * @title VolumeWeightedAveragePriceMethodology
 * @dev Calculates price using Volume Weighted Average Price (VWAP).
 *      For each oracle, calculates VWAP from historical values,
 *      then takes the median of those VWAPs across oracles.
 */
contract VolumeWeightedAveragePriceMethodology is IPriceMethodology {
    error ThresholdNotMet(uint256 validValues, uint256 threshold);
    
    struct VWAPResult {
        uint128 vwap;
        uint128 maxTimestamp;
        bool valid;
    }
    
    /**
     * @notice Calculates price using VWAP methodology
     * @param key The asset identifier
     * @param oracles Array of oracle addresses
     * @param timeoutSeconds Timeout period for valid values
     * @param threshold Minimum number of valid oracle values required
     * @param windowSize Maximum number of recent historical values to consider per oracle
     * @return value The calculated median of VWAPs
     * @return timestamp Current block timestamp
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
            VWAPResult memory result = _calculateOracleVWAP(
                IDIAOracleV3(oracles[i]),
                key,
                timeoutSeconds,
                windowSize
            );
            
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
        return (vwaps[validValues / 2], maxTimestamp);
    }
    
    /**
     * @notice Calculates VWAP for a single oracle
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
