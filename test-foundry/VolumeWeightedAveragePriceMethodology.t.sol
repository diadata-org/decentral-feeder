// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "../contracts/DIAOracleV3.sol";
import "../contracts/IDIAOracleV3.sol";
import "../contracts/methodologies/VolumeWeightedAveragePriceMethodology.sol";
import "forge-std/console.sol";

contract VolumeWeightedAveragePriceMethodologyTest is Test {
    VolumeWeightedAveragePriceMethodology methodology;
    DIAOracleV3 oracle1;
    DIAOracleV3 oracle2;
    DIAOracleV3 oracle3;

    function setUp() public {
        // Deploy methodology
        methodology = new VolumeWeightedAveragePriceMethodology();

        // Deploy oracle 1
        DIAOracleV3 impl1 = new DIAOracleV3();
        ERC1967Proxy proxy1 = new ERC1967Proxy(address(impl1), "");
        oracle1 = DIAOracleV3(address(proxy1));
        oracle1.initialize(18);

        // Deploy oracle 2
        DIAOracleV3 impl2 = new DIAOracleV3();
        ERC1967Proxy proxy2 = new ERC1967Proxy(address(impl2), "");
        oracle2 = DIAOracleV3(address(proxy2));
        oracle2.initialize(18);

        // Deploy oracle 3
        DIAOracleV3 impl3 = new DIAOracleV3();
        ERC1967Proxy proxy3 = new ERC1967Proxy(address(impl3), "");
        oracle3 = DIAOracleV3(address(proxy3));
        oracle3.initialize(18);

        // Set block timestamp
        vm.warp(1710000000);
    }

    function testVWAPSingleOracleSingleValue() public {
        string memory key = "BTC/USD";

        // Set a single value with volume
        bytes memory data = abi.encode(key, uint128(50000), uint128(1710000000), uint128(1000000), bytes(""));
        oracle1.setRawValue(data);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, uint128 timestamp) = methodology.calculateValue(
            key,
            oracles,
            3600, // 1 hour timeout
            1,    // threshold
            10    // window size
        );

        assertEq(value, 50000, "VWAP should be 50000");
        assertEq(timestamp, 1710000000, "Timestamp should match");
    }

    function testVWAPSingleOracleMultipleValues() public {
        string memory key = "ETH/USD";

        // Set multiple values with different volumes
        // VWAP = (100*1000 + 200*2000 + 300*3000) / (1000 + 2000 + 3000)
        //      = (100000 + 400000 + 900000) / 6000
        //      = 1400000 / 6000 = 233.33
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000001), uint128(2000), bytes(""));
        bytes memory data3 = abi.encode(key, uint128(300), uint128(1710000002), uint128(3000), bytes(""));

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

        // Expected: 233
        assertEq(value, 233, "VWAP should be 233");
    }

    function testVWAPMultipleOraclesMedian() public {
        string memory key = "SOL/USD";

        // Oracle 1: VWAP = (100*1000 + 200*1000) / 2000 = 150
        bytes memory data1a = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data1b = abi.encode(key, uint128(200), uint128(1710000001), uint128(1000), bytes(""));
        oracle1.setRawValue(data1a);
        oracle1.setRawValue(data1b);

        // Oracle 2: VWAP = (300*1000 + 400*1000) / 2000 = 350
        bytes memory data2a = abi.encode(key, uint128(300), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data2b = abi.encode(key, uint128(400), uint128(1710000001), uint128(1000), bytes(""));
        oracle2.setRawValue(data2a);
        oracle2.setRawValue(data2b);

        // Oracle 3: VWAP = (200*1000 + 250*1000) / 2000 = 225
        bytes memory data3a = abi.encode(key, uint128(200), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data3b = abi.encode(key, uint128(250), uint128(1710000001), uint128(1000), bytes(""));
        oracle3.setRawValue(data3a);
        oracle3.setRawValue(data3b);

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

        // Median of [150, 225, 350] is 225
        assertEq(value, 225, "Median VWAP should be 225");
    }

    function testVWAPMedianEvenNumberOfOracles() public {
        string memory key = "BTC/USD";

        // Oracle 1: VWAP = 100
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        oracle1.setRawValue(data1);

        // Oracle 2: VWAP = 200
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000000), uint128(1000), bytes(""));
        oracle2.setRawValue(data2);

        // Oracle 3: VWAP = 300
        bytes memory data3 = abi.encode(key, uint128(300), uint128(1710000000), uint128(1000), bytes(""));
        oracle3.setRawValue(data3);

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

        // Median of [100, 200, 300] is 200
        assertEq(value, 200, "Median of odd count should be middle value");
    }

    function testVWAPWindowSizeLimitsValues() public {
        string memory key = "AVAX/USD";

        // Add 5 values, but only use windowSize of 3
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000001), uint128(1000), bytes(""));
        bytes memory data3 = abi.encode(key, uint128(300), uint128(1710000002), uint128(1000), bytes(""));
        bytes memory data4 = abi.encode(key, uint128(400), uint128(1710000003), uint128(1000), bytes(""));
        bytes memory data5 = abi.encode(key, uint128(500), uint128(1710000004), uint128(1000), bytes(""));

        oracle1.setRawValue(data1);
        oracle1.setRawValue(data2);
        oracle1.setRawValue(data3);
        oracle1.setRawValue(data4);
        oracle1.setRawValue(data5);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            3  // Only use 3 most recent values
        );

        // VWAP of 3 most recent: (300 + 400 + 500) / 3 = 400
        assertEq(value, 400, "Should only use windowSize values");
    }

    function testVWAPTimeoutFiltersExpiredValues() public {
        string memory key = "LINK/USD";

        // Set first value at time T
        vm.warp(1710000000);
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        oracle1.setRawValue(data1);

        // Warp forward and set second value
        vm.warp(1710000300); // 5 minutes later
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000300), uint128(2000), bytes(""));
        oracle1.setRawValue(data2);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        // Warp forward more so first value is outside timeout window
        vm.warp(1710000900); // 900 seconds (15 minutes) after first value

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            600, // 10 minute timeout - first value should be filtered
            1,
            10
        );

        // Only the second value should be used (it's only 600 seconds old)
        assertEq(value, 200, "Should filter out expired values");
    }

    function testVWAPZeroVolumeIsSkipped() public {
        string memory key = "UNI/USD";

        // Value with zero volume should be skipped
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(0), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000001), uint128(1000), bytes(""));
        oracle1.setRawValue(data1);
        oracle1.setRawValue(data2);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Only the value with non-zero volume should be used
        assertEq(value, 200, "Should skip zero volume values");
    }

    function testVWAPAllZeroVolumeReturnsInvalid() public {
        string memory key = "AAVE/USD";

        // All values have zero volume
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(0), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000001), uint128(0), bytes(""));
        oracle1.setRawValue(data1);
        oracle1.setRawValue(data2);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        vm.expectRevert(
            abi.encodeWithSelector(
                VolumeWeightedAveragePriceMethodology.ThresholdNotMet.selector,
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

    function testVWAPThresholdNotMet() public {
        string memory key = "COMP/USD";

        bytes memory data = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        oracle1.setRawValue(data);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        // Require 2 oracles but only 1 is available
        vm.expectRevert(
            abi.encodeWithSelector(
                VolumeWeightedAveragePriceMethodology.ThresholdNotMet.selector,
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

    function testVWAPEmptyOracleArray() public {
        string memory key = "MKR/USD";

        address[] memory oracles = new address[](0);

        vm.expectRevert(
            abi.encodeWithSelector(
                VolumeWeightedAveragePriceMethodology.ThresholdNotMet.selector,
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

    function testVWAPReturnsMaxTimestamp() public {
        string memory key = "YFI/USD";

        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000005), uint128(1000), bytes(""));
        oracle1.setRawValue(data1);
        oracle1.setRawValue(data2);

        bytes memory data3 = abi.encode(key, uint128(300), uint128(1710000003), uint128(1000), bytes(""));
        bytes memory data4 = abi.encode(key, uint128(400), uint128(1710000007), uint128(1000), bytes(""));
        oracle2.setRawValue(data3);
        oracle2.setRawValue(data4);

        address[] memory oracles = new address[](2);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);

        (, uint128 timestamp) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Should return max timestamp (1710000007)
        assertEq(timestamp, 1710000007, "Should return max timestamp");
    }

    function testVWAPComplexCalculation() public {
        string memory key = "CRV/USD";

        // Oracle 1: Complex VWAP
        // (100*100 + 150*200 + 200*300) / 600 = 100000 / 600 = 166
        bytes memory data1a = abi.encode(key, uint128(100), uint128(1710000000), uint128(100), bytes(""));
        bytes memory data1b = abi.encode(key, uint128(150), uint128(1710000001), uint128(200), bytes(""));
        bytes memory data1c = abi.encode(key, uint128(200), uint128(1710000002), uint128(300), bytes(""));
        oracle1.setRawValue(data1a);
        oracle1.setRawValue(data1b);
        oracle1.setRawValue(data1c);

        // Oracle 2: Different VWAP
        // (250*400 + 300*500) / 900 = 250000 / 900 = 277
        bytes memory data2a = abi.encode(key, uint128(250), uint128(1710000000), uint128(400), bytes(""));
        bytes memory data2b = abi.encode(key, uint128(300), uint128(1710000001), uint128(500), bytes(""));
        oracle2.setRawValue(data2a);
        oracle2.setRawValue(data2b);

        address[] memory oracles = new address[](2);
        oracles[0] = address(oracle1);
        oracles[1] = address(oracle2);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            2,
            10
        );

        // Median of [166, 277] = (166 + 277 + 1) / 2 = 222
        assertEq(value, 222, "Complex VWAP calculation should be correct");
    }

    function testVWAPWithMixedExpiredAndValidValues() public {
        string memory key = "SNX/USD";

        // Set first value
        vm.warp(1710000000);
        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        oracle1.setRawValue(data1);

        // Set second value 2 minutes later
        vm.warp(1710000120);
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000120), uint128(2000), bytes(""));
        oracle1.setRawValue(data2);

        // Set third value 4 minutes after second
        vm.warp(1710000360);
        bytes memory data3 = abi.encode(key, uint128(300), uint128(1710000360), uint128(3000), bytes(""));
        oracle1.setRawValue(data3);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        // Warp forward to make first value expire (but keep others valid)
        vm.warp(1710000800); // First value is 800s old (expired), second is 680s (valid), third is 440s (valid)

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            700, // timeout that includes second and third but excludes first
            1,
            10
        );

        // Only valid values: (200*2000 + 300*3000) / 5000 = 1300000 / 5000 = 260
        assertEq(value, 260, "Should only use non-expired values");
    }

    function testVWAPZeroPriceWithVolume() public {
        string memory key = "ZERO/USD";

        bytes memory data = abi.encode(key, uint128(0), uint128(1710000000), uint128(1000), bytes(""));
        oracle1.setRawValue(data);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(
            key,
            oracles,
            3600,
            1,
            10
        );

        // Zero price is valid if volume > 0
        assertEq(value, 0, "Zero price with volume should be valid");
    }

    function testVWAPOracleWithNoHistory() public {
        string memory key = "EMPTY/USD";

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1); // No data set

        vm.expectRevert(
            abi.encodeWithSelector(
                VolumeWeightedAveragePriceMethodology.ThresholdNotMet.selector,
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

    function testVWAPMultipleKeysIndependent() public {
        string memory key1 = "BTC/USD";
        string memory key2 = "ETH/USD";

        // Set different prices for different keys
        bytes memory data1 = abi.encode(key1, uint128(50000), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode(key2, uint128(3000), uint128(1710000000), uint128(2000), bytes(""));

        oracle1.setRawValue(data1);
        oracle1.setRawValue(data2);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value1, ) = methodology.calculateValue(key1, oracles, 3600, 1, 10);
        (uint128 value2, ) = methodology.calculateValue(key2, oracles, 3600, 1, 10);

        assertEq(value1, 50000, "BTC value should be correct");
        assertEq(value2, 3000, "ETH value should be correct");
    }

    function testVWAPLargeValues() public {
        string memory key = "LARGE/USD";

        uint128 largePrice = 100000000000; // 100 billion
        uint128 largeVolume = 1000000000000; // 1 trillion

        bytes memory data = abi.encode(key, largePrice, uint128(1710000000), largeVolume, bytes(""));
        oracle1.setRawValue(data);

        address[] memory oracles = new address[](1);
        oracles[0] = address(oracle1);

        (uint128 value, ) = methodology.calculateValue(key, oracles, 3600, 1, 10);

        assertEq(value, largePrice, "Should handle large values");
    }

    function testVWAPWindowSizeLargerThanHistory() public {
        string memory key = "WINDOW/USD";

        bytes memory data1 = abi.encode(key, uint128(100), uint128(1710000000), uint128(1000), bytes(""));
        bytes memory data2 = abi.encode(key, uint128(200), uint128(1710000001), uint128(1000), bytes(""));

        oracle1.setRawValue(data1);
        oracle1.setRawValue(data2);

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
}
