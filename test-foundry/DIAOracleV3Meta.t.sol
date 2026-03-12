// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "../contracts/DIAOracleV3.sol";
import "../contracts/DIAOracleV3Meta.sol";
import "../contracts/IDIAOracleV3.sol";
import "../contracts/methodologies/AveragePriceMethodology.sol";
import "../contracts/methodologies/MedianPriceMethodology.sol";
import {ERC165} from "@openzeppelin/contracts/utils/introspection/ERC165.sol";

// Mock Oracle contract implementing IDIAOracleV3
contract MockDIAOracleV3 is IDIAOracleV3, ERC165 {
    IDIAOracleV3.ValueEntry[] private history;
    uint256 private maxHistorySize;
    uint8 public decimals;

    constructor(uint256 _maxHistorySize) {
        maxHistorySize = _maxHistorySize;
    }

    function setValue(string memory, uint128 value, uint128 timestamp) external {
        history.push(IDIAOracleV3.ValueEntry(value, timestamp, 0));
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

    function getValueAt(string memory, uint256 index)
        external
        view
        returns (uint128 value, uint128 timestamp, uint128 volume)
    {
        require(index < history.length, "Invalid index");
        IDIAOracleV3.ValueEntry memory entry = history[history.length - 1 - index];
        return (entry.value, entry.timestamp, entry.volume);
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

    function setRawValue(bytes calldata) external {
        revert("Not implemented");
    }

    function setMultipleRawValues(bytes[] calldata) external {
        revert("Not implemented");
    }

    function getRawData(string memory) external view returns (bytes memory) {
        return "";
    }

    function setDecimals(uint8 _decimals) external {
        decimals = _decimals;
    }

    function getDecimals() external view returns (uint8) {
        return decimals;
    }

    function supportsInterface(bytes4 interfaceId) public view virtual override(ERC165, IERC165) returns (bool) {
        return interfaceId == type(IDIAOracleV3).interfaceId || super.supportsInterface(interfaceId);
    }
}

contract DIAOracleV3MetaTest is Test {
    DIAOracleV3Meta public oracleMeta;
    DIAOracleV3 public oracle1;
    DIAOracleV3 public oracle2;
    DIAOracleV3 public oracle3;

    address public admin = address(0x123);

    function deployOracle(uint256 maxHistorySize) internal returns (DIAOracleV3) {
        DIAOracleV3 implementation = new DIAOracleV3();
        bytes memory initData = abi.encodeWithSelector(
            DIAOracleV3.initialize.selector,
            maxHistorySize
        );
        ERC1967Proxy proxy = new ERC1967Proxy(address(implementation), initData);
        return DIAOracleV3(address(proxy));
    }

    function setUp() public {
        vm.startPrank(admin);
        // Deploy methodology first
        AveragePriceMethodology methodology = new AveragePriceMethodology();
        oracleMeta = new DIAOracleV3Meta(address(methodology));
        oracle1 = deployOracle(10);
        oracle2 = deployOracle(10);
        oracle3 = deployOracle(10);

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

        // setThreshold should reject threshold above numOracles
        vm.expectRevert(abi.encodeWithSelector(
            DIAOracleV3Meta.InvalidThreshold.selector,
            2  
        ));
        oracleMeta.setThreshold(2);

        // Valid: set threshold equal to numOracles
        oracleMeta.setThreshold(1);
        assertEq(oracleMeta.getThreshold(), 1);

        vm.stopPrank();
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
        (uint128 value,) = oracleMeta.getValue("BTC");
        // Oracle1: average of [110, 100] = 105
        // Oracle2: average of [200] = 200
        // Sorted: [105, 200], validValues = 2
        // Median of even count: (averages[0] + averages[1]) / 2 = (105 + 200) / 2 = 152
        assertEq(value, 152, "Should get median of averages: (105 + 200) / 2 = 152");
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
        (uint128 valueDefault,) = oracleMeta.getValue("BTC");
        // Oracle1: average of [110, 100] = 105
        // Oracle2: average of [210, 200] = 205
        // Oracle3: average of [310, 300] = 305
        // Median of [105, 205, 305] = 205
        assertEq(valueDefault, 205, "Default should use average methodology");

        // Test with custom windowSize=1, median methodology, custom timeout=2000, custom threshold=2
        (uint128 valueCustom,) = oracleMeta.getValueByConfig("BTC", 1, address(medianMethodology), 2000, 2);
        // Oracle1: median of [110] = 110
        // Oracle2: median of [210] = 210
        // Oracle3: median of [310] = 310
        // Median of [110, 210, 310] = 210
        assertEq(valueCustom, 210, "Custom should use median methodology with windowSize=1");
    }

    function testConstructorZeroAddress() public {
        vm.expectRevert(DIAOracleV3Meta.InvalidMethodology.selector);
        new DIAOracleV3Meta(address(0));
    }

    function testAddOracleZeroAddress() public {
        vm.startPrank(admin);
        vm.expectRevert(DIAOracleV3Meta.ZeroAddress.selector);
        oracleMeta.addOracle(address(0));
        vm.stopPrank();
    }

    function testAddOracleExists() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.expectRevert(DIAOracleV3Meta.OracleExists.selector);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();
    }

    function testRemoveOracle() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));
        assertEq(oracleMeta.getNumOracles(), 3);

        oracleMeta.removeOracle(address(oracle2));
        assertEq(oracleMeta.getNumOracles(), 2);

        assertEq(oracleMeta.oracles(0), address(oracle1));
        assertEq(oracleMeta.oracles(1), address(oracle3));
        vm.stopPrank();
    }

    function testRemoveOracleWithThresholdImbalance() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));

        // Set threshold equal to numOracles
        oracleMeta.setThreshold(3);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);

        // removeOracle should reject removal when threshold > new numOracles
        vm.expectRevert(abi.encodeWithSelector(
            DIAOracleV3Meta.InvalidThreshold.selector,
            3 // threshold (3) > new numOracles (2)
        ));
        oracleMeta.removeOracle(address(oracle3));

        // To remove, must first lower threshold
        oracleMeta.setThreshold(2);
        oracleMeta.removeOracle(address(oracle3));

        assertEq(oracleMeta.getNumOracles(), 2);
        assertEq(oracleMeta.getThreshold(), 2);

        vm.stopPrank();
    }

    function testRemoveOracleNotFound() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.expectRevert(DIAOracleV3Meta.OracleNotFound.selector);
        oracleMeta.removeOracle(address(oracle2));
        vm.stopPrank();
    }

    function testRemoveOracleLast() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));

        oracleMeta.removeOracle(address(oracle2));
        assertEq(oracleMeta.getNumOracles(), 1);
        assertEq(oracleMeta.oracles(0), address(oracle1));
        vm.stopPrank();
    }

    function testSetThresholdZero() public {
        vm.startPrank(admin);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidThreshold.selector, 0));
        oracleMeta.setThreshold(0);
        vm.stopPrank();
    }

    function testSetThreshold() public {
        vm.startPrank(admin);

        // Should reject threshold above numOracles (0 oracles)
        vm.expectRevert(abi.encodeWithSelector(
            DIAOracleV3Meta.InvalidThreshold.selector,
            3 // threshold (3) > numOracles (0)
        ));
        oracleMeta.setThreshold(3);

        // Add oracles first
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));

        // Now threshold 3 is valid
        oracleMeta.setThreshold(3);
        assertEq(oracleMeta.getThreshold(), 3);
        vm.stopPrank();
    }

    function testSetTimeoutSecondsZero() public {
        vm.startPrank(admin);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidTimeOut.selector, 0));
        oracleMeta.setTimeoutSeconds(0);
        vm.stopPrank();
    }

    function testSetTimeoutSecondsExceedsLimit() public {
        vm.startPrank(admin);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.TimeoutExceedsLimit.selector, 86401));
        oracleMeta.setTimeoutSeconds(86401);
        vm.stopPrank();
    }

    function testSetTimeoutSeconds() public {
        vm.startPrank(admin);
        oracleMeta.setTimeoutSeconds(3600);
        assertEq(oracleMeta.getTimeoutSeconds(), 3600);
        vm.stopPrank();
    }

    function testSetPriceMethodology() public {
        vm.startPrank(admin);
        MedianPriceMethodology newMethodology = new MedianPriceMethodology();
        address oldMethodology = address(oracleMeta.priceMethodology());

        oracleMeta.setPriceMethodology(address(newMethodology));
        assertEq(address(oracleMeta.priceMethodology()), address(newMethodology));
        vm.stopPrank();
    }

    function testSetPriceMethodologyZeroAddress() public {
        vm.startPrank(admin);
        vm.expectRevert(DIAOracleV3Meta.InvalidMethodology.selector);
        oracleMeta.setPriceMethodology(address(0));
        vm.stopPrank();
    }

    function testSetWindowSizeZero() public {
        vm.startPrank(admin);
        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidWindowSize.selector, 0));
        oracleMeta.setWindowSize(0);
        vm.stopPrank();
    }

    function testSetWindowSize() public {
        vm.startPrank(admin);
        oracleMeta.setWindowSize(20);
        assertEq(oracleMeta.getWindowSize(), 20);
        vm.stopPrank();
    }

    function testGetValueZeroTimeout() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setThreshold(1);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidTimeOut.selector, 0));
        oracleMeta.getValue("BTC");
    }

    function testGetValueZeroThreshold() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidThreshold.selector, 0));
        oracleMeta.getValue("BTC");
    }

    function testGetValueZeroWindowSize() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidWindowSize.selector, 0));
        oracleMeta.getValue("BTC");
    }

    function testGetValueByConfigZeroTimeout() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));
        AveragePriceMethodology methodology = new AveragePriceMethodology();

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidTimeOut.selector, 0));
        oracleMeta.getValueByConfig("BTC", 10, address(methodology), 0, 1);
    }

    function testGetValueByConfigZeroThreshold() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));
        AveragePriceMethodology methodology = new AveragePriceMethodology();

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidThreshold.selector, 0));
        oracleMeta.getValueByConfig("BTC", 10, address(methodology), 1000, 0);
    }

    function testGetValueByConfigZeroWindowSize() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));
        AveragePriceMethodology methodology = new AveragePriceMethodology();

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidWindowSize.selector, 0));
        oracleMeta.getValueByConfig("BTC", 0, address(methodology), 1000, 1);
    }

    function testGetValueByConfigZeroMethodology() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(DIAOracleV3Meta.InvalidMethodology.selector);
        oracleMeta.getValueByConfig("BTC", 10, address(0), 1000, 1);
    }

    // Tests for new volume and raw data functions

    function testGetAggregatedVolume() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setTimeoutSeconds(1000);
        vm.stopPrank();

        // Set values with volume using setRawValue
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(2000000), bytes(""));

        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        (uint128 totalVolume, uint256 validCount) = oracleMeta.getAggregatedVolume("BTC");

        assertEq(totalVolume, 3000000, "Total volume should be sum of both oracles");
        assertEq(validCount, 2, "Should have 2 valid oracles");
    }

    function testGetAggregatedVolumeWithExpiredData() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setTimeoutSeconds(100);
        vm.stopPrank();

        // Warp to a future time to avoid underflow
        vm.warp(1000);

        // Set one with current timestamp, one with old timestamp (expired)
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        bytes memory data2 =
            abi.encode("BTC", uint128(51000), uint128(block.timestamp - 200), uint128(2000000), bytes(""));

        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        (uint128 totalVolume, uint256 validCount) = oracleMeta.getAggregatedVolume("BTC");

        assertEq(totalVolume, 1000000, "Total volume should only include non-expired oracle");
        assertEq(validCount, 1, "Should have 1 valid oracle");
    }

    function testGetRawDataFromOracle() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        vm.stopPrank();

        bytes memory rawData1 = abi.encode("extra data 1");
        bytes memory rawData2 = abi.encode("extra data 2");

        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000), rawData1);
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(2000), rawData2);

        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        bytes memory retrievedData1 = oracleMeta.getRawDataFromOracle(0, "BTC");
        bytes memory retrievedData2 = oracleMeta.getRawDataFromOracle(1, "BTC");

        assertEq(keccak256(retrievedData1), keccak256(rawData1), "Raw data 1 should match");
        assertEq(keccak256(retrievedData2), keccak256(rawData2), "Raw data 2 should match");
    }

    function testGetRawDataFromOracleInvalidIndex() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidOracleIndex.selector, 5));
        oracleMeta.getRawDataFromOracle(5, "BTC");
    }

    function testGetAllRawData() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        vm.stopPrank();

        bytes memory rawData1 = abi.encode("data1");
        bytes memory rawData2 = abi.encode("data2");

        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000), rawData1);
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(2000), rawData2);

        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        bytes[] memory allRawData = oracleMeta.getAllRawData("BTC");

        assertEq(allRawData.length, 2, "Should have 2 raw data entries");
        assertEq(keccak256(allRawData[0]), keccak256(rawData1), "First raw data should match");
        assertEq(keccak256(allRawData[1]), keccak256(rawData2), "Second raw data should match");
    }

    function testGetValueWithVolumeFromOracle() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();

        bytes memory data = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        oracle1.setRawValue(data);

        (uint128 value, uint128 timestamp, uint128 volume) = oracleMeta.getValueWithVolumeFromOracle(0, "BTC", 0);

        assertEq(value, 50000, "Value should match");
        assertEq(timestamp, uint128(block.timestamp), "Timestamp should match");
        assertEq(volume, 1000000, "Volume should match");
    }

    function testGetValueWithVolumeFromOracleInvalidIndex() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        vm.stopPrank();

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidOracleIndex.selector, 5));
        oracleMeta.getValueWithVolumeFromOracle(5, "BTC", 0);
    }

    function testGetAllValuesWithVolume() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));
        vm.stopPrank();

        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(2000000), bytes(""));
        // oracle3 has no data for BTC

        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        (
            uint128[] memory values,
            uint128[] memory timestamps,
            uint128[] memory volumes,
            address[] memory oracleAddresses
        ) = oracleMeta.getAllValuesWithVolume("BTC");

        assertEq(values.length, 2, "Should have 2 values");
        assertEq(values[0], 50000, "First value should match");
        assertEq(values[1], 51000, "Second value should match");
        assertEq(volumes[0], 1000000, "First volume should match");
        assertEq(volumes[1], 2000000, "Second volume should match");
        assertEq(oracleAddresses[0], address(oracle1), "First oracle address should match");
        assertEq(oracleAddresses[1], address(oracle2), "Second oracle address should match");
    }

    function testGetValueWithVolume() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setThreshold(2);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        bytes memory data2 = abi.encode("BTC", uint128(52000), uint128(block.timestamp), uint128(2000000), bytes(""));

        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        (uint128 value, uint128 timestamp, uint128 totalVolume) = oracleMeta.getValueWithVolume("BTC");

        // Value should be aggregated using methodology (median of averages)
        assertGt(value, 0, "Value should be greater than 0");
        assertEq(totalVolume, 3000000, "Total volume should be sum of both");
    }

    function testGetValueWithVolumeZeroTimeout() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setThreshold(1);
        oracleMeta.setWindowSize(10);
        // timeoutSeconds is 0
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidTimeOut.selector, 0));
        oracleMeta.getValueWithVolume("BTC");
    }

    function testGetAggregatedVolumeOverflow() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setTimeoutSeconds(1000);
        vm.stopPrank();

        //  volume overflow
        uint128 maxUint128 = type(uint128).max;
        bytes memory data = abi.encode("BTC", uint128(50000), uint128(block.timestamp), maxUint128, bytes(""));
        oracle1.setRawValue(data);

        // overflow in getAggregatedVolume
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setDecimals(8); // Match meta oracle decimals
        vm.stopPrank();

        oracle2.setDecimals(8);
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(1), bytes(""));
        oracle2.setRawValue(data2);

        // getAggregatedVolume should revert with SafeCast overflow
        vm.expectRevert();
        oracleMeta.getAggregatedVolume("BTC");
    }

    function testGetValueWithVolumeZeroThreshold() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        // threshold is 0
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidThreshold.selector, 0));
        oracleMeta.getValueWithVolume("BTC");
    }

    function testGetValueWithVolumeZeroWindowSize() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setThreshold(1);
        // windowSize is 0
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidWindowSize.selector, 0));
        oracleMeta.getValueWithVolume("BTC");
    }

    function testGetValueWithVolumeWithExpiredData() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));
        oracleMeta.setThreshold(2);
        oracleMeta.setTimeoutSeconds(100);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        // Warp to a future time
        vm.warp(1000);

        // oracle1: current data with volume
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        // oracle2: current data with volume
        bytes memory data2 = abi.encode("BTC", uint128(52000), uint128(block.timestamp), uint128(500000), bytes(""));
        // oracle3: expired data with volume (timestamp 200 seconds ago, timeout is 100)
        bytes memory data3 = abi.encode("BTC", uint128(53000), uint128(block.timestamp - 200), uint128(2000000), bytes(""));

        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);
        oracle3.setRawValue(data3);

        (uint128 value,, uint128 totalVolume) = oracleMeta.getValueWithVolume("BTC");

        // oracle1 + oracle2 volume should be counted (oracle3 is expired)
        assertGt(value, 0, "Value should be greater than 0");
        assertEq(totalVolume, 1500000, "Total volume should only count non-expired oracles");
    }

    function testGetValueWithVolumeWithNoData() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        vm.stopPrank();

        // Only oracle1 has data, oracle2 has no data (tests continue branch)
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        oracle1.setRawValue(data1);
        // oracle2 has no data

        (uint128 value,, uint128 totalVolume) = oracleMeta.getValueWithVolume("BTC");

        assertGt(value, 0, "Value should be greater than 0");
        assertEq(totalVolume, 1000000, "Total volume should only count oracle with data");
    }

    function testGetAggregatedVolumeWithNoData() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setTimeoutSeconds(1000);
        vm.stopPrank();

        // Only oracle1 has data, oracle2 has no data (tests continue branch)
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        oracle1.setRawValue(data1);
        // oracle2 has no data

        (uint128 totalVolume, uint256 validCount) = oracleMeta.getAggregatedVolume("BTC");

        assertEq(totalVolume, 1000000, "Total volume should only count oracle with data");
        assertEq(validCount, 1, "Should have 1 valid oracle");
    }

    function testGetAggregatedVolumeZeroTimeout() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        // timeoutSeconds is 0
        vm.stopPrank();

        oracle1.setValue("BTC", 100, uint128(block.timestamp));

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidTimeOut.selector, 0));
        oracleMeta.getAggregatedVolume("BTC");
    }

    function testGetAggregatedVolumeSkipsZeroVolume() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));
        oracleMeta.setTimeoutSeconds(1000);
        vm.stopPrank();

        // Oracle1: value with volume 1000000
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000000), bytes(""));
        oracle1.setRawValue(data1);

        // Oracle2: value with ZERO volume (should be skipped)
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(0), bytes(""));
        oracle2.setRawValue(data2);

        // Oracle3: value with volume 2000000
        bytes memory data3 = abi.encode("BTC", uint128(52000), uint128(block.timestamp), uint128(2000000), bytes(""));
        oracle3.setRawValue(data3);

        (uint128 totalVolume, uint256 validCount) = oracleMeta.getAggregatedVolume("BTC");

        // Should only count oracle1 and oracle3 (skip oracle2 with zero volume)
        assertEq(totalVolume, 3000000, "Total volume should skip zero-volume oracle");
        assertEq(validCount, 2, "Should have 2 valid oracles (zero-volume skipped)");
    }

    // =================================================================
    // ERC-165 Interface Validation Tests
    // =================================================================

    /// @notice Test that adding a valid DIAOracleV3 contract succeeds
    function testAddOracleWithValidInterface() public {
        vm.startPrank(admin);

        // DIAOracleV3 implements ERC-165 and IDIAOracleV3
        oracleMeta.addOracle(address(oracle1));

        assertEq(oracleMeta.getNumOracles(), 1);
        vm.stopPrank();
    }

    /// @notice Test that adding an EOA (regular address) reverts
    function testAddOracleEOAFails() public {
        vm.startPrank(admin);

        // Create a random address (EOA)
        address randomEOA = address(0x456);

        // EOAs don't implement supportsInterface
        // The EOA call will revert without error data, then our catch block will revert with InvalidOracle
        vm.expectRevert();
        oracleMeta.addOracle(randomEOA);

        vm.stopPrank();
    }

    /// @notice Test that adding a contract without ERC-165 support reverts
    function testAddOracleNoERC165Support() public {
        vm.startPrank(admin);

        // Deploy a contract that doesn't implement ERC-165
        InvalidOracleContract invalidContract = new InvalidOracleContract();

        // Contract exists but doesn't have supportsInterface, will revert
        vm.expectRevert();
        oracleMeta.addOracle(address(invalidContract));

        vm.stopPrank();
    }

    /// @notice Test that adding a contract with wrong interface reverts
    function testAddOracleWrongInterface() public {
        vm.startPrank(admin);

        // Deploy a contract that implements ERC-165 but not IDIAOracleV3
        WrongInterfaceContract wrongInterface = new WrongInterfaceContract();

        vm.expectRevert(abi.encodeWithSelector(DIAOracleV3Meta.InvalidOracle.selector, address(wrongInterface)));
        oracleMeta.addOracle(address(wrongInterface));

        vm.stopPrank();
    }

    /// @notice Test that MockDIAOracleV3 with ERC-165 works
    function testAddOracleMockWithERC165() public {
        vm.startPrank(admin);

        // MockDIAOracleV3 properly implements ERC-165 and IDIAOracleV3
        MockDIAOracleV3 mockOracle = new MockDIAOracleV3(10);
        mockOracle.setDecimals(8); // Set to 8 to match oracleMeta's default

        oracleMeta.addOracle(address(mockOracle));

        assertEq(oracleMeta.getNumOracles(), 1);

        // Verify it actually works by setting and getting a value
        mockOracle.setValue("ETH", 1000, uint128(block.timestamp));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);

        (uint128 value,) = oracleMeta.getValue("ETH");
        assertEq(value, 1000);

        vm.stopPrank();
    }

    // =================================================================
    // Decimals Functionality Tests
    // =================================================================

    function testSetDecimals() public {
        vm.startPrank(admin);
        uint8 decimalPrecision = 8;

        oracleMeta.setDecimals(decimalPrecision);

        assertEq(oracleMeta.getDecimals(), decimalPrecision, "Decimals should match");
        vm.stopPrank();
    }

    function testSetDecimalsMultipleTimes() public {
        vm.startPrank(admin);
        oracleMeta.setDecimals(8);
        assertEq(oracleMeta.getDecimals(), 8, "First decimals should be 8");

        oracleMeta.setDecimals(18);
        assertEq(oracleMeta.getDecimals(), 18, "Decimals should be updated to 18");

        oracleMeta.setDecimals(6);
        assertEq(oracleMeta.getDecimals(), 6, "Decimals should be updated to 6");
        vm.stopPrank();
    }

    function testDecimalsEvent() public {
        vm.startPrank(admin);
        vm.expectEmit(true, true, true, true);
        emit DIAOracleV3Meta.DecimalsUpdate(8);

        oracleMeta.setDecimals(8);
        vm.stopPrank();
    }

    function testDecimalsDefaultZero() public {
        // Decimals should default to 8
        assertEq(oracleMeta.getDecimals(), 8, "Default decimals should be 8");
    }

    function testSetDecimalsOnlyOwner() public {
        // Try to set decimals from non-owner address
        vm.prank(address(0x456));
        vm.expectRevert();
        oracleMeta.setDecimals(8);
    }

    function testDecimalsEdgeCaseMax() public {
        vm.startPrank(admin);
        uint8 maxDecimals = 255; // uint8 max value

        oracleMeta.setDecimals(maxDecimals);
        assertEq(oracleMeta.getDecimals(), maxDecimals, "Should handle max decimals");
        vm.stopPrank();
    }

    function testDecimalsEdgeCaseZero() public {
        vm.startPrank(admin);
        oracleMeta.setDecimals(0);
        assertEq(oracleMeta.getDecimals(), 0, "Should handle zero decimals");
        vm.stopPrank();
    }

    // =================================================================
    // Decimal Filtering Tests
    // =================================================================

    function testGetValueFiltersOraclesByDecimals() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        oracleMeta.setDecimals(8); // Meta oracle expects 8 decimals

        vm.stopPrank();

        // Set same decimals for both oracles
        oracle1.setDecimals(8);
        oracle2.setDecimals(8);

        // Set values in both oracles
        oracle1.setValue("BTC", 50000, uint128(block.timestamp));
        oracle2.setValue("BTC", 51000, uint128(block.timestamp));

        // Both should be included (same decimals)
        (uint128 value,) = oracleMeta.getValue("BTC");
        assertEq(value, 50500, "Should average both oracle values"); // (50000 + 51000) / 2
    }

    function testGetValueIgnoresOraclesWithDifferentDecimals() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        oracleMeta.setDecimals(8); // Meta oracle expects 8 decimals

        vm.stopPrank();

        // Set different decimals for oracles
        oracle1.setDecimals(8);  // Matches meta
        oracle2.setDecimals(18); // Different decimals

        // Set values in both oracles
        oracle1.setValue("BTC", 50000, uint128(block.timestamp));
        oracle2.setValue("BTC", 51000, uint128(block.timestamp));

        // Only oracle1 should be included (matching decimals)
        (uint128 value,) = oracleMeta.getValue("BTC");
        assertEq(value, 50000, "Should only use oracle1 value (oracle2 ignored due to decimals mismatch)");
    }

    function testGetValueWithNoMatchingDecimalsOracles() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        oracleMeta.setDecimals(8); // Meta oracle expects 8 decimals

        vm.stopPrank();

        // Set different decimals for oracle
        oracle1.setDecimals(18); // Different decimals

        // Set value
        oracle1.setValue("BTC", 50000, uint128(block.timestamp));

        // Should fail because no oracles match
        vm.expectRevert(abi.encodeWithSelector(AveragePriceMethodology.ThresholdNotMet.selector, 0, 1));
        oracleMeta.getValue("BTC");
    }

    function testGetAggregatedVolumeFiltersByDecimals() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setDecimals(8); // Meta oracle expects 8 decimals

        vm.stopPrank();

        // Set different decimals for oracles
        oracle1.setDecimals(8);  // Matches meta
        oracle2.setDecimals(18); // Different decimals

        // Set values with volume in both oracles
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(2000), bytes(""));
        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        // Only oracle1 volume should be included
        (uint128 totalVolume, uint256 validCount) = oracleMeta.getAggregatedVolume("BTC");
        assertEq(totalVolume, 1000, "Should only include oracle1 volume");
        assertEq(validCount, 1, "Should have 1 valid oracle");
    }

    function testGetValueWithVolumeFiltersByDecimals() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        oracleMeta.setDecimals(8); // Meta oracle expects 8 decimals

        vm.stopPrank();

        // Set different decimals for oracles
        oracle1.setDecimals(8);  // Matches meta
        oracle2.setDecimals(18); // Different decimals

        // Set values with volume in both oracles
        bytes memory data1 = abi.encode("BTC", uint128(50000), uint128(block.timestamp), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode("BTC", uint128(51000), uint128(block.timestamp), uint128(2000), bytes(""));
        oracle1.setRawValue(data1);
        oracle2.setRawValue(data2);

        // Value should only use oracle1, volume should only include oracle1
        (uint128 value,, uint128 totalVolume) = oracleMeta.getValueWithVolume("BTC");
        assertEq(value, 50000, "Should only use oracle1 value");
        assertEq(totalVolume, 1000, "Should only include oracle1 volume");
    }

    function testGetValueByConfigFiltersByDecimals() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.setDecimals(8); // Meta oracle expects 8 decimals

        vm.stopPrank();

        // Set different decimals for oracles
        oracle1.setDecimals(8);  // Matches meta
        oracle2.setDecimals(18); // Different decimals

        // Set values in both oracles
        oracle1.setValue("BTC", 50000, uint128(block.timestamp));
        oracle2.setValue("BTC", 51000, uint128(block.timestamp));

        // Only oracle1 should be included (matching decimals)
        (uint128 value,) = oracleMeta.getValueByConfig("BTC", 10, address(oracleMeta.priceMethodology()), 1000, 1);
        assertEq(value, 50000, "Should only use oracle1 value (oracle2 ignored due to decimals mismatch)");
    }

    function testMultipleOraclesMixedDecimals() public {
        vm.startPrank(admin);
        oracleMeta.addOracle(address(oracle1));
        oracleMeta.addOracle(address(oracle2));
        oracleMeta.addOracle(address(oracle3));
        oracleMeta.setThreshold(1);
        oracleMeta.setTimeoutSeconds(1000);
        oracleMeta.setWindowSize(10);
        oracleMeta.setDecimals(8); // Meta oracle expects 8 decimals

        vm.stopPrank();

        // Set mixed decimals for oracles
        oracle1.setDecimals(8);  // Matches meta
        oracle2.setDecimals(18); // Different decimals
        oracle3.setDecimals(8);  // Matches meta

        // Set values in all oracles
        oracle1.setValue("BTC", 50000, uint128(block.timestamp));
        oracle2.setValue("BTC", 55000, uint128(block.timestamp)); // Should be ignored
        oracle3.setValue("BTC", 52000, uint128(block.timestamp));

        // Only oracle1 and oracle3 should be included (matching decimals)
        (uint128 value,) = oracleMeta.getValue("BTC");
        assertEq(value, 51000, "Should average oracle1 and oracle3 values"); // (50000 + 52000) / 2
    }
}

// =================================================================
// Helper contracts for testing ERC-165 validation
// =================================================================

/// @notice Contract that doesn't implement ERC-165 at all
contract InvalidOracleContract {
    function setValue(string memory, uint128, uint128) external {}

    function getValue(string memory) external pure returns (uint128, uint128) {
        return (0, 0);
    }

    // Missing: supportsInterface() - No ERC-165 support!
}

/// @notice Contract that implements ERC-165 but not IDIAOracleV3
contract WrongInterfaceContract is ERC165 {
    // No override needed, just inherit from ERC165

    }
