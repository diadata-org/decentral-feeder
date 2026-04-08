// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {IERC165} from "@openzeppelin/contracts/utils/introspection/IERC165.sol";
import {SafeCast} from "@openzeppelin/contracts/utils/math/SafeCast.sol";
import "./IDIAOracleV3.sol";
import "./IPriceMethodology.sol";

/**
 * @title DIAOracleV3Meta
 * @notice Meta oracle contract that aggregates and validates prices from multiple DIAOracleV3 instances
 * @dev This contract queries multiple oracle sources and aggregates their values using
 *      configurable calculation methodologies (median, VWAP, etc.).
 *
 *      Key Features:
 *      - Aggregates data from multiple DIAOracleV3 oracles
 *      - Pluggable price methodology system (median, VWAP, custom)
 *      - Configurable validation parameters (timeout, threshold, window size)
 *      - Filters oracles by decimal precision to ensure consistency
 *      - Supports volume aggregation
 *      - Allows custom methodology calls for one-off calculations
 *
 *      Access Control:
 *      - Only owner can add/remove oracles
 *      - Only owner can change methodology and validation parameters
 *
 *      Security Considerations:
 *      - All oracles must implement IDIAOracleV3 interface
 *      - Only oracles with matching decimals are included in aggregation
 *      - Timeout mechanism prevents stale data usage
 *      - Threshold ensures minimum number of valid responses
 *
 */
