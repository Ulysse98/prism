import hardhatToolboxViemPlugin from "@nomicfoundation/hardhat-toolbox-viem";
import { configVariable, defineConfig } from "hardhat/config";

export default defineConfig({
  plugins: [hardhatToolboxViemPlugin],

  solidity: {
    profiles: {
      default: {
        version: "0.8.34",
        settings: {
          evmVersion: "prague",
        },
      },

      production: {
        version: "0.8.34",
        settings: {
          evmVersion: "prague",
          optimizer: {
            enabled: true,
            runs: 200,
          },
        },
      },
    },
  },

  networks: {
    hardhatMainnet: {
      type: "edr-simulated",
      chainType: "l1",
    },

    hardhatOp: {
      type: "edr-simulated",
      chainType: "op",
    },

    sepolia: {
      type: "http",
      chainType: "l1",
      url: configVariable("SEPOLIA_RPC_URL"),
      accounts: [
        configVariable("SEPOLIA_PRIVATE_KEY"),
      ],
    },

    arbitrumSepolia: {
      type: "http",
      chainType: "generic",
      chainId: 421614,
      url: "https://sepolia-rollup.arbitrum.io/rpc",
      accounts: [
        configVariable(
          "ARBITRUM_SEPOLIA_PRIVATE_KEY",
        ),
      ],
    },
  },
});