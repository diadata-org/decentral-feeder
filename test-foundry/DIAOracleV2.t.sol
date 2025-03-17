// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import "forge-std/Test.sol";
import "../contracts/DIAOracleV2.sol";
import "forge-std/console.sol";

contract DIAOracleV2Test is Test {
    DIAOracleV2 oracle;
    address deployer = address(this); // Test contract as deployer
    address newUpdater = address(0xBEEF);

    function setUp() public {
        oracle = new DIAOracleV2();
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

    function testSetMultipleValuesAndGetValue() public {
        string[] memory keys = new string[](2);

        keys[0] = "ETH/USD";
        keys[1] = "SOL/USD";

        uint256[] memory compressedValues = new uint256[](2);

        uint128 ethPrice = 3000;
        uint128 solPrice = 150;
        uint128 timestamp = 1710000001;

        compressedValues[0] = (uint256(ethPrice) << 128) + timestamp;
        compressedValues[1] = (uint256(solPrice) << 128) + timestamp;

        console.log("---");

        oracle.setMultipleValues(keys, compressedValues);

        (uint128 storedEthPrice, uint128 storedEthTimestamp) = oracle.getValue(
            "ETH/USD"
        );
        (uint128 storedSolPrice, uint128 storedSolTimestamp) = oracle.getValue(
            "SOL/USD"
        );

        assertEq(storedEthPrice, ethPrice, "ETH price mismatch");
        assertEq(storedEthTimestamp, timestamp, "ETH timestamp mismatch");
        assertEq(storedSolPrice, solPrice, "SOL price mismatch");
        assertEq(storedSolTimestamp, timestamp, "SOL timestamp mismatch");
    }

    function testOnlyUpdaterCanSetValue() public {
        address attacker = address(0x1234);

        vm.prank(attacker);
        vm.expectRevert("Only the oracleUpdater role can update the oracle.");
        oracle.setValue("BTC/USD", 60000, 1710000002);
    }

    function testUpdateOracleUpdaterAddress() public {
        // oracle.grantRole(newUpdater);
        oracle.grantRole(keccak256("UPDATER_ROLE"), newUpdater);

        vm.prank(newUpdater);
        oracle.setValue("BTC/USD", 65000, 1710000003);

        (uint128 storedPrice, ) = oracle.getValue("BTC/USD");
        assertEq(
            storedPrice,
            65000,
            "New updater should be able to set values"
        );
    }

    function testUnauthorizedGrantRole() public {
        address user = address(0x987);
        vm.startPrank(user);
        vm.expectRevert();
        oracle.grantRole(keccak256("UPDATER_ROLE"), user);
        vm.stopPrank();
    }
}
