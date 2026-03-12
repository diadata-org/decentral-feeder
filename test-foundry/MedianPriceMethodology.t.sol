// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "../contracts/DIAOracleV3.sol";
import "../contracts/IDIAOracleV3.sol";
import "../contracts/methodologies/MedianPriceMethodology.sol";
import "forge-std/console.sol";

contract MedianPriceMethodologyTest is Test {
    MedianPriceMethodology methodology;
    DIAOracleV3 oracle1;
    DIAOracleV3 oracle2;
    DIAOracleV3 oracle3;

    function setUp() public {
        // Deploy methodology
        methodology = new MedianPriceMethodology();

        // Deploy oracle 1
        DIAOracleV3 impl1 = new DIAOracleV3();
        ERC1967Proxy proxy1 = new ERC1967Proxy(address(impl1), "");
        oracle1 = DIAOracleV3(address(proxy1));
        oracle1.initialize();

        // Deploy oracle 2
        DIAOracleV3 impl2 = new DIAOracleV3();
        ERC1967Proxy proxy2 = new ERC1967Proxy(address(impl2), "");
        oracle2 = DIAOracleV3(address(proxy2));
        oracle2.initialize();

        // Deploy oracle 3
        DIAOracleV3 impl3 = new DIAOracleV3();
        ERC1967Proxy proxy3 = new ERC1967Proxy(address(impl3), "");
        oracle3 = DIAOracleV3(address(proxy3));
        oracle3.initialize();

        // Set block timestamp
        vm.warp(1710000000);
    }

    function testMedianSingleOracleSingleValue() public {
        string memory key = "BTC/USD";

        oracle1.setValue(key, 50000, 1710000000);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, uint128 timestamp) = methodology.calculateValue(
            key,
            oracles,
            3600, // 1 hour timeout
            1,    // threshold
            10    // window size
        );

        assertEq(value, 50000, "Median should be 50000");
        assertEq(timestamp, 1710000000, "Timestamp should match");
    }

    function testMedianSingleOracleMultipleValuesOdd() public {
        string memory key = "ETH/USD";

        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 200, 1710000001);
        oracle1.setValue(key, 300, 1710000002);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Median of [100, 200, 300] is 200
        assertEq(value, 200, "Median of odd count should be middle value");
    }

    function testMedianSingleOracleMultipleValuesEven() public {
        string memory key = "SOL/USD";

        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 200, 1710000001);
        oracle1.setValue(key, 300, 1710000002);
        oracle1.setValue(key, 400, 1710000003);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Median of [100, 200, 300, 400] = (200 + 300) / 2 = 250
        assertEq(value, 250, "Median of even count should be average of middle values");
    }

    function testMedianMultipleOraclesOdd() public {
        string memory key = "AVAX/USD";

        // Oracle 1: median of [100, 150, 200] = 150
        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 150, 1710000001);
        oracle1.setValue(key, 200, 1710000002);

        // Oracle 2: median of [250, 300, 350] = 300
        oracle2.setValue(key, 250, 1710000000);
        oracle2.setValue(key, 300, 1710000001);
        oracle2.setValue(key, 350, 1710000002);

        // Oracle 3: median of [175, 225, 275] = 225
        oracle3.setValue(key, 175, 1710000000);
        oracle3.setValue(key, 225, 1710000001);
        oracle3.setValue(key, 275, 1710000002);

        address[] memory oracles = new address[](3);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);
        oracles[2] = address(oracle3);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Median of [150, 225, 300] = 225
        assertEq(value, 225, "Median of medians should be correct");
    }

    function testMedianMultipleOraclesEven() public {
        string memory key = "LINK/USD";

        // Oracle 1: median = 100
        oracle1.setValue(key, 50, 1710000000);
        oracle1.setValue(key, 100, 1710000001);
        oracle1.setValue(key, 150, 1710000002);

        // Oracle 2: median = 200
        oracle2.setValue(key, 150, 1710000000);
        oracle2.setValue(key, 200, 1710000001);
        oracle2.setValue(key, 250, 1710000002);

        // Oracle 3: median = 300
        oracle3.setValue(key, 250, 1710000000);
        oracle3.setValue(key, 300, 1710000001);
        oracle3.setValue(key, 350, 1710000002);

        address[] memory oracles = new address[](3);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);
        oracles[2] = address(oracle3);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Median of [100, 200, 300] = 200
        assertEq(value, 200, "Median of odd number of oracles should be middle");
    }

    function testMedianWindowSizeLimitsValues() public {
        string memory key = "UNI/USD";

        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 200, 1710000001);
        oracle1.setValue(key, 300, 1710000002);
        oracle1.setValue(key, 400, 1710000003);
        oracle1.setValue(key, 500, 1710000004);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            3  // Only use 3 most recent values
        );

        // Median of [300, 400, 500] is 400
        assertEq(value, 400, "Should only use windowSize values");
    }

    function testMedianTimeoutFiltersExpiredValues() public {
        string memory key = "COMP/USD";

        // Set first value
        vm.warp(1710000000);
        oracle1.setValue(key, 100, 1710000000);

        // Warp forward and set second value
        vm.warp(1710000300); // 5 minutes later
        oracle1.setValue(key, 200, 1710000300);

        // Warp forward and set third value
        vm.warp(1710000600); // 5 minutes later
        oracle1.setValue(key, 300, 1710000600);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        // Warp forward so first value is outside timeout window
        vm.warp(1710001200); // 20 minutes after first value

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            900, // 15 minute timeout - first value should be filtered
            1,
            10
        );

        // Median of [200, 300] = 250
        assertEq(value, 250, "Should filter out expired values");
    }

    function testMedianThresholdNotMet() public {
        string memory key = "AAVE/USD";

        oracle1.setValue(key, 100, 1710000000);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        // Require 2 oracles but only 1 is available
        vm.expectRevert(
            abi.encodeWithSelector(
                MedianPriceMethodology.ThresholdNotMet.selector,
                1,
                2
            )
        );
        methodology.calculateValue(
            key,
            oracles,
            3600,
            2,  // threshold = 2
            10
        );
    }

    function testMedianEmptyOracleArray() public {
        string memory key = "MKR/USD";

        address[] memory oracles = new address[](0);

        vm.expectRevert(
            abi.encodeWithSelector(
                MedianPriceMethodology.ThresholdNotMet.selector,
                0,
                1
            )
        );
        methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );
    }

    function testMedianReturnsTimestampOfMedian() public {
        string memory key = "YFI/USD";

        // Oracle 1: values with different timestamps (monotonically increasing)
        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 300, 1710000002);
        oracle1.setValue(key, 200, 1710000005); // Latest timestamp, will be median

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, uint128 timestamp) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Sorted values: [100, 200, 300], median is 200 with timestamp 1710000005
        assertEq(value, 200, "Value should be median");
        assertEq(timestamp, 1710000005, "Should return timestamp of median value");
    }

    function testMedianComplexEvenCount() public {
        string memory key = "CRV/USD";

        // Oracle 1: [100, 150, 200, 250] -> median = (150 + 200) / 2 = 175
        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 150, 1710000001);
        oracle1.setValue(key, 200, 1710000002);
        oracle1.setValue(key, 250, 1710000003);

        // Oracle 2: [175, 225, 275, 325] -> median = (225 + 275) / 2 = 250
        oracle2.setValue(key, 175, 1710000000);
        oracle2.setValue(key, 225, 1710000001);
        oracle2.setValue(key, 275, 1710000002);
        oracle2.setValue(key, 325, 1710000003);

        address[] memory oracles = new address[](2);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);

        (uint128 value, uint128 timestamp) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Final median of [175, 250] = (175 + 250) / 2 = 212
        assertEq(value, 212, "Complex even count median should be correct");
    }

    function testMedianWithMixedExpiredAndValidValues() public {
        string memory key = "SNX/USD";

        // Set first value
        vm.warp(1710000000);
        oracle1.setValue(key, 100, 1710000000);

        // Set second value
        vm.warp(1710000120);
        oracle1.setValue(key, 200, 1710000120);

        // Set third value
        vm.warp(1710000240);
        oracle1.setValue(key, 300, 1710000240);

        // Set fourth value
        vm.warp(1710000360);
        oracle1.setValue(key, 400, 1710000360);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        // Warp forward to make first value expire
        vm.warp(1710001000); // First value is 1000s old (expired), others are valid

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            900, // 15 minute timeout - first value should be expired
            1,
            10
        );

        // Valid values: [200, 300, 400] -> median = 300
        assertEq(value, 300, "Should only use non-expired values");
    }

    function testMedianZeroPrice() public {
        string memory key = "ZERO/USD";

        oracle1.setValue(key, 0, 1710000000);
        oracle1.setValue(key, 100, 1710000001);
        oracle1.setValue(key, 200, 1710000002);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Median of [0, 100, 200] = 100
        assertEq(value, 100, "Zero price should be included in calculation");
    }

    function testMedianOracleWithNoHistory() public {
        string memory key = "EMPTY/USD";

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1); // No data set

        vm.expectRevert(
            abi.encodeWithSelector(
                MedianPriceMethodology.ThresholdNotMet.selector,
                0,
                1
            )
        );
        methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );
    }

    function testMedianMultipleKeysIndependent() public {
        string memory key1 = "BTC/USD";
        string memory key2 = "ETH/USD";

        // Set different prices for different keys
        oracle1.setValue(key1, 50000, 1710000000);
        oracle1.setValue(key1, 51000, 1710000001);
        oracle1.setValue(key1, 52000, 1710000002);

        oracle1.setValue(key2, 3000, 1710000000);
        oracle1.setValue(key2, 3100, 1710000001);
        oracle1.setValue(key2, 3200, 1710000002);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value1, ) = methodology.calculateValue(key1, oracles, 3600, 1, 10);
        (uint128 value2, ) = methodology.calculateValue(key2, oracles, 3600, 1, 10);

        assertEq(value1, 51000, "BTC median should be 51000");
        assertEq(value2, 3100, "ETH median should be 3100");
    }

    function testMedianLargeValues() public {
        string memory key = "LARGE/USD";

        uint128 largePrice1 = 100000000000; // 100 billion
        uint128 largePrice2 = 200000000000; // 200 billion
        uint128 largePrice3 = 300000000000; // 300 billion

        oracle1.setValue(key, largePrice1, 1710000000);
        oracle1.setValue(key, largePrice2, 1710000001);
        oracle1.setValue(key, largePrice3, 1710000002);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(key, oracles, 3600, 1, 10);

        assertEq(value, largePrice2, "Should handle large values");
    }

    function testMedianWindowSizeLargerThanHistory() public {
        string memory key = "WINDOW/USD";

        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 200, 1710000001);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        // Window size larger than actual history
        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            100  // Much larger than actual history
        );

        // Should use all available values
        assertEq(value, 150, "Should use all available values when windowSize > history");
    }

    function testMedianSingleValuePerOracle() public {
        string memory key = "SINGLE/USD";

        oracle1.setValue(key, 100, 1710000000);
        oracle2.setValue(key, 200, 1710000001);
        oracle3.setValue(key, 300, 1710000002);

        address[] memory oracles = new address[](3);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);
        oracles[2] = address(oracle3);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Median of [100, 200, 300] = 200
        assertEq(value, 200, "Should handle single value per oracle");
    }

    function testMedianTwoOraclesEvenCount() public {
        string memory key = "TWO/USD";

        // Oracle 1: [100, 200, 300] -> median = 200
        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 200, 1710000001);
        oracle1.setValue(key, 300, 1710000002);

        // Oracle 2: [400, 500, 600] -> median = 500
        oracle2.setValue(key, 400, 1710000000);
        oracle2.setValue(key, 500, 1710000001);
        oracle2.setValue(key, 600, 1710000002);

        address[] memory oracles = new address[](2);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);

        (uint128 value, uint128 timestamp) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Final median of [200, 500] = (200 + 500) / 2 = 350
        assertEq(value, 350, "Even count of oracles should average middle values");
    }

    function testMedianUnsortedValues() public {
        string memory key = "UNSORTED/USD";

        // Add values in non-sorted order (monotonically increasing timestamps)
        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 300, 1710000001);
        oracle1.setValue(key, 500, 1710000002);
        oracle1.setValue(key, 400, 1710000003);
        oracle1.setValue(key, 200, 1710000004);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Sorted: [100, 200, 300, 400, 500] -> median = 300
        assertEq(value, 300, "Should handle unsorted values correctly");
    }

    function testMedianAllSameValues() public {
        string memory key = "SAME/USD";

        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 100, 1710000001);
        oracle1.setValue(key, 100, 1710000002);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        assertEq(value, 100, "Should handle all same values");
    }

    function testMedianWithVolumeIgnored() public {
        string memory key = "WITHVOL/USD";

        // Set values with volume - methodology should ignore volume
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(5000), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000001), uint128(1000), bytes(""));
        bytes memory data3 = abi.encode(key, uint128(300), uint128(1710000002), uint128(2000), bytes(""));

        oracle1.setRawValue(data1);
        oracle1.setRawValue(data2);
        oracle1.setRawValue(data3);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Volume should be ignored, just pure median
        assertEq(value, 200, "Should ignore volume in calculation");
    }

    function testMedianTimestampFromEvenCount() public {
        string memory key = "TIME/USD";

        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 300, 1710000002);
        oracle1.setValue(key, 400, 1710000003);
        oracle1.setValue(key, 200, 1710000005);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, uint128 timestamp) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Sorted: [100, 200, 300, 400]
        // Median = (200 + 300) / 2 = 250
        // Timestamp should be max of 200 and 300 = max(1710000005, 1710000002) = 1710000005
        assertEq(value, 250, "Value should be correct");
        assertEq(timestamp, 1710000005, "Should return max timestamp of median values");
    }

    function testMedianFiveOracles() public {
        string memory key = "FIVE/USD";

        // Oracle 1 values: [100, 400]
        oracle1.setValue(key, 100, 1710000000);
        oracle1.setValue(key, 400, 1710000001);

        // Oracle 2 values: [200, 500]
        oracle2.setValue(key, 200, 1710000000);
        oracle2.setValue(key, 500, 1710000001);

        // Oracle 3 values: [300]
        oracle3.setValue(key, 300, 1710000000);

        address[] memory oracles = new address[](3);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);
        oracles[2] = address(oracle3);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Oracle medians:
        // - Oracle1: [100, 400] -> median = 250
        // - Oracle2: [200, 500] -> median = 350
        // - Oracle3: [300] -> median = 300
        // Final median of [250, 300, 350] = 300
        assertEq(value, 300, "Should handle multiple oracles correctly");
    }
}
