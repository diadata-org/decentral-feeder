// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "forge-std/Script.sol";
import "../contracts/DIAOracleV3Meta.sol";
import "../contracts/methodologies/MedianPriceMethodology.sol";

/**
 * @title DeployMetaOracle
 * @notice Deployment script for DIAOracleV3Meta contract
 * @dev This script deploys:
 *      1. A price methodology contract (MedianPriceMethodology)
 *      2. The DIAOracleV3Meta contract that uses the methodology
 *
 *      Usage:
 *      forge script script/DeployMetaOracle.s.sol:DeployMetaOracle --rpc-url <RPC_URL> --broadcast
 */
contract DeployMetaOracle is Script {
    // Deployed addresses
    MedianPriceMethodology public priceMethodology;
    DIAOracleV3Meta public metaOracle;

    function run() external {
        // Get deployer private key from environment
        uint256 deployerPrivateKey = vm.envUint("PRIVATE_KEY");
        vm.startBroadcast(deployerPrivateKey);

        // Deploy price methodology contract
        priceMethodology = new MedianPriceMethodology();
        console.log("MedianPriceMethodology deployed at:", address(priceMethodology));

        // Deploy DIAOracleV3Meta with methodology address
        metaOracle = new DIAOracleV3Meta(address(priceMethodology));
        console.log("DIAOracleV3Meta deployed at:", address(metaOracle));

        vm.stopBroadcast();

        // Log deployment summary
        console.log("\n=== Deployment Summary ===");
        console.log("Price Methodology:", address(priceMethodology));
        console.log("Meta Oracle:", address(metaOracle));
        console.log("Owner:", vm.addr(deployerPrivateKey));
        console.log("=========================\n");
    }
}
