// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "forge-std/Test.sol";
import "../contracts/DIAOracleV3.sol";
import "../contracts/IDIAOracleV3.sol";
import "forge-std/console.sol";

contract DIAOracleV3Test is Test {
    DIAOracleV3 oracle;
    address deployer = address(this);  
    address newUpdater = address(0xBEEF);
    uint256 constant DEFAULT_MAX_HISTORY_SIZE = 10;

    function setUp() public {
        oracle = new DIAOracleV3(DEFAULT_MAX_HISTORY_SIZE);
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

         (uint128 value1, uint128 timestamp1) = oracle.getValueAt(key, 1);
        assertEq(value1, 3100, "Second value should be 3100");
        assertEq(timestamp1, 1710000002, "Second timestamp should match");

        // Get the oldest value (index 2)
        (uint128 value2, uint128 timestamp2) = oracle.getValueAt(key, 2);
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
        (uint128 oldestValue, ) = oracle.getValueAt(key, maxSize - 1);
        assertEq(oldestValue, 105, "Oldest value should be 105 (the 6th value)");

        // The newest value should be the last one
        (uint128 newestValue, ) = oracle.getValueAt(key, 0);
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

        (uint128 btcValue, ) = oracle.getValueAt("BTC/USD", 0);
        assertEq(btcValue, 51000, "BTC most recent should be 51000");

        (uint128 ethValue, ) = oracle.getValueAt("ETH/USD", 0);
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

        (uint128 ethValue, ) = oracle.getValueAt("ETH/USD", 0);
        assertEq(ethValue, 3100, "ETH most recent should be 3100");

        (uint128 solValue, ) = oracle.getValueAt("SOL/USD", 0);
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

        (uint128 storedPrice, ) = oracle.getValue("BTC/USD");
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
        (uint128 valueAt0, uint128 timestampAt0) = oracle.getValueAt(key, 0);
        assertEq(latestValue, valueAt0, "getValue should match getValueAt(0)");
        assertEq(latestTimestamp, timestampAt0, "getValue timestamp should match getValueAt(0)");
    }
}
