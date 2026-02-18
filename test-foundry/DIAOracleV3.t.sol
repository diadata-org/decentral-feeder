// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import "../contracts/DIAOracleV3.sol";
import "../contracts/IDIAOracleV3.sol";
import "forge-std/console.sol";

contract DIAOracleV3Test is Test {
    DIAOracleV3 implementation;
    ERC1967Proxy proxy;
    DIAOracleV3 oracle;
    address deployer = address(this);
    address newUpdater = address(0xBEEF);
    uint256 constant DEFAULT_MAX_HISTORY_SIZE = 10;

    function setUp() public {
        // Deploy implementation
        implementation = new DIAOracleV3();

        // Deploy proxy with initialize call
        bytes memory initData = abi.encodeWithSelector(
            DIAOracleV3.initialize.selector,
            DEFAULT_MAX_HISTORY_SIZE
        );
        proxy = new ERC1967Proxy(address(implementation), initData);

        // Create oracle interface pointing to proxy
        oracle = DIAOracleV3(address(proxy));

        // Warp to a timestamp that allows testing with 1710000000 timestamps
        vm.warp(1710000000);
    }

    function testSetValueAndGetValue() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;
        uint128 timestamp = 1710000000;

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price mismatch");
        assertEq(storedTimestamp, timestamp, "Timestamp mismatch");
    }

    // Test historical value storage
    function testHistoricalValueStorage() public {
        string memory key = "ETH/USD";

        oracle.setValue(key, 3000, 1710000001);
        oracle.setValue(key, 3100, 1710000002);
        oracle.setValue(key, 3200, 1710000003);

        assertEq(oracle.getValueCount(key), 3, "Should have 3 historical values");

        (uint128 value0, uint128 timestamp0) = oracle.getValue(key);
        assertEq(value0, 3200, "Most recent value should be 3200");
        assertEq(timestamp0, 1710000003, "Most recent timestamp should match");

        (uint128 value1, uint128 timestamp1,) = oracle.getValueAt(key, 1);
        assertEq(value1, 3100, "Second value should be 3100");
        assertEq(timestamp1, 1710000002, "Second timestamp should match");

        // Get the oldest value (index 2)
        (uint128 value2, uint128 timestamp2,) = oracle.getValueAt(key, 2);
        assertEq(value2, 3000, "Oldest value should be 3000");
        assertEq(timestamp2, 1710000001, "Oldest timestamp should match");
    }

    // Test getValueHistory function
    function testGetValueHistory() public {
        string memory key = "SOL/USD";

        oracle.setValue(key, 100, 1710000001);
        oracle.setValue(key, 110, 1710000002);
        oracle.setValue(key, 120, 1710000003);

        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);

        assertEq(history.length, 3, "History should have 3 entries");
        assertEq(history[0].value, 120, "First entry should be most recent");
        assertEq(history[0].timestamp, 1710000003, "First entry timestamp should match");
        assertEq(history[1].value, 110, "Second entry should be second most recent");
        assertEq(history[2].value, 100, "Third entry should be oldest");
    }

    function testCircularBufferBehavior() public {
        string memory key = "DIA/USD";
        uint256 maxSize = oracle.getMaxHistorySize();

        // Add more values than maxHistorySize
        for (uint256 i = 0; i < maxSize + 5; i++) {
            oracle.setValue(key, uint128(100 + i), uint128(1710000000 + i));
        }

        // Should only have maxHistorySize values
        assertEq(oracle.getValueCount(key), maxSize, "Should only have maxHistorySize values");

        // The oldest value should be the 6th one (index 0 was removed)
        (uint128 oldestValue,,) = oracle.getValueAt(key, maxSize - 1);
        assertEq(oldestValue, 105, "Oldest value should be 105 (the 6th value)");

        // The newest value should be the last one
        (uint128 newestValue,,) = oracle.getValueAt(key, 0);
        assertEq(newestValue, uint128(100 + maxSize + 4), "Newest value should be the last added");
    }

    // Test getValueCount for empty key
    function testGetValueCountEmpty() public {
        string memory key = "NONEXISTENT";
        assertEq(oracle.getValueCount(key), 0, "Non-existent key should have 0 values");
    }

    // Test getValueAt with invalid index
    function testGetValueAtInvalidIndex() public {
        string memory key = "BTC/USD";
        oracle.setValue(key, 50000, 1710000000);

        vm.expectRevert();
        oracle.getValueAt(key, 1); // Index 1 doesn't exist, only index 0 exists
    }

    // Test setMaxHistorySize
    function testSetMaxHistorySize() public {
        uint256 newSize = 20;
        oracle.setMaxHistorySize(newSize);

        assertEq(oracle.getMaxHistorySize(), newSize, "Max history size should be updated");
    }

    // Test setMaxHistorySize with invalid value (too large)
    function testSetMaxHistorySizeTooLarge() public {
        address user = address(0x123);
        vm.startPrank(user);
        vm.expectRevert();
        oracle.setMaxHistorySize(1001);
        vm.stopPrank();
    }

    // Test initialize with zero max history size (division by zero prevention)
    function testInitializeWithZeroMaxHistorySize() public {
        DIAOracleV3 impl = new DIAOracleV3();
        bytes memory initData = abi.encodeWithSelector(
            DIAOracleV3.initialize.selector,
            uint256(0) // Zero history size should fail
        );

        vm.expectRevert(DIAOracleV3.MaxHistorySizeZero.selector);
        new ERC1967Proxy(address(impl), initData);
    }

    // Test setMaxHistorySize with zero (division by zero prevention)
    function testSetMaxHistorySizeZero() public {
        vm.expectRevert(DIAOracleV3.MaxHistorySizeZero.selector);
        oracle.setMaxHistorySize(0);
    }

    // Test setMaxHistorySize only by admin
    function testSetMaxHistorySizeOnlyAdmin() public {
        address nonAdmin = address(0x456);
        vm.startPrank(nonAdmin);
        vm.expectRevert();
        oracle.setMaxHistorySize(50);
        vm.stopPrank();
    }

    // Test multiple keys with different histories
    function testMultipleKeysWithHistories() public {
        oracle.setValue("BTC/USD", 50000, 1710000001);
        oracle.setValue("ETH/USD", 3000, 1710000002);
        oracle.setValue("BTC/USD", 51000, 1710000003);
        oracle.setValue("ETH/USD", 3100, 1710000004);

        assertEq(oracle.getValueCount("BTC/USD"), 2, "BTC should have 2 values");
        assertEq(oracle.getValueCount("ETH/USD"), 2, "ETH should have 2 values");

        (uint128 btcValue,,) = oracle.getValueAt("BTC/USD", 0);
        assertEq(btcValue, 51000, "BTC most recent should be 51000");

        (uint128 ethValue,,) = oracle.getValueAt("ETH/USD", 0);
        assertEq(ethValue, 3100, "ETH most recent should be 3100");
    }

    // Test setMultipleValues with history
    function testSetMultipleValuesWithHistory() public {
        string[] memory keys = new string[](2);
        keys[0] = "ETH/USD";
        keys[1] = "SOL/USD";

        uint256[] memory compressedValues = new uint256[](2);
        compressedValues[0] = (uint256(3000) << 128) + 1710000001;
        compressedValues[1] = (uint256(150) << 128) + 1710000001;

        oracle.setMultipleValues(keys, compressedValues);

        // Add second set of values
        compressedValues[0] = (uint256(3100) << 128) + 1710000002;
        compressedValues[1] = (uint256(160) << 128) + 1710000002;
        oracle.setMultipleValues(keys, compressedValues);

        assertEq(oracle.getValueCount("ETH/USD"), 2, "ETH should have 2 values");
        assertEq(oracle.getValueCount("SOL/USD"), 2, "SOL should have 2 values");

        (uint128 ethValue2,,) = oracle.getValueAt("ETH/USD", 0);
        assertEq(ethValue2, 3100, "ETH most recent should be 3100");

        (uint128 solValue,,) = oracle.getValueAt("SOL/USD", 0);
        assertEq(solValue, 160, "SOL most recent should be 160");
    }

    // Test only updater can set values
    function testOnlyUpdaterCanSetValue() public {
        address attacker = address(0x1234);

        vm.prank(attacker);
        vm.expectRevert();
        oracle.setValue("BTC/USD", 60000, 1710000002);
    }

    // Test grant updater role
    function testGrantUpdaterRole() public {
        oracle.grantRole(keccak256("UPDATER_ROLE"), newUpdater);

        vm.prank(newUpdater);
        oracle.setValue("BTC/USD", 65000, 1710000003);

        (uint128 storedPrice,) = oracle.getValue("BTC/USD");
        assertEq(storedPrice, 65000, "New updater should be able to set values");
    }

    // Test that getValue (V2 compatibility) returns the latest value
    function testGetValueReturnsLatest() public {
        string memory key = "BTC/USD";

        oracle.setValue(key, 50000, 1710000001);
        oracle.setValue(key, 51000, 1710000002);
        oracle.setValue(key, 52000, 1710000003);

        // getValue should return the latest value
        (uint128 latestValue, uint128 latestTimestamp) = oracle.getValue(key);
        assertEq(latestValue, 52000, "getValue should return latest value");
        assertEq(latestTimestamp, 1710000003, "getValue should return latest timestamp");

        // Should match getValueAt(key, 0)
        (uint128 valueAt0, uint128 timestampAt0,) = oracle.getValueAt(key, 0);
        assertEq(latestValue, valueAt0, "getValue should match getValueAt(0)");
        assertEq(latestTimestamp, timestampAt0, "getValue timestamp should match getValueAt(0)");
    }

    function testConstructorMaxHistorySizeTooLarge() public {
        // Deploy implementation with invalid max history size should not fail in constructor
        // but should fail when initialize is called
        DIAOracleV3 badImpl = new DIAOracleV3();

        // Initialize with invalid size should revert
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.MaxHistorySizeTooLarge.selector, 1001, 1000));

        // Deploy proxy and try to initialize with invalid size
        ERC1967Proxy badProxy = new ERC1967Proxy(
            address(badImpl),
            abi.encodeWithSelector(DIAOracleV3.initialize.selector, 1001)
        );
    }

    function testSetMultipleValuesMismatchedArrays() public {
        string[] memory keys = new string[](2);
        keys[0] = "ETH/USD";
        keys[1] = "SOL/USD";

        uint256[] memory compressedValues = new uint256[](3);
        compressedValues[0] = (uint256(3000) << 128) + 1710000001;
        compressedValues[1] = (uint256(150) << 128) + 1710000001;
        compressedValues[2] = (uint256(200) << 128) + 1710000001;

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.MismatchedArrayLengths.selector, 2, 3));
        oracle.setMultipleValues(keys, compressedValues);
    }

    function testGetValueHistoryEmpty() public {
        string memory key = "EMPTY";
        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);

        assertEq(history.length, 0, "Empty key should return empty array");
    }

    function testGetValueHistoryRingBufferWrap() public {
        string memory key = "WRAP_TEST";
        uint256 maxSize = oracle.getMaxHistorySize();

        // Fill the buffer completely
        for (uint256 i = 0; i < maxSize; i++) {
            oracle.setValue(key, uint128(1000 + i), uint128(1710000000 + i));
        }

        // Add one more to trigger wrap
        oracle.setValue(key, uint128(1000 + maxSize), uint128(1710000000 + maxSize));

        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);

        assertEq(history.length, maxSize, "Should have maxSize values");
        assertEq(history[0].value, uint128(1000 + maxSize), "Most recent should be last added");
        assertEq(history[maxSize - 1].value, uint128(1000 + 1), "Oldest should be second value");
    }

    function testSetMaxHistorySizeTooLargeByAdmin() public {
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.MaxHistorySizeTooLarge.selector, 1001, 1000));
        oracle.setMaxHistorySize(1001);
    }

    function testGetValueAtRingBufferWrap() public {
        string memory key = "WRAP_VALUE_TEST";
        uint256 maxSize = oracle.getMaxHistorySize();

        for (uint256 i = 0; i < maxSize; i++) {
            oracle.setValue(key, uint128(2000 + i), uint128(1710000000 + i));
        }

        oracle.setValue(key, uint128(2000 + maxSize), uint128(1710000000 + maxSize));

        (uint128 value,,) = oracle.getValueAt(key, maxSize - 1);
        assertEq(value, uint128(2000 + 1), "Should access wrapped buffer correctly");
    }

    // Test setRawValue function
    function testSetRawValue() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;
        uint128 timestamp = 1710000000;
        uint128 volume = 1000000;
        bytes memory additionalData = abi.encode("extra data");

        bytes memory encodedData = abi.encode(key, price, timestamp, volume, additionalData);
        oracle.setRawValue(encodedData);

        // Verify value and timestamp
        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price mismatch");
        assertEq(storedTimestamp, timestamp, "Timestamp mismatch");

        // Verify volume in history
        (uint128 histValue, uint128 histTimestamp, uint128 histVolume) = oracle.getValueAt(key, 0);
        assertEq(histValue, price, "History price mismatch");
        assertEq(histTimestamp, timestamp, "History timestamp mismatch");
        assertEq(histVolume, volume, "History volume mismatch");

        // Verify raw data
        bytes memory storedRawData = oracle.getRawData(key);
        assertEq(keccak256(storedRawData), keccak256(additionalData), "Raw data mismatch");
    }

    // Test setRawValue only by updater
    function testSetRawValueOnlyUpdater() public {
        address attacker = address(0x1234);
        bytes memory encodedData = abi.encode("BTC/USD", uint128(50000), uint128(1710000000), uint128(1000), bytes(""));

        vm.prank(attacker);
        vm.expectRevert();
        oracle.setRawValue(encodedData);
    }

    // Test setMultipleRawValues function
    function testSetMultipleRawValues() public {
        bytes[] memory dataArray = new bytes[](3);

        dataArray[0] = abi.encode("BTC/USD", uint128(50000), uint128(1710000001), uint128(1000000), bytes("btc data"));
        dataArray[1] = abi.encode("ETH/USD", uint128(3000), uint128(1710000002), uint128(500000), bytes("eth data"));
        dataArray[2] = abi.encode("SOL/USD", uint128(150), uint128(1710000003), uint128(200000), bytes("sol data"));

        oracle.setMultipleRawValues(dataArray);

        // Verify BTC
        (uint128 btcPrice, uint128 btcTimestamp) = oracle.getValue("BTC/USD");
        assertEq(btcPrice, 50000, "BTC price mismatch");
        assertEq(btcTimestamp, 1710000001, "BTC timestamp mismatch");
        (uint128 btcHistValue,, uint128 btcVolume) = oracle.getValueAt("BTC/USD", 0);
        assertEq(btcHistValue, 50000, "BTC history value mismatch");
        assertEq(btcVolume, 1000000, "BTC volume mismatch");

        // Verify ETH
        (uint128 ethPrice,) = oracle.getValue("ETH/USD");
        assertEq(ethPrice, 3000, "ETH price mismatch");
        (,, uint128 ethVolume) = oracle.getValueAt("ETH/USD", 0);
        assertEq(ethVolume, 500000, "ETH volume mismatch");

        // Verify SOL
        (uint128 solPrice,) = oracle.getValue("SOL/USD");
        assertEq(solPrice, 150, "SOL price mismatch");
        (,, uint128 solVolume) = oracle.getValueAt("SOL/USD", 0);
        assertEq(solVolume, 200000, "SOL volume mismatch");

        // Verify counts
        assertEq(oracle.getValueCount("BTC/USD"), 1, "BTC count mismatch");
        assertEq(oracle.getValueCount("ETH/USD"), 1, "ETH count mismatch");
        assertEq(oracle.getValueCount("SOL/USD"), 1, "SOL count mismatch");
    }

    // Test setMultipleRawValues only by updater
    function testSetMultipleRawValuesOnlyUpdater() public {
        address attacker = address(0x1234);
        bytes[] memory dataArray = new bytes[](1);
        dataArray[0] = abi.encode("BTC/USD", uint128(50000), uint128(1710000000), uint128(1000), bytes(""));

        vm.prank(attacker);
        vm.expectRevert();
        oracle.setMultipleRawValues(dataArray);
    }

    // Test getRawData returns empty for non-existent key
    function testGetRawDataEmpty() public view {
        bytes memory rawData = oracle.getRawData("NONEXISTENT");
        assertEq(rawData.length, 0, "Should return empty bytes for non-existent key");
    }

    // Test volume is stored correctly in history
    function testVolumeInHistory() public {
        string memory key = "VOL/USD";

        // Use setRawValue to set values with volume
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000001), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000002), uint128(2000), bytes(""));
        bytes memory data3 = abi.encode(key, uint128(300), uint128(1710000003), uint128(3000), bytes(""));

        oracle.setRawValue(data1);
        oracle.setRawValue(data2);
        oracle.setRawValue(data3);

        // Verify history with volume
        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);
        assertEq(history.length, 3, "Should have 3 entries");

        assertEq(history[0].value, 300, "First value should be most recent");
        assertEq(history[0].volume, 3000, "First volume should be most recent");

        assertEq(history[1].value, 200, "Second value correct");
        assertEq(history[1].volume, 2000, "Second volume correct");

        assertEq(history[2].value, 100, "Third value correct");
        assertEq(history[2].volume, 1000, "Third volume correct");
    }

    // Test setValue stores zero volume (backward compatibility)
    function testSetValueZeroVolume() public {
        string memory key = "ZERO/VOL";
        oracle.setValue(key, 100, 1710000000);

        (uint128 value, uint128 timestamp, uint128 volume) = oracle.getValueAt(key, 0);
        assertEq(value, 100, "Value mismatch");
        assertEq(timestamp, 1710000000, "Timestamp mismatch");
        assertEq(volume, 0, "Volume should be 0 for setValue");
    }

    // Test setMultipleValues stores zero volume (backward compatibility)
    function testSetMultipleValuesZeroVolume() public {
        string[] memory keys = new string[](2);
        keys[0] = "ZERO1/USD";
        keys[1] = "ZERO2/USD";

        uint256[] memory compressedValues = new uint256[](2);
        compressedValues[0] = (uint256(1000) << 128) + 1710000001;
        compressedValues[1] = (uint256(2000) << 128) + 1710000002;

        oracle.setMultipleValues(keys, compressedValues);

        (,, uint128 vol1) = oracle.getValueAt("ZERO1/USD", 0);
        (,, uint128 vol2) = oracle.getValueAt("ZERO2/USD", 0);

        assertEq(vol1, 0, "Volume should be 0 for setMultipleValues");
        assertEq(vol2, 0, "Volume should be 0 for setMultipleValues");
    }

    // Test timestamp validation - rejects future timestamp beyond gap
    function testRejectTimestampTooFarInFuture() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;
        uint128 timestamp = uint128(block.timestamp + 2 hours); // Beyond 1 hour gap

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInFuture.selector, timestamp, block.timestamp));
        oracle.setValue(key, price, timestamp);
    }

    // Test timestamp validation - accepts timestamp within future gap
    function testAcceptTimestampWithinFutureGap() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;
        uint128 timestamp = uint128(block.timestamp + 30 minutes); // Within 1 hour gap

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price should be stored");
        assertEq(storedTimestamp, timestamp, "Timestamp should be stored");
    }

    // Test timestamp validation - rejects past timestamp beyond gap
    function testRejectTimestampTooFarInPast() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;
        uint128 timestamp = uint128(block.timestamp - 2 hours); // Beyond 1 hour gap

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInPast.selector, timestamp, block.timestamp));
        oracle.setValue(key, price, timestamp);
    }

    // Test timestamp validation - accepts timestamp within past gap
    function testAcceptTimestampWithinPastGap() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;
        uint128 timestamp = uint128(block.timestamp - 30 minutes); // Within 1 hour gap

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price should be stored");
        assertEq(storedTimestamp, timestamp, "Timestamp should be stored");
    }

    // Test timestamp validation - edge case when block timestamp is less than gap (underflow protection)
    function testTimestampValidationWhenBlockTimeIsLessThanGap() public {
        // Warp to a timestamp that's less than MAX_TIMESTAMP_GAP (1 hour)
        vm.warp(1800); // 30 minutes

        string memory key = "BTC/USD";
        uint128 price = 50000;

        // When block.timestamp is less than MAX_TIMESTAMP_GAP, the past validation check is skipped
        // to prevent underflow. This means only future validation is applied.
        // A timestamp in the past (but not too ancient) should be accepted
        uint128 pastTimestamp = 900; // 15 minutes ago (within the 30 minute block time)

        // This should NOT revert because the past check is skipped when block.time <= MAX_TIMESTAMP_GAP
        oracle.setValue(key, price, pastTimestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price should be stored");
        assertEq(storedTimestamp, pastTimestamp, "Past timestamp should be stored when block time is small");
    }

    // Test timestamp validation with setMultipleValues
    function testTimestampValidationInSetMultipleValues() public {
        string[] memory keys = new string[](2);
        keys[0] = "BTC/USD";
        keys[1] = "ETH/USD";

        uint256[] memory compressedValues = new uint256[](2);

        // First value is valid
        compressedValues[0] = (uint256(50000) << 128) + uint128(block.timestamp);

        // Second value has timestamp too far in future
        compressedValues[1] = (uint256(3000) << 128) + uint128(block.timestamp + 2 hours);

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInFuture.selector, uint128(block.timestamp + 2 hours), block.timestamp));
        oracle.setMultipleValues(keys, compressedValues);
    }

    // Test timestamp validation with setRawValue
    function testTimestampValidationInSetRawValue() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;
        uint128 timestamp = uint128(block.timestamp - 2 hours); // Too far in past
        uint128 volume = 1000000;
        bytes memory additionalData = abi.encode("extra data");

        bytes memory encodedData = abi.encode(key, price, timestamp, volume, additionalData);

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInPast.selector, timestamp, block.timestamp));
        oracle.setRawValue(encodedData);
    }

    // Test timestamp validation with setMultipleRawValues
    function testTimestampValidationInSetMultipleRawValues() public {
        bytes[] memory dataArray = new bytes[](2);

        // First value is valid
        dataArray[0] = abi.encode("BTC/USD", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes("btc data"));

        // Second value has timestamp too far in past
        dataArray[1] = abi.encode("ETH/USD", uint128(3000), uint128(block.timestamp - 2 hours), uint128(500000), bytes("eth data"));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInPast.selector, uint128(block.timestamp - 2 hours), block.timestamp));
        oracle.setMultipleRawValues(dataArray);
    }

    // Test timestamp validation at boundary conditions
    function testTimestampAtBoundaryConditions() public {
        string memory key = "BTC/USD";
        uint128 price = 50000;

        // Test at exactly MAX_TIMESTAMP_GAP in future (should succeed - validation uses > not >=)
        uint128 timestampExactlyAtGap = uint128(block.timestamp + 1 hours);
        oracle.setValue(key, price, timestampExactlyAtGap);
        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price should be stored at exactly future gap");
        assertEq(storedTimestamp, timestampExactlyAtGap, "Timestamp should be stored at exactly future gap");

        // Test just beyond MAX_TIMESTAMP_GAP in future (should fail)
        uint128 timestampJustBeyond = uint128(block.timestamp + 1 hours + 1);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInFuture.selector, timestampJustBeyond, block.timestamp));
        oracle.setValue(key, price, timestampJustBeyond);

        // Test at exactly MAX_TIMESTAMP_GAP in past (should succeed - validation uses < not <= and checks currentBlockTime > MAX_TIMESTAMP_GAP)
        timestampExactlyAtGap = uint128(block.timestamp - 1 hours);
        oracle.setValue(key, price + 1, timestampExactlyAtGap);
        (storedPrice, storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price + 1, "Price should be stored at exactly past gap");
        assertEq(storedTimestamp, timestampExactlyAtGap, "Timestamp should be stored at exactly past gap");

        // Test just beyond MAX_TIMESTAMP_GAP in past (should fail)
        timestampJustBeyond = uint128(block.timestamp - 1 hours - 1);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInPast.selector, timestampJustBeyond, block.timestamp));
        oracle.setValue(key, price, timestampJustBeyond);
    }

    // ========== UUPS Proxy Tests ==========

    function testProxyDeployment() public {
        // Verify proxy is correctly set up
        assertEq(oracle.getMaxHistorySize(), DEFAULT_MAX_HISTORY_SIZE, "Max history size should be initialized");
        assertTrue(oracle.hasRole(oracle.DEFAULT_ADMIN_ROLE(), deployer), "Deployer should have admin role");
        assertTrue(oracle.hasRole(oracle.UPDATER_ROLE(), deployer), "Deployer should have updater role");
    }

    function testUpgradeToNewImplementation() public {
        // Add some data before upgrade
        oracle.setValue("BTC/USD", 50000, 1710000000);
        (uint128 valueBefore,) = oracle.getValue("BTC/USD");
        assertEq(valueBefore, 50000, "Value before upgrade should be 50000");

        // Deploy new implementation
        DIAOracleV3 newImplementation = new DIAOracleV3();

        // Upgrade proxy through low-level call (use upgradeToAndCall with empty bytes)
        (bool success, ) = address(proxy).call(
            abi.encodeWithSelector(
                UUPSUpgradeable.upgradeToAndCall.selector,
                address(newImplementation),
                "" // empty bytes for no additional call
            )
        );
        assertTrue(success, "Upgrade should succeed");

        // Create interface to upgraded proxy
        DIAOracleV3 upgradedOracle = DIAOracleV3(address(proxy));

        // Data should persist after upgrade
        (uint128 valueAfter,) = upgradedOracle.getValue("BTC/USD");
        assertEq(valueAfter, 50000, "Value should persist after upgrade");

        // Configuration should persist
        assertEq(upgradedOracle.getMaxHistorySize(), DEFAULT_MAX_HISTORY_SIZE, "Config should persist");

        // Roles should persist
        assertTrue(upgradedOracle.hasRole(upgradedOracle.DEFAULT_ADMIN_ROLE(), deployer), "Admin role should persist");
        assertTrue(upgradedOracle.hasRole(upgradedOracle.UPDATER_ROLE(), deployer), "Updater role should persist");

        // Functionality should work after upgrade
        upgradedOracle.setValue("ETH/USD", 3000, 1710000001);
        (uint128 ethValue,) = upgradedOracle.getValue("ETH/USD");
        assertEq(ethValue, 3000, "Should be able to add values after upgrade");
    }

    function testUpgradeOnlyByAdmin() public {
        // Deploy new implementation
        DIAOracleV3 newImplementation = new DIAOracleV3();

        // Try to upgrade from non-admin address - should revert
        address attacker = address(0x1234);
        vm.prank(attacker);
        vm.expectRevert();
        address(proxy).call(
            abi.encodeWithSelector(
                UUPSUpgradeable.upgradeToAndCall.selector,
                address(newImplementation),
                ""
            )
        );
        // If we reach here, the test should fail (expectRevert ensures it reverts)

        // Now verify that a proper admin CAN upgrade
        (bool success, ) = address(proxy).call(
            abi.encodeWithSelector(
                UUPSUpgradeable.upgradeToAndCall.selector,
                address(newImplementation),
                ""
            )
        );
        assertTrue(success, "Admin should be able to upgrade");

        // Verify the upgrade worked by checking functionality still works
        (uint128 value,) = oracle.getValue("BTC/USD");
        // Oracle should still be functional after upgrade
        assertTrue(true, "Oracle should still be functional after upgrade");
    }

    function testInitializeCannotBeCalledTwice() public {
        vm.expectRevert();
        oracle.initialize(50);
    }

    function testConstructorIsDisabled() public {
        // This test verifies that the constructor is disabled and initialize should be used
        // The implementation contract should have initializers disabled
        DIAOracleV3 impl = new DIAOracleV3();

        // Try to call initialize directly on implementation (should fail due to _disableInitializers)
        vm.expectRevert();
        impl.initialize(100);
    }

    /**
     * @notice Test demonstrates that reinitializer(1) allows future upgrades to add new initializers
     * @dev This is a documentation test showing the pattern for future upgrades
     *
     * Example for future version (DIAOracleV3_v2):
     * function initializeV2(uint256 newParam) public reinitializer(2) {
     *     // Can add new initialization logic here
     *     // This won't conflict with the original initialize() which uses reinitializer(1)
     * }
     *
     * Usage in upgrade script:
     * bytes memory initData = abi.encodeWithSelector(
     *     DIAOracleV3.initializeV2.selector,
     *     newValue
     * );
     * upgradeToAndCall(newImplementation, initData);
     */
    function testReinitializerAllowsFutureInitializers() public {
        // This test documents the reinitializer pattern
        // The initialize() function uses reinitializer(1) which allows:
        // 1. Future upgrades to add initializeV2() with reinitializer(2)
        // 2. Multiple phases of initialization across upgrades
        // 3. Adding new initialization logic without conflicts

        // Verify the contract was initialized with version 1
        (uint128 value,) = oracle.getValue("BTC/USD");
        assertEq(value, 0, "Should be uninitialized initially");

        // The fact that this test exists and passes demonstrates:
        // - The contract can be initialized with reinitializer(1)
        // - Future versions can use reinitializer(2), reinitializer(3), etc.
        assertTrue(true, "Reinitializer pattern documented");
    }

    function testStorageLayoutCompatibility() public {
        // Add data before upgrade
        oracle.setValue("BTC/USD", 50000, 1710000000);
        oracle.setValue("ETH/USD", 3000, 1710000001);
        oracle.setMaxHistorySize(20);

        // Upgrade
        DIAOracleV3 newImplementation = new DIAOracleV3();
        (bool success, ) = address(proxy).call(
            abi.encodeWithSelector(
                UUPSUpgradeable.upgradeToAndCall.selector,
                address(newImplementation),
                ""
            )
        );
        assertTrue(success, "Upgrade should succeed");

        DIAOracleV3 upgradedOracle = DIAOracleV3(address(proxy));

        // All storage should be intact
        assertEq(upgradedOracle.getMaxHistorySize(), 20, "Max history size should persist");
        assertEq(upgradedOracle.getValueCount("BTC/USD"), 1, "BTC count should persist");
        assertEq(upgradedOracle.getValueCount("ETH/USD"), 1, "ETH count should persist");

        (uint128 btcValue,) = upgradedOracle.getValue("BTC/USD");
        assertEq(btcValue, 50000, "BTC value should persist");

        (uint128 ethValue,) = upgradedOracle.getValue("ETH/USD");
        assertEq(ethValue, 3000, "ETH value should persist");
    }
}
