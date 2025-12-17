// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import "./IDIAOracleV3.sol";
import "./IPriceMethodology.sol";

/**
 * @title DIAOracleV3Meta
 * @dev Meta oracle that aggregates values from multiple DIAOracleV3 instances.
 *      Uses IPriceMethodology interface for price calculation, allowing different methodologies.
 */
contract DIAOracleV3Meta is Ownable(msg.sender) {
    /// @notice Mapping of registered oracle addresses.
    mapping(uint256 => address) public oracles;

    /// @notice Number of registered oracles.
    uint256 private numOracles;

    /// @notice Minimum number of valid values required to return a result.
    uint256 private threshold;

    /// @notice The timeout period in seconds for oracle values.
    uint256 private timeoutSeconds;

    /// @notice Maximum number of recent historical values to consider per oracle.
    uint256 private windowSize;

    /// @notice The price calculation methodology contract.
    IPriceMethodology public priceMethodology;

    event OracleAdded(address newOracleAddress);
    event OracleRemoved(address removedOracleAddress);
    event PriceMethodologyChanged(address oldMethodology, address newMethodology);

    error OracleNotFound();
    error ZeroAddress();
    error InvalidThreshold(uint256 value);
    error InvalidTimeOut(uint256 value);
    error TimeoutExceedsLimit(uint256 value);
    error OracleExists();
    error ThresholdNotMet(uint256 validValues, uint256 threshold);
    error InvalidHistoryIndex(uint256 index);
    error InvalidMethodology();
    error InvalidWindowSize(uint256 value);

    modifier validateAddress(address _address) {
        if (_address == address(0)) revert ZeroAddress();
        _;
    }

    /**
     * @notice Constructor
     * @param _priceMethodology The address of the price calculation methodology contract
     */
    constructor(address _priceMethodology) {
        if (_priceMethodology == address(0)) {
            revert InvalidMethodology();
        }
        priceMethodology = IPriceMethodology(_priceMethodology);
    }

    /**
     * @notice Adds a new oracle to the registry.
     * @dev Only the administrator can call this function.
     * @param newOracleAddress The address of the oracle contract to add.
     */
    function addOracle(
        address newOracleAddress
    ) public onlyOwner validateAddress(newOracleAddress) {
        for (uint256 i = 0; i < numOracles; i++) {
            if (oracles[i] == newOracleAddress) {
                revert OracleExists();
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
    function removeOracle(address oracleToRemove) public onlyOwner {
        for (uint256 i = 0; i < numOracles; i++) {
            if (oracles[i] == oracleToRemove) {
                oracles[i] = oracles[numOracles - 1];
                oracles[numOracles - 1] = address(0);
                numOracles--;
                emit OracleRemoved(oracleToRemove);
                return;
            }
        }
        revert OracleNotFound();
    }

    /**
     * @notice Sets the required threshold of valid oracle values.
     * @dev Only the administrator can call this function.
     * @param newThreshold The new threshold value.
     */
    function setThreshold(uint256 newThreshold) public onlyOwner {
        if (newThreshold == 0) {
            revert InvalidThreshold(newThreshold);
        }
        threshold = newThreshold;
    }

    /**
     * @notice Sets the timeout period for oracle values.
     * @dev Only the administrator can call this function.
     * @param newTimeoutSeconds The new timeout period in seconds.
     */
    function setTimeoutSeconds(uint256 newTimeoutSeconds) public onlyOwner {
        if (newTimeoutSeconds == 0) {
            revert InvalidTimeOut(newTimeoutSeconds);
        }
        if (newTimeoutSeconds > 86400) {
            revert TimeoutExceedsLimit(newTimeoutSeconds);
        }
        timeoutSeconds = newTimeoutSeconds;
    }

    /**
     * @notice Sets the price calculation methodology contract.
     * @dev Only the administrator can call this function.
     * @param newMethodology The address of the new methodology contract.
     */
    function setPriceMethodology(address newMethodology) public onlyOwner {
        if (newMethodology == address(0)) {
            revert InvalidMethodology();
        }
        address oldMethodology = address(priceMethodology);
        priceMethodology = IPriceMethodology(newMethodology);
        emit PriceMethodologyChanged(oldMethodology, newMethodology);
    }

    /**
     * @notice Sets the window size for historical values.
     * @dev Only the administrator can call this function.
     * @param newWindowSize The new window size value.
     */
    function setWindowSize(uint256 newWindowSize) public onlyOwner {
        if (newWindowSize == 0) {
            revert InvalidWindowSize(newWindowSize);
        }
        windowSize = newWindowSize;
    }

    /**
     * @notice Retrieves the price value for a given asset key from registered oracles.
     * @dev Uses the configured methodology and windowSize to calculate the aggregated value.
     *      Only considers values that are not older than the timeout period.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return value The aggregated price value from available oracles.
     * @return timestamp The current block timestamp.
     */
    function getValue(string memory key) external view returns (uint128, uint128) {
        if (timeoutSeconds == 0) {
            revert InvalidTimeOut(timeoutSeconds);
        }
        if (threshold == 0) {
            revert InvalidThreshold(threshold);
        }
        if (windowSize == 0) {
            revert InvalidWindowSize(windowSize);
        }

         address[] memory oracleAddresses = new address[](numOracles);
        for (uint256 i = 0; i < numOracles; i++) {
            oracleAddresses[i] = oracles[i];
        }

         (uint128 value, uint128 timestamp) = priceMethodology.calculateValue(
            key,
            oracleAddresses,
            timeoutSeconds,
            threshold,
            windowSize
        );

        return (value, timestamp);
    }

    /**
     * @notice Retrieves the price value with custom configuration parameters.
     * @dev Allows overriding the default windowSize, methodology, timeoutSeconds, and threshold for this call.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @param windowSize Maximum number of recent historical values to consider per oracle.
     * @param methodology Address of the methodology contract to use (must implement IPriceMethodology).
     * @param timeoutSeconds Timeout in seconds for value validity.
     * @param threshold Minimum number of valid values required.
     * @return value The aggregated price value from available oracles.
     * @return timestamp The current block timestamp.
     */
    function getValueByConfig(
        string memory key,
        uint256 windowSize,
        address methodology,
        uint256 timeoutSeconds,
        uint256 threshold
    ) external view returns (uint128 value, uint128 timestamp) {
        if (timeoutSeconds == 0) {
            revert InvalidTimeOut(timeoutSeconds);
        }
        if (threshold == 0) {
            revert InvalidThreshold(threshold);
        }
        if (windowSize == 0) {
            revert InvalidWindowSize(windowSize);
        }
        if (methodology == address(0)) {
            revert InvalidMethodology();
        }

        address[] memory oracleAddresses = new address[](numOracles);
        for (uint256 i = 0; i < numOracles; i++) {
            oracleAddresses[i] = oracles[i];
        }

        IPriceMethodology priceMethodology = IPriceMethodology(methodology);
        (value, timestamp) = priceMethodology.calculateValue(
            key,
            oracleAddresses,
            timeoutSeconds,
            threshold,
            windowSize
        );

        return (value, timestamp);
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

    function getWindowSize() external view returns (uint256) {
        return windowSize;
    }
}
