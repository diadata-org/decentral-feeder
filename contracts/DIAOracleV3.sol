// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import "./IDIAOracleV3.sol";

/**
 * @title DIAOracleV3
 * @dev A simple oracle contract that allows an authorized updater to set and retrieve price values with timestamps.
 */
contract DIAOracleV3 is IDIAOracleV3, AccessControl {
    bytes32 public constant UPDATER_ROLE = keccak256("UPDATER_ROLE");
    
    /// @notice Maximum number of historical values to store per key (default: 100)
    uint256 public maxHistorySize;
    
    /// @notice Mapping to store compressed values of assets (price and timestamp).
    /// @dev Upper 128 bits store the price and the lower 128 bits store the timestamp.
    ///      This maintains backward compatibility with V2's getValue() function.
    mapping (string => uint256) public values;
    
    /// @notice Mapping to store historical values for each key (ring buffer).
    /// @dev Pre-allocated arrays of size maxHistorySize, using ring buffer pattern.
    mapping (string => ValueEntry[]) private _valueHistory;
    
    /// @notice Mapping to track the current write index for ring buffer.
    /// @dev Points to the next position to write. When buffer is full, wraps around.
    mapping (string => uint256) private _writeIndex;
    
    /// @notice Mapping to track the actual count of values stored (for partially filled buffers).
    /// @dev Starts at 0, increases up to maxHistorySize, then stays at maxHistorySize.
    mapping (string => uint256) private _valueCount;
    
    event OracleUpdate(string key, uint128 value, uint128 timestamp);
    event UpdaterAddressChange(address newUpdater);
    event MaxHistorySizeChanged(uint256 oldSize, uint256 newSize);
    
    error MismatchedArrayLengths(uint256 keysLength, uint256 valuesLength);
    error InvalidHistoryIndex(uint256 index, uint256 maxIndex);
    error MaxHistorySizeTooLarge(uint256 requestedSize, uint256 maxAllowed);
    
    /// @notice Maximum allowed history size to prevent gas issues (set to 1000)
    uint256 public constant MAX_ALLOWED_HISTORY_SIZE = 1000;
    
    constructor(uint256 _maxHistorySize) {
        if (_maxHistorySize > MAX_ALLOWED_HISTORY_SIZE) {
            revert MaxHistorySizeTooLarge(_maxHistorySize, MAX_ALLOWED_HISTORY_SIZE);
        }
        maxHistorySize = _maxHistorySize;
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(UPDATER_ROLE, msg.sender);
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
       uint256 cValue = (((uint256)(value)) << 128) + timestamp;
        values[key] = cValue;
        
        // Add to historical storage
        _addToHistory(key, value, timestamp);
        
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
            uint128 timestamp = (uint128)(currentCvalue % 2**128);
            
            // Update the current value (backward compatibility with V2)
            values[currentKey] = currentCvalue;
            
            // Add to historical storage
            _addToHistory(currentKey, value, timestamp);
            
            emit OracleUpdate(currentKey, value, timestamp);
        }
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
        uint128 timestamp = (uint128)(cValue % 2**128);
        uint128 value = (uint128)(cValue >> 128);
        return (value, timestamp);
    }
    
    /**
     * @notice Retrieves a specific historical value by index using ring buffer.
     * @dev Index 0 is the most recent value, index 1 is the second most recent, etc.
     *      Uses ring buffer logic: most recent is at (writeIndex - 1) % maxHistorySize.
     * @param key The asset identifier (e.g., "BTC/USD").
     * @param index The index of the historical value (0 = most recent).
     * @return value The price value at the specified index.
     * @return timestamp The timestamp at the specified index.
     */
    function getValueAt(string memory key, uint256 index) external view returns (uint128 value, uint128 timestamp) {
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
             position = (currentWriteIndex + maxHistorySize - 1 - index) % maxHistorySize;
        }
        
        ValueEntry memory entry = history[position];
        return (entry.value, entry.timestamp);
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
                 position = (currentWriteIndex + maxHistorySize - 1 - i) % maxHistorySize;
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
     * @notice Sets the maximum number of historical values to store per key.
     * @dev Only callable by addresses with DEFAULT_ADMIN_ROLE.
     *      This setting applies to all future updates (existing history is not affected).
     * @param newMaxSize The new maximum history size (must be <= MAX_ALLOWED_HISTORY_SIZE).
     */
    function setMaxHistorySize(uint256 newMaxSize) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (newMaxSize > MAX_ALLOWED_HISTORY_SIZE) {
            revert MaxHistorySizeTooLarge(newMaxSize, MAX_ALLOWED_HISTORY_SIZE);
        }
        
        uint256 oldSize = maxHistorySize;
        maxHistorySize = newMaxSize;
        emit MaxHistorySizeChanged(oldSize, newMaxSize);
    }
    
    /**
     * @notice Returns the current maximum history size setting.
     * @return The maximum number of historical values that will be stored per key.
     */
    function getMaxHistorySize() external view returns (uint256) {
        return maxHistorySize;
    }
    
    /**
     * @notice Internal function to add a value to the historical storage using ring buffer.
     * @dev Uses a ring buffer (circular buffer) for O(1) insertion instead of O(n) array shifting.
     *      Pre-allocates array to maxHistorySize and uses modulo arithmetic for wrapping.
     * @param key The asset identifier.
     * @param value The price value to store.
     * @param timestamp The timestamp to store.
     */
    function _addToHistory(string memory key, uint128 value, uint128 timestamp) private {
        ValueEntry[] storage history = _valueHistory[key];
        uint256 currentWriteIndex = _writeIndex[key];
        uint256 currentCount = _valueCount[key];
        
         if (history.length == 0) {
             for (uint256 i = 0; i < maxHistorySize; i++) {
                history.push(ValueEntry(0, 0));
            }
            currentWriteIndex = 0;
            currentCount = 0;
        }
        
         history[currentWriteIndex] = ValueEntry(value, timestamp);
        
        currentWriteIndex = (currentWriteIndex + 1) % maxHistorySize;
        _writeIndex[key] = currentWriteIndex;
        
         if (currentCount < maxHistorySize) {
            currentCount++;
            _valueCount[key] = currentCount;
        }
     }
}
