// SPDX-License-Identifier: GPL

pragma solidity 0.8.26;

interface IDIAOracleV2 {                                                           
    function setValue(string memory key, uint128 value, uint128 timestamp) external;
    function getValue(string memory key) external view returns (uint128, uint128);
    function updateOracleUpdaterAddress(address newOracleUpdaterAddress) external;
}

/**
 * @title DIAOracleV2Meta
 */
contract DIAOracleV2Meta {
    /// @notice Mapping of registered oracle addresses.
    mapping (uint256 => address) oracles;

    /// @notice Number of registered oracles.
    uint256 private numOracles;
    
    /// @notice Minimum number of valid values required to return a result.
    uint256 private threshold;

    /// @notice The timeout period in seconds for oracle values.
    uint256 private timeoutSeconds;

    /// @notice Address of the administrator who can manage oracles.
    address admin;

    constructor() {
        admin = msg.sender;
    }

    /**
     * @notice Adds a new oracle to the registry.
     * @dev Only the administrator can call this function.
     * @param newOracleAddress The address of the oracle contract to add.
     */
    function addOracle(address newOracleAddress) public {
        require(msg.sender == admin);
        oracles[numOracles] = newOracleAddress;
        numOracles += 1;
    }


    /**
     * @notice Removes an oracle from the registry.
     * @dev Only the administrator can call this function.
     * @param oracleToRemove The address of the oracle contract to remove.
     */

    function removeOracle(address oracleToRemove) public {
        require(msg.sender == admin);
        for (uint256 i = 0; i < numOracles; i++) {
            if (oracles[i] == oracleToRemove) {
                oracles[i] = oracles[numOracles];
                oracles[numOracles] = address(0);
                numOracles--;
                return;
            }
        }
    }


     /**
     * @notice Sets the required threshold of valid oracle values.
     * @dev Only the administrator can call this function.
     * @param newThreshold The new threshold value.
     */

    function setThreshold(uint256 newThreshold) public {
        require(msg.sender == admin);
        threshold = newThreshold;
    }

 /**
     * @notice Sets the timeout period for oracle values.
     * @dev Only the administrator can call this function.
     * @param newTimeoutSeconds The new timeout period in seconds.
     */
    function setTimeoutSeconds(uint256 newTimeoutSeconds) public {
        require(msg.sender == admin);
        timeoutSeconds = newTimeoutSeconds;
    }

    /**
     * @notice Retrieves the median price value for a given asset key from registered oracles.
     * @dev Only returns values that are not older than the timeout period.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return value The median price from available oracles.
     * @return timestamp The current block timestamp.
     */

    function getValue(string memory key) external view returns (uint128, uint128) {
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

            // Sort by timestamp, throw out values older than threshold
            if ((currTimestamp + timeoutSeconds) < block.timestamp) {
                continue;
            }
            values[validValues] = currValue;

            validValues += 1;
        }

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
