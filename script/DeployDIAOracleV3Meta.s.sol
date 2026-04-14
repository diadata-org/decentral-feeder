// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "forge-std/Script.sol";
import "../contracts/DIAOracleV3Meta.sol";
import "../contracts/methodologies/MedianPriceMethodology.sol";
import "../contracts/methodologies/AveragePriceMethodology.sol";
import "../contracts/methodologies/VolumeWeightedOracleMethodology.sol";

/**
 * @title DeployDIAOracleV3Meta
 * @notice Deployment script for DIAOracleV3Meta meta oracle contract
 * @dev This script deploys:
 *      1. A price methodology contract (median, average, or VWAP)
 *      2. The DIAOracleV3Meta contract that aggregates multiple oracle sources
 *
 *      The deployer will be the owner and can:
 *      - Add/remove oracle sources
 *      - Change methodology
 *      - Configure validation parameters (threshold, timeout, window size)
 *
 *      Usage:
 *      forge script script/DeployDIAOracleV3Meta.s.sol:DeployDIAOracleV3Meta --rpc-url <RPC_URL> --broadcast
 *
 *      Environment variables:
 *      - PRIVATE_KEY: The private key of the deployer account
 *      - DECIMALS: (optional) Decimal precision for oracle values (default: 18)
 *      - METHODOLOGY: (optional) Price methodology - median, average, or vwap (default: median)
 *      - THRESHOLD: (optional) Minimum valid oracle responses required (default: 1)
 *      - TIMEOUT: (optional) Timeout period in seconds for oracle values (default: 3600)
 *      - WINDOW_SIZE: (optional) Number of historical values to consider (default: 1)
 */
contract DeployDIAOracleV3Meta is Script {
    DIAOracleV3Meta public metaOracle;
    IPriceMethodology public methodology;

    enum Methodology { MEDIAN, AVERAGE, VWAP }

    function run() external {
        // Get deployer private key from environment
        uint256 deployerPrivateKey = vm.envUint("PRIVATE_KEY");

        // Get decimal precision from environment (default to 18 if not set)
        uint8 decimalPrecision = 18;
        try vm.envUint("DECIMALS") returns (uint256 decimalsValue) {
            decimalPrecision = uint8(decimalsValue);
        } catch {
            // Use default value of 18
        }

        // Get methodology type from environment (default to MEDIAN if not set)
        Methodology methodologyType = Methodology.MEDIAN;
        try vm.envString("METHODOLOGY") returns (string memory methodologyStr) {
            if (keccak256(bytes(methodologyStr)) == keccak256(bytes("average"))) {
                methodologyType = Methodology.AVERAGE;
            } else if (keccak256(bytes(methodologyStr)) == keccak256(bytes("vwap"))) {
                methodologyType = Methodology.VWAP;
            } else if (keccak256(bytes(methodologyStr)) == keccak256(bytes("median"))) {
                methodologyType = Methodology.MEDIAN;
            }
        } catch {
            // Use default value of MEDIAN
        }

        // Get threshold from environment (default to 1 if not set)
        uint256 threshold = 1;
        try vm.envUint("THRESHOLD") returns (uint256 thresholdValue) {
            threshold = thresholdValue;
        } catch {
            // Use default value of 1
        }

        // Get timeout from environment (default to 3600 if not set)
        uint256 timeoutSeconds = 3600;
        try vm.envUint("TIMEOUT") returns (uint256 timeoutValue) {
            timeoutSeconds = timeoutValue;
        } catch {
            // Use default value of 3600 (1 hour)
        }

        // Get window size from environment (default to 1 if not set)
        uint256 windowSize = 1;
        try vm.envUint("WINDOW_SIZE") returns (uint256 windowSizeValue) {
            windowSize = windowSizeValue;
        } catch {
            // Use default value of 1
        }

        vm.startBroadcast(deployerPrivateKey);

        // Deploy methodology contract based on selection
        if (methodologyType == Methodology.MEDIAN) {
            methodology = new MedianPriceMethodology();
            console.log("MedianPriceMethodology deployed at:", address(methodology));
        } else if (methodologyType == Methodology.AVERAGE) {
            methodology = new AveragePriceMethodology();
            console.log("AveragePriceMethodology deployed at:", address(methodology));
        } else if (methodologyType == Methodology.VWAP) {
            methodology = new VolumeWeightedOracleMethodology();
            console.log("VolumeWeightedOracleMethodology deployed at:", address(methodology));
        }

        // Deploy DIAOracleV3Meta
        metaOracle = new DIAOracleV3Meta(address(methodology));
        console.log("DIAOracleV3Meta deployed at:", address(metaOracle));

        vm.stopBroadcast();

        // Log deployment summary
        console.log("\n=== Deployment Summary ===");
        console.log("DIAOracleV3Meta:", address(metaOracle));
        console.log("Price Methodology:", address(methodology));
        console.log("Methodology Type:");
        if (methodologyType == Methodology.MEDIAN) {
            console.log("  Median");
        } else if (methodologyType == Methodology.AVERAGE) {
            console.log("  Average");
        } else if (methodologyType == Methodology.VWAP) {
            console.log("  Volume Weighted (VWAP)");
        }
        console.log("Owner:", vm.addr(deployerPrivateKey));
        console.log("=========================\n");

        // Log configuration instructions
        console.log("=== Configuration Instructions ===");
        console.log("To add oracle sources, use:");
        console.log("  metaOracle.addOracle(<oracle_address>)");
        console.log("");
        console.log("To update configuration, use:");
        console.log("  metaOracle.setThreshold(<new_threshold>)");
        console.log("  metaOracle.setTimeoutSeconds(<new_timeout>)");
        console.log("  metaOracle.setWindowSize(<new_window_size>)");
        console.log("  metaOracle.setPriceMethodology(<new_methodology_address>)");
        console.log("=========================\n");

        // Log verification instructions
        console.log("=== Verification ===");
        console.log("Verify DIAOracleV3Meta on Etherscan:");
        console.log("  forge verify-contract <metaOracle_address> DIAOracleV3Meta \\");
        console.log("    --chain-id <CHAIN_ID> \\");
        console.log("    --watch");
        console.log("");
        console.log("Verify methodology contract on Etherscan:");
        console.log("  forge verify-contract <methodology_address> <MethodologyContract> \\");
        console.log("    --chain-id <CHAIN_ID> \\");
        console.log("    --watch");
        console.log("=========================\n");
    }
}
