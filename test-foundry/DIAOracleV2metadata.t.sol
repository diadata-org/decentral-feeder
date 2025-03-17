// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "forge-std/Test.sol";
import "../contracts/DIAOracleV2Meta.sol";

// Mock Oracle contract implementing IDIAOracleV2
contract MockDIAOracle is IDIAOracleV2 {
    uint128 private storedValue;
    uint128 private storedTimestamp;

    function setValue(
        string memory,
        uint128 value,
        uint128 timestamp
    ) external override {
        storedValue = value;
        storedTimestamp = timestamp;
    }

    function getValue(
        string memory
    ) external view override returns (uint128, uint128) {
        return (storedValue, storedTimestamp);
    }

    function updateOracleUpdaterAddress(address) external override {}
}

// Forge test contract
contract DIAOracleV2MetaTest is Test {
    DIAOracleV2Meta public oracleMeta;
    MockDIAOracle public oracle1;
    MockDIAOracle public oracle2;
    MockDIAOracle public oracle3;

    address public admin = address(0x123);

    function setUp() public {
        vm.startPrank(admin);
        oracleMeta = new DIAOracleV2Meta();
        oracle1 = new MockDIAOracle();
        oracle2 = new MockDIAOracle();
        oracle3 = new MockDIAOracle();
        vm.stopPrank();
    }

    function testAddOracle() public {
        vm.startPrank(admin);

        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));

        // Assert that numOracles increased
        assertEq(oracleMeta.getNumOracles(), 2);

        vm.stopPrank();
    }

    function testAddDuplicateOracle() public {
        vm.startPrank(admin);

        oracleMeta.addOracle(address(oracle1));
        vm.expectRevert();
        oracleMeta.addOracle(address(oracle1));

        vm.stopPrank();
    }

    function testRemoveOracle() public {
        vm.startPrank(admin);

        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));

        // Remove oracle2 and check numOracles
        oracleMeta.removeOracle(address(oracle2));
        assertEq(oracleMeta.getNumOracles(), 2);

        vm.stopPrank();
    }

    function testRemoveNonExistentOracle() public {
        vm.startPrank(admin);
        vm.expectRevert();
        oracleMeta.removeOracle(address(oracle2));

        vm.stopPrank();
    }

    function testSetThreshold() public {
        vm.startPrank(admin);

        oracleMeta.setThreshold(2);
        assertEq(oracleMeta.getThreshold(), 2);

        vm.stopPrank();
    }

    function testZeroSetThreshold() public {
        vm.startPrank(admin);

        vm.expectRevert();
        oracleMeta.setThreshold(0);

        vm.stopPrank();
    }

    function testSetTimeout() public {
        vm.startPrank(admin);

        oracleMeta.setTimeoutSeconds(100);
        assertEq(oracleMeta.getTimeoutSeconds(), 100);

        vm.stopPrank();
    }

    function testZeroSetTimeout() public {
        vm.startPrank(admin);

        vm.expectRevert();
        oracleMeta.setTimeoutSeconds(0);

        vm.stopPrank();
    }

    function testGetValueMedian() public {
        vm.startPrank(admin);

        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));

        oracleMeta.setThreshold(2);
        oracleMeta.setTimeoutSeconds(1000);

        vm.stopPrank();

        // Set values in oracles
        oracle1.setValue("BTC", 100, uint128(block.timestamp));
        oracle2.setValue("BTC", 200, uint128(block.timestamp));
        oracle3.setValue("BTC", 300, uint128(block.timestamp));
        oracle3.setValue("BTC", 400, uint128(block.timestamp + 2000)); //timeout

        // Fetch median value
        (uint128 value, uint128 timestamp) = oracleMeta.getValue("BTC");

        assertEq(value, 200, "Median should be 200");
        assertEq(
            timestamp,
            uint128(block.timestamp),
            "Timestamp should match current time"
        );
    }

    function testGetValueFailsWithoutEnoughOracles() public {
        vm.startPrank(admin);

        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setThreshold(2);
        oracleMeta.setTimeoutSeconds(100);

        vm.stopPrank();

        // Set value
        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        // Expect revert due to insufficient valid oracles
        vm.expectRevert();
        oracleMeta.getValue("BTC");
    }
}
