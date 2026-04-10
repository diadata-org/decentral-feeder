// SPDX-License-Identifier: GPL-3.0
pragma solidity 0.8.34;

import "forge-std/Script.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import "../contracts/DIAOracleV3/DIAOracleV3.sol";

/**
 * @title DeployDIAOracleV3
 * @notice Deployment script for DIAOracleV3 UUPS upgradeable oracle contract
 * @dev This script deploys:
 *      1. The DIAOracleV3 implementation contract
 *      2. An ERC1967 proxy pointing to the implementation
 *      3. Initializes the proxy with the specified decimal precision
 *
 *      The deployer will receive both DEFAULT_ADMIN_ROLE and UPDATER_ROLE.
 *
 *      Usage:
 *      forge script script/DeployDIAOracleV3.s.sol:DeployDIAOracleV3 --rpc-url <RPC_URL> --broadcast
 *
 *      Environment variables:
 *      - PRIVATE_KEY: The private key of the deployer account
 *      - DECIMALS: (optional) Decimal precision for oracle values (default: 18)
 */
contract DeployDIAOracleV3 is Script {
    // Deployed addresses
    DIAOracleV3 public implementation;
    ERC1967Proxy public proxy;
    DIAOracleV3 public oracle;

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

        vm.startBroadcast(deployerPrivateKey);

        // Deploy implementation contract
        implementation = new DIAOracleV3();
        console.log("DIAOracleV3 implementation deployed at:", address(implementation));

        // Deploy proxy with empty initialization data
        // We'll call initialize separately for clarity
        bytes memory initData = abi.encodeWithSelector(
            DIAOracleV3.initialize.selector,
            decimalPrecision
        );

        proxy = new ERC1967Proxy(
            address(implementation),
            initData
        );
        console.log("ERC1967Proxy deployed at:", address(proxy));

        // Wrap proxy in the oracle interface for easier interaction
        oracle = DIAOracleV3(address(proxy));

        vm.stopBroadcast();

        // Log deployment summary
        console.log("\n=== Deployment Summary ===");
        console.log("Implementation:", address(implementation));
        console.log("Proxy (Oracle):", address(proxy));
        console.log("Decimals:", decimalPrecision);
        console.log("Owner/Admin:", vm.addr(deployerPrivateKey));
        console.log("=========================\n");

        // Log role information
        console.log("=== Role Information ===");
        console.log("Deployer has DEFAULT_ADMIN_ROLE and UPDATER_ROLE");
        console.log("You can grant UPDATER_ROLE to other addresses using:");
        console.log("  oracle.grantRole(keccak256('UPDATER_ROLE'), <updater_address>)");
        console.log("=========================\n");

        // Log verification instructions
        console.log("=== Verification ===");
        console.log("Verify implementation on Etherscan:");
        console.log("  forge verify-contract <implementation_address> DIAOracleV3 \\");
        console.log("    --chain-id <CHAIN_ID> \\");
        console.log("    --watch");
        console.log("");
        console.log("Note: The proxy doesn't need verification as it's a standard ERC1967Proxy");
        console.log("=========================\n");
    }
}
