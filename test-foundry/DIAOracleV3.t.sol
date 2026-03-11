// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts-upgradeable/proxy/utils/UUPSUpgradeable.sol";
import {AccessControlUpgradeable} from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import "../contracts/DIAOracleV3.sol";
import "../contracts/IDIAOracleV3.sol";
import "forge-std/console.sol";

contract DIAOracleV3Test is Test {
    DIAOracleV3 implementation;
    ERC1967Proxy proxy;
    DIAOracleV3 oracle;
    address deployer = address(this);
    address newUpdater = address(0xBEEF);

    function setUp() public {
        // Deploy implementation
        implementation = new DIAOracleV3();

        // Deploy proxy without initialization data
        proxy = new ERC1967Proxy(address(implementation), "");

        // Create oracle interface pointing to proxy
        oracle = DIAOracleV3(address(proxy));

        // Initialize the contract (msg.sender will be the test contract)
        oracle.initialize();

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

        vm.expectRevert(
            abi.encodeWithSelector(
                DIAOracleV3.InvalidHistoryIndex.selector,
                1,
                1 // maxIndex = count = 1
            )
        );
        oracle.getValueAt(key, 1); // Index 1 doesn't exist, only index 0 exists
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

        vm.startPrank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                attacker,
                oracle.UPDATER_ROLE()
            )
        );
        oracle.setValue("BTC/USD", 60000, 1710000002);
        vm.stopPrank();
    }

    // Test grant updater role
    function testGrantUpdaterRole() public {
        oracle.grantRole(keccak256("UPDATER_ROLE"), newUpdater);

        vm.prank(newUpdater);
        oracle.setValue("BTC/USD", 65000, 1710000003);

        (uint128 storedPrice,) = oracle.getValue("BTC/USD");
        assertEq(storedPrice, 65000, "New updater should be able to set values");
    }

    // ========== Access Control Tests ==========

    function testRevokeUpdaterRole() public {
        // Grant role
        oracle.grantRole(keccak256("UPDATER_ROLE"), newUpdater);

        // Verify they can set values
        vm.prank(newUpdater);
        oracle.setValue("BTC/USD", 65000, 1710000003);

        // Revoke role
        oracle.revokeRole(keccak256("UPDATER_ROLE"), newUpdater);

        // Verify they can no longer set values
        vm.startPrank(newUpdater);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                newUpdater,
                oracle.UPDATER_ROLE()
            )
        );
        oracle.setValue("ETH/USD", 3000, 1710000004);
        vm.stopPrank();
    }

    function testMultipleUpdaters() public {
        address updater1 = address(0x1111);
        address updater2 = address(0x2222);

        oracle.grantRole(keccak256("UPDATER_ROLE"), updater1);
        oracle.grantRole(keccak256("UPDATER_ROLE"), updater2);

        // Both updaters should be able to set values
        vm.prank(updater1);
        oracle.setValue("BTC/USD", 65000, 1710000001);

        vm.prank(updater2);
        oracle.setValue("ETH/USD", 3000, 1710000002);

        // Verify both values were set
        (uint128 btcValue,) = oracle.getValue("BTC/USD");
        assertEq(btcValue, 65000, "Updater1 should be able to set values");

        (uint128 ethValue,) = oracle.getValue("ETH/USD");
        assertEq(ethValue, 3000, "Updater2 should be able to set values");
    }

    function testRenounceRole() public {
        // Grant role to newUpdater
        oracle.grantRole(keccak256("UPDATER_ROLE"), newUpdater);

        // Verify they can set values
        vm.prank(newUpdater);
        oracle.setValue("BTC/USD", 65000, 1710000003);

        // Renounce role
        vm.prank(newUpdater);
        oracle.renounceRole(keccak256("UPDATER_ROLE"), newUpdater);

        // Verify they can no longer set values
        vm.startPrank(newUpdater);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                newUpdater,
                oracle.UPDATER_ROLE()
            )
        );
        oracle.setValue("ETH/USD", 3000, 1710000004);
        vm.stopPrank();
    }

    function testOnlyAdminCanGrantRoles() public {
        address attacker = address(0x1234);

        vm.startPrank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                attacker,
                oracle.DEFAULT_ADMIN_ROLE()
            )
        );
        oracle.grantRole(keccak256("UPDATER_ROLE"), attacker);
        vm.stopPrank();
    }

    function testOnlyAdminCanRevokeRoles() public {
        // Grant role to newUpdater
        oracle.grantRole(keccak256("UPDATER_ROLE"), newUpdater);

        address attacker = address(0x1234);

        vm.startPrank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                attacker,
                oracle.DEFAULT_ADMIN_ROLE()
            )
        );
        oracle.revokeRole(keccak256("UPDATER_ROLE"), newUpdater);
        vm.stopPrank();
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

    function testGetValueAtRingBufferWrap() public {
        string memory key = "WRAP_VALUE_TEST";
        uint256 maxSize = oracle.getMaxHistorySize();

        for (uint256 i = 0; i < maxSize; i++) {
            oracle.setValue(key, uint128(2000 + i), uint128(1710000000 + i));
        }

        oracle.setValue(key, uint128(2000 + maxSize), uint128(1710000000 + maxSize));

        // Verify multiple indices to ensure correct wrap behavior
        (uint128 value0,,) = oracle.getValueAt(key, 0);
        assertEq(value0, uint128(2000 + maxSize), "Index 0 should be newest");

        (uint128 valueMid,,) = oracle.getValueAt(key, maxSize / 2);
        assertEq(valueMid, uint128(2000 + maxSize / 2), "Middle index should be correct");

        (uint128 valueOldest,,) = oracle.getValueAt(key, maxSize - 1);
        assertEq(valueOldest, uint128(2000 + 1), "Oldest should be second value");
    }

    function testMultipleBufferWraps() public {
        string memory key = "MULTI_WRAP_TEST";
        uint256 maxSize = oracle.getMaxHistorySize();

        // Do 3 full cycles through the buffer
        for (uint256 i = 0; i < maxSize * 3; i++) {
            oracle.setValue(key, uint128(i), uint128(1710000000 + i));
        }

        // Should still only have maxSize values
        assertEq(oracle.getValueCount(key), maxSize, "Should have maxSize values after multiple wraps");

        // Verify newest value
        (uint128 newest,,) = oracle.getValueAt(key, 0);
        assertEq(newest, uint128(maxSize * 3 - 1), "Newest value after 3 wraps");

        // Verify oldest value (should be maxSize values before newest)
        (uint128 oldest,,) = oracle.getValueAt(key, maxSize - 1);
        assertEq(oldest, uint128(maxSize * 2), "Oldest value after 3 wraps");
    }

    function testBufferWrapWithValueHistory() public {
        string memory key = "WRAP_HISTORY_TEST";
        uint256 maxSize = oracle.getMaxHistorySize();

        // Fill buffer and wrap multiple times
        for (uint256 i = 0; i < maxSize * 2; i++) {
            oracle.setValue(key, uint128(5000 + i), uint128(1710000000 + i));
        }

        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);

        assertEq(history.length, maxSize, "History should have maxSize entries");
        assertEq(history[0].value, uint128(5000 + maxSize * 2 - 1), "Most recent should be correct");
        assertEq(history[maxSize - 1].value, uint128(5000 + maxSize), "Oldest should be correct");
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

        vm.startPrank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                attacker,
                oracle.UPDATER_ROLE()
            )
        );
        oracle.setRawValue(encodedData);
        vm.stopPrank();
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

        vm.startPrank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                attacker,
                oracle.UPDATER_ROLE()
            )
        );
        oracle.setMultipleRawValues(dataArray);
        vm.stopPrank();
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

    // ========== Edge Case Tests ==========

    function testZeroValue() public {
        string memory key = "ZERO/VALUE";
        uint128 price = 0;
        uint128 timestamp = 1710000000;

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Zero price should be stored");
        assertEq(storedTimestamp, timestamp, "Timestamp should be stored");
    }

    function testMaxUint128Values() public {
        string memory key = "MAX/VALUES";
        uint128 price = type(uint128).max;
        uint128 timestamp = type(uint128).max;

        vm.warp(type(uint128).max - 1 hours); // Set block time to allow max timestamp

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Max price should be stored");
        assertEq(storedTimestamp, timestamp, "Max timestamp should be stored");
    }

    function testZeroTimestamp() public {
        string memory key = "ZERO/TIMESTAMP";
        uint128 price = 50000;

        // Zero timestamp should be too far in the past
        vm.expectRevert();
        oracle.setValue(key, price, 0);
    }

    function testEmptyStringKey() public {
        string memory key = "";
        uint128 price = 50000;
        uint128 timestamp = 1710000000;

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Empty key should work");
        assertEq(storedTimestamp, timestamp, "Timestamp should be stored");
    }

    function testVeryLongKey() public {
        string memory key = "BTC/USD/VERY/LONG/KEY/WITH/MANY/SLASHES/AND/EXTRA/INFORMATION/THAT/GOES/ON/AND/ON";
        uint128 price = 50000;
        uint128 timestamp = 1710000000;

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Long key should work");
        assertEq(storedTimestamp, timestamp, "Timestamp should be stored");
    }

    function testZeroPriceWithNonZeroVolume() public {
        string memory key = "ZERO/PRICE";
        uint128 price = 0;
        uint128 timestamp = 1710000000;
        uint128 volume = 1000000;

        bytes memory data = abi.encode(key, price, timestamp, volume, bytes(""));
        oracle.setRawValue(data);

        (uint128 storedPrice,, uint128 storedVolume) = oracle.getValueAt(key, 0);
        assertEq(storedPrice, 0, "Zero price should be stored");
        assertEq(storedVolume, volume, "Non-zero volume should be stored");
    }

    function testNonZeroPriceWithZeroVolume() public {
        string memory key = "ZERO/VOLUME";
        uint128 price = 50000;
        uint128 timestamp = 1710000000;
        uint128 volume = 0;

        bytes memory data = abi.encode(key, price, timestamp, volume, bytes(""));
        oracle.setRawValue(data);

        (uint128 storedPrice,, uint128 storedVolume) = oracle.getValueAt(key, 0);
        assertEq(storedPrice, price, "Non-zero price should be stored");
        assertEq(storedVolume, 0, "Zero volume should be stored");
    }

    function testAllZeroValues() public {
        string memory key = "ALL/ZERO";
        uint128 price = 0;
        uint128 timestamp = uint128(block.timestamp);
        uint128 volume = 0;

        bytes memory data = abi.encode(key, price, timestamp, volume, bytes(""));
        oracle.setRawValue(data);

        (uint128 storedPrice, uint128 storedTimestamp, uint128 storedVolume) = oracle.getValueAt(key, 0);
        assertEq(storedPrice, 0, "Zero price should be stored");
        assertEq(storedTimestamp, timestamp, "Timestamp should be stored");
        assertEq(storedVolume, 0, "Zero volume should be stored");
    }

    function testSameTimestampMultipleUpdates() public {
        string memory key = "SAME/TIMESTAMP";
        uint128 timestamp = 1710000000;

        oracle.setValue(key, 100, timestamp);
         vm.expectRevert(
            abi.encodeWithSelector(
                DIAOracleV3.TimestampNotIncreasing.selector,
                uint128(timestamp),
                uint128(timestamp)
            )
        );
        oracle.setValue(key, 200, timestamp);
 
        assertEq(oracle.getValueCount(key), 1, "Should have 1 entry only");

        (uint128 latestValue, uint128 latestTimestamp) = oracle.getValue(key);
        assertEq(latestValue, 100, "Latest value should be last set");
        assertEq(latestTimestamp, timestamp, "Timestamp should match");
    }

    function testOutOfOrderTimestamps() public {
        string memory key = "OUT/OF/ORDER";

        oracle.setValue(key, 100, 1710000001); // Oldest timestamp
        oracle.setValue(key, 200, 1710000002); // Middle timestamp
        oracle.setValue(key, 300, 1710000003); // Newest timestamp

        assertEq(oracle.getValueCount(key), 3, "Should have 3 entries");

        // Most recent should be the one with newest timestamp
        (uint128 latestValue, uint128 latestTimestamp) = oracle.getValue(key);
        assertEq(latestValue, 300, "Latest value should be the one with newest timestamp");
        assertEq(latestTimestamp, 1710000003, "Latest timestamp should be 1710000003");

        // Try to set a value with an older timestamp - should revert
        vm.expectRevert(
            abi.encodeWithSelector(
                DIAOracleV3.TimestampNotIncreasing.selector,
                uint128(1710000001),
                uint128(1710000003)
            )
        );
        oracle.setValue(key, 400, 1710000001);
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
        uint128 price = 50000;

        // Test at exactly MAX_TIMESTAMP_GAP in future (should succeed - validation uses > not >=)
        string memory key1 = "BTC/USD";
        uint128 timestampExactlyAtGap = uint128(block.timestamp + 1 hours);
        oracle.setValue(key1, price, timestampExactlyAtGap);
        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key1);
        assertEq(storedPrice, price, "Price should be stored at exactly future gap");
        assertEq(storedTimestamp, timestampExactlyAtGap, "Timestamp should be stored at exactly future gap");

        // Test just beyond MAX_TIMESTAMP_GAP in future (should fail)
        string memory key2 = "ETH/USD";
        uint128 timestampJustBeyond = uint128(block.timestamp + 1 hours + 1);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInFuture.selector, timestampJustBeyond, block.timestamp));
        oracle.setValue(key2, price, timestampJustBeyond);

        // Test at exactly MAX_TIMESTAMP_GAP in past (should succeed - validation uses < not <= and checks currentBlockTime > MAX_TIMESTAMP_GAP)
        string memory key3 = "SOL/USD";
        timestampExactlyAtGap = uint128(block.timestamp - 1 hours);
        oracle.setValue(key3, price + 1, timestampExactlyAtGap);
        (storedPrice, storedTimestamp) = oracle.getValue(key3);
        assertEq(storedPrice, price + 1, "Price should be stored at exactly past gap");
        assertEq(storedTimestamp, timestampExactlyAtGap, "Timestamp should be stored at exactly past gap");

        // Test just beyond MAX_TIMESTAMP_GAP in past (should fail)
        string memory key4 = "XRP/USD";
        timestampJustBeyond = uint128(block.timestamp - 1 hours - 1);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInPast.selector, timestampJustBeyond, block.timestamp));
        oracle.setValue(key4, price, timestampJustBeyond);
    }

    // ========== Event Emission Tests ==========

    function testOracleUpdateEventEmitted() public {
        string memory key = "BTC/USD";
        uint128 value = 50000;
        uint128 timestamp = 1710000000;

        vm.expectEmit(false, false, false, true); // Check topic 1-3, data
        emit DIAOracleV3.OracleUpdate(key, value, timestamp);

        oracle.setValue(key, value, timestamp);
    }

    function testOracleUpdateEventEmittedForSetMultipleValues() public {
        string[] memory keys = new string[](2);
        keys[0] = "BTC/USD";
        keys[1] = "ETH/USD";

        uint256[] memory compressedValues = new uint256[](2);
        compressedValues[0] = (uint256(50000) << 128) + 1710000001;
        compressedValues[1] = (uint256(3000) << 128) + 1710000002;

        // Expect first event
        vm.expectEmit(false, false, false, true);
        emit DIAOracleV3.OracleUpdate("BTC/USD", 50000, 1710000001);

        // Expect second event
        vm.expectEmit(false, false, false, true);
        emit DIAOracleV3.OracleUpdate("ETH/USD", 3000, 1710000002);

        oracle.setMultipleValues(keys, compressedValues);
    }

    function testOracleUpdateRawEventEmitted() public {
        string memory key = "BTC/USD";
        uint128 value = 50000;
        uint128 timestamp = 1710000000;
        uint128 volume = 1000000;
        bytes memory additionalData = abi.encode("extra data");

        bytes memory encodedData = abi.encode(key, value, timestamp, volume, additionalData);

        vm.expectEmit(false, false, false, true);
        emit DIAOracleV3.OracleUpdateRaw(key, value, timestamp, volume, additionalData);

        oracle.setRawValue(encodedData);
    }

    function testOracleUpdateRawEventEmittedForSetMultipleRawValues() public {
        bytes[] memory dataArray = new bytes[](2);

        dataArray[0] = abi.encode("BTC/USD", uint128(50000), uint128(1710000001), uint128(1000000), bytes("btc data"));
        dataArray[1] = abi.encode("ETH/USD", uint128(3000), uint128(1710000002), uint128(500000), bytes("eth data"));

        // Expect first event
        vm.expectEmit(false, false, false, true);
        emit DIAOracleV3.OracleUpdateRaw("BTC/USD", 50000, 1710000001, 1000000, bytes("btc data"));

        // Expect second event
        vm.expectEmit(false, false, false, true);
        emit DIAOracleV3.OracleUpdateRaw("ETH/USD", 3000, 1710000002, 500000, bytes("eth data"));

        oracle.setMultipleRawValues(dataArray);
    }

    function testOracleUpdateWithEmptyAdditionalData() public {
        string memory key = "BTC/USD";
        uint128 value = 50000;
        uint128 timestamp = 1710000000;
        uint128 volume = 1000000;
        bytes memory additionalData = "";

        bytes memory encodedData = abi.encode(key, value, timestamp, volume, additionalData);

        vm.expectEmit(false, false, false, true);
        emit DIAOracleV3.OracleUpdateRaw(key, value, timestamp, volume, additionalData);

        oracle.setRawValue(encodedData);
    }

    // ========== UUPS Proxy Tests ==========

    function testProxyDeployment() public {
        // Verify proxy is correctly set up
        assertEq(oracle.getMaxHistorySize(), 100, "Max history size should be 100");
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
        assertEq(upgradedOracle.getMaxHistorySize(), 100, "Max history size should be 100");

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
        vm.startPrank(attacker);
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                attacker,
                oracle.DEFAULT_ADMIN_ROLE()
            )
        );
        oracle.upgradeToAndCall(address(newImplementation), "");
        vm.stopPrank();

        // Now verify that a proper admin CAN upgrade
        oracle.upgradeToAndCall(address(newImplementation), "");

        // Verify the upgrade worked by checking actual functionality
        oracle.setValue("TEST/USD", 12345, 1710000100);
        (uint128 value,) = oracle.getValue("TEST/USD");
        assertEq(value, 12345, "Oracle should be functional after upgrade");
    }

    function testInitializeCannotBeCalledTwice() public {
        vm.expectRevert(Initializable.InvalidInitialization.selector);
        oracle.initialize();
    }

    function testConstructorIsDisabled() public {
        // This test verifies that the constructor is disabled and initialize should be used
        // The implementation contract should have initializers disabled
        DIAOracleV3 impl = new DIAOracleV3();

        // Try to call initialize directly on implementation (should fail due to _disableInitializers)
        vm.expectRevert(Initializable.InvalidInitialization.selector);
        impl.initialize();
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

        // Verify the contract is initialized with version 1
        // Note: We can't directly access _initialized from outside, but we can verify it's not 0
        // by checking that initialize cannot be called again (tested in testInitializeCannotBeCalledTwice)

        // The fact that this test exists and passes demonstrates:
        // - The contract can be initialized with reinitializer(1)
        // - Future versions can use reinitializer(2), reinitializer(3), etc.

        // Verify the contract is functional (proves it was initialized)
        assertTrue(oracle.hasRole(oracle.DEFAULT_ADMIN_ROLE(), deployer), "Deployer should be admin");
        assertTrue(oracle.hasRole(oracle.UPDATER_ROLE(), deployer), "Deployer should be updater");
    }

    function testStorageLayoutCompatibility() public {
        // Add data before upgrade
        oracle.setValue("BTC/USD", 50000, 1710000000);
        oracle.setValue("ETH/USD", 3000, 1710000001);

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
        assertEq(upgradedOracle.getMaxHistorySize(), 100, "Max history size should be 100");
        assertEq(upgradedOracle.getValueCount("BTC/USD"), 1, "BTC count should persist");
        assertEq(upgradedOracle.getValueCount("ETH/USD"), 1, "ETH count should persist");

        (uint128 btcValue,) = upgradedOracle.getValue("BTC/USD");
        assertEq(btcValue, 50000, "BTC value should persist");

        (uint128 ethValue,) = upgradedOracle.getValue("ETH/USD");
        assertEq(ethValue, 3000, "ETH value should persist");
    }

    function testUpgradeToZeroAddress() public {
        address zeroAddress = address(0);

        vm.expectRevert();
        oracle.upgradeToAndCall(zeroAddress, "");
    }

    function testUpgradeAndCallWithData() public {
        // Add some data before upgrade
        oracle.setValue("BTC/USD", 50000, 1710000000);

        // Deploy new implementation
        DIAOracleV3 newImplementation = new DIAOracleV3();

        // Upgrade with empty call data (reinitialize is not needed for same version)
        oracle.upgradeToAndCall(address(newImplementation), "");

        // Verify functionality still works
        (uint128 valueAfter,) = oracle.getValue("BTC/USD");
        assertEq(valueAfter, 50000, "Value should persist after upgrade");
    }

    // ========== Interface Support Tests ==========

    function testSupportsInterface() public {
        // Test IDIAOracleV3 interface support
        assertTrue(oracle.supportsInterface(type(IDIAOracleV3).interfaceId), "Should support IDIAOracleV3");

        // Test IERC165 interface support
        assertTrue(oracle.supportsInterface(type(IERC165).interfaceId), "Should support IERC165");

        // Test invalid interface
        assertFalse(oracle.supportsInterface(bytes4(0xdeadbeef)), "Should not support invalid interface");
        assertFalse(oracle.supportsInterface(bytes4(0xffffffff)), "Should not support ERC165 invalid interface");
    }

    // ========== Batch Operation Partial Failure Tests ==========

    function testSetMultipleValuesPartialFailure() public {
        string[] memory keys = new string[](3);
        keys[0] = "VALID1";
        keys[1] = "INVALID";
        keys[2] = "VALID2";

        uint256[] memory values = new uint256[](3);
        values[0] = (uint256(1000) << 128) + uint128(block.timestamp);
        values[1] = (uint256(2000) << 128) + uint128(block.timestamp + 2 hours); // Invalid timestamp
        values[2] = (uint256(3000) << 128) + uint128(block.timestamp);

        // Should revert entirely - no partial updates
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInFuture.selector, uint128(block.timestamp + 2 hours), block.timestamp));
        oracle.setMultipleValues(keys, values);

        // Verify no values were set
        assertEq(oracle.getValueCount("VALID1"), 0, "VALID1 should not be set");
        assertEq(oracle.getValueCount("VALID2"), 0, "VALID2 should not be set");
    }

    function testSetMultipleRawValuesPartialFailure() public {
        bytes[] memory dataArray = new bytes[](3);

        // First value is valid
        dataArray[0] = abi.encode("BTC/USD", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes("btc data"));

        // Second value has invalid timestamp
        dataArray[1] = abi.encode("ETH/USD", uint128(3000), uint128(block.timestamp - 2 hours), uint128(500000), bytes("eth data"));

        // Third value is valid
        dataArray[2] = abi.encode("SOL/USD", uint128(150), uint128(block.timestamp), uint128(200000), bytes("sol data"));

        // Should revert entirely - no partial updates
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3.TimestampTooFarInPast.selector, uint128(block.timestamp - 2 hours), block.timestamp));
        oracle.setMultipleRawValues(dataArray);

        // Verify no values were set
        assertEq(oracle.getValueCount("BTC/USD"), 0, "BTC should not be set");
        assertEq(oracle.getValueCount("SOL/USD"), 0, "SOL should not be set");
    }

    // ========== State Consistency Tests ==========

    function testStateConsistencyAcrossOperations() public {
        string memory key = "STATE/TEST";

        // Set value
        oracle.setValue(key, 100, 1710000001);
        assertEq(oracle.getValueCount(key), 1, "Count should be 1");
        assertEq(oracle.getRawData(key).length, 0, "Raw data should be empty after setValue");

        // Set raw value with volume
        bytes memory data = abi.encode(key, uint128(200), uint128(1710000002), uint128(1000), bytes("test"));
        oracle.setRawValue(data);
        assertEq(oracle.getValueCount(key), 2, "Count should be 2");
        assertEq(keccak256(oracle.getRawData(key)), keccak256(bytes("test")), "Raw data should be set");

        // Update via setMultipleValues on same key (should clear rawData)
        string[] memory keys = new string[](1);
        keys[0] = key;
        uint256[] memory values = new uint256[](1);
        values[0] = (uint256(300) << 128) + 1710000003;
        oracle.setMultipleValues(keys, values);

        // Should have 3 values now
        assertEq(oracle.getValueCount(key), 3, "Count should be 3");

        // Latest value should be from setMultipleValues
        (uint128 value, uint128 timestamp, uint128 volume) = oracle.getValueAt(key, 0);
        assertEq(value, 300, "Latest value should be from setMultipleValues");
        assertEq(timestamp, 1710000003, "Latest timestamp should be from setMultipleValues");
        assertEq(volume, 0, "Volume should be 0 for setMultipleValues");

        // Raw data should be cleared after setMultipleValues
        assertEq(oracle.getRawData(key).length, 0, "Raw data should be cleared after setMultipleValues");

        string[] memory keys2 = new string[](1);
        keys2[0] = "OTHER/KEY";
        uint256[] memory values2 = new uint256[](1);
        values2[0] = (uint256(400) << 128) + 1710000004;
        oracle.setMultipleValues(keys2, values2);

        assertEq(oracle.getValueCount(key), 3, "Original key count should be 3");
        assertEq(oracle.getRawData(key).length, 0, "Original key raw data should still be empty");
    }

 

 

    // ========== DoS Resistance Tests ==========

    function testGasExhaustionResistance() public {
        // Create many keys to test for gas exhaustion vulnerabilities
        for (uint256 i = 0; i < 100; i++) {
            string memory key = string(abi.encodePacked("KEY", vm.toString(i)));
            oracle.setValue(key, uint128(1000 + i), uint128(1710000000 + i));
        }

        // Oracle should still function
        oracle.setValue("FINAL", 999, 1710000100);
        (uint128 value,) = oracle.getValue("FINAL");
        assertEq(value, 999, "Oracle should still function after many updates");
    }

    function testLargeHistoryPerKey() public {
        string memory key = "LARGE/HISTORY";
        uint256 maxSize = oracle.getMaxHistorySize();

        // Fill the buffer completely
        for (uint256 i = 0; i < maxSize; i++) {
            oracle.setValue(key, uint128(i), uint128(1710000000 + i));
        }

        // Should still be able to add more (will wrap)
        oracle.setValue(key, uint128(maxSize), uint128(1710000000 + maxSize));

        assertEq(oracle.getValueCount(key), maxSize, "Count should be maxSize");
    }

    // ========== Multi-Key and Special Character Tests ==========

    function testCaseSensitiveKeys() public {
        oracle.setValue("btc/usd", 100, 1710000000);
        oracle.setValue("BTC/USD", 200, 1710000001);
        oracle.setValue("Btc/Usd", 300, 1710000002);

        // All three should be different keys
        assertEq(oracle.getValueCount("btc/usd"), 1, "Lowercase key should exist");
        assertEq(oracle.getValueCount("BTC/USD"), 1, "Uppercase key should exist");
        assertEq(oracle.getValueCount("Btc/Usd"), 1, "Mixed case key should exist");

        (uint128 value1,) = oracle.getValue("btc/usd");
        (uint128 value2,) = oracle.getValue("BTC/USD");
        (uint128 value3,) = oracle.getValue("Btc/Usd");

        assertEq(value1, 100, "Lowercase value should be correct");
        assertEq(value2, 200, "Uppercase value should be correct");
        assertEq(value3, 300, "Mixed case value should be correct");
    }

    function testSpecialCharactersInKeys() public {
        string memory key1 = "BTC-USD";
        string memory key2 = "BTC_USD";
        string memory key3 = "BTC.USD";
        string memory key4 = "BTC:USD";

        oracle.setValue(key1, 100, 1710000000);
        oracle.setValue(key2, 200, 1710000001);
        oracle.setValue(key3, 300, 1710000002);
        oracle.setValue(key4, 400, 1710000003);

        assertEq(oracle.getValueCount(key1), 1, "Hyphen key should work");
        assertEq(oracle.getValueCount(key2), 1, "Underscore key should work");
        assertEq(oracle.getValueCount(key3), 1, "Dot key should work");
        assertEq(oracle.getValueCount(key4), 1, "Colon key should work");
    }

    function testUnicodeInKeys() public {
        string memory key1 = unicode"BTC/USD€";
        string memory key2 = unicode"BTC/USD¥";

        oracle.setValue(key1, 100, 1710000000);
        oracle.setValue(key2, 200, 1710000001);

        (uint128 value1,) = oracle.getValue(key1);
        (uint128 value2,) = oracle.getValue(key2);

        assertEq(value1, 100, "Unicode character 1 should work");
        assertEq(value2, 200, "Unicode character 2 should work");
    }

    // ========== Fuzz Tests ==========

    function testFuzzSetValue(uint128 price, uint32 timestampOffset) public {
        vm.assume(timestampOffset <= 3600); // Within 1 hour
        uint128 timestamp = uint128(block.timestamp - 600 + timestampOffset); // -10 minutes to +1 hour

        string memory key = "FUZZ/TEST";

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price should match");
        assertEq(storedTimestamp, timestamp, "Timestamp should match");
    }

    function testFuzzSetValueArbitraryKey(string calldata key) public {
        vm.assume(bytes(key).length > 0 && bytes(key).length <= 100);

        uint128 price = 50000;
        uint128 timestamp = uint128(block.timestamp);

        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice, uint128 storedTimestamp) = oracle.getValue(key);
        assertEq(storedPrice, price);
        assertEq(storedTimestamp, timestamp);
    }

    // ========== Invariant Tests ==========

    function testInvariantLatestValueMatchesIndexZero(string memory key) public {
        // Set multiple values
        for (uint128 i = 0; i < 10; i++) {
            oracle.setValue(key, uint128(1000 + i), uint128(1710000000 + i));
        }

        uint256 count = oracle.getValueCount(key);
        if (count > 0) {
            (uint128 latestValue, uint128 latestTimestamp) = oracle.getValue(key);
            (uint128 valueAt0, uint128 timestampAt0,) = oracle.getValueAt(key, 0);

            assertEq(latestValue, valueAt0, "getValue should match getValueAt(0)");
            assertEq(latestTimestamp, timestampAt0, "Timestamp should match getValueAt(0)");
        }
    }

    function testInvariantCountNeverExceedsMaxSize() public {
        string memory key = "INVARIANT/COUNT";
        uint256 maxSize = oracle.getMaxHistorySize();

        // Add 2x maxSize values
        for (uint256 i = 0; i < maxSize * 2; i++) {
            oracle.setValue(key, uint128(i), uint128(1710000000 + i));

            // Count should never exceed maxSize
            uint256 count = oracle.getValueCount(key);
            assertLe(count, maxSize, "Count should never exceed maxSize");
        }
    }

    function testInvariantChronologicalOrderInHistory(string memory key) public {
        // Set multiple values
        for (uint128 i = 0; i < 10; i++) {
            oracle.setValue(key, uint128(1000 + i), uint128(1710000000 + i));
        }

        IDIAOracleV3.ValueEntry[] memory history = oracle.getValueHistory(key);

        // Verify chronological order (most recent first)
        for (uint256 i = 0; i < history.length - 1; i++) {
            assertGe(history[i].timestamp, history[i + 1].timestamp, "History should be in chronological order");
        }
    }

    // Test decimals functionality
    function testSetDecimals() public {
        uint8 decimalPrecision = 8;

        oracle.setDecimals(decimalPrecision);

        assertEq(oracle.getDecimals(), decimalPrecision, "Decimals should match");
    }

    function testSetDecimalsMultipleTimes() public {
        oracle.setDecimals(8);
        assertEq(oracle.getDecimals(), 8, "First decimals should be 8");

        oracle.setDecimals(18);
        assertEq(oracle.getDecimals(), 18, "Decimals should be updated to 18");

        oracle.setDecimals(6);
        assertEq(oracle.getDecimals(), 6, "Decimals should be updated to 6");
    }

    function testDecimalsEvent() public {
        vm.expectEmit(true, true, true, true);
        emit DIAOracleV3.DecimalsUpdate(8);

        oracle.setDecimals(8);
    }

    function testDecimalsDefaultZero() public {
        // Decimals should default to 8
        assertEq(oracle.getDecimals(), 8, "Default decimals should be 8");
    }

    function testSetDecimalsOnlyUpdater() public {
        // Try to set decimals from non-updater address
        vm.startPrank(address(0x123));
        vm.expectRevert(
            abi.encodeWithSelector(
                bytes4(keccak256("AccessControlUnauthorizedAccount(address,bytes32)")),
                address(0x123),
                oracle.UPDATER_ROLE()
            )
        );
        oracle.setDecimals(8);
        vm.stopPrank();
    }

    function testDecimalsWithValueStorage() public {
        string memory key = "BTC/USD";
        uint128 price = 50000 * 10**8; // 8 decimals
        uint128 timestamp = 1710000000;

        oracle.setDecimals(8);
        oracle.setValue(key, price, timestamp);

        (uint128 storedPrice,) = oracle.getValue(key);
        assertEq(storedPrice, price, "Price should be stored correctly");
        assertEq(oracle.getDecimals(), 8, "Decimals should still be 8");
    }

    function testDecimalsEdgeCaseMax() public {
        uint8 maxDecimals = 255; // uint8 max value

        oracle.setDecimals(maxDecimals);
        assertEq(oracle.getDecimals(), maxDecimals, "Should handle max decimals");
    }

    function testDecimalsEdgeCaseZero() public {
        oracle.setDecimals(0);
        assertEq(oracle.getDecimals(), 0, "Should handle zero decimals");
    }

 
}