contract DIAOracleV3Meta is Ownable(msg.sender) {
    /// @notice Mapping of registered oracle addresses.
    mapping(uint256 => address) public oracles;

    /// @notice Number of registered oracles.
    uint256 private _numOracles;

    /// @notice Minimum number of valid values required to return a result.
    uint256 private _threshold;

    /// @notice The timeout period in seconds for oracle values.
    uint256 private _timeoutSeconds;

    /// @notice Maximum number of recent historical values to consider per oracle.
    uint256 private _windowSize;

    /// @notice The price calculation methodology contract.
    IPriceMethodology public priceMethodology;

    /// @notice Global decimal precision for all asset values.
    uint8 public decimals;

    event OracleAdded(address newOracleAddress);
    event OracleRemoved(address removedOracleAddress);
    event PriceMethodologyChanged(address oldMethodology, address newMethodology);
    event ThresholdChanged(uint256 oldThreshold, uint256 newThreshold);
    event TimeoutSecondsChanged(uint256 oldTimeoutSeconds, uint256 newTimeoutSeconds);
    event WindowSizeChanged(uint256 oldWindowSize, uint256 newWindowSize);
    event DecimalsUpdate(uint8 decimals);

    error OracleNotFound();
    error ZeroAddress();
    error InvalidThreshold(uint256 value);
    error InvalidTimeOut(uint256 value);
    error TimeoutExceedsLimit(uint256 value);
    error OracleExists();
    error InvalidHistoryIndex(uint256 index);
    error InvalidOracleIndex(uint256 index);
    error InvalidMethodology();
    error InvalidWindowSize(uint256 value);
    error InvalidOracle(address oracleAddress);

    modifier validateAddress(address _address) {
        if (_address == address(0)) revert ZeroAddress();
        _;
    }

    /**
     * @dev Modifier to check if an address implements the IDIAOracleV3 interface.
     * @param _oracleAddress The address to validate.
     */
    modifier validateOracleInterface(address _oracleAddress) {
        if (!IERC165(_oracleAddress).supportsInterface(type(IDIAOracleV3).interfaceId)) {
            revert InvalidOracle(_oracleAddress);
        }
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
        decimals = 18; // Default to 18 decimal places (standard for many tokens)
        priceMethodology = IPriceMethodology(_priceMethodology);
    }

    /**
     * @notice Adds a new oracle to the registry.
     * @dev Only the administrator can call this function.
     *      Validates that the address implements the IDIAOracleV3 interface.
     * @param newOracleAddress The address of the oracle contract to add.
     */
    function addOracle(
        address newOracleAddress
    ) public onlyOwner validateAddress(newOracleAddress) validateOracleInterface(newOracleAddress) {
        for (uint256 i = 0; i < _numOracles; i++) {
            if (oracles[i] == newOracleAddress) {
                revert OracleExists();
            }
        }
        oracles[_numOracles] = newOracleAddress;
        _numOracles += 1;
        emit OracleAdded(newOracleAddress);
    }

    /**
     * @notice Removes an oracle from the registry.
     * @dev Only the administrator can call this function.
     * @param oracleToRemove The address of the oracle contract to remove.
     */
    function removeOracle(address oracleToRemove) public onlyOwner {
        uint256 oracleCount = _numOracles;
        bool found = false;
        uint256 indexToRemove = 0;

        for (uint256 i = 0; i < oracleCount; i++) {
            if (oracles[i] == oracleToRemove) {
                indexToRemove = i;
                found = true;
                break;
            }
        }

        if (!found) {
            revert OracleNotFound();
        }

        uint256 newNumOracles = oracleCount - 1;

        if (_threshold > newNumOracles) {
            revert InvalidThreshold(_threshold);
        }

        oracles[indexToRemove] = oracles[oracleCount - 1];
        oracles[oracleCount - 1] = address(0);
        _numOracles = newNumOracles;
        emit OracleRemoved(oracleToRemove);
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

        // Validate that threshold doesn't exceed number of oracles
        if (newThreshold > _numOracles) {
            revert InvalidThreshold(newThreshold);
        }

        uint256 oldThreshold = _threshold;
        _threshold = newThreshold;
        emit ThresholdChanged(oldThreshold, newThreshold);
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
        uint256 oldTimeoutSeconds = _timeoutSeconds;
        _timeoutSeconds = newTimeoutSeconds;
        emit TimeoutSecondsChanged(oldTimeoutSeconds, newTimeoutSeconds);
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
        uint256 oldWindowSize = _windowSize;
        _windowSize = newWindowSize;
        emit WindowSizeChanged(oldWindowSize, newWindowSize);
    }

    /**
     * @notice Sets the global decimal precision for all asset values.
     * @dev Only the administrator can call this function.
     * @param decimalPrecision The number of decimal places for all asset values.
     */
    function setDecimals(uint8 decimalPrecision) public onlyOwner {
        decimals = decimalPrecision;
        emit DecimalsUpdate(decimalPrecision);
    }

    /**
     * @notice Retrieves the global decimal precision for all asset values.
     * @return The number of decimal places for asset values.
     */
    function getDecimals() external view returns (uint8) {
        return decimals;
    }

    /**
     * @notice Retrieves the price value for a given asset key from registered oracles.
     * @dev Uses the configured methodology and windowSize to calculate the aggregated value.
     *      Only considers values that are not older than the timeout period and from oracles
     *      with matching decimal precision.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return value The aggregated price value from available oracles.
     * @return timestamp The current block timestamp.
     */
    function getValue(string memory key) external view returns (uint128, uint128) {
        if (_timeoutSeconds == 0) {
            revert InvalidTimeOut(_timeoutSeconds);
        }
        if (_threshold == 0) {
            revert InvalidThreshold(_threshold);
        }
        if (_windowSize == 0) {
            revert InvalidWindowSize(_windowSize);
        }

         address[] memory oracleAddresses = new address[](_numOracles);
        uint256 matchingCount = 0;
        for (uint256 i = 0; i < _numOracles; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracles[i]);
            if (oracle.getDecimals() == decimals) {
                oracleAddresses[matchingCount] = oracles[i];
                matchingCount++;
            }
        }

         if (matchingCount < _numOracles) {
            address[] memory trimmedAddresses = new address[](matchingCount);
            for (uint256 i = 0; i < matchingCount; i++) {
                trimmedAddresses[i] = oracleAddresses[i];
            }
            oracleAddresses = trimmedAddresses;
        }

        (uint128 value, uint128 timestamp) = priceMethodology.calculateValue(
            key,
            oracleAddresses,
            _timeoutSeconds,
            _threshold,
            _windowSize
        );

        return (value, timestamp);
    }

    /**
     * @notice Retrieves the price value with custom configuration parameters.
     * @dev Allows overriding the default windowSize, methodology, timeoutSeconds, and threshold for this call.
     *      Only considers oracles with matching decimal precision.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @param customWindowSize Maximum number of recent historical values to consider per oracle.
     * @param customMethodology Address of the methodology contract to use (must implement IPriceMethodology).
     * @param customTimeoutSeconds Timeout in seconds for value validity.
     * @param customThreshold Minimum number of valid values required.
     * @return value The aggregated price value from available oracles.
     * @return timestamp The current block timestamp.
     */
    function getValueByConfig(
        string memory key,
        uint256 customWindowSize,
        address customMethodology,
        uint256 customTimeoutSeconds,
        uint256 customThreshold
    ) external view returns (uint128 value, uint128 timestamp) {
        if (customTimeoutSeconds == 0) {
            revert InvalidTimeOut(customTimeoutSeconds);
        }
        if (customThreshold == 0) {
            revert InvalidThreshold(customThreshold);
        }
        if (customWindowSize == 0) {
            revert InvalidWindowSize(customWindowSize);
        }
        if (customMethodology == address(0)) {
            revert InvalidMethodology();
        }

         address[] memory oracleAddresses = new address[](_numOracles);
        uint256 matchingCount = 0;
        for (uint256 i = 0; i < _numOracles; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracles[i]);
            if (oracle.getDecimals() == decimals) {
                oracleAddresses[matchingCount] = oracles[i];
                matchingCount++;
            }
        }

         if (matchingCount < _numOracles) {
            address[] memory trimmedAddresses = new address[](matchingCount);
            for (uint256 i = 0; i < matchingCount; i++) {
                trimmedAddresses[i] = oracleAddresses[i];
            }
            oracleAddresses = trimmedAddresses;
        }

        IPriceMethodology methodologyContract = IPriceMethodology(customMethodology);
        (value, timestamp) = methodologyContract.calculateValue(
            key,
            oracleAddresses,
            customTimeoutSeconds,
            customThreshold,
            customWindowSize
        );

        return (value, timestamp);
    }

    /**
     * @notice Retrieves the aggregated volume for a given asset key from all registered oracles.
     * @dev Sums up the most recent volume from each oracle that has valid (non-expired) data
     *      and matching decimal precision.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return totalVolume The sum of volumes from all valid oracles.
     * @return validOracleCount The number of oracles with valid volume data.
     */
    function getAggregatedVolume(
        string memory key
    ) external view returns (uint128 totalVolume, uint256 validOracleCount) {
        if (_timeoutSeconds == 0) {
            revert InvalidTimeOut(_timeoutSeconds);
        }

        uint256 sum = 0;
        uint256 count = 0;

        for (uint256 i = 0; i < _numOracles; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracles[i]);

            // Only consider oracles with matching decimals
            if (oracle.getDecimals() != decimals) {
                continue;
            }

            uint256 valueCount = oracle.getValueCount(key);

            if (valueCount == 0) {
                continue;
            }

            (uint128 _value, uint128 timestamp, uint128 volume) = oracle.getValueAt(key, 0);
            _value;

            if (volume == 0) {
                continue;
            }

            // Check if value is not expired
            if ((timestamp + _timeoutSeconds) >= block.timestamp) {
                sum += volume;
                count++;
            }
        }

        return (SafeCast.toUint128(sum), count);
    }

    /**
     * @notice Retrieves the raw data for a given asset key from a specific oracle.
     * @param oracleIndex The index of the oracle in the registry.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return data The raw data stored for this key in the specified oracle.
     */
    function getRawDataFromOracle(uint256 oracleIndex, string memory key) external view returns (bytes memory) {
        if (oracleIndex >= _numOracles) {
            revert InvalidOracleIndex(oracleIndex);
        }

        IDIAOracleV3 oracle = IDIAOracleV3(oracles[oracleIndex]);
        return oracle.getRawData(key);
    }

    /**
     * @notice Retrieves raw data for a given asset key from all registered oracles.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return dataArray Array of raw data from each oracle (in order of registration).
     */
    function getAllRawData(string memory key) external view returns (bytes[] memory) {
        bytes[] memory dataArray = new bytes[](_numOracles);

        for (uint256 i = 0; i < _numOracles; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracles[i]);
            dataArray[i] = oracle.getRawData(key);
        }

        return dataArray;
    }

    /**
     * @notice Retrieves value, timestamp, and volume for a given asset key from a specific oracle.
     * @param oracleIndex The index of the oracle in the registry.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @param historyIndex The index in the oracle's history (0 = most recent).
     * @return value The price value.
     * @return timestamp The timestamp of the value.
     * @return volume The volume associated with the value.
     */
    function getValueWithVolumeFromOracle(
        uint256 oracleIndex,
        string memory key,
        uint256 historyIndex
    ) external view returns (uint128 value, uint128 timestamp, uint128 volume) {
        if (oracleIndex >= _numOracles) {
            revert InvalidOracleIndex(oracleIndex);
        }

        IDIAOracleV3 oracle = IDIAOracleV3(oracles[oracleIndex]);
        return oracle.getValueAt(key, historyIndex);
    }

    /**
     * @notice Retrieves the most recent value with volume from all oracles.
     * @dev Returns arrays of values, timestamps, and volumes from each oracle.
     *      Only includes oracles that have data for the given key.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return values Array of price values from each oracle.
     * @return timestamps Array of timestamps from each oracle.
     * @return volumes Array of volumes from each oracle.
     * @return oracleAddresses Array of oracle addresses that provided data.
     */
    function getAllValuesWithVolume(
        string memory key
    )
        external
        view
        returns (
            uint128[] memory values,
            uint128[] memory timestamps,
            uint128[] memory volumes,
            address[] memory oracleAddresses
        )
    {
        // First pass: count oracles with data
        uint256 count = 0;
        for (uint256 i = 0; i < _numOracles; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracles[i]);
            if (oracle.getValueCount(key) > 0) {
                count++;
            }
        }

        // Allocate arrays
        values = new uint128[](count);
        timestamps = new uint128[](count);
        volumes = new uint128[](count);
        oracleAddresses = new address[](count);

        // Second pass: populate arrays
        uint256 idx = 0;
        for (uint256 i = 0; i < _numOracles; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracles[i]);
            if (oracle.getValueCount(key) > 0) {
                (values[idx], timestamps[idx], volumes[idx]) = oracle.getValueAt(key, 0);
                oracleAddresses[idx] = oracles[i];
                idx++;
            }
        }

        return (values, timestamps, volumes, oracleAddresses);
    }

    /**
     * @notice Retrieves aggregated value with total volume.
     * @dev Uses the configured methodology for price calculation and sums volumes from valid oracles.
     *      Only considers oracles with matching decimal precision.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return value The aggregated price value.
     * @return timestamp The timestamp from methodology.
     * @return totalVolume The sum of volumes from all valid oracles.
     */
    function getValueWithVolume(
        string memory key
    ) external view returns (uint128 value, uint128 timestamp, uint128 totalVolume) {
        if (_timeoutSeconds == 0) {
            revert InvalidTimeOut(_timeoutSeconds);
        }
        if (_threshold == 0) {
            revert InvalidThreshold(_threshold);
        }
        if (_windowSize == 0) {
            revert InvalidWindowSize(_windowSize);
        }

         address[] memory oracleAddresses = new address[](_numOracles);
        uint256 matchingCount = 0;
        for (uint256 i = 0; i < _numOracles; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracles[i]);
            if (oracle.getDecimals() == decimals) {
                oracleAddresses[matchingCount] = oracles[i];
                matchingCount++;
            }
        }

         if (matchingCount < _numOracles) {
            address[] memory trimmedAddresses = new address[](matchingCount);
            for (uint256 i = 0; i < matchingCount; i++) {
                trimmedAddresses[i] = oracleAddresses[i];
            }
            oracleAddresses = trimmedAddresses;
        }

        // Get aggregated value using methodology
        (value, timestamp) = priceMethodology.calculateValue(
            key,
            oracleAddresses,
            _timeoutSeconds,
            _threshold,
            _windowSize
        );

        // Calculate total volume from valid oracles with matching decimals
        uint256 volumeSum = 0;
        for (uint256 i = 0; i < matchingCount; i++) {
            IDIAOracleV3 oracle = IDIAOracleV3(oracleAddresses[i]);
            uint256 valueCount = oracle.getValueCount(key);

            if (valueCount == 0) {
                continue;
            }

            (uint128 _oracleValue, uint128 oracleTimestamp, uint128 oracleVolume) = oracle.getValueAt(key, 0);
            _oracleValue;

            // Check if value is not expired
            if ((oracleTimestamp + _timeoutSeconds) >= block.timestamp) {
                volumeSum += oracleVolume;
            }
        }

        totalVolume = SafeCast.toUint128(volumeSum);
        return (value, timestamp, totalVolume);
    }

    /**
     * @notice Gets the number of registered oracles
     * @return count The total number of oracles in the registry
     */
    function getNumOracles() external view returns (uint256) {
        return _numOracles;
    }

    /**
     * @notice Gets the minimum threshold of valid oracle values required
     * @return threshold The minimum number of valid values needed to return a result
     */
    function getThreshold() external view returns (uint256) {
        return _threshold;
    }

    /**
     * @notice Gets the timeout period for oracle value validity
     * @return timeoutSeconds The number of seconds after which a value is considered stale
     */
    function getTimeoutSeconds() external view returns (uint256) {
        return _timeoutSeconds;
    }

    /**
     * @notice Gets the window size for historical value consideration
     * @return windowSize The maximum number of recent historical values to consider per oracle
     */
    function getWindowSize() external view returns (uint256) {
        return _windowSize;
    }
}
