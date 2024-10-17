// SPDX-License-Identifier: GPL

pragma solidity 0.8.26;

interface IDIAOracleV2 {                                                           
    function setValue(string memory key, uint128 value, uint128 timestamp) external;
    function getValue(string memory key) external view returns (uint128, uint128);
    function updateOracleUpdaterAddress(address newOracleUpdaterAddress) external;
}

contract DIAOracleV2Meta {
    mapping (uint256 => address) oracles;
    uint256 numOracles;
    uint256 threshold;
    uint256 timeoutSeconds;

    address admin;

    constructor() {
        admin = msg.sender;
    }

    function addOracle(address newOracleAddress) public {
        require(msg.sender == admin);
        oracles[numOracles] = newOracleAddress;
        numOracles += 1;
    }

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

    function setThreshold(uint256 newThreshold) public {
        require(msg.sender == admin);
        threshold = newThreshold;
    }

    function setTimeoutSeconds(uint256 newTimeoutSeconds) public {
        require(msg.sender == admin);
        timeoutSeconds = newTimeoutSeconds;
    }

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
}
