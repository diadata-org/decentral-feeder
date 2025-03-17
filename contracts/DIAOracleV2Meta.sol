// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.26;

import "https://github.com/OpenZeppelin/openzeppelin-contracts/blob/master/contracts/access/Ownable.sol";

interface IDIAOracleV2 {                                                           
    function setValue(string memory key, uint128 value, uint128 timestamp) external;
    function getValue(string memory key) external view returns (uint128, uint128);
    function updateOracleUpdaterAddress(address newOracleUpdaterAddress) external;
}

/**
* @title sort given array using Quick sort.
* @author [Priyda](https://github.com/priyda)
*/
library QuickSort {
    function sort(uint128[] memory data) public returns (uint128[] memory) {
        quickSort(data, int256(0), int256(data.length - 1));
        return data;
    }

    /** Quicksort is a sorting algorithm based on the divide and conquer approach **/
    function quickSort(
        uint128[] memory _arr,
        int256 left,
        int256 right
    ) internal {
        int256 i = left;
        int256 j = right;
        if (i == j) return;
        uint256 pivot = _arr[uint256(left + (right - left) / 2)];
        while (i <= j) {
            while (_arr[uint256(i)] < pivot) i++;
            while (pivot < _arr[uint256(j)]) j--;
            if (i <= j) {
                (_arr[uint256(i)], _arr[uint256(j)]) = (
                    _arr[uint256(j)],
                    _arr[uint256(i)]
                );
                i++;
                j--;
            }
        }
        if (left < j) quickSort(_arr, left, j);
        if (i < right) quickSort(_arr, i, right);
    }
}

/**
 * @title DIAOracleV2Meta
 */
contract DIAOracleV2Meta is Ownable(msg.sender) {
    /// @notice Mapping of registered oracle addresses.
    mapping (uint256 => address) oracles;

    /// @notice Number of registered oracles.
    uint256 private numOracles;
    
    /// @notice Minimum number of valid values required to return a result.
    uint256 private threshold;

    /// @notice The timeout period in seconds for oracle values.
    uint256 private timeoutSeconds;

    event OracleAdded(address newOracleAddress);
    event OracleRemoved(address removedOracleAddress);

    /**
     * @notice Adds a new oracle to the registry.
     * @dev Only the administrator can call this function.
     * @param newOracleAddress The address of the oracle contract to add.
     */
    function addOracle(address newOracleAddress) onlyOwner public {
        require(newOracleAddress != address(0));
        for (uint256 i = 0; i < numOracles; i++) {
            if (oracles[i] == newOracleAddress) {
                revert("Oracle already added.");
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

    function removeOracle(address oracleToRemove) onlyOwner public {
        for (uint256 i = 0; i < numOracles; i++) {
            if (oracles[i] == oracleToRemove) {
                oracles[i] = oracles[numOracles - 1];
                oracles[numOracles] = address(0);
                numOracles--;
                emit OracleRemoved(oracleToRemove);
                return;
            }
        }
        revert("Oracle not found because it was not in the registry.");
    }

     /**
     * @notice Sets the required threshold of valid oracle values.
     * @dev Only the administrator can call this function.
     * @param newThreshold The new threshold value.
     */

    function setThreshold(uint256 newThreshold) onlyOwner public {
        require(newThreshold > 0);
        threshold = newThreshold;
    }

    /**
     * @notice Sets the timeout period for oracle values.
     * @dev Only the administrator can call this function.
     * @param newTimeoutSeconds The new timeout period in seconds.
     */
    function setTimeoutSeconds(uint256 newTimeoutSeconds) onlyOwner public {
        require(newTimeoutSeconds > 0);
        // Timeout should be at most one day
        require(newTimeoutSeconds <= 86400);
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
        require(timeoutSeconds > 0);
        require(threshold > 0);

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

        // Sort by value to retrieve the median
        values = QuickSort.sort(values);

        // Check that we have enough values
        require(validValues >= threshold);
        
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
