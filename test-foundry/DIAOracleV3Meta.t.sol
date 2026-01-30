// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.29;

import "forge-std/Test.sol";
import "../contracts/DIAOracleV3.sol";
import "../contracts/DIAOracleV3Meta.sol";
import "../contracts/IDIAOracleV3.sol";
import "../contracts/methodologies/AveragePriceMethodology.sol";
import "../contracts/methodologies/MedianPriceMethodology.sol";

// Mock Oracle contract implementing IDIAOracleV3
contract MockDIAOracleV3 is IDIAOracleV3 {
    IDIAOracleV3.ValueEntry[] private history;
    uint256 private maxHistorySize;

    constructor(uint256 _maxHistorySize) {
        maxHistorySize = _maxHistorySize;
    }

    function setValue(string memory, uint128 value, uint128 timestamp) external {
        history.push(IDIAOracleV3.ValueEntry(value, timestamp));
        if (history.length > maxHistorySize) {
            // Remove oldest
            for (uint256 i = 0; i < history.length - 1; i++) {
                history[i] = history[i + 1];
            }
            history.pop();
        }
    }

    function getValue(string memory key) external view returns (uint128, uint128) {
        if (history.length == 0) return (0, 0);
        IDIAOracleV3.ValueEntry memory latest = history[history.length - 1];
        return (latest.value, latest.timestamp);
    }

    function setMultipleValues(string[] memory, uint256[] memory) external {
        revert("Not implemented");
    }

    function getValueAt(string memory, uint256 index) external view returns (uint128 value, uint128 timestamp) {
        require(index < history.length, "Invalid index");
        IDIAOracleV3.ValueEntry memory entry = history[history.length - 1 - index];
        return (entry.value, entry.timestamp);
    }

    function getValueHistory(string memory) external view returns (IDIAOracleV3.ValueEntry[] memory) {
        uint256 length = history.length;
        IDIAOracleV3.ValueEntry[] memory result = new IDIAOracleV3.ValueEntry[](length);
        for (uint256 i = 0; i < length; i++) {
            result[i] = history[length - 1 - i];
        }
        return result;
    }

    function getValueCount(string memory) external view returns (uint256) {
        return history.length;
    }

    function setMaxHistorySize(uint256) external {
        revert("Not implemented");
    }

    function getMaxHistorySize() external view returns (uint256) {
        return maxHistorySize;
    }
}

contract DIAOracleV3MetaTest is Test {
    DIAOracleV3Meta public oracleMeta;
    DIAOracleV3 public oracle1;
    DIAOracleV3 public oracle2;
    DIAOracleV3 public oracle3;

    address public admin = address(0x123);

    function setUp() public {
        vm.startPrank(admin);
        // Deploy methodology first
        AveragePriceMethodology methodology = new AveragePriceMethodology();
        oracleMeta = new DIAOracleV3Meta(address(methodology));
        oracle1 = new DIAOracleV3(10);
        oracle2 = new DIAOracleV3(10);
        oracle3 = new DIAOracleV3(10);
        
        // Grant UPDATER_ROLE to this test contract so we can call setValue
        oracle1.grantRole(keccak256("UPDATER_ROLE"), address(this));
        oracle2.grantRole(keccak256("UPDATER_ROLE"), address(this));
        oracle3.grantRole(keccak256("UPDATER_ROLE"), address(this));
        vm.stopPrank();
    }

    function testAddOracle() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        assertEq(oracleMeta.getNumOracles(), 2);
        vm.stopPrank();
    }

    function testGetValueWithHistory() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));
        oracleMeta.setThreshold(2);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        // Set values in oracles with history
        oracle1.setValue("BTC", 100, uint128(block.timestamp));
        oracle2.setValue("BTC", 200, uint128(block.timestamp));
        oracle3.setValue("BTC", 300, uint128(block.timestamp));

        // Add more history
        oracle1.setValue("BTC", 110, uint128(block.timestamp + 1));
        oracle2.setValue("BTC", 210, uint128(block.timestamp + 1));
        oracle3.setValue("BTC", 310, uint128(block.timestamp + 1));

        // Get median value
        // Oracle1 average: (100 + 110) / 2 = 105
        // Oracle2 average: (200 + 210) / 2 = 205
        // Oracle3 average: (300 + 310) / 2 = 305
        // Median of [105, 205, 305] = 205
        (uint128 value, uint128 timestamp) = oracleMeta.getValue("BTC");
        assertEq(value, 205, "Median should be 205 (median of averages)");
    }


    function testGetValueFailsWithoutEnoughOracles() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setThreshold(2);
        oracleMeta.setTimeoutSeconds(100);
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert();
        oracleMeta.getValue("BTC");
    }

    function testGetValueWithInsufficientHistory() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        // Only oracle1 has 2 values, oracle2 has 1
        oracle1.setValue("BTC", 100, uint128(block.timestamp));
        oracle1.setValue("BTC", 110, uint128(block.timestamp + 1));
        oracle2.setValue("BTC", 200, uint128(block.timestamp));

        // Both oracles have values, should get aggregated value
        (uint128 value, ) = oracleMeta.getValue("BTC");
        // Oracle1: average of [110] = 110
        // Oracle2: average of [200] = 200
        // Sorted: [110, 200], medianIndex = 1 -> values[1] = 200
        assertEq(value, 200, "Should get median (higher value when 2 values): 200");
    }

    function testGetValueWithCustomParams() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));
        oracleMeta.setThreshold(2);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        // Deploy median methodology
        MedianPriceMethodology medianMethodology = new MedianPriceMethodology();

        // Set values in oracles
        oracle1.setValue("BTC", 100, uint128(block.timestamp));
        oracle2.setValue("BTC", 200, uint128(block.timestamp));
        oracle3.setValue("BTC", 300, uint128(block.timestamp));

        oracle1.setValue("BTC", 110, uint128(block.timestamp + 1));
        oracle2.setValue("BTC", 210, uint128(block.timestamp + 1));
        oracle3.setValue("BTC", 310, uint128(block.timestamp + 1));

        // Test with default (average methodology, windowSize 10)
        (uint128 valueDefault, ) = oracleMeta.getValue("BTC");
        // Oracle1: average of [110, 100] = 105
        // Oracle2: average of [210, 200] = 205
        // Oracle3: average of [310, 300] = 305
        // Median of [105, 205, 305] = 205
        assertEq(valueDefault, 205, "Default should use average methodology");

        // Test with custom windowSize=1, median methodology, custom timeout=2000, custom threshold=2
        (uint128 valueCustom, ) = oracleMeta.getValueByConfig("BTC", 1, address(medianMethodology), 2000, 2);
        // Oracle1: median of [110] = 110
        // Oracle2: median of [210] = 210
        // Oracle3: median of [310] = 310
        // Median of [110, 210, 310] = 210
        assertEq(valueCustom, 210, "Custom should use median methodology with windowSize=1");
    }
}
