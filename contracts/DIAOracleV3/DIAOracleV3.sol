// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;
import {AccessControlUpgradeable} from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import {Initializable} from "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "./IDIAOracleV3.sol";
/**
 * @title DIAOracleV3
 * @notice UUPS upgradeable oracle contract for storing and retrieving asset price data
 * @dev This contract allows authorized updaters to set price values with timestamps,
 *      maintaining both current values and historical data in a ring buffer structure.
 *
 *      Key Features:
 *      - Stores price and timestamp in compressed format (uint256) for backward compatibility
 *      - Maintains historical values using a ring buffer (max 100 entries per key)
 *      - Supports volume data and arbitrary additional data via raw value functions
 *      - Timestamp validation prevents stale or future-dated data
 *      - Monotonic timestamp enforcement ensures data integrity
 *
 *      Access Control:
 *      - DEFAULT_ADMIN_ROLE: Can grant/revoke roles and authorize upgrades
 *      - UPDATER_ROLE: Can update price values
 *
 *      Storage Layout:
 *      - Uses UUPS upgradeable pattern with __gap for future upgrades
 *      - MAX_HISTORY_SIZE is immutable and set at deployment
 *
 */
contract DIAOracleV3 is Initializable, IDIAOracleV3, AccessControlUpgradeable, UUPSUpgradeable {
    bytes32 public constant UPDATER_ROLE = keccak256("UPDATER_ROLE");
    /// @notice Maximum number of historical values to store per key
    uint256 public immutable MAX_HISTORY_SIZE = 100;
    /// @notice Mapping to store compressed values of assets (price and timestamp).
    /// @dev Upper 128 bits store the price and the lower 128 bits store the timestamp.
    ///      This maintains backward compatibility with V2's getValue() function.
    mapping(string => uint256) public values;
    /// @notice Mapping to store historical values for each key (ring buffer).
    /// @dev Pre-allocated arrays of size MAX_HISTORY_SIZE, using ring buffer pattern.
    // slither-disable-next-line uninitialized-state-variables
    mapping(string => ValueEntry[]) private _valueHistory;
    /// @notice Mapping to track the current write index for ring buffer.
    /// @dev Points to the next position to write. When buffer is full, wraps around.
    mapping(string => uint256) private _writeIndex;
    /// @notice Mapping to track the actual count of values stored (for partially filled buffers).
    /// @dev Starts at 0, increases up to MAX_HISTORY_SIZE, then stays at MAX_HISTORY_SIZE.
    mapping(string => uint256) private _valueCount;
    /// @notice Mapping to store raw data for each asset key (volume and any additional data).
    mapping(string => bytes) public rawData;
    /// @notice Global decimal precision for all asset values.
    uint8 public decimals;
    event OracleUpdate(string key, uint128 value, uint128 timestamp);
    event OracleUpdateRaw(string key, uint128 value, uint128 timestamp, uint128 volume, bytes data);
    event UpdaterAddressChange(address newUpdater);
    event DecimalsUpdate(uint8 decimals);
    error MismatchedArrayLengths(uint256 keysLength, uint256 valuesLength);
    error InvalidHistoryIndex(uint256 index, uint256 maxIndex);
    error TimestampTooFarInFuture(uint128 timestamp, uint256 blockTime);
    error TimestampTooFarInPast(uint128 timestamp, uint256 blockTime);
    error TimestampNotIncreasing(uint128 newTimestamp, uint128 existingTimestamp);
    /// @notice Maximum allowed history size to prevent gas issues (set to 1000)
    uint256 public constant MAX_ALLOWED_HISTORY_SIZE = 1000;
    /// @notice Maximum timestamp gap in the future (1 hour)
    uint256 public constant MAX_TIMESTAMP_GAP = 1 hours;
    /// @notice Reserved storage space for future upgrades (100 slots)
    /// @dev Storage slots from forge build --extra-output storageLayout:
    ///
    /// Slot | Label              | Type
    /// ----|--------------------|-------------------------------------------------
    ///   0  | MAX_HISTORY_SIZE     | uint256
    ///   1  | values             | mapping(string => uint256)
    ///   2  | _valueHistory      | mapping(string => array(ValueEntry)_dyn_storage)
    ///   3  | _writeIndex        | mapping(string => uint256)
    ///   4  | _valueCount        | mapping(string => uint256)
    ///   5  | rawData            | mapping(string => bytes)
    ///   6+ | __gap             | 100 slots reserved for future upgrades
    ///
    /// Note: Parent contract storage (before slot 0):
    /// - Initializable: 2 slots (_initialized, _initializing)
    /// - AccessControlUpgradeable: ~4 slots (_roles mapping)
    /// Total contract uses slots 0-5 + parent slots + 100 gap slots
    // slither-disable-next-line unused-state-variables
    uint256[100] private __gap;
    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }
    /**
     * @notice Initializes the contract with roles.
     * @dev Replaces constructor for upgradeable contracts. Uses reinitializer(1)
     *      to allow future upgrades to add new initialization logic with version 2, 3, etc.
     */
    function initialize() public reinitializer(1) {
        decimals = 8;
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(UPDATER_ROLE, msg.sender);
    }
    /**
     * @notice Authorizes upgrade to new implementation.
     * @dev Only callable by addresses with DEFAULT_ADMIN_ROLE.
     * @param newImplementation Address of the new implementation contract.
     */
    // slither-disable-next-line unused-param
    function _authorizeUpgrade(address newImplementation) internal override onlyRole(DEFAULT_ADMIN_ROLE) {
        // Only addresses with DEFAULT_ADMIN_ROLE can authorize upgrades
    }
    /**
     * @notice Updates the price and timestamp for a given asset key.
     * @dev Only callable by addresses with UPDATER_ROLE.
     *      Maintains backward compatibility with V2 by updating the values mapping.
     *      Also adds the value to the historical storage.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @param value The price value to set.
     * @param timestamp The timestamp associated with the value.
     */
    function setValue(string memory key, uint128 value, uint128 timestamp) public onlyRole(UPDATER_ROLE) {
        _validateTimestamp(key, timestamp);
        uint256 cValue = (((uint256)(value)) << 128) + timestamp;
        values[key] = cValue;
        // Add to historical storage (volume = 0 for backward compatibility)
        _addToHistory(key, value, timestamp, 0);
        emit OracleUpdate(key, value, timestamp);
    }
    /**
     * @notice Updates multiple asset values in a single transaction.
     * @dev Each entry in `compressedValues` should be a 256-bit integer where:
     *      - The upper 128 bits represent the price value.
     *      - The lower 128 bits represent the timestamp.
     * @param keys The array of asset identifiers.
     * @param compressedValues The array of compressed values (price and timestamp combined).
     */
    function setMultipleValues(string[] memory keys, uint256[] memory compressedValues) public onlyRole(UPDATER_ROLE) {
        if (keys.length != compressedValues.length) {
            revert MismatchedArrayLengths(keys.length, compressedValues.length);
        }
        for (uint128 i = 0; i < keys.length; i++) {
            string memory currentKey = keys[i];
            uint256 currentCvalue = compressedValues[i];
            uint128 value = (uint128)(currentCvalue >> 128);
            uint128 timestamp = (uint128)(currentCvalue % 2 ** 128);
            _validateTimestamp(currentKey, timestamp);
            // Update the current value (backward compatibility with V2)
            values[currentKey] = currentCvalue;
            // Add to historical storage (volume = 0 for backward compatibility)
            _addToHistory(currentKey, value, timestamp, 0);
            emit OracleUpdate(currentKey, value, timestamp);
        }
    }
    /**
     * @notice Updates the price, timestamp, volume, and arbitrary data for a given asset key using raw calldata.
     * @dev Only callable by addresses with UPDATER_ROLE.
     *      Decodes calldata to extract key, value, timestamp, volume, and arbitrary additional data.
     * @param data The encoded calldata containing (string key, uint128 value, uint128 timestamp,
     *             uint128 volume, bytes additionalData).
     */
    function setRawValue(bytes calldata data) public onlyRole(UPDATER_ROLE) {
        (string memory key, uint128 value, uint128 timestamp, uint128 volume, bytes memory additionalData) = abi.decode(
            data,
            (string, uint128, uint128, uint128, bytes)
        );
        _validateTimestamp(key, timestamp);
        uint256 cValue = (((uint256)(value)) << 128) + timestamp;
        values[key] = cValue;
        rawData[key] = additionalData;
        // Add to historical storage with volume
        _addToHistory(key, value, timestamp, volume);
        emit OracleUpdateRaw(key, value, timestamp, volume, additionalData);
    }
    /**
     * @notice Updates multiple asset values with volume and additional data in a single transaction.
     * @dev Only callable by addresses with UPDATER_ROLE.
     *      Each element in the array should be encoded as (string key, uint128 value, uint128 timestamp,
     *      uint128 volume, bytes additionalData).
     * @param dataArray The array of encoded calldata entries.
     */
    function setMultipleRawValues(bytes[] calldata dataArray) public onlyRole(UPDATER_ROLE) {
        for (uint256 i = 0; i < dataArray.length; i++) {
            (string memory key, uint128 value, uint128 timestamp, uint128 volume, bytes memory additionalData) = abi
                .decode(dataArray[i], (string, uint128, uint128, uint128, bytes));
            _validateTimestamp(key, timestamp);
            uint256 cValue = (((uint256)(value)) << 128) + timestamp;
            values[key] = cValue;
            rawData[key] = additionalData;
            // Add to historical storage with volume
            _addToHistory(key, value, timestamp, volume);
            emit OracleUpdateRaw(key, value, timestamp, volume, additionalData);
        }
    }
    /**
     * @notice Retrieves the raw data for a given asset key.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return The stored raw data (can be decoded by the caller based on expected format).
     */
    function getRawData(string memory key) external view returns (bytes memory) {
        return rawData[key];
    }
    /**
     * @notice Retrieves the latest price and timestamp for a given asset key.
     * @dev Maintains backward compatibility with V2 interface.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return value The stored price value.
     * @return timestamp The stored timestamp.
     */
    function getValue(string memory key) external view returns (uint128, uint128) {
        uint256 cValue = values[key];
        uint128 timestamp = (uint128)(cValue % 2 ** 128);
        uint128 value = (uint128)(cValue >> 128);
        return (value, timestamp);
    }
    /**
     * @notice Retrieves a specific historical value by index using ring buffer.
     * @dev Index 0 is the most recent value, index 1 is the second most recent, etc.
     *      Uses ring buffer logic: most recent is at (writeIndex - 1) % MAX_HISTORY_SIZE.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @param index The index of the historical value (0 = most recent).
     * @return value The price value at the specified index.
     * @return timestamp The timestamp at the specified index.
     * @return volume The volume at the specified index.
     */
    function getValueAt(
        string memory key,
        uint256 index
    ) external view returns (uint128 value, uint128 timestamp, uint128 volume) {
        ValueEntry[] storage history = _valueHistory[key];
        uint256 count = _valueCount[key];
        if (index >= count) {
            revert InvalidHistoryIndex(index, count);
        }
        uint256 currentWriteIndex = _writeIndex[key];
        uint256 position;
        if (index + 1 <= currentWriteIndex) {
            position = currentWriteIndex - 1 - index;
        } else {
            position = (currentWriteIndex + MAX_HISTORY_SIZE - 1 - index) % MAX_HISTORY_SIZE;
        }
        ValueEntry memory entry = history[position];
        return (entry.value, entry.timestamp, entry.volume);
    }
    /**
     * @notice Retrieves all historical values for a given key using ring buffer.
     * @dev Returns values in reverse chronological order (most recent first).
     *      Reads from ring buffer in correct order.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return An array of ValueEntry structs containing all stored historical values.
     */
    function getValueHistory(string memory key) external view returns (ValueEntry[] memory) {
        ValueEntry[] storage history = _valueHistory[key];
        uint256 count = _valueCount[key];
        if (count == 0) {
            return new ValueEntry[](0);
        }
        ValueEntry[] memory result = new ValueEntry[](count);
        uint256 currentWriteIndex = _writeIndex[key];
        for (uint256 i = 0; i < count; i++) {
            uint256 position;
            if (i + 1 <= currentWriteIndex) {
                position = currentWriteIndex - 1 - i;
            } else {
                position = (currentWriteIndex + MAX_HISTORY_SIZE - 1 - i) % MAX_HISTORY_SIZE;
            }
            result[i] = history[position];
        }
        return result;
    }
    /**
     * @notice Returns the number of historical values stored for a given key.
     * @dev Uses valueCount mapping which tracks actual stored values (not array length).
     * @param key The asset identifier (e.g., "BTC/USD").
     * @return The count of historical values stored.
     */
    function getValueCount(string memory key) external view returns (uint256) {
        return _valueCount[key];
    }
    /**
     * @notice Sets the global decimal precision for all asset values.
     * @dev Only callable by addresses with UPDATER_ROLE.
     * @param decimalPrecision The number of decimal places for all asset values.
     */
    function setDecimals(uint8 decimalPrecision) public onlyRole(UPDATER_ROLE) {
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
     * @notice Returns the current maximum history size setting.
     * @return The maximum number of historical values that will be stored per key.
     */
    function getMaxHistorySize() external view returns (uint256) {
        return MAX_HISTORY_SIZE;
    }
    /**
     * @notice ERC-165 interface support check.
     * @dev Returns true if this contract implements the interface defined by
     *      `interfaceId`. See the corresponding ERC-165 section in the ERC
     * @param interfaceId The interface identifier to check for support.
     * @return True if the contract supports the interface, false otherwise.
     */
    function supportsInterface(
        bytes4 interfaceId
    ) public view virtual override(AccessControlUpgradeable, IERC165) returns (bool) {
        return interfaceId == type(IDIAOracleV3).interfaceId || super.supportsInterface(interfaceId);
    }
    /**
     * @notice Internal function to add a value to the historical storage using ring buffer.
     * @dev Uses a ring buffer (circular buffer) for O(1) insertion instead of O(n) array shifting.
     *      Pre-allocates array to MAX_HISTORY_SIZE and uses modulo arithmetic for wrapping.
     * @param key The asset identifier.
     * @param value The price value to store.
     * @param timestamp The timestamp to store.
     * @param volume The volume to store.
     */
    function _addToHistory(string memory key, uint128 value, uint128 timestamp, uint128 volume) private {
        ValueEntry[] storage history = _valueHistory[key];
        uint256 currentWriteIndex = _writeIndex[key];
        uint256 currentCount = _valueCount[key];
        if (history.length == 0) {
            for (uint256 i = 0; i < MAX_HISTORY_SIZE; i++) {
                history.push(ValueEntry(0, 0, 0));
            }
            currentWriteIndex = 0;
            currentCount = 0;
        }
        history[currentWriteIndex] = ValueEntry(value, timestamp, volume);
        currentWriteIndex = (currentWriteIndex + 1) % MAX_HISTORY_SIZE;
        _writeIndex[key] = currentWriteIndex;
        if (currentCount < MAX_HISTORY_SIZE) {
            currentCount++;
            _valueCount[key] = currentCount;
        }
    }
    /**
     * @notice Validates that a timestamp is within acceptable bounds.
     * @dev Timestamp must not be too far in the future or too far in the past.
     *      This prevents invalid data from polluting the oracle.
     *      Also ensures timestamps are monotonically increasing for each key.
     * @param key The asset identifier to check existing timestamp for.
     * @param timestamp The timestamp to validate.
     */
    function _validateTimestamp(string memory key, uint128 timestamp) private view {
        uint256 currentBlockTime = block.timestamp;
        // Check if timestamp is too far in the future
        if (timestamp > uint128(currentBlockTime + MAX_TIMESTAMP_GAP)) {
            revert TimestampTooFarInFuture(timestamp, currentBlockTime);
        }
        // Check if timestamp is too far in the past
        if (currentBlockTime > MAX_TIMESTAMP_GAP && timestamp < uint128(currentBlockTime - MAX_TIMESTAMP_GAP)) {
            revert TimestampTooFarInPast(timestamp, currentBlockTime);
        }
        // Ensure timestamp is not older than existing value for this key
        uint256 existingValue = values[key];
        if (existingValue != 0) {
            uint128 existingTimestamp = uint128(existingValue);
            if (timestamp < existingTimestamp) {
                revert TimestampNotIncreasing(timestamp, existingTimestamp);
            }
        }
    }
}
