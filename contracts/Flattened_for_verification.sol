// SPDX-License-Identifier: GPL-3.0

pragma solidity ^0.8.20;

/**
 * @dev Provides information about the current execution context, including the
 * sender of the transaction and its data. While these are generally available
 * via msg.sender and msg.data, they should not be accessed in such a direct
 * manner, since when dealing with meta-transactions the account sending and
 * paying for execution may not be the actual sender (as far as an application
 * is concerned).
 *
 * This contract is only required for intermediate, library-like contracts.
 */
abstract contract Context {
    function _msgSender() internal view virtual returns (address) {
        return msg.sender;
    }

    function _msgData() internal view virtual returns (bytes calldata) {
        return msg.data;
    }

    function _contextSuffixLength() internal view virtual returns (uint256) {
        return 0;
    }
}

pragma solidity ^0.8.20;

/**
 * @dev Contract module which provides a basic access control mechanism, where
 * there is an account (an owner) that can be granted exclusive access to
 * specific functions.
 *
 * The initial owner is set to the address provided by the deployer. This can
 * later be changed with {transferOwnership}.
 *
 * This module is used through inheritance. It will make available the modifier
 * `onlyOwner`, which can be applied to your functions to restrict their use to
 * the owner.
 */
abstract contract Ownable is Context {
    address private _owner;

    /**
     * @dev The caller account is not authorized to perform an operation.
     */
    error OwnableUnauthorizedAccount(address account);

    /**
     * @dev The owner is not a valid owner account. (eg. `address(0)`)
     */
    error OwnableInvalidOwner(address owner);

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    /**
     * @dev Initializes the contract setting the address provided by the deployer as the initial owner.
     */
    constructor(address initialOwner) {
        if (initialOwner == address(0)) {
            revert OwnableInvalidOwner(address(0));
        }
        _transferOwnership(initialOwner);
    }

    /**
     * @dev Throws if called by any account other than the owner.
     */
    modifier onlyOwner() {
        _checkOwner();
        _;
    }

    /**
     * @dev Returns the address of the current owner.
     */
    function owner() public view virtual returns (address) {
        return _owner;
    }

    /**
     * @dev Throws if the sender is not the owner.
     */
    function _checkOwner() internal view virtual {
        if (owner() != _msgSender()) {
            revert OwnableUnauthorizedAccount(_msgSender());
        }
    }

    /**
     * @dev Leaves the contract without owner. It will not be possible to call
     * `onlyOwner` functions. Can only be called by the current owner.
     *
     * NOTE: Renouncing ownership will leave the contract without an owner,
     * thereby disabling any functionality that is only available to the owner.
     */
    function renounceOwnership() public virtual onlyOwner {
        _transferOwnership(address(0));
    }

    /**
     * @dev Transfers ownership of the contract to a new account (`newOwner`).
     * Can only be called by the current owner.
     */
    function transferOwnership(address newOwner) public virtual onlyOwner {
        if (newOwner == address(0)) {
            revert OwnableInvalidOwner(address(0));
        }
        _transferOwnership(newOwner);
    }

    /**
     * @dev Transfers ownership of the contract to a new account (`newOwner`).
     * Internal function without access restriction.
     */
    function _transferOwnership(address newOwner) internal virtual {
        address oldOwner = _owner;
        _owner = newOwner;
        emit OwnershipTransferred(oldOwner, newOwner);
    }
}
pragma solidity 0.8.29;


interface IDIAOracleV2 {
    function setValue(
        string memory key,
        uint128 value,
        uint128 timestamp
    ) external;

    function getValue(
        string memory key
    ) external view returns (uint128, uint128);
}
pragma solidity 0.8.29;

/**
 * @title sort given array using Quick sort.
 * @author [Priyda](https://github.com/priyda)
 */
library QuickSort {
    function sort(uint128[] memory data, uint256 lValue, uint256 rValue) public view returns (uint128[] memory) {
        quickSort(data, int256(lValue), int256(rValue));
        return data;
    }

    /** Quicksort is a sorting algorithm based on the divide and conquer approach **/
    function quickSort(
        uint128[] memory _arr,
        int256 left,
        int256 right
    ) internal view {
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
pragma solidity 0.8.29;

/**
 * @title DIAOracleV2Meta
 */
contract DIAOracleV2Meta is Ownable(msg.sender) {
    /// @notice Mapping of registered oracle addresses.
    mapping(uint256 => address) public oracles;

    /// @notice Number of registered oracles.
    uint256 private numOracles;

    /// @notice Minimum number of valid values required to return a result.
    uint256 private threshold;

    /// @notice The timeout period in seconds for oracle values.
    uint256 private timeoutSeconds;

    event OracleAdded(address newOracleAddress);
    event OracleRemoved(address removedOracleAddress);

    error OracleNotFound();
    error ZeroAddress();
    error InvalidThreshold(uint256 value);
    error InvalidTimeOut(uint256 value);
    error TimeoutExceedsLimit(uint256 value);
    error OracleExists();
    error ThresholdNotMet(uint256 validValues, uint256 threshold);


    modifier validateAddress(address _address) {
        if (_address == address(0)) revert ZeroAddress();
        _;
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

        // Timeout should be at most one day
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
        if (timeoutSeconds == 0) {
            revert InvalidTimeOut(timeoutSeconds);
        }
        if (threshold == 0) {
            revert InvalidThreshold(threshold);
        }

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

        if (validValues < threshold) {
            revert ThresholdNotMet(validValues, threshold);
        }

        // Sort by value to retrieve the median
        values = QuickSort.sort(values, 0, validValues - 1);

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

